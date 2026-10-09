<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  completeHabit, createTask, taskAction, createTimeEntry, deleteTimeEntry, getToday, listProjects, listTasks, listTimeEntries, uncompleteHabit, updateTask, updateTimeEntry,
  isOpen, type Project, type Task, type TimeEntry, type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { daysAgo, dur, entryMinutes, hhmm, hm, ymd } from '@/lib/dates'
import { parseQuickAdd, taskBody } from '@/lib/quickAdd'
import { columns, doneSeconds, minutesOfDay, stopwatch } from '@/lib/dayPlan'
import QuickHints from '@/components/QuickHints.vue'
import { habitColor, projectColor } from '@/lib/projectColor'
import TaskActions from '@/components/TaskActions.vue'
import { locale, t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

// Planungsfenster der Tagesleiste und des Tagesplans.
const START_H = 7
const END_H = 22
const HOUR = 44

const { offer, setDone } = useUndo()
const today = ref<Today | null>(null)
const now = ref(Date.now())
const quick = ref('')
const quickEl = ref<HTMLInputElement>()
const manualOpen = ref(false)
const entries = ref<TimeEntry[]>([])
const tasksById = ref(new Map<string, Task>())
const taskTitles = computed(() => new Map([...tasksById.value].map(([id, t]) => [id, t.title])))
const projectNames = ref(new Map<string, string>())
const projectList = ref<Project[]>([])
let tick: ReturnType<typeof setInterval> | undefined

const { error, load, run } = useLoader(async () => {
  const t = await getToday()
  const from = new Date(`${t.date}T00:00:00`)
  const [es, tasks, projects] = await Promise.all([listTimeEntries(from, new Date(from.getTime() + 864e5)), listTasks(), listProjects()])
  today.value = t
  entries.value = es.reverse()
  tasksById.value = new Map(tasks.map((x) => [x.id, x]))
  projectNames.value = new Map(projects.map((x) => [x.id, x.name]))
  projectList.value = projects.filter((p) => p.status !== 'ARCHIVED') // für #name in der Schnelleingabe
})

// n springt in die Schnelleingabe.
useShortcuts((e) => {
  if (e.key !== 'n') return
  e.preventDefault()
  quickEl.value?.focus()
})

useLiveEvents(load)
onMounted(() => {
  void load()
  tick = setInterval(() => (now.value = Date.now()), 1000)
})
onUnmounted(() => clearInterval(tick))

// Das Backend meldet ohne konfigurierte Zeitzone "Local"; das ist kein gültiger IANA-Name.
const tz = computed(() => (today.value?.timezone === 'Local' ? undefined : today.value?.timezone))
const fmt = (iso: string) => new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit', timeZone: tz.value }).format(new Date(iso))
// Minuten seit Mitternacht in der Zeitzone des Backends.
const minsOf = (ms: number) => minutesOfDay(ms, tz.value)

const dateLabel = computed(() =>
  new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date(now.value)))

const nowMin = computed(() => minsOf(now.value))
const runEntry = computed(() => today.value?.running_time_entry ?? null)
const runningTaskId = computed(() => runEntry.value?.task_id ?? null)
// Zuletzt pausierte Task von heute: bleibt in der Karte, damit man sie fortsetzen kann.
const pausedTask = computed(() => {
  if (runEntry.value) return undefined
  const e = [...entries.value].reverse().find((x) => x.task_id && tasksById.value.get(x.task_id)?.status === 'PAUSED')
  return e ? tasksById.value.get(e.task_id!) : undefined
})
// Heute schon abgeschlossene Zeit der Task, damit der Timer nach Pause und Fortsetzen weiterzählt.
const doneSecs = (id: string) => doneSeconds(entries.value, id)
const runSecs = computed(() => {
  const e = runEntry.value
  if (e) return (e.task_id ? doneSecs(e.task_id) : 0) + Math.max(0, (now.value - Date.parse(e.started_at)) / 1000)
  return pausedTask.value ? doneSecs(pausedTask.value.id) : 0
})
const elapsed = computed(() => stopwatch(runSecs.value))

interface Block { key: string; title: string; start: number; end: number; kind: 'event' | 'task'; done: boolean; running?: boolean; color: string; sub: string }

