// Schnelleingabe: „Sport 30m @morgen @14:30 #Kairo !hoch“.
// Jedes Token zählt nur, wenn es exakt passt; ansonsten verbleibt es im Titel.
//
// Syntax:
//   Dauer:        30m · 30 min · 1h · 1,5h · 1h30
//   Tag:          @heute · @morgen · @übermorgen · @mo … @sonntag · @12.10.
//   Uhrzeit:      @14:30 (ohne Tag: heute)
//   Projekt:      #name (ganzer Name oder Präfix, Groß-/Kleinschreibung irrelevant)
//   Priorität:    !dringend · !hoch · !mittel · !niedrig
//   Wiederholung: @täglich · @werktags · @wöchentlich · @monatlich · @jährlich · @jeden freitag · @jeden monat
//                 (ohne Tag: erster passender ab heute; monatlich/jährlich am Tag des ersten Termins)
//
// Englisch wird ebenfalls unterstützt:
//   @today · @tomorrow · @mon … @sunday · !urgent · !high · !medium · !low.
//
// Hinweis: Reine Funktion ohne externe Imports, damit quickAdd.test.ts direkt in Node ausgeführt werden kann.

export interface QuickProject {
  id: string
  name: string
}

export interface Quick {
  title: string
  minutes: number // 0 = keine Schätzung
  date: string | null // Format: YYYY-MM-DD
  time: string | null // Format: HH:MM
  project: QuickProject | null
  priority: 'URGENT' | 'HIGH' | 'MEDIUM' | 'LOW' | null
  rule: string | null // RRULE wie beim Backend, z. B. FREQ=WEEKLY;BYDAY=FR
}

export type TokenKind = '' | 'dur' | 'date' | 'project' | 'prio' | 'bad'

export interface QuickToken {
  w: string
  kind: TokenKind
}

const PRIORITY_MAP = {
  dringend: 'URGENT',
  hoch: 'HIGH',
  mittel: 'MEDIUM',
  niedrig: 'LOW',
  urgent: 'URGENT',
  high: 'HIGH',
  medium: 'MEDIUM',
  low: 'LOW',
} as const

// Index entspricht Date.getUTCDay() (0 = Sonntag, 1 = Montag, ...).
// Ab zwei Buchstaben reicht der Wortanfang (@mo, @do, @tue). Kein Wochentagsanfang kollidiert.
const WEEKDAYS: [string, string][] = [
  ['sonntag', 'sunday'],
  ['montag', 'monday'],
  ['dienstag', 'tuesday'],
  ['mittwoch', 'wednesday'],
  ['donnerstag', 'thursday'],
  ['freitag', 'friday'],
  ['samstag', 'saturday'],
]

const RRULE_DAYS = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
const WORKDAYS_RULE = 'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR'

const SIMPLE_RULES: Record<string, string> = {
  täglich: 'FREQ=DAILY',
  daily: 'FREQ=DAILY',
  werktags: WORKDAYS_RULE,
  weekdays: WORKDAYS_RULE,
  wöchentlich: 'FREQ=WEEKLY',
  weekly: 'FREQ=WEEKLY',
  monatlich: 'FREQ=MONTHLY',
  monthly: 'FREQ=MONTHLY',
  jährlich: 'FREQ=YEARLY',
  yearly: 'FREQ=YEARLY',
}

const MAX_MINUTES = 24 * 60

function padTwo(num: number): string {
  return String(num).padStart(2, '0')
}

function normalizeSearch(str: string): string {
  return str.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '')
}

function parseUtcDate(dateString: string): Date {
  return new Date(`${dateString}T12:00:00Z`)
}

function addDaysToDateString(dateString: string, daysToAdd: number): string {
  const date = parseUtcDate(dateString)
  date.setUTCDate(date.getUTCDate() + daysToAdd)
  return date.toISOString().slice(0, 10)
}

// Parst den Tag nach einem „@“-Token. Gibt null zurück, wenn kein gültiger Tag erkannt wird.
// Ein genannter Wochentag meint immer den nächsten passenden Wochentag (nie heute).
function parseDayWord(word: string, today: string): string | null {
  if (word === 'heute' || word === 'today') {
    return today
  }
  if (word === 'morgen' || word === 'tomorrow') {
    return addDaysToDateString(today, 1)
  }
  if (word === 'übermorgen' || word === 'uebermorgen') {
    return addDaysToDateString(today, 2)
  }

  // Wochentag ab 2 Zeichen prüfen
  if (word.length >= 2) {
    const weekdayIndex = WEEKDAYS.findIndex((names) => names.some((name) => name.startsWith(word)))
    if (weekdayIndex >= 0) {
      const todayWeekday = parseUtcDate(today).getUTCDay()
      const daysUntilNext = ((weekdayIndex - todayWeekday + 6) % 7) + 1
      return addDaysToDateString(today, daysUntilNext)
    }
  }

  // Datumsformat wie "12.10." oder "5.10"
  const dateMatch = word.match(/^(\d{1,2})\.(\d{1,2})\.?$/)
  if (!dateMatch) {
    return null
  }

  const currentYear = Number(today.slice(0, 4))
  const day = padTwo(Number(dateMatch[1]))
  const month = padTwo(Number(dateMatch[2]))

  let candidateDate = `${currentYear}-${month}-${day}`
  // Falls das Datum dieses Jahr bereits in der Vergangenheit liegt, für das nächste Jahr ansetzen
  if (candidateDate < today) {
    candidateDate = `${currentYear + 1}-${month}-${day}`
  }

  const parsed = parseUtcDate(candidateDate)
  const isValidDate =
    !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === candidateDate

  return isValidDate ? candidateDate : null
}

