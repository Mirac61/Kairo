// Schnelleingabe wie in der Extension (extension/src/core.ts): „30 min Sport“ oder „Sport 30m“ setzt die Schätzung.
export function parseQuickAdd(raw: string): { title: string; minutes: number } {
  const t = raw.trim()
  const lead = t.match(/^(\d{1,3})\s*m(?:in)?\s+(.+)$/i)
  if (lead) return { title: lead[2]!, minutes: Number(lead[1]) }
  const trail = t.match(/^(.+?)\s+(\d{1,3})\s*m(?:in)?$/i)
  return trail ? { title: trail[1]!, minutes: Number(trail[2]) } : { title: t, minutes: 0 }
}
