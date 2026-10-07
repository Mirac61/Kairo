import { ref } from 'vue'
import { getToday, getTrash, isOpen, listProjects, listTasks, listTimeEntries } from '@/api/client'
import { addDays, entryMinutes, weekStart, ymd } from '@/lib/dates'

// Zähler und Projekte der Sidebar. Ein gemeinsamer Zustand; App.vue lädt ihn bei jedem Live-Ereignis neu.
export const counts = ref({ tasks: 0, habits: '', projects: 0, trash: 0 })
export const pins = ref<{ id: string; name: string; color: string; minutes: number }[]>([])

const PINS = 4

export async function loadSidebar() {
  try {
    const mon = weekStart()
    const [tasks, projects, today, trash, times] = await Promise.all([
      listTasks(), listProjects(), getToday(ymd(new Date())), getTrash(), listTimeEntries(mon, addDays(mon, 7)),
    ])
    const week = new Map<string, number>() // erfasste Minuten je Projekt; ein laufender Eintrag zählt bis jetzt
    for (const e of times) {
      if (e.project_id) week.set(e.project_id, (week.get(e.project_id) ?? 0) + entryMinutes(e))
    }
    const active = projects.filter((p) => p.status === 'ACTIVE')
    counts.value = {
      tasks: tasks.filter(isOpen).length,
      habits: today.habits.length ? `${today.habits.filter((h) => h.done).length}/${today.habits.length}` : '',
      projects: active.length,
      trash: trash.tasks.length + trash.events.length + trash.habits.length,
    }
    pins.value = active
      .map((p) => ({ id: p.id, name: p.name, color: p.color, minutes: week.get(p.id) ?? 0 }))
      .sort((a, b) => b.minutes - a.minutes || a.name.localeCompare(b.name, 'de'))
      .slice(0, PINS)
  } catch {
    /* Backend nicht erreichbar: die Zähler bleiben stehen, der Status unten zeigt „Getrennt“ */
  }
}
