import type { Ref } from 'vue'
import { deleteTask, restoreTask, updateTask, type Task } from '@/api/client'
import { dayLabel, daysAgo, dm, hhmm, shortDay } from '@/lib/dates'
import { t } from '@/lib/i18n'

export function useTaskOperations(
  run: (fn: () => Promise<unknown>) => Promise<boolean>,
  offer: (text: string, undo: () => void) => void,
  setDone: (
    task: Task,
    status: 'COMPLETED' | 'CANCELLED',
    runner: (fn: () => Promise<unknown>) => unknown,
  ) => unknown,
  todayString: () => string,
  dayOf: (task: Task) => string | null,
  overdue: (task: Task) => boolean,
  openId: Ref<string | null>,
) {
  function formatTaskWhen(task: Task): { text: string; tip: string } {
    const d = dayOf(task)
    if (!d) return { text: '', tip: '' }

    if (overdue(task)) {
      return { text: daysAgo(d, todayString()), tip: `${t('Fällig')} ${shortDay(d)}` }
    }
    if (!task.planned_date) {
      return { text: `${t('fällig')} ${dm(d)}`, tip: '' }
    }

    const timeStr = task.planned_start_at ? hhmm(new Date(task.planned_start_at)) : ''
    return {
      text: `${dayLabel(d, todayString())} ${timeStr}`.trim(),
      tip: '',
    }
  }

  function setDate(task: Task, date: string) {
    const time = task.planned_start_at ? hhmm(new Date(task.planned_start_at)) : ''
    void run(() =>
      updateTask(task.id, {
        planned_date: date,
        planned_start_at: date && time ? new Date(`${date}T${time}`).toISOString() : '',
      }),
    )
  }

  function setTime(task: Task, time: string) {
    if (!task.planned_date) return
    void run(() =>
      updateTask(task.id, {
        planned_start_at: time ? new Date(`${task.planned_date}T${time}`).toISOString() : '',
      }),
    )
  }

  function setEstimate(task: Task, minutes: string) {
    void run(() => updateTask(task.id, { estimated_minutes: Number(minutes) || 0 }))
  }

  function setField(
    task: Task,
    field: 'title' | 'description' | 'priority' | 'project_id' | 'status' | 'recurrence_rule',
    value: string,
  ) {
    if (field === 'title' && !value.trim()) return
    if (field === 'status' && value === 'CANCELLED') return setDone(task, 'CANCELLED', run)
    void run(() => updateTask(task.id, { [field]: value }))
  }

  function setDue(task: Task, day: string) {
    void run(() =>
      updateTask(task.id, {
        due_at: day ? new Date(`${day}T23:59:00`).toISOString() : '',
      }),
    )
  }

  function toggleTaskStatus(task: Task) {
    if (task.status === 'COMPLETED') {
      return run(() => updateTask(task.id, { status: task.planned_date ? 'PLANNED' : 'BACKLOG' }))
    }
    return setDone(task, 'COMPLETED', run)
  }

  function trashTask(task: Task) {
    void run(async () => {
      await deleteTask(task.id)
      offer(t('„{title}“ gelöscht', { title: task.title }), () => run(() => restoreTask(task.id)))
      if (openId.value === task.id) {
        openId.value = null
      }
    })
  }

  return {
    formatTaskWhen,
    setDate,
    setTime,
    setEstimate,
    setField,
    setDue,
    toggleTaskStatus,
    trashTask,
  }
}
