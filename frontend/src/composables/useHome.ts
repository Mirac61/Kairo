import { computed, inject, onMounted, onUnmounted, provide, ref, type InjectionKey } from 'vue'
import { getToday, isOpen, listProjects, listTasks, listTimeEntries, updateTask, type Task, type TimeEntry, type Today } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useUndo } from '@/composables/useUndo'
import { blockEndMinutes, doneSeconds, gaps, minutesOfDay } from '@/lib/dayPlan'
import { locale, t } from '@/lib/i18n'
import { prefs } from '@/lib/prefs'
import { projectColor } from '@/lib/projectColor'

// Gemeinsame Daten der Startseite: HomeView lädt sie einmal, die Widgets holen sie mit useHome().
// Widgets mit eigenen Daten (Ziel, Lernkarten, Journal, Woche, Notiz, GitHub) laden diese selbst.

export interface Block { key: string; title: string; start: number; end: number; kind: 'event' | 'task'; done: boolean; running?: boolean; color: string; sub: string; task?: Task }

function createHome() {
  const { offer, setDone } = useUndo()
  const today = ref<Today | null>(null)
  const entries = ref<TimeEntry[]>([])
  const tasks = ref<Task[]>([])
  const projectNames = ref(new Map<string, string>())
  const now = ref(Date.now())
  let tick: ReturnType<typeof setInterval> | undefined

  const { error, load, run } = useLoader(async () => {
    const td = await getToday()
    const from = new Date(`${td.date}T00:00:00`)
    const [es, ts, ps] = await Promise.all([listTimeEntries(from, new Date(from.getTime() + 864e5)), listTasks(), listProjects()])
    today.value = td
    entries.value = es.reverse()
    tasks.value = ts
    projectNames.value = new Map(ps.map((x) => [x.id, x.name]))
  })
  useLiveEvents(load)
  onMounted(() => {
    void load()
    tick = setInterval(() => (now.value = Date.now()), 1000)
  })
  onUnmounted(() => clearInterval(tick))

  const tasksById = computed(() => new Map(tasks.value.map((x) => [x.id, x])))
  // Das Backend meldet ohne konfigurierte Zeitzone "Local"; das ist kein gültiger IANA-Name.
  const tz = computed(() => (today.value?.timezone === 'Local' ? undefined : today.value?.timezone))
  const fmt = (iso: string) => new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit', timeZone: tz.value }).format(new Date(iso))
  const minsOf = (ms: number) => minutesOfDay(ms, tz.value)
  const nowMin = computed(() => minsOf(now.value))

  const runEntry = computed(() => today.value?.running_time_entry ?? null)
  const runningTaskId = computed(() => runEntry.value?.task_id ?? null)
  // Zuletzt pausierte Task von heute: bleibt im Jetzt-Widget, damit man sie fortsetzen kann.
  const pausedTask = computed(() => {
    if (runEntry.value) return undefined
    const e = [...entries.value].reverse().find((x) => x.task_id && tasksById.value.get(x.task_id)?.status === 'PAUSED')
    return e ? tasksById.value.get(e.task_id!) : undefined
  })
  // Heute schon erfasste Zeit der Task plus laufender Eintrag, damit der Timer nach Pause weiterzählt.
  const runSecs = computed(() => {
    const e = runEntry.value
    if (e) return (e.task_id ? doneSeconds(entries.value, e.task_id) : 0) + Math.max(0, (now.value - Date.parse(e.started_at)) / 1000)
    return pausedTask.value ? doneSeconds(entries.value, pausedTask.value.id) : 0
  })

  // Termine und Tasks mit Uhrzeit als Blöcke; Tasks ohne Dauer zählen 30 Minuten.
  const allBlocks = computed<Block[]>(() => {
    const td = today.value
    if (!td) return []
    const out: Block[] = td.events.map((e) => {
      const start = minsOf(Date.parse(e.occurrence_start))
      return {
        key: `e${e.id}${e.occurrence_start}`, title: e.title, kind: 'event', done: false, color: 'var(--a-event)', sub: e.location,
        start, end: blockEndMinutes(start, minsOf(Date.parse(e.occurrence_end))),
      }
    })
    for (const task of td.tasks) {
      if (!task.planned_start_at) continue
      const start = minsOf(Date.parse(task.planned_start_at))
      out.push({
        key: `t${task.id}`, title: task.title, kind: 'task', done: task.status === 'COMPLETED', running: task.id === runningTaskId.value, sub: '', task,
        color: projectColor(task.project_id), start, end: start + (task.estimated_minutes || 30),
      })
    }
    return out.sort((a, b) => a.start - b.start)
  })
  // Ganztägige Termine stehen im Kopf, nicht im Raster.
  const isAllDay = (b: Block) => b.kind === 'event' && b.start === 0 && b.end >= 24 * 60
  const allDay = computed(() => allBlocks.value.filter(isAllDay))
  const blocks = computed(() => allBlocks.value.filter((b) => !isAllDay(b)))
  // Freie Zeit ab jetzt bis Tagesende (Einstellungen „Sichtbarer Tagesbereich“), auf 5 Minuten gerundet.
  const freeGaps = computed(() => {
    const from = Math.max(Math.ceil(nowMin.value / 5) * 5, prefs.dayStart * 60)
    return gaps(blocks.value.filter((b) => !b.done), from, prefs.dayEnd * 60, 30)
  })
  // Offene Aufgaben für heute ohne Uhrzeit (inkl. Überfälligem) stehen in der Agenda unter den Zeiten.
  const unscheduled = computed(() => [...(today.value?.overdue ?? []), ...(today.value?.tasks ?? [])].filter((x) => isOpen(x) && !x.planned_start_at))

  // Minuten seit Mitternacht am heutigen Tag als Zeitpunkt, in der Zeitzone des Backends.
  function instantAt(min: number) {
    const guess = Date.parse(`${today.value!.date}T00:00:00Z`) + min * 60_000
    const off = (((minsOf(guess) - min + 720) % 1440) + 1440) % 1440 - 720
    return new Date(guess - off * 60_000)
  }

  // Task auf eine Uhrzeit heute legen; Rückgängig stellt den bisherigen Tag und die Uhrzeit wieder her.
  function scheduleAt(task: Task, min: number) {
    const { id, title, planned_date, planned_start_at } = task
    void run(async () => {
      await updateTask(id, { planned_date: today.value!.date, planned_start_at: instantAt(min).toISOString() })
      offer(t('„{title}“ eingeplant', { title }), () => run(() => updateTask(id, { planned_date: planned_date ?? '', planned_start_at: planned_start_at ?? '' })))
    })
  }

  // „+ planen“ in einer Lücke: HomeView zeigt die Auswahl offener Aufgaben.
  const planAt = ref<number | null>(null)

  return {
    today, entries, tasks, tasksById, projectNames, now, nowMin, error, load, run, offer, setDone, fmt, minsOf,
    runEntry, runningTaskId, pausedTask, runSecs, blocks, allDay, freeGaps, unscheduled, scheduleAt, planAt,
  }
}

export type Home = ReturnType<typeof createHome>
const KEY: InjectionKey<Home> = Symbol('home')

export function provideHome() {
  const home = createHome()
  provide(KEY, home)
  return home
}

export const useHome = () => inject(KEY)!
