<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
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
  createEvent, createTask, deleteEvent, errorMessage, getOccurrences, importIcs, isOpen, listProjects, listTasks, listTimeEntries, restoreEvent, skipOccurrence, taskAction, updateEvent, updateTask,
  type CalendarEvent, type EventBody, type Project, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { vDialog } from '@/lib/dialog'
import { addDays, hhmm, hm, ymd } from '@/lib/dates'
import { editForm, isAllDay, newForm, ruleOf, WEEKDAYS, type Form } from '@/lib/eventForm'
import { projectColor } from '@/lib/projectColor'
import { store } from '@/lib/storage'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

const el = (tag: string, cls: string, text = '') => Object.assign(document.createElement(tag), { className: cls, textContent: text })
// Je Tag: [geplant, erfasst] in Minuten; der Tageskopf zeigt beides (Plan/Ist).
const stats = new Map<string, [number, number]>()
const bump = (day: string, i: 0 | 1, min: number) => {
  const v = stats.get(day) ?? [0, 0]
  v[i] += Math.max(0, Math.round(min))
  stats.set(day, v)
}
// ISO-Kalenderwoche (Donnerstag der Woche entscheidet über das Jahr).
function isoWeek(d: Date) {
  const t = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()))
  t.setUTCDate(t.getUTCDate() + 4 - (t.getUTCDay() || 7))
  return Math.ceil(((t.getTime() - Date.UTC(t.getUTCFullYear(), 0, 1)) / 86_400_000 + 1) / 7)
}
const inMin = (d: Date) => {
  const m = Math.max(0, Math.round((d.getTime() - Date.now()) / 60_000))
  return m < 60 ? `${m} min` : `${hm(m)} h`
}
function paintDay(th: HTMLElement) {
  const q = (sel: string) => th.querySelector<HTMLElement>(sel)
  const plan = q('.kt-plan')
  if (!plan) return // Monatskopf hat keine Statistik
  const [p, t] = stats.get(th.dataset.date ?? '') ?? [0, 0]
  plan.textContent = p ? `Plan ${hm(p)}` : 'frei'
  q('.kt-ist')!.textContent = t ? `Ist ${hm(t)}` : ''
  q('.kt-bar .p')!.style.width = `${Math.min(100, p / 4.8)}%` // 8 h = volle Breite
  q('.kt-bar .t')!.style.width = `${Math.min(100, t / 4.8)}%`
}

const router = useRouter()
const { offer, setDone } = useUndo()
const cal = ref<InstanceType<typeof FullCalendar>>()
const error = ref('')
const notice = ref<{ text: string; notes: string[] } | null>(null)
const projects = ref<Project[]>([])
const unplanned = ref<Task[]>([]) // offene Tasks ohne planned_date: Quelle zum Ziehen
interface NextUp { id: string; title: string; start: Date; end: Date; proj: string; color: string; task?: Task }
const dayStat = ref<{ plan: number; ist: number; free: number } | null>(null)
const nextUp = ref<NextUp | null>(null)
let nextId = '' // Eintrag mit „Als Nächstes“-Ring im Raster

