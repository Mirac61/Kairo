import { reactive } from 'vue'

// Palette wie im Backend (domain.ProjectColors) mit Anzeigenamen für die Auswahl.
export const PROJECT_COLORS = [
  ['violet', 'Violett'], ['blue', 'Blau'], ['orange', 'Orange'], ['aqua', 'Türkis'],
  ['pink', 'Rosa'], ['yellow', 'Gelb'], ['green', 'Grün'], ['red', 'Rot'],
] as const

// Farbe je Projekt-ID. listProjects füllt sie, damit jede Ansicht dieselbe Farbe zeigt.
const colors = reactive(new Map<string, string>())
export const setProjectColors = (ps: { id: string; color: string }[]) => ps.forEach((p) => colors.set(p.id, p.color))

export function projectColor(id: string | null | undefined): string {
  const c = id ? colors.get(id) : undefined
  return `var(--p-${c ?? 'neutral'})`
}