// Ermittelt die RRULE nach „@jeden“ / „@every“.
// Erlaubt: tag, woche, monat, jahr oder einen Wochentag.
function parseEveryRule(word: string): string | null {
  if (['tag', 'day'].includes(word)) return 'FREQ=DAILY'
  if (['woche', 'week'].includes(word)) return 'FREQ=WEEKLY'
  if (['monat', 'month'].includes(word)) return 'FREQ=MONTHLY'
  if (['jahr', 'year'].includes(word)) return 'FREQ=YEARLY'

  if (word.length >= 2) {
    const weekdayIndex = WEEKDAYS.findIndex((names) => names.some((name) => name.startsWith(word)))
    if (weekdayIndex >= 0) {
      return `FREQ=WEEKLY;BYDAY=${RRULE_DAYS[weekdayIndex]}`
    }
  }

  return null
}

// Erster Tag ab heute (inklusive), an dem die gegebene Regel greift.
function findFirstOccurrence(rule: string, today: string): string {
  const byDayMatch = rule.match(/BYDAY=([A-Z,]+)/)
  if (!byDayMatch || !byDayMatch[1]) {
    return today
  }

  const allowedDays = byDayMatch[1].split(',')
  for (let offset = 0; offset < 7; offset++) {
    const candidateDay = addDaysToDateString(today, offset)
    const weekday = RRULE_DAYS[parseUtcDate(candidateDay).getUTCDay()]!
    if (allowedDays.includes(weekday)) {
      return candidateDay
    }
  }

  return today
}

// Lesbare Voreinstellungen für Wiederholungsregeln
export const RULE_PRESETS: [string, string][] = [
  ['FREQ=DAILY', 'Täglich'],
  [WORKDAYS_RULE, 'Werktags'],
  ['FREQ=WEEKLY', 'Wöchentlich'],
  ['FREQ=WEEKLY;INTERVAL=2', 'Alle 2 Wochen'],
  ['FREQ=MONTHLY', 'Monatlich'],
  ['FREQ=MONTHLY;BYMONTHDAY=-1', 'Am Monatsletzten'],
  ['FREQ=YEARLY', 'Jährlich'],
]

const GERMAN_DAY_NAMES = [
  'Sonntag',
  'Montag',
  'Dienstag',
  'Mittwoch',
  'Donnerstag',
  'Freitag',
  'Samstag',
]

export function ruleLabel(rule: string): { key: string; day?: string } {
  const preset = RULE_PRESETS.find(([presetRule]) => presetRule === rule)
  if (preset) {
    return { key: preset[1] }
  }

  const weeklyMatch = rule.match(/^FREQ=WEEKLY;BYDAY=([A-Z]{2})$/)
  if (weeklyMatch && weeklyMatch[1]) {
    const weekdayIndex = RRULE_DAYS.indexOf(weeklyMatch[1])
    if (weekdayIndex >= 0) {
      return { key: 'Jeden {day}', day: GERMAN_DAY_NAMES[weekdayIndex] }
    }
  }

  return { key: rule }
}

function parseMinutes(token: string, explicitUnit?: string): number | null {
  let numStr = token
  let unitStr = explicitUnit

  if (!explicitUnit) {
    const match = token.match(/^(\d+(?:[.,]\d+)?)(h|std|min|m)$/i)
    if (match) {
      numStr = match[1]!
      unitStr = match[2]!
    } else {
      // Format wie "1h30" oder "1h30m"
      const hmMatch = token.match(/^(\d{1,2})h(\d{1,2})(?:m|min)?$/i)
      if (hmMatch && Number(hmMatch[2]) < 60) {
        return Number(hmMatch[1]) * 60 + Number(hmMatch[2])
      }
      return null
    }
  }

  if (unitStr && /^\d+(?:[.,]\d+)?$/.test(numStr)) {
    const numericValue = Number(numStr.replace(',', '.'))
    const totalMinutes = Math.round(/^(h|std)$/i.test(unitStr) ? numericValue * 60 : numericValue)
    if (totalMinutes > 0 && totalMinutes <= MAX_MINUTES) {
      return totalMinutes
    }
  }

  return null
}

