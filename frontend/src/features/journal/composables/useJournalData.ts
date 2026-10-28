import { computed, ref } from 'vue'
import {
  errorMessage,
  getJournalSummary,
  getToday,
  isOpen,
  listJournalTaskImports,
  listNotes,
  listTasks,
  readNote,
  type JournalTaskImport,
  type Task,
} from '@/api/client'
import { shiftDate, ymd } from '@/lib/dates'
import { journalPath, parseEntry, splitPoints } from '@/lib/journal'
import { tn } from '@/lib/i18n'
import { useUndo } from '@/composables/useUndo'
import type { JournalEntryData } from '../components/JournalEntryEditor.vue'

const RECENT_ENTRIES_COUNT = 60

export interface JournalRow {
  date: string
  answers: string[]
  other: string
  mtime: string
}

export type DoneTask = Task & { completed_at: string | null }

export function useJournalData() {
  const todayStr = ref('')
  const totalEntriesCount = ref(0)
  const currentStreak = ref(0)
  const isLoaded = ref(false)
  const errorMessageText = ref('')
  const allDates = ref<string[]>([])
  const pastRows = ref<JournalRow[]>([])
  const todayEntry = ref<JournalEntryData | null>(null)
  const taskImports = ref<Record<string, { count: number; titles: string[] }>>({})
  const allTasks = ref<DoneTask[]>([])
  const completedHabits = ref<string[]>([])
  const completingTaskId = ref('')

  const { setDone } = useUndo()

  function getEntryForDate(d: string): JournalRow | JournalEntryData | null {
    if (d === todayStr.value) return todayEntry.value
    return pastRows.value.find((r) => r.date === d) ?? null
  }

  function getDoneItemsOn(dateStr: string): string[] {
    const tasksCompleted = allTasks.value
      .filter(
        (t) =>
          t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) === dateStr,
      )
      .sort((a, b) => (a.completed_at ?? '').localeCompare(b.completed_at ?? ''))
      .map((t) => t.title)

    const habitsCompleted =
      dateStr === todayStr.value && completedHabits.value.length
        ? [tn(completedHabits.value.length, '{n} Gewohnheit', '{n} Gewohnheiten')]
        : []

    return [...tasksCompleted, ...habitsCompleted]
  }

  function getPlanForDate(dateStr: string): string[] {
    const yesterdayStr = shiftDate(dateStr, -1)
    const yesterdayEntry = getEntryForDate(yesterdayStr)
    return splitPoints(yesterdayEntry?.answers[2] ?? '')
  }

  const openJournalTasks = computed(() =>
    allTasks.value
      .filter((t) => t.source_journal_date && isOpen(t))
      .sort((a, b) => (a.due_at ?? '').localeCompare(b.due_at ?? '')),
  )

  function getImportedTitles(dateStr: string): string[] {
    return taskImports.value[dateStr]?.titles ?? []
  }

  async function readEntries(dates: string[]): Promise<JournalRow[]> {
    return Promise.all(
      dates.map(async (date) => {
        const note = await readNote(journalPath(date))
        const entry = parseEntry(note.content)
        return {
          date,
          answers: entry.answers,
          other: entry.other,
          mtime: note.mtime,
        }
      }),
    )
  }

  async function loadData() {
    try {
      const [notes, imports, summary, todayData, tasks] = await Promise.all([
        listNotes(),
        listJournalTaskImports(),
        getJournalSummary(),
        getToday(),
        listTasks(),
      ])

      const journalDir = notes.find((n) => n.dir && n.path === 'Journal')
      const dates = (journalDir?.children ?? [])
        .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
        .filter((d): d is string => !!d)
        .sort()
        .reverse()

      const rows = await readEntries(dates.slice(0, RECENT_ENTRIES_COUNT))
      todayStr.value = summary.date

      totalEntriesCount.value = summary.total
      currentStreak.value = summary.streak
      allDates.value = dates
      taskImports.value = Object.fromEntries(
        imports.map((i) => [i.journal_date, { count: i.task_count, titles: i.titles }]),
      )
      todayEntry.value = rows.find((r) => r.date === summary.date) ?? null
      pastRows.value = rows.filter((r) => r.date !== summary.date)

      allTasks.value = tasks as DoneTask[]
      completedHabits.value = todayData.habits.filter((h) => h.done).map((h) => h.name)

      errorMessageText.value = ''
    } catch (e) {
      errorMessageText.value = errorMessage(e)
    }
    isLoaded.value = true
  }

  function onEntrySaved(date: string, entry: JournalEntryData) {
    if (date === todayStr.value) {
      todayEntry.value = entry
      if (!allDates.value.includes(todayStr.value)) {
        allDates.value = [...allDates.value, todayStr.value].sort().reverse()
      }
    } else {
      pastRows.value = pastRows.value.map((r) => (r.date === date ? { ...r, ...entry } : r))
      if (!allDates.value.includes(date)) {
        allDates.value = [...allDates.value, date].sort().reverse()
        totalEntriesCount.value++
      }
    }
  }

  function isDraftRow(row: JournalRow): boolean {
    return !row.mtime && !row.other.trim() && row.answers.every((a) => !a.trim())
  }

  function removeDraftIfEmpty(date: string) {
    if (pastRows.value.some((r) => r.date === date && isDraftRow(r))) {
      pastRows.value = pastRows.value.filter((r) => r.date !== date)
    }
  }

  async function ensureRowLoaded(date: string): Promise<boolean> {
    if (pastRows.value.some((r) => r.date === date)) return true
    if (allDates.value.includes(date)) {
      try {
        const n = await readNote(journalPath(date))
        const e = parseEntry(n.content)
        const row = { date, answers: e.answers, other: e.other, mtime: n.mtime }
        pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
        return true
      } catch (e) {
        errorMessageText.value = errorMessage(e)
        return false
      }
    } else {
      const row = { date, answers: ['', '', ''], other: '', mtime: '' }
      pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
      return true
    }
  }

  async function refreshTasks() {
    try {
      allTasks.value = (await listTasks()) as DoneTask[]
    } catch (e) {
      errorMessageText.value = errorMessage(e)
    }
  }

  function handleTodayImported(result: JournalTaskImport) {
    taskImports.value = {
      ...taskImports.value,
      [result.journal_date]: { count: result.task_count, titles: result.titles },
    }
    void refreshTasks()
  }

  function handleCompleteTask(task: Task) {
    if (completingTaskId.value) return
    completingTaskId.value = task.id
    setDone(task, 'COMPLETED', async (fn) => {
      try {
        await fn()
        await refreshTasks()
      } catch (e) {
        errorMessageText.value = errorMessage(e)
      } finally {
        completingTaskId.value = ''
      }
    })
  }

  return {
    todayStr,
    totalEntriesCount,
    currentStreak,
    isLoaded,
    errorMessageText,
    allDates,
    pastRows,
    todayEntry,
    taskImports,
    allTasks,
    completedHabits,
    completingTaskId,
    openJournalTasks,
    getEntryForDate,
    getDoneItemsOn,
    getPlanForDate,
    getImportedTitles,
    loadData,
    onEntrySaved,
    removeDraftIfEmpty,
    ensureRowLoaded,
    refreshTasks,
    handleTodayImported,
    handleCompleteTask,
  }
}
