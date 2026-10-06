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
import { ymd } from '@/lib/dates'

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
    extendedProps: { kind: 'event', id: e.id, location: e.location, recurring: !!e.recurrence_rule },
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

// Anlegen per Markieren: Dialog fragt Titel und Art.
const sel = ref<{ start: Date; end: Date; allDay: boolean } | null>(null)
const form = ref({ kind: 'event', title: '' })
const kindOptions = [{ label: 'Termin', value: 'event' }, { label: 'Task', value: 'task' }]

function save() {
  const s = sel.value
  const title = form.value.title.trim()
  if (!s || !title) return
  void guarded(() => {
    if (form.value.kind === 'task') {
      const minutes = Math.round((s.end.getTime() - s.start.getTime()) / 60_000)
      return createTask({
        title,
        planned_date: ymd(s.start),
        planned_start_at: s.allDay ? null : s.start.toISOString(),
        estimated_minutes: s.allDay ? 0 : minutes,
      })
    }
    return createEvent({ title, start_at: s.start.toISOString(), end_at: s.end.toISOString() })
  })
  sel.value = null
}

// Klick auf einen Termin zeigt Details und Löschen; ein Klick auf eine Task öffnet die Tasks-Liste.
const picked = ref<{ id: string; title: string; location: string; recurring: boolean } | null>(null)

function clicked(info: EventClickArg) {
  const p = info.event.extendedProps as { kind: string; id: string; location: string; recurring: boolean }
  if (p.kind === 'task') void router.push('/tasks')
  else picked.value = { id: p.id, title: info.event.title, location: p.location, recurring: p.recurring }
}

function remove() {
  const p = picked.value
  picked.value = null
  if (p) void guarded(() => deleteEvent(p.id))
}

// Eigene Toolbar steuert FullCalendar über die API.
const title = ref('')
const view = ref('timeGridWeek')
const views = [['dayGridMonth', 'Monat'], ['timeGridWeek', 'Woche'], ['timeGridDay', 'Tag']] as const

function openNew() {
  const start = new Date()
  start.setHours(start.getHours() + 1, 0, 0, 0)
  form.value = { kind: 'event', title: '' }
  sel.value = { start, end: new Date(start.getTime() + 3_600_000), allDay: false }
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
  select: (info) => {
    form.value = { kind: 'event', title: '' }
    sel.value = info
  },
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

    <div v-if="sel" class="overlay open" @mousedown.self="sel = null" @keydown.esc="sel = null">
      <form class="dialog" @submit.prevent="save">
        <div class="dlg-head"><h3>Neuer Eintrag</h3></div>
        <div class="dlg-body">
          <div class="seg">
            <button v-for="o in kindOptions" :key="o.value" type="button" :aria-pressed="form.kind === o.value" @click="form.kind = o.value">{{ o.label }}</button>
          </div>
          <div class="field"><label>Titel</label><input v-model="form.title" class="input" placeholder="Titel" autofocus /></div>
        </div>
        <div class="dlg-foot">
          <button type="button" class="btn btn-ghost" @click="sel = null">Abbrechen</button>
          <button type="submit" class="btn btn-primary">Anlegen</button>
        </div>
      </form>
    </div>

    <div v-if="picked" class="overlay open" @mousedown.self="picked = null" @keydown.esc="picked = null">
      <div class="dialog">
        <div class="dlg-head"><h3>{{ picked.title }}</h3></div>
        <div class="dlg-body">
          <span v-if="picked.location" class="lbl">{{ picked.location }}</span>
          <span v-if="picked.recurring" class="lbl">Serie: Löschen entfernt alle Wiederholungen.</span>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" @click="picked = null">Schließen</button>
          <button class="btn btn-secondary" @click="remove">Löschen</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; display: flex; flex-direction: column; gap: 12px; }
.cal { flex: 1; min-height: 0; display: flex; flex-direction: column; padding: 0; }
.cal-fc { flex: 1; min-height: 0; }
</style>
