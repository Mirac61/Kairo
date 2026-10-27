<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { deleteCard, errorMessage, updateCard, type Card, type CardLog, type Deck } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { intervalText, lapses } from '@/lib/cards'
import { locale, t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

// Kartenbrowser wie bei Anki: Filter links, Tabelle (Suche, Sortierung per Spaltenkopf), rechts die gewählte Karte.
const props = defineProps<{ cards: Card[]; decks: Deck[]; log: CardLog[]; deck: string | null; colorOf: (deck: string) => string }>()
const emit = defineEmits<{ back: []; changed: []; add: [] }>()

type Filter = 'all' | 'due' | 'new' | 'lapsed' | 'notes'
const filter = ref<Filter>('all')
const deck = ref(props.deck ?? '')
const query = ref('')
const sort = ref<'question' | 'answer' | 'deck' | 'due' | 'interval'>('due')
const selKey = ref<string | null>(null)
const error = ref('')
const searchEl = ref<HTMLInputElement>()

const lapsed = computed(() => lapses(props.log))
const TESTS: Record<Filter, (c: Card) => boolean> = {
  all: () => true,
  due: (c) => c.due,
  new: (c) => c.new,
  lapsed: (c) => (lapsed.value.get(c.key) ?? 0) >= 2,
  notes: (c) => !!c.note_path,
}
const FILTERS: [Filter, string][] = [['all', t('Alle Karten')], ['due', t('Heute fällig')], ['new', t('Neu')], ['lapsed', t('Oft vergessen')], ['notes', t('Aus Notizen')]]
const COLS = [['question', t('Frage')], ['answer', t('Antwort')], ['deck', t('Fach')], ['due', t('Fällig')], ['interval', t('Abstand')]] as const

// Sortierschlüssel für „Fällig“: neu zuerst, dann nach Datum.
const dueKey = (c: Card) => (c.new ? '' : c.due ? '0' : c.due_date ?? '')
const rows = computed(() => {
  const q = query.value.trim().toLowerCase()
  const out = props.cards.filter((c) => TESTS[filter.value](c) && (!deck.value || c.deck === deck.value) && (!q || `${c.question}\n${c.answer}`.toLowerCase().includes(q)))
  const key = sort.value
  return out.sort((a, b) =>
    key === 'interval' ? a.interval_days - b.interval_days
      : key === 'due' ? dueKey(a).localeCompare(dueKey(b))
        : a[key].localeCompare(b[key], locale.value))
})
const sel = computed(() => props.cards.find((c) => c.key === selKey.value) ?? rows.value[0])

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }))
const dueText = (c: Card) => (c.new ? t('neu') : c.due ? t('heute') : dateFmt.value.format(new Date(c.due_date + 'T00:00')))

