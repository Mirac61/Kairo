// Schnelleingabe: „Sport 30m @morgen @14:30 #Kairo !hoch“. Jedes Token zählt nur, wenn es ganz passt; sonst bleibt es im Titel.
//   Dauer:    30m · 30 min · 1h · 1,5h · 1h30        Tag:  @heute · @morgen · @übermorgen · @mo … @sonntag · @12.10.
//   Uhrzeit:  @14:30 (ohne Tag: heute)               Projekt: #name (ganzer Name oder Anfang, Groß/Klein egal)
//   Priorität: !dringend · !hoch · !mittel · !niedrig
// Englisch geht immer mit: @today · @tomorrow · @mon … @sunday · !urgent · !high · !medium · !low.
// Reine Funktion ohne Imports, damit quickAdd.test.ts sie direkt mit Node prüfen kann.
export interface QuickProject { id: string; name: string }
export interface Quick {
  title: string
  minutes: number // 0 = keine Schätzung
  date: string | null // YYYY-MM-DD
  time: string | null // HH:MM
  project: QuickProject | null
  priority: 'URGENT' | 'HIGH' | 'MEDIUM' | 'LOW' | null
}

const PRIO = {
  dringend: 'URGENT', hoch: 'HIGH', mittel: 'MEDIUM', niedrig: 'LOW',
  urgent: 'URGENT', high: 'HIGH', medium: 'MEDIUM', low: 'LOW',
} as const
// Index = Date.getUTCDay(); ab zwei Buchstaben reicht der Anfang (@mo, @do, @tue). Kein Anfang passt auf zwei Tage.
const WEEKDAYS = [
  ['sonntag', 'sunday'], ['montag', 'monday'], ['dienstag', 'tuesday'], ['mittwoch', 'wednesday'],
  ['donnerstag', 'thursday'], ['freitag', 'friday'], ['samstag', 'saturday'],
]
const MAX_MINUTES = 24 * 60
const pad = (n: number) => String(n).padStart(2, '0')
const norm = (s: string) => s.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '')
const at = (day: string) => new Date(`${day}T12:00:00Z`)
const addDays = (day: string, n: number) => { const d = at(day); d.setUTCDate(d.getUTCDate() + n); return d.toISOString().slice(0, 10) }

// Tag aus dem Teil nach „@“, null wenn es kein Tag ist. Ein Wochentag meint den nächsten (nie heute).
function dayOf(word: string, today: string): string | null {
  if (word === 'heute' || word === 'today') return today
  if (word === 'morgen' || word === 'tomorrow') return addDays(today, 1)
  if (word === 'übermorgen' || word === 'uebermorgen') return addDays(today, 2)
  const wd = word.length < 2 ? -1 : WEEKDAYS.findIndex((names) => names.some((n) => n.startsWith(word)))
  if (wd >= 0) return addDays(today, ((wd - at(today).getUTCDay() + 6) % 7) + 1)
  const m = word.match(/^(\d{1,2})\.(\d{1,2})\.?$/)
  if (!m) return null
  let day = `${today.slice(0, 4)}-${pad(Number(m[2]))}-${pad(Number(m[1]))}`
  if (day < today) day = `${Number(today.slice(0, 4)) + 1}${day.slice(4)}`
  return !Number.isNaN(at(day).getTime()) && at(day).toISOString().slice(0, 10) === day ? day : null
}

function minutesOf(token: string, unit?: string): number | null {
  const [, num, u] = unit ? [0, token, unit] : (token.match(/^(\d+(?:[.,]\d+)?)(h|std|min|m)$/i) ?? [])
  if (num && u && /^\d+(?:[.,]\d+)?$/.test(num)) {
    const v = Number(num.replace(',', '.'))
    const min = Math.round(/^(h|std)$/i.test(u) ? v * 60 : v)
    return min > 0 && min <= MAX_MINUTES ? min : null
  }
  const hm = unit ? null : token.match(/^(\d{1,2})h(\d{1,2})(?:m|min)?$/i)
  return hm && Number(hm[2]) < 60 ? Number(hm[1]) * 60 + Number(hm[2]) : null
}

export function parseQuickAdd(raw: string, projects: QuickProject[] = [], today = new Date().toLocaleDateString('sv')): Quick {
  const q: Quick = { title: '', minutes: 0, date: null, time: null, project: null, priority: null }
  const words = raw.trim().split(/\s+/).filter(Boolean)
  const rest: string[] = []
  for (let i = 0; i < words.length; i++) {
    const w = words[i]!
    const body = w.slice(1).toLowerCase()
    if (!q.minutes) {
      // „30 min Sport“: Zahl und Einheit als zwei Wörter
      const two = /^\d+(?:[.,]\d+)?$/.test(w) && words[i + 1] && /^(h|std|min|m)$/i.test(words[i + 1]!) ? minutesOf(w, words[i + 1]) : null
      const one = two ?? minutesOf(w)
      if (one) { q.minutes = one; if (two) i++; continue }
    }
    if (w[0] === '@' && !q.date && dayOf(body, today)) { q.date = dayOf(body, today); continue }
    const t = w[0] === '@' && !q.time ? body.match(/^(\d{1,2}):(\d{2})$/) : null
    if (t && Number(t[1]) < 24 && Number(t[2]) < 60) { q.time = `${pad(Number(t[1]))}:${t[2]}`; continue }
    if (w[0] === '#' && !q.project && norm(body)) {
      const n = norm(body)
      q.project = projects.find((p) => norm(p.name) === n) ?? projects.find((p) => norm(p.name).startsWith(n)) ?? null
      if (q.project) continue
    }
    if (w[0] === '!' && !q.priority && body in PRIO) { q.priority = PRIO[body as keyof typeof PRIO]; continue }
    rest.push(w)
  }
  q.title = rest.join(' ')
  if (q.time && !q.date) q.date = today
  return q
}

// Felder für createTask. Mit Tag (oder Uhrzeit) ist die Task geplant; fallbackDate gilt, wenn die Eingabe keinen Tag nennt.
export function taskBody(q: Quick, fallbackDate?: string) {
  const date = q.date ?? fallbackDate
  return {
    estimated_minutes: q.minutes,
    ...(q.priority ? { priority: q.priority } : {}),
    ...(q.project ? { project_id: q.project.id } : {}),
    ...(date ? { planned_date: date, status: 'PLANNED' as const } : {}),
    ...(date && q.time ? { planned_start_at: new Date(`${date}T${q.time}:00`).toISOString() } : {}),
  }
}