// Termine und Tasks als Kalendereinträge. Zeiten rechnen in der Zeitzone des Browsers.
async function loadEntries(from: Date, to: Date): Promise<EventInput[]> {
  // Projekte mitladen, damit die Projektfarben für die Einträge feststehen.
  const [occ, tasks, ps, times] = await Promise.all([getOccurrences(from, to), listTasks(), listProjects(), listTimeEntries(from, to)])
  projects.value = ps
  const names = new Map(ps.map((p) => [p.id, p.name]))
  const proj = (id: string | null) => names.get(id ?? '') ?? ''
  stats.clear()
  // Tagesansicht: Zeitspannen für „frei“ und der nächste Eintrag nach jetzt („Als Nächstes“).
  const spans: [number, number][] = []
  const nowMs = Date.now()
  let next: NextUp | null = null
  const consider = (n: NextUp) => { if (n.start.getTime() > nowMs && (!next || n.start < next.start)) next = n }
  unplanned.value = tasks.filter((t) => !t.planned_date && isOpen(t))
  const entries: EventInput[] = occ.map((e) => {
    const [s, en] = [new Date(e.occurrence_start), new Date(e.occurrence_end)]
    const allDay = isAllDay(s, en)
    if (!allDay) {
      bump(ymd(s), 0, (en.getTime() - s.getTime()) / 60_000)
      spans.push([s.getTime(), en.getTime()])
      consider({ id: `e:${e.id}:${e.occurrence_start}`, title: e.title, start: s, end: en, proj: proj(e.project_id), color: projectColor(e.project_id) })
    }
    return {
      id: `e:${e.id}:${e.occurrence_start}`,
      title: e.title,
      allDay,
      start: allDay ? ymd(s) : e.occurrence_start,
      end: allDay ? ymd(en) : e.occurrence_end,
      classNames: ['kt-event', ...(allDay ? ['kt-allday'] : [])],
      editable: !e.recurrence_rule, // bei Serien gehören start_at/end_at zur ersten Wiederholung
      extendedProps: { kind: 'event', id: e.id, ev: e, proj: proj(e.project_id), place: e.location, color: e.project_id ? projectColor(e.project_id) : undefined },
    }
  })
  const [fromDay, toDay, today] = [ymd(from), ymd(to), ymd(new Date())]
  for (const t of tasks) {
    if (!t.planned_date || t.status === 'CANCELLED' || t.planned_date < fromDay || t.planned_date >= toDay) continue
    const done = t.status === 'COMPLETED'
    const late = !done && t.planned_date < today // überfällig: Titel in Rot, Plan nicht erledigt
    const start = t.planned_start_at
    if (start) {
      const [s, en] = [new Date(start), new Date(Date.parse(start) + (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000)]
      bump(ymd(s), 0, (en.getTime() - s.getTime()) / 60_000)
      spans.push([s.getTime(), en.getTime()])
      if (!done) consider({ id: `t:${t.id}`, title: t.title, start: s, end: en, proj: proj(t.project_id), color: projectColor(t.project_id), task: t })
    }
    entries.push({
      id: `t:${t.id}`,
      title: t.title,
      start: start ?? t.planned_date,
      end: start ? new Date(Date.parse(start) + (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000) : undefined,
      allDay: !start,
      classNames: ['kt-task', ...(start ? [] : ['kt-allday']), ...(done ? ['kt-done'] : []), ...(late ? ['kt-over'] : []), ...(t.status === 'IN_PROGRESS' ? ['kt-running'] : [])],
      editable: !done,
      extendedProps: { kind: 'task', id: t.id, est: t.estimated_minutes, proj: proj(t.project_id), color: t.project_id ? projectColor(t.project_id) : undefined },
    })
  }
  weekendEntries.value = entries.filter((e) => [0, 6].includes(new Date(String(e.start).length === 10 ? `${e.start}T12:00:00` : String(e.start)).getDay())).length
  // Erfasste Zeit als schmale Spur links im Tag (Hintergrund-Eintrag); läuft sie noch, reicht sie bis jetzt.
  for (const te of times) {
    const [s, en] = [new Date(te.started_at), te.ended_at ? new Date(te.ended_at) : new Date()]
    bump(ymd(s), 1, (en.getTime() - s.getTime()) / 60_000)
    const pid = te.project_id ?? tasks.find((t) => t.id === te.task_id)?.project_id
    const min = Math.round((en.getTime() - s.getTime()) / 60_000)
    const title = tasks.find((t) => t.id === te.task_id)?.title ?? 'Zeit'
    entries.push({
      id: `x:${te.id}`, start: s, end: en, display: 'background', classNames: ['kt-track'],
      extendedProps: { kind: 'track', title, label: `${hhmm(s)}–${hhmm(en)} · ${hm(min)}`, color: pid ? projectColor(pid) : undefined },
    })
  }
  if (to.getTime() - from.getTime() <= 25 * 3_600_000) { // Tagesansicht
    const [plan, ist] = stats.get(ymd(from)) ?? [0, 0]
    const [open, close] = [new Date(from).setHours(7), new Date(from).setHours(22)]
    const left = Math.max(open, Math.min(close, nowMs)) // frei = ab jetzt (vor 7 Uhr ab 7 Uhr) bis 22 Uhr, ohne Geplantes
    const busy = spans.reduce((a, [s, e]) => a + Math.max(0, Math.min(e, close) - Math.max(s, left)), 0) // ponytail: überlappende Termine zählen doppelt
    dayStat.value = { plan, ist, free: Math.max(0, Math.round((close - left - busy) / 60_000)) }
    nextUp.value = ymd(from) === ymd(new Date()) ? next : null
  } else {
    dayStat.value = nextUp.value = null
  }
  nextId = (nextUp.value as NextUp | null)?.id ?? ''
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
    const end = event.end ?? (event.allDay ? addDays(start, 1) : null)
    if (!end) return info.revert()
    const before = { start_at: ev.start_at, end_at: ev.end_at }
    void guarded(async () => {
      await updateEvent(id, { start_at: start.toISOString(), end_at: end.toISOString() })
      offer(label, () => guarded(() => updateEvent(id, before)))
    }, info.revert)
  }
}

// Ein Dialog zum Anlegen (Termin oder Task) und Bearbeiten von Terminen.
const kindOptions = [{ label: 'Termin', value: 'event' }, { label: 'Task', value: 'task' }]
const form = ref<Form | null>(null)
const openForm = (start: Date, end: Date, allDay: boolean) => (form.value = newForm(start, end, allDay))
const openEdit = (ev: CalendarEvent, day: string) => (form.value = editForm(ev, day))

const toggleDay = (code: string) => {
  const f = form.value!
  f.days = f.days.includes(code) ? f.days.filter((d) => d !== code) : WEEKDAYS.map(([c]) => c).filter((c) => c === code || f.days.includes(c))
}

function save() {
  const f = form.value
  const title = f?.title.trim()
  if (!f || !title) return
  const start = new Date(`${f.date}T${f.allDay ? '00:00' : f.from}`)
  const end = f.allDay ? addDays(new Date(`${f.endDate || f.date}T00:00`), 1) : new Date(`${f.date}T${f.to}`)
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
const week = ref(0) // Kalenderwoche, nur in der Tagesansicht
// „Woche“ zeigt Mo–Fr (timeGridWorkWeek); der Schalter „Sa/So“ wechselt zur vollen Woche (timeGridWeek).
const WORK = 'timeGridWorkWeek'
const views = [['dayGridMonth', 'Monat', 'm'], [WORK, 'Woche', 'w'], ['timeGridDay', 'Tag', 'd']] as const
const fullWeek = ref(store.get('kairo-cal-weekend') === '1')
const weekView = () => (fullWeek.value ? 'timeGridWeek' : WORK)
const storedView = store.get('kairo-cal-view')
const view = ref<string>(storedView === 'dayGridMonth' || storedView === 'timeGridDay' ? storedView : weekView())
const isWeek = computed(() => view.value === WORK || view.value === 'timeGridWeek')
const isDay = computed(() => view.value === 'timeGridDay')
const goView = (v: string) => cal.value?.getApi().changeView(v === WORK ? weekView() : v)
function toggleWeekend() {
  fullWeek.value = !fullWeek.value
  store.set('kairo-cal-weekend', fullWeek.value ? '1' : '0')
  goView(WORK)
}
// Einträge auf Sa/So, die in der Woche Mo–Fr verborgen sind: der Schalter zeigt ihre Zahl.
const weekendEntries = ref(0)

// „Ungeplant“ lässt sich einklappen, damit die Woche die volle Breite bekommt.
const upOpen = ref(store.get('kairo-cal-unplanned') !== '0')
function toggleUp() {
  upOpen.value = !upOpen.value
  store.set('kairo-cal-unplanned', upOpen.value ? '1' : '0')
  void nextTick(() => cal.value?.getApi().updateSize())
}

const run = (fn: () => Promise<unknown>) => guarded(fn)
// „Verschieben“ setzt den Eintrag auf morgen, gleiche Uhrzeit.
function postpone(t: Task) {
  const before = { planned_date: t.planned_date ?? '', planned_start_at: t.planned_start_at ?? '' }
  const shift = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1, d.getHours(), d.getMinutes())
  void guarded(async () => {
    await updateTask(t.id, { planned_date: ymd(shift(new Date(`${t.planned_date}T00:00:00`))), planned_start_at: t.planned_start_at ? shift(new Date(t.planned_start_at)).toISOString() : '' })
    offer(`„${t.title}“ auf morgen verschoben`, () => guarded(() => updateTask(t.id, before)))
  })
}

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

// Erster Fokus der Rückfrage liegt auf der sicheren Wahl.
const safeBtn = ref<HTMLElement>()
const askDelete = () => {
  form.value!.ask = true
  void nextTick(() => safeBtn.value?.focus())
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

// Tastenkürzel: t heute · ←/→ zurück/weiter · m/w/d Ansicht · n neuer Eintrag. Nicht beim Tippen, nicht in Dialogen.
useShortcuts((e) => {
  if (form.value) return
  const api = cal.value?.getApi()
  const v = views.find(([, , k]) => k === e.key)
  if (!api) return
  if (e.key === 't') api.today()
  else if (e.key === 'ArrowLeft') api.prev()
  else if (e.key === 'ArrowRight') api.next()
  else if (v) goView(v[0])
  else if (e.key === 'n') openNew()
  else return
  e.preventDefault()
})

// Das Zeitraster beginnt ein bis anderthalb Stunden vor jetzt, nie vor dem ersten Slot.
// Der Start liegt auf :30, weil das Label einer vollen Stunde am oberen Rand sonst halb abgeschnitten wird.
const SLOT_MIN_HOUR = 7
const scrollStart = () => {
  const h = new Date().getHours() - 1
  return h <= SLOT_MIN_HOUR ? `${String(SLOT_MIN_HOUR).padStart(2, '0')}:00:00` : `${String(h - 1).padStart(2, '0')}:30:00`
}

const options: CalendarOptions = {
  plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
  locales: [deLocale],
  locale: 'de',
  firstDay: 1,
  allDayText: 'Ganztag',
  initialView: view.value,
  views: { [WORK]: { type: 'timeGrid', duration: { weeks: 1 }, hiddenDays: [0, 6] }, dayGridMonth: { eventDisplay: 'block', dayMaxEvents: 3 } },
  moreLinkContent: (a) => `+ ${a.num} weitere`,
  headerToolbar: false,
  height: '100%',
  nowIndicator: true,
  slotEventOverlap: false, // gleichzeitige Einträge nebeneinander statt gestapelt
  slotMinTime: `${String(SLOT_MIN_HOUR).padStart(2, '0')}:00:00`, // bis 22 Uhr, damit Abendtermine nicht verschwinden
  slotMaxTime: '22:00:00',
  scrollTime: scrollStart(),
  eventClassNames: (a) => (a.event.id === nextId ? ['kt-next'] : []),
  datesSet: (a) => {
    const day = a.view.type === 'timeGridDay'
    title.value = day ? a.view.currentStart.toLocaleDateString('de-DE', { weekday: 'long', day: 'numeric', month: 'long' }) : a.view.title
    week.value = day ? isoWeek(a.view.currentStart) : 0
    view.value = a.view.type
    store.set('kairo-cal-view', a.view.type)
    a.view.calendar.scrollToTime(scrollStart()) // scrollTime stammt vom Seitenaufruf, die Seite kann länger offen sein
  },
  dayHeaderContent: (a) => {
    const month = a.view.type === 'dayGridMonth'
    const wd = a.date.toLocaleDateString('de-DE', { weekday: month ? 'short' : 'long' }).replace('.', '')
    if (month) return wd
    if (a.view.type === 'timeGridDay') { // Spaltenköpfe Plan | Erfasst
      const head = el('div', 'kt-dayhead')
      head.append(el('span', '', 'Plan'), el('span', 'kt-trk-h', 'Erfasst'))
      return { domNodes: [head] }
    }
    const row = el('div', 'kt-row')
    row.append(el('span', 'kt-plan'), el('span', 'kt-ist'))
    const bar = el('div', 'kt-bar')
    bar.append(el('i', 'p'), el('i', 't'))
    const root = el('div', `kt-dh${a.isToday ? ' today' : ''}`)
    root.append(el('small', '', wd), el('b', '', String(a.date.getDate())), row, bar)
    return { domNodes: [root] }
  },
  dayHeaderDidMount: (a) => paintDay(a.el),
  eventsSet: () => (cal.value?.$el as HTMLElement | undefined)?.querySelectorAll<HTMLElement>('.fc-col-header-cell').forEach(paintDay),
  slotLabelContent: (a) => String(a.date.getHours()).padStart(2, '0'),
  nowIndicatorContent: (a) => (a.isAxis ? hhmm(a.date) : ''),
  // Termin = Punkt, Aufgabe = Ring (gefüllt, wenn erledigt); in der Woche Titel, Zeit, Projekt · Ort, im Monat Zeit und Titel in einer Zeile.
  eventContent: (a) => {
    const { event } = a
    const p = event.extendedProps as { kind: string; proj?: string; place?: string }
    if (p.kind === 'track') { // Woche: nur die Spur; Tag: Karte mit Aufgabe, Zeitraum und Dauer
      if (a.view.type !== 'timeGridDay') return { domNodes: [] }
      const x = event.extendedProps as { title: string; label: string }
      const card = el('div', 'kt-trk')
      card.append(el('b', '', x.title), el('span', '', x.label))
      return { domNodes: [card] }
    }
    const month = a.view.type === 'dayGridMonth'
    const { start, end } = event
    const timed = !event.allDay && start
    const time = el('span', 'kt-time', timed ? hhmm(start) : '')
    if (timed && end && !month) time.append(el('span', 'kt-end', `–${hhmm(end)}`))
    const title = el('span', 'kt-title', event.title)
    const mark = el('i', p.kind === 'task' ? 'kt-ring' : 'kt-dot')
    const meta = [p.proj, p.place].filter(Boolean).join(' · ')
    const root = el('div', 'kt-in')
    if (month) root.append(...(p.kind === 'task' ? [mark] : []), ...(timed ? [time] : []), title)
    else {
      const head = el('div', 'kt-head')
      head.append(mark, title)
      root.append(head, ...(timed ? [time] : []), ...(meta ? [el('span', 'kt-meta', meta)] : []))
    }
    return { domNodes: [root] }
  },
  expandRows: true,
  snapDuration: '00:15:00',
  displayEventEnd: false,
  eventMinHeight: 26, // kurze Einträge (30 min) laufen einzeilig: Titel und Zeit nebeneinander
  eventShortHeight: 36,
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
    a.el.title = `${a.timeText ? `${a.timeText} ` : ''}${a.event.title}` // voller Titel bei Hover
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
          <button class="icon-btn" aria-label="Zurück" title="Zurück (←)" @click="cal?.getApi().prev()"><svg class="ic"><use href="#i-left" /></svg></button>
          <button class="icon-btn" aria-label="Weiter" title="Weiter (→)" @click="cal?.getApi().next()"><svg class="ic"><use href="#i-right" /></svg></button>
        </div>
        <button class="btn btn-ghost" title="Heute (t)" aria-keyshortcuts="t" @click="cal?.getApi().today()">Heute</button>
        <h2 class="cal-title">{{ title }}</h2>
        <span v-if="week" class="cal-kw">KW {{ week }}</span>
        <span class="spacer" />
        <div v-if="isWeek" class="cal-legend" aria-hidden="true">
          <span><i class="lg-termin" />Termin</span><span><i class="lg-task" />Aufgabe</span><span><i class="lg-track" />Erfasst</span>
        </div>
        <div class="seg">
          <button v-for="[v, l, k] in views" :key="v" :aria-pressed="view === v || (v === WORK && view === 'timeGridWeek')" :title="`${l} (${k})`" :aria-keyshortcuts="k" @click="goView(v)">{{ l }}</button>
        </div>
        <button v-if="isWeek" class="btn btn-ghost" :aria-pressed="fullWeek" title="Samstag und Sonntag anzeigen" @click="toggleWeekend">Sa/So<span v-if="!fullWeek && weekendEntries" class="up-count"> {{ weekendEntries }}</span></button>
        <input ref="fileInput" type="file" accept=".ics,text/calendar" hidden @change="importFile" />
        <button class="btn btn-secondary" @click="fileInput?.click()">ICS importieren</button>
        <button class="btn btn-primary" title="Neuer Eintrag (n)" aria-keyshortcuts="n" @click="openNew"><svg class="ic"><use href="#i-plus" /></svg>Neuer Eintrag</button>
      </div>
      <div v-if="isDay && dayStat" class="cal-stats">
        <span>geplant <b>{{ hm(dayStat.plan) }}</b></span><span>erfasst <b>{{ hm(dayStat.ist) }}</b></span><span>frei <b>{{ hm(dayStat.free) }}</b></span>
      </div>
      <div class="cal-fc"><FullCalendar ref="cal" :options="options" /></div>
    </div>
    <aside class="unplanned" :class="{ shut: !upOpen, wide: isDay }" aria-labelledby="up-title">
      <section v-if="isDay && nextUp && upOpen" class="next" aria-label="Als Nächstes">
        <div class="next-head"><span>Als Nächstes · in {{ inMin(nextUp.start) }}</span><span>{{ nextUp.task ? 'Aufgabe' : 'Termin' }}</span></div>
        <h3>{{ nextUp.title }}</h3>
        <div class="next-meta">
          <span><i class="next-dot" :style="{ background: nextUp.color }" />{{ nextUp.proj || 'Ohne Projekt' }}</span>
          <span>{{ hhmm(nextUp.start) }}–{{ hhmm(nextUp.end) }}</span><span>{{ hm(Math.round((nextUp.end.getTime() - nextUp.start.getTime()) / 60_000)) }}</span>
        </div>
        <div v-if="nextUp.task" class="row nowrap">
          <button v-if="nextUp.task.status === 'IN_PROGRESS'" type="button" class="btn btn-primary" @click="run(() => taskAction(nextUp!.task!.id, 'pause'))">Pause</button>
          <button v-else type="button" class="btn btn-primary" @click="run(() => taskAction(nextUp!.task!.id, 'start'))">Start</button>
          <button type="button" class="btn btn-secondary" @click="setDone(nextUp.task, 'COMPLETED', run)">Fertig</button>
          <button type="button" class="btn btn-ghost" @click="postpone(nextUp.task)">Verschieben</button>
        </div>
      </section>
      <div class="up-head">
        <h3 id="up-title"><span class="up-lbl">Ungeplant </span><span class="up-count">{{ unplanned.length }}</span></h3>
        <span v-if="isDay && upOpen" class="up-hint">in den Plan ziehen</span>
        <button type="button" class="icon-btn" :aria-expanded="upOpen" :aria-label="upOpen ? 'Ungeplant einklappen' : 'Ungeplant ausklappen'" @click="toggleUp"><svg class="ic"><use :href="upOpen ? '#i-right' : '#i-left'" /></svg></button>
      </div>
      <div v-show="upOpen" ref="unplannedEl" class="up-list">
        <div v-for="t in unplanned" :key="t.id" class="up-item" :data-id="t.id" :data-min="t.estimated_minutes || ''" :style="{ '--acc': projectColor(t.project_id) }">
          <span class="up-dot" />
          <span class="up-name">{{ t.title }}</span>
          <small v-if="t.estimated_minutes" class="up-min">{{ hm(t.estimated_minutes) }}</small>
        </div>
      </div>
    </aside>
    </div>

    <div v-if="form" v-dialog="() => (form = null)" class="overlay open" @mousedown.self="form = null">
      <form class="dialog" aria-labelledby="cal-dlg-title" @submit.prevent="save">
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
          <button ref="safeBtn" type="button" class="btn btn-ghost" @click="form.ask = false">Abbrechen</button>
          <button v-if="form.day" type="button" class="btn btn-secondary" @click="remove(true)">Nur dieser Termin</button>
          <button type="button" class="btn btn-primary" @click="remove(false)">{{ form.day ? 'Ganze Serie löschen' : 'Termin löschen' }}</button>
        </div>
        <div v-else class="dlg-foot">
          <button v-if="form.id" type="button" class="btn btn-secondary" @click="askDelete">Löschen</button>
          <span class="spacer" />
          <button type="button" class="btn btn-ghost" @click="form = null">Abbrechen</button>
          <button type="submit" class="btn btn-primary">{{ form.id ? 'Speichern' : 'Anlegen' }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; max-width: none; display: flex; flex-direction: column; gap: 12px; }
.notes { margin: 6px 0 0; padding-left: 18px; max-height: 120px; overflow: auto; font-size: 12px; }
.cal-wrap { flex: 1; min-height: 0; display: flex; gap: 12px; }
.cal { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; padding: 0; container-type: inline-size; }
.unplanned {
  flex: none; width: 240px; display: flex; flex-direction: column; min-height: 0; padding: 12px;
  background: var(--bg-1); border: 1px solid var(--br-default); border-radius: var(--r-m);
}
.up-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.unplanned.wide:not(.shut) { width: 300px; }
.unplanned.shut { width: 44px; padding: 12px 6px; align-items: center; }
.unplanned.shut .up-head { flex-direction: column; }
.unplanned.shut .up-lbl { display: none; }
.unplanned h3 { margin: 0; font: 600 14px/1.3 var(--font-ui); color: var(--tx-primary); }
.up-count { font: 400 12px/1 var(--font-mono); color: var(--tx-muted); }
.up-list { margin-top: 10px; }
.up-list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 6px; }
.up-hint { flex: 1; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); text-align: right; }
.up-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px; cursor: grab;
  border: 1px dashed var(--br-strong); border-radius: 8px;
  font: 400 13px/1.3 var(--font-ui); color: var(--tx-primary);
}
.up-item:hover { background: var(--bg-hover); }
.up-dot { flex: none; width: 11px; height: 11px; box-sizing: border-box; border: 1.5px solid var(--acc); border-radius: 50%; }
.up-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.up-min { flex: none; font: 400 12px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-secondary); }
.next { display: flex; flex-direction: column; gap: 12px; padding: 16px; margin-bottom: 16px; background: var(--bg-0); border: 1px solid var(--br-subtle); border-radius: 12px; }
.next-head { display: flex; justify-content: space-between; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.next h3 { margin: 0; font: 600 17px/1.3 var(--font-ui); color: var(--tx-primary); text-wrap: pretty; }
.next-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; margin-top: -6px; font: 400 13px/1.3 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-secondary); }
.next-meta span { display: flex; align-items: center; gap: 6px; }
.next-dot { width: 7px; height: 7px; border-radius: 50%; }
@media (max-width: 900px) {
  .cal-wrap { flex-direction: column; }
  .unplanned { width: auto; max-height: 180px; order: 2; }
}
.cal-fc { flex: 1; min-height: 0; }
.cal-kw { font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.cal-stats { display: flex; gap: 16px; padding: 0 10px 12px 56px; font: 400 13px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.cal-stats b { margin-left: 4px; font-weight: 600; color: var(--tx-primary); }
.cal-legend { display: flex; align-items: center; gap: 14px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.cal-legend span { display: flex; align-items: center; gap: 6px; }
.cal-legend i { display: block; width: 10px; height: 10px; border-radius: 3px; box-sizing: border-box; border: 1px solid var(--tx-muted); }
.lg-termin { background: color-mix(in oklab, var(--tx-secondary) 30%, transparent); }
.lg-task { border-style: dashed !important; }
.lg-track { width: 3px !important; height: 12px !important; border: 0 !important; border-radius: 2px !important; background: var(--tx-secondary); }
@container (max-width: 1180px) { .cal-legend { display: none; } }
.dlg-row { display: flex; gap: 12px; }
.dlg-foot { flex-wrap: wrap; }
.dlg-foot .spacer { flex: 1; }
.dlg-foot .q { flex: 1 0 100%; font-size: 13px; color: var(--tx-secondary); }
.dlg-row .field { flex: 1; }
</style>
