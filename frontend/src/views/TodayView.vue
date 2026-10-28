<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { completeHabit, dueCards, goalToday, isOpen, readNote, reviewCard, taskAction, uncompleteHabit, updateTask, type Card, type Goal, type Task } from '@/api/client'
import { provideHome } from '@/composables/useHome'
import { useShortcuts } from '@/composables/useShortcuts'
import { columns, findSlot, stopwatch } from '@/lib/dayPlan'
import { dur, hm, shiftDate } from '@/lib/dates'
import { vDialog } from '@/lib/dialog'
import { goalProgressText } from '@/lib/goals'
import { locale, t, tn } from '@/lib/i18n'
import { journalPath, parseEntry } from '@/lib/journal'
import { prefs } from '@/lib/prefs'
import { projectColor } from '@/lib/projectColor'
import { store } from '@/lib/storage'

// Startseite: oben eine Karte mit dem, was jetzt dran ist, darunter Gewohnheiten, Lernkarte und Ziel,
// dann die Aufgaben nach Überfällig/Heute/Demnächst; rechts der Zeitplan mit freien Lücken zum Einplanen.
const home = provideHome()
const { today, tasks, tasksById, projectNames, nowMin, error, blocks, allDay, freeGaps, runEntry, runningTaskId, pausedTask, runSecs, run, offer, setDone, scheduleAt } = home

const events = computed(() => blocks.value.filter((b) => b.kind === 'event'))
const upcomingEvent = computed(() => {
  const cur = events.value.find((b) => b.start <= nowMin.value && nowMin.value < b.end)
  const next = events.value.find((b) => b.start >= nowMin.value)
  if (cur) return t('Aktuell: {title} bis {time} Uhr', { title: cur.title, time: hm(cur.end) })
  if (next) return t('Als Nächstes im Kalender: {title} um {time} Uhr', { title: next.title, time: hm(next.start) })
  if (allDay.value.length) return allDay.value.map((b) => b.title).join(', ')
  return null
})
const shortDate = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(new Date(`${d}T12:00:00`))
const todayFormatted = computed(() =>
  new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())
)
const freeMin = computed(() => freeGaps.value.reduce((s, g) => s + g.end - g.start, 0))
const est = (x: Task) => x.estimated_minutes || 30
const estLabel = (x: Task) => (x.estimated_minutes ? dur(x.estimated_minutes) : '')
const startOf = (x: Task) => blocks.value.find((b) => b.task?.id === x.id)?.start

// Aufgaben: Überfälliges, heute Offenes, Demnächst (nach Datum). Erledigtes von heute klappt unten auf.
const overdue = computed(() => (today.value?.overdue ?? []).filter(isOpen))
const todays = computed(() => (today.value?.tasks ?? []).filter(isOpen))
const doneToday = computed(() => (today.value?.tasks ?? []).filter((x) => x.status === 'COMPLETED'))
const dayOf = (x: Task) => x.planned_date || x.due_at?.slice(0, 10) || null
const upcoming = computed(() => {
  const shown = new Set([...overdue.value, ...todays.value].map((x) => x.id))
  return tasks.value.filter((x) => isOpen(x) && !x.parent_task_id && !shown.has(x.id))
    .sort((a, b) => (dayOf(a) ?? '~').localeCompare(dayOf(b) ?? '~')).slice(0, 6)
})
const shortDay = (d: string | null) => (d ? new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'numeric' }).format(new Date(`${d}T12:00:00`)) : '')

// Karte oben: laufende/pausierte Aufgabe, morgens ohne Plan die Auswahl, ab 18 Uhr der Rückblick,
// sonst die nächste offene Aufgabe von heute, und wenn nichts mehr offen ist „Alles erledigt“.
const current = computed(() => (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined) ?? pausedTask.value)
const heroTask = computed(() => [...overdue.value, ...todays.value][0] ?? null)
const planSkip = ref(store.get('kairo-plan-skip'))
const candidates = computed(() => [...overdue.value, ...todays.value, ...upcoming.value].filter((x) => !x.planned_start_at).slice(0, 4))
const kind = computed(() => {
  const day = today.value?.date
  if (current.value || runEntry.value) return 'run'
  if (nowMin.value >= 18 * 60) return 'review'
  if (nowMin.value < 12 * 60 && planSkip.value !== day && candidates.value.length && !blocks.value.some((b) => b.kind === 'task')) return 'plan'
  return heroTask.value ? 'task' : 'done'
})

const fitGap = (x: Task) => freeGaps.value.find((g) => g.end - g.start >= est(x))
const heroFit = computed(() => {
  const x = heroTask.value
  if (!x) return ''
  const at = startOf(x)
  if (at !== undefined) return t('Eingeplant um {time} Uhr.', { time: hm(at) })
  const g = fitGap(x)
  if (!g) return t('Heute ist keine Lücke groß genug.')
  return g.start <= nowMin.value + 5 ? t('Passt jetzt in deine freie Zeit.') : t('Passt in deine freie Lücke um {time} Uhr.', { time: hm(g.start) })
})

const runTitle = computed(() => current.value?.title ?? projectNames.value.get(runEntry.value?.project_id ?? '') ?? t('Projektzeit'))
const runEst = computed(() => (current.value ? est(current.value) * 60 : 0))
const runPct = computed(() => (runEst.value ? Math.min(100, (runSecs.value / runEst.value) * 100) : 0))
const runRest = computed(() => {
  const over = runSecs.value - runEst.value
  return over > 0 ? t('{n} min über der Schätzung', { n: Math.floor(over / 60) }) : t('noch {n} min', { n: Math.ceil(-over / 60) })
})
const toggleTimer = (x: Task) => run(() => taskAction(x.id, runningTaskId.value === x.id ? 'pause' : 'start'))

const tracked = computed(() => today.value?.tracked_minutes ?? 0)
const pull = computed(() => upcoming.value[0])
const pullForward = (x: Task) => run(() => updateTask(x.id, { planned_date: today.value!.date }))
const openToday = computed(() => [...overdue.value, ...todays.value])
const reviewText = computed(() => {
  const [first, ...rest] = openToday.value
  if (!first) return t('Alles abgehakt. Zwei Sätze ins Journal, dann ist der Tag rund.')
  return rest.length ? t('„{title}“ ist offen geblieben, dazu {n} weitere.', { title: first.title, n: rest.length }) : t('„{title}“ ist offen geblieben.', { title: first.title })
})
function moveOpen() {
  const moved = openToday.value.map(({ id, planned_date, planned_start_at }) => ({ id, planned_date, planned_start_at }))
  const tomorrow = shiftDate(today.value!.date, 1)
  void run(async () => {
    await Promise.all(moved.map((x) => updateTask(x.id, { planned_date: tomorrow, planned_start_at: '' })))
    offer(tn(moved.length, '{n} Aufgabe auf morgen gelegt', '{n} Aufgaben auf morgen gelegt'), () =>
      run(() => Promise.all(moved.map((x) => updateTask(x.id, { planned_date: x.planned_date ?? '', planned_start_at: x.planned_start_at ?? '' })))))
  })
}

