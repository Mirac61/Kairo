// Zeichnen auf PDF-Seiten und Bildern. Formen liegen in Seiteneinheiten (PDF: Punkt der gedrehten Seite, Bild: Pixel).
// Alle Formen außer Text sind Pfade: dieselbe Pfadangabe zeichnen SVG (Ansicht), Canvas (Bild speichern) und pdf-lib (PDF speichern).
// Ohne Laufzeit-Imports (pdf-lib kommt als Parameter), damit annotate.test.ts die Datei direkt mit Node prüfen kann.
import type * as PdfLib from 'pdf-lib'

export type Pt = [number, number]
export interface Box { x: number; y: number; w: number; h: number }
export type Tool = 'select' | 'pen' | 'marker' | 'line' | 'arrow' | 'rect' | 'ellipse' | 'text'
export type Size = 's' | 'm' | 'l'
type Base = { id: number; color: string }
export type Stroke =
  | (Base & { kind: 'pen' | 'marker'; width: number; pts: Pt[] })
  | (Base & { kind: 'line' | 'arrow'; width: number; a: Pt; b: Pt })
  | (Base & { kind: 'rect' | 'ellipse'; width: number } & Box)
export type Text = Base & { kind: 'text'; size: number; x: number; y: number; text: string }
export type Shape = Stroke | Text
// Seite: Größe in Seiteneinheiten; t bildet PDF-Koordinaten auf die Seite ab (pdf.js viewport.transform bei Zoom 1).
export interface Page { w: number; h: number; t: number[] }

export const FONT = 'Helvetica, Arial, sans-serif' // wie StandardFonts.Helvetica in pdf-lib
export const LINE = 1.25 // Zeilenhöhe in Schriftgrößen
export const ASCENT = 0.9 // Grundlinie der ersten Zeile unter der Oberkante (wie im Textfeld beim Schreiben)
export const MARKER_ALPHA = 0.4

// Grundeinheit einer Seite: Strichstärken und Schrift wachsen mit der Seite, damit „M“ auf Folie und Screenshot gleich wirkt.
const unit = (p: { w: number; h: number }) => Math.max(p.w, p.h) / 800
const WIDTHS = { s: 1.5, m: 3, l: 6 }
const MARKERS = { s: 8, m: 14, l: 24 }
const FONTS = { s: 12, m: 18, l: 28 }
export function sizeFor(kind: Shape['kind'], size: Size, page: { w: number; h: number }) {
  return unit(page) * (kind === 'text' ? FONTS : kind === 'marker' ? MARKERS : WIDTHS)[size]
}

const n = (v: number) => String(Math.round(v * 100) / 100)
const xy = ([x, y]: Pt) => `${n(x)} ${n(y)}`
const mid = (a: Pt, b: Pt): Pt => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2]

// Freihand: Mittelpunkte als Stützstellen, die Messpunkte als Kontrollpunkte (glatte Kurve statt Zickzack).
function smooth(p: Pt[]) {
  const [x0, y0] = p[0]!
  if (p.length === 1) return `M${xy([x0, y0])}L${xy([x0 + 0.01, y0])}` // Punkt: runde Kappe zeichnet ihn
  let d = `M${xy([x0, y0])}`
  for (let i = 1; i < p.length - 1; i++) d += `Q${xy(p[i]!)} ${xy(mid(p[i]!, p[i + 1]!))}`
  return `${d}L${xy(p[p.length - 1]!)}`
}

export function arrowHead(s: { a: Pt; b: Pt; width: number }): [Pt, Pt] {
  const [dx, dy] = [s.b[0] - s.a[0], s.b[1] - s.a[1]]
  const len = Math.hypot(dx, dy) || 1
  const head = Math.min(s.width * 4 + 4, len * 0.6)
  const side = (sign: number): Pt => {
    const a = Math.atan2(dy, dx) + Math.PI + sign * 0.5
    return [s.b[0] + Math.cos(a) * head, s.b[1] + Math.sin(a) * head]
  }
  return [side(1), side(-1)]
}

