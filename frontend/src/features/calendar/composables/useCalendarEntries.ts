import { ref } from 'vue'
import type { EventInput } from '@fullcalendar/core'
import {
  getOccurrences,
  habitsDue,
  isOpen,
  listProjects,
  listTasks,
  listTimeEntries,
  type Project,
  type Task,
} from '@/api/client'
import type { NextUpItem } from '../components/CalendarUnplannedAside.vue'
import { addDays, hhmm, hm, ymd } from '@/lib/dates'
import { isAllDay } from '@/lib/eventForm'
import { habitColor, projectColor } from '@/lib/projectColor'
import { prefs } from '@/lib/prefs'
import { t } from '@/lib/i18n'

// Tasks ohne Dauer erscheinen mit dieser Länge im Raster.
const DEFAULT_TASK_MINUTES = 30

export function useCalendarEntries() {
  // Je Tag: [geplant, erfasst] in Minuten; der Tageskopf zeigt beides (Plan/Ist).
  const stats = new Map<string, [number, number]>()
  const projects = ref<Project[]>([])
  const unplannedTasks = ref<Task[]>([])
  const dayStat = ref<{ plan: number; ist: number; free: number } | null>(null)
  const nextUp = ref<NextUpItem | null>(null)
  const nextId = ref('')

  function bumpStats(day: string, index: 0 | 1, minutes: number) {
    const current = stats.get(day) ?? [0, 0]
    current[index] += Math.max(0, Math.round(minutes))
    stats.set(day, current)
  }

  async function loadEntries(from: Date, to: Date): Promise<EventInput[]> {
    const span = to.getTime() - from.getTime()
    const withHabits = span <= 86_400_000 * 1.1 || (prefs.calHabits && span <= 8 * 86_400_000)

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
      if (candidate.start.getTime() > nowMs && (!nextItem || candidate.start < nextItem.start)) {
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
          ? new Date(Date.parse(start) + (t.estimated_minutes || DEFAULT_TASK_MINUTES) * 60_000)
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
      const start = h.preferred_time ? new Date(`${h.date}T${h.preferred_time}:00`) : null
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
        0,
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

    nextId.value = (nextUp.value as NextUpItem | null)?.id ?? ''
    return entries
  }

  async function loadProjects() {
    try {
      projects.value = await listProjects()
    } catch {
      // Ignore
    }
  }

  return {
    stats,
    projects,
    unplannedTasks,
    dayStat,
    nextUp,
    nextId,
    loadEntries,
    loadProjects,
    bumpStats,
  }
}
