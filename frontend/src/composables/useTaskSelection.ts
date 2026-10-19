import { ref } from 'vue'
import { bulkUpdateTasks, deleteTask, restoreTask, type Task } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { tn } from '@/lib/i18n'
import { pick } from '@/lib/selection'

// Mehrfachauswahl samt Sammelaktionen. order: Aufgaben in Anzeigereihenfolge, run/tomorrow wie in der Ansicht.
// Shift-/⌘-/Strg-Klick wählt; im Auswahlmodus (Knopf, für Touch) genügt ein Tipp. Esc beendet beides.
// Long-Press (Touch) schaltet in den Auswahlmodus und wählt die Zeile.
export function useTaskSelection(order: () => Task[], run: (fn: () => Promise<unknown>) => unknown, tomorrow: () => string) {
  const picked = ref<string[]>([])
  const mode = ref(false)
  let anchor: string | null = null
  let lpTimer: number | undefined
  const { offer } = useUndo()

  function clear() {
    picked.value = []
    mode.value = false
    anchor = null
  }
  // Als Capture-Handler der Zeile: ein Auswahlklick erreicht Titel und Häkchen darin nicht.
  function onClick(e: MouseEvent, id: string) {
    if (!mode.value && !e.shiftKey && !e.metaKey && !e.ctrlKey) return
    e.preventDefault()
    e.stopPropagation()
    picked.value = pick(order().map((x) => x.id), picked.value, anchor, id, e.shiftKey)
    anchor = id
  }
  // Long-Press auf Touch: Auswahlmodus einschalten und die Zeile dazunehmen (ohne Tastatur).
  function onPointerDown(e: PointerEvent, id: string) {
    if (e.pointerType !== 'touch') return
    lpTimer = window.setTimeout(() => {
      mode.value = true
      if (!picked.value.includes(id)) picked.value = [...picked.value, id]
      navigator.vibrate?.(15)
    }, 500)
  }
  function onPointerUp() {
    if (lpTimer !== undefined) {
      clearTimeout(lpTimer)
      lpTimer = undefined
    }
  }
  useShortcuts((e) => e.key === 'Escape' && clear())

  const chosen = () => order().filter((x) => picked.value.includes(x.id))
  // Eine Sammel-Anfrage statt n einzelner PATCH (bei vielen Aufgaben spürbar); Rückgängig stellt per Bulk den alten Wert wieder her.
  async function patchAll(ts: Task[], patch: (x: Task) => Record<string, unknown>) {
    await bulkUpdateTasks(ts.map((x) => ({ id: x.id, ...patch(x) })))
  }

  function toTomorrow(ts: Task[] = chosen()) {
    if (!ts.length) return
    const day = tomorrow()
    void run(async () => {
      await patchAll(ts, () => ({ planned_date: day, planned_start_at: '' })) // die alte Uhrzeit entfällt
      offer(tn(ts.length, '{n} Aufgabe auf morgen verschoben', '{n} Aufgaben auf morgen verschoben'),
        () => run(() => patchAll(ts, (x) => ({ planned_date: x.planned_date ?? '', planned_start_at: x.planned_start_at ?? '' }))))
    })
    clear()
  }
  function toProject(projectId: string) {
    const ts = chosen()
    if (!ts.length) return
    void run(async () => {
      await patchAll(ts, () => ({ project_id: projectId }))
      offer(tn(ts.length, '{n} Aufgabe verschoben', '{n} Aufgaben verschoben'), () => run(() => patchAll(ts, (x) => ({ project_id: x.project_id ?? '' }))))
    })
    clear()
  }
  // Löschen legt in den Papierkorb; Rückgängig holt alle zurück. Teilaufgaben, deren
  // Elternaufgabe mitgewählt ist, fallen heraus (das Löschen der Eltern nimmt sie ohnehin mit).
  function trash() {
    const ts = chosen().filter((x) => !x.parent_task_id || !picked.value.includes(x.parent_task_id))
    if (!ts.length) return
    void run(async () => {
      for (const x of ts) await deleteTask(x.id)
      offer(tn(ts.length, '{n} Aufgabe gelöscht', '{n} Aufgaben gelöscht'), () => run(async () => { for (const x of ts) await restoreTask(x.id) }))
    })
    clear()
  }
  return { picked, mode, onClick, onPointerDown, onPointerUp, clear, toTomorrow, toProject, trash }
}
