import { ref } from 'vue'
import { deleteTask, taskAction, updateTask, type Task } from '@/api/client'
import { t } from '@/lib/i18n'

const TTL_MS = 8_000

// Ein Toast zugleich: ein neues Angebot ersetzt das alte. Der Zustand ist modulweit, App.vue zeigt ihn.
const pending = ref<{ text: string; revert: () => unknown } | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined

export function useUndo() {
  const dismiss = () => {
    clearTimeout(timer)
    pending.value = null
  }
  // revert macht die Aktion rückgängig und lädt die Ansicht neu (meist über deren run/guarded).
  function offer(text: string, revert: () => unknown) {
    clearTimeout(timer)
    pending.value = { text, revert }
    timer = setTimeout(dismiss, TTL_MS)
  }
  function undo() {
    const p = pending.value
    dismiss()
    void p?.revert()
  }
  // Task erledigen oder abbrechen; Rückgängig setzt den vorherigen Status per PATCH.
  // Ein laufender Timer ist dann beendet, daher PAUSED statt IN_PROGRESS.
  function setDone(task: Task, status: 'COMPLETED' | 'CANCELLED', run: (fn: () => Promise<unknown>) => unknown) {
    const { id, title, recurrence_rule } = task
    const prev = task.status === 'IN_PROGRESS' ? 'PAUSED' : task.status
    void run(async () => {
      const res = await (status === 'COMPLETED' ? taskAction(id, 'complete') : updateTask(id, { status }))
      // Wiederkehrend: Rückgängig entfernt die neu angelegte Wiederholung und gibt der Task ihre Regel zurück.
      const next = (res as { next?: Task } | undefined)?.next
      offer(t(status === 'COMPLETED' ? '„{title}“ erledigt' : '„{title}“ abgebrochen', { title }), () => run(async () => {
        if (next) await deleteTask(next.id, true)
        await updateTask(id, { status: prev, ...(next ? { recurrence_rule: recurrence_rule ?? '' } : {}) })
      }))
    })
  }
  return { pending, offer, undo, dismiss, setDone }
}
