const COLORS = ['violet', 'blue', 'orange', 'aqua', 'pink']

// Feste Farbreihenfolge je Projekt-ID, damit ein Projekt überall gleich aussieht.
export function projectColor(id: string | null | undefined): string {
  if (!id) return 'var(--a-neutral)'
  let h = 0
  for (const c of id) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return `var(--a-${COLORS[h % COLORS.length]})`
}
