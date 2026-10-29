<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  deleteCard,
  errorMessage,
  setDeckColor,
  updateCard,
  type Card,
  type CardLog,
  type Deck,
} from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { accuracy, intervalText, lapses, perDay, streak, SETTLED_DAYS } from '@/lib/cards'
import { locale, t } from '@/lib/i18n'
import { PROJECT_COLORS } from '@/lib/projectColor'
import DeleteButton from '@/components/DeleteButton.vue'
import CardText from './CardText.vue'

// Startansicht wie bei Anki: Fächer links (Neu/Lernen/Fällig), Tabelle mit Suche und Sortierung, darunter die gewählte Karte.
const props = defineProps<{
  cards: Card[]
  decks: Deck[]
  log: CardLog[]
  deck: string | null
  colorOf: (deck: string) => string
}>()
const emit = defineEmits<{
  changed: []
  add: [deck: string | null]
  learn: [deck: string | null]
  options: [deck: string]
}>()

type Filter = 'all' | 'due' | 'new' | 'lapsed' | 'notes'
const filter = ref<Filter>('all')
const deck = ref(props.deck ?? '')
const query = ref('')
const sort = ref<'question' | 'answer' | 'deck' | 'due' | 'interval'>('due')
const selKey = ref<string | null>(null)
const error = ref('')
const searchEl = ref<HTMLInputElement>()
const previewMode = ref(false)

const lapsed = computed(() => lapses(props.log))
const TESTS: Record<Filter, (c: Card) => boolean> = {
  all: () => true,
  due: (c) => c.due,
  new: (c) => c.new,
  lapsed: (c) => (lapsed.value.get(c.key) ?? 0) >= 2,
  notes: (c) => !!c.note_path,
}
const FILTERS: [Filter, string][] = [
  ['all', t('Alle Karten')],
  ['due', t('Heute fällig')],
  ['new', t('Neu')],
  ['lapsed', t('Oft vergessen')],
  ['notes', t('Aus Notizen')],
]
const COLS = [
  ['question', t('Frage')],
  ['answer', t('Antwort')],
  ['deck', t('Fach')],
  ['due', t('Fällig')],
  ['interval', t('Stufe')],
] as const

// Sortierschlüssel für „Fällig“: neu zuerst, dann nach Datum.
const dueKey = (c: Card) => (c.new ? '' : c.due ? '0' : (c.due_date ?? ''))
const rows = computed(() => {
  const q = query.value.trim().toLowerCase()
  const out = props.cards.filter(
    (c) =>
      TESTS[filter.value](c) &&
      (!deck.value || c.deck === deck.value) &&
      (!q || `${c.question}\n${c.answer}`.toLowerCase().includes(q)),
  )
  const key = sort.value
  return out.sort((a, b) =>
    key === 'interval'
      ? a.interval_days - b.interval_days
      : key === 'due'
        ? dueKey(a).localeCompare(dueKey(b))
        : a[key].localeCompare(b[key], locale.value),
  )
})
const sel = computed(() => props.cards.find((c) => c.key === selKey.value) ?? rows.value[0])

const dateFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }),
)
const dueText = (c: Card) =>
  c.new ? t('neu') : c.due ? t('heute') : dateFmt.value.format(new Date(c.due_date + 'T00:00'))

