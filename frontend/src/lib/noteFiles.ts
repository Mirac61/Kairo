// Dateien im Notizordner: Markdown, PDF, Bilder, C/C++, Makefiles und Zeichnungen (wie Kind in backend/internal/notes).
// Reine Funktionen ohne Imports, damit noteFiles.test.ts sie direkt mit Node prüfen kann.

export type FileKind = 'md' | 'pdf' | 'image' | 'code' | 'excalidraw' | ''

export function fileKind(path: string): FileKind {
  if (/\.md$/i.test(path)) return 'md'
  if (/\.pdf$/i.test(path)) return 'pdf'
  if (/\.(png|jpe?g|gif|webp)$/i.test(path)) return 'image'
  if (/\.excalidraw$/i.test(path)) return 'excalidraw'
  if (/\.(c|h|cc|cpp|cxx|hh|hpp|hxx)$|(^|\/)makefile(\.[^/]*)?$/i.test(path)) return 'code'
  return ''
}

export function filesUrl(path: string): string {
  const encodedParts = path.split('/').map(encodeURIComponent).join('/')
  return `/api/files/${encodedParts}`
}

// Vorschau: relative <img src> (neben der Notiz, auch „../bilder/x.png“) zeigen auf /api/files.
// Absolute URLs, data: und /pfad bleiben, wie sie sind. Zeichnungen (.excalidraw) bekommen data-drawing mit ihrem Pfad
// und als src das fertige Bild aus drawing(pfad), solange es fehlt ein leeres Platzhalterbild.
// Obsidian-Einbettungen (![[Name]], als <img data-wiki> aus NoteEditor) löst resolve(name) zum Pfad auf; ohne Treffer bleibt der Text sichtbar.
const PENDING = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="120"/>')}`

function escapeHtmlAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

function unescapeHtmlAttr(s: string): string {
  return s
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
}

export function previewImages(
  html: string,
  notePath: string,
  drawing: (path: string) => string | undefined = () => undefined,
  resolve: (name: string) => string = () => '',
): string {
  return html.replace(/<img\b[^>]*>/g, (tag) => {
    const src = /\ssrc="([^"]*)"/.exec(tag)?.[1]
    let url: string

    if (src !== undefined && /\sdata-wiki=/.test(tag)) {
      const p = resolve(unescapeHtmlAttr(src))
      if (!p) {
        return `<span class="md-wiki-missing">![[${src}]]</span>`
      }
      url = filesUrl(p)
    } else {
      if (src === undefined || /^([a-z][a-z\d+.-]*:|\/)/i.test(src)) {
        return tag
      }
      url = new URL(src, `http://x${filesUrl(notePath)}`).pathname
    }

    if (fileKind(url) !== 'excalidraw') {
      return tag.replace(/\ssrc="[^"]*"/, () => ` src="${url}"`)
    }

    const p = decodeURIComponent(url.slice('/api/files/'.length))
    tag = tag.replace(
      /\ssrc="[^"]*"/,
      () => ` data-drawing="${escapeHtmlAttr(p)}" src="${drawing(p) ?? PENDING}"`,
    )

    // not-zoom: md-editor legt sonst seinen Bild-Zoom darüber; ein Klick öffnet die Zeichnung.
    return /\sclass="/.test(tag)
      ? tag.replace(/\sclass="/, ' class="not-zoom ')
      : tag.replace(/^<img/, '<img class="not-zoom"')
  })
}

// [[Name]], [[Name|Alias]], [[Name#Überschrift]] wie in Obsidian; ohne Name ([[#Überschrift]]) ist die Notiz selbst gemeint.
export function parseWikiLink(inner: string): { name: string; heading: string; label: string } {
  const [target = '', ...alias] = inner.split('|')
  const [name = '', ...h] = target.split('#')
  const n = name.trim()
  const heading = h.join('#').trim()
  const customLabel = alias.join('|').trim()
  const fallbackLabel = [n, heading].filter(Boolean).join(' > ') || inner.trim()

  return {
    name: n,
    heading,
    label: customLabel || fallbackLabel,
  }
}

// Ziel einer [[…]]-Verknüpfung: Dateiname ohne .md, ohne Unterscheidung der Schreibweise (wie notes.Backlinks im Backend).
// Erst Pfad ab der Wurzel bzw. neben der Notiz (dir), sonst der kürzeste Pfad mit diesem Namen. "" = nicht gefunden.
export function resolveNote(name: string, dir: string, files: string[]): string {
  const want = name.replace(/\.md$/i, '').toLowerCase()
  const base = want.slice(want.lastIndexOf('/') + 1)
  let best = ''

  for (const f of files) {
    if (fileKind(f) !== 'md') continue
    const stem = f.slice(0, -3).toLowerCase()
    if (stem === want || stem === join(dir, want).toLowerCase()) {
      return f
    }
    if (stem.slice(stem.lastIndexOf('/') + 1) === base && (!best || f.length < best.length)) {
      best = f
    }
  }

  return best
}

