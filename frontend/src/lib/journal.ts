// Journal „3 Dinge heute“: eine Notiz je Tag unter Journal/, die drei Fragen als Überschriften.
// Die Überschriften bleiben deutsch, damit die Dateien in jeder Sprache gleich aussehen und lesbar bleiben.
// Reine Funktionen ohne Imports, damit journal.test.ts sie direkt mit Node prüfen kann.
export const QUESTIONS = ['Was lief gut?', 'Was lerne ich daraus?', 'Was ist morgen wichtig?'] as const

export const journalPath = (date: string) => `Journal/${date}.md`

export interface Entry {
  answers: string[] // in der Reihenfolge von QUESTIONS
  other: string // alles andere (eigene Abschnitte aus dem Notizeditor), bleibt beim Speichern erhalten
}

const normalizeHeading = (heading: string) =>
  heading.replace(/[*_`~]/g, '').trim().replace(/\s+/g, ' ').replace(/[?!:.;]+$/g, '').trim().toLowerCase()

const questionIndexes = new Map(QUESTIONS.map((question, index) => [normalizeHeading(question), index]))

export function toMarkdown(date: string, { answers, other }: Entry) {
  const parts = QUESTIONS.map((q, i) => [`## ${q}`, (answers[i] ?? '').trim()].filter(Boolean).join('\n\n'))
  return [`# Journal ${date}`, ...parts, other.trim()].filter(Boolean).join('\n\n') + '\n'
}

export function parseEntry(md: string): Entry {
  const answers = QUESTIONS.map(() => '')
  const other: string[] = []
  // Abschnittsüberschriften dürfen anders geschrieben oder eingerückt sein; der Titel davor fällt weg.
  for (const sec of md.split(/^(?=#{2,6}\s)/m)) {
    const [head = '', ...body] = sec.split('\n')
    const heading = /^#{2,6}\s+(.+?)\s*$/.exec(head)?.[1]
    if (!heading) continue
    const i = questionIndexes.get(normalizeHeading(heading))
    if (i !== undefined) answers[i] = body.join('\n').trim()
    else other.push(sec.trim())
  }
  return { answers, other: other.join('\n\n') }
}

// Teilt eine Antwort in eine Punktliste: ein Punkt je Zeile (bewusst nicht an Satzenden).
// Führende Aufzählungszeichen werden entfernt, leere Zeilen fallen weg.
export function splitPoints(text: string): string[] {
  return text
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/^\s*(?:[-*+•]\s+|\d+[.)]\s+)/, '').trim())
    .filter(Boolean)
}

// Vergleicht Titel wie das Backend: ohne Groß-/Kleinschreibung und Mehrfach-Leerraum.
export const normalizeTitle = (s: string) => s.toLowerCase().split(/\s+/).filter(Boolean).join(' ')
