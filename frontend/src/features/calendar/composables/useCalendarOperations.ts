import { ref, type Ref } from 'vue'
import type { Router } from 'vue-router'
import type FullCalendar from '@fullcalendar/vue3'
import type { EventClickArg, EventDropArg } from '@fullcalendar/core'
import type { EventResizeDoneArg } from '@fullcalendar/interaction'
import {
  completeHabit,
  createEvent,
  createTask,
  deleteEvent,
  errorMessage,
  importIcs,
  restoreEvent,
  skipOccurrence,
  uncompleteHabit,
  updateEvent,
  updateHabit,
  updateTask,
  type CalendarEvent,
  type Task,
} from '@/api/client'
import { editForm, newForm, requestOf, type Form } from '@/lib/eventForm'
import { addDays, hhmm, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'

export function useCalendarOperations(
  cal: Ref<InstanceType<typeof FullCalendar> | undefined>,
  unplannedTasks: Ref<Task[]>,
  offer: (label: string, undo: () => void) => void,
  router: Router,
) {
  const error = ref('')
  const notice = ref<{ text: string; notes: string[] } | null>(null)
  const form = ref<Form | null>(null)

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

    const actionLabel = t(resized ? '„{title}“ angepasst' : '„{title}“ verschoben', {
      title: event.title,
    })

    if (kind === 'habit') {
      if (ymd(start) !== ymd(old.start)) return info.revert()
      const p = event.extendedProps as { dur: number; time: string | null }
      const before = { preferred_time: p.time ?? '', duration_minutes: p.dur }
      void guarded(async () => {
        const body: { preferred_time: string; duration_minutes?: number } = {
          preferred_time: event.allDay ? '' : hhmm(start),
        }
        if (resized && event.end) {
          body.duration_minutes = Math.round((event.end.getTime() - start.getTime()) / 60_000)
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
          body.estimated_minutes = Math.round((event.end.getTime() - start.getTime()) / 60_000)
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

  const openForm = (start: Date, end: Date, allDay: boolean) =>
    (form.value = newForm(start, end, allDay))
  const openEdit = (ev: CalendarEvent, day: string) => (form.value = editForm(ev, day))

  function openNew() {
    const start = new Date()
    start.setHours(start.getHours() + 1, 0, 0, 0)
    openForm(start, new Date(start.getTime() + 3_600_000), false)
  }

  function save(savedForm?: Form) {
    const f = savedForm ?? form.value
    const title = f?.title.trim()
    if (!f || !title) return
    form.value = null
    const r = requestOf(f, title)
    void guarded(() =>
      'task' in r ? createTask(r.task) : f.id ? updateEvent(f.id, r.event) : createEvent(r.event),
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
      void guarded(() => (p.done ? uncompleteHabit(p.id, p.day) : completeHabit(p.id, p.day)))
    } else if (p.kind === 'task') {
      void router.push({ path: '/tasks', query: { task: p.id } })
    } else {
      openEdit(p.ev, ymd(info.event.start!))
    }
  }

  function remove(onlyThis: boolean, savedForm?: Form) {
    const f = savedForm ?? form.value
    form.value = null
    if (!f?.id) return
    const { id, day, title } = f
    void guarded(async () => {
      if (onlyThis) {
        const before = await skipOccurrence(id, day)
        offer(t('„{title}“ gelöscht', { title }), () =>
          guarded(() => updateEvent(id, { recurrence_exdates: before })),
        )
      } else {
        await deleteEvent(id)
        offer(t('„{title}“ gelöscht', { title }), () => guarded(() => restoreEvent(id)))
      }
    })
  }

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
        guarded(() => updateTask(task.id, before)),
      )
    })
  }

  async function importFile(file: File) {
    try {
      const r = await importIcs(await file.text())
      const parts = [t('{n} neu', { n: r.created }), t('{n} aktualisiert', { n: r.updated })]
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

  function handleDrop(a: { draggedEl: HTMLElement; date: Date; allDay: boolean }) {
    const id = a.draggedEl.dataset.id
    if (!id) return
    const body: Record<string, unknown> = { planned_date: ymd(a.date) }
    if (!a.allDay) body.planned_start_at = a.date.toISOString()
    const taskTitle = unplannedTasks.value.find((t) => t.id === id)?.title
    void guarded(async () => {
      await updateTask(id, body)
      offer(t('„{title}“ geplant', { title: taskTitle ?? '' }), () =>
        guarded(() => updateTask(id, { planned_date: '', planned_start_at: '' })),
      )
    })
  }

  return {
    error,
    notice,
    form,
    guarded,
    moved,
    clicked,
    openForm,
    openEdit,
    openNew,
    save,
    remove,
    postpone,
    importFile,
    handleDrop,
  }
}
