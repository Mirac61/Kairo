<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
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
import { accuracy, lapses, perDay, streak, SETTLED_DAYS } from '@/lib/cards'
import { locale, t } from '@/lib/i18n'
import { PROJECT_COLORS } from '@/lib/projectColor'
import IconButton from '@/components/IconButton.vue'
import SearchField from '@/components/SearchField.vue'
import CardBrowserAside from './CardBrowserAside.vue'
import CardInlineText from './CardInlineText.vue'

// Startansicht wie bei Anki: Fächer oben, Tabelle mit Suche und Sortierung, rechts die gewählte Karte.
const props = defineProps<{
  cards: Card[]
  decks: Deck[]
  log: CardLog[]
  deck: string | null
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

const lapsed = computed(() => lapses(props.log))
const TESTS: Record<Filter, (c: Card) => boolean> = {
  all: () => true,
  due: (c) => c.due,
  new: (c) => c.new,
  lapsed: (c) => (lapsed.value.get(c.key) ?? 0) >= 2,
  notes: (c) => !!c.note_path,
}
const FILTERS = computed<[Filter, string][]>(() => [
  ['all', t('Alle Karten')],
  ['due', t('Heute fällig')],
  ['new', t('Neu')],
  ['lapsed', t('Oft vergessen')],
  ['notes', t('Aus Notizen')],
])
const COLS = computed(
  () =>
    [
      ['question', t('Frage')],
      ['answer', t('Antwort')],
      ['deck', t('Fach')],
      ['due', t('Fällig')],
      ['interval', t('Stufe')],
    ] as const,
)

const deckCards = computed(() =>
  deck.value ? props.cards.filter((c) => c.deck === deck.value) : props.cards,
)
const filterCounts = computed<Record<Filter, number>>(() => {
  const counts: Record<Filter, number> = { all: 0, due: 0, new: 0, lapsed: 0, notes: 0 }
  for (const c of deckCards.value) {
    counts.all++
    if (c.due) counts.due++
    if (c.new) counts.new++
    if ((lapsed.value.get(c.key) ?? 0) >= 2) counts.lapsed++
    if (c.note_path) counts.notes++
  }
  return counts
})

// Sortierschlüssel für „Fällig“: neu zuerst, dann nach Datum.
const dueKey = (c: Card) => (c.new ? '' : c.due ? '0' : (c.due_date ?? ''))
const rows = computed(() => {
  const q = query.value.trim().toLowerCase()
  const out = deckCards.value.filter(
    (c) => TESTS[filter.value](c) && (!q || `${c.question}\n${c.answer}`.toLowerCase().includes(q)),
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
const sel = computed(() => props.cards.find((c) => c.key === selKey.value))

const dateFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }),
)
const dueText = (c: Card) =>
  c.new
    ? t('neu')
    : c.due
      ? t('heute')
      : c.due_date
        ? dateFmt.value.format(new Date(`${c.due_date}T00:00`))
        : '–'

async function saveCard(c: Card, body: { deck: string; question: string; answer: string }) {
  if (!c.id) return
  try {
    const saved = await updateCard(c.id, body)
    if (selKey.value === c.key) selKey.value = saved.key
    error.value = ''
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
async function removeCard(c: Card) {
  if (!c.id) return
  try {
    await deleteCard(c.id)
    if (selKey.value === c.key) selKey.value = null
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
  const allNew = sum('new_due')
  const allLearn = sum('learn_due')
  const allDue = sum('due')
  return [
    {
      name: '',
      label: t('Alle'),
      new_due: allNew,
      learn_due: allLearn,
      due: allDue,
      review: allDue - allNew - allLearn,
    },
    ...props.decks.map((d) => ({
      name: d.name,
      label: d.name,
      new_due: d.new_due,
      learn_due: d.learn_due,
      due: d.due,
      review: d.due - d.new_due - d.learn_due,
    })),
  ]
})
const current = computed(
  () => deckRows.value.find((d) => d.name === deck.value) ?? deckRows.value[0]!,
)

// Stufe 0–4 aus dem Abstand: neu, in Arbeit, jung, fast sicher, sitzt.
const stage = (c: Card) =>
  c.new ? 0 : c.learning ? 1 : c.interval_days < SETTLED_DAYS ? 2 : c.interval_days < 30 ? 3 : 4
const days = computed(() => perDay(props.log))
const inRow = computed(() => streak(days.value))
const acc30 = computed(() =>
  accuracy(props.log.filter((l) => Date.now() - Date.parse(l.at) < 30 * 864e5)),
)

const newDeck = ref('')
const addingDeck = ref(false)
const newDeckEl = ref<HTMLInputElement>()
function openAddDeck() {
  addingDeck.value = true
  void nextTick(() => newDeckEl.value?.focus())
}
function cancelAddDeck() {
  addingDeck.value = false
  newDeck.value = ''
}
async function addDeck() {
  const name = newDeck.value.trim()
  if (!name) return
  if (props.decks.some((d) => d.name === name)) {
    deck.value = name
    cancelAddDeck()
    return
  }
  const used = new Set(props.decks.map((d) => d.color))
  const color =
    PROJECT_COLORS.find(([c]) => !used.has(c))?.[0] ??
    PROJECT_COLORS[props.decks.length % PROJECT_COLORS.length]![0]
  try {
    await setDeckColor(name, color)
    cancelAddDeck()
    deck.value = name
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

useShortcuts((e) => {
  if (e.key === 'n') {
    e.preventDefault()
    emit('add', deck.value || null)
  } else if (e.key === 'l' && current.value.due) {
    emit('learn', deck.value || null)
  } else if (/^[1-9]$/.test(e.key)) {
    const d = props.decks[Number(e.key) - 1]
    if (d) deck.value = d.name
  } else if (e.key === 'Escape' && selKey.value) {
    selKey.value = null
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
  <div class="kb" :class="{ 'has-aside': !!sel }">
    <div class="kb-main">
      <h1 class="v-title sr-only">{{ $t('Lernkarten') }}</h1>

      <header class="kb-top">
        <div class="kb-bar">
          <div class="seg kb-tabs" role="tablist" :aria-label="$t('Fächer')">
            <button
              v-for="d in deckRows"
              :key="d.name"
              type="button"
              role="tab"
              :aria-selected="deck === d.name"
              :aria-label="`${d.label}: ${$t('{a} neu, {b} in Arbeit, {c} Wiederholung', { a: d.new_due, b: d.learn_due, c: d.review })}`"
              @click="deck = d.name"
            >
              <span class="kb-fname" :title="d.label">{{ d.label }}</span>
              <span v-if="d.due" class="count">{{ d.due }}</span>
            </button>
          </div>

          <form v-if="addingDeck" class="kb-newdeck" @submit.prevent="addDeck">
            <input
              ref="newDeckEl"
              v-model="newDeck"
              class="input kb-newdeck-in"
              :placeholder="$t('Neues Fach, z. B. Mathe 2')"
              :aria-label="$t('Neues Fach')"
              @keydown.esc="cancelAddDeck"
            />
            <button class="btn btn-primary btn-sm" type="submit" :disabled="!newDeck.trim()">
              {{ $t('Anlegen') }}
            </button>
            <button
              class="icon-btn"
              type="button"
              :aria-label="$t('Abbrechen')"
              @click="cancelAddDeck"
            >
              <svg class="ic"><use href="#i-x" /></svg>
            </button>
          </form>
          <button v-else type="button" class="kb-add-tab" @click="openAddDeck">
            {{ $t('+ Fach anlegen') }}
          </button>

          <span class="kb-spacer" />

          <div class="kb-actions">
            <select v-model="filter" class="input kb-filter" :aria-label="$t('Ansicht')">
              <option v-for="[f, label] in FILTERS" :key="f" :value="f">
                {{ label }} ({{ filterCounts[f] }})
              </option>
            </select>
            <SearchField v-model="query" :label="$t('Karten durchsuchen')" />
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
            <IconButton
              icon="plus"
              :label="$t('Hinzufügen')"
              shortcut="N"
              @click="emit('add', deck || null)"
            />
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!current.due"
              :title="current.due ? `${$t('Lernen')} (L)` : undefined"
              @click="emit('learn', deck || null)"
            >
              {{ current.due ? $t('{n} lernen', { n: current.due }) : $t('Nichts fällig') }}
            </button>
          </div>
        </div>

        <div class="kb-sub">
          <span>
            {{ $tn(deckCards.length, '{n} Karte', '{n} Karten')
            }}<template v-if="current.due">
              · {{ $t('{n} heute fällig', { n: current.due }) }}</template
            >
          </span>
          <span class="kb-stats">
            <span>{{ $tn(inRow, '{n} Tag in Folge', '{n} Tage in Folge') }}</span>
            <span v-if="acc30 !== null">· {{ $t('{n} % richtig', { n: acc30 }) }}</span>
          </span>
        </div>
      </header>

      <div v-if="error" class="badge kb-err" role="alert">{{ error }}</div>

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
          :aria-selected="c.key === sel?.key"
          class="kb-tr"
          :class="{ on: c.key === sel?.key }"
          @click="selKey = c.key === sel?.key ? null : c.key"
        >
          <span role="cell" class="kb-cell kb-q"><CardInlineText :text="c.question" /></span>
          <span role="cell" class="kb-cell kb-a"><CardInlineText :text="c.answer" /></span>
          <span role="cell" class="kb-cell kb-d">{{ c.deck }}</span>
          <span role="cell" class="kb-due" :class="{ due: c.due && !c.new, new: c.new }">{{
            dueText(c)
          }}</span>
          <span role="cell" class="kb-pips" :aria-label="$t('Stufe {n} von 4', { n: stage(c) })"
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
    </div>

    <CardBrowserAside
      v-if="sel"
      :card="sel"
      :decks="decks"
      :due-text="dueText(sel)"
      :stage="stage(sel)"
      :lapses-count="lapsed.get(sel.key) ?? 0"
      @close="selKey = null"
      @save="saveCard"
      @remove="removeCard"
    />
  </div>
</template>

<style scoped>
.kb {
  flex: 1;
  display: flex;
  margin: -8px -32px -64px;
  min-height: 100dvh;
}
.kb-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.kb-top {
  padding: 12px 24px 10px;
  border-bottom: 1px solid var(--br-subtle);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kb-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.kb-tabs {
  display: flex;
  align-items: center;
  overflow-x: auto;
  scrollbar-width: none;
  max-width: 100%;
}
.kb-tabs::-webkit-scrollbar {
  display: none;
}
.kb-tabs button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.kb-tabs button[aria-selected='true'] {
  background: var(--bg-3);
  color: var(--tx-primary);
}
.kb-tabs .count {
  font: 500 11.5px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--a-blue-tx);
}
.kb-fname {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.kb-add-tab {
  height: 30px;
  padding: 0 8px;
  border: 0;
  border-radius: var(--r-s);
  background: none;
  color: var(--tx-muted);
  font: 400 12.5px/1 var(--font-ui);
  cursor: pointer;
  white-space: nowrap;
}
.kb-add-tab:hover {
  color: var(--tx-primary);
  background: var(--bg-hover);
}
.kb-newdeck {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.kb-newdeck-in {
  height: 30px;
  width: 190px;
  font-size: 13px;
}
.kb-spacer {
  flex: 1;
}
.kb-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.kb-more {
  width: 34px;
  padding: 0;
  justify-content: center;
}
.kb-filter {
  height: 32px;
  width: auto;
  max-width: 210px;
  font-size: 13px;
}
.kb-sub {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.kb-stats {
  display: inline-flex;
  gap: 6px;
}
.kb-err {
  margin: 12px 24px 0;
}
.kb-table {
  flex: 1;
  padding: 4px 12px 24px;
}
.kb-tr {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) 150px 84px 64px;
  gap: 16px;
  align-items: center;
  width: 100%;
  min-height: 46px;
  padding: 0 12px;
  border: 0;
  border-bottom: 1px solid var(--br-subtle);
  border-radius: 0;
  background: none;
  color: var(--tx-primary);
  font: 400 13.5px/1.35 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.kb-tr:hover {
  background: var(--bg-hover);
}
.kb-tr.on {
  background: var(--bg-selected);
}
.kb-th {
  min-height: 36px;
  cursor: default;
  border-bottom-color: var(--br-default);
}
.kb-th:hover {
  background: none;
}
.kb-th button {
  padding: 0;
  border: 0;
  background: none;
  color: var(--tx-muted);
  font: 400 12px/1 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.kb-th button:hover,
.kb-th button.on {
  color: var(--tx-primary);
}
.kb-cell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kb-a {
  color: var(--tx-secondary);
}
.kb-d {
  color: var(--tx-secondary);
  font-size: 13px;
}
.kb-due {
  font-size: 12.5px;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.kb-due.due {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.kb-due.new {
  color: var(--tx-secondary);
}
.kb-pips {
  display: inline-flex;
  gap: 3px;
}
.kb-pips i {
  width: 10px;
  height: 3px;
  border-radius: 2px;
  background: var(--br-default);
}
.kb-pips i.on {
  background: var(--tx-secondary);
}
.kb-empty {
  margin: 0;
  padding: 40px 12px;
  font: 400 13.5px/1.5 var(--font-ui);
  color: var(--tx-muted);
}
.kb-empty button {
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}
.btn-sm {
  height: 28px;
  padding: 0 10px;
  font-size: 12.5px;
}

@media (max-width: 1200px) {
  .kb.has-aside .kb-tr {
    grid-template-columns: minmax(0, 1fr) 84px;
  }
  .kb.has-aside .kb-a,
  .kb.has-aside .kb-d,
  .kb.has-aside .kb-pips,
  .kb.has-aside .kb-th button:nth-child(2),
  .kb.has-aside .kb-th button:nth-child(3),
  .kb.has-aside .kb-th button:nth-child(5) {
    display: none;
  }
}
@media (max-width: 960px) {
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
  .kb-top {
    padding: 12px 12px 8px;
  }
  .kb-stats {
    display: none;
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
}
</style>