// Termine und Tasks mit Uhrzeit als Blöcke; Tasks ohne Dauer zählen 30 Minuten.
const allBlocks = computed<Block[]>(() => {
  const t = today.value
  if (!t) return []
  const out: Block[] = t.events.map((e) => ({
    key: `e${e.id}${e.occurrence_start}`, title: e.title, kind: 'event', done: false, color: 'var(--p-neutral)', sub: e.location,
    start: minsOf(Date.parse(e.occurrence_start)), end: minsOf(Date.parse(e.occurrence_end)) || 24 * 60,
  }))
  for (const task of t.tasks) {
    if (!task.planned_start_at) continue
    const start = minsOf(Date.parse(task.planned_start_at))
    out.push({
      key: `t${task.id}`, title: task.title, kind: 'task', done: task.status === 'COMPLETED', running: task.id === runningTaskId.value, sub: '',
      color: task.project_id ? projectColor(task.project_id) : 'var(--p-neutral)', start, end: start + (task.estimated_minutes || 30),
    })
  }
  return out.sort((a, b) => a.start - b.start)
})

// Ganztägige Termine stehen im Kopf, nicht im Raster (und zählen nicht für die Spalten bei Überlappung).
const isAllDay = (b: Block) => b.kind === 'event' && b.start === 0 && b.end >= 24 * 60
const allDay = computed(() => allBlocks.value.filter(isAllDay))
const blocks = computed(() => allBlocks.value.filter((b) => !isAllDay(b)))
const upcoming = computed(() => blocks.value.filter((b) => b.kind === 'event' && !b.done))
const nextEvent = computed(() => upcoming.value.find((b) => b.start > nowMin.value))

const openTasks = computed(() => (today.value?.tasks ?? []).filter(isOpen))
const runningTask = computed(() => (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined))
const cardTask = computed(() => runningTask.value ?? pausedTask.value)
const runProject = computed(() => cardTask.value?.project_id ?? runEntry.value?.project_id ?? null)
const runGoal = computed(() => cardTask.value?.estimated_minutes || 0)
const runPct = computed(() => (runGoal.value ? Math.min(100, (runSecs.value / 60 / runGoal.value) * 100) : 0))

// „Als Nächstes“: die nächste offene Aufgabe (das Backend sortiert nach Uhrzeit), oder ein Termin, der vorher beginnt.
const queue = computed(() => openTasks.value.filter((t) => t.id !== cardTask.value?.id))
const lead = (start: number | null) => (start === null ? '' : start > nowMin.value ? t('in {time}', { time: dur(start - nowMin.value) }) : t('geplant {time}', { time: hm(start) }))
const nextItem = computed(() => {
  const next = queue.value[0]
  const tStart = next?.planned_start_at ? minsOf(Date.parse(next.planned_start_at)) : null
  const ev = nextEvent.value
  if (ev && (!next || (tStart !== null && ev.start < tStart))) {
    return { kind: t('Termin'), title: ev.title, project: '', color: ev.color, when: `${hm(ev.start)}–${hm(ev.end)}`, lead: lead(ev.start), task: undefined, then: '' }
  }
  if (!next) return null
  const after = queue.value[1]
  return {
    kind: t('Aufgabe'), title: next.title, project: projectNames.value.get(next.project_id ?? '') ?? '', color: projectColor(next.project_id), task: next, lead: lead(tStart),
    when: [next.planned_start_at ? `${hm(tStart!)}–${hm(tStart! + (next.estimated_minutes || 30))}` : '', next.estimated_minutes ? hm(next.estimated_minutes) : ''].filter(Boolean).join(' · '),
    then: after ? `${t('danach')} ${after.planned_start_at ? `${fmt(after.planned_start_at)} ` : ''}${after.title}` : '',
  }
})

