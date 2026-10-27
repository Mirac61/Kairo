<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { deleteDeck, errorMessage, renameDeck, setDeck, setDeckColor, type Card, type CardLog, type Deck } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { accuracy, forecast, intervalText, stageOf } from '@/lib/cards'
import { addDays, ymd } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { PROJECT_COLORS } from '@/lib/projectColor'
import DeleteButton from '@/components/DeleteButton.vue'

// Ein Fach wie Ankis Deck-Bildschirm: oben Neu/Lernen/Fällig und „Jetzt lernen“, darunter die Karten und Zahlen.
const props = defineProps<{ deck: Deck; cards: Card[]; log: CardLog[]; colorOf: (deck: string) => string }>()
const emit = defineEmits<{ back: []; learn: [practice: boolean]; browse: []; add: []; changed: [name: string | null] }>()

const error = ref('')
const menu = ref(false)
const renaming = ref(false)
const newName = ref('')
const nameEl = ref<HTMLInputElement>()

const review = computed(() => props.deck.due - props.deck.new_due - props.deck.learn_due)
const fromNotes = computed(() => props.cards.filter((c) => c.note_path).length)
const keys = computed(() => new Set(props.cards.map((c) => c.key)))
const deckLog = computed(() => props.log.filter((l) => keys.value.has(l.key)))
const last = computed(() => deckLog.value.at(-1))

const stages = computed(() => {
  const n = { new: 0, learn: 0, young: 0, settled: 0 }
  for (const c of props.cards) n[stageOf(c)]++
  return n
})
const dueKey = (c: Card) => (c.new ? '' : c.due ? '0' : c.due_date ?? '')
const list = computed(() => [...props.cards].sort((a, b) => dueKey(a).localeCompare(dueKey(b))).slice(0, 10))
const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }))
const dueText = (c: Card) => (c.new || c.due ? t('heute') : c.due_date === ymd(addDays(new Date(), 1)) ? t('morgen') : dateFmt.value.format(new Date(c.due_date + 'T00:00')))
const dueClass = (c: Card) => (c.new ? 'c-new' : c.learning ? 'c-learn' : c.due ? 'c-due' : '')

// Trefferquote je Woche, älteste zuerst (letzte 8 Wochen).
const weeks = computed(() => {
  const now = Date.now()
  return Array.from({ length: 8 }, (_, i) => {
    const w = 7 - i
    return accuracy(deckLog.value.filter((l) => Math.floor((now - Date.parse(l.at)) / (7 * 864e5)) === w))
  })
})
const acc30 = computed(() => accuracy(deckLog.value.filter((l) => Date.now() - Date.parse(l.at) < 30 * 864e5)))
const ahead = computed(() => forecast(props.cards))
const aheadMax = computed(() => Math.max(1, ...ahead.value))
const known = computed(() => props.cards.filter((c) => !c.new))
const avgInterval = computed(() => (known.value.length ? Math.round(known.value.reduce((n, c) => n + c.interval_days, 0) / known.value.length) : 0))
const maxInterval = computed(() => Math.max(0, ...known.value.map((c) => c.interval_days)))
const dayLabels = computed(() => Array.from({ length: 7 }, (_, i) => addDays(new Date(), i).toLocaleDateString(locale.value, { weekday: 'narrow' })))

async function run(fn: () => Promise<void>, name: string | null) {
  try {
    await fn()
    error.value = ''
    emit('changed', name)
  } catch (e) {
    error.value = errorMessage(e)
  }
}
async function startRename() {
  menu.value = false
  newName.value = props.deck.name
  renaming.value = true
  await nextTick()
  nameEl.value?.select()
}
function rename() {
  const to = newName.value.trim()
  renaming.value = false
  if (to && to !== props.deck.name) void run(() => renameDeck(props.deck.name, to), to)
}
const recolor = (c: string) => run(() => setDeckColor(props.deck.name, c), props.deck.name)
function relimit(e: Event) {
  const n = (e.target as HTMLInputElement).valueAsNumber
  if (Number.isInteger(n) && n >= 0 && n !== props.deck.new_per_day) void run(() => setDeck(props.deck.name, { new_per_day: n }), props.deck.name)
}
const remove = () => run(() => deleteDeck(props.deck.name), null)