// map: für PDF die Umrechnung in PDF-Koordinaten; affine Abbildungen erhalten Bézierkurven, also reicht es, die Punkte abzubilden.
export function pathOf(s: Stroke, map: (p: Pt) => Pt = (p) => p): string {
  const m = (p: Pt) => xy(map(p))
  switch (s.kind) {
    case 'pen':
    case 'marker':
      return smooth(s.pts.map(map))
    case 'line':
      return `M${m(s.a)}L${m(s.b)}`
    case 'arrow': {
      const [l, r] = arrowHead(s)
      return `M${m(s.a)}L${m(s.b)}M${m(l)}L${m(s.b)}L${m(r)}`
    }
    case 'rect':
      return `M${m([s.x, s.y])}L${m([s.x + s.w, s.y])}L${m([s.x + s.w, s.y + s.h])}L${m([s.x, s.y + s.h])}Z`
    case 'ellipse': {
      const [rx, ry, cx, cy, k] = [s.w / 2, s.h / 2, s.x + s.w / 2, s.y + s.h / 2, 0.5523]
      const p = (dx: number, dy: number): string => m([cx + dx, cy + dy])
      return `M${p(rx, 0)}C${p(rx, ry * k)} ${p(rx * k, ry)} ${p(0, ry)}C${p(-rx * k, ry)} ${p(-rx, ry * k)} ${p(-rx, 0)}`
        + `C${p(-rx, -ry * k)} ${p(-rx * k, -ry)} ${p(0, -ry)}C${p(rx * k, -ry)} ${p(rx, -ry * k)} ${p(rx, 0)}Z`
    }
  }
}

export const linesOf = (t: Text) => t.text.split('\n')
export const baseline = (t: Text, i: number) => t.y + t.size * (ASCENT + i * LINE)

export function bounds(s: Shape, measure: (text: string, size: number) => number): Box {
  if (s.kind === 'text') {
    const lines = linesOf(s)
    return { x: s.x, y: s.y, w: Math.max(...lines.map((l) => measure(l, s.size)), s.size / 2), h: lines.length * s.size * LINE }
  }
  if ('w' in s) return { x: s.x, y: s.y, w: s.w, h: s.h } // Rechteck, Ellipse
  const pts = 'pts' in s ? s.pts : [s.a, s.b]
  const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1])
  const [x, y] = [Math.min(...xs), Math.min(...ys)]
  return { x, y, w: Math.max(...xs) - x, h: Math.max(...ys) - y }
}

// Bildet eine Form von Box from auf Box to ab (Verschieben = gleiche Größe, Skalieren = andere Größe). Text skaliert gleichmäßig.
export function transformed<S extends Shape>(s: S, from: Box, to: Box): S {
  const sx = from.w ? to.w / from.w : 1, sy = from.h ? to.h / from.h : 1
  const p = ([x, y]: Pt): Pt => [to.x + (x - from.x) * sx, to.y + (y - from.y) * sy]
  switch (s.kind) {
    case 'text': return { ...s, x: to.x, y: to.y, size: s.size * (from.w ? sx : sy) }
    case 'rect': case 'ellipse': { const [x, y] = p([s.x, s.y]); return { ...s, x, y, w: s.w * sx, h: s.h * sy } }
    case 'line': case 'arrow': return { ...s, a: p(s.a), b: p(s.b) }
    default: return { ...s, pts: s.pts.map(p) }
  }
}

// Box aus zwei Ecken; square (Shift) macht ein Quadrat bzw. einen Kreis.
export function boxFrom(a: Pt, b: Pt, square = false): Box {
  let [w, h] = [b[0] - a[0], b[1] - a[1]]
  if (square) {
    const side = Math.max(Math.abs(w), Math.abs(h))
    ;[w, h] = [Math.sign(w || 1) * side, Math.sign(h || 1) * side]
  }
  return { x: Math.min(a[0], a[0] + w), y: Math.min(a[1], a[1] + h), w: Math.abs(w), h: Math.abs(h) }
}

