<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  createCard, deleteCard, errorMessage, listCards, listDecks, reviewCard, setDeckColor, updateCard,
  type Card, type Deck,
} from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { PROJECT_COLORS } from '@/lib/projectColor'
import { store } from '@/lib/storage'
import { locale, t, tn } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

// Drei Ansichten: Übersicht der Fächer, ein Fach mit seinen Karten, Lernen (eine Runde).
// Zahlentasten: in der Übersicht 1–9 lernt das Fach mit dieser Nummer, beim Lernen bewerten 1–4.
const GRADES = [
  { label: t('Nochmal'), color: 'red' },
  { label: t('Schwer'), color: 'orange' },
  { label: t('Gut'), color: 'green' },
  { label: t('Leicht'), color: 'blue' },
]

const cards = ref<Card[]>([])
const decks = ref<Deck[]>([])
const error = ref('')
const loaded = ref(false)
const deck = ref<string | null>(null) // offenes Fach
const learning = ref(false)

const colorOf = (name: string) => `var(--p-${decks.value.find((d) => d.name === name)?.color ?? 'neutral'})`
const dueTotal = computed(() => decks.value.reduce((n, d) => n + d.due, 0))
const current = computed(() => decks.value.find((d) => d.name === deck.value))
const deckCards = computed(() => cards.value.filter((c) => c.deck === deck.value))

