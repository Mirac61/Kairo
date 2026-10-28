import { computed, nextTick, ref, watch, type Ref } from 'vue'
import { isOpen, type Project, type Task } from '@/api/client'
import { addDays, dayLabel, weekStart, ymd } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { store } from '@/lib/storage'

const VIEW_STORAGE_KEY = 'kairo-tasks-view'

export interface TaskGroup {
  key: string
  title: string
  rows: Task[]
  minutes: number
}

export function useTaskFilter(
  tasks: Ref<Task[]>,
  projects: Ref<Project[]>,
) {
  const filter = ref('alle')
  const projectId = ref('alle')
  const search = ref('')

  const view = ref({ sort: 'created', group: 'due' })
  try {
    const saved = store.get(VIEW_STORAGE_KEY)
    if (saved) {
      Object.assign(view.value, JSON.parse(saved))
    }
  } catch {
    // Ungültiger oder beschädigter Wert im Storage wird ignoriert
  }

  watch(
    view,
    (updated) => {
      store.set(VIEW_STORAGE_KEY, JSON.stringify(updated))
    },
    { deep: true },
  )

  const FILTERS = [
    { id: 'alle', label: t('Offen') },
    { id: 'ueber', label: t('Überfällig') },
    { id: 'heute', label: t('Heute') },
    { id: 'woche', label: t('Nächste 7 Tage') },
    { id: 'spaeter', label: t('Später') },
    { id: 'ohne', label: t('Ohne Datum') },
    { id: 'erledigt', label: t('Erledigt') },
    { id: 'prio', label: t('Prio hoch') },
    { id: 'journal', label: t('Aus Journal') },
    { id: 'abgebrochen', label: t('Abgebrochen') },
  ]

  const SORTS = [
    ['created', t('Angelegt')],
    ['day', t('Tag')],
    ['prio', t('Priorität')],
    ['title', t('Titel')],
  ] as const

  const GROUPS = [
    ['due', t('Fällig')],
    ['none', t('Keine')],
    ['project', t('Projekt')],
    ['day', t('Tag')],
  ] as const

  const DUE_TITLES: Record<string, string> = {
    '0': t('Überfällig'),
    '1': t('Früher'),
    '3': t('Diese Woche'),
    '4': t('Später'),
    '5': t('Ohne Datum'),
  }

  const PRIORITY_RANK: Record<string, number> = {
    URGENT: 0,
    HIGH: 1,
    MEDIUM: 2,
    LOW: 3,
  }

  // Tasten 1-7 für Schnellfilter
  const MAIN_FILTER_IDS = FILTERS.slice(0, 7).map((f) => f.id)
  const SHOWN_DEFAULT_TABS = ['alle', 'heute', 'erledigt']

  const projectNameMap = computed(() => {
    return new Map(projects.value.map((p) => [p.id, p.name]))
  })

  const todayString = () => ymd(new Date())
  const nextWeekStart = () => ymd(addDays(weekStart(), 7))

  // Der maßgebliche Tag für eine Aufgabe: geplanter Tag, sonst Fälligkeit
  function dayOf(task: Task): string | null {
    if (task.planned_date) {
      return task.planned_date
    }
    if (task.due_at) {
      return ymd(new Date(task.due_at))
    }
    return null
  }

  function inScope(task: Task): boolean {
    if (projectId.value !== 'alle' && task.project_id !== projectId.value) {
      return false
    }
    const query = search.value.trim().toLowerCase()
    if (!query) {
      return true
    }
    const haystack = `${task.title} ${task.description}`.toLowerCase()
    return haystack.includes(query)
  }

  function matches(task: Task, filterId: string): boolean {
    const taskDay = dayOf(task)
    const today = todayString()

    switch (filterId) {
      case 'erledigt':
        return task.status === 'COMPLETED'

      case 'abgebrochen':
        return task.status === 'CANCELLED'

      case 'heute':
        return isOpen(task) && taskDay === today

      case 'woche': {
        const inNext7Days = ymd(addDays(new Date(), 7))
        return (
          isOpen(task) &&
          taskDay !== null &&
          taskDay > today &&
          taskDay <= inNext7Days
        )
      }

      case 'spaeter': {
        const inNext7Days = ymd(addDays(new Date(), 7))
        return isOpen(task) && taskDay !== null && taskDay > inNext7Days
      }

      case 'ohne':
        return isOpen(task) && taskDay === null

      case 'ueber':
        return isOpen(task) && taskDay !== null && taskDay < today

      case 'prio':
        return (
          isOpen(task) &&
          (task.priority === 'HIGH' || task.priority === 'URGENT')
        )

      case 'journal':
        return isOpen(task) && Boolean(task.source_journal_date)

      default:
        return isOpen(task)
    }
  }

  const visible = computed(() => {
    return tasks.value.filter((t) => inScope(t) && matches(t, filter.value))
  })

  const counts = computed(() => {
    const result: Record<string, number> = {}
    for (const f of FILTERS) {
      result[f.id] = tasks.value.filter((t) => inScope(t) && matches(t, f.id)).length
    }
    return result
  })

  const tabs = computed(() => {
    return FILTERS.filter(
      (f) => SHOWN_DEFAULT_TABS.includes(f.id) || f.id === filter.value,
    )
  })

  const extra = computed(() => {
    return FILTERS.filter(
      (f) => !SHOWN_DEFAULT_TABS.includes(f.id) && f.id !== filter.value,
    )
  })

  function onTabKey(e: KeyboardEvent) {
    const tabIds = tabs.value.map((f) => f.id)
    const currentIndex = tabIds.indexOf(filter.value)

    let targetIndex: number | undefined
    if (e.key === 'ArrowRight') {
      targetIndex = currentIndex + 1
    } else if (e.key === 'ArrowLeft') {
      targetIndex = currentIndex - 1 + tabIds.length
    } else if (e.key === 'Home') {
      targetIndex = 0
    } else if (e.key === 'End') {
      targetIndex = tabIds.length - 1
    }

    if (targetIndex === undefined) {
      return
    }

    e.preventDefault()
    filter.value = tabIds[targetIndex % tabIds.length]!
    void nextTick(() => {
      document.getElementById(`tk-tab-${filter.value}`)?.focus()
    })
  }

  // Teilaufgaben werden im Detail ihrer Elternaufgabe angezeigt;
  // in der Hauptliste nur, falls die Elternaufgabe nicht gelistet ist.
  const rows = computed(() => {
    const taskMap = new Map(visible.value.map((t) => [t.id, t]))
    return visible.value.filter((task) => {
      const parent = task.parent_task_id ? taskMap.get(task.parent_task_id) : undefined
      return !parent || Boolean(parent.parent_task_id)
    })
  })

  function compareSort(a: Task, b: Task): number {
    switch (view.value.sort) {
      case 'day': {
        const dayA = dayOf(a) ?? '9'
        const dayB = dayOf(b) ?? '9'
        const dayDiff = dayA.localeCompare(dayB)
        if (dayDiff !== 0) return dayDiff

        const startA = a.planned_start_at ?? ''
        const startB = b.planned_start_at ?? ''
        return startA.localeCompare(startB)
      }

      case 'prio': {
        const rankA = PRIORITY_RANK[a.priority] ?? 2
        const rankB = PRIORITY_RANK[b.priority] ?? 2
        return rankA - rankB
      }

      case 'title':
        return a.title.localeCompare(b.title, locale.value)

      default:
        // 'created': Behält Reihenfolge der API bei
        return 0
    }
  }

  function dueBucket(task: Task): string {
    const day = dayOf(task)
    if (!day) return '5'

    const today = todayString()
    if (day < today) {
      return isOpen(task) ? '0' : '1'
    }
    if (day === today) {
      return '2'
    }
    if (day < nextWeekStart()) {
      return '3'
    }
    return '4'
  }

  function formatGroupTitle(groupBy: string, groupKey: string): string {
    const today = todayString()
    const todayFormatted = new Intl.DateTimeFormat(locale.value, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
    })
      .format(new Date())
      .replace('.,', '')

    if (groupBy === 'due') {
      if (groupKey === '2') {
        return `${t('Heute')} · ${todayFormatted}`
      }
      return DUE_TITLES[groupKey] ?? t('Ohne Datum')
    }

    if (groupBy === 'project') {
      if (!groupKey) {
        return t('Ohne Projekt')
      }
      return projectNameMap.value.get(groupKey) ?? t('Projekt')
    }

    // groupBy === 'day'
    if (!groupKey) {
      return t('Ohne Tag')
    }
    return dayLabel(groupKey, today)
  }

  const groups = computed<TaskGroup[]>(() => {
    const sortedTasks = [...rows.value].sort(compareSort)
    const groupBy = view.value.group

    if (!sortedTasks.length) {
      return []
    }

    if (groupBy !== 'project' && groupBy !== 'day' && groupBy !== 'due') {
      return [
        {
          key: '',
          title: '',
          rows: sortedTasks,
          minutes: 0,
        },
      ]
    }

    const resolveKey = (task: Task): string => {
      if (groupBy === 'project') return task.project_id ?? ''
      if (groupBy === 'due') return dueBucket(task)
      return dayOf(task) ?? ''
    }

    const groupMap = new Map<string, Task[]>()
    for (const task of sortedTasks) {
      const key = resolveKey(task)
      const existing = groupMap.get(key) ?? []
      existing.push(task)
      groupMap.set(key, existing)
    }

    return [...groupMap]
      .sort(([keyA], [keyB]) => {
        const emptyA = Number(keyA === '')
        const emptyB = Number(keyB === '')
        if (emptyA !== emptyB) {
          return emptyA - emptyB
        }
        if (groupBy === 'project') {
          const titleA = formatGroupTitle(groupBy, keyA)
          const titleB = formatGroupTitle(groupBy, keyB)
          return titleA.localeCompare(titleB, locale.value)
        }
        return keyA.localeCompare(keyB)
      })
      .map(([key, groupRows]) => ({
        key,
        title: formatGroupTitle(groupBy, key),
        rows: groupRows,
        minutes: groupRows.reduce((sum, task) => sum + task.estimated_minutes, 0),
      }))
  })

  const flat = computed(() => groups.value.flatMap((g) => g.rows))

  return {
    filter,
    projectId,
    search,
    view,
    FILTERS,
    SORTS,
    GROUPS,
    MAIN_FILTER_IDS,
    projectNameMap,
    todayString,
    dayOf,
    inScope,
    matches,
    visible,
    counts,
    tabs,
    extra,
    rows,
    groups,
    flat,
    onTabKey,
  }
}
