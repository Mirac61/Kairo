<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  completeHabit, createTask, deleteTimeEntry, errorMessage, getToday, listTasks, listTimeEntries, uncompleteHabit, updateTask, updateTimeEntry,
  type Task, type TimeEntry, type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { projectColor } from '@/lib/projectColor'
import TaskActions from '@/components/TaskActions.vue'
import DeleteButton from '@/components/DeleteButton.vue'

// Planungsfenster der Tagesleiste und des Tagesplans.
const START_H = 7
const END_H = 22
const HOUR = 44
const SPAN = (END_H - START_H) * 60

const today = ref<Today | null>(null)
const error = ref('')
const now = ref(Date.now())
const quick = ref('')
const entries = ref<TimeEntry[]>([])
const taskTitles = ref(new Map<string, string>())
let tick: ReturnType<typeof setInterval> | undefined

async function load() {
  try {
    const t = await getToday()
    const from = new Date(`${t.date}T00:00:00`)
    const [es, tasks] = await Promise.all([listTimeEntries(from, new Date(from.getTime() + 864e5)), listTasks()])
    today.value = t
    entries.value = es.reverse()
    taskTitles.value = new Map(tasks.map((x) => [x.id, x.title]))
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
const hm = (m: number) => `${Math.floor(m / 60)}:${String(m % 60).padStart(2, '0')}`

const greeting = computed(() => {
  const h = new Date(now.value).getHours()
  return h < 11 ? 'Guten Morgen' : h < 18 ? 'Guten Tag' : 'Guten Abend'
})
const dateLabel = computed(() =>
  new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(now.value)))

const nowMin = computed(() => minsOf(now.value))
const runningTaskId = computed(() => today.value?.running_time_entry?.task_id ?? null)
const elapsed = computed(() => {
  const e = today.value?.running_time_entry
  if (!e) return ''
  const s = Math.max(0, Math.floor((now.value - Date.parse(e.started_at)) / 1000))
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(Math.floor(s / 3600))}:${p(Math.floor(s / 60) % 60)}:${p(s % 60)}`
})

interface Block { key: string; title: string; start: number; end: number; kind: 'event' | 'task'; done: boolean; color: string; sub: string }

// Termine und Tasks mit Uhrzeit als Blöcke; Tasks ohne Dauer zählen 30 Minuten.
const blocks = computed<Block[]>(() => {
  const t = today.value
  if (!t) return []
  const out: Block[] = t.events.map((e) => ({
    key: `e${e.id}${e.occurrence_start}`, title: e.title, kind: 'event', done: false, color: 'var(--a-blue)', sub: e.location,
    start: minsOf(Date.parse(e.occurrence_start)), end: minsOf(Date.parse(e.occurrence_end)) || 24 * 60,
  }))
  for (const task of t.tasks) {
    if (!task.planned_start_at) continue
    const start = minsOf(Date.parse(task.planned_start_at))
    out.push({
      key: `t${task.id}`, title: task.title, kind: 'task', done: task.status === 'COMPLETED', sub: '',
      color: task.project_id ? projectColor(task.project_id) : 'var(--a-green)', start, end: start + (task.estimated_minutes || 30),
    })
  }
  return out.sort((a, b) => a.start - b.start)
})

const upcoming = computed(() => blocks.value.filter((b) => b.kind === 'event' && !b.done))
const current = computed(() => upcoming.value.find((b) => b.start <= nowMin.value && b.end > nowMin.value))
const next = computed(() => upcoming.value.find((b) => b.start > nowMin.value))

// Tagesleiste: belegte Zeit der Termine im Planungsfenster.
const loadBar = computed(() => {
  const von = START_H * 60
  const bis = END_H * 60
  const parts: { w: number; busy?: boolean; now?: boolean }[] = []
  let cursor = von
  for (const b of upcoming.value) {
    const s = Math.max(b.start, von, cursor)
    const e = Math.min(b.end, bis)
    if (e <= s) continue
    if (s > cursor) parts.push({ w: ((s - cursor) / SPAN) * 100 })
    parts.push({ w: ((e - s) / SPAN) * 100, busy: true })
    cursor = e
  }
  if (cursor < bis) parts.push({ w: ((bis - cursor) / SPAN) * 100 })
  return parts
})

const openTasks = computed(() => (today.value?.tasks ?? []).filter((t) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'))
const groups = computed(() => [
  { title: 'Mit Uhrzeit', tasks: openTasks.value.filter((t) => t.planned_start_at) },
  { title: 'Ohne Uhrzeit', tasks: openTasks.value.filter((t) => !t.planned_start_at) },
  { title: 'Aktiv, nicht für heute geplant', tasks: today.value?.active_tasks ?? [] },
].filter((g) => g.tasks.length))
const doneTasks = computed(() => (today.value?.tasks ?? []).filter((t) => t.status === 'COMPLETED'))

const y = (m: number) => ((m - START_H * 60) / 60) * HOUR
const style = (b: Block) => {
  const top = Math.max(y(b.start), 0)
  const height = Math.max(y(Math.min(b.end, END_H * 60)) - top, 22)
  return { top: `${top}px`, height: `${height}px`, '--acc': b.color }
}
const hours = Array.from({ length: END_H - START_H + 1 }, (_, i) => START_H + i)
const nowTop = computed(() => (nowMin.value >= START_H * 60 && nowMin.value <= END_H * 60 ? y(nowMin.value) : null))

function addQuick() {
  const title = quick.value.trim()
  if (!title || !today.value) return
  void run(async () => {
    await createTask({ title, planned_date: today.value!.date })
    quick.value = ''
  })
}
const taskTitleClass = (t: Task) => ({ done: t.status === 'COMPLETED' })

// Überfällige Task auf heute (0) oder morgen (1) verschieben; die alte Uhrzeit entfällt.
function moveTo(t: Task, days: number) {
  const d = new Date(`${today.value!.date}T12:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  void run(() => updateTask(t.id, { planned_date: d.toISOString().slice(0, 10), planned_start_at: '' }))
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
          <h1 class="v-title">{{ greeting }}</h1>
          <div class="v-sub mono">{{ dateLabel }}</div>
        </div>

        <div v-if="current" class="focus-next">
          <span class="fn-when">Läuft gerade</span>
          <span class="fn-what"><span class="cdot" style="background:var(--a-blue)"></span><span class="fn-t">{{ current.title }}</span><span class="fn-time">noch {{ dur(current.end - nowMin) }}</span></span>
        </div>
        <div v-else-if="next" class="focus-next">
          <span class="fn-when">In {{ dur(next.start - nowMin) }}</span>
          <span class="fn-what"><span class="cdot" style="background:var(--a-blue)"></span><span class="fn-t">{{ next.title }}</span><span class="fn-time">{{ hm(next.start) }}–{{ hm(next.end) }}</span></span>
        </div>
        <div v-else class="focus-next is-free">
          <span class="fn-when">{{ today.events.length ? 'Termine erledigt' : 'Keine Termine heute' }}</span>
          <span class="fn-what"><span class="fn-time">Der Rest des Tages gehört dir.</span></span>
        </div>

        <div class="dayload" role="img" :aria-label="`Tagesauslastung: ${dur(today.calendar_minutes)} Termine`">
          <span v-for="(p, i) in loadBar" :key="i" :class="{ 'dl-busy': p.busy }" :style="{ width: p.w + '%', '--acc': 'var(--a-blue)' }"></span>
        </div>

        <div class="focus-stats">
          <div class="fstat"><span class="n">{{ hm(today.free_minutes) }}<span class="u">h</span></span><span class="c">Frei</span></div>
          <div class="fstat"><span class="n">{{ hm(today.planned_minutes) }}<span class="u">h</span></span><span class="c">Geplant</span></div>
          <div class="fstat"><span class="n">{{ hm(today.tracked_minutes) }}<span class="u">h</span></span><span class="c">Erfasst</span></div>
          <div class="fstat"><span class="n">{{ openTasks.length }}</span><span class="c">Offen heute</span></div>
          <div v-if="today.running_time_entry" class="fstat"><span class="n">{{ elapsed }}</span><span class="c">Timer läuft</span></div>
        </div>
        <div v-if="today.overplanned_minutes" class="badge"><span class="cdot" style="background:var(--a-red)"></span>Überplant um {{ hm(today.overplanned_minutes) }} h</div>
      </section>

      <div class="start-grid">
        <section class="start-col">
          <div class="col-head"><h2 class="col-title">Aufgaben</h2><router-link class="col-link" to="/tasks">Alle ansehen</router-link></div>
          <div v-if="today.overdue.length" class="tgroup">
            <span class="lbl">Überfällig<span class="count">{{ today.overdue.length }}</span></span>
            <div class="card tasklist">
              <div v-for="t in today.overdue" :key="t.id" class="task-row">
                <span class="t">{{ t.title }}</span>
                <span class="due od">{{ t.planned_date }}</span>
                <button type="button" class="btn btn-ghost" @click="moveTo(t, 0)">→ Heute</button>
                <button type="button" class="btn btn-ghost" @click="moveTo(t, 1)">→ Morgen</button>
              </div>
            </div>
          </div>
          <div v-for="g in groups" :key="g.title" class="tgroup">
            <span class="lbl">{{ g.title }}<span class="count">{{ g.tasks.length }}</span></span>
            <div class="card tasklist">
              <div v-for="t in g.tasks" :key="t.id" class="task-row">
                <span class="t" :class="taskTitleClass(t)">{{ t.title }}</span>
                <span v-if="t.planned_start_at" class="due">{{ fmt(t.planned_start_at) }}</span>
                <TaskActions :task="t" :running="runningTaskId === t.id" @run="run" />
              </div>
            </div>
          </div>
          <div v-if="!groups.length" class="v-sub">Keine offenen Aufgaben für heute.</div>
          <div v-if="doneTasks.length" class="v-sub">{{ doneTasks.length }} erledigt</div>
          <form class="addrow" @submit.prevent="addQuick">
            <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
            <input v-model="quick" type="text" placeholder="Aufgabe für heute hinzufügen" aria-label="Aufgabe für heute hinzufügen" />
          </form>

          <template v-if="entries.length">
            <div class="col-head" style="margin-top:28px"><h2 class="col-title">Zeiterfassung</h2></div>
            <div class="card tasklist">
              <div v-for="e in entries" :key="e.id" class="task-row">
                <span class="t">{{ (e.task_id && taskTitles.get(e.task_id)) || 'Projektzeit' }}</span>
                <input class="input te-time" type="time" :value="hhmm(e.started_at)" aria-label="Start" @change="setTime(e, 'started_at', ($event.target as HTMLInputElement).value)" />
                <span class="due">–</span>
                <input v-if="e.ended_at" class="input te-time" type="time" :value="hhmm(e.ended_at)" aria-label="Ende" @change="setTime(e, 'ended_at', ($event.target as HTMLInputElement).value)" />
                <span v-else class="due te-time">läuft</span>
                <DeleteButton v-if="e.ended_at" text="Zeiteintrag löschen?" @confirm="run(() => deleteTimeEntry(e.id))" />
              </div>
            </div>
          </template>

          <template v-if="today.habits.length">
            <div class="col-head" style="margin-top:28px"><h2 class="col-title">Gewohnheiten</h2><router-link class="col-link" to="/habits">Alle ansehen</router-link></div>
            <div class="card" style="padding:4px 16px">
              <div v-for="h in today.habits" :key="h.id" class="hb-row" :class="{ doneToday: h.done }">
                <button
                  type="button" class="cb cb-orange" role="checkbox" :aria-checked="h.done" :aria-label="h.name"
                  @click="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
                ></button>
                <span class="hb-name">{{ h.name }}</span>
                <span v-if="h.week_progress" class="hb-streak">{{ h.week_progress.done }}/{{ h.week_progress.target }}</span>
              </div>
            </div>
          </template>
        </section>

        <section class="start-col">
          <div class="col-head"><h2 class="col-title">Tagesplan</h2><router-link class="col-link" to="/calendar">Woche ansehen</router-link></div>
          <div class="card dayplan">
            <div class="dp-scroll">
              <div class="dp-body">
                <div class="hourcol" :style="{ '--hour': HOUR + 'px' }">
                  <div v-for="h in hours" :key="h" class="hl"><span class="mono">{{ String(h).padStart(2, '0') }}:00</span></div>
                </div>
                <div class="dp-grid" :style="{ height: (END_H - START_H) * HOUR + 'px' }">
                  <div v-for="h in hours" :key="h" class="hline" :style="{ top: (h - START_H) * HOUR + 'px' }"></div>
                  <div v-if="nowTop !== null" class="now-line" :style="{ top: nowTop + 'px' }"></div>
                  <div
                    v-for="b in blocks" :key="b.key" class="dp-ev" :class="[`kind-${b.kind}`, { done: b.done, short: b.end - b.start <= 30 }]" :style="style(b)"
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
.te-time { width: 92px; flex: none; }
.now-line { position: absolute; left: 0; right: 0; height: 1px; background: var(--a-red); z-index: 3; pointer-events: none; }
.now-line::before { content: ''; position: absolute; left: -4px; top: -3px; width: 7px; height: 7px; border-radius: 50%; background: var(--a-red); }
</style>