// Bearbeiten: Felder spiegeln die Auswahl; gespeichert wird beim Verlassen eines geänderten Felds.
const form = ref({ deck: '', question: '', answer: '' })
watch(sel, (c) => (form.value = { deck: c?.deck ?? '', question: c?.question ?? '', answer: c?.answer ?? '' }), { immediate: true })
async function save() {
  const c = sel.value
  const body = { deck: form.value.deck.trim(), question: form.value.question.trim(), answer: form.value.answer.trim() }
  if (!c?.id || !body.deck || !body.question || !body.answer) return
  if (body.deck === c.deck && body.question === c.question && body.answer === c.answer) return
  try {
    const saved = await updateCard(c.id, body)
    selKey.value = saved.key
    error.value = ''
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
async function remove() {
  if (!sel.value?.id) return
  try {
    await deleteCard(sel.value.id)
    selKey.value = null
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const clear = () => { query.value = ''; filter.value = 'all'; deck.value = '' }

useShortcuts((e) => {
  if (e.key === '/') {
    e.preventDefault()
    searchEl.value?.focus()
  } else if (e.key === 'Escape') {
    emit('back')
  } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const i = rows.value.findIndex((c) => c.key === sel.value?.key)
    const next = rows.value[Math.min(rows.value.length - 1, Math.max(0, i + (e.key === 'ArrowDown' ? 1 : -1)))]
    if (next) selKey.value = next.key
  }
})
</script>

<template>
  <div class="kb">
    <aside class="kb-side">
      <button type="button" class="col-link kb-back" @click="emit('back')"><svg class="ic"><use href="#i-left" /></svg>{{ $t('Lernkarten') }}</button>
      <div class="kb-group" role="group" :aria-label="$t('Ansicht')">
        <div class="kb-gh">{{ $t('Ansicht') }}</div>
        <button v-for="[f, label] in FILTERS" :key="f" type="button" class="kb-fi" :aria-pressed="filter === f" @click="filter = f">
          {{ label }}<b class="mono">{{ cards.filter(TESTS[f]).length }}</b>
        </button>
      </div>
      <div class="kb-group" role="group" :aria-label="$t('Fächer')">
        <div class="kb-gh">{{ $t('Fächer') }}</div>
        <button v-for="d in decks" :key="d.name" type="button" class="kb-fi" :aria-pressed="deck === d.name" @click="deck = deck === d.name ? '' : d.name">
          <i class="kb-dot" :style="{ background: colorOf(d.name) }"></i><span class="kb-fname">{{ d.name }}</span><b class="mono">{{ d.total }}</b>
        </button>
      </div>
    </aside>

    <main class="kb-main">
      <header class="kb-head">
        <label class="kb-search">
          <svg class="ic" aria-hidden="true"><use href="#i-search" /></svg>
          <input ref="searchEl" v-model="query" type="search" :aria-label="$t('Karten durchsuchen')" :placeholder="$t('Frage oder Antwort suchen')" />
          <kbd>/</kbd>
        </label>
        <span class="mono kb-count">{{ $t('{n} Karten', { n: rows.length }) }}</span>
        <button type="button" class="btn btn-secondary" @click="emit('add')">{{ $t('Hinzufügen') }}</button>
      </header>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div class="kb-body">
        <div class="kb-table" role="table" :aria-label="$t('Karten')">
          <div class="kb-tr kb-th" role="row">
            <button v-for="[k, label] in COLS" :key="k" type="button" role="columnheader" :aria-sort="sort === k ? 'ascending' : 'none'" :class="{ on: sort === k }" @click="sort = k">
              {{ label }}<span v-if="sort === k" aria-hidden="true"> ↓</span>
            </button>
          </div>
          <button v-for="c in rows" :key="c.key" type="button" role="row" class="kb-tr" :class="{ on: c.key === sel?.key }" @click="selKey = c.key">
            <span class="kb-cell kb-q">{{ c.question }}</span>
            <span class="kb-cell kb-a">{{ c.answer }}</span>
            <span class="kb-cell kb-d"><i class="kb-dot" :style="{ background: colorOf(c.deck) }"></i>{{ c.deck }}</span>
            <span class="mono kb-due" :class="{ due: c.due }">{{ dueText(c) }}</span>
            <span class="mono kb-iv">{{ c.new || !c.interval_days ? '–' : intervalText(c.interval_days) }}</span>
          </button>
          <p v-if="!rows.length" class="kb-empty">{{ $t('Keine Karte passt.') }} <button type="button" class="col-link" @click="clear">{{ $t('Filter zurücksetzen') }}</button></p>
        </div>

        <aside v-if="sel" class="kb-edit" :aria-label="$t('Karte bearbeiten')">
          <div class="kb-eh"><span>{{ $t('Ausgewählt') }}</span><span class="mono">{{ sel.new ? $t('neu') : `${sel.interval_days ? intervalText(sel.interval_days) : $t('in Arbeit')} · ${dueText(sel)}` }}<template v-if="lapsed.get(sel.key)"> · {{ $t('{n}× vergessen', { n: lapsed.get(sel.key)! }) }}</template></span></div>
          <template v-if="sel.id">
            <div class="field"><label for="kb-deck">{{ $t('Fach') }}</label>
              <input id="kb-deck" v-model="form.deck" class="input" list="kb-decks" @change="save" />
              <datalist id="kb-decks"><option v-for="d in decks" :key="d.name" :value="d.name" /></datalist></div>
            <div class="field"><label for="kb-q">{{ $t('Vorderseite') }}</label><textarea id="kb-q" v-model="form.question" class="input" rows="3" @change="save"></textarea></div>
            <div class="field"><label for="kb-a">{{ $t('Rückseite') }}</label><textarea id="kb-a" v-model="form.answer" class="input" rows="6" @change="save"></textarea></div>
            <p class="kb-hint">{{ $t('Markdown, $Formeln$ und Bilder gehen. Speichert beim Verlassen des Felds; eine neue Frage beginnt von vorn.') }}</p>
            <div class="kb-eact"><DeleteButton ghost :text="$t('Karte löschen?')" @confirm="remove" /></div>
          </template>
          <template v-else>
            <div class="kb-ro"><b>{{ sel.question }}</b><span>{{ sel.answer }}</span></div>
            <p class="kb-hint">{{ $t('Karten aus Notizen änderst du in der Notiz.') }}</p>
            <router-link class="btn btn-secondary kb-note" :to="{ path: '/notes', query: { open: sel.note_path } }">{{ $t('Notiz öffnen') }}</router-link>
          </template>
        </aside>
      </div>
    </main>
  </div>
</template>

<style scoped>
.kb { flex: 1; display: flex; margin: -32px -32px -64px; }
.kb-side { width: 230px; flex-shrink: 0; display: flex; flex-direction: column; gap: 18px; padding: 20px 12px; border-right: 1px solid var(--br-subtle); }
.kb-back { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border: 0; background: none; cursor: pointer; }
.kb-group { display: flex; flex-direction: column; gap: 2px; }
.kb-gh { padding: 0 10px 4px; font: 500 11.5px/1 var(--font-ui); letter-spacing: 0.04em; text-transform: uppercase; color: var(--tx-muted); }
.kb-fi { display: flex; align-items: center; gap: 10px; height: 34px; padding: 0 10px; border: 0; border-radius: var(--r-s); background: none; color: var(--tx-secondary); font: 400 13.5px/1 var(--font-ui); text-align: left; cursor: pointer; }
.kb-fi:hover { background: var(--bg-1); color: var(--tx-primary); }
.kb-fi[aria-pressed='true'] { background: var(--bg-2); color: var(--tx-primary); }
.kb-fi b { margin-left: auto; font-size: 12px; font-weight: 400; color: var(--tx-muted); }
.kb-fname { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kb-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.kb-main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.kb-head { display: flex; align-items: center; gap: 10px; padding: 14px 18px; border-bottom: 1px solid var(--br-subtle); }
.kb-search { flex: 1; display: flex; align-items: center; gap: 8px; height: 38px; padding: 0 12px; border-radius: var(--r-s); border: 1px solid var(--br-default); background: var(--bg-1); color: var(--tx-muted); }
.kb-search:focus-within { border-color: var(--a-focus); }
.kb-search input { flex: 1; min-width: 0; height: 34px; border: 0; background: none; outline: none; color: var(--tx-primary); font: 400 13.5px/1 var(--font-ui); }
kbd { font: 500 10.5px/1 var(--font-mono); padding: 2px 5px; border-radius: 4px; border: 1px solid var(--br-default); color: var(--tx-muted); }
.kb-count { font-size: 12.5px; color: var(--tx-muted); white-space: nowrap; }
.kb-body { flex: 1; min-height: 0; display: flex; }
.kb-table { flex: 1; min-width: 0; overflow: auto; }
.kb-tr { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.3fr) 160px 80px 64px; gap: 14px; align-items: center; width: 100%; min-height: 44px; padding: 0 18px; border: 0; border-top: 1px solid var(--br-subtle); background: none; color: var(--tx-primary); font: 400 13.5px/1.3 var(--font-ui); text-align: left; cursor: pointer; }
.kb-tr:hover { background: var(--bg-1); }
.kb-tr.on { background: var(--bg-2); box-shadow: inset 2px 0 0 var(--a-event); }
.kb-th { position: sticky; top: 0; z-index: 1; min-height: 36px; border-top: 0; background: var(--bg-0); cursor: default; }
.kb-th:hover { background: var(--bg-0); }
.kb-th button { padding: 0; border: 0; background: none; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); text-align: left; cursor: pointer; }
.kb-th button.on { color: var(--tx-primary); }
.kb-cell { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kb-q { font-weight: 500; }
.kb-a, .kb-d { color: var(--tx-secondary); }
.kb-d { display: flex; align-items: center; gap: 8px; }
.kb-due, .kb-iv { font-size: 12px; color: var(--tx-muted); }
.kb-due.due { color: var(--p-green); }
.kb-empty { padding: 24px 18px; margin: 0; font: 400 13.5px/1.4 var(--font-ui); color: var(--tx-muted); }
.kb-empty button { padding: 0; border: 0; background: none; cursor: pointer; }
.kb-edit { width: 360px; flex-shrink: 0; display: flex; flex-direction: column; gap: 14px; padding: 18px; border-left: 1px solid var(--br-subtle); background: var(--bg-1); overflow-y: auto; }
.kb-eh { display: flex; justify-content: space-between; gap: 8px; font: 400 12.5px/1.3 var(--font-ui); color: var(--tx-muted); }
.kb-edit textarea.input { height: auto; resize: vertical; font: 400 14px/1.5 var(--font-ui); padding: 10px 12px; }
.kb-hint { margin: 0; font: 400 12px/1.5 var(--font-ui); color: var(--tx-muted); }
.kb-eact { margin-top: auto; display: flex; gap: 8px; }
.kb-ro { display: flex; flex-direction: column; gap: 8px; font: 400 14px/1.5 var(--font-ui); color: var(--tx-secondary); white-space: pre-wrap; }
.kb-ro b { font-weight: 500; color: var(--tx-primary); }
.kb-note { align-self: flex-start; }

@media (max-width: 1100px) {
  .kb-edit { width: 300px; }
  .kb-tr { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 80px 56px; }
  .kb-d, .kb-th button:nth-child(3) { display: none; }
}
@media (max-width: 760px) {
  .kb { flex-direction: column; margin: -20px -16px -96px; }
  .kb-side { width: auto; flex-direction: row; flex-wrap: wrap; gap: 6px; padding: 12px; border-right: 0; border-bottom: 1px solid var(--br-subtle); }
  .kb-group { flex-direction: row; flex-wrap: wrap; gap: 4px; }
  .kb-gh { display: none; }
  .kb-back { width: 100%; }
  .kb-head kbd { display: none; }
  .kb-body { flex-direction: column; }
  .kb-tr { grid-template-columns: minmax(0, 1fr) 70px; }
  .kb-a, .kb-iv, .kb-th button:nth-child(2), .kb-th button:nth-child(5) { display: none; }
  .kb-edit { width: auto; border-left: 0; border-top: 1px solid var(--br-subtle); }
}
</style>
