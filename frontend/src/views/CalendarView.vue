<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import '@/calendar.css'
import { useRoute, useRouter } from 'vue-router'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin, { type EventResizeDoneArg } from '@fullcalendar/interaction'
import deLocale from '@fullcalendar/core/locales/de'
import enGbLocale from '@fullcalendar/core/locales/en-gb'
import type {
  CalendarOptions,
  EventApi,
  EventClickArg,
  EventDropArg,
  EventInput,
} from '@fullcalendar/core'
import Message from 'primevue/message'
import {
  completeHabit,
  createEvent,
  createTask,
  deleteEvent,
  errorMessage,
  getOccurrences,
  habitsDue,
  importIcs,
  isOpen,
  listProjects,
  listTasks,
  listTimeEntries,
  restoreEvent,
  skipOccurrence,
  taskAction,
  uncompleteHabit,
  updateEvent,
  updateHabit,
  updateTask,
  type CalendarEvent,
  type Project,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useShortcuts } from '@/composables/useShortcuts'
import { useTaskSelection } from '@/composables/useTaskSelection'
import { useUndo } from '@/composables/useUndo'
import EventDialog from '@/components/EventDialog.vue'
import CalendarUnplannedAside, {
  type NextUpItem,
} from '@/components/CalendarUnplannedAside.vue'
import { addDays, hhmm, hm, ymd } from '@/lib/dates'
import { editForm, isAllDay, newForm, requestOf, type Form } from '@/lib/eventForm'
import { habitColor, projectColor } from '@/lib/projectColor'
import { store } from '@/lib/storage'
import { prefs } from '@/lib/prefs'
import { lang, locale, t } from '@/lib/i18n'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

function createDomElement(tag: string, className: string, textContent = ''): HTMLElement {
  const element = document.createElement(tag)
  element.className = className
  element.textContent = textContent
  return element
}

// Je Tag: [geplant, erfasst] in Minuten; der Tageskopf zeigt beides (Plan/Ist).
const stats = new Map<string, [number, number]>()
function bumpStats(day: string, index: 0 | 1, minutes: number) {
  const current = stats.get(day) ?? [0, 0]
  current[index] += Math.max(0, Math.round(minutes))
  stats.set(day, current)
}

// ISO-Kalenderwoche (Donnerstag der Woche entscheidet über das Jahr).
function isoWeek(date: Date): number {
  const target = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()))
  target.setUTCDate(target.getUTCDate() + 4 - (target.getUTCDay() || 7))
  const yearStart = Date.UTC(target.getUTCFullYear(), 0, 1)
  return Math.ceil(((target.getTime() - yearStart) / 86_400_000 + 1) / 7)
}

function paintDay(th: HTMLElement) {
  const planEl = th.querySelector<HTMLElement>('.kt-plan')
  if (!planEl) return // Monatskopf hat keine Statistik

  const [p, ist] = stats.get(th.dataset.date ?? '') ?? [0, 0]
  planEl.textContent = p ? `${t('Plan')} ${hm(p)}` : t('frei')

  const istEl = th.querySelector<HTMLElement>('.kt-ist')
  if (istEl) istEl.textContent = ist ? `${t('Ist')} ${hm(ist)}` : ''

  const pBar = th.querySelector<HTMLElement>('.kt-bar .p')
  if (pBar) pBar.style.width = `${Math.min(100, p / 4.8)}%` // 8 h = volle Breite

  const tBar = th.querySelector<HTMLElement>('.kt-bar .t')
  if (tBar) tBar.style.width = `${Math.min(100, ist / 4.8)}%`

  th.title = `${t('Plan')}: ${p ? hm(p) : t('frei')}${ist ? ` · ${t('Ist')}: ${hm(ist)}` : ''}`
}

const router = useRouter()
const route = useRoute()
const { offer, setDone } = useUndo()
const cal = ref<InstanceType<typeof FullCalendar>>()
const error = ref('')
const notice = ref<{ text: string; notes: string[] } | null>(null)
const projects = ref<Project[]>([])
const unplannedTasks = ref<Task[]>([])
const dayStat = ref<{ plan: number; ist: number; free: number } | null>(null)
const nextUp = ref<NextUpItem | null>(null)
let nextId = ''

