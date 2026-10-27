// Widget-Raster der Startseite: welche Widgets es gibt, welche Größen sie können, wohin ein Klick führt, und die Standard-Anordnungen.
// Ohne Vue, damit test/homeLayout.test.ts es direkt mit Node prüfen kann.

// Größen wie bei iOS: S 1×1, M 2×1, L 2×2 Felder (Desktop 4 Spalten, Handy 2).
export type Size = 'S' | 'M' | 'L'
export type Device = 'desktop' | 'mobile'

export const WIDGETS = {
  dayplan: { title: 'Tagesplan', sizes: ['M', 'L'], to: '/calendar' },
  agenda: { title: 'Agenda „Heute“', sizes: ['M', 'L'], to: '/calendar' },
  now: { title: 'Jetzt', sizes: ['S', 'M'], to: '/tasks' },
  habits: { title: 'Gewohnheiten', sizes: ['S', 'M'], to: '/habits' },
  goal: { title: 'Ziel', sizes: ['S', 'M'], to: '/goals' },
  cards: { title: 'Lernkarten', sizes: ['S'], to: '/cards' },
  journal: { title: 'Journal', sizes: ['S', 'M'], to: '/journal' },
  week: { title: 'Woche', sizes: ['S', 'M'], to: '/review' },
  note: { title: 'Angepinnte Notiz', sizes: ['S', 'M'], to: '/notes' },
  github: { title: 'GitHub', sizes: ['S', 'M', 'L'], to: '/tasks' },
} as const satisfies Record<string, { title: string; sizes: readonly Size[]; to: string }>

export type WidgetId = keyof typeof WIDGETS
export interface Slot { id: WidgetId; size: Size }

export const DEFAULTS: Record<Device, Slot[]> = {
  // Nur was heute eine Entscheidung auslöst; Ziel, Journal, Woche, Notiz und GitHub kommen über „Anpassen“ dazu.
  desktop: [{ id: 'dayplan', size: 'L' }, { id: 'now', size: 'M' }, { id: 'habits', size: 'S' }, { id: 'cards', size: 'S' }],
  mobile: [{ id: 'now', size: 'M' }, { id: 'agenda', size: 'L' }, { id: 'habits', size: 'S' }, { id: 'cards', size: 'S' }],
}

export const sizesOf = (id: WidgetId): readonly Size[] => WIDGETS[id].sizes

// Gespeicherte Anordnung lesen: unbekannte Widgets, falsche Größen und Doppelte fallen weg.
// Kein gültiges JSON-Array (auch leer = nie angepasst) → Standard; ein gespeichertes [] bleibt leer.
export function parseLayout(json: string, device: Device): Slot[] {
  let raw: unknown
  try {
    raw = JSON.parse(json)
  } catch {
    raw = null
  }
  if (!Array.isArray(raw)) return DEFAULTS[device].map((s) => ({ ...s }))
  const seen = new Set<string>()
  const out: Slot[] = []
  for (const s of raw as Partial<Slot>[]) {
    const id = s?.id
    if (!id || !Object.hasOwn(WIDGETS, id) || seen.has(id) || !sizesOf(id).includes(s.size as Size)) continue
    seen.add(id)
    out.push({ id, size: s.size as Size })
  }
  return out
}

// Element von from an Stelle to verschieben (für Ziehen und die Pfeile am Handy).
export function moveSlot<T>(list: T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length || from === to) return list
  const out = [...list]
  out.splice(to, 0, ...out.splice(from, 1))
  return out
}