// „Heute fällig“: erst Überfälliges, dann der Tag; Aktives ohne Plan für heute folgt darunter.
const dueRows = computed(() => [
  ...(today.value?.overdue ?? []).map((t) => ({ t, od: true })),
  ...openTasks.value.map((t) => ({ t, od: false })),
])
const whenOf = (r: { t: Task; od: boolean }) => (r.od ? daysAgo(r.t.planned_date!, today.value!.date) : r.t.planned_start_at ? fmt(r.t.planned_start_at) : t('Heute'))
const activeOnly = computed(() => today.value?.active_tasks ?? [])
const doneCount = computed(() => (today.value?.tasks ?? []).filter((t) => t.status === 'COMPLETED').length)

const y = (m: number) => ((m - START_H * 60) / 60) * HOUR
// Kürzere Blöcke zeichnet das Raster mit Mindesthöhe (22 px = 30 Minuten).
const MIN_BLOCK = 30

// Überlappende Blöcke stehen nebeneinander.
const layout = computed(() => columns(blocks.value, MIN_BLOCK))

const style = (b: Block) => {
  const top = Math.max(y(b.start), 0)
  const height = Math.max(y(Math.min(b.end, END_H * 60)) - top, 22)
  const { col, cols } = layout.value.get(b.key) ?? { col: 0, cols: 1 }
  // Nutzbreite: 100 % abzüglich der Ränder (6 px links, 10 px rechts).
  return {
    top: `${top}px`, height: `${height}px`, '--acc': b.color,
    left: `calc(6px + (100% - 16px) * ${col / cols})`, width: `calc((100% - 16px) / ${cols} - 2px)`, right: 'auto',
  }
}
const hours = Array.from({ length: END_H - START_H + 1 }, (_, i) => START_H + i)
const nowTop = computed(() => (nowMin.value >= START_H * 60 && nowMin.value <= END_H * 60 ? y(nowMin.value) : null))

// Aufgaben aus der Liste lassen sich in den Plan ziehen (HTML-Drag-and-drop; das Raster ist kein FullCalendar).
const dragTask = (e: DragEvent, t: Task) => {
  e.dataTransfer!.setData('text/plain', t.id)
  e.dataTransfer!.effectAllowed = 'move'
}

// Minuten seit Mitternacht am Tag der Ansicht als Zeitpunkt, in der Zeitzone des Backends.
function instantAt(min: number) {
  const guess = Date.parse(`${today.value!.date}T00:00:00Z`) + min * 60_000
  const off = (((minsOf(guess) - min + 720) % 1440) + 1440) % 1440 - 720 // Zeitzonenversatz in Minuten
  return new Date(guess - off * 60_000)
}

// Plan einer Task ändern; Rückgängig stellt den bisherigen Tag und die Uhrzeit wieder her.
function reschedule(task: Task, plan: { planned_date: string; planned_start_at: string }, done: string) {
  const { id, title, planned_date, planned_start_at } = task
  void run(async () => {
    await updateTask(id, plan)
    offer(t(done, { title }), () => run(() => updateTask(id, { planned_date: planned_date ?? '', planned_start_at: planned_start_at ?? '' })))
  })
}

function dropTask(e: DragEvent) {
  const task = tasksById.value.get(e.dataTransfer?.getData('text/plain') ?? '')
  if (!task || !today.value) return
  const top = (e.currentTarget as HTMLElement).getBoundingClientRect().top
  const snapped = START_H * 60 + Math.round(((e.clientY - top) / HOUR) * 4) * 15 // auf 15 Minuten
  const min = Math.min(Math.max(snapped, START_H * 60), END_H * 60 - 15)
  reschedule(task, { planned_date: today.value.date, planned_start_at: instantAt(min).toISOString() }, '„{title}“ eingeplant')
}

const quickParsed = computed(() => parseQuickAdd(quick.value, projectList.value, today.value?.date))
// Ohne Tag in der Eingabe landet die Task heute.
function addQuick() {
  const q = quickParsed.value
  if (!q.title || !today.value) return
  void run(async () => {
    await createTask({ ...taskBody(q, today.value!.date), title: q.title })
    quick.value = ''
  })
}

