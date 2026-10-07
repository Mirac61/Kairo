<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  completeHabit, createTask, createTimeEntry, deleteTimeEntry, errorMessage, getToday, listProjects, listTasks, listTimeEntries, uncompleteHabit, updateTask, updateTimeEntry,
  type Project, type Task, type TimeEntry, type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useUndo } from '@/composables/useUndo'
import { daysAgo, hm, ymd } from '@/lib/dates'
import { parseQuickAdd, taskBody } from '@/lib/quickAdd'
import QuickHints from '@/components/QuickHints.vue'
import { projectColor } from '@/lib/projectColor'
import TaskActions from '@/components/TaskActions.vue'
import DeleteButton from '@/components/DeleteButton.vue'

// Planungsfenster der Tagesleiste und des Tagesplans.
const START_H = 7
const END_H = 22
const HOUR = 44

const { offer } = useUndo()
const today = ref<Today | null>(null)
const error = ref('')
const now = ref(Date.now())
const quick = ref('')
const entries = ref<TimeEntry[]>([])
const tasksById = ref(new Map<string, Task>())
const taskTitles = computed(() => new Map([...tasksById.value].map(([id, t]) => [id, t.title])))
const projectNames = ref(new Map<string, string>())
const projectList = ref<Project[]>([])
let tick: ReturnType<typeof setInterval> | undefined