// Bearbeiten: Felder spiegeln die Auswahl; gespeichert wird beim Verlassen eines geänderten Felds.
const form = ref({ deck: '', question: '', answer: '' })
watch(
  sel,
  (c) =>
    (form.value = { deck: c?.deck ?? '', question: c?.question ?? '', answer: c?.answer ?? '' }),
  { immediate: true },
)
async function save() {
  const c = sel.value
  const body = {
    deck: form.value.deck.trim(),
    question: form.value.question.trim(),
    answer: form.value.answer.trim(),
  }
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
const clear = () => {
  query.value = ''
  filter.value = 'all'
  deck.value = ''
}

// Fächerzeilen; „Alle“ (name '') zuerst. review = fällige Wiederholungen ohne neue und Lernkarten.
const deckRows = computed(() => {
  const sum = (k: 'new_due' | 'learn_due' | 'due') => props.decks.reduce((n, d) => n + d[k], 0)
  const all = {
    name: '',
    label: t('Alle'),
    new_due: sum('new_due'),
    learn_due: sum('learn_due'),
    due: sum('due'),
  }
  return [all, ...props.decks.map((d) => ({ ...d, label: d.name }))].map((d) => ({
    ...d,
    review: d.due - d.new_due - d.learn_due,
  }))
})
const current = computed(
  () => deckRows.value.find((d) => d.name === deck.value) ?? deckRows.value[0]!,
)
const shown = computed(() => props.cards.filter((c) => !deck.value || c.deck === deck.value).length)

// Stufe 0–4 aus dem Abstand: neu, in Arbeit, jung, fast sicher, sitzt.
const stage = (c: Card) =>
  c.new ? 0 : c.learning ? 1 : c.interval_days < SETTLED_DAYS ? 2 : c.interval_days < 30 ? 3 : 4
const days = computed(() => perDay(props.log))
const inRow = computed(() => streak(days.value))
const acc30 = computed(() =>
  accuracy(props.log.filter((l) => Date.now() - Date.parse(l.at) < 30 * 864e5)),
)
const createdFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }),
)