// Morgens ohne Plan: bis zu drei Aufgaben wählen, sie landen nacheinander in den ersten passenden Lücken.
const picks = ref(new Set<string>())
function togglePick(id: string) {
  const s = new Set(picks.value)
  if (s.has(id)) s.delete(id)
  else if (s.size < 3) s.add(id)
  picks.value = s
}
function planDay() {
  const busy = blocks.value.filter((b) => !b.done).map((b) => ({ start: b.start, end: b.end }))
  const from = Math.max(nowMin.value, prefs.dayStart * 60)
  const plan: { task: Task; at: number }[] = []
  for (const x of candidates.value.filter((c) => picks.value.has(c.id))) {
    const slot = findSlot(busy.sort((a, b) => a.start - b.start), from, prefs.dayStart * 60, prefs.dayEnd * 60, est(x))
    if (!slot) continue
    plan.push({ task: x, at: slot.start })
    busy.push({ start: slot.start, end: slot.start + est(x) + 15 }) // 15 Minuten Luft zwischen den Blöcken
  }
  plan.forEach((p) => scheduleAt(p.task, p.at))
  picks.value = new Set()
}
function skipPlan() {
  planSkip.value = today.value?.date ?? ''
  store.set('kairo-plan-skip', planSkip.value)
}

// Einplanen/Ausplanen: ohne Uhrzeit in die erste passende Lücke, mit Uhrzeit wieder heraus.
function togglePlan(x: Task, at?: number) {
  if (at === undefined && x.planned_start_at) return run(() => updateTask(x.id, { planned_start_at: '' }))
  const start = at ?? fitGap(x)?.start
  if (start === undefined) error.value = t('Heute ist keine Lücke groß genug.')
  else scheduleAt(x, start)
}

// Gewohnheiten, Lernkarte, Ziel. Fehler beim Nachladen lassen die Karte nur leer.
const habits = computed(() => today.value?.habits ?? [])
// Offene zuerst, höchstens vier; der Rest steht unter Gewohnheiten.
const habitsShown = computed(() => [...habits.value].sort((a, b) => Number(a.done) - Number(b.done)).slice(0, 4))
const toggleHabit = (id: string, done: boolean) => run(() => (done ? uncompleteHabit(id, today.value!.date) : completeHabit(id, today.value!.date)))
const RING = 2 * Math.PI * 12
const cards = ref<Card[] | null>(null)
const flip = ref(false)
const goal = ref<Goal | null>(null)
onMounted(() => {
  dueCards().then((cs) => (cards.value = cs), () => (cards.value = []))
  goalToday().then((g) => (goal.value = g), () => {})
})
const card = computed(() => cards.value?.[0])
function grade(g: number) {
  const c = card.value
  if (!c) return
  flip.value = false
  void run(async () => {
    await reviewCard(c.key, g)
    cards.value = cards.value!.slice(1)
  })
}
// Vorhaben von gestern (dritte Journal-Frage) als Erinnerung mit Link auf den Eintrag.
const yesterday = ref<{ date: string; intention: string } | null>(null)
watch(() => today.value?.date, (day) => {
  if (!day) return
  const prev = shiftDate(day, -1)
  readNote(journalPath(prev)).then((n) => {
    const intention = parseEntry(n.content).answers[2]?.trim()
    yesterday.value = intention ? { date: prev, intention } : null
  }, () => (yesterday.value = null))
}, { immediate: true })
const goalSegs = computed(() => Math.round((goal.value?.progress?.percent ?? 0) / 12.5))

const openStrip = ref<'habits' | 'cards' | 'goal' | null>(null)

const habitSummary = computed(() => {
  const openCount = habits.value.filter((h) => !h.done).length
  if (openCount > 0) return tn(openCount, '{n} Gewohnheit offen', '{n} Gewohnheiten offen')
  if (habits.value.length) return t('Alle Gewohnheiten erledigt')
  return t('Keine Gewohnheiten')
})

const cardSummary = computed(() => {
  const count = cards.value?.length ?? 0
  if (count > 0) return tn(count, '{n} Karte wartet', '{n} Karten warten')
  return t('Keine Karten fällig')
})

const goalSummary = computed(() => {
  if (!goal.value) return t('Kein Ziel')
  if (goal.value.progress) return t('Ziel {p}%', { p: goal.value.progress.percent })
  return goal.value.title
})

// Liste mit Tastatur: J/K bewegen, X abhaken, S starten, P einplanen, Leertaste dreht die Lernkarte.
const heroId = computed(() => (kind.value === 'task' ? heroTask.value?.id : kind.value === 'run' ? current.value?.id : undefined))
const groups = computed(() => [
  { key: 'over', label: t('Überfällig'), rows: overdue.value },
  { key: 'today', label: t('Heute'), rows: todays.value },
  { key: 'soon', label: t('Demnächst'), rows: upcoming.value },
].map((g) => ({ ...g, rows: g.rows.filter((x) => x.id !== heroId.value).sort((a, b) => Number(!!a.planned_start_at) - Number(!!b.planned_start_at)) }))
  .filter((g) => g.rows.length))
const ids = computed(() => groups.value.flatMap((g) => g.rows.map((x) => x.id)))
const sel = ref<string | null>(null)
const showDone = ref(false)
const dueLabel = (key: string, x: Task) => (x.planned_start_at ? '' : key === 'today' ? '' : shortDay(dayOf(x)))

useShortcuts((e) => {
  const k = e.key.toLowerCase()
  const i = sel.value ? ids.value.indexOf(sel.value) : -1
  const x = sel.value ? tasksById.value.get(sel.value) : undefined
  const hero = current.value ?? heroTask.value
  if (k === 'j' || k === 'arrowdown') sel.value = ids.value[Math.min(ids.value.length - 1, i + 1)] ?? null
  else if (k === 'k' || k === 'arrowup') sel.value = ids.value[Math.max(0, i - 1)] ?? null
  else if (k === 'x' && x) {
    sel.value = ids.value[i + 1] ?? ids.value[i - 1] ?? null
    setDone(x, 'COMPLETED', run)
  } else if (k === 's' && (x || hero)) void toggleTimer((x ?? hero)!)
  else if (k === 'p' && (x || heroTask.value)) void togglePlan((x ?? heroTask.value)!)
  else if (k === 'escape' && sel.value) sel.value = null
  else if (k === ' ' && card.value && !(e.target as HTMLElement).closest('button, a')) flip.value = !flip.value
  else return
  e.preventDefault()
})

