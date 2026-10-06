<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin, { type EventResizeDoneArg } from '@fullcalendar/interaction'
import deLocale from '@fullcalendar/core/locales/de'
import type { CalendarOptions, DateSelectArg, EventClickArg, EventDropArg, EventInput } from '@fullcalendar/core'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import SelectButton from 'primevue/selectbutton'
import {
  createEvent, createTask, deleteEvent, errorMessage, getOccurrences, listTasks, updateEvent, updateTask,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { ymd } from '@/lib/dates'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

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
      classNames: ['kt-task', ...(done ? ['kt-done'] : [])],
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
const sel = ref<DateSelectArg | null>(null)
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

const options: CalendarOptions = {
  plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
  locales: [deLocale],
  locale: 'de',
  firstDay: 1,
  allDayText: 'Ganztag',
  initialView: 'timeGridWeek',
  headerToolbar: { left: 'prev,next today', center: 'title', right: 'dayGridMonth,timeGridWeek,timeGridDay' },
  height: '100%',
  nowIndicator: true,
  scrollTime: '08:00:00',
  snapDuration: '00:15:00',
  slotLabelFormat: { hour: '2-digit', minute: '2-digit', hour12: false },
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
  <div class="wide stack page">
    <h1 class="title">Kalender</h1>
    <Message v-if="error" severity="error">{{ error }}</Message>
    <div class="cal"><FullCalendar ref="cal" :options="options" /></div>

    <Dialog :visible="!!sel" modal header="Neuer Eintrag" :style="{ width: '24rem' }" @update:visible="sel = null">
      <form class="stack" @submit.prevent="save">
        <SelectButton v-model="form.kind" :options="kindOptions" option-label="label" option-value="value" :allow-empty="false" />
        <InputText v-model="form.title" placeholder="Titel" autofocus />
        <div class="row"><Button type="submit" label="Anlegen" /><Button type="button" label="Abbrechen" severity="secondary" text @click="sel = null" /></div>
      </form>
    </Dialog>

    <Dialog :visible="!!picked" modal :header="picked?.title" :style="{ width: '24rem' }" @update:visible="picked = null">
      <div class="stack">
        <span v-if="picked?.location" class="muted">{{ picked.location }}</span>
        <span v-if="picked?.recurring" class="muted">Serie: Löschen entfernt alle Wiederholungen.</span>
        <div class="row"><Button label="Löschen" severity="danger" @click="remove" /><Button label="Schließen" severity="secondary" text @click="picked = null" /></div>
      </div>
    </Dialog>
  </div>
</template>

<style scoped>
.page { height: 100%; }
.cal { flex: 1; min-height: 0; }
</style>