// Termine und Tasks als Kalendereinträge. Zeiten rechnen in der Zeitzone des Browsers.
async function loadEntries(from: Date, to: Date): Promise<EventInput[]> {
  const span = to.getTime() - from.getTime()
  const withHabits =
    span <= 86_400_000 * 1.1 || (prefs.calHabits && span <= 8 * 86_400_000)

  const [occurrences, tasks, projectList, times, habits] = await Promise.all([
    getOccurrences(from, to),
    listTasks(),
    listProjects(),
    listTimeEntries(from, to),
    withHabits ? habitsDue(ymd(from), ymd(addDays(to, -1))) : [],
  ])

  projects.value = projectList
  const projectNames = new Map(projectList.map((p) => [p.id, p.name]))
  const getProjectName = (id: string | null) => projectNames.get(id ?? '') ?? ''

  stats.clear()
  const spans: [number, number][] = []
  const nowMs = Date.now()
  let nextItem: NextUpItem | null = null

  const considerNext = (candidate: NextUpItem) => {
    if (
      candidate.start.getTime() > nowMs &&
      (!nextItem || candidate.start < nextItem.start)
    ) {
      nextItem = candidate
    }
  }

  unplannedTasks.value = tasks.filter((t) => !t.planned_date && isOpen(t))

  const entries: EventInput[] = occurrences.map((e) => {
    const s = new Date(e.occurrence_start)
    const en = new Date(e.occurrence_end)
    const allDay = isAllDay(s, en)

    if (!allDay) {
      bumpStats(ymd(s), 0, (en.getTime() - s.getTime()) / 60_000)
      spans.push([s.getTime(), en.getTime()])
      considerNext({
        id: `e:${e.id}:${e.occurrence_start}`,
        title: e.title,
        start: s,
        end: en,
        proj: getProjectName(e.project_id),
        color: projectColor(e.project_id),
      })
    }

    return {
      id: `e:${e.id}:${e.occurrence_start}`,
      title: e.title,
      allDay,
      start: allDay ? ymd(s) : e.occurrence_start,
      end: allDay ? ymd(en) : e.occurrence_end,
      classNames: ['kt-event', ...(allDay ? ['kt-allday'] : [])],
      editable: !e.recurrence_rule,
      extendedProps: {
        kind: 'event',
        id: e.id,
        ev: e,
        proj: getProjectName(e.project_id),
        place: e.location,
        color: e.project_id ? projectColor(e.project_id) : undefined,
      },
    }
  })

  const fromDay = ymd(from)
  const toDay = ymd(to)
  const todayDay = ymd(new Date())

  for (const t of tasks) {
    if (
      !t.planned_date ||
      t.status === 'CANCELLED' ||
      t.planned_date < fromDay ||
      t.planned_date >= toDay
    ) {
      continue
    }

    const done = t.status === 'COMPLETED'
    const late = !done && t.planned_date < todayDay
    const start = t.planned_start_at

    if (start) {
      const s = new Date(start)
      const durationMs = (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000
      const en = new Date(Date.parse(start) + durationMs)
      bumpStats(ymd(s), 0, (en.getTime() - s.getTime()) / 60_000)
      spans.push([s.getTime(), en.getTime()])
      if (!done) {
        considerNext({
          id: `t:${t.id}`,
          title: t.title,
          start: s,
          end: en,
          proj: getProjectName(t.project_id),
          color: projectColor(t.project_id),
          task: t,
        })
      }
    }

    entries.push({
      id: `t:${t.id}`,
      title: t.title,
      start: start ?? t.planned_date,
      end: start
        ? new Date(
            Date.parse(start) + (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000
          )
        : undefined,
      allDay: !start,
      classNames: [
        'kt-task',
        ...(start ? [] : ['kt-allday']),
        ...(done ? ['kt-done'] : []),
        ...(late ? ['kt-over'] : []),
        ...(t.status === 'IN_PROGRESS' ? ['kt-running'] : []),
      ],
      editable: !done,
      extendedProps: {
        kind: 'task',
        id: t.id,
        est: t.estimated_minutes,
        proj: getProjectName(t.project_id),
        color: t.project_id ? projectColor(t.project_id) : undefined,
      },
    })
  }

  for (const h of habits) {
    const start = h.preferred_time
      ? new Date(`${h.date}T${h.preferred_time}:00`)
      : null
    entries.push({
      id: `h:${h.habit_id}:${h.date}`,
      title: h.name,
      start: start ?? h.date,
      end: start ? new Date(start.getTime() + h.duration_minutes * 60_000) : undefined,
      allDay: !start,
      classNames: [
        'kt-task',
        'kt-habit',
        ...(start ? [] : ['kt-allday']),
        ...(h.done ? ['kt-done'] : []),
      ],
      extendedProps: {
        kind: 'habit',
        id: h.habit_id,
        day: h.date,
        done: h.done,
        dur: h.duration_minutes,
        time: h.preferred_time,
        color: habitColor(h.habit_id),
      },
    })
  }

  for (const te of times) {
    const s = new Date(te.started_at)
    const en = te.ended_at ? new Date(te.ended_at) : new Date()
    if (te.ended_at && en.getTime() - s.getTime() < 60_000) continue
    bumpStats(ymd(s), 1, (en.getTime() - s.getTime()) / 60_000)
    const pid = te.project_id ?? tasks.find((t) => t.id === te.task_id)?.project_id
    const min = Math.round((en.getTime() - s.getTime()) / 60_000)
    const taskTitle = tasks.find((t) => t.id === te.task_id)?.title ?? t('Zeit')
    entries.push({
      id: `x:${te.id}`,
      start: s,
      end: en,
      display: 'background',
      classNames: ['kt-track'],
      extendedProps: {
        kind: 'track',
        title: taskTitle,
        label: `${hhmm(s)}–${hhmm(en)} · ${hm(min)}`,
        color: pid ? projectColor(pid) : undefined,
      },
    })
  }

  if (to.getTime() - from.getTime() <= 25 * 3_600_000) {
    // Tagesansicht
    const [plan, ist] = stats.get(ymd(from)) ?? [0, 0]
    const open = new Date(from).setHours(7)
    const close = new Date(from).setHours(22)
    const left = Math.max(open, Math.min(close, nowMs))
    const busy = spans.reduce(
      (a, [s, e]) => a + Math.max(0, Math.min(e, close) - Math.max(s, left)),
      0
    )
    dayStat.value = {
      plan,
      ist,
      free: Math.max(0, Math.round((close - left - busy) / 60_000)),
    }
    nextUp.value = ymd(from) === ymd(new Date()) ? nextItem : null
  } else {
    dayStat.value = nextUp.value = null
  }

  nextId = (nextUp.value as NextUpItem | null)?.id ?? ''
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
  const { kind, id, ev, est } = event.extendedProps as {
    kind: string
    id: string
    ev: CalendarEvent
    est: number
  }
  const start = event.start
  const old = info.oldEvent
  if (!start || !old.start) return info.revert()

  const actionLabel = t(
    resized ? '„{title}“ angepasst' : '„{title}“ verschoben',
    { title: event.title }
  )

  if (kind === 'habit') {
    if (ymd(start) !== ymd(old.start)) return info.revert()
    const p = event.extendedProps as { dur: number; time: string | null }
    const before = { preferred_time: p.time ?? '', duration_minutes: p.dur }
    void guarded(async () => {
      const body: { preferred_time: string; duration_minutes?: number } = {
        preferred_time: event.allDay ? '' : hhmm(start),
      }
      if (resized && event.end) {
        body.duration_minutes = Math.round(
          (event.end.getTime() - start.getTime()) / 60_000
        )
      }
      await updateHabit(id, body)
      offer(actionLabel, () => guarded(() => updateHabit(id, before)))
    }, info.revert)
  } else if (kind === 'task') {
    const before: Record<string, unknown> = {
      planned_date: ymd(old.start),
      planned_start_at: old.allDay ? '' : old.start.toISOString(),
    }
    if (resized) before.estimated_minutes = est
    void guarded(async () => {
      const body: Record<string, unknown> = {
        planned_date: ymd(start),
        planned_start_at: event.allDay ? '' : start.toISOString(),
      }
      if (resized && event.end) {
        body.estimated_minutes = Math.round(
          (event.end.getTime() - start.getTime()) / 60_000
        )
      }
      await updateTask(id, body)
      offer(actionLabel, () => guarded(() => updateTask(id, before)))
    }, info.revert)
  } else {
    const end = event.end ?? (event.allDay ? addDays(start, 1) : null)
    if (!end) return info.revert()
    const before = { start_at: ev.start_at, end_at: ev.end_at }
    void guarded(async () => {
      await updateEvent(id, {
        start_at: start.toISOString(),
        end_at: end.toISOString(),
      })
      offer(actionLabel, () => guarded(() => updateEvent(id, before)))
    }, info.revert)
  }
}

// Dialog
const form = ref<Form | null>(null)
const openForm = (start: Date, end: Date, allDay: boolean) =>
  (form.value = newForm(start, end, allDay))
const openEdit = (ev: CalendarEvent, day: string) => (form.value = editForm(ev, day))

function save() {
  const f = form.value
  const title = f?.title.trim()
  if (!f || !title) return
  form.value = null
  const r = requestOf(f, title)
  void guarded(() =>
    'task' in r
      ? createTask(r.task)
      : f.id
        ? updateEvent(f.id, r.event)
        : createEvent(r.event)
  )
}

function clicked(info: EventClickArg) {
  const p = info.event.extendedProps as {
    kind: string
    id: string
    ev: CalendarEvent
    day: string
    done: boolean
  }
  if (p.kind === 'habit') {
    void guarded(() =>
      p.done ? uncompleteHabit(p.id, p.day) : completeHabit(p.id, p.day)
    )
  } else if (p.kind === 'task') {
    void router.push({ path: '/tasks', query: { task: p.id } })
  } else {
    openEdit(p.ev, ymd(info.event.start!))
  }
}

function remove(onlyThis: boolean) {
  const f = form.value
  form.value = null
  if (!f?.id) return
  const { id, day, title } = f
  void guarded(async () => {
    if (onlyThis) {
      const before = await skipOccurrence(id, day)
      offer(t('„{title}“ gelöscht', { title }), () =>
        guarded(() => updateEvent(id, { recurrence_exdates: before }))
      )
    } else {
      await deleteEvent(id)
      offer(t('„{title}“ gelöscht', { title }), () =>
        guarded(() => restoreEvent(id))
      )
    }
  })
}

// Toolbar
const title = ref('')
const week = ref(0)
const SPAN = 'timeGridSpan'
const views = [
  ['dayGridMonth', t('Monat'), 'm'],
  [SPAN, t('7 Tage'), 'w'],
  ['timeGridDay', t('Tag'), 'd'],
] as const

const spanView = () => `timeGrid${prefs.calDays}`
const spanStart = () => addDays(new Date(), -Math.floor(prefs.calDays / 2))
const storedView = store.get('kairo-cal-view')
const narrow = matchMedia('(max-width:760px)').matches
const view = ref<string>(
  storedView === 'dayGridMonth'
    ? storedView
    : narrow || storedView === 'timeGridDay'
      ? 'timeGridDay'
      : spanView()
)
const isWeek = computed(() => view.value === 'timeGrid5' || view.value === 'timeGrid7')
const isDay = computed(() => view.value === 'timeGridDay')

function goView(v: string) {
  const api = cal.value?.getApi()
  if (v === SPAN) api?.changeView(spanView(), spanStart())
  else api?.changeView(v)
}

function goToday() {
  const api = cal.value?.getApi()
  if (isWeek.value) api?.gotoDate(spanStart())
  else api?.today()
}

function setDays(n: 5 | 7) {
  prefs.calDays = n
  goView(SPAN)
}

// Ungeplant-Spalte
const upOpen = ref(
  (store.get('kairo-cal-unplanned') ?? (innerWidth < 1440 ? '0' : '1')) !== '0'
)
function toggleUp() {
  upOpen.value = !upOpen.value
  store.set('kairo-cal-unplanned', upOpen.value ? '1' : '0')
  void nextTick(() => cal.value?.getApi().updateSize())
}

const run = (fn: () => Promise<unknown>) => guarded(fn)
const selection = useTaskSelection(
  () => unplannedTasks.value,
  run,
  () => ymd(addDays(new Date(), 1))
)
const { picked, onClick: pickRow, onPointerDown, onPointerUp } = selection

function postpone(task: Task) {
  const before = {
    planned_date: task.planned_date ?? '',
    planned_start_at: task.planned_start_at ?? '',
  }
  const shift = (d: Date) =>
    new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1, d.getHours(), d.getMinutes())
  void guarded(async () => {
    await updateTask(task.id, {
      planned_date: ymd(shift(new Date(`${task.planned_date}T00:00:00`))),
      planned_start_at: task.planned_start_at
        ? shift(new Date(task.planned_start_at)).toISOString()
        : '',
    })
    offer(t('„{title}“ auf morgen verschoben', { title: task.title }), () =>
      guarded(() => updateTask(task.id, before))
    )
  })
}

function openNew() {
  const start = new Date()
  start.setHours(start.getHours() + 1, 0, 0, 0)
  openForm(start, new Date(start.getTime() + 3_600_000), false)
}

const fileInput = ref<HTMLInputElement>()
async function importFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const r = await importIcs(await file.text())
    const parts = [
      t('{n} neu', { n: r.created }),
      t('{n} aktualisiert', { n: r.updated }),
    ]
    if (r.skipped) parts.push(t('{n} übersprungen', { n: r.skipped }))
    if (r.unsupported_rules) {
      parts.push(t('{n} Serien als Einzeltermin', { n: r.unsupported_rules }))
    }
    notice.value = { text: parts.join(', '), notes: r.notes }
    error.value = ''
    cal.value?.getApi().refetchEvents()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

const projectOptions = computed(() =>
  projects.value.filter(
    (p) => p.status !== 'ARCHIVED' || p.id === form.value?.projectId
  )
)
const loadProjects = () =>
  listProjects().then(
    (ps) => (projects.value = ps),
    () => {}
  )

const calMenu = ref(false)

function gotoFromRoute() {
  const d = route.query.date
  if (typeof d !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(d)) return
  const day = new Date(`${d}T12:00:00`)
  cal.value
    ?.getApi()
    .gotoDate(isWeek.value ? addDays(day, -Math.floor(prefs.calDays / 2)) : day)
}

watch(() => route.query.date, gotoFromRoute)
onMounted(() => {
  void loadProjects()
  void nextTick(gotoFromRoute)
})

useShortcuts((e) => {
  if (form.value) return
  const api = cal.value?.getApi()
  const v = views.find(([, , k]) => k === e.key)
  if (!api) return
  if (e.key === 't') goToday()
  else if (e.key === 'ArrowLeft') api.prev()
  else if (e.key === 'ArrowRight') api.next()
  else if (v) goView(v[0])
  else if (e.key === 'n') openNew()
  else return
  e.preventDefault()
})

const { dayStart: SLOT_MIN_HOUR, dayEnd: SLOT_MAX_HOUR } = prefs
function scrollStart(evs: EventApi[] = []): string {
  const h = new Date().getHours() - 1
  let m = h <= SLOT_MIN_HOUR ? SLOT_MIN_HOUR * 60 : (h - 1) * 60 + 30
  for (const e of evs) {
    if (e.allDay || !e.start || !e.end || e.extendedProps.kind === 'track') continue
    const s = e.start.getHours() * 60 + e.start.getMinutes()
    if (s < m && m < e.end.getHours() * 60 + e.end.getMinutes()) {
      m = Math.floor(s / 60) * 60 - 30
    }
  }
  m = Math.max(m, SLOT_MIN_HOUR * 60)
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}:00`
}

let scrollFitted = false

const options: CalendarOptions = {
  plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
  locales: [deLocale, enGbLocale],
  locale: lang.value === 'de' ? 'de' : 'en-gb',
  firstDay: prefs.weekStart,
  allDayText: t('Ganztag'),
  initialView: view.value,
  initialDate: isWeek.value ? spanStart() : undefined,
  views: {
    timeGrid5: { type: 'timeGrid', duration: { days: 5 } },
    timeGrid7: { type: 'timeGrid', duration: { days: 7 } },
    dayGridMonth: { eventDisplay: 'block', dayMaxEvents: 3 },
  },
  moreLinkContent: (a) => `+ ${a.num} weitere`,
  headerToolbar: false,
  height: '100%',
  nowIndicator: true,
  slotEventOverlap: false,
  slotMinTime: `${String(SLOT_MIN_HOUR).padStart(2, '0')}:00:00`,
  slotMaxTime: `${String(SLOT_MAX_HOUR).padStart(2, '0')}:00:00`,
  scrollTime: scrollStart(),
  eventClassNames: (a) => (a.event.id === nextId ? ['kt-next'] : []),
  datesSet: (a) => {
    const day = a.view.type === 'timeGridDay'
    title.value = day
      ? a.view.currentStart.toLocaleDateString(locale.value, {
          weekday: 'long',
          day: 'numeric',
          month: 'long',
        })
      : a.view.title
    week.value = day ? isoWeek(a.view.currentStart) : 0
    view.value = a.view.type
    store.set('kairo-cal-view', a.view.type)
    a.view.calendar.scrollToTime(scrollStart())
    scrollFitted = false
  },
  dayHeaderContent: (a) => {
    const month = a.view.type === 'dayGridMonth'
    const wd = a.date
      .toLocaleDateString(locale.value, { weekday: month ? 'short' : 'long' })
      .replace('.', '')
    if (month) return wd
    if (a.view.type === 'timeGridDay') {
      const head = createDomElement('div', 'kt-dayhead')
      head.append(
        createDomElement('span', '', t('Plan')),
        createDomElement('span', 'kt-trk-h', t('Erfasst'))
      )
      return { domNodes: [head] }
    }
    const row = createDomElement('div', 'kt-row')
    row.append(createDomElement('span', 'kt-plan'), createDomElement('span', 'kt-ist'))
    const bar = createDomElement('div', 'kt-bar')
    bar.append(createDomElement('i', 'p'), createDomElement('i', 't'))
    const root = createDomElement('div', `kt-dh${a.isToday ? ' today' : ''}`)
    root.append(
      createDomElement('small', 'kt-wd-l', wd),
      createDomElement(
        'small',
        'kt-wd-s',
        a.date.toLocaleDateString(locale.value, { weekday: 'short' }).replace('.', '')
      ),
      createDomElement('b', '', String(a.date.getDate())),
      row,
      bar
    )
    return { domNodes: [root] }
  },
  dayHeaderDidMount: (a) => paintDay(a.el),
  eventsSet: (evs) => {
    const root = cal.value?.$el as HTMLElement | undefined
    root?.querySelectorAll<HTMLElement>('.fc-col-header-cell').forEach(paintDay)
    if (scrollFitted || !evs.length) return
    scrollFitted = true
    const s = scrollStart(evs)
    if (s !== scrollStart()) cal.value?.getApi().scrollToTime(s)
  },
  slotLabelContent: (a) => String(a.date.getHours()).padStart(2, '0'),
  nowIndicatorContent: (a) => (a.isAxis ? hhmm(a.date) : ''),
  eventContent: (a) => {
    const { event } = a
    const p = event.extendedProps as { kind: string; proj?: string; place?: string }
    if (p.kind === 'track') {
      if (a.view.type !== 'timeGridDay') return { domNodes: [] }
      const x = event.extendedProps as { title: string; label: string }
      const card = createDomElement('div', 'kt-trk')
      card.append(createDomElement('b', '', x.title), createDomElement('span', '', x.label))
      return { domNodes: [card] }
    }
    const month = a.view.type === 'dayGridMonth'
    const { start, end } = event
    const timed = !event.allDay && start
    const time = createDomElement('span', 'kt-time', timed ? hhmm(start) : '')
    if (timed && end && !month) {
      time.append(createDomElement('span', 'kt-end', `–${hhmm(end)}`))
    }
    const titleText = createDomElement('span', 'kt-title', event.title)
    const mark = createDomElement('i', p.kind === 'event' ? 'kt-dot' : 'kt-ring')
    const meta = [p.proj, p.place].filter(Boolean).join(' · ')
    const root = createDomElement('div', 'kt-in')
    if (month) {
      root.append(
        ...(p.kind === 'event' ? [] : [mark]),
        ...(timed ? [time] : []),
        titleText
      )
    } else {
      const head = createDomElement('div', 'kt-head')
      head.append(mark, titleText)
      root.append(
        head,
        ...(timed ? [time] : []),
        ...(meta ? [createDomElement('span', 'kt-meta', meta)] : [])
      )
    }
    return { domNodes: [root] }
  },
  expandRows: true,
  snapDuration: '00:15:00',
  displayEventEnd: false,
  eventMinHeight: 26,
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
    if (color) a.el.style.setProperty('--acc', color)
    const x = a.event.extendedProps as { kind: string; title?: string; label?: string }
    a.el.title =
      x.kind === 'track'
        ? `${t('Erfasst')}: ${x.title} · ${x.label}`
        : `${a.timeText ? `${a.timeText} ` : ''}${a.event.title}`
    if (x.kind === 'habit') {
      a.el.title += ` · ${t('Klick hakt ab, Verschieben ändert die Uhrzeit an allen Tagen')}`
    }
  },
  droppable: true,
  drop: (a) => {
    const id = a.draggedEl.dataset.id
    if (!id) return
    const body: Record<string, unknown> = { planned_date: ymd(a.date) }
    if (!a.allDay) body.planned_start_at = a.date.toISOString()
    const taskTitle = unplannedTasks.value.find((t) => t.id === id)?.title
    void guarded(async () => {
      await updateTask(id, body)
      offer(t('„{title}“ geplant', { title: taskTitle ?? '' }), () =>
        guarded(() => updateTask(id, { planned_date: '', planned_start_at: '' }))
      )
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
      <ul v-if="notice.notes.length" class="notes">
        <li v-for="n in notice.notes" :key="n">{{ n }}</li>
      </ul>
    </Message>

    <div class="cal-wrap">
      <div class="cal">
        <div class="cal-toolbar">
          <div class="cal-nav">
            <button
              class="icon-btn"
              :aria-label="$t('Zurück')"
              :title="$t('Zurück (←)')"
              @click="cal?.getApi().prev()"
            >
              <svg class="ic"><use href="#i-left" /></svg>
            </button>
            <button
              class="icon-btn"
              :aria-label="$t('Weiter')"
              :title="$t('Weiter (→)')"
              @click="cal?.getApi().next()"
            >
              <svg class="ic"><use href="#i-right" /></svg>
            </button>
          </div>

          <button
            class="btn btn-ghost"
            :title="$t('Heute (t)')"
            aria-keyshortcuts="t"
            @click="goToday"
          >
            {{ $t('Heute') }}
          </button>

          <h1 class="v-title cal-title">{{ title }}</h1>
          <span v-if="week" class="cal-kw">{{ $t('KW') }} {{ week }}</span>
          <span class="spacer" />

          <div v-if="isWeek" class="cal-legend" aria-hidden="true">
            <span><i class="lg-termin" />{{ $t('Termin') }}</span>
            <span><i class="lg-task" />{{ $t('Aufgabe') }}</span>
            <span><i class="lg-track" />{{ $t('Erfasst') }}</span>
          </div>

          <div class="seg">
            <button
              v-for="[v, l, k] in views"
              :key="v"
              :aria-pressed="view === v || (v === SPAN && isWeek)"
              :title="`${l} (${k})`"
              :aria-keyshortcuts="k"
              @click="goView(v)"
            >
              {{ l }}
            </button>
          </div>

          <input
            ref="fileInput"
            type="file"
            accept=".ics,text/calendar"
            hidden
            @change="importFile"
          />

          <div class="menu-wrap" @keydown.esc="calMenu = false">
            <button
              class="icon-btn"
              type="button"
              :aria-label="$t('Weitere Aktionen')"
              :aria-expanded="calMenu"
              @click="calMenu = !calMenu"
            >
              <svg class="ic"><use href="#i-more" /></svg>
            </button>

            <template v-if="calMenu">
              <div class="menu-scrim" @click="calMenu = false"></div>
              <div class="menu">
                <div v-if="isWeek" class="menu-row">
                  {{ $t('Tage in der Wochenansicht') }}
                  <span class="seg" role="group" :aria-label="$t('Tage in der Wochenansicht')">
                    <button
                      v-for="n in ([5, 7] as const)"
                      :key="n"
                      type="button"
                      :aria-pressed="prefs.calDays === n"
                      @click="setDays(n)"
                    >
                      {{ n }}
                    </button>
                  </span>
                </div>
                <button
                  class="menu-i"
                  type="button"
                  @click="calMenu = false; fileInput?.click()"
                >
                  {{ $t('ICS importieren') }} …
                </button>
              </div>
            </template>
          </div>

          <button
            class="btn btn-primary"
            :title="$t('Neuer Eintrag (n)')"
            aria-keyshortcuts="n"
            @click="openNew"
          >
            <svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neuer Eintrag') }}
          </button>
        </div>

        <div v-if="isDay && dayStat" class="cal-stats">
          <span>{{ $t('geplant') }} <b>{{ hm(dayStat.plan) }}</b></span>
          <span>{{ $t('erfasst') }} <b>{{ hm(dayStat.ist) }}</b></span>
          <span>{{ $t('frei') }} <b>{{ hm(dayStat.free) }}</b></span>
        </div>

        <div class="cal-fc">
          <FullCalendar ref="cal" :options="options" />
        </div>
      </div>

      <!-- Ungeplant-Seitenleiste -->
      <CalendarUnplannedAside
        :tasks="unplannedTasks"
        :projects="projects"
        :is-day="isDay"
        :is-open="upOpen"
        :next-up="nextUp"
        :picked-ids="picked"
        :selection-mode="selection.mode.value"
        @toggle-open="toggleUp"
        @pick-row="pickRow"
        @pointer-down="onPointerDown"
        @pointer-up="onPointerUp"
        @start-next-up="(task) => run(() => taskAction(task.id, 'start'))"
        @pause-next-up="(task) => run(() => taskAction(task.id, 'pause'))"
        @complete-next-up="(task) => setDone(task, 'COMPLETED', run)"
        @postpone-next-up="postpone"
        @to-tomorrow="selection.toTomorrow"
        @to-project="selection.toProject"
        @trash-selection="selection.trash"
        @clear-selection="selection.clear"
      />
    </div>

    <EventDialog
      v-if="form"
      :form="form"
      :projects="projectOptions"
      @save="save"
      @remove="remove"
      @close="form = null"
    />
  </div>
</template>

<style scoped>
.page {
  height: 100%;
  max-width: none;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.notes {
  margin: 6px 0 0;
  padding-left: 18px;
  max-height: 120px;
  overflow: auto;
  font-size: 12px;
}
.cal-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 12px;
}
.cal {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0;
  container-type: inline-size;
}
.cal-fc {
  flex: 1;
  min-height: 0;
}

@media (max-width: 900px) {
  .cal-wrap {
    flex-direction: column;
  }
}

@media (max-width: 760px) {
  .cal-toolbar .btn-secondary {
    display: none;
  }
  .cal-title {
    flex: 1 1 auto;
    font-size: 18px;
  }
  .cal-toolbar .spacer {
    display: none;
  }
  .page {
    height: auto;
    padding-bottom: 16px;
  }
  .cal-fc {
    flex: none;
    height: 70dvh;
  }
}

.cal-kw {
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
.cal-stats {
  display: flex;
  gap: 16px;
  padding: 0 10px 12px 56px;
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.cal-stats b {
  margin-left: 4px;
  font-weight: 600;
  color: var(--tx-primary);
}
.cal-legend {
  display: flex;
  align-items: center;
  gap: 14px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.cal-legend span {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cal-legend i {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  box-sizing: border-box;
  border: 1px solid var(--tx-muted);
}
.lg-termin {
  background: color-mix(in oklab, var(--tx-secondary) 30%, transparent);
}
.lg-task {
  border-style: dashed !important;
}
.lg-track {
  width: 3px !important;
  height: 12px !important;
  border: 0 !important;
  border-radius: 2px !important;
  background: var(--tx-secondary);
}

@container (max-width: 1180px) {
  .cal-legend {
    display: none;
  }
}
</style>