// Zeitplan: senkrechte Schiene über den sichtbaren Tagesbereich, 1,35 px je Minute.
const r0 = computed(() => prefs.dayStart * 60)
const r1 = computed(() => Math.max(prefs.dayEnd * 60, r0.value + 60))
// Zeitplan passt in die Fensterhöhe: Lagen in Prozent der Schiene, deren Höhe misst ein ResizeObserver (für kurze Blöcke).
const ry = (m: number) => ((Math.min(Math.max(m, r0.value), r1.value) - r0.value) / (r1.value - r0.value)) * 100
const rail = ref<HTMLElement | null>(null)
const railH = ref(0)
const ro = new ResizeObserver(([e]) => (railH.value = e!.contentRect.height))
watch(rail, (el, old) => { if (old) ro.unobserve(old); if (el) ro.observe(el) })
onUnmounted(() => ro.disconnect())
const isShort = (b: { start: number; end: number }) => ((b.end - b.start) / (r1.value - r0.value)) * railH.value < 44
const hours = computed(() => Array.from({ length: prefs.dayEnd - prefs.dayStart + 1 }, (_, i) => prefs.dayStart + i))
const railBlocks = computed(() => blocks.value.filter((b) => b.end > r0.value && b.start < r1.value))
// Überlappende Blöcke nebeneinander; kurze zählen mit 30 Minuten, so hoch sind sie mindestens lesbar.
const railCols = computed(() => columns(railBlocks.value, 30))
const colStyle = (key: string) => {
  const c = railCols.value.get(key) ?? { col: 0, cols: 1 }
  return { left: `calc(52px + (100% - 52px) * ${c.col / c.cols})`, width: `calc((100% - 52px) / ${c.cols} - ${c.cols > 1 ? 3 : 0}px)` }
}
// Je Lücke ein Vorschlag: die erste noch nicht verplante offene Aufgabe, die hineinpasst.
const gapSugs = computed(() => {
  const used = new Set<string>()
  return freeGaps.value.map((g) => {
    const s = openToday.value.concat(upcoming.value).find((x) => !x.planned_start_at && !used.has(x.id) && est(x) <= g.end - g.start)
    if (s) used.add(s.id)
    return s
  })
})
const selectedTask = computed(() => (sel.value ? tasksById.value.get(sel.value) : undefined))
const gapToFill = ref<{ start: number; end: number } | null>(null)
const tasksForGap = (g: { start: number; end: number }) =>
  openToday.value.concat(upcoming.value).filter((x) => !x.planned_start_at && est(x) <= g.end - g.start)

function onGapClick(g: { start: number; end: number }) {
  if (selectedTask.value) {
    if (est(selectedTask.value) <= g.end - g.start) {
      scheduleAt(selectedTask.value, g.start)
      sel.value = null
    }
    return
  }
  gapToFill.value = g
}

const drag = ref<Task | null>(null)
const dropAt = ref<number | null>(null)
const fits = (g: { start: number; end: number }) => !!drag.value && est(drag.value) <= g.end - g.start
function drop(g: { start: number; end: number }) {
  if (drag.value && fits(g)) scheduleAt(drag.value, g.start)
  drag.value = null
  dropAt.value = null
}
</script>

