import { reactive } from 'vue'

// Palette wie im Backend (domain.ProjectColors) mit Anzeigenamen für die Auswahl.
export const PROJECT_COLORS = [
  ['violet', 'Violett'], ['blue', 'Blau'], ['orange', 'Orange'], ['aqua', 'Türkis'],
  ['pink', 'Rosa'], ['yellow', 'Gelb'], ['green', 'Grün'], ['red', 'Rot'],
] as const

// Farbe je Projekt-ID. listProjects füllt sie, damit jede Ansicht dieselbe Farbe zeigt.
const colors = reactive(new Map<string, string>())
export const setProjectColors = (ps: { id: string; color: string }[]) => ps.forEach((p) => colors.set(p.id, p.color))

// Gewohnheiten haben keine Farbe im Backend: stabile Zuordnung aus der ID (bei Bedarf ein Feld wie bei Projekten).
export function habitColor(id: string): string {
  let h = 0
  for (const c of id) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return `var(--p-${PROJECT_COLORS[h % PROJECT_COLORS.length]![0]})`
}

export function projectColor(id: string | null | undefined): string {
  const c = id ? colors.get(id) : undefined
  return `var(--p-${c ?? 'neutral'})`
}