async function load() {
  try {
    ;[cards.value, decks.value] = await Promise.all([listCards(), listDecks()])
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)

// --- Lernen
const queue = ref<Card[]>([])
const roundSize = ref(0)
const done = ref(0)
const shown = ref(false)
const busy = ref(false)
const card = computed(() => queue.value[0])

function learn(name: string | null) {
  queue.value = cards.value.filter((c) => c.due && (name === null || c.deck === name))
  if (!queue.value.length) return
  deck.value = name
  roundSize.value = queue.value.length
  done.value = 0
  shown.value = false
  learning.value = true
}

// „Nochmal“ stellt die Karte ans Ende der Runde; sonst ist sie für heute erledigt.
async function grade(g: number) {
  const c = card.value
  if (!c || !shown.value || busy.value) return
  busy.value = true
  try {
    await reviewCard(c.key, g)
    queue.value.shift()
    if (g === 0) queue.value.push(c)
    else done.value++
    shown.value = false
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

async function stopLearning() {
  learning.value = false
  await load()
}

// --- Neue Karte
const qDeck = ref('')
const qQuestion = ref('')
const qAnswer = ref('')
const questionEl = ref<HTMLInputElement>()
const editing = ref<string | null>(null) // id der Karte, die oben im Formular bearbeitet wird

function startAdd() {
  qDeck.value = deck.value ?? store.get('kairo-cards-deck') ?? decks.value[0]?.name ?? ''
  void nextTick(() => questionEl.value?.focus())
}

async function saveCard() {
  const body = { deck: qDeck.value.trim(), question: qQuestion.value.trim(), answer: qAnswer.value.trim() }
  if (!body.deck || !body.question || !body.answer) return
  try {
    if (editing.value) await updateCard(editing.value, body)
    else await createCard(body)
    store.set('kairo-cards-deck', body.deck)
    qQuestion.value = qAnswer.value = ''
    editing.value = null
    await load()
    questionEl.value?.focus()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function edit(c: Card) {
  editing.value = c.id
  qDeck.value = c.deck
  qQuestion.value = c.question
  qAnswer.value = c.answer
  void nextTick(() => questionEl.value?.focus())
}
function cancelEdit() {
  editing.value = null
  qQuestion.value = qAnswer.value = ''
}

const remove = async (c: Card) => {
  try {
    await deleteCard(c.id!)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// --- Fächer
const newDeck = ref('')
async function addDeck() {
  const name = newDeck.value.trim()
  if (!name) return
  const used = new Set(decks.value.map((d) => d.color))
  const color = PROJECT_COLORS.find(([c]) => !used.has(c))?.[0] ?? PROJECT_COLORS[decks.value.length % PROJECT_COLORS.length]![0]
  try {
    await setDeckColor(name, color)
    newDeck.value = ''
    await load()
    openDeck(name)
  } catch (e) {
    error.value = errorMessage(e)
  }
}
async function recolor(color: string) {
  if (!deck.value) return
  try {
    await setDeckColor(deck.value, color)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
function openDeck(name: string) {
  deck.value = name
  cancelEdit()
  qDeck.value = name
}

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }))
function nextText(c: Card) {
  if (c.new) return t('neu')
  if (c.due) return t('heute')
  return dateFmt.value.format(new Date(c.due_date + 'T00:00'))
}

useShortcuts((e) => {
  if (learning.value) {
    if (!card.value) return
    if (!shown.value && (e.key === ' ' || e.key === 'Enter')) {
      e.preventDefault()
      shown.value = true
    } else if (shown.value && ['1', '2', '3', '4'].includes(e.key)) {
      e.preventDefault()
      void grade(Number(e.key) - 1)
    } else if (e.key === 'Escape') {
      void stopLearning()
    }
    return
  }
  if (e.key === 'n') {
    e.preventDefault()
    startAdd()
  } else if (e.key === 'Escape' && deck.value) {
    deck.value = null
  } else if (!deck.value && /^[1-9]$/.test(e.key)) {
    const d = decks.value[Number(e.key) - 1]
    if (d?.due) learn(d.name)
  } else if (deck.value && e.key === 'l') {
    learn(deck.value)
  }
})
</script>

<template>
  <div class="view-inner">
    <!-- Lernen -->
    <template v-if="learning">
      <button type="button" class="col-link cd-back" @click="stopLearning"><svg class="ic"><use href="#i-left" /></svg>{{ deck ?? $t('Alle Fächer') }}</button>
      <div class="v-head">
        <h1 class="v-title">{{ card ? $t('Lernen') : $t('Geschafft') }}</h1>
        <div class="v-sub">{{ $t('{n} von {m} gelernt', { n: done, m: roundSize }) }}</div>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>
      <div class="pbar cd-progress" aria-hidden="true"><div class="pbar-fill" :style="{ width: (done / roundSize) * 100 + '%' }"></div></div>

      <section v-if="card" class="card cd-card" :style="{ '--dc': colorOf(card.deck) }" :aria-label="$t('Karte')">
        <div class="cd-meta">
          <span class="cd-deck"><i></i>{{ card.deck }}</span>
          <span v-if="card.new" class="cd-new">{{ $t('Neu') }}</span>
          <router-link v-if="card.note_path" class="col-link cd-src" :to="{ path: '/notes', query: { open: card.note_path } }">{{ card.note_path }}</router-link>
        </div>
        <p class="cd-q">{{ card.question }}</p>
        <template v-if="shown">
          <p class="cd-a">{{ card.answer }}</p>
          <div class="cd-grades">
            <button v-for="(g, i) in GRADES" :key="i" type="button" class="cd-grade" :style="{ '--gc': `var(--p-${g.color})` }" :disabled="busy" @click="grade(i)">
              <b>{{ i + 1 }}</b>{{ g.label }}
            </button>
          </div>
        </template>
        <button v-else type="button" class="btn btn-primary cd-show" @click="shown = true">{{ $t('Antwort zeigen') }}<kbd>{{ $t('Leertaste') }}</kbd></button>
      </section>
      <div v-else class="card cd-finish">
        <p class="cd-finish-t">{{ $t('Fertig für heute. Morgen geht es weiter.') }}</p>
        <button class="btn btn-secondary" type="button" @click="stopLearning">{{ $t('Zur Übersicht') }}</button>
      </div>
    </template>

    <template v-else>
      <!-- Kopf: Übersicht oder Fach -->
      <template v-if="deck">
        <button type="button" class="col-link cd-back" @click="deck = null"><svg class="ic"><use href="#i-left" /></svg>{{ $t('Alle Fächer') }}</button>
        <div class="v-head v-head-row">
          <div>
            <h1 class="v-title cd-title" :style="{ '--dc': colorOf(deck) }"><i></i>{{ deck }}</h1>
            <div class="v-sub">{{ $tn(current?.total ?? 0, '{n} Karte', '{n} Karten') }} · {{ $t('{n} fällig', { n: current?.due ?? 0 }) }}</div>
          </div>
          <button class="btn btn-primary" type="button" :disabled="!current?.due" aria-keyshortcuts="l" @click="learn(deck)">{{ $t('Lernen') }}<span v-if="current?.due" class="cd-count">{{ current.due }}</span></button>
        </div>
        <div class="cd-swatches" role="group" :aria-label="$t('Farbe')">
          <button v-for="[c, name] in PROJECT_COLORS" :key="c" type="button" class="cd-swatch" :style="{ '--sc': `var(--p-${c})` }"
            :aria-label="$t(name)" :aria-pressed="current?.color === c" @click="recolor(c)"></button>
        </div>
      </template>
      <div v-else class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ $t('Lernkarten') }}</h1>
          <div class="v-sub">{{ dueTotal ? tn(dueTotal, '{n} Karte fällig', '{n} Karten fällig') : $t('Nichts fällig') }} · {{ $tn(decks.length, '{n} Fach', '{n} Fächer') }}</div>
        </div>
        <button class="btn btn-primary" type="button" :disabled="!dueTotal" @click="learn(null)">{{ $t('Alle lernen') }}<span v-if="dueTotal" class="cd-count">{{ dueTotal }}</span></button>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <!-- Schnelleingabe -->
      <form class="card cd-add" :class="{ 'is-edit': editing }" @submit.prevent="saveCard" @keydown.esc="cancelEdit">
        <div class="cd-add-row">
          <input v-model="qDeck" class="input cd-add-deck" list="cd-decks" :placeholder="$t('Fach')" :aria-label="$t('Fach')" required
            :style="{ '--dc': colorOf(qDeck) }" />
          <datalist id="cd-decks"><option v-for="d in decks" :key="d.name" :value="d.name" /></datalist>
          <input ref="questionEl" v-model="qQuestion" class="input" :placeholder="$t('Frage')" :aria-label="$t('Frage')" aria-keyshortcuts="n" required @focus="!qDeck && startAdd()" />
          <input v-model="qAnswer" class="input" :placeholder="$t('Antwort')" :aria-label="$t('Antwort')" required />
          <button class="btn btn-primary" type="submit">{{ editing ? $t('Speichern') : $t('Hinzufügen') }}</button>
          <button v-if="editing" class="btn btn-ghost" type="button" @click="cancelEdit">{{ $t('Abbrechen') }}</button>
        </div>
        <div class="cd-hint">{{ $t('Enter fügt hinzu und springt zur nächsten Frage. In Notizen geht auch eine Zeile „Frage :: Antwort“; das Fach ist dann der Ordner.') }}</div>
      </form>

      <!-- Fächer -->
      <div v-if="!deck" class="cd-decks">
        <div v-for="(d, i) in decks" :key="d.name" class="cd-tile" :style="{ '--dc': `var(--p-${d.color})` }">
          <button type="button" class="cd-tile-open" @click="openDeck(d.name)">
            <span class="cd-tile-top"><span class="cd-tile-name">{{ d.name }}</span><kbd v-if="i < 9 && d.due">{{ i + 1 }}</kbd></span>
            <span class="cd-tile-num"><b>{{ d.due }}</b>{{ $t('fällig') }}</span>
            <span class="cd-tile-sub">{{ $tn(d.total, '{n} Karte', '{n} Karten') }}<template v-if="d.new_due"> · {{ $t('{n} neu', { n: d.new_due }) }}</template></span>
            <span class="cd-tile-bar" aria-hidden="true"><i :style="{ width: d.total ? ((d.total - d.due) / d.total) * 100 + '%' : '0' }"></i></span>
          </button>
          <button v-if="d.due" type="button" class="cd-tile-learn" @click="learn(d.name)">{{ $t('Lernen') }}</button>
        </div>
        <form class="cd-tile cd-tile-new" @submit.prevent="addDeck">
          <input v-model="newDeck" class="input" :placeholder="$t('Neues Fach, z. B. Mathe 2')" :aria-label="$t('Neues Fach')" />
          <button class="btn btn-secondary" type="submit" :disabled="!newDeck.trim()">{{ $t('Fach anlegen') }}</button>
        </form>
      </div>
      <p v-if="loaded && !decks.length && !error" class="v-sub cd-empty">{{ $t('Noch keine Karten. Leg oben ein Fach an und schreib die erste Frage.') }}</p>

      <!-- Karten eines Fachs -->
      <div v-if="deck" class="card cd-list">
        <div v-for="c in deckCards" :key="c.key" class="cd-row">
          <span class="cd-row-q">{{ c.question }}</span>
          <span class="cd-row-a">{{ c.answer }}</span>
          <span class="cd-row-next" :class="{ 'is-due': c.due }">{{ nextText(c) }}</span>
          <span class="cd-row-act">
            <template v-if="c.id">
              <button type="button" class="btn btn-ghost" @click="edit(c)">{{ $t('Bearbeiten') }}</button>
              <DeleteButton ghost :text="$t('Karte löschen?')" @confirm="remove(c)" />
            </template>
            <router-link v-else class="col-link" :to="{ path: '/notes', query: { open: c.note_path } }" :title="c.note_path ?? ''">{{ $t('In Notiz') }}</router-link>
          </span>
        </div>
        <p v-if="!deckCards.length" class="v-sub cd-row">{{ $t('Noch keine Karten in diesem Fach.') }}</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.cd-back { display: inline-flex; align-items: center; gap: 4px; margin-bottom: 12px; padding: 0; border: 0; background: none; cursor: pointer; }
.cd-count { margin-left: 8px; padding: 1px 7px; border-radius: 999px; font: 600 12px/1.4 var(--font-ui); background: color-mix(in srgb, currentColor 20%, transparent); }
.cd-title { display: flex; align-items: center; gap: 12px; }
.cd-title i { width: 14px; height: 14px; border-radius: 4px; background: var(--dc); }

/* Schnelleingabe */
.cd-add { display: grid; gap: 8px; padding: 14px; margin-bottom: 24px; }
.cd-add.is-edit { border-color: var(--a-blue); }
.cd-add-row { display: grid; grid-template-columns: 180px minmax(0, 1fr) minmax(0, 1fr) auto auto; gap: 8px; }
.cd-add-deck { border-left: 4px solid var(--dc); }
.cd-hint { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }

/* Fächer */
.cd-decks { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 14px; }
.cd-tile {
  position: relative; display: grid; border-radius: var(--r-m); overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--dc) 35%, var(--br-default));
  background: linear-gradient(160deg, color-mix(in srgb, var(--dc) 22%, var(--bg-1)), var(--bg-1) 70%);
}
.cd-tile-open { display: grid; gap: 6px; padding: 16px 16px 14px; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.cd-tile-open:hover .cd-tile-name { text-decoration: underline; }
.cd-tile-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.cd-tile-name { font: 600 15px/1.3 var(--font-ui); color: var(--dc); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cd-tile-top kbd { font: 500 11px/1 var(--font-mono); padding: 3px 6px; border-radius: 4px; color: var(--tx-muted); background: var(--bg-2); }
.cd-tile-num { display: flex; align-items: baseline; gap: 6px; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.cd-tile-num b { font: 600 34px/1 var(--font-ui); letter-spacing: -0.02em; color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.cd-tile-sub { font: 400 12px/1.3 var(--font-ui); color: var(--tx-secondary); }
.cd-tile-bar { display: block; height: 4px; margin-top: 6px; border-radius: 2px; background: color-mix(in srgb, var(--dc) 18%, transparent); }
.cd-tile-bar i { display: block; height: 100%; border-radius: inherit; background: var(--dc); }
.cd-tile-learn {
  position: absolute; right: 12px; bottom: 22px; padding: 6px 12px; border: 0; border-radius: 999px; cursor: pointer;
  font: 600 12px/1 var(--font-ui); color: var(--bg-0); background: var(--dc);
}
.cd-tile-new { --dc: var(--tx-muted); align-content: center; gap: 8px; padding: 16px; border-style: dashed; background: none; }
.cd-empty { margin-top: 16px; }

/* Fach: Karten */
.cd-swatches { display: flex; gap: 8px; margin: -8px 0 20px; }
.cd-swatch { width: 20px; height: 20px; border-radius: 50%; border: 2px solid transparent; background: var(--sc); cursor: pointer; }
.cd-swatch[aria-pressed='true'] { box-shadow: 0 0 0 2px var(--bg-0), 0 0 0 4px var(--sc); }
.cd-list { padding: 0; overflow: hidden; }
.cd-row { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr) 70px auto; gap: 16px; align-items: center; padding: 10px 16px; border-top: 1px solid var(--br-default); margin: 0; }
.cd-row:first-child { border-top: 0; }
.cd-row-q { font: 500 14px/1.4 var(--font-ui); color: var(--tx-primary); }
.cd-row-a { font: 400 13px/1.4 var(--font-ui); color: var(--tx-secondary); }
.cd-row-next { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.cd-row-next.is-due { color: var(--a-blue); font-weight: 600; }
.cd-row-act { display: flex; justify-content: flex-end; gap: 4px; }

/* Lernen */
.cd-progress { max-width: 680px; margin-bottom: 20px; }
.cd-card {
  display: grid; gap: 18px; max-width: 680px; padding: 28px; border-top: 4px solid var(--dc);
  background: linear-gradient(180deg, color-mix(in srgb, var(--dc) 10%, var(--bg-1)), var(--bg-1) 40%);
}
.cd-meta { display: flex; align-items: center; gap: 10px; min-width: 0; }
.cd-deck { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; font: 600 12px/1 var(--font-ui); color: var(--dc); background: color-mix(in srgb, var(--dc) 15%, transparent); }
.cd-deck i { width: 7px; height: 7px; border-radius: 50%; background: var(--dc); }
.cd-new { font: 600 11px/1 var(--font-ui); text-transform: uppercase; letter-spacing: 0.06em; color: var(--a-blue); }
.cd-src { min-width: 0; margin-left: auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cd-q { margin: 0; font: 600 22px/1.4 var(--font-ui); color: var(--tx-primary); white-space: pre-wrap; }
.cd-a { margin: 0; padding: 14px 16px; border-radius: var(--r-s); font: 400 16px/1.5 var(--font-ui); color: var(--tx-primary); background: var(--bg-2); white-space: pre-wrap; }
.cd-show { justify-self: start; }
.cd-show kbd { margin-left: 8px; font: 400 11px/1 var(--font-mono); opacity: 0.6; }
.cd-grades { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.cd-grade {
  display: grid; justify-items: center; gap: 4px; padding: 10px 6px; border-radius: var(--r-s); cursor: pointer;
  border: 1px solid color-mix(in srgb, var(--gc) 40%, transparent); background: color-mix(in srgb, var(--gc) 12%, transparent);
  color: var(--tx-primary); font: 500 13px/1 var(--font-ui);
}
.cd-grade b { font: 600 20px/1 var(--font-ui); color: var(--gc); }
.cd-grade:hover:not(:disabled) { background: color-mix(in srgb, var(--gc) 24%, transparent); }
.cd-finish { display: grid; justify-items: start; gap: 12px; max-width: 680px; padding: 24px; }
.cd-finish-t { margin: 0; font: 600 16px/1.4 var(--font-ui); color: var(--tx-primary); }

@media (max-width: 760px) {
  .cd-add-row { grid-template-columns: 1fr; }
  .cd-row { grid-template-columns: 1fr auto; gap: 4px 12px; }
  .cd-row-a { grid-column: 1; }
  .cd-row-next { grid-row: 1; grid-column: 2; }
  .cd-row-act { grid-column: 1 / -1; justify-content: flex-start; }
  .cd-tile-top kbd { display: none; }
  .cd-card { padding: 20px; }
  .cd-q { font-size: 19px; }
}
</style>