// Ecken einer Box: 0 oben links, 1 oben rechts, 2 unten rechts, 3 unten links.
export const corners = (b: Box): Pt[] => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h]]

// Ecke corner nach p ziehen, die Gegenecke bleibt stehen; keep hält das Seitenverhältnis (Text immer, sonst mit Shift).
export function resizeBox(b: Box, corner: number, p: Pt, keep: boolean): Box {
  const o = corners(b)[(corner + 2) % 4]!
  if (!keep || !b.w || !b.h) return boxFrom(o, p)
  const k = Math.max(Math.abs(p[0] - o[0]) / b.w, Math.abs(p[1] - o[1]) / b.h)
  return boxFrom(o, [o[0] + (Math.sign(p[0] - o[0]) || 1) * b.w * k, o[1] + (Math.sign(p[1] - o[1]) || 1) * b.h * k])
}

// Linie mit Shift: auf 45°-Schritte einrasten.
export function snapped(a: Pt, b: Pt): Pt {
  const len = Math.hypot(b[0] - a[0], b[1] - a[1])
  const ang = Math.round(Math.atan2(b[1] - a[1], b[0] - a[0]) / (Math.PI / 4)) * (Math.PI / 4)
  return [a[0] + Math.cos(ang) * len, a[1] + Math.sin(ang) * len]
}

// Affine Abbildungen im PDF-Format [a, b, c, d, e, f].
type Mat = [number, number, number, number, number, number]
export function apply(t: number[], [x, y]: Pt): Pt {
  const [a, b, c, d, e, f] = t as Mat
  return [a * x + c * y + e, b * x + d * y + f]
}
export function invert(t: number[]): Mat {
  const [a, b, c, d, e, f] = t as Mat
  const det = a * d - b * c
  return [d / det, -b / det, -c / det, a / det, (c * f - d * e) / det, (b * e - a * f) / det]
}

export function rgbOf(hex: string): [number, number, number] {
  const v = parseInt(hex.slice(1), 16)
  return [(v >> 16) / 255, ((v >> 8) & 255) / 255, (v & 255) / 255]
}

// Zeichnet die Formen in einen Canvas (Bild speichern). Der Canvas ist schon auf Seiteneinheiten skaliert.
export function drawOnCanvas(ctx: CanvasRenderingContext2D, shapes: Shape[]) {
  for (const s of shapes) {
    ctx.save()
    if (s.kind === 'text') {
      ctx.fillStyle = s.color
      ctx.font = `${s.size}px ${FONT}`
      linesOf(s).forEach((l, i) => ctx.fillText(l, s.x, baseline(s, i)))
    } else {
      if (s.kind === 'marker') {
        ctx.globalAlpha = MARKER_ALPHA
        ctx.globalCompositeOperation = 'multiply'
      }
      ctx.strokeStyle = s.color
      ctx.lineWidth = s.width
      ctx.lineCap = ctx.lineJoin = 'round'
      ctx.stroke(new Path2D(pathOf(s)))
    }
    ctx.restore()
  }
}

