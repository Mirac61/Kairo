<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { cardLog, errorMessage, listCards, listDecks, setDeckColor, type Card, type CardLog, type Deck } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { accuracy, forecast, lapses, perDay, shuffled, streak } from '@/lib/cards'
import { addDays, ymd } from '@/lib/dates'
import { locale, t, tn } from '@/lib/i18n'
import { PROJECT_COLORS } from '@/lib/projectColor'
import CardAddDialog from '@/components/cards/CardAddDialog.vue'
import CardBrowser from '@/components/cards/CardBrowser.vue'
import CardDeck from '@/components/cards/CardDeck.vue'
import CardLearn from '@/components/cards/CardLearn.vue'

// Lernkarten wie Anki: Fächerliste (diese Datei), ein Fach, Kartenbrowser, Lernrunde. Zahlentasten 1–9 lernen ein Fach.
const cards = ref<Card[]>([])
const decks = ref<Deck[]>([])
const log = ref<CardLog[]>([])
const error = ref('')
const loaded = ref(false)
const mode = ref<'decks' | 'deck' | 'browse' | 'learn'>('decks')
const deckName = ref<string | null>(null)
const adding = ref(false)

async function load() {
  try {
    ;[cards.value, decks.value, log.value] = await Promise.all([listCards(), listDecks(), cardLog()])
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)

const colorOf = (name: string) => `var(--p-${decks.value.find((d) => d.name === name)?.color ?? 'neutral'})`
const current = computed(() => decks.value.find((d) => d.name === deckName.value))
const cardsOf = (name: string) => cards.value.filter((c) => c.deck === name)
const dueTotal = computed(() => decks.value.reduce((n, d) => n + d.due, 0))

function openDeck(name: string) {
  deckName.value = name
  mode.value = 'deck'
}
function back() {
  mode.value = deckName.value && current.value ? 'deck' : 'decks'
}

// --- Lernrunde: erst Karten in Arbeit, dann fällige Wiederholungen, dann neue. Übungsrunden sind gemischt.
const round = ref<{ id: number; cards: Card[]; practice: boolean; title: string | null }>({ id: 0, cards: [], practice: false, title: null })
const order = (c: Card) => (c.learning ? 0 : c.new ? 2 : 1)
function learn(name: string | null, practice = false, keys?: string[]) {
  const pool = name === null ? cards.value : cardsOf(name)
  const list = keys ? shuffled(cards.value.filter((c) => keys.includes(c.key)))
    : practice ? shuffled(pool)
      : pool.filter((c) => c.due).sort((a, b) => order(a) - order(b))
  if (!list.length) return
  if (name !== null) deckName.value = name
  round.value = { id: round.value.id + 1, cards: list, practice: practice || !!keys, title: name }
  mode.value = 'learn'
}
function stopLearning() {
  back()
  void load()
}

// --- Kennzahlen und Verlauf
const days = computed(() => perDay(log.value))
const todayN = computed(() => days.value.get(ymd(new Date())) ?? 0)
const inRow = computed(() => streak(days.value))
const acc30 = computed(() => accuracy(log.value.filter((l) => Date.now() - Date.parse(l.at) < 30 * 864e5)))
const ahead = computed(() => forecast(cards.value))
const aheadMax = computed(() => Math.max(1, ...ahead.value))
const aheadLabels = computed(() => Array.from({ length: 7 }, (_, i) => addDays(new Date(), i).toLocaleDateString(locale.value, { weekday: 'narrow' })))
const forgotten = computed(() => [...lapses(log.value)].sort((a, b) => b[1] - a[1])
  .map(([key, n]) => ({ card: cards.value.find((c) => c.key === key), n })).filter((x) => x.card && x.n >= 2).slice(0, 3))
const recent = computed(() => cards.value.filter((c) => c.created_at).sort((a, b) => b.created_at!.localeCompare(a.created_at!)).slice(0, 4))
const weekAdded = computed(() => cards.value.filter((c) => c.created_at && Date.now() - Date.parse(c.created_at) < 7 * 864e5).length)

const WEEKS = 53
// Heatmap: ein Jahr, Spalten ab Montag, Stufe nach Anzahl der Bewertungen am Tag.
const heat = computed(() => {
  const today = new Date()
  const start = addDays(today, -((today.getDay() + 6) % 7) - (WEEKS - 1) * 7)
  return Array.from({ length: WEEKS * 7 }, (_, i) => {
    const d = addDays(start, i)
    const n = days.value.get(ymd(d)) ?? 0
    return { n, d: ymd(d), lvl: d > today ? -1 : n === 0 ? 0 : n < 6 ? 1 : n < 16 ? 2 : n < 31 ? 3 : 4, today: ymd(d) === ymd(today) }
  })
})
const heatTotal = computed(() => heat.value.reduce((a, h) => a + h.n, 0))
const bestRun = computed(() => {
  let best = 0, run = 0
  for (const h of heat.value) best = Math.max(best, (run = h.n ? run + 1 : 0))
  return best
})

// --- Fachzeilen
const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const relDay = (iso: string) => {
  const d = ymd(new Date(iso))
  return d === ymd(new Date()) ? t('heute') : d === ymd(addDays(new Date(), -1)) ? t('gestern') : dateFmt.value.format(new Date(iso))
}
const rows = computed(() => decks.value.map((d) => {
  const cs = cardsOf(d.name)
  const keys = new Set(cs.map((c) => c.key))
  const last = log.value.filter((l) => keys.has(l.key)).at(-1)
  const next = cs.filter((c) => !c.due && c.due_date).map((c) => c.due_date!).sort()[0]
  const sub = !cs.length ? '' : last && d.due ? t('zuletzt {when}', { when: relDay(last.at) })
    : next ? t('nächste {when}', { when: dateFmt.value.format(new Date(next + 'T00:00')) }) : ''
  return { ...d, review: d.due - d.new_due - d.learn_due, sub }
}))

const newDeck = ref('')
const addingDeck = ref(false)
async function addDeck() {
  const name = newDeck.value.trim()
  if (!name) return
  const used = new Set(decks.value.map((d) => d.color))
  const color = PROJECT_COLORS.find(([c]) => !used.has(c))?.[0] ?? PROJECT_COLORS[decks.value.length % PROJECT_COLORS.length]![0]
  try {
    await setDeckColor(name, color)
    newDeck.value = ''
    addingDeck.value = false
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function onDeckChanged(name: string | null) {
  deckName.value = name
  if (!name) mode.value = 'decks'
  void load()
}

useShortcuts((e) => {
  if (mode.value !== 'decks') return
  if (e.key === 'n') {
    e.preventDefault()
    adding.value = true
  } else if (e.key === 'b') {
    mode.value = 'browse'
  } else if (/^[1-9]$/.test(e.key)) {
    const d = decks.value[Number(e.key) - 1]
    if (d?.due) learn(d.name)
  }
})
</script>

<template>
  <div class="view-inner cv-page">
    <CardLearn v-if="mode === 'learn'" :key="round.id" :round="round.cards" :title="round.title" :practice="round.practice"
      :cards="cards" :decks="decks" :log="log" :color-of="colorOf"
      @exit="stopLearning" @finished="load" @again="(keys) => learn(round.title, true, keys)" />

    <CardBrowser v-else-if="mode === 'browse'" :cards="cards" :decks="decks" :log="log" :deck="deckName" :color-of="colorOf"
      @back="back" @changed="load" @add="adding = true" />

    <CardDeck v-else-if="mode === 'deck' && current" :deck="current" :cards="cardsOf(current.name)" :log="log" :color-of="colorOf"
      @back="mode = 'decks'; deckName = null" @learn="(p) => learn(current!.name, p)" @browse="mode = 'browse'" @add="adding = true" @changed="onDeckChanged" />

    <div v-else class="cv-over">
      <div class="v-head v-head-row cv-head">
        <div>
          <div class="v-sub cv-sub">{{ dueTotal ? tn(dueTotal, '{n} Karte wartet', '{n} Karten warten') : $t('Nichts fällig') }}</div>
          <h1 class="v-title">{{ $t('Lernkarten') }}</h1>
        </div>
        <div class="cv-tools">
          <button type="button" class="btn btn-secondary" @click="mode = 'browse'; deckName = null">{{ $t('Kartenbrowser') }}<kbd>B</kbd></button>
          <button type="button" class="btn btn-secondary" @click="adding = true">{{ $t('Hinzufügen') }}<kbd>N</kbd></button>
          <button type="button" class="btn btn-primary" :disabled="!dueTotal" @click="learn(null)">{{ $t('Alle lernen') }}<span v-if="dueTotal" class="cv-n">{{ dueTotal }}</span></button>
        </div>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <p class="cv-stats" :aria-label="$t('Kennzahlen')">
        <span><b class="mono">{{ todayN }}</b>/{{ todayN + dueTotal }} {{ $t('heute') }}</span>
        <span><b class="mono">{{ inRow }}</b> {{ inRow === 1 ? $t('Tag') : $t('Tage') }} {{ $t('in Folge gelernt') }} ({{ $t('Rekord {n}', { n: bestRun }) }})</span>
        <span><b class="mono">{{ acc30 === null ? '–' : acc30 + ' %' }}</b> {{ $t('richtig (30 Tage)') }}</span>
        <span><b class="mono">{{ cards.length }}</b> {{ $t('Karten') }} · {{ $tn(decks.length, '{n} Fach', '{n} Fächer') }}</span>
        <span><b class="mono">{{ ahead[1] ?? 0 }}</b> {{ $t('morgen fällig') }}</span>
      </p>

      <div class="cv-cols">
        <div class="cv-col">
        <section class="cv-decks" :aria-label="$t('Fächer')">
          <div class="cv-row cv-th"><span>{{ $t('Fach') }}</span><span>{{ $t('Neu') }}</span><span>{{ $t('Lernen') }}</span><span>{{ $t('Fällig') }}</span></div>
          <div v-for="(d, i) in rows" :key="d.name" class="cv-row">
            <div class="cv-name">
              <button type="button" class="cv-open" @click="openDeck(d.name)">{{ d.name }}<kbd v-if="i < 9 && d.due">{{ i + 1 }}</kbd></button>
              <span class="cv-meta">
                <template v-if="d.total">{{ $tn(d.total, '{n} Karte', '{n} Karten') }}<template v-if="d.sub"> · {{ d.sub }}</template></template>
                <template v-else>{{ $t('leer') }} · <button type="button" class="col-link" @click="deckName = d.name; adding = true">{{ $t('erste Karte') }}</button></template>
              </span>
            </div>
            <span class="mono cv-num" :class="{ z: !d.new_due }">{{ d.total ? d.new_due : '–' }}</span>
            <span class="mono cv-num" :class="{ z: !d.learn_due }">{{ d.total ? d.learn_due : '–' }}</span>
            <span class="mono cv-num" :class="d.review ? 'c-due' : 'z'">{{ d.total ? d.review : '–' }}</span>
          </div>
          <form v-if="addingDeck" class="cv-row cv-newdeck" @submit.prevent="addDeck">
            <input v-model="newDeck" class="input" autofocus :placeholder="$t('Neues Fach, z. B. Mathe 2')" :aria-label="$t('Neues Fach')" @keydown.esc="addingDeck = false" />
            <button class="btn btn-primary" type="submit" :disabled="!newDeck.trim()">{{ $t('Fach anlegen') }}</button>
            <button class="btn btn-ghost" type="button" @click="addingDeck = false">{{ $t('Abbrechen') }}</button>
          </form>
          <div class="cv-foot">
            <button type="button" class="col-link" @click="addingDeck = true">{{ $t('+ Fach anlegen') }}</button>
            <span>{{ $t('Zahl drücken lernt das Fach') }}</span>
          </div>
          <p v-if="loaded && !decks.length && !error" class="cv-empty">{{ $t('Noch keine Karten. Leg ein Fach an oder füg eine Liste aus Anki oder Quizlet ein.') }}</p>
        </section>
        </div>

        <aside class="cv-col">
        <section class="cv-sec" :aria-label="$t('Fällig · nächste 7 Tage')">
          <div class="cv-ph"><span>{{ $t('Fällig · nächste 7 Tage') }}</span><span class="mono">{{ ahead.reduce((a, b) => a + b, 0) }}</span></div>
          <div class="cv-bars" role="img" :aria-label="ahead.join(', ')">
            <i v-for="(n, i) in ahead" :key="i" :class="{ on: i === 0 }" :style="{ height: Math.max(3, (n / aheadMax) * 100) + '%' }"></i>
          </div>
          <div class="cv-x mono"><span v-for="(d, i) in aheadLabels" :key="i" :class="{ on: i === 0 }">{{ d }}</span></div>
        </section>

        <div class="cv-lists">
          <section class="cv-sec" :aria-label="$t('Oft vergessen')">
            <div class="cv-ph"><span>{{ $t('Oft vergessen') }}</span><button v-if="forgotten.length" type="button" class="col-link" @click="mode = 'browse'; deckName = null">{{ $t('Alle ›') }}</button></div>
            <div v-for="f in forgotten" :key="f.card!.key" class="cv-li"><span>{{ f.card!.question }}</span><small class="mono">{{ f.n }}×</small></div>
            <p v-if="!forgotten.length" class="cv-li-empty">{{ $t('Noch nichts – Karten, die du öfter mit „Nochmal“ bewertest, landen hier.') }}</p>
            <button v-else type="button" class="btn btn-secondary cv-li-btn" @click="learn(null, true, forgotten.map((f) => f.card!.key))">{{ $t('Diese üben') }}</button>
          </section>

          <section class="cv-sec" :aria-label="$t('Zuletzt hinzugefügt')">
            <div class="cv-ph"><span>{{ $t('Zuletzt hinzugefügt') }}</span><span class="mono">{{ $t('diese Woche {n}', { n: weekAdded }) }}</span></div>
            <div v-for="c in recent" :key="c.key" class="cv-li"><span>{{ c.question }}</span><small>{{ relDay(c.created_at!) }}</small></div>
            <p v-if="!recent.length" class="cv-li-empty">{{ $t('Noch keine Karten in Kairo angelegt.') }}</p>
          </section>
        </div>
        </aside>
      </div>

      <section class="cv-sec" :aria-label="$t('Aktivität')">
        <div class="cv-ph"><span>{{ $t('Aktivität · letzte 12 Monate') }}</span><span class="mono">{{ $tn(heatTotal, '{n} Wiederholung', '{n} Wiederholungen') }}</span></div>
        <div class="cv-heat" role="img" :aria-label="$t('Lernaktivität pro Tag, heute {n}', { n: todayN })">
          <i v-for="h in heat" :key="h.d" :class="[`l${h.lvl}`, { today: h.today }]" :title="`${h.d}: ${h.n}`"></i>
        </div>
        <div class="cv-heat-foot">
          <span>{{ $t('Längste Serie {n} Tage', { n: bestRun }) }}</span>
          <span class="cv-legend">{{ $t('weniger') }}<i class="l0"></i><i class="l1"></i><i class="l2"></i><i class="l3"></i><i class="l4"></i>{{ $t('mehr') }}</span>
        </div>
      </section>
    </div>

    <CardAddDialog v-if="adding" :decks="decks" :deck="deckName" @close="adding = false" @added="load" />
  </div>
</template>

<style scoped>
/* Ruhige Übersicht: eine Spalte, keine Kacheln, nur die Akzentfarbe für Fälliges und Aktivität. */
.cv-page { max-width: none; min-height: 100dvh; box-sizing: border-box; }
.cv-over { display: flex; flex-direction: column; gap: 36px; }
.cv-cols { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); gap: 56px; align-items: start; }
.cv-col { display: flex; flex-direction: column; gap: 36px; min-width: 0; }
kbd { margin-left: 8px; font: 500 10.5px/1 var(--font-mono); padding: 2px 5px; border-radius: 4px; border: 1px solid var(--br-default); color: var(--tx-muted); }
.cv-head { margin-bottom: -20px; }
.cv-sub { margin: 0 0 4px; }
.cv-tools { display: flex; gap: 8px; flex-wrap: wrap; }
.cv-n { margin-left: 8px; font: 500 12px/1 var(--font-mono); opacity: 0.7; }
.c-due { color: var(--a-blue-tx); }
.z { color: var(--br-strong); }

.cv-stats { display: flex; flex-wrap: wrap; gap: 6px 22px; margin: 0; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.cv-stats b { font-weight: 500; font-size: 14px; color: var(--tx-primary); }

.cv-row { display: grid; grid-template-columns: minmax(0, 1fr) 64px 64px 64px; align-items: center; gap: 8px; min-height: 58px; padding: 0 10px; margin: 0 -10px; border-top: 1px solid var(--br-subtle); border-radius: 6px; }
.cv-row:hover:not(.cv-th) { background: var(--bg-hover); }
.cv-th { min-height: 32px; border-top: 0; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.cv-th span:nth-child(n + 2) { text-align: right; }
.cv-name { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.cv-open { display: flex; align-items: center; min-width: 0; padding: 0; border: 0; background: none; color: var(--tx-primary); font: 500 15px/1.3 var(--font-ui); text-align: left; cursor: pointer; }
@media (pointer: coarse) { .cv-open { padding-block: 10px; margin-block: -10px; } } /* Trefferfläche am Handy, ohne die Zeile zu verschieben */
.cv-open:hover { text-decoration: underline; text-underline-offset: 3px; }
.cv-meta { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
.cv-meta button { padding: 0; border: 0; background: none; font: inherit; cursor: pointer; }
.cv-num { text-align: right; font-size: 14px; color: var(--tx-secondary); }
.cv-num.c-due { color: var(--a-blue-tx); }
.cv-num.z { color: var(--tx-muted); }
.cv-newdeck { grid-template-columns: minmax(0, 1fr) auto auto; }
.cv-newdeck:hover { background: none; }
.cv-foot { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; padding-top: 6px; border-top: 1px solid var(--br-subtle); font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
.cv-foot button { padding: 6px 0; border: 0; background: none; font-size: 13.5px; cursor: pointer; }
.cv-empty { margin: 0; padding: 14px 0; font: 400 13.5px/1.4 var(--font-ui); color: var(--tx-muted); }

.cv-sec { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.cv-ph { display: flex; justify-content: space-between; align-items: center; font: 400 12.5px/1 var(--font-ui); color: var(--tx-muted); }
.cv-ph button { padding: 0; border: 0; background: none; font-size: 12.5px; cursor: pointer; }
.cv-heat { display: grid; grid-auto-flow: column; grid-template-rows: repeat(7, auto); grid-auto-columns: minmax(0, 1fr); gap: 3px; }
.cv-heat i { aspect-ratio: 1; border-radius: 3px; }
.cv-legend i { width: 11px; height: 11px; border-radius: 3px; }
.l-1 { background: none; }
.l0 { background: var(--bg-2); }
.l1 { background: color-mix(in srgb, var(--a-blue) 30%, var(--bg-2)); }
.l2 { background: color-mix(in srgb, var(--a-blue) 55%, var(--bg-2)); }
.l3 { background: color-mix(in srgb, var(--a-blue) 80%, var(--bg-2)); }
.l4 { background: var(--a-blue); }
.cv-heat i.today { box-shadow: 0 0 0 1.5px var(--tx-primary); }
.cv-heat-foot { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.cv-legend { display: flex; align-items: center; gap: 4px; }
.cv-bars { display: flex; gap: 8px; align-items: flex-end; height: 44px; }
.cv-bars i { flex: 1; border-radius: 3px; background: var(--bg-3); }
.cv-bars i.on { background: var(--a-blue); }
.cv-x { display: flex; justify-content: space-between; margin-top: -4px; font-size: 11px; color: var(--tx-muted); }
.cv-x span { flex: 1; text-align: center; }
.cv-x .on { color: var(--tx-primary); }
.cv-lists { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 36px; }
/* Die Aside-Überschrift steht auf Höhe des Fächer-Tabellenkopfs. */
.cv-col > .cv-sec:first-child .cv-ph { min-height: 32px; }
.cv-li { display: flex; align-items: center; gap: 10px; padding: 9px 0; border-top: 1px solid var(--br-subtle); font: 400 13.5px/1.3 var(--font-ui); }
.cv-ph + .cv-li { margin-top: -4px; }
.cv-li + .cv-li { margin-top: -12px; }
.cv-li span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cv-li small { font-size: 12px; color: var(--tx-muted); }
.cv-li-empty { margin: 0; font: 400 12.5px/1.5 var(--font-ui); color: var(--tx-muted); }
.cv-li-btn { justify-content: center; align-self: flex-start; }

@media (max-width: 1100px) {
  .cv-cols { grid-template-columns: 1fr; gap: 36px; }
}
@media (max-width: 760px) {
  .cv-page { min-height: 0; }
  .cv-over { gap: 28px; }
  .cv-tools kbd, .cv-open kbd { display: none; }
  .cv-tools .btn-primary { flex: 1; }
  .cv-row { grid-template-columns: minmax(0, 1fr) 36px 36px 36px; }
  .cv-newdeck { grid-template-columns: 1fr; padding: 12px 10px; }
  .cv-cols, .cv-col { gap: 28px; }
  .cv-lists { grid-template-columns: 1fr; gap: 28px; }
  .cv-heat { gap: 2px; }
  .cv-heat i { border-radius: 2px; }
}
</style>
