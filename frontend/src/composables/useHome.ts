import {
  computed,
  inject,
  onMounted,
  onUnmounted,
  provide,
  ref,
  type InjectionKey,
} from 'vue'
import {
  getToday,
  isOpen,
  listProjects,
  listTasks,
  listTimeEntries,
  updateTask,
  type Task,
  type TimeEntry,
  type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useUndo } from '@/composables/useUndo'
import { blockEndMinutes, doneSeconds, gaps, minutesOfDay } from '@/lib/dayPlan'
import { locale, t } from '@/lib/i18n'
import { prefs } from '@/lib/prefs'
import { projectColor } from '@/lib/projectColor'

const MILLISECONDS_PER_DAY = 24 * 60 * 60 * 1000

// Gemeinsame Daten der Startseite:
// TodayView lädt sie einmal, die Widgets holen sie sich über useHome().
// Widgets mit eigenen Daten (Ziel, Lernkarten, Journal, Woche, Notiz, GitHub) laden diese selbst.

export interface Block {
  key: string
  title: string
  start: number
  end: number
  kind: 'event' | 'task'
  done: boolean
  running?: boolean
  color: string
  sub: string
  task?: Task
}

function createHome() {
  const { offer, setDone } = useUndo()
  const today = ref<Today | null>(null)
  const entries = ref<TimeEntry[]>([])
  const tasks = ref<Task[]>([])
  const projectNames = ref(new Map<string, string>())
  const now = ref(Date.now())
  let intervalTimer: ReturnType<typeof setInterval> | undefined

  const { error, load, run } = useLoader(async () => {
    const todayData = await getToday()
    const startDate = new Date(`${todayData.date}T00:00:00`)
    const endDate = new Date(startDate.getTime() + MILLISECONDS_PER_DAY)

    const [loadedEntries, loadedTasks, loadedProjects] = await Promise.all([
      listTimeEntries(startDate, endDate),
      listTasks(),
      listProjects(),
    ])

    today.value = todayData
    entries.value = loadedEntries.reverse()
    tasks.value = loadedTasks
    projectNames.value = new Map(loadedProjects.map((proj) => [proj.id, proj.name]))
  })

  useLiveEvents(load)

  onMounted(() => {
    void load()
    intervalTimer = setInterval(() => {
      now.value = Date.now()
    }, 1000)
  })

  onUnmounted(() => {
    if (intervalTimer) {
      clearInterval(intervalTimer)
    }
  })

  const tasksById = computed(() => {
    return new Map(tasks.value.map((task) => [task.id, task]))
  })

  // Das Backend meldet ohne konfigurierte Zeitzone "Local"; das ist kein gültiger IANA-Name.
  const timezone = computed(() => {
    return today.value?.timezone === 'Local' ? undefined : today.value?.timezone
  })

  function formatTime(isoString: string): string {
    return new Intl.DateTimeFormat(locale.value, {
      hour: '2-digit',
      minute: '2-digit',
      timeZone: timezone.value,
    }).format(new Date(isoString))
  }

  function getMinutesOfDay(timestampMs: number): number {
    return minutesOfDay(timestampMs, timezone.value)
  }

  const nowMinutes = computed(() => getMinutesOfDay(now.value))

  const runningEntry = computed(() => today.value?.running_time_entry ?? null)
  const runningTaskId = computed(() => runningEntry.value?.task_id ?? null)

  // Zuletzt pausierte Task von heute: bleibt im Jetzt-Widget, damit man sie fortsetzen kann.
  const pausedTask = computed<Task | undefined>(() => {
    if (runningEntry.value) {
      return undefined
    }

    const reversedEntries = [...entries.value].reverse()
    const lastPausedEntry = reversedEntries.find((entry) => {
      if (!entry.task_id) return false
      const associatedTask = tasksById.value.get(entry.task_id)
      return associatedTask?.status === 'PAUSED'
    })

    if (!lastPausedEntry || !lastPausedEntry.task_id) {
      return undefined
    }

    return tasksById.value.get(lastPausedEntry.task_id)
  })

  // Heute schon erfasste Zeit der Task plus laufender Eintrag, damit der Timer nach Pause weiterzählt.
  const runningSeconds = computed(() => {
    const active = runningEntry.value
    if (active) {
      const recorded = active.task_id ? doneSeconds(entries.value, active.task_id) : 0
      const currentRun = Math.max(0, (now.value - Date.parse(active.started_at)) / 1000)
      return recorded + currentRun
    }

    if (pausedTask.value) {
      return doneSeconds(entries.value, pausedTask.value.id)
    }

    return 0
  })

  // Termine und Tasks mit Uhrzeit als Blöcke; Tasks ohne Dauer zählen 30 Minuten.
  const allBlocks = computed<Block[]>(() => {
    const todayData = today.value
    if (!todayData) return []

    const blockList: Block[] = []

    // 1. Kalender-Events
    for (const event of todayData.events) {
      const startMin = getMinutesOfDay(Date.parse(event.occurrence_start))
      const rawEndMin = getMinutesOfDay(Date.parse(event.occurrence_end))
      const endMin = blockEndMinutes(startMin, rawEndMin)

      blockList.push({
        key: `e${event.id}${event.occurrence_start}`,
        title: event.title,
        kind: 'event',
        done: false,
        color: 'var(--a-event)',
        sub: event.location,
        start: startMin,
        end: endMin,
      })
    }

    // 2. Geplante Aufgaben mit Startzeit
    for (const task of todayData.tasks) {
      if (!task.planned_start_at) continue

      const startMin = getMinutesOfDay(Date.parse(task.planned_start_at))
      const duration = task.estimated_minutes || 30
      const isRunning = task.id === runningTaskId.value
      const isDone = task.status === 'COMPLETED'

      blockList.push({
        key: `t${task.id}`,
        title: task.title,
        kind: 'task',
        done: isDone,
        running: isRunning,
        sub: '',
        task,
        color: projectColor(task.project_id),
        start: startMin,
        end: startMin + duration,
      })
    }

    return blockList.sort((a, b) => a.start - b.start)
  })

  // Ganztägige Termine stehen im Kopf, nicht im Raster.
  function isAllDayBlock(b: Block): boolean {
    return b.kind === 'event' && b.start === 0 && b.end >= 24 * 60
  }

  const allDay = computed(() => allBlocks.value.filter(isAllDayBlock))
  const blocks = computed(() => allBlocks.value.filter((b) => !isAllDayBlock(b)))

  // Freie Zeit ab jetzt bis Tagesende (Einstellungen „Sichtbarer Tagesbereich“), auf 5 Minuten gerundet.
  const freeGaps = computed(() => {
    const roundedNow = Math.ceil(nowMinutes.value / 5) * 5
    const fromMinute = Math.max(roundedNow, prefs.dayStart * 60)
    const toMinute = prefs.dayEnd * 60
    const unfinishedBlocks = blocks.value.filter((b) => !b.done)

    return gaps(unfinishedBlocks, fromMinute, toMinute, 30)
  })

  // Offene Aufgaben für heute ohne Uhrzeit (inkl. Überfälligem) stehen in der Agenda unter den Zeiten.
  const unscheduled = computed(() => {
    const overdueList = today.value?.overdue ?? []
    const todayTasks = today.value?.tasks ?? []
    const combined = [...overdueList, ...todayTasks]

    return combined.filter((task) => isOpen(task) && !task.planned_start_at)
  })

  // Minuten seit Mitternacht am heutigen Tag als Zeitpunkt, in der Zeitzone des Backends.
  function instantAt(targetMinute: number): Date {
    const dayBase = Date.parse(`${today.value!.date}T00:00:00Z`)
    const roughTimestamp = dayBase + targetMinute * 60_000
    const zoneOffset =
      (((getMinutesOfDay(roughTimestamp) - targetMinute + 720) % 1440) + 1440) % 1440 - 720

    return new Date(roughTimestamp - zoneOffset * 60_000)
  }

  // Task auf eine Uhrzeit heute legen; Rückgängig stellt den bisherigen Tag und die Uhrzeit wieder her.
  function scheduleAt(task: Task, minute: number) {
    const { id, title, planned_date, planned_start_at } = task
    const targetDate = today.value!.date
    const targetIso = instantAt(minute).toISOString()

    void run(async () => {
      await updateTask(id, {
        planned_date: targetDate,
        planned_start_at: targetIso,
      })

      offer(t('„{title}“ eingeplant', { title }), () =>
        run(() =>
          updateTask(id, {
            planned_date: planned_date ?? '',
            planned_start_at: planned_start_at ?? '',
          }),
        ),
      )
    })
  }

  // „+ planen“ in einer Lücke: TodayView zeigt die Auswahl offener Aufgaben.
  const planAt = ref<number | null>(null)

  return {
    today,
    entries,
    tasks,
    tasksById,
    projectNames,
    now,
    nowMin: nowMinutes,
    error,
    load,
    run,
    offer,
    setDone,
    fmt: formatTime,
    minsOf: getMinutesOfDay,
    runEntry: runningEntry,
    runningTaskId,
    pausedTask,
    runSecs: runningSeconds,
    blocks,
    allDay,
    freeGaps,
    unscheduled,
    scheduleAt,
    planAt,
  }
}

export type Home = ReturnType<typeof createHome>
const HOME_KEY: InjectionKey<Home> = Symbol('home')

export function provideHome(): Home {
  const home = createHome()
  provide(HOME_KEY, home)
  return home
}

export function useHome(): Home {
  return inject(HOME_KEY)!
}
