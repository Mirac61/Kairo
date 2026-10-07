import { ref } from 'vue'
import { taskAction, updateTask, type Task } from '@/api/client'

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
    const { id, title } = task
    const prev = task.status === 'IN_PROGRESS' ? 'PAUSED' : task.status
    void run(async () => {
      await (status === 'COMPLETED' ? taskAction(id, 'complete') : updateTask(id, { status }))
      offer(`„${title}“ ${status === 'COMPLETED' ? 'erledigt' : 'abgebrochen'}`, () => run(() => updateTask(id, { status: prev })))
    })
  }
  return { pending, offer, undo, dismiss, setDone }
}