const newDeck = ref('')
const addingDeck = ref(false)
async function addDeck() {
  const name = newDeck.value.trim()
  if (!name) return
  const used = new Set(props.decks.map((d) => d.color))
  const color =
    PROJECT_COLORS.find(([c]) => !used.has(c))?.[0] ??
    PROJECT_COLORS[props.decks.length % PROJECT_COLORS.length]![0]
  try {
    await setDeckColor(name, color)
    newDeck.value = ''
    addingDeck.value = false
    deck.value = name
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

useShortcuts((e) => {
  if (e.key === '/') {
    e.preventDefault()
    searchEl.value?.focus()
  } else if (e.key === 'n') {
    e.preventDefault()
    emit('add', deck.value || null)
  } else if (e.key === 'l' && current.value.due) {
    emit('learn', deck.value || null)
  } else if (/^[1-9]$/.test(e.key)) {
    const d = props.decks[Number(e.key) - 1]
    if (d) deck.value = d.name
  } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const i = rows.value.findIndex((c) => c.key === sel.value?.key)
    const next =
      rows.value[Math.min(rows.value.length - 1, Math.max(0, i + (e.key === 'ArrowDown' ? 1 : -1)))]
    if (next) selKey.value = next.key
  }
})
</script>

<template>
  <div class="kb">
    <aside class="kb-side">
      <h1 class="v-title kb-title">{{ $t('Lernkarten') }}</h1>
      <div class="kb-group" role="group" :aria-label="$t('Fächer')">
        <div class="kb-dh" aria-hidden="true">
          <span>{{ $t('Fach') }}</span
          ><span>{{ $t('Neu') }}</span
          ><span>{{ $t('Lern.') }}</span
          ><span>{{ $t('Fällig') }}</span>
        </div>
        <button
          v-for="d in deckRows"
          :key="d.name"
          type="button"
          class="kb-deck"
          :aria-pressed="deck === d.name"
          :aria-label="`${d.label}: ${$t('{a} neu, {b} in Arbeit, {c} Wiederholung', { a: d.new_due, b: d.learn_due, c: d.review })}`"
          @click="deck = d.name"
        >
          <span class="kb-dn"
            ><i
              class="kb-dot"
              :style="{ background: d.name ? colorOf(d.name) : 'var(--tx-muted)' }"
            ></i
            ><span class="kb-fname" :title="d.label">{{ d.label }}</span></span
          >
          <b class="mono" :class="{ z: !d.new_due }">{{ d.new_due }}</b>
          <b class="mono" :class="{ z: !d.learn_due }">{{ d.learn_due }}</b>
          <b class="mono due" :class="{ z: !d.review }">{{ d.review }}</b>
        </button>
        <form v-if="addingDeck" class="kb-newdeck" @submit.prevent="addDeck">
          <input
            v-model="newDeck"
            class="input"
            autofocus
            :placeholder="$t('Neues Fach, z. B. Mathe 2')"
            :aria-label="$t('Neues Fach')"
            @keydown.esc="addingDeck = false"
          />
          <button class="btn btn-primary" type="submit" :disabled="!newDeck.trim()">
            {{ $t('Fach anlegen') }}
          </button>
        </form>
        <button v-else type="button" class="kb-fi kb-add" @click="addingDeck = true">
          {{ $t('+ Fach anlegen') }}
        </button>
      </div>
      <div class="kb-group" role="group" :aria-label="$t('Ansicht')">
        <div class="kb-gh">{{ $t('Ansicht') }}</div>
        <button
          v-for="[f, label] in FILTERS"
          :key="f"
          type="button"
          class="kb-fi"
          :aria-pressed="filter === f"
          @click="filter = f"
        >
          {{ label
          }}<b class="mono">{{
            cards.filter((c) => TESTS[f](c) && (!deck || c.deck === deck)).length
          }}</b>
        </button>
      </div>
      <p class="kb-stats">
        <span>{{ $tn(inRow, '{n} Tag in Folge', '{n} Tage in Folge') }}</span>
        <span v-if="acc30 !== null">{{ $t('{n} % richtig', { n: acc30 }) }}</span>
      </p>
    </aside>

    <main class="kb-main">
      <header class="kb-head">
        <div class="kb-ht">
          <h2>{{ deck || $t('Alle Karten') }}</h2>
          <span
            >{{ $tn(shown, '{n} Karte', '{n} Karten')
            }}<template v-if="current.due">
              · {{ $t('{n} heute fällig', { n: current.due }) }}</template
            ></span
          >
        </div>
        <label class="kb-search">
          <svg class="ic" aria-hidden="true"><use href="#i-search" /></svg>
          <input
            ref="searchEl"
            v-model="query"
            type="search"
            :aria-label="$t('Karten durchsuchen')"
            :placeholder="$t('Suchen')"
          />
          <kbd>/</kbd>
        </label>
        <button
          v-if="deck"
          type="button"
          class="btn btn-secondary kb-more"
          :aria-label="$t('Fach-Optionen')"
          :title="$t('Fach-Optionen')"
          @click="emit('options', deck)"
        >
          ⋯
        </button>
        <button type="button" class="btn btn-secondary" @click="emit('add', deck || null)">
          {{ $t('Hinzufügen') }}<kbd>N</kbd>
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!current.due"
          @click="emit('learn', deck || null)"
        >
          {{ current.due ? $t('{n} lernen', { n: current.due }) : $t('Nichts fällig')
          }}<kbd v-if="current.due">L</kbd>
        </button>
      </header>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div class="kb-table" role="table" :aria-label="$t('Karten')">
        <div class="kb-tr kb-th" role="row">
          <button
            v-for="[k, label] in COLS"
            :key="k"
            type="button"
            role="columnheader"
            :aria-sort="sort === k ? 'ascending' : 'none'"
            :class="{ on: sort === k }"
            @click="sort = k"
          >
            {{ label }}<span v-if="sort === k" aria-hidden="true"> ↑</span>
          </button>
        </div>
        <button
          v-for="c in rows"
          :key="c.key"
          type="button"
          role="row"
          class="kb-tr"
          :class="{ on: c.key === sel?.key }"
          @click="selKey = c.key"
        >
          <span class="kb-cell kb-q">{{ c.question }}</span>
          <span class="kb-cell kb-a">{{ c.answer }}</span>
          <span class="kb-cell kb-d"
            ><i class="kb-dot" :style="{ background: colorOf(c.deck) }"></i>{{ c.deck }}</span
          >
          <span class="kb-due" :class="{ due: c.due && !c.new, new: c.new }">{{ dueText(c) }}</span>
          <span class="kb-pips" :aria-label="$t('Stufe {n} von 4', { n: stage(c) })"
            ><i v-for="n in 4" :key="n" :class="{ on: n <= stage(c) }"></i
          ></span>
        </button>
        <p v-if="!rows.length && cards.length" class="kb-empty">
          {{ $t('Keine Karte passt.') }}
          <button type="button" class="col-link" @click="clear">
            {{ $t('Filter zurücksetzen') }}
          </button>
        </p>
        <p v-else-if="!rows.length" class="kb-empty">
          {{
            $t('Noch keine Karten. Leg ein Fach an oder füg eine Liste aus Anki oder Quizlet ein.')
          }}
        </p>
      </div>

      <section v-if="sel" class="kb-edit" :aria-label="$t('Karte bearbeiten')">
        <template v-if="sel.id">
          <div v-if="!previewMode" class="kb-edit-fields">
            <div class="field">
              <label for="kb-q">{{ $t('Vorderseite') }}</label>
              <textarea
                id="kb-q"
                v-model="form.question"
                class="input kb-q-in"
                rows="2"
                @change="save"
              ></textarea>
            </div>
            <div class="field">
              <label for="kb-a">{{ $t('Rückseite') }}</label>
              <textarea
                id="kb-a"
                v-model="form.answer"
                class="input"
                rows="3"
                @change="save"
              ></textarea>
            </div>
            <p class="kb-hint">
              {{
                $t(
                  'Markdown, $Formeln$ und Bilder gehen. Speichert beim Verlassen des Felds; eine neue Frage beginnt von vorn.',
                )
              }}
            </p>
          </div>
          <div v-else class="kb-preview-card">
            <CardText class="kb-preview-front" :text="form.question || '–'" />
            <div class="kb-preview-divider"></div>
            <CardText class="kb-preview-back" :text="form.answer || '–'" />
          </div>
        </template>
        <div v-else class="kb-ro">
          <b>{{ sel.question }}</b
          ><span>{{ sel.answer }}</span>
          <p class="kb-hint">{{ $t('Karten aus Notizen änderst du in der Notiz.') }}</p>
        </div>

        <div class="kb-info">
          <div v-if="sel.id" class="field">
            <label for="kb-deck">{{ $t('Fach') }}</label>
            <input id="kb-deck" v-model="form.deck" class="input" list="kb-decks" @change="save" />
            <datalist id="kb-decks">
              <option v-for="d in decks" :key="d.name" :value="d.name" />
            </datalist>
          </div>
          <dl>
            <dt>{{ $t('Fällig') }}</dt>
            <dd :class="{ due: sel.due && !sel.new }">{{ dueText(sel) }}</dd>
            <dt>{{ $t('Stufe') }}</dt>
            <dd>
              {{ $t('{n} von 4', { n: stage(sel) })
              }}<template v-if="sel.interval_days && !sel.new">
                · {{ intervalText(sel.interval_days) }}</template
              >
            </dd>
            <template v-if="lapsed.get(sel.key)"
              ><dt>{{ $t('Vergessen') }}</dt>
              <dd>{{ lapsed.get(sel.key) }}×</dd></template
            >
            <template v-if="sel.created_at"
              ><dt>{{ $t('Erstellt') }}</dt>
              <dd>{{ createdFmt.format(new Date(sel.created_at)) }}</dd></template
            >
          </dl>
          <div class="kb-eact">
            <template v-if="sel.id">
              <button
                type="button"
                class="btn btn-secondary btn-s"
                @click="previewMode = !previewMode"
              >
                {{ previewMode ? $t('Bearbeiten') : $t('Vorschau') }}
              </button>
              <DeleteButton ghost :text="$t('Karte löschen?')" @confirm="remove" />
            </template>
            <router-link
              v-else
              class="btn btn-secondary btn-s"
              :to="{ path: '/notes', query: { open: sel.note_path } }"
              >{{ $t('Notiz öffnen') }}</router-link
            >
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<style scoped>
/* Eine Akzentfarbe: Blau heißt „jetzt fällig“. Alles andere bleibt in Grautönen; Fachfarben nur als Punkt. */
.kb {
  flex: 1;
  display: flex;
  margin: -32px -32px -64px;
  min-height: 100dvh;
}
.kb-side {
  width: 300px;
  flex-shrink: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 22px;
  padding: 28px 14px 20px;
  border-right: 1px solid var(--br-subtle);
}
.kb-title {
  margin: 0 10px;
}
.kb-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.kb-gh {
  padding: 0 10px 4px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.kb-dh,
.kb-deck {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 26px 26px 34px;
  gap: 4px;
  align-items: center;
}
.kb-dh {
  padding: 0 10px 6px;
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.kb-dh span + span {
  text-align: right;
}
.kb-deck,
.kb-fi {
  height: 36px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--r-s);
  background: none;
  color: var(--tx-secondary);
  font: 400 13.5px/1 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.kb-deck:hover,
.kb-fi:hover {
  background: var(--bg-hover);
  color: var(--tx-primary);
}
.kb-deck[aria-pressed='true'],
.kb-fi[aria-pressed='true'] {
  background: var(--bg-selected);
  color: var(--tx-primary);
}
.kb-dn {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.kb-deck b {
  text-align: right;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--tx-secondary);
}
.kb-deck b.due {
  color: var(--a-blue-tx);
}
.kb-deck b.z {
  color: var(--tx-disabled);
  font-weight: 400;
}
.kb-fi {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 32px;
  font-size: 13px;
}
.kb-fi b {
  margin-left: auto;
  font-size: 12px;
  font-weight: 400;
  color: var(--tx-muted);
}
.kb-add {
  color: var(--tx-muted);
}
.kb-newdeck {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 4px 0;
}
.kb-fname {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kb-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.kb-stats {
  margin: auto 10px 0;
  padding-top: 12px;
  border-top: 1px solid var(--br-subtle);
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font: 400 12px/1.4 var(--font-ui);
  color: var(--tx-muted);
}

.kb-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.kb-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 24px 24px 16px;
  border-bottom: 1px solid var(--br-subtle);
}
.kb-ht {
  flex: 1;
  min-width: 180px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.kb-ht h2 {
  margin: 0;
  font: 600 19px/1.25 var(--font-ui);
  color: var(--tx-primary);
}
.kb-ht span {
  font: 400 12.5px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.kb-search {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 220px;
  height: 36px;
  padding: 0 10px;
  box-sizing: border-box;
  border-radius: var(--r-s);
  border: 1px solid var(--br-default);
  background: var(--bg-1);
  color: var(--tx-muted);
}
.kb-search:focus-within {
  border-color: var(--a-focus);
}
.kb-search input {
  flex: 1;
  min-width: 0;
  height: 32px;
  border: 0;
  background: none;
  outline: none;
  color: var(--tx-primary);
  font: 400 13.5px/1 var(--font-ui);
}
.kb-more {
  width: 36px;
  padding: 0;
  justify-content: center;
}
kbd {
  margin-left: 8px;
  font: 500 10.5px/1 var(--font-mono);
  padding: 2px 5px;
  border-radius: 4px;
  border: 1px solid var(--br-default);
  color: var(--tx-muted);
}
.kb-search kbd {
  margin-left: auto;
}
.btn-primary kbd {
  border-color: color-mix(in srgb, var(--tx-on-accent) 35%, transparent);
  color: inherit;
  opacity: 0.8;
}

.kb-table {
  flex: 1;
  padding: 4px 12px 12px;
}
.kb-tr {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) 150px 84px 64px;
  gap: 16px;
  align-items: center;
  width: 100%;
  min-height: 42px;
  padding: 0 12px;
  border: 0;
  border-bottom: 1px solid var(--br-subtle);
  border-radius: 0;
  background: none;
  color: var(--tx-primary);
  font: 400 13.5px/1.3 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.kb-tr:hover {
  background: var(--bg-hover);
}
.kb-tr.on {
  background: var(--bg-selected);
  border-radius: var(--r-s);
  border-bottom-color: transparent;
}
.kb-th {
  position: sticky;
  top: 0;
  z-index: 1;
  min-height: 38px;
  background: var(--bg-0);
  cursor: default;
}
.kb-th:hover {
  background: var(--bg-0);
}
.kb-th button {
  padding: 0;
  border: 0;
  background: none;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  text-align: left;
  cursor: pointer;
}
.kb-th button.on {
  color: var(--tx-primary);
}
.kb-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kb-a {
  color: var(--tx-muted);
}
.kb-d {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--tx-secondary);
}
.kb-due {
  font-size: 12.5px;
  color: var(--tx-muted);
}
.kb-due.due {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.kb-due.new {
  color: var(--tx-primary);
}
.kb-pips {
  display: flex;
  gap: 3px;
}
.kb-pips i {
  width: 12px;
  height: 4px;
  border-radius: 2px;
  background: var(--bg-3);
}
.kb-pips i.on {
  background: var(--tx-secondary);
}
.kb-empty {
  padding: 24px 12px;
  margin: 0;
  font: 400 13.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.kb-empty button {
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}

/* Gewählte Karte: klebt unten, damit sie beim Scrollen der Liste sichtbar bleibt. */
.kb-edit {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 240px;
  gap: 28px;
  padding: 18px 24px 22px;
  border-top: 1px solid var(--br-subtle);
  background: var(--bg-1);
}
.kb-edit-fields {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.kb-edit textarea.input {
  height: auto;
  resize: vertical;
  font: 400 14px/1.5 var(--font-ui);
  padding: 10px 12px;
}
.kb-edit textarea.kb-q-in {
  font: 500 16px/1.4 var(--font-ui);
}
.kb-preview-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px 24px;
  border-radius: var(--r-m);
  background: var(--bg-2);
  border: 1px solid var(--br-subtle);
  text-align: center;
}
.kb-preview-front {
  font: 500 18px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.kb-preview-back {
  font: 400 15px/1.5 var(--font-ui);
  color: var(--tx-secondary);
}
.kb-preview-divider {
  border-top: 1px dashed var(--br-strong);
}
.kb-hint {
  margin: 0;
  font: 400 12px/1.5 var(--font-ui);
  color: var(--tx-muted);
}
.kb-info {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}
.kb-info dl {
  margin: 0;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 14px;
  font: 400 12.5px/1.3 var(--font-ui);
}
.kb-info dt {
  color: var(--tx-muted);
}
.kb-info dd {
  margin: 0;
  color: var(--tx-primary);
}
.kb-info dd.due {
  color: var(--a-blue-tx);
}
.kb-eact {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.kb-ro {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font: 400 14px/1.5 var(--font-ui);
  color: var(--tx-secondary);
  white-space: pre-wrap;
}
.kb-ro b {
  font-weight: 500;
  color: var(--tx-primary);
}

@media (max-width: 1100px) {
  .kb-tr {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 84px 64px;
  }
  .kb-d,
  .kb-th button:nth-child(3) {
    display: none;
  }
}
@media (max-width: 760px) {
  .kb {
    flex-direction: column;
    margin: -20px -16px -96px;
    min-height: 0;
  }
  .kb-side {
    width: auto;
    gap: 12px;
    padding: 16px 12px 12px;
    border-right: 0;
    border-bottom: 1px solid var(--br-subtle);
  }
  .kb-title {
    margin: 0 4px;
  }
  .kb-group[aria-label] + .kb-group,
  .kb-stats,
  .kb-dh,
  kbd {
    display: none;
  }
  .kb-group {
    flex-direction: row;
    gap: 6px;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .kb-deck {
    flex-shrink: 0;
    display: flex;
    gap: 8px;
    height: 34px;
    border: 1px solid var(--br-default);
    border-radius: 999px;
  }
  .kb-deck b:not(.due),
  .kb-deck b.z {
    display: none;
  }
  .kb-add {
    flex-shrink: 0;
  }
  .kb-head {
    padding: 14px 12px;
  }
  .kb-search {
    order: 5;
    width: 100%;
  }
  .kb-table {
    padding: 0 4px 12px;
  }
  .kb-tr {
    grid-template-columns: minmax(0, 1fr) 70px;
    min-height: 48px;
  }
  .kb-a,
  .kb-pips,
  .kb-th button:nth-child(2),
  .kb-th button:nth-child(5) {
    display: none;
  }
  .kb-edit {
    position: static;
    grid-template-columns: 1fr;
    gap: 16px;
    padding: 16px 12px 24px;
  }
}
</style>
