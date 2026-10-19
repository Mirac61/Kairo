// Rechenteile der Startseite ohne Vue, damit test/dayPlan.test.ts sie direkt mit Node prüfen kann.

// Minuten seit Mitternacht in der Zeitzone tz (undefined = Browser).
export function minutesOfDay(ms: number, tz?: string) {
  const p = new Intl.DateTimeFormat('de-DE', { hour: 'numeric', minute: 'numeric', hourCycle: 'h23', timeZone: tz }).formatToParts(new Date(ms))
  const n = (t: string) => Number(p.find((x) => x.type === t)?.value ?? 0)
  return n('hour') * 60 + n('minute')
}

// Sekunden als H:MM:SS.
export function stopwatch(secs: number) {
  const s = Math.floor(secs)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${Math.floor(s / 3600)}:${p(Math.floor(s / 60) % 60)}:${p(s % 60)}`
}

// Abgeschlossene Zeit einer Task in Sekunden.
export const doneSeconds = (entries: { task_id?: string | null; started_at: string; ended_at?: string | null }[], id: string) =>
  entries.reduce((s, e) => (e.task_id === id && e.ended_at ? s + (Date.parse(e.ended_at) - Date.parse(e.started_at)) / 1000 : s), 0)

// Überlappende Blöcke stehen nebeneinander: Spalte und Spaltenzahl je Block.
// blocks sind nach Beginn sortiert; kürzere Blöcke zählen mit minLen (so hoch zeichnet sie das Raster).
export function columns(blocks: { key: string; start: number; end: number }[], minLen: number) {
  const pos = new Map<string, { col: number; cols: number }>()
  let cluster: { key: string; col: number }[] = []
  let ends: number[] = [] // Ende des letzten Blocks je Spalte
  const flush = () => {
    for (const c of cluster) pos.set(c.key, { col: c.col, cols: ends.length })
    cluster = []
    ends = []
  }
  for (const b of blocks) {
    if (ends.length && b.start >= Math.max(...ends)) flush() // nichts überlappt mehr
    let col = ends.findIndex((e) => e <= b.start)
    if (col < 0) col = ends.length
    ends[col] = Math.max(b.end, b.start + minLen)
    cluster.push({ key: b.key, col })
  }
  flush()
  return pos
}

// Erster freier Platz für `minutes` ab `from` im Fenster [winStart, winEnd] (Minuten seit Mitternacht), auf 5 Minuten aufgerundet.
// busy ist nach Beginn sortiert. prev/next sind die Blöcke direkt davor und danach; null = kein freier Platz.
export function findSlot<T extends { start: number; end: number }>(busy: T[], from: number, winStart: number, winEnd: number, minutes: number) {
  let at = Math.ceil(Math.max(from, winStart) / 5) * 5
  for (const b of busy) {
    if (b.end <= at) continue
    if (b.start - at >= minutes) break
    at = Math.ceil(Math.max(at, b.end) / 5) * 5
  }
  if (at + minutes > winEnd) return null
  const prev = busy.filter((b) => b.end <= at).sort((a, b) => b.end - a.end)[0]
  return { start: at, prev, next: busy.find((b) => b.start >= at + minutes) }
}
