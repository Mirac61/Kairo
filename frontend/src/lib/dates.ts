import { locale, t } from './i18n.ts'
import { prefs } from './prefs.ts'
import { ruleLabel } from './quickAdd.ts'

const MILLISECONDS_PER_DAY = 24 * 60 * 60 * 1000

function padTwo(num: number): string {
  return String(num).padStart(2, '0')
}

// Formatiert ein Datum nach YYYY-MM-DD
export function ymd(date: Date): string {
  const year = date.getFullYear()
  const month = padTwo(date.getMonth() + 1)
  const day = padTwo(date.getDate())
  return `${year}-${month}-${day}`
}

// Formatiert die Uhrzeit nach HH:MM
export function hhmm(date: Date): string {
  const hours = padTwo(date.getHours())
  const minutes = padTwo(date.getMinutes())
  return `${hours}:${minutes}`
}

// Addiert n Tage auf ein Date-Objekt (lokale Zeit)
export function addDays(date: Date, daysToAdd: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + daysToAdd)
}

// Verschiebt ein lokales "YYYY-MM-DD" als reines Kalenderdatum ohne Zeitzonen-/DST-Versatz
export function shiftDate(dayString: string, daysToAdd: number): string {
  const [yearStr, monthStr, dateStr] = dayString.split('-')
  const year = Number(yearStr)
  const month = Number(monthStr)
  const date = Number(dateStr)

  const utcDate = new Date(Date.UTC(year, month - 1, date + daysToAdd))
  return utcDate.toISOString().slice(0, 10)
}

// Ermittelt den Wochenstart für ein Datum um 00:00 Uhr Ortszeit.
// Berücksichtigt prefs.weekStart (0 = Sonntag, 1 = Montag).
export function weekStart(date = new Date(), startDay = prefs.weekStart): Date {
  const configuredStart = startDay ?? 1
  const currentDay = date.getDay()
  const offset = (currentDay - configuredStart + 7) % 7
  return addDays(date, -offset)
}

// Formatiert relative Tage, z. B. "gestern", "vor 3 Tagen" relativ zu today ("YYYY-MM-DD")
export function daysAgo(day: string, today: string): string {
  const diffDays = Math.round((Date.parse(today) - Date.parse(day)) / MILLISECONDS_PER_DAY)
  const formatter = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  return formatter.format(-diffDays, 'day')
}

// Konvertiert "YYYY-MM-DD" in ein lokales Date um 00:00:00 Uhr
export function dateOf(dateString: string | null): Date | null {
  if (!dateString) {
    return null
  }
  return new Date(`${dateString}T00:00:00`)
}

// Konvertiert "HH:MM" in ein heutiges Date-Objekt mit dieser Uhrzeit
export function timeOf(timeString: string | null): Date | null {
  if (!timeString) {
    return null
  }
  const [hoursStr = '0', minutesStr = '0'] = timeString.split(':')
  const hours = Number(hoursStr)
  const minutes = Number(minutesStr)

  const date = new Date()
  date.setHours(hours, minutes, 0, 0)
  return date
}

// Hilfsfunktion: Prüft, ob ein Wert ein Date-Objekt ist
export function single(value: unknown): Date | null {
  return value instanceof Date ? value : null
}

// Formatiert Minuten als h:mm (ohne Einheit, spaltenbündig)
export function hm(minutes: number): string {
  const hours = Math.floor(minutes / 60)
  const mins = padTwo(minutes % 60)
  return `${hours}:${mins}`
}

// Formatiert Minuten als lesbaren Text: z. B. "45 Min", "1 Std 30 Min"
export function dur(totalMinutes = 0): string {
  const hoursUnit = t('Std')
  const minutesUnit = t('Min')

  if (totalMinutes >= 60) {
    const hours = Math.floor(totalMinutes / 60)
    const remainingMinutes = totalMinutes % 60
    const minuteSuffix = remainingMinutes ? ` ${remainingMinutes} ${minutesUnit}` : ''
    return `${hours} ${hoursUnit}${minuteSuffix}`
  }

  return `${totalMinutes} ${minutesUnit}`
}

// Berechnet die Dauer eines Zeiteintrags in vollen Minuten.
// Ein noch laufender Eintrag (ended_at = null) wird bis `now` gezählt.
export function entryMinutes(
  entry: { started_at: string; ended_at: string | null },
  now = Date.now(),
): number {
  const startTime = Date.parse(entry.started_at)
  const endTime = entry.ended_at ? Date.parse(entry.ended_at) : now
  const durationMs = endTime - startTime
  return Math.max(0, Math.floor(durationMs / 60_000))
}

// Formatiert Tag und Monat, z. B. "12.10." (deutsch) oder "12/10" (englisch)
export function dm(dayString: string): string {
  const dayNumber = Number(dayString.slice(8))
  const monthNumber = Number(dayString.slice(5, 7))

  if (locale.value === 'de-DE') {
    return `${dayNumber}.${monthNumber}.`
  }
  return `${dayNumber}/${monthNumber}`
}

// Wochentagskürzel mit Tag/Monat, z. B. "Fr 9.10."
export function shortDay(dayString: string): string {
  const date = new Date(`${dayString}T00:00:00`)
  const weekdayShort = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
    .format(date)
    .replace('.', '')

  return `${weekdayShort} ${dm(dayString)}`
}

// "Heute", "Morgen", ansonsten "Fr 9.10."
export function dayLabel(dayString: string, todayString: string): string {
  const diffDays = Math.round(
    (Date.parse(dayString) - Date.parse(todayString)) / MILLISECONDS_PER_DAY,
  )

  if (diffDays === 0) return t('Heute')
  if (diffDays === 1) return t('Morgen')
  return shortDay(dayString)
}

// "heute", "morgen" oder der volle Wochentag (z. B. "Freitag") für Zeitvorschläge
export function dayWord(dayString: string, todayString: string): string {
  const diffDays = Math.round(
    (Date.parse(dayString) - Date.parse(todayString)) / MILLISECONDS_PER_DAY,
  )

  if (diffDays === 0) return t('heute')
  if (diffDays === 1) return t('morgen')

  const date = new Date(`${dayString}T00:00:00`)
  return new Intl.DateTimeFormat(locale.value, { weekday: 'long' }).format(date)
}

// Lesbare Wiederholungsregel einer Aufgabe, z. B. „Jeden Freitag“
export function ruleText(rule: string): string {
  const labelInfo = ruleLabel(rule)
  if (labelInfo.day) {
    return t(labelInfo.key, { day: t(labelInfo.day) })
  }
  return t(labelInfo.key)
}
