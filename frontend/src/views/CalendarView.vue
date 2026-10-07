<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import '@/calendar.css'
import { useRouter } from 'vue-router'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin, { Draggable, type EventResizeDoneArg } from '@fullcalendar/interaction'
import deLocale from '@fullcalendar/core/locales/de'
import type { CalendarOptions, EventClickArg, EventDropArg, EventInput } from '@fullcalendar/core'
import Message from 'primevue/message'
import {
  createEvent, createTask, deleteEvent, errorMessage, getOccurrences, importIcs, listProjects, listTasks, restoreEvent, skipOccurrence, updateEvent, updateTask,
  type CalendarEvent, type EventBody, type Project, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useUndo } from '@/composables/useUndo'
import { hhmm, ymd } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

const nextDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1)
// Letzter Tag eines Zeitraums mit exklusivem Ende.
const lastDay = (end: Date) => ymd(new Date(end.getFullYear(), end.getMonth(), end.getDate() - 1))
// Ganztägig = von Mitternacht bis Mitternacht (ab 23 h wegen der Zeitumstellung); das Backend kennt kein eigenes Flag.
const atMidnight = (d: Date) => d.getHours() === 0 && d.getMinutes() === 0
const isAllDay = (start: Date, end: Date) => atMidnight(start) && atMidnight(end) && end.getTime() - start.getTime() >= 23 * 3_600_000

const router = useRouter()
const { offer } = useUndo()
const cal = ref<InstanceType<typeof FullCalendar>>()
const error = ref('')
const notice = ref<{ text: string; notes: string[] } | null>(null)
const projects = ref<Project[]>([])
const unplanned = ref<Task[]>([]) // offene Tasks ohne planned_date: Quelle zum Ziehen