function scan(raw: string, projects: QuickProject[], today: string) {
  const result: Quick = {
    title: '',
    minutes: 0,
    date: null,
    time: null,
    project: null,
    priority: null,
    rule: null,
  }

  const words = raw.trim().split(/\s+/).filter(Boolean)
  const kinds: TokenKind[] = []
  const remainingTitleWords: string[] = []

  for (let index = 0; index < words.length; index++) {
    const word = words[index]!
    const body = word.slice(1).toLowerCase()

    // 1. Dauereingabe (z. B. "45m" oder "30 min")
    if (!result.minutes) {
      const nextWord = words[index + 1]
      const isTwoWordDuration =
        /^\d+(?:[.,]\d+)?$/.test(word) && Boolean(nextWord && /^(h|std|min|m)$/i.test(nextWord))

      const parsedMinutes = isTwoWordDuration ? parseMinutes(word, nextWord) : parseMinutes(word)

      if (parsedMinutes) {
        result.minutes = parsedMinutes
        kinds[index] = 'dur'
        if (isTwoWordDuration) {
          index++
          kinds[index] = 'dur'
        }
        continue
      }
    }

    // 2. Wiederholungsregel direkt nach @ (z. B. @täglich, @monatlich)
    if (word.startsWith('@') && !result.rule && Object.hasOwn(SIMPLE_RULES, body)) {
      result.rule = SIMPLE_RULES[body]!
      kinds[index] = 'date'
      continue
    }

    // 3. Mehrteilige Wiederholung (z. B. "@jeden freitag", "@every week")
    if (
      word.startsWith('@') &&
      !result.rule &&
      ['jeden', 'jede', 'jedes', 'every'].includes(body) &&
      words[index + 1]
    ) {
      const everyRule = parseEveryRule(words[index + 1]!.toLowerCase())
      if (everyRule) {
        result.rule = everyRule
        kinds[index] = 'date'
        index++
        kinds[index] = 'date'
        continue
      }
    }

    // 4. Tag nach @ (z. B. @morgen, @12.10., @fr)
    if (word.startsWith('@') && !result.date) {
      const parsedDay = parseDayWord(body, today)
      if (parsedDay) {
        result.date = parsedDay
        kinds[index] = 'date'
        continue
      }
    }

    // 5. Uhrzeit nach @ (z. B. @14:30)
    if (word.startsWith('@') && !result.time) {
      const timeMatch = body.match(/^(\d{1,2}):(\d{2})$/)
      if (timeMatch) {
        const hours = Number(timeMatch[1])
        const minutes = Number(timeMatch[2])
        if (hours < 24 && minutes < 60) {
          result.time = `${padTwo(hours)}:${timeMatch[2]}`
          kinds[index] = 'date'
          continue
        }
      }
    }

    // 6. Projekt nach # (z. B. #kairo, #haushalt)
    if (word.startsWith('#') && !result.project && normalizeSearch(body)) {
      const normalizedQuery = normalizeSearch(body)
      const matchedProject =
        projects.find((p) => normalizeSearch(p.name) === normalizedQuery) ??
        projects.find((p) => normalizeSearch(p.name).startsWith(normalizedQuery)) ??
        null

      if (matchedProject) {
        result.project = matchedProject
        kinds[index] = 'project'
        continue
      }
    }

    // 7. Priorität nach ! (z. B. !hoch, !dringend)
    if (word.startsWith('!') && !result.priority && body in PRIORITY_MAP) {
      result.priority = PRIORITY_MAP[body as keyof typeof PRIORITY_MAP]
      kinds[index] = 'prio'
      continue
    }

    // Fallback: Ist es ein ungültiges Token-Präfix oder normaler Titeltext?
    const isMalformedToken = /^[#@!]./.test(word)
    kinds[index] = isMalformedToken ? 'bad' : ''
    remainingTitleWords.push(word)
  }

  result.title = remainingTitleWords.join(' ')

  if (result.rule && !result.date) {
    result.date = findFirstOccurrence(result.rule, today)
  }
  if (result.time && !result.date) {
    result.date = today
  }

  const tokens: QuickToken[] = words.map((w, i) => ({
    w,
    kind: kinds[i] ?? '',
  }))

  return { q: result, tokens }
}

export function parseQuickAdd(
  raw: string,
  projects: QuickProject[] = [],
  today = new Date().toLocaleDateString('sv'),
): Quick {
  return scan(raw, projects, today).q
}

export function quickTokens(
  raw: string,
  projects: QuickProject[] = [],
  today = new Date().toLocaleDateString('sv'),
): QuickToken[] {
  return scan(raw, projects, today).tokens
}

// Erstellt den Request-Body für createTask.
// Mit Tag oder Uhrzeit wird die Aufgabe direkt als PLANNED markiert.
export function taskBody(q: Quick, fallbackDate?: string) {
  const date = q.date ?? fallbackDate
  return {
    estimated_minutes: q.minutes,
    ...(q.priority ? { priority: q.priority } : {}),
    ...(q.project ? { project_id: q.project.id } : {}),
    ...(q.rule ? { recurrence_rule: q.rule } : {}),
    ...(date ? { planned_date: date, status: 'PLANNED' as const } : {}),
    ...(date && q.time ? { planned_start_at: new Date(`${date}T${q.time}:00`).toISOString() } : {}),
  }
}