// Brennt die Formen in die PDF-Seiten ein und liefert die neue Datei.
// ponytail: Text nur in Helvetica (WinAnsi: Deutsch geht, Emoji o. Ä. wird zu „?“); eigene Schrift mit @pdf-lib/fontkit, falls nötig.
export async function drawOnPdf(lib: typeof PdfLib, bytes: ArrayBuffer, pages: Page[], shapes: Shape[][]): Promise<Uint8Array> {
  const { PDFDocument, StandardFonts, rgb, degrees, BlendMode, LineCapStyle, LineJoinStyle, pushGraphicsState, popGraphicsState, setLineJoin } = lib
  const doc = await PDFDocument.load(bytes) // verschlüsselte PDFs wirft pdf-lib hier ab
  const font = await doc.embedFont(StandardFonts.Helvetica)
  const known = new Set(font.getCharacterSet())
  const clean = (s: string) => [...s].map((c) => (known.has(c.codePointAt(0)!) ? c : '?')).join('')
  shapes.forEach((list, i) => {
    if (!list.length) return
    const page = doc.getPage(i)
    const inv = invert(pages[i]!.t)
    const angle = (Math.atan2(inv[1], inv[0]) * 180) / Math.PI // Textrichtung auf gedrehten Seiten
    for (const s of list) {
      const color = rgb(...rgbOf(s.color))
      if (s.kind === 'text') {
        linesOf(s).forEach((l, k) => {
          const [x, y] = apply(inv, [s.x, baseline(s, k)])
          page.drawText(clean(l), { x, y, size: s.size, font, color, rotate: degrees(angle) })
        })
        continue
      }
      // drawSvgPath spiegelt y (SVG zeigt nach unten); daher die PDF-Punkte mit -y übergeben.
      const d = pathOf(s, (p) => { const [x, y] = apply(inv, p); return [x, -y] })
      const marker = s.kind === 'marker'
      page.pushOperators(pushGraphicsState(), setLineJoin(LineJoinStyle.Round))
      page.drawSvgPath(d, {
        x: 0, y: 0, borderColor: color, borderWidth: s.width, borderLineCap: LineCapStyle.Round,
        borderOpacity: marker ? MARKER_ALPHA : 1, blendMode: marker ? BlendMode.Multiply : BlendMode.Normal,
      })
      page.pushOperators(popGraphicsState())
    }
  })
  return doc.save()
}

// ---------- Werkzeugleiste
export const TOOLS: { id: Tool; label: string; key: string; icon: string; sep?: boolean }[] = [
  { id: 'select', label: 'Auswählen', key: 'V', icon: '#i-cursor', sep: true },
  { id: 'pen', label: 'Stift', key: 'P', icon: '#i-pen' },
  { id: 'marker', label: 'Textmarker', key: 'M', icon: '#i-marker', sep: true },
  { id: 'line', label: 'Linie', key: 'L', icon: '#i-line' },
  { id: 'arrow', label: 'Pfeil', key: 'A', icon: '#i-arrow' },
  { id: 'rect', label: 'Rechteck', key: 'R', icon: '#i-rect' },
  { id: 'ellipse', label: 'Ellipse', key: 'O', icon: '#i-ellipse', sep: true },
  { id: 'text', label: 'Text', key: 'T', icon: '#i-text' },
]
export const TOOL_KEYS: Record<string, Tool> = Object.fromEntries(TOOLS.map((t) => [t.key.toLowerCase(), t.id]))
export const COLORS = [['#e5484d', 'Rot'], ['#ffd60a', 'Gelb'], ['#30a46c', 'Grün'], ['#0090ff', 'Blau'], ['#1c1c1c', 'Schwarz'], ['#ffffff', 'Weiß']] as const
export const SIZES: { id: Size; label: string }[] = [{ id: 's', label: 'Dünn' }, { id: 'm', label: 'Mittel' }, { id: 'l', label: 'Dick' }]
export const SCALES = [1, 0.75, 0.5, 0.25]

// Nächste Zoomstufe in Prozent; zwischen zwei Stufen geht es zur nächsten in Richtung dir.
const ZOOM_STEPS = [10, 25, 33, 50, 67, 75, 90, 100, 125, 150, 200, 300, 400, 600, 800]
export function zoomStep(pct: number, dir: 1 | -1): number {
  return dir > 0 ? ZOOM_STEPS.find((s) => s > pct + 0.5) ?? 800 : [...ZOOM_STEPS].reverse().find((s) => s < pct - 0.5) ?? 10
}