useShortcuts((e) => {
  if (e.key === 'Escape') return menu.value ? (menu.value = false) : emit('back')
  if ((e.key === 'l' || e.key === ' ') && props.deck.due) {
    e.preventDefault()
    emit('learn', false)
  } else if (e.key === 'n') {
    e.preventDefault()
    emit('add')
  } else if (e.key === 'b') emit('browse')
})
</script>

<template>
  <div class="dk">
    <header class="dk-top">
      <button type="button" class="col-link dk-back" @click="emit('back')"><svg class="ic"><use href="#i-left" /></svg>{{ $t('Alle Fächer') }}</button>
      <div class="dk-tools">
        <button type="button" class="btn btn-secondary" @click="emit('browse')">{{ $t('Kartenbrowser') }}<kbd>B</kbd></button>
        <button type="button" class="btn btn-secondary" @click="emit('add')">{{ $t('Hinzufügen') }}<kbd>N</kbd></button>
        <div class="dk-menu-wrap">
          <button type="button" class="btn btn-secondary dk-more" :aria-expanded="menu" :aria-label="$t('Fach-Optionen')" @click="menu = !menu">⋯</button>
          <template v-if="menu">
            <div class="dk-scrim" @click="menu = false"></div>
            <div class="dk-menu" role="menu">
              <button type="button" role="menuitem" class="dk-mi" @click="startRename">{{ $t('Umbenennen') }}</button>
              <div class="dk-sw" role="group" :aria-label="$t('Farbe')">
                <button v-for="[c, name] in PROJECT_COLORS" :key="c" type="button" :style="{ '--sc': `var(--p-${c})` }" :aria-label="$t(name)" :aria-pressed="deck.color === c" @click="recolor(c)"></button>
              </div>
              <label class="dk-limit">{{ $t('Neue Karten pro Tag') }}
                <input class="input" type="number" min="0" max="9999" :value="deck.new_per_day" @change="relimit" @keydown.enter="relimit" />
              </label>
              <hr />
              <DeleteButton ghost :label="$t('Fach löschen …')" :text="fromNotes
                ? $t('„{name}“ und seine Karten in Kairo löschen? {n} Karten aus Notizen bleiben im Ordner.', { name: deck.name, n: fromNotes })
                : $t('„{name}“ mit allen Karten löschen?', { name: deck.name })" @confirm="remove" />
            </div>
          </template>
        </div>
      </div>
    </header>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <section class="card dk-hero" :aria-label="$t('Heute')">
      <div class="dk-name">
        <form v-if="renaming" class="dk-rename" @submit.prevent="rename">
          <input ref="nameEl" v-model="newName" class="input" :aria-label="$t('Name des Fachs')" @blur="rename" @keydown.esc.stop="renaming = false" />
        </form>
        <h1 v-else class="dk-title"><i :style="{ background: colorOf(deck.name) }"></i><button type="button" :title="$t('Umbenennen')" @click="startRename">{{ deck.name }}</button></h1>
        <div class="dk-sub">
          {{ $tn(deck.total, '{n} Karte', '{n} Karten') }}<template v-if="fromNotes"> · {{ $tn(fromNotes, '{n} aus Notizen', '{n} aus Notizen') }}</template>
          <template v-if="last"> · {{ $t('zuletzt {when}', { when: new Date(last.at).toLocaleString(locale, { weekday: 'short', hour: '2-digit', minute: '2-digit' }) }) }}</template>
        </div>
      </div>
      <div class="dk-big"><b class="mono c-new">{{ deck.new_due }}</b><span>{{ $t('Neu') }}</span></div>
      <div class="dk-big"><b class="mono c-learn">{{ deck.learn_due }}</b><span>{{ $t('Lernen') }}</span></div>
      <div class="dk-big"><b class="mono c-due">{{ review }}</b><span>{{ $t('Fällig') }}</span></div>
      <div class="dk-go">
        <button type="button" class="btn btn-primary dk-learn" :disabled="!deck.due" @click="emit('learn', false)">{{ $t('Jetzt lernen') }}<kbd>L</kbd></button>
        <button type="button" class="col-link dk-practice" :disabled="!deck.total" @click="emit('learn', true)">{{ $t('ohne Wertung üben') }}</button>
      </div>
    </section>

    <div class="dk-grid">
      <section class="card dk-cards" :aria-label="$t('Karten')">
        <div class="dk-ch"><span>{{ $t('Karten') }}</span><button type="button" class="col-link" @click="emit('browse')">{{ $t('Im Kartenbrowser öffnen ›') }}</button></div>
        <div class="dk-tr dk-th"><span>{{ $t('Frage') }}</span><span>{{ $t('Fällig') }}</span><span>{{ $t('Abstand') }}</span></div>
        <div v-for="c in list" :key="c.key" class="dk-tr">
          <span class="dk-cell">{{ c.question }}<em v-if="c.new">{{ $t('neu') }}</em><em v-else-if="c.note_path">{{ $t('Notiz') }}</em></span>
          <span class="mono dk-when" :class="dueClass(c)">{{ dueText(c) }}</span>
          <span class="mono dk-when">{{ c.new || !c.interval_days ? '–' : intervalText(c.interval_days) }}</span>
        </div>
        <div v-if="!cards.length" class="dk-empty">
          <p>{{ $t('Noch keine Karten in diesem Fach.') }}</p>
          <button type="button" class="btn btn-primary" @click="emit('add')">{{ $t('Erste Karte') }}</button>
        </div>
        <div v-else-if="cards.length > list.length" class="dk-more-row">{{ $t('und {n} weitere', { n: cards.length - list.length }) }}<template v-if="fromNotes"> · {{ $t('Karten aus Notizen änderst du in der Notiz.') }}</template></div>
      </section>

      <div class="dk-side">
        <section class="card dk-panel" :aria-label="$t('Bestand')">
          <div class="dk-ph"><span>{{ $t('Bestand') }}</span><span class="mono">{{ deck.total }}</span></div>
          <div class="dk-stack" role="img" :aria-label="$t('{a} sitzen, {b} in Arbeit, {c} neu', { a: stages.settled, b: stages.young + stages.learn, c: stages.new })">
            <i class="s0" :style="{ flex: stages.settled }"></i><i class="s1" :style="{ flex: stages.young + stages.learn }"></i><i class="s2" :style="{ flex: stages.new }"></i>
          </div>
          <div class="dk-legend"><span>{{ $t('{n} sitzen', { n: stages.settled }) }}</span><span>{{ $t('{n} in Arbeit', { n: stages.young + stages.learn }) }}</span><span>{{ $t('{n} neu', { n: stages.new }) }}</span></div>
        </section>
        <section class="card dk-panel" :aria-label="$t('Trefferquote')">
          <div class="dk-ph"><span>{{ $t('Richtig beantwortet · 8 Wochen') }}</span><span class="mono dk-strong">{{ acc30 === null ? '–' : acc30 + ' %' }}</span></div>
          <div class="dk-bars" role="img" :aria-label="weeks.map((w) => (w === null ? '–' : w + ' %')).join(', ')">
            <i v-for="(w, i) in weeks" :key="i" :class="{ on: i === 7 }" :style="{ height: (w ?? 3) + '%' }"></i>
          </div>
          <div class="dk-x mono"><span>{{ $t('vor 8 Wochen') }}</span><span>{{ $t('diese Woche') }}</span></div>
        </section>
        <section class="card dk-panel dk-grow" :aria-label="$t('Fällig · nächste 7 Tage')">
          <div class="dk-ph"><span>{{ $t('Fällig · nächste 7 Tage') }}</span><span class="mono">{{ ahead.reduce((a, b) => a + b, 0) }}</span></div>
          <div class="dk-bars" role="img" :aria-label="ahead.join(', ')">
            <i v-for="(n, i) in ahead" :key="i" :class="{ on: i === 0 }" :style="{ height: Math.max(3, (n / aheadMax) * 100) + '%' }"></i>
          </div>
          <div class="dk-x mono"><span v-for="(d, i) in dayLabels" :key="i">{{ d }}</span></div>
          <div class="dk-foot"><span>{{ $t('Ø Abstand {n}', { n: intervalText(avgInterval) }) }}</span><span>{{ $t('längster {n}', { n: intervalText(maxInterval) }) }}</span></div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dk { flex: 1; display: flex; flex-direction: column; gap: 18px; }
