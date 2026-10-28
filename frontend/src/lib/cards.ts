import type { Card, CardLog } from '@/api/client'
import { addDays, ymd } from './dates.ts'
import { t } from './i18n.ts'

// Ab diesem Abstand (Tage) „sitzt“ eine Karte; darunter ist sie in Arbeit (wie „mature“ bei Anki, nur kürzer).
export const SETTLED_DAYS = 7

export type Stage = 'new' | 'learn' | 'young' | 'settled'
export const stageOf = (c: Card): Stage =>
  c.new ? 'new' : c.learning ? 'learn' : c.interval_days >= SETTLED_DAYS ? 'settled' : 'young'

// Eine Karte pro Zeile: „Frage :: Antwort“ oder durch Tab getrennt (Export aus Anki/Quizlet). Leere Zeilen zählen nicht.
export function parseCardList(text: string) {
  const cards: { question: string; answer: string }[] = []
  let skipped = 0
  for (const line of text.split('\n')) {
    const s = line.trim()
    if (!s) continue
    const [q, a] = s.includes(' :: ') ? s.split(/ :: (.*)/s) : s.split(/\t(.*)/s)
    if (q?.trim() && a?.trim())
      cards.push({ question: q.trim().replace(/^[-*+]\s+/, ''), answer: a.trim() })
    else skipped++
  }
  return { cards, skipped }
}

// Fällige Karten je Tag ab heute; heute zählt alles, was schon dran ist (auch neue und überfällige).
export function forecast(cards: Card[], today = new Date(), days = 7): number[] {
  const keys = Array.from({ length: days }, (_, i) => ymd(addDays(today, i)))
  return keys.map((k, i) => cards.filter((c) => (i === 0 ? c.due : c.due_date === k)).length)
}

// Bewertungen je Tag (Ortszeit, YYYY-MM-DD).
export function perDay(log: CardLog[]): Map<string, number> {
  const m = new Map<string, number>()
  for (const l of log) {
    const d = ymd(new Date(l.at))
    m.set(d, (m.get(d) ?? 0) + 1)
  }
  return m
}

// Tage in Folge mit mindestens einer Bewertung; ist heute noch nichts gelernt, zählt die Serie bis gestern.
export function streak(days: Map<string, number>, today = new Date()): number {
  let d = days.has(ymd(today)) ? today : addDays(today, -1)
  let n = 0
  while (days.has(ymd(d))) {
    n++
    d = addDays(d, -1)
  }
  return n
}

// Anteil der Bewertungen besser als „Nochmal“ (0–100), null ohne Bewertungen.
export function accuracy(log: CardLog[]): number | null {
  return log.length ? Math.round((log.filter((l) => l.grade > 0).length / log.length) * 100) : null
}

// Wie oft jede Karte mit „Nochmal“ bewertet wurde.
export function lapses(log: CardLog[]): Map<string, number> {
  const m = new Map<string, number>()
  for (const l of log) if (l.grade === 0) m.set(l.key, (m.get(l.key) ?? 0) + 1)
  return m
}

// Abstand für die Knöpfe und Listen: 0 heißt „gleich nochmal in dieser Runde“.
export function intervalText(days: number): string {
  if (days <= 0) return t('gleich')
  if (days < 30) return t('{n} T', { n: days })
  if (days < 365) return t('{n} Mon.', { n: Math.round(days / 30) })
  return t('{n} J.', { n: Math.round((days / 365) * 10) / 10 })
}

// Fisher-Yates; für Übungsrunden ohne Wertung.
export function shuffled<T>(xs: T[]): T[] {
  const a = [...xs]
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[a[i], a[j]] = [a[j]!, a[i]!]
  }
  return a
}
