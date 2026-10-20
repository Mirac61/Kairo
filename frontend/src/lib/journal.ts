// Journal „3 Dinge heute“: eine Notiz je Tag unter Journal/, die drei Fragen als Überschriften.
// Die Überschriften bleiben deutsch, damit die Dateien in jeder Sprache gleich aussehen und lesbar bleiben.
// Reine Funktionen ohne Imports, damit journal.test.ts sie direkt mit Node prüfen kann.
export const QUESTIONS = ['Was lief gut?', 'Was lerne ich daraus?', 'Was ist morgen wichtig?'] as const

export const journalPath = (date: string) => `Journal/${date}.md`

export interface Entry {
  answers: string[] // in der Reihenfolge von QUESTIONS
  other: string // alles andere (eigene Abschnitte aus dem Notizeditor), bleibt beim Speichern erhalten
}

export function toMarkdown(date: string, { answers, other }: Entry) {
  const parts = QUESTIONS.map((q, i) => [`## ${q}`, (answers[i] ?? '').trim()].filter(Boolean).join('\n\n'))
  return [`# Journal ${date}`, ...parts, other.trim()].filter(Boolean).join('\n\n') + '\n'
}

export function parseEntry(md: string): Entry {
  const answers = QUESTIONS.map(() => '')
  const other: string[] = []
  // Abschnitte ab jeder „## “-Überschrift; der Titel „# Journal …“ davor fällt weg.
  for (const sec of md.split(/^(?=## )/m)) {
    const [head = '', ...body] = sec.split('\n')
    const i = QUESTIONS.indexOf(head.slice(3).trim() as (typeof QUESTIONS)[number])
    if (i >= 0) answers[i] = body.join('\n').trim()
    else if (head.startsWith('## ')) other.push(sec.trim())
  }
  return { answers, other: other.join('\n\n') }
}
