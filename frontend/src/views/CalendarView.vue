<script setup lang="ts">
import { ref } from 'vue'
import '@/calendar.css'
import { useRouter } from 'vue-router'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin, { type EventResizeDoneArg } from '@fullcalendar/interaction'
import deLocale from '@fullcalendar/core/locales/de'
import type { CalendarOptions, EventClickArg, EventDropArg, EventInput } from '@fullcalendar/core'
import Message from 'primevue/message'
import {
  createEvent, createTask, deleteEvent, errorMessage, getOccurrences, listTasks, updateEvent, updateTask,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { hhmm, ymd } from '@/lib/dates'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

// Projektfarbe: feste Reihenfolge, je Projekt-ID-Hash (Klassen kt-p0..4 in calendar.css).
function projectClass(id: string | null): string[] {
  if (!id) return []
  let h = 0
  for (const c of id) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return [`kt-p${h % 5}`]
}

const router = useRouter()
const cal = ref<InstanceType<typeof FullCalendar>>()
const error = ref('')

// Termine und Tasks als Kalendereinträge. Zeiten rechnen in der Zeitzone des Browsers.
async function loadEntries(from: Date, to: Date): Promise<EventInput[]> {
  const [occ, tasks] = await Promise.all([getOccurrences(from, to), listTasks()])
  const entries: EventInput[] = occ.map((e) => ({
    id: `e:${e.id}:${e.occurrence_start}`,
    title: e.title,
    start: e.occurrence_start,
    end: e.occurrence_end,
    classNames: ['kt-event'],
    editable: !e.recurrence_rule, // bei Serien gehören start_at/end_at zur ersten Wiederholung
    extendedProps: { kind: 'event', id: e.id, location: e.location, startAt: e.start_at, endAt: e.end_at, rule: e.recurrence_rule },
  }))
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
      classNames: ['kt-task', ...projectClass(t.project_id), ...(done ? ['kt-done'] : [])],
      editable: !done,
      extendedProps: { kind: 'task', id: t.id },
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
  const { kind, id } = event.extendedProps as { kind: string; id: string }
  const start = event.start
  if (!start) return info.revert()
  if (kind === 'task') {
    void guarded(() => {
      const body: Record<string, unknown> = { planned_date: ymd(start), planned_start_at: event.allDay ? '' : start.toISOString() }
      if (resized && event.end) body.estimated_minutes = Math.round((event.end.getTime() - start.getTime()) / 60_000)
      return updateTask(id, body)
    }, info.revert)
  } else {
    if (!event.end || event.allDay) return info.revert() // Termine brauchen Beginn und Ende im Zeitraster
    const end = event.end
    void guarded(() => updateEvent(id, { start_at: start.toISOString(), end_at: end.toISOString() }), info.revert)
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
  allDay: boolean // nur für Tasks: ohne Uhrzeit
  repeat: boolean
  days: string[]
  until: string
  customRule: string | null // Regel, die der Dialog nicht abbildet; bleibt unverändert
}
const form = ref<Form | null>(null)

function openForm(start: Date, end: Date, allDay: boolean) {
  form.value = {
    id: null, kind: 'event', title: '', location: '', date: ymd(start),
    from: allDay ? '09:00' : hhmm(start), to: allDay ? '10:00' : hhmm(end), allDay,
    repeat: false, days: [CODE_OF_DAY[start.getDay()]!], until: '', customRule: null,
  }
}

function openEdit(id: string, title: string, location: string, startAt: string, endAt: string, rule: string | null) {
  const start = new Date(startAt)
  const f: Form = {
    id, kind: 'event', title, location, date: ymd(start), from: hhmm(start), to: hhmm(new Date(endAt)), allDay: false,
    repeat: false, days: [CODE_OF_DAY[start.getDay()]!], until: '', customRule: null,
  }
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
  const start = new Date(`${f.date}T${f.from}`)
  const end = new Date(`${f.date}T${f.to}`)
  form.value = null
  void guarded(() => {
    if (f.kind === 'task') {
      return createTask({
        title,
        planned_date: f.date,
        planned_start_at: f.allDay ? null : start.toISOString(),
        estimated_minutes: f.allDay ? 0 : Math.max(0, Math.round((end.getTime() - start.getTime()) / 60_000)),
      })
    }
    const body = { title, location: f.location, start_at: start.toISOString(), end_at: end.toISOString() }
    if (f.customRule !== null) return updateEvent(f.id!, body)
    const rule = ruleOf(f)
    return f.id ? updateEvent(f.id, { ...body, recurrence_rule: rule }) : createEvent({ ...body, recurrence_rule: rule || undefined })
  })
}

// Klick auf einen Termin öffnet den Dialog; ein Klick auf eine Task öffnet die Tasks-Liste.
function clicked(info: EventClickArg) {
  const p = info.event.extendedProps as { kind: string; id: string; location: string; startAt: string; endAt: string; rule: string | null }
  if (p.kind === 'task') void router.push('/tasks')
  else openEdit(p.id, info.event.title, p.location, p.startAt, p.endAt, p.rule)
}

function remove() {
  const id = form.value?.id
  form.value = null
  if (id) void guarded(() => deleteEvent(id))
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
}

useLiveEvents(() => cal.value?.getApi().refetchEvents())
</script>

<template>
  <div class="view-inner page">
    <Message v-if="error" severity="error">{{ error }}</Message>
    <div class="cal">
      <div class="cal-toolbar">
        <div class="cal-nav">
          <button class="icon-btn" aria-label="Zurück" @click="cal?.getApi().prev()"><svg width="16" height="16"><use href="#i-left" /></svg></button>
          <button class="icon-btn" aria-label="Weiter" @click="cal?.getApi().next()"><svg width="16" height="16"><use href="#i-right" /></svg></button>
        </div>
        <button class="btn btn-ghost" @click="cal?.getApi().today()">Heute</button>
        <h2 class="cal-title">{{ title }}</h2>
        <span class="spacer" />
        <div class="seg">
          <button v-for="[v, l] in views" :key="v" :aria-pressed="view === v" @click="cal?.getApi().changeView(v)">{{ l }}</button>
        </div>
        <button class="btn btn-primary" @click="openNew"><svg width="16" height="16"><use href="#i-plus" /></svg>Neuer Eintrag</button>
      </div>
      <div class="cal-fc"><FullCalendar ref="cal" :options="options" /></div>
    </div>

    <div v-if="form" class="overlay open" @mousedown.self="form = null" @keydown.esc="form = null">
      <form class="dialog" @submit.prevent="save">
        <div class="dlg-head"><h3>{{ form.id ? 'Termin bearbeiten' : 'Neuer Eintrag' }}</h3></div>
        <div class="dlg-body">
          <div v-if="!form.id" class="seg">
            <button v-for="o in kindOptions" :key="o.value" type="button" :aria-pressed="form.kind === o.value" @click="form.kind = o.value">{{ o.label }}</button>
          </div>
          <div class="field"><label>Titel</label><input v-model="form.title" class="input" placeholder="Titel" autofocus /></div>
          <template v-if="form.kind === 'event'">
            <div class="dlg-row">
              <div class="field"><label>{{ form.repeat || form.customRule ? 'Erster Termin' : 'Datum' }}</label><input v-model="form.date" class="input" type="date" required /></div>
              <div class="field"><label>Von</label><input v-model="form.from" class="input" type="time" required /></div>
              <div class="field"><label>Bis</label><input v-model="form.to" class="input" type="time" required /></div>
            </div>
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
            <span v-if="form.id && (form.repeat || form.customRule)" class="lbl">Änderungen und Löschen gelten für die ganze Serie.</span>
          </template>
        </div>
        <div class="dlg-foot">
          <button v-if="form.id" type="button" class="btn btn-secondary" @click="remove">Löschen</button>
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
.cal { flex: 1; min-height: 0; display: flex; flex-direction: column; padding: 0; }
.cal-fc { flex: 1; min-height: 0; }
.dlg-row { display: flex; gap: 12px; }
.dlg-foot .spacer { flex: 1; }
.dlg-row .field { flex: 1; }
</style>
