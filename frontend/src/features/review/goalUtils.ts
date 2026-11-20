import type { Goal } from '@/api/client'
import { locale, t } from '@/lib/i18n'

export function isVideoPath(path: string | null | undefined): boolean {
  if (!path) return false
  return /\.(mp4|webm|mov|m4v)$/i.test(path)
}

export function goalPercent(g: Goal, steps: Goal[]): number {
  if (g.progress) return Math.min(100, Math.max(0, Math.round(g.progress.percent)))
  if (steps.length) {
    const done = steps.filter((s) => s.archived).length
    return Math.round((done / steps.length) * 100)
  }
  return g.archived ? 100 : 0
}

// DST-sichere Tagesdifferenz über UTC-Datum (vermeidet +/-1-Tag-Fehler bei Zeitumstellung).
export function daysUntilYmd(dueYmd: string, now = new Date()): number {
  const [y, m, d] = dueYmd.split('-').map(Number)
  if (!y || !m || !d) return 0
  const dueUtc = Date.UTC(y, m - 1, d)
  const todayUtc = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((dueUtc - todayUtc) / 864e5)
}

export function formatDeadline(dueYmd: string | null | undefined): string {
  if (!dueYmd) return t('Ohne Frist')
  const dt = new Date(`${dueYmd}T00:00:00`)
  const fmt = new Intl.DateTimeFormat(locale.value, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(dt)
  const days = daysUntilYmd(dueYmd)
  if (days < 0) return `${fmt} · ${t('überfällig')}`
  if (days === 0) return `${fmt} · ${t('Heute')}`
  return `${fmt} · ${days}d`
}