// Termine und Tasks als Kalendereinträge. Zeiten rechnen in der Zeitzone des Browsers.
async function loadEntries(from: Date, to: Date): Promise<EventInput[]> {
  const [occ, tasks] = await Promise.all([getOccurrences(from, to), listTasks()])
  unplanned.value = tasks.filter((t) => !t.planned_date && t.status !== 'COMPLETED' && t.status !== 'CANCELLED')
  const entries: EventInput[] = occ.map((e) => {
    const [s, en] = [new Date(e.occurrence_start), new Date(e.occurrence_end)]
    const allDay = isAllDay(s, en)
    return {
      id: `e:${e.id}:${e.occurrence_start}`,
      title: e.title,
      allDay,
      start: allDay ? ymd(s) : e.occurrence_start,
      end: allDay ? ymd(en) : e.occurrence_end,
      classNames: ['kt-event'],
      editable: !e.recurrence_rule, // bei Serien gehören start_at/end_at zur ersten Wiederholung
      extendedProps: { kind: 'event', id: e.id, ev: e, color: e.project_id ? projectColor(e.project_id) : undefined },
    }
  })
  const [fromDay, toDay] = [ymd(from), ymd(to)]
  for (const t of tasks) {
    if (!t.planned_date || t.status === 'CANCELLED' || t.planned_date < fromDay || t.planned_date >= toDay) continue
    const done = t.status === 'COMPLETED'
    const start = t.planned_start_at
    entries.push({
      id: `t:${t.id}`,
      title: t.title,
      start: start ?? t.planned_date,
      end: start ? new Date(Date.parse(start) + (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000) : undefined,
      allDay: !start,
      classNames: ['kt-task', ...(done ? ['kt-done'] : [])],
      editable: !done,
      extendedProps: { kind: 'task', id: t.id, est: t.estimated_minutes, color: t.project_id ? projectColor(t.project_id) : undefined },
    })
  }
  return entries
}

async function guarded(fn: () => Promise<unknown>, revert?: () => void) {
  try {
    await fn()
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
    revert?.()
  }
  cal.value?.getApi().refetchEvents()
}

function moved(info: EventDropArg | EventResizeDoneArg, resized: boolean) {
  const { event } = info
  const { kind, id, ev, est } = event.extendedProps as { kind: string; id: string; ev: CalendarEvent; est: number }
  const start = event.start
  const old = info.oldEvent
  if (!start || !old.start) return info.revert()
  const label = `„${event.title}“ ${resized ? 'angepasst' : 'verschoben'}`
  if (kind === 'task') {
    const before: Record<string, unknown> = { planned_date: ymd(old.start), planned_start_at: old.allDay ? '' : old.start.toISOString() }
    if (resized) before.estimated_minutes = est
    void guarded(async () => {
      const body: Record<string, unknown> = { planned_date: ymd(start), planned_start_at: event.allDay ? '' : start.toISOString() }
      if (resized && event.end) body.estimated_minutes = Math.round((event.end.getTime() - start.getTime()) / 60_000)
      await updateTask(id, body)
      offer(label, () => guarded(() => updateTask(id, before)))
    }, info.revert)
  } else {
    // Ganztägige Termine liefern evtl. kein Ende: dann genau ein Tag.
    const end = event.end ?? (event.allDay ? nextDay(start) : null)
    if (!end) return info.revert()
    const before = { start_at: ev.start_at, end_at: ev.end_at }
    void guarded(async () => {
      await updateEvent(id, { start_at: start.toISOString(), end_at: end.toISOString() })
      offer(label, () => guarded(() => updateEvent(id, before)))
    }, info.revert)
  }
}

// Ein Dialog zum Anlegen (Termin oder Task) und Bearbeiten von Terminen.
// Wiederholung: wöchentlich an Wochentagen, optional mit Enddatum (RRULE-Teilmenge des Backends).
const WEEKDAYS = [['MO', 'Mo'], ['TU', 'Di'], ['WE', 'Mi'], ['TH', 'Do'], ['FR', 'Fr'], ['SA', 'Sa'], ['SU', 'So']] as const
const CODE_OF_DAY = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'] // Index = Date.getDay()
const kindOptions = [{ label: 'Termin', value: 'event' }, { label: 'Task', value: 'task' }]

interface Form {
  id: string | null // gesetzt beim Bearbeiten eines Termins
  kind: string
  title: string
  location: string
  date: string
  from: string
  to: string
  allDay: boolean // Task: ohne Uhrzeit; Termin: ganztägig
  endDate: string // letzter Tag eines ganztägigen Termins
  keepTimes: boolean // mehrtägiger Termin mit Uhrzeit: Zeiten bleiben unverändert (der Dialog kennt nur einen Tag)
  projectId: string // '' = kein Projekt
  repeat: boolean
  days: string[]
  until: string
  customRule: string | null // Regel, die der Dialog nicht abbildet; bleibt unverändert
  day: string // Tag der angeklickten Instanz, nur bei Serien gesetzt
  ask: boolean // Löschen wartet auf Bestätigung
}
const form = ref<Form | null>(null)

function openForm(start: Date, end: Date, allDay: boolean) {
  form.value = {
    id: null, kind: 'event', title: '', location: '', date: ymd(start),
    from: allDay ? '09:00' : hhmm(start), to: allDay ? '10:00' : hhmm(end), allDay,
    endDate: allDay ? lastDay(end) : ymd(start), keepTimes: false, projectId: '',
    repeat: false, days: [CODE_OF_DAY[start.getDay()]!], until: '', customRule: null, day: '', ask: false,
  }
}

function openEdit(ev: CalendarEvent, day: string) {
  const [start, end] = [new Date(ev.start_at), new Date(ev.end_at)]
  const allDay = isAllDay(start, end)
  const f: Form = {
    id: ev.id, kind: 'event', title: ev.title, location: ev.location, date: ymd(start),
    from: allDay ? '09:00' : hhmm(start), to: allDay ? '10:00' : hhmm(end), allDay,
    endDate: allDay ? lastDay(end) : ymd(start), keepTimes: !allDay && ymd(end) !== ymd(start), projectId: ev.project_id ?? '',
    repeat: false, days: [CODE_OF_DAY[start.getDay()]!], until: '', customRule: null, day: ev.recurrence_rule ? day : '', ask: false,
  }
  const rule = ev.recurrence_rule
  const m = rule?.match(/^FREQ=WEEKLY;BYDAY=([A-Z,]+)(?:;UNTIL=(\d{4})(\d{2})(\d{2}))?$/)
  if (m) Object.assign(f, { repeat: true, days: m[1]!.split(','), until: m[2] ? `${m[2]}-${m[3]}-${m[4]}` : '' })
  else if (rule) f.customRule = rule
  form.value = f
}

const toggleDay = (code: string) => {
  const f = form.value!
  f.days = f.days.includes(code) ? f.days.filter((d) => d !== code) : WEEKDAYS.map(([c]) => c).filter((c) => c === code || f.days.includes(c))
}

function ruleOf(f: Form): string {
  if (!f.repeat) return ''
  const days = f.days.length ? f.days : [CODE_OF_DAY[new Date(`${f.date}T00:00:00`).getDay()]!]
  return `FREQ=WEEKLY;BYDAY=${days.join(',')}${f.until ? `;UNTIL=${f.until.replaceAll('-', '')}` : ''}`
}

function save() {
  const f = form.value
  const title = f?.title.trim()
  if (!f || !title) return
  const start = new Date(`${f.date}T${f.allDay ? '00:00' : f.from}`)
  const end = f.allDay ? nextDay(new Date(`${f.endDate || f.date}T00:00`)) : new Date(`${f.date}T${f.to}`)
  form.value = null
  void guarded(() => {
    if (f.kind === 'task') {
      return createTask({
        title,
        planned_date: f.date,
        planned_start_at: f.allDay ? null : start.toISOString(),
        project_id: f.projectId || undefined,
        estimated_minutes: f.allDay ? 0 : Math.max(0, Math.round((end.getTime() - start.getTime()) / 60_000)),
      })
    }
    const body: EventBody = { title, location: f.location, project_id: f.projectId }
    if (!f.keepTimes) Object.assign(body, { start_at: start.toISOString(), end_at: end.toISOString() })
    if (f.customRule !== null) return updateEvent(f.id!, body)
    const rule = ruleOf(f)
    return f.id ? updateEvent(f.id, { ...body, recurrence_rule: rule }) : createEvent({ ...body, recurrence_rule: rule || undefined })
  })
}

// Klick auf einen Termin öffnet den Dialog; ein Klick auf eine Task öffnet sie in der Tasks-Liste.
function clicked(info: EventClickArg) {
  const p = info.event.extendedProps as { kind: string; id: string; ev: CalendarEvent }
  if (p.kind === 'task') void router.push({ path: '/tasks', query: { task: p.id } })
  else openEdit(p.ev, ymd(info.event.start!))
}

// Bei Serien nur die angeklickte Instanz auslassen (recurrence_exdates) oder die ganze Serie löschen.
function remove(onlyThis: boolean) {
  const f = form.value
  form.value = null
  if (!f?.id) return
  const { id, day, title } = f
  void guarded(async () => {
    if (onlyThis) {
      const before = await skipOccurrence(id, day)
      offer(`„${title}“ gelöscht`, () => guarded(() => updateEvent(id, { recurrence_exdates: before })))
    } else {
      await deleteEvent(id)
      offer(`„${title}“ gelöscht`, () => guarded(() => restoreEvent(id)))
    }
  })
}

// Eigene Toolbar steuert FullCalendar über die API.
const title = ref('')
const view = ref('timeGridWeek')
const views = [['dayGridMonth', 'Monat'], ['timeGridWeek', 'Woche'], ['timeGridDay', 'Tag']] as const

function openNew() {
  const start = new Date()
  start.setHours(start.getHours() + 1, 0, 0, 0)
  openForm(start, new Date(start.getTime() + 3_600_000), false)
}

// ICS-Import: Dateiinhalt an das Backend; Termine mit bekannter UID werden aktualisiert.
async function importFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // dieselbe Datei darf erneut gewählt werden
  if (!file) return
  try {
    const r = await importIcs(await file.text())
    const parts = [`${r.created} neu`, `${r.updated} aktualisiert`]
    if (r.skipped) parts.push(`${r.skipped} übersprungen`)
    if (r.unsupported_rules) parts.push(`${r.unsupported_rules} Serien als Einzeltermin`)
    notice.value = { text: parts.join(', '), notes: r.notes }
    error.value = ''
    cal.value?.getApi().refetchEvents()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

const projectOptions = computed(() => projects.value.filter((p) => p.status !== 'ARCHIVED' || p.id === form.value?.projectId))
const loadProjects = () => listProjects().then((ps) => (projects.value = ps), () => {})

// Ungeplante Tasks lassen sich in den Kalender ziehen; kein Termin wird angelegt, nur die Task geplant.
const unplannedEl = ref<HTMLElement>()
const fileInput = ref<HTMLInputElement>()
let draggable: Draggable | undefined
onMounted(() => {
  void loadProjects()
  draggable = new Draggable(unplannedEl.value!, {
    itemSelector: '.up-item',
    eventData: (el) => ({ create: false, duration: { minutes: Number(el.dataset.min) || DEFAULT_TASK_MINUTES } }),
  })
})
onBeforeUnmount(() => draggable?.destroy())

const options: CalendarOptions = {
  plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
  locales: [deLocale],
  locale: 'de',
  firstDay: 1,
  allDayText: 'Ganztag',
  initialView: 'timeGridWeek',
  headerToolbar: false,
  height: '100%',
  nowIndicator: true,
  slotMinTime: '06:00:00',
  slotMaxTime: '23:00:00',
  scrollTime: '08:00:00',
  datesSet: (a) => {
    title.value = a.view.title
    view.value = a.view.type
  },
  dayHeaderContent: (a) => {
    const wd = a.date.toLocaleDateString('de-DE', { weekday: 'short' }).replace('.', '')
    const html = a.view.type === 'dayGridMonth'
      ? wd
      : `<span class="kt-dh${a.isToday ? ' today' : ''}"><small>${wd}</small><b>${a.date.getDate()}</b></span>`
    return { html }
  },
  expandRows: true,
  snapDuration: '00:15:00',
  slotLabelFormat: { hour: '2-digit', minute: '2-digit', hour12: false },
  displayEventEnd: false,
  eventTimeFormat: { hour: '2-digit', minute: '2-digit', hour12: false },
  selectable: true,
  selectMirror: true,
  dayMaxEvents: true,
  events: (info, ok, fail) => {
    loadEntries(info.start, info.end).then(ok, (e) => {
      error.value = errorMessage(e)
      fail(e)
    })
  },
  select: (info) => openForm(info.start, info.end, info.allDay),
  eventDrop: (info) => moved(info, false),
  eventResize: (info) => moved(info, true),
  eventClick: clicked,
  eventDidMount: (a) => {
    const color = a.event.extendedProps.color as string | undefined
    if (color) a.el.style.setProperty('--acc', color) // Projektfarbe
  },
  droppable: true,
  drop: (a) => {
    const id = a.draggedEl.dataset.id
    if (!id) return
    // Uhrzeit nur bei einem Drop ins Zeitraster; in Monat und Ganztag zählt der Tag.
    const body: Record<string, unknown> = { planned_date: ymd(a.date) }
    if (!a.allDay) body.planned_start_at = a.date.toISOString()
    const title = unplanned.value.find((t) => t.id === id)?.title
    void guarded(async () => {
      await updateTask(id, body)
      offer(`„${title}“ geplant`, () => guarded(() => updateTask(id, { planned_date: '', planned_start_at: '' })))
    })
  },
}

useLiveEvents(() => {
  cal.value?.getApi().refetchEvents()
  void loadProjects()
})
</script>

<template>
  <div class="view-inner page">
    <Message v-if="error" severity="error">{{ error }}</Message>
    <Message v-if="notice" severity="success" closable @close="notice = null">
      {{ notice.text }}
      <ul v-if="notice.notes.length" class="notes"><li v-for="n in notice.notes" :key="n">{{ n }}</li></ul>
    </Message>
    <div class="cal-wrap">
    <div class="cal">
      <div class="cal-toolbar">
        <div class="cal-nav">
          <button class="icon-btn" aria-label="Zurück" @click="cal?.getApi().prev()"><svg class="ic"><use href="#i-left" /></svg></button>
          <button class="icon-btn" aria-label="Weiter" @click="cal?.getApi().next()"><svg class="ic"><use href="#i-right" /></svg></button>
        </div>
        <button class="btn btn-ghost" @click="cal?.getApi().today()">Heute</button>
        <h2 class="cal-title">{{ title }}</h2>
        <span class="spacer" />
        <div class="seg">
          <button v-for="[v, l] in views" :key="v" :aria-pressed="view === v" @click="cal?.getApi().changeView(v)">{{ l }}</button>
        </div>
        <input ref="fileInput" type="file" accept=".ics,text/calendar" hidden @change="importFile" />
        <button class="btn btn-secondary" @click="fileInput?.click()">ICS importieren</button>
        <button class="btn btn-primary" @click="openNew"><svg class="ic"><use href="#i-plus" /></svg>Neuer Eintrag</button>
      </div>
      <div class="cal-fc"><FullCalendar ref="cal" :options="options" /></div>
    </div>
    <aside class="unplanned" aria-labelledby="up-title">
      <h3 id="up-title">Ungeplant <span class="up-count">{{ unplanned.length }}</span></h3>
      <p class="up-hint">Task in den Kalender ziehen, um sie zu planen.</p>
      <div ref="unplannedEl" class="up-list">
        <div v-for="t in unplanned" :key="t.id" class="up-item" :data-id="t.id" :data-min="t.estimated_minutes || ''">
          <span class="up-dot" :style="{ background: projectColor(t.project_id) }" />
          <span class="up-name">{{ t.title }}</span>
          <small v-if="t.estimated_minutes" class="up-min">{{ t.estimated_minutes }} min</small>
        </div>
      </div>
    </aside>
    </div>

    <div v-if="form" class="overlay open" @mousedown.self="form = null" @keydown.esc="form = null">
      <form class="dialog" role="dialog" aria-modal="true" aria-labelledby="cal-dlg-title" @submit.prevent="save">
        <div class="dlg-head"><h3 id="cal-dlg-title">{{ form.id ? 'Termin bearbeiten' : 'Neuer Eintrag' }}</h3></div>
        <div class="dlg-body">
          <div v-if="!form.id" class="seg">
            <button v-for="o in kindOptions" :key="o.value" type="button" :aria-pressed="form.kind === o.value" @click="form.kind = o.value">{{ o.label }}</button>
          </div>
          <div class="field"><label>Titel</label><input v-model="form.title" class="input" placeholder="Titel" autofocus /></div>
          <template v-if="form.kind === 'event'">
            <label class="lbl"><input v-model="form.allDay" type="checkbox" :disabled="form.keepTimes" /> Ganztägig</label>
            <div class="dlg-row">
              <div class="field"><label>{{ form.repeat || form.customRule ? 'Erster Termin' : 'Datum' }}</label><input v-model="form.date" class="input" type="date" required /></div>
              <div v-if="form.allDay" class="field"><label>Bis (einschließlich)</label><input v-model="form.endDate" class="input" type="date" :min="form.date" required /></div>
              <template v-else-if="!form.keepTimes">
                <div class="field"><label>Von</label><input v-model="form.from" class="input" type="time" required /></div>
                <div class="field"><label>Bis</label><input v-model="form.to" class="input" type="time" required /></div>
              </template>
            </div>
            <span v-if="form.keepTimes" class="lbl">Mehrtägiger Termin: Beginn und Ende bleiben unverändert.</span>
            <div class="field"><label>Ort</label><input v-model="form.location" class="input" placeholder="optional" /></div>
            <span v-if="form.customRule" class="lbl">Serie mit eigener Regel ({{ form.customRule }}); sie bleibt unverändert.</span>
            <template v-else>
              <label class="lbl"><input v-model="form.repeat" type="checkbox" /> Wöchentlich wiederholen</label>
              <template v-if="form.repeat">
                <div class="seg" role="group" aria-label="Wochentage">
                  <button v-for="[code, label] in WEEKDAYS" :key="code" type="button" :aria-pressed="form.days.includes(code)" @click="toggleDay(code)">{{ label }}</button>
                </div>
                <div class="field"><label>Endet am</label><input v-model="form.until" class="input" type="date" :min="form.date" /></div>
              </template>
            </template>
            <span v-if="form.id && (form.repeat || form.customRule)" class="lbl">Änderungen gelten für die ganze Serie.</span>
          </template>
          <div class="field">
            <label for="cal-project">Projekt</label>
            <select id="cal-project" v-model="form.projectId" class="input">
              <option value="">Kein Projekt</option>
              <option v-for="p in projectOptions" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
        </div>
        <div v-if="form.ask" class="dlg-foot">
          <span class="q">{{ form.day ? 'Nur diesen Termin oder die ganze Serie löschen?' : 'Termin löschen?' }}</span>
          <button type="button" class="btn btn-ghost" @click="form.ask = false">Abbrechen</button>
          <button v-if="form.day" type="button" class="btn btn-danger" @click="remove(true)">Nur dieser Termin</button>
          <button type="button" class="btn btn-danger" @click="remove(false)">{{ form.day ? 'Ganze Serie' : 'Löschen' }}</button>
        </div>
        <div v-else class="dlg-foot">
          <button v-if="form.id" type="button" class="btn btn-secondary" @click="form.ask = true">Löschen</button>
          <span class="spacer" />
          <button type="button" class="btn btn-ghost" @click="form = null">Abbrechen</button>
          <button type="submit" class="btn btn-primary">{{ form.id ? 'Speichern' : 'Anlegen' }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; display: flex; flex-direction: column; gap: 12px; }
.notes { margin: 6px 0 0; padding-left: 18px; max-height: 120px; overflow: auto; font-size: 12px; }
.cal-wrap { flex: 1; min-height: 0; display: flex; gap: 12px; }
.cal { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; padding: 0; }
.unplanned {
  flex: none; width: 240px; display: flex; flex-direction: column; min-height: 0; padding: 12px;
  background: var(--bg-1); border: 1px solid var(--br-default); border-radius: var(--r-m);
}
.unplanned h3 { margin: 0; font: 600 14px/1.3 var(--font-ui); color: var(--tx-primary); }
.up-count { font: 400 12px/1 var(--font-mono); color: var(--tx-muted); }
.up-hint { margin: 4px 0 10px; font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }
.up-list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 6px; }
.up-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 8px; cursor: grab;
  background: var(--bg-0); border: 1px dashed var(--br-default); border-radius: var(--r-s);
  font: 400 13px/1.3 var(--font-ui); color: var(--tx-primary);
}
.up-item:hover { border-color: var(--br-strong); }
.up-dot { flex: none; width: 8px; height: 8px; border-radius: 50%; }
.up-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.up-min { flex: none; font: 400 11px/1 var(--font-mono); color: var(--tx-muted); }
@media (max-width: 900px) {
  .cal-wrap { flex-direction: column; }
  .unplanned { width: auto; max-height: 180px; order: 2; }
}
.cal-fc { flex: 1; min-height: 0; }
.dlg-row { display: flex; gap: 12px; }
.dlg-foot { flex-wrap: wrap; }
.dlg-foot .spacer { flex: 1; }
.dlg-foot .q { flex: 1 0 100%; font-size: 13px; color: var(--tx-secondary); }
.dlg-row .field { flex: 1; }
</style>