// Vorschau: die <a data-wikilink="…"> aus NoteEditor bekommen data-note mit dem Pfad der Zielnotiz (und data-heading
// für [[Name#Abschnitt]]), ohne Treffer die Klasse md-wiki-unresolved samt data-name (dem gewünschten Namen, damit ein
// Klick die Notiz anlegen kann).
export function previewWikiLinks(html: string, resolve: (name: string) => string): string {
  return html.replace(/<a\b[^>]*?\sdata-wikilink="([^"]*)"[^>]*>/g, (_, v: string) => {
    const { name, heading } = parseWikiLink(unescapeHtmlAttr(v))
    const p = resolve(name)
    if (p) {
      return `<a href="#" class="md-wikilink" data-note="${escapeHtmlAttr(p)}" data-heading="${escapeHtmlAttr(heading)}">`
    }
    return `<a href="#" class="md-wikilink md-wiki-unresolved" data-name="${escapeHtmlAttr(name)}">`
  })
}

// fold: Suche ohne Groß/Klein und ohne Akzente (normalize ist eingebaut).
export function fold(s: string): string {
  return s.toLowerCase().normalize('NFD').replace(/\p{M}/gu, '')
}

// slugify: Anker für Überschriften ([[Name#Abschnitt]] scrollt darauf). Ohne Akzente, Kleinschreibung,
// Nicht-Buchstaben/Ziffern werden zu „-“.
export function slugify(s: string): string {
  return fold(s)
    .replace(/[^\p{L}\p{N}]+/gu, '-')
    .replace(/^-+|-+$/g, '')
}

// Pfade aller Zeichnungen in einer mit previewImages bearbeiteten Vorschau.
export function drawingsIn(html: string): string[] {
  return [...html.matchAll(/data-drawing="([^"]*)"/g)].map((m) => unescapeHtmlAttr(m[1]!))
}

// Sucht ein Link-Ziel wie Obsidian: Pfad ab der Wurzel, dann neben der Notiz (dir), sonst die Datei mit diesem
// Namen irgendwo im Ordner, bei mehreren die mit dem kürzesten Pfad. "" = nicht gefunden.
export function resolveLink(name: string, dir: string, files: string[]): string {
  let best = ''
  for (const f of files) {
    if (f === name || f === join(dir, name)) {
      return f
    }
    if (f.slice(f.lastIndexOf('/') + 1) === name && (!best || f.length < best.length)) {
      best = f
    }
  }
  return best
}

// Zeichnungen aus Obsidian (Excalidraw-Plugin) liegen als „Name.excalidraw.md“ vor: die Szene als ```json- oder
// ```compressed-json-Block (LZ-String, Base64; decompress kommt als Parameter, damit die Datei ohne Imports bleibt),
// eingefügte Bilder und Formeln unter „## Embedded Files“ als „<fileId>: [[bild.png]]“ bzw. „<fileId>: $$TeX$$“.
export function isObsidianDrawing(path: string): boolean {
  return /\.excalidraw\.md$/i.test(path)
}

export interface ObsidianScene {
  elements: { type?: string; fileId?: string; customData?: object }[]
  files?: Record<string, unknown>
  [k: string]: unknown
}

export type Embedded = { file: string } | { tex: string } | { other: string }

export function parseObsidianDrawing(
  text: string,
  decompress: (s: string) => string | null,
): { scene: ObsidianScene; embedded: Map<string, Embedded> } {
  const m = /```(compressed-json|json)\n([\s\S]*?)\n```/.exec(text)
  if (!m) {
    throw new Error('kein json- oder compressed-json-Block')
  }

  const raw = m[1] === 'compressed-json' ? decompress(m[2]!.replace(/\s+/g, '')) : m[2]!
  if (!raw) {
    throw new Error('compressed-json nicht lesbar')
  }

  const scene = JSON.parse(raw) as ObsidianScene
  if (!Array.isArray(scene.elements)) {
    throw new Error('Szene ohne elements')
  }

  const embedded = new Map<string, Embedded>()
  const section = /^## Embedded Files\n([\s\S]*?)(?=^%%|^#|(?![\s\S]))/m.exec(text)?.[1] ?? ''

  for (const entry of section.split(/^(?=[0-9a-f]{8,}: )/m)) {
    const e = /^([0-9a-f]{8,}): ([\s\S]*)$/.exec(entry.trim())
    if (!e) continue

    const v = e[2]!.trim()
    const link = /^!?\[\[([^\]|]+)(?:\|[^\]]*)?\]\]$/.exec(v)
    const tex = /^\$\$([\s\S]*)\$\$$/.exec(v)

    embedded.set(
      e[1]!,
      link ? { file: link[1]!.trim() } : tex ? { tex: tex[1]!.trim() } : { other: v },
    )
  }

  return { scene, embedded }
}

// Vorlagen: {{titel}} {{ordner}} {{datum}} {{wochentag}} {{uhrzeit}}; Unbekanntes bleibt stehen.
// {{titel}} ist der Name ohne Datum vorn („2026-10-09 VL 3“ → „VL 3“), das steht schon in {{datum}}.
const DAYS = ['Sonntag', 'Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag']

