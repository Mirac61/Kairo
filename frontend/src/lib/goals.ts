import type { Goal } from '@/api/client'
import { t } from './i18n.ts'

// „3 von 10 Std.“ usw.; leer bei Zielen ohne Fortschritt.
export function goalProgressText({ kind, progress: p }: Pick<Goal, 'kind' | 'progress'>) {
  if (!p) return ''
  const vars = { n: p.current, m: p.target }
  if (kind === 'PROJECT_HOURS') return t('{n} von {m} Std.', vars)
  if (kind === 'HABIT_COUNT') return t('{n} von {m}-mal', vars)
  return t('{n} von {m} Aufgaben', vars)
}
