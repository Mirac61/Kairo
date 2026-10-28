<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { errorMessage, reviewCard, undoReview, type Card, type CardLog, type Deck } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { forecast, intervalText, perDay } from '@/lib/cards'
import { addDays, ymd } from '@/lib/dates'
import { t, tn } from '@/lib/i18n'
import CardText from './CardText.vue'

// Eine Lernrunde wie bei Anki: Vorderseite, „Antwort zeigen“, vier Bewertungen. „Nochmal“ stellt die Karte ans Ende.
// practice: Übungsrunde, nichts wird gespeichert. Am Ende die Auswertung der Runde.
const props = defineProps<{
  round: Card[]
  title: string | null
  practice: boolean
  cards: Card[] // alle Karten, für „morgen fällig“ (nach dem Neuladen am Rundenende)
  decks: Deck[]
  log: CardLog[]
  colorOf: (deck: string) => string
}>()
const emit = defineEmits<{ exit: []; finished: []; again: [keys: string[]] }>()

const GRADES = [t('Nochmal'), t('Schwer'), t('Gut'), t('Leicht')]

const queue = ref<Card[]>([...props.round])
const practice = ref(props.practice)
const reverse = ref(false)
const shown = ref(false)
const busy = ref(false)
const error = ref('')
const started = Date.now()
const minutes = ref(0)
// Pro Karte: alle Bewertungen dieser Runde und der nächste Abstand (null in der Übungsrunde).
const results = reactive(new Map<string, { card: Card; grades: number[]; next: number | null }>())
const history = ref<{ queue: Card[]; key: string; saved: boolean }[]>([])

const card = computed(() => queue.value[0])
const done = computed(() => props.round.length - new Set(queue.value.map((c) => c.key)).size)
const counts = computed(() => {
  const n = { new: 0, learn: 0, due: 0 }
  for (const c of queue.value) n[c.new ? 'new' : c.learning ? 'learn' : 'due']++
  return n
})
const kindOf = (c?: Card) => (!c ? '' : c.new ? 'new' : c.learning ? 'learn' : 'due')
const front = computed(() => (card.value ? (reverse.value ? card.value.answer : card.value.question) : ''))
const back = computed(() => (card.value ? (reverse.value ? card.value.question : card.value.answer) : ''))

