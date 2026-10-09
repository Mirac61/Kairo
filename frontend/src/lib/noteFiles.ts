// Dateien im Notizordner: Markdown, PDF, Bilder (wie Kind in backend/internal/notes).
// Reine Funktionen ohne Imports, damit noteFiles.test.ts sie direkt mit Node prüfen kann.
export type FileKind = 'md' | 'pdf' | 'image' | ''

export function fileKind(path: string): FileKind {
  if (/\.md$/i.test(path)) return 'md'
  if (/\.pdf$/i.test(path)) return 'pdf'
  return /\.(png|jpe?g|gif|webp)$/i.test(path) ? 'image' : ''
}

export const filesUrl = (path: string) => `/api/files/${path.split('/').map(encodeURIComponent).join('/')}`

// Vorschau: relative <img src> (neben der Notiz, auch „../bilder/x.png“) zeigen auf /api/files.
// Absolute URLs, data: und /pfad bleiben, wie sie sind.
export const previewImages = (html: string, notePath: string) =>
  html.replace(/(<img\b[^>]*?\ssrc=")([^"]*)"/g, (_, pre: string, src: string) =>
    /^([a-z][a-z\d+.-]*:|\/)/i.test(src) ? `${pre}${src}"` : `${pre}${new URL(src, `http://x${filesUrl(notePath)}`).pathname}"`,
  )

// Vorlagen: {{titel}} {{ordner}} {{datum}} {{wochentag}} {{uhrzeit}}; Unbekanntes bleibt stehen.
// {{titel}} ist der Name ohne Datum vorn („2026-10-09 VL 3“ → „VL 3“), das steht schon in {{datum}}.
const DAYS = ['Sonntag', 'Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag']
export function fillTemplate(text: string, notePath: string, now = new Date()) {
  const pad = (n: number) => String(n).padStart(2, '0')
  const parts = notePath.split('/')
  const values: Record<string, string> = {
    titel: parts.at(-1)!.replace(/\.md$/i, '').replace(/^\d{4}-\d{2}-\d{2}\s+(?=\S)/, ''),
    ordner: parts.at(-2) ?? '',
    datum: `${pad(now.getDate())}.${pad(now.getMonth() + 1)}.${now.getFullYear()}`,
    wochentag: DAYS[now.getDay()]!,
    uhrzeit: `${pad(now.getHours())}:${pad(now.getMinutes())}`,
  }
  return text.replace(/\{\{(\w+)\}\}/g, (m, k: string) => values[k] ?? m)
}