// Überfällige Task auf heute (0) oder morgen (1) verschieben; die alte Uhrzeit entfällt.
function moveTo(t: Task, days: number) {
  const d = new Date(`${today.value!.date}T12:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  reschedule(t, { planned_date: d.toISOString().slice(0, 10), planned_start_at: '' }, '„{title}“ verschoben')
}

function removeEntry(e: TimeEntry) {
  const { task_id, started_at, ended_at } = e
  void run(async () => {
    await deleteTimeEntry(e.id)
    if (task_id && ended_at) offer(t('Zeiteintrag gelöscht'), () => run(() => createTimeEntry({ task_id, started_at, ended_at })))
  })
}

// Dauer eines Eintrags in Minuten; ein laufender zählt bis jetzt.
const entryMin = (e: TimeEntry) => entryMinutes(e, now.value)
const entryProject = (e: TimeEntry) => e.project_id ?? tasksById.value.get(e.task_id ?? '')?.project_id ?? null
const entryLabel = (e: TimeEntry) =>
  (e.task_id && taskTitles.value.get(e.task_id)) || (e.project_id && projectNames.value.get(e.project_id)) || t('Projektzeit')

// Zeit nachtragen: Task, Tag und Uhrzeiten in der Zeitzone des Browsers.
const manual = ref({ task: '', date: ymd(new Date()), from: '', to: '' })
const saved = ref(false)
function addEntry() {
  const m = manual.value
  void run(async () => {
    await createTimeEntry({ task_id: m.task, started_at: new Date(`${m.date}T${m.from}`).toISOString(), ended_at: new Date(`${m.date}T${m.to}`).toISOString() })
    saved.value = true
  })
}

// Zeiteinträge: HH:MM-Felder in der Zeitzone des Browsers, der Tag bleibt der des Eintrags.
const clock = (iso: string) => hhmm(new Date(iso))
function setTime(e: TimeEntry, field: 'started_at' | 'ended_at', value: string) {
  const old = e[field]
  if (!old || !value || value === clock(old)) return
  const d = new Date(old)
  const [h, m] = value.split(':').map(Number)
  d.setHours(h!, m!, 0, 0)
  void run(() => updateTimeEntry(e.id, { [field]: d.toISOString() }))
}
</script>

<template>
  <div class="view-inner start">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="today">
      <header class="st-head">
        <div>
          <h1 class="day-title">{{ dateLabel }}</h1>
          <div v-if="allDay.length" class="st-allday">
            <span class="cdot" :style="{ background: allDay[0]!.color }"></span>{{ $t('Ganztags') }}: {{ allDay.map((b) => b.title).join(', ') }}
          </div>
        </div>
        <div class="st-stats">
          <div><span>{{ $t('geplant') }}</span><b>{{ hm(today.planned_minutes) }}</b></div>
          <div><span>{{ $t('erfasst') }}</span><b>{{ hm(today.tracked_minutes) }}</b></div>
          <div :title="$t('Frei ab jetzt')"><span>{{ $t('frei') }}</span><b>{{ hm(today.free_minutes) }}</b></div>
        </div>
      </header>

      <div class="st-cards">
        <section class="st-card" :aria-label="$t('Läuft')">
          <template v-if="runEntry || pausedTask">
            <div class="st-top"><span class="st-lbl"><i class="run-dot" :class="{ off: !runEntry }"></i>{{ runEntry ? $t('Läuft') : $t('Pausiert') }}</span><span v-if="runGoal" class="st-aside">{{ $t('Ziel') }} {{ hm(runGoal) }}</span></div>
            <div class="st-run">
              <div class="st-main">
                <div class="st-t">{{ cardTask?.title ?? entryLabel(runEntry!) }}</div>
                <div class="st-meta"><span class="chip" :style="{ '--chip-c': projectColor(runProject) }"><span class="cdot"></span>{{ projectNames.get(runProject ?? '') ?? $t('Ohne Projekt') }}<template v-if="runEntry"> · {{ $t('seit {time}', { time: fmt(runEntry.started_at) }) }}</template></span></div>
              </div>
              <span class="st-timer mono">{{ elapsed }}</span>
            </div>
            <div v-if="runGoal" class="st-bar" aria-hidden="true"><i :style="{ width: runPct + '%', background: projectColor(runProject) }"></i></div>
            <div v-if="cardTask" class="st-btns">
              <button v-if="runningTask" type="button" class="btn btn-secondary" @click="run(() => taskAction(runningTask!.id, 'pause'))">{{ $t('Pause') }}</button>
              <button v-else type="button" class="btn btn-primary" @click="run(() => taskAction(cardTask!.id, 'start'))">{{ $t('Fortsetzen') }}</button>
              <button type="button" class="btn btn-secondary" @click="setDone(cardTask!, 'COMPLETED', run)">{{ $t('Fertig') }}</button>
            </div>
          </template>
          <template v-else>
            <div class="st-top"><span class="st-lbl"><i class="run-dot off"></i>{{ $t('Kein Timer') }}</span></div>
            <div class="st-t is-free">{{ $t('Es läuft gerade nichts.') }}</div>
            <div class="st-meta">{{ $t('Starte eine Aufgabe, um Zeit zu erfassen.') }}</div>
          </template>
        </section>

        <section class="st-card" :aria-label="$t('Als Nächstes')">
          <template v-if="nextItem">
            <div class="st-top"><span class="st-lbl">{{ $t('Als Nächstes') }}<span v-if="nextItem.lead" class="st-lead">· {{ nextItem.lead }}</span></span><span class="st-aside">{{ nextItem.kind }}</span></div>
            <div class="st-t">{{ nextItem.title }}</div>
            <div class="st-meta">
              <span v-if="nextItem.project" class="chip" :style="{ '--chip-c': nextItem.color }"><span class="cdot"></span>{{ nextItem.project }}</span>
              <span v-if="nextItem.when">{{ nextItem.when }}</span>
            </div>
            <div v-if="nextItem.task" class="st-btns">
              <button type="button" class="btn btn-primary" @click="run(() => taskAction(nextItem!.task!.id, 'start'))"><svg class="ic" viewBox="0 0 24 24" style="fill:currentColor;stroke:none;width:11px;height:11px"><path d="M7 4.5v15l12-7.5z" /></svg>{{ $t('Jetzt starten') }}</button>
              <button type="button" class="btn btn-secondary" @click="moveTo(nextItem.task, 1)">{{ $t('Verschieben') }}</button>
              <span v-if="nextItem.then" class="st-then">{{ nextItem.then }}</span>
            </div>
          </template>
          <template v-else>
            <div class="st-top"><span class="st-lbl">{{ $t('Als Nächstes') }}</span></div>
            <div class="st-t is-free">{{ $t('Keine offene Aufgabe für heute.') }}</div>
          </template>
        </section>
      </div>

      <div v-if="today.unestimated_tasks" class="v-sub">{{ $tn(today.unestimated_tasks, '{n} Aufgabe ohne Schätzung, gerechnet mit 30 Min.', '{n} Aufgaben ohne Schätzung, gerechnet mit 30 Min.') }}</div>
      <div v-if="today.overplanned_minutes" class="badge"><span class="cdot" style="background:var(--a-red)"></span>{{ $t('Überplant um {time} h', { time: hm(today.overplanned_minutes) }) }}</div>

      <div class="start-grid">
        <section class="start-col">
          <div class="col-head"><h2 class="col-title">{{ $t('Heute fällig') }} <span class="count">{{ dueRows.length }}</span></h2><router-link class="col-link" to="/tasks">{{ $t('Alle Aufgaben') }}</router-link></div>
          <div v-if="dueRows.length" class="tasklist">
            <div v-for="r in dueRows" :key="r.t.id" class="task-row" draggable="true" @dragstart="dragTask($event, r.t)">
              <button type="button" class="cb ring" :style="{ '--rc': projectColor(r.t.project_id) }" role="checkbox" aria-checked="false" :aria-label="$t('{title} erledigt', { title: r.t.title })" @click="setDone(r.t, 'COMPLETED', run)"></button>
              <span class="t">{{ r.t.title }}</span>
              <span class="chip st-proj" :style="{ '--chip-c': projectColor(r.t.project_id) }"><template v-if="r.t.project_id"><span class="cdot"></span>{{ projectNames.get(r.t.project_id) }}</template></span>
              <span class="due" :class="{ od: r.od, now: !r.od && !!r.t.planned_start_at }">{{ whenOf(r) }}</span>
              <span class="dur">{{ r.t.estimated_minutes ? hm(r.t.estimated_minutes) : '' }}</span>
              <span v-if="r.od" class="row-act"><button type="button" class="btn btn-ghost" @click="moveTo(r.t, 0)">{{ $t('→ Heute') }}</button><button type="button" class="btn btn-ghost" @click="moveTo(r.t, 1)">{{ $t('→ Morgen') }}</button></span>
              <TaskActions v-else class="row-act" :task="r.t" :running="runningTaskId === r.t.id" @run="run" />
            </div>
          </div>
          <div v-else class="v-sub">{{ $t('Keine offenen Aufgaben für heute.') }}</div>
          <div v-if="doneCount" class="v-sub">{{ $t('{n} erledigt', { n: doneCount }) }}</div>
          <form class="addrow" @submit.prevent="addQuick">
            <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
            <input ref="quickEl" v-model="quick" type="text" :placeholder="$t('Aufgabe hinzufügen, z. B. Sport 30m @morgen #Kairo')" :aria-label="$t('Aufgabe für heute hinzufügen')" aria-keyshortcuts="n" />
            <kbd class="key" aria-hidden="true">N</kbd>
          </form>
          <QuickHints :q="quickParsed" :today="today.date" />

          <template v-if="activeOnly.length">
            <div class="col-head st-gap"><h2 class="col-title">{{ $t('Aktiv, nicht für heute geplant') }} <span class="count">{{ activeOnly.length }}</span></h2></div>
            <div class="tasklist">
              <div v-for="t in activeOnly" :key="t.id" class="task-row" draggable="true" @dragstart="dragTask($event, t)">
                <button type="button" class="cb ring" :style="{ '--rc': projectColor(t.project_id) }" role="checkbox" aria-checked="false" :aria-label="$t('{title} erledigt', { title: t.title })" @click="setDone(t, 'COMPLETED', run)"></button>
                <span class="t">{{ t.title }}</span>
                <TaskActions class="row-act" :task="t" :running="runningTaskId === t.id" @run="run" />
              </div>
            </div>
          </template>

          <template v-if="today.habits.length">
            <div class="col-head st-gap"><h2 class="col-title">{{ $t('Gewohnheiten') }}</h2><router-link class="col-link" to="/habits">{{ $t('Alle ansehen') }}</router-link></div>
            <div class="st-habits">
              <button
                v-for="h in today.habits" :key="h.id" type="button" class="st-hab" :class="{ 'is-done': h.done }" role="checkbox" :aria-checked="h.done"
                @click="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
              >
                <span class="cb sq" :style="{ '--rc': habitColor(h.id) }" :aria-checked="h.done" aria-hidden="true"></span>
                <span class="st-hab-t"><b>{{ h.name }}</b><small>{{ h.done ? $t('erledigt') : $t('offen') }}<template v-if="h.week_progress"> · {{ h.week_progress.done }}/{{ h.week_progress.target }}</template></small></span>
              </button>
            </div>
          </template>

          <div class="col-head st-gap">
            <h2 class="col-title">{{ $t('Zeiterfassung') }} <span class="count">{{ hm(today.tracked_minutes) }} {{ $t('heute') }}</span></h2>
            <button type="button" class="col-link" :aria-expanded="manualOpen" @click="manualOpen = !manualOpen">{{ $t('Zeit nachtragen') }}</button>
          </div>
          <form v-if="manualOpen" class="manual-form" @submit.prevent="addEntry" @input="saved = false">
            <label class="field wide"><span>{{ $t('Aufgabe') }}</span>
              <select v-model="manual.task" class="input" required>
                <option value="" disabled>{{ $t('Aufgabe wählen') }}</option>
                <option v-for="[id, title] in taskTitles" :key="id" :value="id">{{ title }}</option>
              </select>
            </label>
            <label class="field"><span>{{ $t('Datum') }}</span><input v-model="manual.date" class="input" type="date" required /></label>
            <label class="field"><span>{{ $t('Start') }}</span><input v-model="manual.from" class="input" type="time" required /></label>
            <label class="field"><span>{{ $t('Ende') }}</span><input v-model="manual.to" class="input" type="time" required /></label>
            <button type="submit" class="btn btn-secondary">{{ $t('Eintragen') }}</button>
            <span v-if="saved" class="v-sub" role="status">{{ $t('Gespeichert.') }}</span>
          </form>
          <div v-if="entries.length" class="tasklist">
            <div v-for="e in entries" :key="e.id" class="task-row te-row" :style="{ '--acc': projectColor(entryProject(e)) }">
              <span class="te-bar"></span>
              <span class="t">{{ entryLabel(e) }}</span>
              <span class="st-proj te-proj">{{ projectNames.get(entryProject(e) ?? '') ?? '' }}</span>
              <span class="te-range">
                <input class="input te-time" type="time" :value="clock(e.started_at)" :aria-label="$t('Start')" @change="setTime(e, 'started_at', ($event.target as HTMLInputElement).value)" />
                <span>–</span>
                <input v-if="e.ended_at" class="input te-time" type="time" :value="clock(e.ended_at)" :aria-label="$t('Ende')" @change="setTime(e, 'ended_at', ($event.target as HTMLInputElement).value)" />
                <span v-else class="te-run">{{ $t('läuft') }}</span>
              </span>
              <span class="dur te-dur">{{ hm(entryMin(e)) }}</span>
              <span v-if="e.ended_at" class="row-act"><DeleteButton ghost :text="$t('Zeiteintrag löschen?')" @confirm="removeEntry(e)" /></span>
            </div>
          </div>
          <div v-else class="v-sub">{{ $t('Noch keine Zeit erfasst.') }}</div>
        </section>

        <section class="start-col">
          <div class="col-head"><h2 class="col-title">{{ $t('Tagesplan') }}</h2><router-link class="col-link" to="/calendar">{{ $t('Kalender') }}<svg class="ic" aria-hidden="true"><use href="#i-right" /></svg></router-link></div>
          <div class="dayplan">
            <div class="dp-scroll">
              <div class="dp-body">
                <div class="hourcol" :style="{ '--hour': HOUR + 'px' }">
                  <div v-for="h in hours" :key="h" class="hl"><span class="mono">{{ String(h).padStart(2, '0') }}</span></div>
                </div>
                <div class="dp-grid" :style="{ height: (END_H - START_H) * HOUR + 'px' }" @dragover.prevent @drop.prevent="dropTask">
                  <div v-for="h in hours" :key="h" class="hline" :style="{ top: (h - START_H) * HOUR + 'px' }"></div>
                  <div v-if="nowTop !== null" class="now-line" :style="{ top: nowTop + 'px' }"><span class="mono">{{ hm(nowMin) }}</span></div>
                  <div
                    v-for="b in blocks" :key="b.key" class="dp-ev" :class="[`kind-${b.kind}`, { done: b.done, now: b.running, short: b.end - b.start <= 30 }]" :style="style(b)"
                  >
                    <div class="dp-t"><span class="cdot"></span><span class="dp-n">{{ b.title }}</span><span v-if="b.end - b.start <= 30" class="dp-m">{{ hm(b.start) }}</span></div>
                    <div v-if="b.end - b.start > 30" class="dp-m">{{ hm(b.start) }}–{{ hm(b.end) }}<template v-if="b.sub"> · {{ b.sub }}</template></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="v-sub st-hint">{{ $t('Aufgaben aus der Liste in den Plan ziehen, um sie einzuplanen.') }}</div>
        </section>
      </div>
    </template>
  </div>
</template>

<style scoped>
.day-title { font: 600 28px/1.2 var(--font-ui); letter-spacing: -0.02em; color: var(--tx-primary); }
.st-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; flex-wrap: wrap; margin-bottom: 24px; }
.st-allday { display: flex; align-items: center; gap: 8px; margin-top: 10px; font: 400 13px/1.3 var(--font-ui); color: var(--tx-secondary); }
.st-allday .cdot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
.st-stats { display: flex; gap: 28px; }
.st-stats div { display: grid; gap: 4px; justify-items: end; }
.st-stats span { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.st-stats b { font: 500 18px/1.1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-primary); }

.st-cards { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-bottom: 36px; }
.st-card { display: flex; flex-direction: column; gap: 12px; padding: 18px 20px; background: var(--bg-1); border: 1px solid var(--br-subtle); border-radius: 14px; min-width: 0; }
.st-top { display: flex; align-items: center; justify-content: space-between; gap: 12px; font: 400 12.5px/1 var(--font-ui); color: var(--tx-muted); }
.st-lbl { display: inline-flex; align-items: center; gap: 7px; }
.st-lead { color: var(--tx-muted); }
.run-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--a-run); }
.run-dot.off { background: var(--br-strong); }
.st-run { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }
.st-main { display: grid; gap: 8px; min-width: 0; }
.st-t { font: 600 18px/1.3 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); text-wrap: pretty; }
.st-t.is-free { color: var(--tx-secondary); font-weight: 500; }
.st-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 12px; font: 400 13px/1.3 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-secondary); }
.st-timer { font: 500 30px/1 var(--font-mono); font-variant-numeric: tabular-nums; letter-spacing: -0.02em; color: var(--tx-primary); white-space: nowrap; }
.st-bar { height: 3px; border-radius: 2px; background: var(--bg-3); overflow: hidden; }
.st-bar i { display: block; height: 100%; border-radius: 2px; }
.st-btns { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-top: auto; }
.st-btns .btn { height: 30px; }
.st-then { margin-left: auto; font: 400 12.5px/1.3 var(--font-ui); color: var(--tx-muted); }

.col-title .count { margin-left: 4px; font: 400 14px/1 var(--font-ui); }
.st-gap { margin-top: 32px; }
button.col-link { background: none; }
.st-proj { flex: none; width: 118px; overflow: hidden; }
.task-row .due.now { color: var(--tx-primary); }
.task-row .due { min-width: 64px; }
.task-row[draggable='true'] { cursor: grab; }
.addrow .key { margin-left: auto; }

.st-habits { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; }
.st-hab { display: flex; align-items: center; gap: 10px; min-height: 52px; padding: 0 12px; text-align: left; background: var(--bg-1); border: 1px solid var(--br-subtle); border-radius: 10px; transition: border-color var(--dur) ease-out, background var(--dur) ease-out; }
.st-hab:hover { border-color: var(--br-default); background: var(--bg-2); }
.st-hab .cb { width: 20px; height: 20px; border-radius: 6px; pointer-events: none; }
.st-hab-t { display: grid; gap: 2px; min-width: 0; }
.st-hab-t b { font: 600 13px/1.2 var(--font-ui); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.st-hab-t small { font: 400 12px/1.2 var(--font-ui); color: var(--tx-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.te-row { position: relative; padding-left: 12px; }
.te-bar { flex: none; width: 3px; height: 16px; border-radius: 2px; background: var(--acc); }
.te-row .t { font-weight: 500; }
.te-proj { font: 400 13px/1 var(--font-ui); color: var(--tx-secondary); }
.te-range { display: inline-flex; align-items: center; gap: 2px; color: var(--tx-secondary); font: 400 13px/1 var(--font-ui); font-variant-numeric: tabular-nums; }
.te-time { width: 66px; flex: none; text-align: center; padding: 0 2px; background: transparent; border-color: transparent; font-size: 13px; color: var(--tx-secondary); }
.te-time:hover, .te-time:focus-visible { background: var(--bg-2); }
.te-time::-webkit-calendar-picker-indicator { display: none; }
.te-run { width: 66px; text-align: center; color: var(--tx-muted); }
.te-dur { min-width: 40px; color: var(--tx-primary); }
.manual-form { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; margin-bottom: 14px; }
.manual-form .field { width: 140px; }
.manual-form .field.wide { width: 100%; }

.st-hint { margin-top: 10px; font-size: 12.5px; color: var(--tx-muted); }
.now-line { position: absolute; left: 0; right: 0; height: 1.5px; background: var(--tx-primary); z-index: 3; pointer-events: none; }
.now-line span { position: absolute; left: -52px; top: -8px; padding: 2px 5px; border-radius: 4px; background: var(--tx-primary); color: var(--bg-0); font: 600 11px/1.2 var(--font-mono); }
@media (max-width: 900px) { .st-cards { grid-template-columns: 1fr; } .st-run { flex-wrap: wrap; } }
</style>