async function grade(g: number) {
  const c = card.value
  if (!c || !shown.value || busy.value) return
  busy.value = true
  try {
    const saved = !practice.value
    const res = saved ? await reviewCard(c.key, g) : null
    history.value.push({ queue: [...queue.value], key: c.key, saved })
    const r = results.get(c.key) ?? { card: c, grades: [], next: null }
    r.grades.push(g)
    r.next = res?.interval_days ?? null
    results.set(c.key, r)
    queue.value.shift()
    // Nach „Nochmal“ ist die Karte wieder in Arbeit; die Abstände sind dann die einer neuen Wiederholung (SM-2 im Backend).
    if (g === 0) queue.value.push({ ...c, new: false, learning: true, next_intervals: [0, 1, 1, 4] })
    shown.value = false
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

async function undo() {
  const h = history.value.at(-1)
  if (!h || busy.value) return
  busy.value = true
  try {
    if (h.saved) await undoReview()
    history.value.pop()
    queue.value = h.queue
    const r = results.get(h.key)!
    r.grades.pop()
    if (!r.grades.length) results.delete(h.key)
    shown.value = true
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

watch(card, (c) => {
  if (c) return
  minutes.value = Math.max(1, Math.round((Date.now() - started) / 60000))
  emit('finished')
})

// --- Auswertung
const gradeCounts = computed(() => {
  const n = [0, 0, 0, 0]
  for (const r of results.values()) for (const g of r.grades) n[g]!++
  return n
})
const roundList = computed(() => [...results.values()].sort((a, b) => Math.min(...a.grades) - Math.min(...b.grades) || b.grades.length - a.grades.length))
const hard = computed(() => roundList.value.filter((r) => r.grades.some((g) => g <= 1)).map((r) => r.card.key))
const answerText = (gs: number[]) => {
  const again = gs.filter((g) => g === 0).length
  return again ? tn(again, '{n}× nochmal', '{n}× nochmal') : GRADES[gs.at(-1)!]!
}
const open = computed(() => props.decks.filter((d) => d.due > 0))
const tomorrow = computed(() => forecast(props.cards, new Date(), 2)[1] ?? 0)
const week = computed(() => {
  const days = perDay(props.log)
  const today = new Date()
  const monday = addDays(today, -((today.getDay() + 6) % 7))
  return Array.from({ length: 7 }, (_, i) => {
    const d = addDays(monday, i)
    return { n: days.get(ymd(d)) ?? 0, today: ymd(d) === ymd(today), label: d.toLocaleDateString(undefined, { weekday: 'narrow' }) }
  })
})
const weekTotal = computed(() => week.value.reduce((a, d) => a + d.n, 0))
const weekMax = computed(() => Math.max(1, ...week.value.map((d) => d.n)))

useShortcuts((e) => {
  if (e.key === 'Escape') return emit('exit')
  if (e.key === 'z') {
    e.preventDefault()
    return void undo()
  }
  if (!card.value) return
  if (e.key === ' ' || e.key === 'Enter') {
    // Wie bei Anki: erst aufdecken, dann „Gut“.
    e.preventDefault()
    if (shown.value) void grade(2)
    else shown.value = true
  } else if (shown.value && ['1', '2', '3', '4'].includes(e.key)) {
    e.preventDefault()
    void grade(Number(e.key) - 1)
  }
})
</script>

<template>
  <div class="cl">
    <header class="cl-top">
      <button type="button" class="col-link cl-back" @click="emit('exit')">
        <svg class="ic"><use href="#i-left" /></svg>
        <i v-if="title" class="cl-dot" :style="{ background: colorOf(title) }"></i>
        <span>{{ title ?? $t('Alle Fächer') }}</span>
        <kbd>Esc</kbd>
      </button>

      <div class="cl-counts" :aria-label="$t('{a} neu, {b} lernen, {c} fällig', { a: counts.new, b: counts.learn, c: counts.due })">
        <span class="cl-count-pill c-new" :class="{ on: kindOf(card) === 'new' }">
          <small>{{ $t('Neu') }}</small>
          <b class="mono">{{ counts.new }}</b>
        </span>
        <span class="cl-count-pill c-learn" :class="{ on: kindOf(card) === 'learn' }">
          <small>{{ $t('Lernen') }}</small>
          <b class="mono">{{ counts.learn }}</b>
        </span>
        <span class="cl-count-pill c-due" :class="{ on: kindOf(card) === 'due' }">
          <small>{{ $t('Fällig') }}</small>
          <b class="mono">{{ counts.due }}</b>
        </span>
      </div>

      <div class="cl-tools">
        <button type="button" class="cl-tool" :aria-pressed="practice" :title="$t('Üben ohne Wertung')" @click="practice = !practice">
          <span>{{ $t('Üben ohne Wertung') }}</span>
        </button>
        <button type="button" class="cl-tool" :aria-pressed="reverse" :title="$t('Umgedreht')" @click="reverse = !reverse; shown = false">
          <svg class="ic"><use href="#i-redo" /></svg>
          <span>{{ $t('Umgedreht') }}</span>
        </button>
        <button type="button" class="cl-tool" :disabled="!history.length || busy" :title="$t('Rückgängig')" @click="undo">
          <svg class="ic"><use href="#i-undo" /></svg>
          <span>{{ $t('Rückgängig') }}</span>
          <kbd>Z</kbd>
        </button>
      </div>
    </header>

    <div class="cl-progress" aria-hidden="true">
      <i :style="{ width: ((round.length ? done / round.length : 0) * 100) + '%' }"></i>
    </div>
    <div v-if="error" class="badge cl-err" role="alert">{{ error }}</div>

    <!-- Karte -->
    <div v-if="card" class="cl-stage">
      <article
        class="cl-card"
        :class="{ 'is-hidden': !shown, 'is-shown': shown }"
        :style="{ '--dc': colorOf(card.deck) }"
        :aria-label="$t('Karte')"
        @click="!shown && (shown = true)"
      >
        <div class="cl-meta">
          <div class="cl-meta-left">
            <span class="cl-chip"><i></i>{{ card.deck }}</span>
            <span v-if="card.new" class="cl-tag c-new">{{ $t('Neu') }}</span>
            <span v-else-if="card.learning" class="cl-tag c-learn">{{ $t('Lernen') }}</span>
            <span v-if="reverse" class="cl-tag c-rev">{{ $t('Antwort → Frage') }}</span>
          </div>
          <span class="cl-pos mono">{{ done + 1 }} / {{ round.length }}</span>
        </div>

        <div class="cl-body">
          <p v-if="practice" class="cl-practice">{{ $t('Übungsrunde · Bewertungen ändern den Lernplan nicht') }}</p>

          <!-- Vor dem Aufdecken: Nur Frage im Zentrum -->
          <div v-if="!shown" class="cl-front-wrap">
            <CardText class="cl-front" :text="front" />
            <div class="cl-hint">
              <span>{{ $t('Klicken oder Leertaste drücken') }}</span>
            </div>
          </div>

          <!-- Nach dem Aufdecken: Frage oben kompakt, Antwort dominant im Zentrum -->
          <div v-else class="cl-revealed-wrap">
            <div class="cl-q-context">
              <span class="cl-sec-label">{{ $t('Frage') }}</span>
              <CardText class="cl-front-compact" :text="front" />
            </div>
            <div class="cl-divider" aria-hidden="true"></div>
            <div class="cl-a-context">
              <span class="cl-sec-label cl-a-label">{{ $t('Antwort') }}</span>
              <CardText class="cl-back-text" :text="back" />
              <router-link v-if="card.note_path" class="col-link cl-src" :to="{ path: '/notes', query: { open: card.note_path } }">
                {{ $t('aus {path}', { path: card.note_path }) }}
              </router-link>
            </div>
          </div>
        </div>
      </article>

      <div class="cl-actions">
        <button v-if="!shown" type="button" class="btn btn-primary cl-show" @click="shown = true">
          <span>{{ $t('Antwort zeigen') }}</span>
          <kbd>{{ $t('Leertaste') }}</kbd>
        </button>

        <template v-else>
          <button
            v-for="(g, i) in GRADES"
            :key="i"
            type="button"
            class="cl-grade"
            :class="[`g${i}`, { 'is-default': i === 2 }]"
            :disabled="busy"
            @click="grade(i)"
          >
            <div class="cl-grade-top">
              <span class="cl-grade-name">{{ g }}</span>
              <kbd class="cl-grade-kbd">{{ i + 1 }}<template v-if="i === 2"> · ␣</template></kbd>
            </div>
            <div class="cl-grade-interval">
              <b class="mono">{{ practice ? '–' : intervalText(card.next_intervals[i]!) }}</b>
            </div>
          </button>
        </template>
      </div>
    </div>

    <!-- Runde geschafft -->
    <div v-else class="cl-done">
      <div class="cl-done-main">
        <div class="v-head-row cl-done-head">
          <div>
            <div class="v-sub cl-done-sub">
              {{ $tn(results.size, '{n} Karte', '{n} Karten') }} · {{ $t('{n} min', { n: minutes }) }} · {{ $t('morgen {n} fällig', { n: tomorrow }) }}
            </div>
            <h1 class="v-title cl-celebrate-title">
              <span>🎉</span>
              {{ $t('Runde geschafft') }}
            </h1>
          </div>
          <div class="cl-done-act">
            <button type="button" class="btn btn-secondary" @click="emit('exit')">{{ $t('Zu den Fächern') }}</button>
            <button v-if="hard.length" type="button" class="btn btn-primary" @click="emit('again', hard)">
              {{ $t('Schwere nochmal üben') }}<span class="cl-n">{{ hard.length }}</span>
            </button>
          </div>
        </div>

        <section class="card cl-grades-sum" :aria-label="$t('Bewertungen')">
          <div class="cl-kpis">
            <div v-for="(g, i) in GRADES" :key="i" class="cl-kpi">
              <b class="mono" :class="`c-g${i}`">{{ gradeCounts[i] }}</b>
              <span>{{ g }}</span>
            </div>
          </div>
          <div class="cl-dist" role="img" :aria-label="GRADES.map((g, i) => `${gradeCounts[i]} ${g}`).join(', ')">
            <i v-for="(n, i) in gradeCounts" :key="i" :class="`g${i}`" :style="{ flex: n }"></i>
          </div>
        </section>

        <section class="card cl-list" :aria-label="$t('Karten dieser Runde')">
          <div class="cl-row cl-row-h">
            <span>{{ $t('Karte') }}</span>
            <span>{{ $t('Antwort') }}</span>
            <span>{{ $t('Wieder dran') }}</span>
          </div>
          <div v-for="r in roundList" :key="r.card.key" class="cl-row">
            <span class="cl-cell">{{ r.card.question }}</span>
            <span class="cl-pill" :class="{ 'is-again': r.grades.includes(0) }">{{ answerText(r.grades) }}</span>
            <span class="mono cl-when">{{ r.next === null ? $t('Übung') : r.next === 0 ? $t('heute') : intervalText(r.next) }}</span>
          </div>
          <p v-if="!roundList.length" class="v-sub cl-row">{{ $t('Nichts bewertet.') }}</p>
        </section>
      </div>

      <aside class="cl-done-side">
        <section class="card cl-side" :aria-label="$t('Heute noch offen')">
          <div class="cl-side-h"><span>{{ $t('Heute noch offen') }}</span><span class="mono">{{ open.reduce((n, d) => n + d.due, 0) }}</span></div>
          <div v-for="d in open" :key="d.name" class="cl-side-r">
            <i class="cl-dot" :style="{ background: colorOf(d.name) }"></i>
            <span>{{ d.name }}</span>
            <span class="mono c-due">{{ $t('{n} fällig', { n: d.due }) }}</span>
          </div>
          <p v-if="!open.length" class="cl-side-empty">{{ $t('Alles erledigt für heute.') }}</p>
        </section>
        <section class="card cl-side cl-week" :aria-label="$t('Diese Woche gelernt')">
          <div class="cl-side-h"><span>{{ $t('Diese Woche gelernt') }}</span><span class="mono">{{ $tn(weekTotal, '{n} Karte', '{n} Karten') }}</span></div>
          <div class="cl-bars" role="img" :aria-label="week.map((d) => `${d.label} ${d.n}`).join(', ')">
            <i v-for="(d, i) in week" :key="i" :class="{ on: d.today }" :style="{ height: Math.max(4, (d.n / weekMax) * 100) + '%' }"></i>
          </div>
          <div class="cl-bars-x mono"><span v-for="(d, i) in week" :key="i" :class="{ on: d.today }">{{ d.label }}</span></div>
        </section>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.cl { flex: 1; display: flex; flex-direction: column; margin: -32px -32px -64px; }
.cl-top {
  display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center; gap: 16px; height: 60px; padding: 0 24px;
  background: var(--bg-1); border-bottom: 1px solid var(--br-subtle);
}
.cl-back {
  display: inline-flex; align-items: center; gap: 8px; padding: 0;
  border: 0; background: none; cursor: pointer; min-width: 0;
  color: var(--tx-secondary); font: 500 13.5px/1 var(--font-ui);
}
.cl-back:hover { color: var(--tx-primary); }
.cl-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
kbd {
  font: 500 10.5px/1 var(--font-mono); padding: 2px 5px; border-radius: 4px;
  border: 1px solid var(--br-default); color: var(--tx-muted); background: var(--bg-2);
}

.cl-counts { display: flex; gap: 6px; align-items: center; }
.cl-count-pill {
  display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px;
  border-radius: 999px; background: var(--bg-2); border: 1px solid var(--br-subtle);
  font: 500 12px/1 var(--font-ui); opacity: 0.65; transition: opacity 0.15s, border-color 0.15s;
}
.cl-count-pill small { font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--tx-muted); }
.cl-count-pill b { font-size: 12.5px; }
.cl-count-pill.on { opacity: 1; border-color: var(--br-strong); background: var(--bg-3); }

/* Eine Akzentfarbe wie in der Liste: Blau = fällig; Rot nur für „Nochmal“. */
.c-new, .c-learn { color: var(--tx-secondary); }
.c-again { color: var(--a-red); }
.c-due { color: var(--a-blue-tx); }

.cl-tools { display: flex; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }
.cl-tool {
  display: inline-flex; align-items: center; gap: 6px; height: 34px; padding: 0 10px;
  border-radius: var(--r-s); border: 1px solid transparent; background: none;
  color: var(--tx-secondary); font: 400 12.5px/1 var(--font-ui); cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.cl-tool .ic { width: 14px; height: 14px; }
.cl-tool:hover:not(:disabled) { background: var(--bg-2); color: var(--tx-primary); }
.cl-tool[aria-pressed='true'] { border-color: var(--br-strong); background: var(--bg-2); color: var(--tx-primary); }
.cl-tool:disabled { opacity: 0.35; cursor: default; }

.cl-progress { height: 3px; background: var(--bg-2); }
.cl-progress i {
  display: block; height: 100%;
  background: var(--tx-secondary);
  transition: width 0.25s ease-out;
}
.cl-err { margin: 12px auto 0; }

/* Stage & Card */
.cl-stage {
  flex: 1; width: 100%; max-width: 860px; margin: 0 auto; padding: 32px 20px 48px;
  box-sizing: border-box; display: flex; flex-direction: column; justify-content: center; gap: 16px;
}

.cl-card {
  min-height: 440px; display: flex; flex-direction: column; gap: 18px; padding: 24px 32px 36px;
  border-radius: 18px; border: 1px solid var(--br-default);
  background: var(--bg-1);
  box-shadow: 0 14px 40px rgba(0, 0, 0, 0.3);
  transition: border-color 0.2s, box-shadow 0.2s;
}
.cl-card.is-hidden { cursor: pointer; }
.cl-card.is-hidden:hover {
  border-color: var(--br-strong);
}

.cl-meta { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.cl-meta-left { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.cl-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px;
  border-radius: 999px; background: var(--bg-2); border: 1px solid var(--br-subtle);
  font: 500 12px/1 var(--font-ui); color: var(--tx-secondary);
}
.cl-chip i { width: 7px; height: 7px; border-radius: 50%; background: var(--dc); }
.cl-tag {
  padding: 3px 8px; border-radius: 6px; background: var(--bg-2);
  font: 600 10.5px/1 var(--font-ui); letter-spacing: 0.05em; text-transform: uppercase;
}
.cl-tag.c-rev { color: var(--tx-muted); }
.cl-pos { font-size: 12px; color: var(--tx-muted); }

.cl-body {
  flex: 1; display: flex; flex-direction: column; justify-content: center;
  position: relative; min-height: 280px;
}
.cl-practice {
  position: absolute; top: 0; left: 0; right: 0; margin: 0;
  text-align: center; font: 500 12px/1.4 var(--font-ui); color: var(--p-yellow);
}

/* Vor dem Aufdecken */
.cl-front-wrap {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 24px; text-align: center; padding: 20px 0;
}
.cl-front {
  width: 100%; font: 500 28px/1.35 var(--font-ui); letter-spacing: -0.015em; color: var(--tx-primary);
}
.cl-hint {
  display: inline-flex; align-items: center; gap: 6px; font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted); opacity: 0.8;
}

/* Nach dem Aufdecken: Strukturierte Aufteilung */
.cl-revealed-wrap {
  flex: 1; display: flex; flex-direction: column; justify-content: space-between;
  gap: 18px; width: 100%; animation: cl-fade-in 0.2s ease-out;
}
@keyframes cl-fade-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.cl-sec-label {
  display: block; font: 600 11px/1 var(--font-ui); text-transform: uppercase;
  letter-spacing: 0.08em; color: var(--tx-muted); margin-bottom: 8px;
}
.cl-q-context { padding: 12px 8px 4px; text-align: center; }
.cl-front-compact {
  font: 600 22px/1.35 var(--font-ui); color: var(--tx-primary);
}

.cl-divider {
  width: 100%; border-top: 1px dashed var(--br-strong); margin: 2px 0;
}

.cl-a-context {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  text-align: center; padding: 16px 8px; min-height: 140px;
}
.cl-a-label { color: var(--tx-muted); }
.cl-back-text {
  width: 100%; font: 400 19px/1.55 var(--font-ui); color: var(--tx-secondary);
}
.cl-src { margin-top: 14px; font-size: 12.5px; color: var(--tx-muted); }

/* Aktionen / Bewertungsleiste */
.cl-actions { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.cl-show {
  grid-column: 1 / -1; height: 54px; justify-content: center; border-radius: 14px;
  font-size: 15px; font-weight: 500; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}
.cl-show kbd {
  margin-left: 10px; border-color: color-mix(in srgb, currentColor 30%, transparent);
  color: inherit; opacity: 0.8; background: rgba(0, 0, 0, 0.2);
}

.cl-grade {
  display: flex; flex-direction: column; justify-content: space-between; gap: 8px;
  padding: 14px 14px 12px; border-radius: 14px; border: 1px solid var(--br-default);
  background: var(--bg-1); color: var(--tx-primary); cursor: pointer;
  transition: background 0.15s, border-color 0.15s, transform 0.15s, box-shadow 0.15s;
  text-align: left;
}
.cl-grade:hover:not(:disabled) { border-color: var(--br-strong); background: var(--bg-2); }
/* „Gut“ ist der Normalfall und trägt als einziger Knopf Farbe. */
.cl-grade.is-default { border-color: transparent; background: color-mix(in oklab, var(--a-blue) 84%, #000); }
.cl-grade.is-default:hover:not(:disabled) { border-color: transparent; background: color-mix(in oklab, var(--a-blue) 74%, #000); }
.cl-grade.is-default .cl-grade-name, .cl-grade.is-default .cl-grade-interval { color: #fff; }
.cl-grade.is-default .cl-grade-kbd { color: #fff; border-color: rgba(255, 255, 255, 0.35); background: none; }
.cl-grade.g0 .cl-grade-name { color: var(--a-red); }
.cl-grade:disabled { opacity: 0.4; cursor: default; }

.cl-grade-top { display: flex; justify-content: space-between; align-items: center; }
.cl-grade-name { font: 500 14px/1 var(--font-ui); color: var(--tx-primary); }
.cl-grade-kbd {
  font-size: 10.5px; border-color: var(--br-default); background: var(--bg-2);
}
.cl-grade-interval {
  display: flex; align-items: center; font-size: 13px; color: var(--tx-muted);
}

:root[data-motion='reduce'] .cl-grade { transition: none; transform: none; }
@media (prefers-reduced-motion: reduce) { .cl-grade { transition: none; transform: none; } }

/* Auswertung */
.cl-done { flex: 1; display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 24px; padding: 32px 32px 48px; }
.cl-done-main, .cl-done-side { display: flex; flex-direction: column; gap: 20px; min-width: 0; }
.cl-done-head { margin-bottom: 0; }
.cl-done-sub { margin: 0 0 6px; }
.cl-celebrate-title { display: flex; align-items: center; gap: 10px; }
.cl-done-act { display: flex; gap: 8px; flex-wrap: wrap; }
.cl-n { margin-left: 8px; font: 500 12px/1 var(--font-mono); opacity: 0.7; }

.cl-grades-sum { padding: 0; overflow: hidden; }
.cl-kpis { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); }
.cl-kpi { display: flex; flex-direction: column; gap: 4px; padding: 18px 20px; }
.cl-kpi + .cl-kpi { border-left: 1px solid var(--br-subtle); }
.cl-kpi b { font-size: 28px; font-weight: 500; }
.cl-kpi span { font-size: 12.5px; color: var(--tx-muted); }
.c-g0 { color: var(--a-red); }
.c-g1, .c-g3 { color: var(--tx-primary); }
.c-g2 { color: var(--a-blue-tx); }

.cl-dist { display: flex; gap: 2px; height: 5px; margin: 0 20px 18px; border-radius: 999px; overflow: hidden; background: var(--bg-3); }
.cl-dist .g0 { background: var(--a-red); }
.cl-dist .g1 { background: var(--tx-muted); }
.cl-dist .g2 { background: var(--a-blue); }
.cl-dist .g3 { background: var(--tx-secondary); }

.cl-list { padding: 0; overflow: hidden; }
.cl-row {
  display: grid; grid-template-columns: minmax(0, 1fr) 130px 110px; gap: 14px;
  align-items: center; min-height: 44px; margin: 0; padding: 0 20px;
  border-top: 1px solid var(--br-subtle); font: 400 13.5px/1.3 var(--font-ui);
}
.cl-row-h { border-top: 0; min-height: 38px; font-size: 12px; color: var(--tx-muted); }
.cl-cell { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--tx-primary); }
.cl-pill { justify-self: start; padding: 3px 10px; border-radius: 999px; border: 1px solid var(--br-default); font-size: 12px; color: var(--tx-secondary); background: var(--bg-2); }
.cl-pill.is-again { color: var(--a-red); border-color: color-mix(in srgb, var(--a-red) 40%, var(--br-default)); }
.cl-when { font-size: 12.5px; color: var(--tx-muted); }

.cl-side { padding: 18px 20px; display: flex; flex-direction: column; gap: 6px; }
.cl-side-h { display: flex; justify-content: space-between; padding-bottom: 8px; font-size: 12.5px; color: var(--tx-muted); }
.cl-side-r { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-top: 1px solid var(--br-subtle); font-size: 13.5px; }
.cl-side-r span:nth-child(2) { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cl-side-r .mono { font-size: 12.5px; }
.cl-side-empty { margin: 0; padding: 10px 0; border-top: 1px solid var(--br-subtle); font-size: 13px; color: var(--tx-muted); }
.cl-week { flex: 1; }
.cl-bars { flex: 1; min-height: 100px; display: flex; gap: 8px; align-items: flex-end; }
.cl-bars i { flex: 1; border-radius: 3px; background: var(--bg-3); }
.cl-bars i.on { background: var(--a-blue); }
.cl-bars-x { display: flex; justify-content: space-between; padding-top: 8px; font-size: 11px; color: var(--tx-muted); }
.cl-bars-x span { flex: 1; text-align: center; }
.cl-bars-x .on { color: var(--tx-primary); }

@media (max-width: 760px) {
  .cl { margin: -20px -16px -96px; }
  .cl-top { grid-template-columns: minmax(0, 1fr) auto; height: auto; padding: 12px 16px; }
  .cl-tools { grid-column: 1 / -1; justify-content: flex-start; }
  .cl-back kbd, .cl-tool kbd { display: none; }
  .cl-stage { padding: 16px 12px; justify-content: flex-start; }
  .cl-card { min-height: 280px; padding: 18px 20px 28px; }
  .cl-front { font-size: 22px; }
  .cl-back-text { font-size: 17px; }
  .cl-actions { position: sticky; bottom: calc(56px + env(safe-area-inset-bottom)); gap: 6px; padding: 8px 0; background: var(--bg-0); }
  .cl-show kbd, .cl-grade-kbd { display: none; }
  .cl-done { grid-template-columns: 1fr; padding: 20px 16px 32px; }
  .cl-row { grid-template-columns: minmax(0, 1fr) auto; }
  .cl-when { display: none; }
}
</style>
