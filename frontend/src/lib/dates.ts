const pad = (n: number) => String(n).padStart(2, '0')

export const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
export const hhmm = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`

// "gestern", "vor 3 Tagen" für einen Tag vor today (beide "YYYY-MM-DD").
export const daysAgo = (day: string, today: string) =>
  new Intl.RelativeTimeFormat('de', { numeric: 'auto' }).format(-Math.round((Date.parse(today) - Date.parse(day)) / 864e5), 'day')

// DatePicker rechnet mit Date; das Backend mit "YYYY-MM-DD" bzw. "HH:mm".
export const dateOf = (s: string | null) => (s ? new Date(`${s}T00:00:00`) : null)
export const timeOf = (s: string | null) => {
  if (!s) return null
  const [h = 0, m = 0] = s.split(':').map(Number)
  const d = new Date()
  d.setHours(h, m, 0, 0)
  return d
}
// Eventwert des DatePickers (Einzelwert) als Date oder null.
export const single = (v: unknown) => (v instanceof Date ? v : null)