async function load() {
  try {
    const t = await getToday()
    const from = new Date(`${t.date}T00:00:00`)
    const [es, tasks, projects] = await Promise.all([listTimeEntries(from, new Date(from.getTime() + 864e5)), listTasks(), listProjects()])
    today.value = t
    entries.value = es.reverse()
    tasksById.value = new Map(tasks.map((x) => [x.id, x]))
    projectNames.value = new Map(projects.map((x) => [x.id, x.name]))
    projectList.value = projects.filter((p) => p.status !== 'ARCHIVED') // für #name in der Schnelleingabe
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function run(fn: () => Promise<unknown>) {
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
  await load() // das /ws-Ereignis lädt ebenfalls, aber so ist die Anzeige auch ohne /ws aktuell
}

useLiveEvents(load)
onMounted(() => {
  void load()
  tick = setInterval(() => (now.value = Date.now()), 1000)
})
onUnmounted(() => clearInterval(tick))

// Das Backend meldet ohne konfigurierte Zeitzone "Local"; das ist kein gültiger IANA-Name.
const tz = computed(() => (today.value?.timezone === 'Local' ? undefined : today.value?.timezone))
const fmt = (iso: string) => new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit', timeZone: tz.value }).format(new Date(iso))
// Minuten seit Mitternacht in der Zeitzone des Backends.
function minsOf(ms: number) {
  const p = new Intl.DateTimeFormat('de-DE', { hour: 'numeric', minute: 'numeric', hourCycle: 'h23', timeZone: tz.value }).formatToParts(new Date(ms))
  const n = (t: string) => Number(p.find((x) => x.type === t)?.value ?? 0)
  return n('hour') * 60 + n('minute')
}
const dur = (m: number) => (m >= 60 ? `${Math.floor(m / 60)} Std${m % 60 ? ` ${m % 60} Min` : ''}` : `${m} Min`)

const dateLabel = computed(() =>
  new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date(now.value)))

const nowMin = computed(() => minsOf(now.value))
const runningTaskId = computed(() => today.value?.running_time_entry?.task_id ?? null)
const elapsed = computed(() => {
  const e = today.value?.running_time_entry
  if (!e) return ''
  const s = Math.max(0, Math.floor((now.value - Date.parse(e.started_at)) / 1000))
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(Math.floor(s / 3600))}:${p(Math.floor(s / 60) % 60)}:${p(s % 60)}`
})

interface Block { key: string; title: string; start: number; end: number; kind: 'event' | 'task'; done: boolean; running?: boolean; color: string; sub: string }

// Termine und Tasks mit Uhrzeit als Blöcke; Tasks ohne Dauer zählen 30 Minuten.
const blocks = computed<Block[]>(() => {
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

const upcoming = computed(() => blocks.value.filter((b) => b.kind === 'event' && !b.done))
const current = computed(() => upcoming.value.find((b) => b.start <= nowMin.value && b.end > nowMin.value))
const next = computed(() => upcoming.value.find((b) => b.start > nowMin.value))

const openTasks = computed(() => (today.value?.tasks ?? []).filter((t) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'))
// Fokus: der laufende Timer, sonst die nächste geplante offene Task (das Backend sortiert nach Uhrzeit, ohne Uhrzeit zuletzt).
const runningTask = computed(() => (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined))
const nextTask = computed(() => openTasks.value[0])
const focusTask = computed(() => runningTask.value ?? nextTask.value)
const groups = computed(() => [
  { title: 'Mit Uhrzeit', tasks: openTasks.value.filter((t) => t.planned_start_at) },
  { title: 'Ohne Uhrzeit', tasks: openTasks.value.filter((t) => !t.planned_start_at) },
  { title: 'Aktiv, nicht für heute geplant', tasks: today.value?.active_tasks ?? [] },
].filter((g) => g.tasks.length))
const doneTasks = computed(() => (today.value?.tasks ?? []).filter((t) => t.status === 'COMPLETED'))

const y = (m: number) => ((m - START_H * 60) / 60) * HOUR
// Kürzere Blöcke zeichnet das Raster mit Mindesthöhe (22 px = 30 Minuten).
const MIN_BLOCK = 30

// Überlappende Blöcke stehen nebeneinander: Spalte und Spaltenzahl je Block.
const layout = computed(() => {
  const pos = new Map<string, { col: number; cols: number }>()
  let cluster: { key: string; col: number }[] = []
  let ends: number[] = [] // Ende des letzten Blocks je Spalte
  const flush = () => {
    for (const c of cluster) pos.set(c.key, { col: c.col, cols: ends.length })
    cluster = []
    ends = []
  }
  for (const b of blocks.value) { // nach Beginn sortiert
    if (ends.length && b.start >= Math.max(...ends)) flush() // nichts überlappt mehr
    let col = ends.findIndex((e) => e <= b.start)
    if (col < 0) col = ends.length
    ends[col] = Math.max(b.end, b.start + MIN_BLOCK)
    cluster.push({ key: b.key, col })
  }
  flush()
  return pos
})

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

function dropTask(e: DragEvent) {
  const task = tasksById.value.get(e.dataTransfer?.getData('text/plain') ?? '')
  if (!task || !today.value) return
  const top = (e.currentTarget as HTMLElement).getBoundingClientRect().top
  const snapped = START_H * 60 + Math.round(((e.clientY - top) / HOUR) * 4) * 15 // auf 15 Minuten
  const min = Math.min(Math.max(snapped, START_H * 60), END_H * 60 - 15)
  const { id, title, planned_date, planned_start_at } = task
  void run(async () => {
    await updateTask(id, { planned_date: today.value!.date, planned_start_at: instantAt(min).toISOString() })
    offer(`„${title}“ eingeplant`, () => run(() => updateTask(id, { planned_date: planned_date ?? '', planned_start_at: planned_start_at ?? '' })))
  })
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
const taskTitleClass = (t: Task) => ({ done: t.status === 'COMPLETED' })

// Überfällige Task auf heute (0) oder morgen (1) verschieben; die alte Uhrzeit entfällt.
function moveTo(t: Task, days: number) {
  const d = new Date(`${today.value!.date}T12:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  const { id, title, planned_date, planned_start_at } = t
  void run(async () => {
    await updateTask(id, { planned_date: d.toISOString().slice(0, 10), planned_start_at: '' })
    offer(`„${title}“ verschoben`, () => run(() => updateTask(id, { planned_date: planned_date ?? '', planned_start_at: planned_start_at ?? '' })))
  })
}

function removeEntry(e: TimeEntry) {
  const { task_id, started_at, ended_at } = e
  void run(async () => {
    await deleteTimeEntry(e.id)
    if (task_id && ended_at) offer('Zeiteintrag gelöscht', () => run(() => createTimeEntry({ task_id, started_at, ended_at })))
  })
}

const entryLabel = (e: TimeEntry) =>
  (e.task_id && taskTitles.value.get(e.task_id)) || (e.project_id && projectNames.value.get(e.project_id)) || 'Projektzeit'

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
const hhmm = (iso: string) => new Date(iso).toTimeString().slice(0, 5)
function setTime(e: TimeEntry, field: 'started_at' | 'ended_at', value: string) {
  const old = e[field]
  if (!old || !value || value === hhmm(old)) return
  const d = new Date(old)
  const [h, m] = value.split(':').map(Number)
  d.setHours(h!, m!, 0, 0)
  void run(() => updateTimeEntry(e.id, { [field]: d.toISOString() }))
}
</script>

<template>
  <div class="view-inner">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="today">
      <section class="focus" aria-label="Fokus">
        <div class="focus-head">
          <h1 class="day-title">{{ dateLabel }}</h1>
          <div class="focus-stats">
            <div class="fstat"><span class="c">geplant</span><span class="n">{{ hm(today.planned_minutes) }}</span></div>
            <div class="fstat"><span class="c">erfasst</span><span class="n">{{ hm(today.tracked_minutes) }}</span></div>
            <div class="fstat" title="Frei ab jetzt"><span class="c">frei</span><span class="n">{{ hm(today.free_minutes) }}</span></div>
          </div>
        </div>

        <div v-if="focusTask" class="fm" :class="{ running: !!runningTask }">
          <div class="fm-main">
            <span class="fm-lbl"><span v-if="runningTask" class="run-dot"></span>{{ runningTask ? `Läuft seit ${fmt(today.running_time_entry!.started_at)}` : 'Als Nächstes' }}</span>
            <div class="fm-t">{{ focusTask.title }}</div>
            <div class="fm-meta">
              <span v-if="focusTask.project_id" class="chip" :style="{ '--chip-c': projectColor(focusTask.project_id) }"><span class="cdot"></span>{{ projectNames.get(focusTask.project_id) }}</span>
              <span v-if="!runningTask && focusTask.planned_start_at">{{ fmt(focusTask.planned_start_at) }}</span>
              <span v-if="focusTask.estimated_minutes">Schätzung {{ hm(focusTask.estimated_minutes) }}</span>
            </div>
          </div>
          <div class="fm-side">
            <span v-if="runningTask" class="fm-timer">{{ elapsed }}</span>
            <TaskActions :task="focusTask" :running="!!runningTask" @run="run" />
          </div>
        </div>
        <div v-else class="fm"><div class="fm-main"><span class="fm-lbl">Als Nächstes</span><div class="fm-t is-free">Keine offene Aufgabe für heute.</div></div></div>

        <div v-if="current" class="focus-next small">
          <span class="fn-when">Termin läuft</span>
          <span class="fn-what"><span class="cdot" :style="{ background: current.color }"></span><span class="fn-t">{{ current.title }}</span><span class="fn-time">noch {{ dur(current.end - nowMin) }}</span></span>
        </div>
        <div v-else-if="next" class="focus-next small">
          <span class="fn-when">In {{ dur(next.start - nowMin) }}</span>
          <span class="fn-what"><span class="cdot" :style="{ background: next.color }"></span><span class="fn-t">{{ next.title }}</span><span class="fn-time">{{ hm(next.start) }}–{{ hm(next.end) }}</span></span>
        </div>
        <div v-else class="focus-next small is-free">
          <span class="fn-when">{{ today.events.length ? 'Termine erledigt' : 'Keine Termine heute' }}</span>
        </div>

        <div v-if="today.unestimated_tasks" class="v-sub">{{ today.unestimated_tasks }} {{ today.unestimated_tasks === 1 ? 'Task' : 'Tasks' }} ohne Schätzung, gerechnet mit 30 Min.</div>
        <div v-if="today.overplanned_minutes" class="badge"><span class="cdot" style="background:var(--a-red)"></span>Überplant um {{ hm(today.overplanned_minutes) }} h</div>
      </section>

      <div class="start-grid">
        <section class="start-col">
          <div class="col-head"><h2 class="col-title">Aufgaben</h2><router-link class="col-link" to="/tasks">Alle ansehen</router-link></div>
          <div v-if="today.overdue.length" class="tgroup">
            <span class="lbl od-h"><svg class="ic" aria-hidden="true"><use href="#i-alert" /></svg>Überfällig<span class="count">{{ today.overdue.length }}</span></span>
            <div class="tasklist">
              <div v-for="t in today.overdue" :key="t.id" class="task-row" draggable="true" @dragstart="dragTask($event, t)">
                <span class="t">{{ t.title }}</span>
                <span class="due od">{{ daysAgo(t.planned_date!, today.date) }}</span>
                <span class="row-act"><button type="button" class="btn btn-ghost" @click="moveTo(t, 0)">→ Heute</button><button type="button" class="btn btn-ghost" @click="moveTo(t, 1)">→ Morgen</button></span>
              </div>
            </div>
          </div>
          <div v-for="g in groups" :key="g.title" class="tgroup">
            <span class="lbl">{{ g.title }}<span class="count">{{ g.tasks.length }}</span></span>
            <div class="tasklist">
              <div v-for="t in g.tasks" :key="t.id" class="task-row" draggable="true" @dragstart="dragTask($event, t)">
                <span class="t" :class="taskTitleClass(t)">{{ t.title }}</span>
                <span v-if="t.planned_start_at" class="due">{{ fmt(t.planned_start_at) }}</span>
                <TaskActions class="row-act" :task="t" :running="runningTaskId === t.id" @run="run" />
              </div>
            </div>
          </div>
          <div v-if="!groups.length" class="v-sub">Keine offenen Aufgaben für heute.</div>
          <div v-if="doneTasks.length" class="v-sub">{{ doneTasks.length }} erledigt</div>
          <form class="addrow" @submit.prevent="addQuick">
            <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
            <input v-model="quick" type="text" placeholder="Aufgabe hinzufügen, z. B. Sport 30m @morgen #Kairo" aria-label="Aufgabe für heute hinzufügen" />
          </form>
          <QuickHints v-if="today" :q="quickParsed" :today="today.date" />

          <div class="col-head" style="margin-top:28px"><h2 class="col-title">Zeiterfassung</h2></div>
          <div v-if="entries.length" class="tasklist">
            <div v-for="e in entries" :key="e.id" class="task-row">
              <span class="t">{{ entryLabel(e) }}</span>
              <input class="input te-time" type="time" :value="hhmm(e.started_at)" aria-label="Start" @change="setTime(e, 'started_at', ($event.target as HTMLInputElement).value)" />
              <span class="due">–</span>
              <input v-if="e.ended_at" class="input te-time" type="time" :value="hhmm(e.ended_at)" aria-label="Ende" @change="setTime(e, 'ended_at', ($event.target as HTMLInputElement).value)" />
              <span v-else class="due te-time">läuft</span>
              <span v-if="e.ended_at" class="row-act"><DeleteButton ghost text="Zeiteintrag löschen?" @confirm="removeEntry(e)" /></span>
            </div>
          </div>
          <details class="manual">
            <summary class="btn btn-ghost">Zeit nachtragen</summary>
            <form class="manual-form" @submit.prevent="addEntry" @input="saved = false">
              <label class="field wide"><span>Task</span>
                <select v-model="manual.task" class="input" required>
                  <option value="" disabled>Task wählen</option>
                  <option v-for="[id, title] in taskTitles" :key="id" :value="id">{{ title }}</option>
                </select>
              </label>
              <label class="field"><span>Datum</span><input v-model="manual.date" class="input" type="date" required /></label>
              <label class="field"><span>Start</span><input v-model="manual.from" class="input" type="time" required /></label>
              <label class="field"><span>Ende</span><input v-model="manual.to" class="input" type="time" required /></label>
              <button type="submit" class="btn btn-secondary">Eintragen</button>
              <span v-if="saved" class="v-sub" role="status">Gespeichert.</span>
            </form>
          </details>

          <template v-if="today.habits.length">
            <div class="col-head" style="margin-top:28px"><h2 class="col-title">Gewohnheiten</h2><router-link class="col-link" to="/habits">Alle ansehen</router-link></div>
            <div>
              <div v-for="h in today.habits" :key="h.id" class="hb-row" :class="{ doneToday: h.done }">
                <button
                  type="button" class="cb" role="checkbox" :aria-checked="h.done" :aria-label="h.name"
                  @click="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
                ></button>
                <span class="hb-name">{{ h.name }}</span>
                <span v-if="h.week_progress" class="hb-streak">{{ h.week_progress.done }}/{{ h.week_progress.target }}</span>
              </div>
            </div>
          </template>
        </section>

        <section class="start-col">
          <div class="col-head"><h2 class="col-title">Tagesplan</h2><router-link class="col-link" to="/calendar">Woche<svg class="ic" aria-hidden="true"><use href="#i-right" /></svg></router-link></div>
          <div class="v-sub">Aufgaben aus der Liste in den Plan ziehen, um sie einzuplanen.</div>
          <div class="card dayplan">
            <div class="dp-scroll">
              <div class="dp-body">
                <div class="hourcol" :style="{ '--hour': HOUR + 'px' }">
                  <div v-for="h in hours" :key="h" class="hl"><span class="mono">{{ String(h).padStart(2, '0') }}</span></div>
                </div>
                <div class="dp-grid" :style="{ height: (END_H - START_H) * HOUR + 'px' }" @dragover.prevent @drop.prevent="dropTask">
                  <div v-for="h in hours" :key="h" class="hline" :style="{ top: (h - START_H) * HOUR + 'px' }"></div>
                  <div v-if="nowTop !== null" class="now-line" :style="{ top: nowTop + 'px' }"></div>
                  <div
                    v-for="b in blocks" :key="b.key" class="dp-ev" :class="[`kind-${b.kind}`, { done: b.done, now: b.running, short: b.end - b.start <= 30 }]" :style="style(b)"
                  >
                    <div class="dp-t"><span class="cdot"></span><span class="dp-n">{{ b.title }}</span></div>
                    <div class="dp-m">{{ hm(b.start) }}–{{ hm(b.end) }}<template v-if="b.sub"> · {{ b.sub }}</template></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<style scoped>
.day-title { font: 600 22px/1.2 var(--font-ui); letter-spacing: -0.015em; color: var(--tx-primary); }
.fm { position: relative; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px; align-items: center; padding: 22px 28px 22px 31px; background: var(--bg-1); border: 1px solid var(--br-default); border-radius: var(--r-m); overflow: hidden; }
.fm.running::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 3px; background: var(--a-now); }
.fm-main { display: grid; gap: 10px; justify-items: start; min-width: 0; }
.fm-lbl { display: inline-flex; align-items: center; gap: 7px; height: 26px; padding: 0 10px; border-radius: var(--r-s); background: var(--bg-2); font: 400 13px/1 var(--font-ui); color: var(--tx-secondary); }
.run-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--a-now); }
.fm-t { font: 600 24px/1.25 var(--font-ui); letter-spacing: -0.015em; color: var(--tx-primary); }
.fm-t.is-free { color: var(--tx-secondary); font-weight: 500; }
.fm-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 16px; font: 400 13px/1.3 var(--font-ui); color: var(--tx-secondary); }
.fm-side { display: flex; flex-direction: column; align-items: flex-end; gap: 14px; }
.fm-timer { font: 500 20px/1 var(--font-mono); font-variant-numeric: tabular-nums; color: var(--tx-primary); }
.focus-next.small .fn-when { font: 500 14px/1.4 var(--font-ui); letter-spacing: 0; color: var(--tx-secondary); }
.focus-next.small .fn-t { font-size: 14px; }
@media (max-width: 720px) { .fm { grid-template-columns: 1fr; } .fm-side { align-items: flex-start; } }
.te-time { width: 88px; flex: none; text-align: center; padding: 0 4px; background: transparent; border-color: transparent; }
.te-time:hover, .te-time:focus-visible { background: var(--bg-2); }
.te-time::-webkit-calendar-picker-indicator { display: none; }
.task-row[draggable='true'] { cursor: grab; }
.manual { margin-top: 12px; }
.manual-form { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; margin-top: 12px; }
.manual-form .field { width: 140px; }
.manual-form .field.wide { width: 100%; }
.now-line { position: absolute; left: 0; right: 0; height: 1.5px; background: var(--tx-primary); z-index: 3; pointer-events: none; }
.now-line::before { content: ''; position: absolute; left: -4px; top: -2.75px; width: 7px; height: 7px; border-radius: 50%; background: var(--tx-primary); }
</style>