export function fillTemplate(text: string, notePath: string, now = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const parts = notePath.split('/')
  const values: Record<string, string> = {
    titel: parts
      .at(-1)!
      .replace(/\.md$/i, '')
      .replace(/^\d{4}-\d{2}-\d{2}\s+(?=\S)/, ''),
    ordner: parts.at(-2) ?? '',
    datum: `${pad(now.getDate())}.${pad(now.getMonth() + 1)}.${now.getFullYear()}`,
    wochentag: DAYS[now.getDay()]!,
    uhrzeit: `${pad(now.getHours())}:${pad(now.getMinutes())}`,
  }

  return text.replace(/\{\{(\w+)\}\}/g, (m, k: string) => values[k] ?? m)
}

// Pfadteile im Notizordner (immer mit „/“, relativ zur Wurzel).
export function withExt(name: string, ext = '.md'): string {
  return name.toLowerCase().endsWith(ext.toLowerCase()) ? name : name + ext
}

export function join(dir: string, name: string): string {
  return dir ? `${dir}/${name}` : name
}

export function under(p: string, prefix: string): boolean {
  return p === prefix || p.startsWith(`${prefix}/`)
}

// Eingefügte Bilder heißen nach Notiz und Zeit („VL 1-2026-10-09-140500.png“), mehrere zugleich mit -1, -2 …
export function assetName(notePath: string, mime: string, now: Date, i = 0, n = 1): string {
  const pad = (x: number) => String(x).padStart(2, '0')
  const base = notePath.slice(notePath.lastIndexOf('/') + 1).replace(/\.md$/i, '')
  const stamp = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
  const indexSuffix = n > 1 ? `-${i + 1}` : ''
  const ext = mime === 'image/jpeg' ? 'jpg' : mime.split('/')[1]

  return `${base}-${stamp}${indexSuffix}.${ext}`
}

// ── Autovervollständigung für [[…]] und ![[…]] (Editor und Suche) ───────────

// Ein Treffer der Link-Vervollständigung. label ist der Name, der in [[…]] steht (bei Notizen ohne .md),
// detail der Ordner, kind entscheidet über das Typ-Icon.
export interface WikiCandidate {
  label: string
  path: string
  detail: string
  kind: 'note' | 'image' | 'pdf' | 'excalidraw'
}

function folderOf(p: string): string {
  return p.slice(0, Math.max(p.lastIndexOf('/'), 0))
}

// Alle Notizen (.md) für [[…]].
export function noteCandidates(files: string[]): WikiCandidate[] {
  return files
    .filter((f) => fileKind(f) === 'md')
    .map((f) => ({
      label: f.slice(f.lastIndexOf('/') + 1, -3),
      path: f,
      detail: folderOf(f),
      kind: 'note' as const,
    }))
}

// Assets (Bild, PDF, Zeichnung) für ![[…]]. .excalidraw.md (Obsidian) bleibt außen vor: es ist noch kein
// einbettbares Bild, sondern muss erst umgewandelt werden.
export function assetCandidates(files: string[]): WikiCandidate[] {
  return files
    .filter((f) => ['image', 'pdf', 'excalidraw'].includes(fileKind(f)))
    .map((f) => ({
      label: f.slice(f.lastIndexOf('/') + 1),
      path: f,
      detail: folderOf(f),
      kind: fileKind(f) as WikiCandidate['kind'],
    }))
}

// fuzzyScore: 0 = kein Treffer, je höher desto besser (exakt > Präfix > Wortanfang > Teilstring > Teilfolge).
export function fuzzyScore(query: string, label: string): number {
  const q = fold(query.trim())
  const l = fold(label)
  if (!q) return 0
  if (l === q) return 1000
  if (l.startsWith(q)) return 900

  const i = l.indexOf(q)
  if (i > 0 && /[/\s\-_.(]/.test(l[i - 1]!)) return 700 // Wortanfang
  if (i >= 0) return 500

  let qi = 0
  for (let k = 0; k < l.length && qi < q.length; k++) {
    if (l[k] === q[qi]) qi++
  }
  return qi === q.length ? 300 : 0
}

// rankCandidates: fuzzy absteigend, dann kürzerer Pfad (näher an der Wurzel), dann Name.
export function rankCandidates(query: string, candidates: WikiCandidate[]): WikiCandidate[] {
  return candidates
    .map((c) => ({ c, s: fuzzyScore(query, c.label) }))
    .filter((x) => x.s > 0)
    .sort(
      (a, b) =>
        b.s - a.s || a.c.path.length - b.c.path.length || a.c.label.localeCompare(b.c.label),
    )
    .map((x) => x.c)
}

// headingsOf: die Überschriften (# bis ######) einer Notiz in Reihenfolge, ohne die Rauten.
export function headingsOf(content: string): string[] {
  const out: string[] = []
  for (const line of content.split('\n')) {
    const m = /^#{1,6}\s+(.+?)\s*$/.exec(line)
    if (m) {
      out.push(m[1]!)
    }
  }
  return out
}
