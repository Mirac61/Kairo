// Dateien im Notizordner: Markdown, PDF, Bilder, C/C++, Makefiles und Zeichnungen (wie Kind in backend/internal/notes).
// Reine Funktionen ohne Imports, damit noteFiles.test.ts sie direkt mit Node prüfen kann.
export type FileKind = 'md' | 'pdf' | 'image' | 'code' | 'excalidraw' | ''

export function fileKind(path: string): FileKind {
  if (/\.md$/i.test(path)) return 'md'
  if (/\.pdf$/i.test(path)) return 'pdf'
  if (/\.(png|jpe?g|gif|webp)$/i.test(path)) return 'image'
  if (/\.excalidraw$/i.test(path)) return 'excalidraw'
  return /\.(c|h|cc|cpp|cxx|hh|hpp|hxx)$|(^|\/)makefile(\.[^/]*)?$/i.test(path) ? 'code' : ''
}

export const filesUrl = (path: string) => `/api/files/${path.split('/').map(encodeURIComponent).join('/')}`

// Vorschau: relative <img src> (neben der Notiz, auch „../bilder/x.png“) zeigen auf /api/files.
// Absolute URLs, data: und /pfad bleiben, wie sie sind. Zeichnungen (.excalidraw) bekommen data-drawing mit ihrem Pfad
// und als src das fertige Bild aus drawing(pfad), solange es fehlt ein leeres Platzhalterbild.
const PENDING = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="120"/>')}`
const attr = (s: string) => s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
export const previewImages = (html: string, notePath: string, drawing: (path: string) => string | undefined = () => undefined) =>
  html.replace(/<img\b[^>]*>/g, (tag) => {
    const src = /\ssrc="([^"]*)"/.exec(tag)?.[1]
    if (src === undefined || /^([a-z][a-z\d+.-]*:|\/)/i.test(src)) return tag
    const url = new URL(src, `http://x${filesUrl(notePath)}`).pathname
    if (fileKind(url) !== 'excalidraw') return tag.replace(/\ssrc="[^"]*"/, () => ` src="${url}"`)
    const p = decodeURIComponent(url.slice('/api/files/'.length))
    tag = tag.replace(/\ssrc="[^"]*"/, () => ` data-drawing="${attr(p)}" src="${drawing(p) ?? PENDING}"`)
    // not-zoom: md-editor legt sonst seinen Bild-Zoom darüber; ein Klick öffnet die Zeichnung.
    return /\sclass="/.test(tag) ? tag.replace(/\sclass="/, ' class="not-zoom ') : tag.replace(/^<img/, '<img class="not-zoom"')
  })

// Pfade aller Zeichnungen in einer mit previewImages bearbeiteten Vorschau.
export const drawingsIn = (html: string) =>
  [...html.matchAll(/data-drawing="([^"]*)"/g)].map((m) => m[1]!.replace(/&quot;/g, '"').replace(/&lt;/g, '<').replace(/&amp;/g, '&'))

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

// Pfadteile im Notizordner (immer mit „/“, relativ zur Wurzel).
export const withExt = (name: string, ext = '.md') => (name.toLowerCase().endsWith(ext.toLowerCase()) ? name : name + ext)
export const join = (dir: string, name: string) => (dir ? `${dir}/${name}` : name)
export const under = (p: string, prefix: string) => p === prefix || p.startsWith(`${prefix}/`)

// Eingefügte Bilder heißen nach Notiz und Zeit („VL 1-2026-10-09-140500.png“), mehrere zugleich mit -1, -2 …
export function assetName(notePath: string, mime: string, now: Date, i = 0, n = 1) {
  const pad = (x: number) => String(x).padStart(2, '0')
  const base = notePath.slice(notePath.lastIndexOf('/') + 1).replace(/\.md$/i, '')
  const stamp = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
  return `${base}-${stamp}${n > 1 ? `-${i + 1}` : ''}.${mime === 'image/jpeg' ? 'jpg' : mime.split('/')[1]}`
}