<template>
  <div class="view-inner st">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div class="st-grid">
      <div class="st-main">
        <div class="v-head">
          <h1 class="v-title">{{ todayFormatted }}</h1>
        </div>
        <div v-if="upcomingEvent" class="st-event-alert">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-cal" /></svg>
          <span>{{ upcomingEvent }}</span>
        </div>
        <RouterLink v-if="yesterday" class="st-journal" :to="{ name: 'journal', query: { date: yesterday.date } }">
          <span>{{ $t('Gestern hast du dir vorgenommen:') }}</span><b>{{ yesterday.intention }}</b>
        </RouterLink>

        <section class="st-hero" :aria-label="$t('Jetzt')">
          <template v-if="kind === 'run'">
            <div class="st-eyebrow">
              <span class="st-dot" :class="{ off: !runEntry }"></span><b>{{ runEntry ? $t('Läuft') : $t('Pausiert') }}</b>
              <template v-if="current?.project_id"><span>·</span><span class="chip" :style="{ '--chip-c': projectColor(current.project_id) }"><span class="cdot"></span>{{ projectNames.get(current.project_id) }}</span></template>
              <span v-if="current" class="st-pill">{{ $t('Schätzung {time}', { time: estLabel(current) }) }}</span>
            </div>
            <div class="st-title">{{ runTitle }}</div>
            <div class="st-clock mono">{{ stopwatch(runSecs) }}</div>
            <template v-if="current">
              <div class="st-prog" aria-hidden="true"><i :style="{ width: runPct + '%' }"></i></div>
              <div class="st-progt"><span>{{ $t('{n} von {m} min', { n: Math.floor(runSecs / 60), m: est(current) }) }}</span><span>{{ runRest }}</span></div>
              <div class="st-acts">
                <button type="button" class="btn btn-primary" @click="setDone(current!, 'COMPLETED', run)">{{ $t('Fertig') }}</button>
                <button type="button" class="btn btn-secondary" @click="toggleTimer(current!)">{{ runEntry ? $t('Pause') : $t('Fortsetzen') }}<kbd aria-hidden="true">S</kbd></button>
              </div>
            </template>
          </template>

          <template v-else-if="kind === 'plan'">
            <div class="st-eyebrow"><b>{{ $t('Noch kein Plan') }}</b></div>
            <div class="st-title">{{ $t('Was soll heute passieren?') }}</div>
            <p class="st-text">{{ $t('Wähle bis zu drei Aufgaben, Kairo legt sie in die Lücken.') }}</p>
            <div class="st-picks">
              <button v-for="x in candidates" :key="x.id" type="button" class="st-pick" role="checkbox" :aria-checked="picks.has(x.id)" @click="togglePick(x.id)">
                <span class="cb" :aria-checked="picks.has(x.id)"></span>
                <span class="st-pick-t">{{ x.title }}</span>
                <span v-if="x.project_id" class="chip" :style="{ '--chip-c': projectColor(x.project_id) }"><span class="cdot"></span>{{ projectNames.get(x.project_id) }}</span>
                <span class="st-est">{{ estLabel(x) }}</span>
              </button>
            </div>
            <div class="st-acts">
              <button type="button" class="btn btn-primary" :disabled="!picks.size" @click="planDay">{{ picks.size ? $t('Tag planen · {n}', { n: picks.size }) : $t('Tag planen') }}</button>
              <button type="button" class="btn btn-secondary" @click="skipPlan">{{ $t('Ohne Plan beginnen') }}</button>
            </div>
          </template>

          <template v-else-if="kind === 'review'">
            <div class="st-eyebrow"><b>{{ $t('Tagesrückblick · {time}', { time: hm(nowMin) }) }}</b></div>
            <div class="st-title">{{ $t('{n} erledigt, {time} erfasst', { n: doneToday.length, time: dur(tracked) }) }}</div>
            <p class="st-text">{{ reviewText }}</p>
            <div class="st-acts">
              <RouterLink :to="{ name: 'journal', query: { date: today?.date } }" class="btn btn-primary">{{ $t('Journal schreiben') }}</RouterLink>
              <button v-if="openToday.length" type="button" class="btn btn-secondary" @click="moveOpen">{{ $t('Offenes auf morgen') }}</button>
            </div>
          </template>

          <template v-else-if="kind === 'task' && heroTask">
            <div class="st-eyebrow">
              <b>{{ $t('Als Nächstes') }}</b>
              <template v-if="heroTask.project_id"><span>·</span><span class="chip" :style="{ '--chip-c': projectColor(heroTask.project_id) }"><span class="cdot"></span>{{ projectNames.get(heroTask.project_id) }}</span></template>
              <span>·</span><span>{{ estLabel(heroTask) }}</span>
              <span class="st-pill">{{ overdue.includes(heroTask) ? $t('Überfällig') : $t('Heute fällig') }}</span>
            </div>
            <div class="st-title">{{ heroTask.title }}</div>
            <p class="st-text">{{ heroFit }}</p>
            <div class="st-acts">
              <button type="button" class="btn btn-primary" @click="toggleTimer(heroTask)"><svg class="ic" viewBox="0 0 12 12" aria-hidden="true"><path d="M3 1.8v8.4L10 6z" fill="currentColor" /></svg>{{ $t('Starten') }}<kbd aria-hidden="true">S</kbd></button>
              <button type="button" class="btn btn-secondary" @click="togglePlan(heroTask)">{{ startOf(heroTask) !== undefined ? $t('Geplant {time}', { time: hm(startOf(heroTask)!) }) : $t('In Lücke legen') }}<kbd aria-hidden="true">P</kbd></button>
            </div>
          </template>

          <template v-else>
            <div class="st-eyebrow"><b>{{ $t('Heute') }}</b></div>
            <div class="st-title">{{ $t('Alles erledigt für heute.') }}</div>
            <p class="st-text">{{ $t('{n} abgehakt, {time} erfasst, noch {free} frei.', { n: doneToday.length, time: dur(tracked), free: dur(freeMin) }) }}</p>
            <div v-if="pull" class="st-acts"><button type="button" class="btn btn-secondary" @click="pullForward(pull)">{{ $t('„{title}“ vorziehen', { title: pull.title }) }}</button></div>
          </template>
        </section>

        <!-- Aufgaben direkt unter dem Timer bzw. unter dem Block „Vorgenommen“ -->
        <div class="st-groups">
          <div v-for="g in groups" :key="g.key" class="tgroup">
            <div class="lbl">
              <span>{{ g.label }}</span>
              <span class="count">{{ g.rows.length }}</span>
            </div>
            <div class="tasklist">
              <div
                v-for="x in g.rows" :key="x.id" class="st-row" :class="{ sel: sel === x.id, dragging: drag?.id === x.id }" draggable="true"
                @click="sel = x.id" @dragstart="(e) => { drag = x; e.dataTransfer?.setData('text/plain', x.id) }" @dragend="drag = null; dropAt = null"
              >
                <button type="button" class="cb ring" role="checkbox" aria-checked="false" :aria-label="$t('{title} erledigt', { title: x.title })" @click.stop="setDone(x, 'COMPLETED', run)"></button>
                <div class="st-row-t">
                  <span class="t">{{ x.title }}</span>
                  <span v-if="startOf(x) !== undefined" class="st-planned mono">{{ hm(startOf(x)!) }}</span>
                </div>
                <span class="pcol">
                  <span v-if="x.project_id" class="chip" :style="{ '--chip-c': projectColor(x.project_id) }">
                    <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
                  </span>
                </span>
                <span class="dur">{{ x.estimated_minutes ? dur(x.estimated_minutes) : '' }}</span>
                <span class="due">{{ dueLabel(g.key, x) }}</span>
                <div class="st-row-acts">
                  <button type="button" class="btn btn-secondary" @click.stop="toggleTimer(x)"><kbd aria-hidden="true">S</kbd>{{ $t('Starten') }}</button>
                  <button type="button" class="btn btn-secondary" @click.stop="togglePlan(x)"><kbd aria-hidden="true">P</kbd>{{ x.planned_start_at ? $t('Aus dem Plan') : $t('Einplanen') }}</button>
                </div>
              </div>
            </div>
          </div>

          <div v-if="doneToday.length" class="tgroup">
            <button type="button" class="lbl st-done-btn" :aria-expanded="showDone" @click="showDone = !showDone">
              <svg class="ic" :class="{ open: showDone }" viewBox="0 0 12 12" aria-hidden="true"><path d="M4.5 3 7.5 6 4.5 9" /></svg>
              <span>{{ $t('Erledigt') }}</span>
              <span class="count">{{ doneToday.length }}</span>
            </button>
            <div v-if="showDone" class="tasklist">
              <div v-for="x in doneToday" :key="x.id" class="st-row done">
                <button type="button" class="cb ring" role="checkbox" aria-checked="true" :aria-label="$t('Wieder öffnen')" :title="$t('Wieder öffnen')" @click="run(() => updateTask(x.id, { status: 'PLANNED' }))"></button>
                <div class="st-row-t"><span class="t">{{ x.title }}</span></div>
                <span class="pcol">
                  <span v-if="x.project_id" class="chip" :style="{ '--chip-c': projectColor(x.project_id) }">
                    <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
                  </span>
                </span>
                <span class="dur"></span>
                <span class="due"></span>
              </div>
            </div>
          </div>
          <div v-if="!groups.length && !doneToday.length" class="st-empty">{{ $t('Nichts für heute geplant.') }}</div>
        </div>

        <!-- Kompakte Zeile: Gewohnheiten, Lernkarte, Fokus-Ziel -->
        <div class="st-strip" role="region" :aria-label="$t('Tagesübersicht')">
          <button type="button" class="st-strip-btn" :class="{ open: openStrip === 'habits' }" :aria-expanded="openStrip === 'habits'" @click="openStrip = openStrip === 'habits' ? null : 'habits'">
            <svg class="ic" aria-hidden="true"><use href="#i-habits" /></svg>
            <span>{{ habitSummary }}</span>
          </button>
          <span class="st-strip-sep" aria-hidden="true">·</span>
          <button type="button" class="st-strip-btn" :class="{ open: openStrip === 'cards' }" :aria-expanded="openStrip === 'cards'" @click="openStrip = openStrip === 'cards' ? null : 'cards'">
            <svg class="ic" aria-hidden="true"><use href="#i-cards" /></svg>
            <span>{{ cardSummary }}</span>
          </button>
          <span class="st-strip-sep" aria-hidden="true">·</span>
          <button type="button" class="st-strip-btn" :class="{ open: openStrip === 'goal' }" :aria-expanded="openStrip === 'goal'" @click="openStrip = openStrip === 'goal' ? null : 'goal'">
            <svg class="ic" aria-hidden="true"><use href="#i-goal" /></svg>
            <span>{{ goalSummary }}</span>
          </button>
        </div>

        <div v-if="openStrip" class="st-strip-detail">
          <!-- 1. Gewohnheiten -->
          <section v-if="openStrip === 'habits'" class="st-card" :aria-label="$t('Gewohnheiten')">
            <div class="st-card-h">
              <RouterLink to="/habits">{{ $t('Gewohnheiten') }}</RouterLink>
              <span class="st-pill">{{ habits.filter((h) => h.done).length }}/{{ habits.length }} {{ $t('heute') }}</span>
            </div>
            <div class="st-card-b">
              <button v-for="h in habitsShown" :key="h.id" type="button" class="st-habit" :aria-pressed="h.done" @click="toggleHabit(h.id, h.done)">
                <svg width="24" height="24" viewBox="0 0 30 30" aria-hidden="true">
                  <circle cx="15" cy="15" r="12" class="st-ring-bg" />
                  <circle cx="15" cy="15" r="12" class="st-ring" :stroke-dasharray="RING" :stroke-dashoffset="h.done ? 0 : RING" transform="rotate(-90 15 15)" />
                  <path d="M10.5 15.3l3 3 6-6.4" class="st-ring-chk" :style="{ opacity: h.done ? 1 : 0 }" />
                </svg>
                <span class="st-habit-n">{{ h.name }}</span>
                <span class="st-habit-m">{{ h.week_progress ? $t('{n} von {m}', { n: h.week_progress.done, m: h.week_progress.target }) : h.preferred_time ?? '' }}</span>
              </button>
              <RouterLink v-if="habits.length > 3" to="/habits" class="st-more">{{ $t('{n} weitere', { n: habits.length - 3 }) }}</RouterLink>
              <span v-if="!habits.length" class="st-empty">{{ $t('Heute keine Gewohnheiten.') }}</span>
            </div>
            <div class="st-card-f">
              <span v-if="habits.length && habits.every((h) => h.done)" class="st-all-done">✓ {{ $t('Alles für heute erledigt') }}</span>
              <span v-else-if="habits.length === 1 && habits[0]?.preferred_time" class="st-meta-note">{{ $t('Geplant um {time} Uhr', { time: habits[0].preferred_time }) }}</span>
              <span v-else-if="habits.length" class="st-meta-note">{{ $tn(habits.filter((h) => !h.done).length, '{n} noch offen', '{n} noch offen') }}</span>
            </div>
          </section>

          <!-- 2. Lernkarte -->
          <section v-else-if="openStrip === 'cards'" class="st-card" :aria-label="$t('Lernkarte')">
            <div class="st-card-h">
              <RouterLink to="/cards">{{ $t('Lernkarte') }}</RouterLink>
              <span v-if="card?.deck" class="st-pill">{{ card.deck }}</span>
              <span v-else-if="cards?.length" class="st-pill">{{ $t('{n} fällig', { n: cards.length }) }}</span>
            </div>
            <div v-if="card" class="st-card-b">
              <div class="st-flip" :class="{ on: flip }" @click="flip = !flip">
                <div class="st-face st-front" :aria-hidden="flip">
                  <span class="st-q">{{ card.question }}</span>
                </div>
                <div class="st-face st-back" :aria-hidden="!flip">
                  <span class="st-a">{{ card.answer }}</span>
                  <div class="st-grade">
                    <button type="button" class="btn btn-secondary" @click.stop="grade(0)">{{ $t('Nochmal') }}</button>
                    <button type="button" class="btn btn-primary" @click.stop="grade(2)">{{ $t('Gewusst') }}</button>
                  </div>
                </div>
              </div>
            </div>
            <div v-else-if="cards" class="st-card-b">
              <div class="st-empty"><b>{{ $t('Für heute gelernt.') }}</b>{{ $t('Die nächste Karte kommt, wenn sie fällig ist.') }}</div>
            </div>
            <div v-if="card" class="st-card-f">
              <span class="st-meta-note">{{ $tn(cards?.length ?? 0, '{n} Karte wartet', '{n} Karten warten') }}</span>
              <span class="st-hint"><kbd>{{ $t('Leertaste') }}</kbd> {{ $t('Umdrehen') }}</span>
            </div>
          </section>

          <!-- 3. Ziel -->
          <section v-else-if="openStrip === 'goal'" class="st-card" :aria-label="$t('Ziel')">
            <div class="st-card-h">
              <RouterLink to="/goals">{{ $t('Fokus-Ziel') }}</RouterLink>
              <span v-if="goal?.due_date" class="st-pill">{{ shortDate(goal.due_date) }}</span>
              <span v-else-if="goal?.progress" class="st-pill">{{ goal.progress.percent }}%</span>
              <span v-else class="st-pill">{{ $t('Langzeitziel') }}</span>
            </div>
            <div v-if="goal" class="st-card-b">
              <span class="st-goal-t">{{ goal.title }}</span>
              <p v-if="goal.why" class="st-goal-why">„{{ goal.why }}“</p>
              <div v-if="goal.progress" class="st-segs" aria-hidden="true"><i v-for="n in 8" :key="n" :class="{ on: n <= goalSegs }"></i></div>
            </div>
            <div v-else class="st-card-b">
              <span class="st-empty">{{ $t('Noch kein Ziel.') }} <RouterLink to="/goals">{{ $t('Ziel anlegen ›') }}</RouterLink></span>
            </div>
            <div class="st-card-f">
              <template v-if="goal">
                <div v-if="goal.progress" class="st-goal-f-line"><span>{{ $t('Fortschritt') }}</span><b>{{ goalProgressText(goal) }}</b></div>
                <RouterLink v-else to="/goals" class="st-goal-link">{{ $t('Teilziele anlegen ›') }}</RouterLink>
              </template>
            </div>
          </section>
        </div>

        <div class="st-keys" aria-hidden="true">
          <span><kbd>J</kbd><kbd>K</kbd>{{ $t('bewegen') }}</span>
          <span><kbd>X</kbd>{{ $t('erledigt') }}</span>
          <span><kbd>S</kbd>{{ $t('starten') }}</span>
          <span><kbd>P</kbd>{{ $t('einplanen') }}</span>
          <span>{{ $t('Zeile in eine Lücke ziehen') }}</span>
        </div>
      </div>

      <aside class="st-side" :aria-label="$t('Zeitplan')">
        <div class="st-side-h"><span>{{ $t('Zeitplan') }}</span><span class="st-pill">{{ $t('{time} frei', { time: dur(freeMin) }) }}</span></div>
        <div ref="rail" class="st-rail" :style="{ '--mins': r1 - r0 }">
          <div v-for="h in hours" :key="h" class="st-hour" :style="{ top: ry(h * 60) + '%' }"><span class="mono">{{ String(h).padStart(2, '0') }}:00</span><i></i></div>
          <div
            v-for="(g, i) in freeGaps" :key="g.start" class="st-gap" :class="{ can: fits(g) || (selectedTask && est(selectedTask) <= g.end - g.start), on: dropAt === g.start && fits(g) }"
            :style="{ top: ry(g.start) + '%', height: ry(g.end) - ry(g.start) + '%' }"
            @dragover="(e) => { if (fits(g)) { e.preventDefault(); dropAt = g.start } }" @dragleave="dropAt = null" @drop.prevent="drop(g)"
            @click="onGapClick(g)"
          >
            <div>
              <span>{{ drag ? (fits(g) ? $t('Hier ablegen') : $t('Zu kurz')) : selectedTask ? (est(selectedTask) <= g.end - g.start ? $t('Hier einplanen') : $t('Zu kurz')) : $t('{time} frei', { time: dur(g.end - g.start) }) }}</span>
              <button v-if="!drag && !selectedTask" type="button" class="st-gap-add" :title="gapSugs[i] ? $t('{title} um {time} einplanen', { title: gapSugs[i]!.title, time: hm(g.start) }) : $t('Aufgabe einplanen')" @click.stop="gapToFill = g">+ {{ $t('Einplanen') }}</button>
            </div>
          </div>
          <div
            v-for="b in railBlocks" :key="b.key" class="st-block" :class="[b.kind, { short: isShort(b), past: b.end <= nowMin, cur: b.start <= nowMin && nowMin < b.end }]"
            :style="{ ...colStyle(b.key), top: ry(b.start) + '%', height: ry(b.end) - ry(b.start) + '%', '--c': b.kind === 'event' ? 'var(--a-event)' : b.color }"
          >
            <div>
              <span class="st-block-t" :class="{ done: b.done }">{{ b.title }}</span>
              <span class="st-block-m mono">{{ isShort(b) ? hm(b.start) : b.start <= nowMin && nowMin < b.end ? $t('bis {time} · noch {left}', { time: hm(b.end), left: dur(b.end - nowMin) }) : `${hm(b.start)}–${hm(b.end)}` }}</span>
              <button v-if="b.task && !b.done && b.start >= nowMin" type="button" class="st-block-x" :aria-label="$t('Aus dem Plan nehmen')" :title="$t('Aus dem Plan nehmen')" @click="togglePlan(b.task)">×</button>
            </div>
          </div>
          <div v-if="nowMin >= r0 && nowMin <= r1" class="st-now" :style="{ top: ry(nowMin) + '%' }"><span class="mono">{{ hm(nowMin) }}</span></div>
        </div>
      </aside>
    </div>

    <!-- Dialog zur Lückenauswahl (Mobil & Touch) -->
    <div v-if="gapToFill" v-dialog="() => gapToFill = null" class="overlay open" @click.self="gapToFill = null">
      <div class="dialog st-gap-dialog">
        <div class="dlg-head">
          <h3>{{ $t('Aufgabe einplanen') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="gapToFill = null"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="st-gap-dialog-info">
            <span class="mono">{{ hm(gapToFill.start) }} – {{ hm(gapToFill.end) }}</span>
            <span>· {{ $t('{time} frei', { time: dur(gapToFill.end - gapToFill.start) }) }}</span>
          </div>
          <div v-if="tasksForGap(gapToFill).length" class="st-gap-tasks">
            <button
              v-for="x in tasksForGap(gapToFill)" :key="x.id"
              type="button" class="st-gap-task-item"
              @click="scheduleAt(x, gapToFill.start); gapToFill = null"
            >
              <span class="t">{{ x.title }}</span>
              <span v-if="x.project_id" class="chip" :style="{ '--chip-c': projectColor(x.project_id) }">
                <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
              </span>
              <span class="dur mono">{{ estLabel(x) }}</span>
            </button>
          </div>
          <div v-else class="st-empty">
            {{ $t('Keine passende offene Aufgabe für diese Lücke.') }}
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" @click="gapToFill = null">{{ $t('Abbrechen') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.st { max-width: 1240px; padding-top: 36px; }
.st-grid { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 48px; align-items: start; }
.st-main { min-width: 0; display: flex; flex-direction: column; }
.st .v-sub { margin-top: 8px; font-variant-numeric: tabular-nums; }
kbd { display: inline-grid; place-items: center; min-width: 18px; height: 18px; padding: 0 4px; box-sizing: border-box; border-radius: 5px; background: color-mix(in oklab, currentColor 12%, transparent); font: 500 11px/1 var(--font-mono); }
.st .btn kbd { margin-left: 4px; opacity: .8; }
.st-pill { display: inline-flex; align-items: center; gap: 6px; height: 20px; padding: 0 8px; border-radius: 6px; background: var(--bg-2); box-shadow: inset 0 0 0 1px var(--br-subtle); font: 500 11.5px/1 var(--font-ui); color: var(--tx-secondary); white-space: nowrap; font-variant-numeric: tabular-nums; }
.st-journal { display: flex; align-items: baseline; gap: 8px; margin-top: 10px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); text-decoration: none; }
.st-journal b { min-width: 0; font-weight: 500; color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.st-journal:hover b { text-decoration: underline; }

/* Karte oben */
.st-event-alert { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; padding: 7px 12px; border-radius: var(--r-s); background: color-mix(in srgb, var(--a-blue) 12%, transparent); border: 1px solid color-mix(in srgb, var(--a-blue) 25%, transparent); font: 500 12.5px/1.3 var(--font-ui); color: var(--tx-primary); }
.st-event-alert .ic { width: 14px; height: 14px; color: var(--a-blue); flex: none; }
.st-hero { margin-top: 8px; padding: 20px 24px 18px; border-radius: 14px; background: linear-gradient(180deg, var(--bg-3), var(--bg-2) 60%); box-shadow: inset 0 0 0 1px var(--br-subtle); }
.st-eyebrow { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; font: 400 12.5px/1.3 var(--font-ui); color: var(--tx-muted); }
.st-eyebrow b { font-weight: 500; color: var(--tx-secondary); }
.st-eyebrow .st-pill { margin-left: auto; }
.st-eyebrow-proj { display: inline-flex; align-items: center; gap: 5px; color: var(--tx-secondary); }
.st-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--a-run); }
.st-dot.off { background: var(--br-strong); }
.st-title { margin-top: 8px; font: 600 22px/1.25 var(--font-ui); letter-spacing: -0.022em; color: var(--tx-primary); text-wrap: pretty; }
.st-text { margin-top: 6px; font: 400 13.5px/1.45 var(--font-ui); color: var(--tx-secondary); text-wrap: pretty; }
.st-acts { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 16px; }
.st-acts .btn { height: 34px; padding: 0 13px; gap: 8px; text-decoration: none; font-size: 13px; }
.st-acts .ic { width: 12px; height: 12px; }
.st-clock { margin-top: 8px; font-size: 36px; font-weight: 500; letter-spacing: -0.02em; color: var(--tx-primary); }
.st-prog { height: 4px; margin-top: 12px; border-radius: 2px; background: var(--br-subtle); }
.st-prog i { display: block; height: 4px; border-radius: 2px; background: var(--a-blue-hi); transition: width 1s linear; }
.st-progt { display: flex; justify-content: space-between; margin-top: 6px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); font-variant-numeric: tabular-nums; }
.st-picks { display: grid; margin-top: 14px; }
.st-pick { display: grid; grid-template-columns: 16px minmax(0, 1fr) auto 56px; align-items: center; gap: 12px; height: 38px; padding: 0 10px; margin: 0 -10px; border: 0; border-radius: 8px; background: none; color: var(--tx-primary); font: 400 13.5px/1.3 var(--font-ui); text-align: left; cursor: pointer; }
.st-pick:hover { background: var(--bg-hover); }
.st-pick .cb { pointer-events: none; }
.st-pick-t { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Kompakte Zeile für Gewohnheiten, Lernkarte, Fokus-Ziel */
.st-strip { display: inline-flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; margin: 24px 0 12px; padding: 6px 14px; border-radius: var(--r-s); background: var(--bg-1); border: 1px solid var(--br-subtle); font: 400 13px/1.3 var(--font-ui); color: var(--tx-secondary); }
.st-strip-btn { display: inline-flex; align-items: center; gap: 6px; padding: 4px 8px; border: 0; border-radius: 6px; background: none; font: 400 13px/1.2 var(--font-ui); color: var(--tx-secondary); cursor: pointer; transition: background var(--dur) ease-out, color var(--dur) ease-out; }
.st-strip-btn:hover, .st-strip-btn.open { background: var(--bg-2); color: var(--tx-primary); }
.st-strip-btn .ic { width: 14px; height: 14px; color: var(--tx-muted); }
.st-strip-btn.open .ic { color: var(--a-blue-hi); }
.st-strip-sep { color: var(--tx-muted); user-select: none; }
.st-strip-detail { margin: 8px 0 20px; max-width: 440px; }

/* Drei Karten */
.st-cards { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }
.st-more { margin-top: auto; font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); text-decoration: none; }
.st-more:hover { color: var(--tx-primary); }
.st-card { display: flex; flex-direction: column; justify-content: space-between; gap: 8px; min-width: 0; min-height: 140px; padding: 14px 16px; border-radius: var(--r-m); background: var(--bg-1); border: 1px solid var(--br-default); transition: border-color var(--dur) ease-out; }
.st-card:hover { border-color: var(--br-strong); }
.st-card-h { display: flex; align-items: center; justify-content: space-between; height: 20px; font: 600 12px/1 var(--font-ui); }
.st-card-h a { color: var(--tx-secondary); text-decoration: none; }
.st-card-h a:hover { color: var(--tx-primary); }
.st-card-b { display: flex; flex-direction: column; gap: 6px; min-width: 0; flex: 1; justify-content: center; }
.st-card-f { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 18px; margin-top: auto; padding-top: 6px; font: 400 11.5px/1.2 var(--font-ui); color: var(--tx-muted); }
.st-meta-note { color: var(--tx-muted); font-size: 11.5px; }
.st-all-done { color: var(--a-green); font-weight: 500; font-size: 11.5px; }
.st-goal-link { color: var(--tx-secondary); text-decoration: none; font-size: 11.5px; }
.st-goal-link:hover { color: var(--tx-primary); text-decoration: underline; }
.st-goal-why { margin: 0; font: italic 400 12.5px/1.35 var(--font-serif); color: var(--tx-secondary); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.st-goal-f-line { display: flex; align-items: center; justify-content: space-between; width: 100%; font-size: 11.5px; }
.st-goal-f-line b { font-weight: 500; color: var(--tx-primary); }
.st-empty { display: grid; gap: 4px; font: 400 12.5px/1.4 var(--font-ui); color: var(--tx-muted); }
.st-empty b { font-weight: 500; color: var(--tx-primary); font-size: 13.5px; }
.st-empty a { color: var(--a-blue-tx); }
.st-habit { display: flex; align-items: center; gap: 10px; height: 34px; padding: 0 6px 0 2px; margin: 0 -6px 0 -2px; border: 0; border-radius: 8px; background: none; color: var(--tx-secondary); cursor: pointer; text-align: left; }
.st-habit:hover { background: var(--bg-hover); }
.st-habit svg { flex: none; width: 22px; height: 22px; }
.st-ring-bg { fill: none; stroke: var(--br-default); stroke-width: 2.4; }
.st-ring { fill: none; stroke: var(--a-blue-hi); stroke-width: 2.4; stroke-linecap: round; transition: stroke-dashoffset 520ms cubic-bezier(.3, .7, .2, 1); }
.st-ring-chk { fill: none; stroke: var(--a-blue-hi); stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; transition: opacity 200ms; }
.st-habit[aria-pressed="true"] .st-habit-n { color: var(--tx-primary); }
.st-habit-n { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: 500 13.5px/1.3 var(--font-ui); }
.st-habit-m { margin-left: auto; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); white-space: nowrap; font-variant-numeric: tabular-nums; }
.st-flip { position: relative; flex: 1; min-height: 64px; perspective: 900px; cursor: pointer; display: flex; flex-direction: column; justify-content: center; }
.st-face { position: absolute; inset: 0; display: flex; flex-direction: column; justify-content: center; gap: 4px; backface-visibility: hidden; transition: transform 520ms cubic-bezier(.3, .7, .2, 1); }
.st-back { transform: rotateY(180deg); justify-content: space-between; }
.st-flip.on .st-front { transform: rotateY(-180deg); }
.st-flip.on .st-back { transform: none; }
.st-deck { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
.st-q, .st-a { font: 500 14px/1.35 var(--font-ui); color: var(--tx-primary); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.st-a { font-size: 13.5px; font-weight: 400; }
.st-hint { display: inline-flex; align-items: center; gap: 5px; font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.st-grade { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 6px; }
.st-grade .btn { height: 28px; font-size: 12px; }
.st-goal-t { font: 500 14px/1.35 var(--font-ui); color: var(--tx-primary); }
.st-segs { display: grid; grid-template-columns: repeat(8, 1fr); gap: 3px; margin-top: 4px; }
.st-segs i { height: 4px; border-radius: 2px; background: var(--br-default); transition: background 300ms; }
.st-segs i.on { background: var(--tx-secondary); }

/* Aufgaben-Liste (minimalistisch wie Aufgaben-Ansicht) */
.st-groups { margin-top: 24px; }
.tgroup { display: flex; flex-direction: column; }
.tgroup > .lbl { display: flex; align-items: center; gap: 8px; height: 28px; margin-bottom: 4px; padding: 0 4px; font: 600 14.5px/1.3 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }
.tgroup > .lbl .count { font: 400 13px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.st-done-btn { border: 0; background: none; cursor: pointer; text-align: left; }
.st-done-btn .ic { width: 12px; height: 12px; fill: none; stroke: currentColor; stroke-width: 1.5; color: var(--tx-muted); transition: transform 160ms; }
.st-done-btn .ic.open { transform: rotate(90deg); }
.tasklist { display: flex; flex-direction: column; }
.st-row { position: relative; display: grid; grid-template-columns: 20px minmax(0, 1fr) auto 64px 84px; align-items: center; column-gap: 14px; min-height: 42px; padding: 6px 12px; margin: 0 -12px; border-radius: var(--r-s); cursor: grab; transition: background 120ms; border: 0; }
.st-row:hover { background: color-mix(in oklab, var(--tx-primary) 4%, transparent); }
.st-row.sel { background: var(--bg-2); }
.st-row.sel::before { content: ""; position: absolute; left: 0; top: 8px; bottom: 8px; width: 2.5px; border-radius: 0 2px 2px 0; background: var(--tx-primary); }
.st-row.dragging { opacity: .45; }
.st-row .cb { width: 18px; height: 18px; flex: none; border-radius: 50%; }
.st-row-t { display: flex; align-items: center; gap: 8px; min-width: 0; }
.st-row .t { font: 500 14px/1.35 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.st-row.done .t { color: var(--tx-muted); text-decoration: line-through; }
.st-planned { display: inline-flex; align-items: center; height: 18px; padding: 0 5px; border-radius: 4px; background: var(--bg-2); font-size: 11px; color: var(--tx-muted); flex: none; }
.pcol { display: flex; justify-content: flex-end; min-width: 0; }
.chip { display: inline-flex; align-items: center; height: 22px; padding: 0 9px; border-radius: 7px; font-size: 12px; font-weight: 500; background: color-mix(in srgb, var(--chip-c) 15%, transparent); color: color-mix(in srgb, var(--chip-c) 80%, var(--tx-primary)); white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
.chip .cdot { display: none; }
.dur, .due { font: 400 12.5px/1 var(--font-ui); color: var(--tx-secondary); text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.st-row-acts { position: absolute; right: 6px; top: 5px; bottom: 5px; display: flex; align-items: center; gap: 6px; padding-left: 28px; background: linear-gradient(90deg, transparent, var(--bg-0) 18px); opacity: 0; pointer-events: none; transition: opacity 120ms; }
.st-row:hover .st-row-acts, .st-row.sel .st-row-acts, .st-row:focus-within .st-row-acts { opacity: 1; pointer-events: auto; }
.st-row-acts .btn { height: 26px; padding: 0 9px 0 6px; gap: 6px; font-size: 12px; }
.st-row-acts kbd { margin: 0 !important; }
.st-keys { display: flex; flex-wrap: wrap; gap: 8px 20px; margin-top: 24px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.st-keys span { display: flex; align-items: center; gap: 5px; }

/* Zeitplan */
/* Ganze Fensterhöhe unter der Kopfleiste (56px) abzüglich Innenabstand, damit der Zeitplan ohne Scrollen passt */
.st-side { position: sticky; top: 92px; display: flex; flex-direction: column; height: calc(100dvh - 56px - 36px - 32px); min-height: 420px; min-width: 0; }
.st-side-h { display: flex; align-items: center; justify-content: space-between; height: 28px; padding-bottom: 8px; border-bottom: 1px solid var(--br-subtle); font: 600 15px/1 var(--font-ui); color: var(--tx-primary); }
.st-rail { position: relative; flex: 1; margin: 18px 0 8px; }
.st-hour { position: absolute; left: 0; right: 0; display: flex; align-items: center; gap: 10px; transform: translateY(-50%); }
.st-hour span { width: 40px; font-size: 12px; color: var(--tx-muted); }
.st-hour i { flex: 1; height: 1px; background: var(--br-subtle); }
.st-gap, .st-block { position: absolute; left: 52px; right: 0; padding: 2px 0; box-sizing: border-box; }
.st-gap > div { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; height: 100%; box-sizing: border-box; padding: 5px 5px 0 11px; border: 1px dashed transparent; border-left: 1.5px dashed var(--br-default); border-radius: 4px 8px 8px 4px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); transition: background 160ms, border-color 160ms; }
.st-gap > div > span { padding-top: 3px; white-space: nowrap; font-variant-numeric: tabular-nums; }
.st-gap:hover > div { border-color: var(--br-default); background: color-mix(in oklab, var(--tx-primary) 2.5%, transparent); color: var(--tx-muted); }
.st-gap.can > div { border-color: var(--br-strong); color: var(--tx-muted); }
.st-gap.on > div { border-color: var(--a-blue-hi); background: color-mix(in oklab, var(--a-blue-hi) 14%, transparent); color: var(--a-blue-tx); }
.st-gap { cursor: pointer; }
.st-gap-add { flex: none; height: 22px; padding: 0 9px; border: 0; border-radius: 11px; background: color-mix(in oklab, var(--a-blue) 84%, #000); color: #fff; font: 600 11.5px/1 var(--font-ui); cursor: pointer; white-space: nowrap; opacity: 0; transform: translateY(2px); transition: opacity 120ms, transform 120ms; }
.st-gap:hover .st-gap-add, .st-gap-add:focus-visible { opacity: 1; transform: none; }
.st-gap-dialog { width: 420px; }
.st-gap-dialog-info { display: flex; align-items: center; gap: 8px; font: 500 13px/1.2 var(--font-ui); color: var(--tx-secondary); margin-bottom: 6px; }
.st-gap-tasks { display: flex; flex-direction: column; gap: 6px; max-height: 280px; overflow-y: auto; }
.st-gap-task-item { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 10px; padding: 8px 10px; border-radius: var(--r-s); border: 1px solid var(--br-subtle); background: var(--bg-2); color: var(--tx-primary); text-align: left; cursor: pointer; font: 500 13px/1.3 var(--font-ui); transition: background 120ms; }
.st-gap-task-item:hover { background: var(--bg-hover); border-color: var(--br-default); }
.st-gap-task-item .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.st-block > div { position: relative; height: 100%; box-sizing: border-box; display: flex; flex-direction: column; gap: 3px; padding: 6px 10px; overflow: hidden; border-left: 3px solid var(--c); border-radius: 4px 8px 8px 4px; background: color-mix(in oklab, var(--c) 7%, var(--bg-2)); }
.st-block.cur > div { background: color-mix(in oklab, var(--c) 11%, var(--bg-2)); }
.st-block.past { opacity: .45; }
.st-block.short > div { flex-direction: row; align-items: center; justify-content: space-between; gap: 8px; padding-top: 0; padding-bottom: 0; }
.st-block.short .st-block-t { flex: 1; min-width: 0; padding-right: 0; }
.st-block.short .st-block-x { position: static; flex: none; }
.st-block-t { font: 500 13px/1.25 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding-right: 18px; }
.st-block-t.done { text-decoration: line-through; }
.st-block-m { font-size: 11.5px; color: var(--tx-muted); white-space: nowrap; }
.st-block-x { position: absolute; top: 4px; right: 4px; width: 20px; height: 20px; padding: 0; border: 0; border-radius: 5px; background: none; color: var(--tx-muted); font-size: 14px; line-height: 1; cursor: pointer; opacity: 0; }
.st-block:hover .st-block-x, .st-block-x:focus-visible { opacity: 1; }
.st-block-x:hover { background: var(--bg-3); color: var(--tx-primary); }
.st-now { position: absolute; left: 0; right: 0; height: 1.5px; margin-top: -0.75px; background: linear-gradient(90deg, transparent 44px, var(--a-blue-hi) 44px); pointer-events: none; z-index: 1; }
.st-now span { position: absolute; left: 0; top: -8px; padding: 1px 4px; border-radius: 4px; background: var(--a-blue-hi); color: var(--bg-0); font-size: 11px; }

@media (prefers-reduced-motion: reduce) {
  .st-face, .st-ring, .st-prog i { transition: none; }
}
@media (max-width: 1100px) {
  .st-cards { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
}
@media (max-width: 960px) {
  .st-grid { grid-template-columns: minmax(0, 1fr); gap: 32px; }
  .st-side { position: static; height: auto; }
  .st-rail { flex: none; height: calc(var(--mins) * 1.35px); }
  .st-gap-add { opacity: 1; transform: none; }
}
@media (max-width: 640px) {
  .st { padding-top: 20px; }
  .st-hero { padding: 20px; }
  .st-title { font-size: 20px; }
  .st-cards { grid-template-columns: minmax(0, 1fr); }
  .st-row { grid-template-columns: 16px minmax(0, 1fr) 56px; }
  .st-proj, .st-due, .st-keys { display: none; }
}
</style>
