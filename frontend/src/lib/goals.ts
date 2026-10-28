import type { Goal } from '@/api/client'
import { t, tn } from './i18n.ts'

// „3 von 10 Std.“ usw.; leer bei Zielen ohne Fortschritt.
export function goalProgressText({ kind, progress: p }: Pick<Goal, 'kind' | 'progress'>) {
  if (!p) return ''
  const vars = { n: p.current, m: p.target }
  if (kind === 'PROJECT_HOURS') return t('{n} von {m} Std.', vars)
  if (kind === 'HABIT_COUNT') return t('{n} von {m}-mal', vars)
  if (kind === 'NONE') return t('{n} von {m} Teilzielen erreicht', vars)
  return t('{n} von {m} Aufgaben', vars)
}

export type PaceState = 'ok' | 'tight' | 'off'
const dayNo = (d: string) => Date.parse(d + 'T00:00:00Z') / 864e5

// Soll-Stand heute: linear von start_date bis due_date. Bis 10 Prozentpunkte darunter ist „knapp“.
// null ohne Frist oder Fortschritt; erreichte 100 % sind immer im Plan.
export function goalPace(g: Pick<Goal, 'kind' | 'start_date' | 'due_date' | 'progress'>, today: string) {
  const p = g.progress
  if (!p || !g.due_date || g.due_date <= g.start_date) return null
  const span = dayNo(g.due_date) - dayNo(g.start_date)
  const share = Math.min(1, Math.max(0, (dayNo(today) - dayNo(g.start_date)) / span))
  const pace = Math.round(share * 100)
  const daysLeft = Math.max(0, dayNo(g.due_date) - dayNo(today))
  const state: PaceState = p.percent >= 100 || p.percent >= pace ? 'ok' : p.percent >= pace - 10 && daysLeft > 0 ? 'tight' : 'off'
  // Vorsprung oder nötiges Tempo in der Einheit des Ziels.
  const amount = (n: number) =>
    g.kind === 'NONE' ? tn(n, '{n} Teilziel', '{n} Teilziele')
      : g.kind === 'PROJECT_TASKS' ? tn(n, '{n} Aufgabe', '{n} Aufgaben')
        : g.kind === 'PROJECT_HOURS' ? t('{n} Std.', { n }) : t('{n}-mal', { n })
  const ahead = Math.floor(p.current - (pace / 100) * p.target)
  let hint: string
  if (p.percent >= 100) hint = t('Erreicht')
  else if (!daysLeft) hint = t('Frist vorbei')
  else if (state === 'ok') hint = ahead > 0 ? t('{n} voraus', { n: amount(ahead) }) : t('Genau im Plan')
  else hint = t('{n} pro Woche nötig', { n: amount(Math.ceil((p.target - p.current) / Math.max(1, daysLeft / 7))) })
  return { pace, state, daysLeft, hint }
}