.dk-top { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
.dk-back { display: inline-flex; align-items: center; gap: 6px; padding: 0; border: 0; background: none; cursor: pointer; }
.dk-tools { display: flex; gap: 8px; }
kbd { margin-left: 8px; font: 500 10.5px/1 var(--font-mono); padding: 2px 5px; border-radius: 4px; border: 1px solid var(--br-default); color: var(--tx-muted); }
.btn-primary kbd { border-color: color-mix(in srgb, currentColor 30%, transparent); color: inherit; opacity: 0.7; }
.dk-menu-wrap { position: relative; }
.dk-more { width: 40px; justify-content: center; padding: 0; font-size: 16px; }
.dk-scrim { position: fixed; inset: 0; z-index: 20; }
.dk-menu { position: absolute; right: 0; top: calc(100% + 6px); z-index: 21; width: 240px; display: flex; flex-direction: column; gap: 2px; padding: 6px; border-radius: var(--r-m); border: 1px solid var(--br-default); background: var(--bg-1); box-shadow: var(--shadow-pop); }
.dk-menu hr { width: 100%; margin: 4px 0; border: 0; border-top: 1px solid var(--br-subtle); }
.dk-menu :deep(.btn) { justify-content: flex-start; color: var(--p-pink); }
.dk-mi { height: 36px; padding: 0 10px; border: 0; border-radius: var(--r-s); background: none; color: var(--tx-primary); font: 400 13.5px/1 var(--font-ui); text-align: left; cursor: pointer; }
.dk-mi:hover { background: var(--bg-2); }
.dk-limit { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 4px 10px; font: 400 13.5px/1.2 var(--font-ui); color: var(--tx-primary); }
.dk-limit .input { width: 72px; height: 32px; text-align: right; }
.dk-sw { display: flex; flex-wrap: wrap; gap: 8px; padding: 8px 10px; }
.dk-sw button { width: 20px; height: 20px; padding: 0; border: 0; border-radius: 50%; background: var(--sc); cursor: pointer; }
.dk-sw button[aria-pressed='true'] { box-shadow: 0 0 0 2px var(--bg-1), 0 0 0 3.5px var(--sc); }

.dk-hero { display: flex; align-items: center; gap: 8px; padding: 22px 24px; }
.dk-name { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.dk-title { display: flex; align-items: center; gap: 12px; margin: 0; font: 500 28px/1.2 var(--font-ui); letter-spacing: -0.02em; }
.dk-title i { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.dk-title button { min-width: 0; padding: 0; border: 0; background: none; color: inherit; font: inherit; letter-spacing: inherit; text-align: left; cursor: text; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dk-rename .input { max-width: 420px; font-size: 22px; height: 44px; }
.dk-sub { padding-left: 22px; font: 400 13.5px/1.4 var(--font-ui); color: var(--tx-muted); }
.dk-big { display: flex; flex-direction: column; gap: 2px; padding: 0 22px; }
.dk-big + .dk-big { border-left: 1px solid var(--br-subtle); }
.dk-big b { font-size: 36px; font-weight: 500; line-height: 1.1; }
.dk-big span { font-size: 12.5px; color: var(--tx-muted); }
.dk-go { display: flex; flex-direction: column; align-items: center; gap: 8px; margin-left: 14px; }
.dk-learn { height: 48px; padding: 0 26px; font-size: 15px; }
.dk-practice { padding: 0; border: 0; background: none; font-size: 12.5px; cursor: pointer; }
.dk-practice:disabled { opacity: 0.4; cursor: default; }
.c-new { color: var(--a-blue-tx); }
.c-learn { color: var(--p-pink); }
.c-due { color: var(--p-green); }

.dk-grid { flex: 1; display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 18px; align-items: stretch; }
.dk-cards { display: flex; flex-direction: column; padding: 0; overflow: hidden; }
.dk-ch { display: flex; justify-content: space-between; align-items: center; padding: 12px 18px; font: 500 13.5px/1 var(--font-ui); }
.dk-ch button { padding: 0; border: 0; background: none; font-size: 12.5px; cursor: pointer; }
.dk-tr { display: grid; grid-template-columns: minmax(0, 1fr) 92px 70px; gap: 14px; align-items: center; min-height: 44px; padding: 0 18px; border-top: 1px solid var(--br-subtle); font: 400 13.5px/1.3 var(--font-ui); }
.dk-th { min-height: 34px; font-size: 12px; color: var(--tx-muted); }
.dk-cell { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dk-cell em { margin-left: 8px; padding: 2px 7px; border-radius: 999px; border: 1px solid var(--br-default); font: normal 11px/1 var(--font-ui); color: var(--tx-muted); }
.dk-when { font-size: 12px; color: var(--tx-muted); }
.dk-empty { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; padding: 18px; border-top: 1px solid var(--br-subtle); }
.dk-empty p { margin: 0; color: var(--tx-muted); }
.dk-more-row { margin-top: auto; padding: 10px 18px; border-top: 1px solid var(--br-subtle); font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }
.dk-side { display: flex; flex-direction: column; gap: 18px; }
.dk-panel { display: flex; flex-direction: column; gap: 12px; padding: 16px 18px; }
.dk-grow { flex: 1; }
.dk-ph { display: flex; justify-content: space-between; font: 400 12.5px/1 var(--font-ui); color: var(--tx-muted); }
.dk-strong { color: var(--tx-primary); }
.dk-stack { display: flex; gap: 2px; height: 8px; border-radius: 4px; overflow: hidden; background: var(--bg-3); }
.dk-stack .s0 { background: var(--a-event); }
.dk-stack .s1 { background: var(--br-strong); }
.dk-stack .s2 { background: var(--br-default); }
.dk-legend, .dk-foot { display: flex; justify-content: space-between; font: 400 12.5px/1.3 var(--font-ui); color: var(--tx-secondary); }
.dk-foot { margin-top: auto; padding-top: 10px; border-top: 1px solid var(--br-subtle); }
.dk-bars { display: flex; gap: 6px; align-items: flex-end; height: 72px; }
.dk-bars i { flex: 1; border-radius: 3px; background: var(--br-default); }
.dk-bars i.on { background: var(--a-event); }
.dk-x { display: flex; justify-content: space-between; font-size: 11px; color: var(--tx-muted); }

@media (max-width: 1000px) {
  .dk-hero { flex-wrap: wrap; }
  .dk-name { flex-basis: 100%; }
  .dk-big:first-of-type { padding-left: 0; }
  .dk-go { margin-left: auto; }
  .dk-grid { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .dk-tools kbd, .dk-learn kbd { display: none; }
  .dk-big { padding: 0 14px; }
  .dk-big b { font-size: 28px; }
  .dk-go { flex-basis: 100%; align-items: stretch; margin: 8px 0 0; }
  .dk-tr { grid-template-columns: minmax(0, 1fr) 70px; }
  .dk-tr > :last-child { display: none; }
}
</style>
