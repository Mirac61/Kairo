// Zeichnen auf PDF-Seiten und Bildern.
// Formen liegen in Seiteneinheiten (PDF: Punkt der gedrehten Seite, Bild: Pixel).
// Alle Formen außer Text sind Pfade: dieselbe Pfadangabe zeichnen SVG (Ansicht), Canvas (Bild speichern) und pdf-lib (PDF speichern).
// Ohne Laufzeit-Imports (pdf-lib kommt als Parameter), damit annotate.test.ts die Datei direkt mit Node prüfen kann.

import type * as PdfLib from 'pdf-lib'

export type Pt = [number, number]

export interface Box {
  x: number
  y: number
  w: number
  h: number
}

export type Tool = 'select' | 'pen' | 'marker' | 'line' | 'arrow' | 'rect' | 'ellipse' | 'text'

export type Size = 's' | 'm' | 'l'

interface BaseShape {
  id: number
  color: string
}

export type Stroke =
  | (BaseShape & { kind: 'pen' | 'marker'; width: number; pts: Pt[] })
  | (BaseShape & { kind: 'line' | 'arrow'; width: number; a: Pt; b: Pt })
  | (BaseShape & { kind: 'rect' | 'ellipse'; width: number } & Box)

export type Text = BaseShape & {
  kind: 'text'
  size: number
  x: number
  y: number
  text: string
}

export type Shape = Stroke | Text

// Seite: Größe in Seiteneinheiten; t bildet PDF-Koordinaten auf die Seite ab (pdf.js viewport.transform bei Zoom 1).
export interface Page {
  w: number
  h: number
  t: number[]
}

export const FONT = 'Helvetica, Arial, sans-serif' // wie StandardFonts.Helvetica in pdf-lib
export const LINE = 1.25 // Zeilenhöhe in Schriftgrößen
export const ASCENT = 0.9 // Grundlinie der ersten Zeile unter der Oberkante (wie im Textfeld beim Schreiben)
export const MARKER_ALPHA = 0.4

// Grundeinheit einer Seite: Strichstärken und Schrift wachsen mit der Seite, damit „M“ auf Folie und Screenshot gleich wirkt.
function unit(page: { w: number; h: number }): number {
  return Math.max(page.w, page.h) / 800
}

const WIDTHS: Record<Size, number> = { s: 1.5, m: 3, l: 6 }
const MARKERS: Record<Size, number> = { s: 8, m: 14, l: 24 }
const FONTS: Record<Size, number> = { s: 12, m: 18, l: 28 }

export function sizeFor(kind: Shape['kind'], size: Size, page: { w: number; h: number }): number {
  const scale = unit(page)
  if (kind === 'text') {
    return scale * FONTS[size]
  }
  if (kind === 'marker') {
    return scale * MARKERS[size]
  }
  return scale * WIDTHS[size]
}

function roundToTwoDecimals(val: number): string {
  return String(Math.round(val * 100) / 100)
}

function formatPoint([x, y]: Pt): string {
  return `${roundToTwoDecimals(x)} ${roundToTwoDecimals(y)}`
}

function midpoint(a: Pt, b: Pt): Pt {
  return [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2]
}

// Freihand: Mittelpunkte als Stützstellen, die Messpunkte als Kontrollpunkte (glatte Kurve statt Zickzack).
function smooth(points: Pt[]): string {
  const [x0, y0] = points[0]!

  // Einzelner Punkt: kurze Linie für runde Kappe
  if (points.length === 1) {
    return `M${formatPoint([x0, y0])}L${formatPoint([x0 + 0.01, y0])}`
  }

  let pathString = `M${formatPoint([x0, y0])}`
  for (let index = 1; index < points.length - 1; index++) {
    const current = points[index]!
    const next = points[index + 1]!
    const controlPoint = formatPoint(current)
    const mid = formatPoint(midpoint(current, next))
    pathString += `Q${controlPoint} ${mid}`
  }

  const last = points[points.length - 1]!
  return `${pathString}L${formatPoint(last)}`
}

export function arrowHead(stroke: { a: Pt; b: Pt; width: number }): [Pt, Pt] {
  const deltaX = stroke.b[0] - stroke.a[0]
  const deltaY = stroke.b[1] - stroke.a[1]
  const length = Math.hypot(deltaX, deltaY) || 1
  const headSize = Math.min(stroke.width * 4 + 4, length * 0.6)

  const computeSide = (sign: number): Pt => {
    const angle = Math.atan2(deltaY, deltaX) + Math.PI + sign * 0.5
    return [stroke.b[0] + Math.cos(angle) * headSize, stroke.b[1] + Math.sin(angle) * headSize]
  }

  return [computeSide(1), computeSide(-1)]
}

// map: für PDF die Umrechnung in PDF-Koordinaten; affine Abbildungen erhalten Bézierkurven, also reicht es, die Punkte abzubilden.
export function pathOf(stroke: Stroke, pointMapper: (p: Pt) => Pt = (p) => p): string {
  const mapPt = (point: Pt) => formatPoint(pointMapper(point))

  switch (stroke.kind) {
    case 'pen':
    case 'marker':
      return smooth(stroke.pts.map(pointMapper))

    case 'line':
      return `M${mapPt(stroke.a)}L${mapPt(stroke.b)}`

    case 'arrow': {
      const [leftWing, rightWing] = arrowHead(stroke)
      return `M${mapPt(stroke.a)}L${mapPt(stroke.b)}M${mapPt(leftWing)}L${mapPt(stroke.b)}L${mapPt(rightWing)}`
    }

    case 'rect': {
      const topLeft = mapPt([stroke.x, stroke.y])
      const topRight = mapPt([stroke.x + stroke.w, stroke.y])
      const bottomRight = mapPt([stroke.x + stroke.w, stroke.y + stroke.h])
      const bottomLeft = mapPt([stroke.x, stroke.y + stroke.h])
      return `M${topLeft}L${topRight}L${bottomRight}L${bottomLeft}Z`
    }

    case 'ellipse': {
      const radiusX = stroke.w / 2
      const radiusY = stroke.h / 2
      const centerX = stroke.x + radiusX
      const centerY = stroke.y + radiusY
      const kappa = 0.5523

      const pointOffset = (deltaX: number, deltaY: number): string =>
        mapPt([centerX + deltaX, centerY + deltaY])

      return (
        `M${pointOffset(radiusX, 0)}` +
        `C${pointOffset(radiusX, radiusY * kappa)} ${pointOffset(radiusX * kappa, radiusY)} ${pointOffset(0, radiusY)}` +
        `C${pointOffset(-radiusX * kappa, radiusY)} ${pointOffset(-radiusX, radiusY * kappa)} ${pointOffset(-radiusX, 0)}` +
        `C${pointOffset(-radiusX, -radiusY * kappa)} ${pointOffset(-radiusX * kappa, -radiusY)} ${pointOffset(0, -radiusY)}` +
        `C${pointOffset(radiusX * kappa, -radiusY)} ${pointOffset(radiusX, -radiusY * kappa)} ${pointOffset(radiusX, 0)}Z`
      )
    }
  }
}

export function linesOf(textShape: Text): string[] {
  return textShape.text.split('\n')
}

export function baseline(textShape: Text, lineIndex: number): number {
  return textShape.y + textShape.size * (ASCENT + lineIndex * LINE)
}

export function bounds(shape: Shape, measureText: (text: string, size: number) => number): Box {
  if (shape.kind === 'text') {
    const lines = linesOf(shape)
    const lineWidths = lines.map((line) => measureText(line, shape.size))
    const maxWidth = Math.max(...lineWidths, shape.size / 2)
    const totalHeight = lines.length * shape.size * LINE
    return { x: shape.x, y: shape.y, w: maxWidth, h: totalHeight }
  }

  if ('w' in shape) {
    return { x: shape.x, y: shape.y, w: shape.w, h: shape.h }
  }

  const points = 'pts' in shape ? shape.pts : [shape.a, shape.b]
  const xCoords = points.map((p) => p[0])
  const yCoords = points.map((p) => p[1])
  const minX = Math.min(...xCoords)
  const minY = Math.min(...yCoords)
  const maxX = Math.max(...xCoords)
  const maxY = Math.max(...yCoords)

  return {
    x: minX,
    y: minY,
    w: maxX - minX,
    h: maxY - minY,
  }
}

// Bildet eine Form von Box from auf Box to ab (Verschieben = gleiche Größe, Skalieren = andere Größe). Text skaliert gleichmäßig.
export function transformed<S extends Shape>(shape: S, from: Box, to: Box): S {
  const scaleX = from.w ? to.w / from.w : 1
  const scaleY = from.h ? to.h / from.h : 1

  const mapPoint = ([x, y]: Pt): Pt => [to.x + (x - from.x) * scaleX, to.y + (y - from.y) * scaleY]

  switch (shape.kind) {
    case 'text':
      return {
        ...shape,
        x: to.x,
        y: to.y,
        size: shape.size * (from.w ? scaleX : scaleY),
      }

    case 'rect':
    case 'ellipse': {
      const [mappedX, mappedY] = mapPoint([shape.x, shape.y])
      return {
        ...shape,
        x: mappedX,
        y: mappedY,
        w: shape.w * scaleX,
        h: shape.h * scaleY,
      }
    }

    case 'line':
    case 'arrow':
      return {
        ...shape,
        a: mapPoint(shape.a),
        b: mapPoint(shape.b),
      }

    default:
      return {
        ...shape,
        pts: shape.pts.map(mapPoint),
      }
  }
}

// Box aus zwei Ecken; square (Shift) erzeugt ein Quadrat bzw. einen Kreis.
export function boxFrom(a: Pt, b: Pt, square = false): Box {
  let deltaX = b[0] - a[0]
  let deltaY = b[1] - a[1]

  if (square) {
    const sideLength = Math.max(Math.abs(deltaX), Math.abs(deltaY))
    deltaX = Math.sign(deltaX || 1) * sideLength
    deltaY = Math.sign(deltaY || 1) * sideLength
  }

  return {
    x: Math.min(a[0], a[0] + deltaX),
    y: Math.min(a[1], a[1] + deltaY),
    w: Math.abs(deltaX),
    h: Math.abs(deltaY),
  }
}

// Ecken einer Box: 0 = oben links, 1 = oben rechts, 2 = unten rechts, 3 = unten links.
export function corners(box: Box): Pt[] {
  return [
    [box.x, box.y],
    [box.x + box.w, box.y],
    [box.x + box.w, box.y + box.h],
    [box.x, box.y + box.h],
  ]
}

// Ecke corner nach point ziehen, die Gegenecke bleibt fixiert; keepRatio hält das Seitenverhältnis.
export function resizeBox(box: Box, cornerIndex: number, targetPoint: Pt, keepRatio: boolean): Box {
  const oppositeCorner = corners(box)[(cornerIndex + 2) % 4]!

  if (!keepRatio || !box.w || !box.h) {
    return boxFrom(oppositeCorner, targetPoint)
  }

  const ratioScale = Math.max(
    Math.abs(targetPoint[0] - oppositeCorner[0]) / box.w,
    Math.abs(targetPoint[1] - oppositeCorner[1]) / box.h,
  )

  const directionX = Math.sign(targetPoint[0] - oppositeCorner[0]) || 1
  const directionY = Math.sign(targetPoint[1] - oppositeCorner[1]) || 1

  const scaledTarget: Pt = [
    oppositeCorner[0] + directionX * box.w * ratioScale,
    oppositeCorner[1] + directionY * box.h * ratioScale,
  ]

  return boxFrom(oppositeCorner, scaledTarget)
}

// Linie mit Shift: auf 45°-Schritte einrasten.
export function snapped(a: Pt, b: Pt): Pt {
  const deltaX = b[0] - a[0]
  const deltaY = b[1] - a[1]
  const length = Math.hypot(deltaX, deltaY)
  const angleStep = Math.PI / 4
  const snappedAngle = Math.round(Math.atan2(deltaY, deltaX) / angleStep) * angleStep

  return [a[0] + Math.cos(snappedAngle) * length, a[1] + Math.sin(snappedAngle) * length]
}

// Affine Abbildungen im PDF-Format [a, b, c, d, e, f].
type Matrix = [number, number, number, number, number, number]

export function apply(transform: number[], [x, y]: Pt): Pt {
  const [a, b, c, d, e, f] = transform as Matrix
  return [a * x + c * y + e, b * x + d * y + f]
}

export function invert(transform: number[]): Matrix {
  const [a, b, c, d, e, f] = transform as Matrix
  const determinant = a * d - b * c

  return [
    d / determinant,
    -b / determinant,
    -c / determinant,
    a / determinant,
    (c * f - d * e) / determinant,
    (b * e - a * f) / determinant,
  ]
}

export function rgbOf(hex: string): [number, number, number] {
  const value = parseInt(hex.slice(1), 16)
  const red = (value >> 16) / 255
  const green = ((value >> 8) & 255) / 255
  const blue = (value & 255) / 255
  return [red, green, blue]
}

// Zeichnet die Formen in einen Canvas (Bild speichern). Der Canvas ist schon auf Seiteneinheiten skaliert.
export function drawOnCanvas(ctx: CanvasRenderingContext2D, shapes: Shape[]): void {
  for (const shape of shapes) {
    ctx.save()
    if (shape.kind === 'text') {
      ctx.fillStyle = shape.color
      ctx.font = `${shape.size}px ${FONT}`
      linesOf(shape).forEach((line, index) => {
        ctx.fillText(line, shape.x, baseline(shape, index))
      })
    } else {
      if (shape.kind === 'marker') {
        ctx.globalAlpha = MARKER_ALPHA
        ctx.globalCompositeOperation = 'multiply'
      }
      ctx.strokeStyle = shape.color
      ctx.lineWidth = shape.width
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.stroke(new Path2D(pathOf(shape)))
    }
    ctx.restore()
  }
}

// Brennt die Formen in die PDF-Seiten ein und liefert die neue Datei.
// Text nur in Helvetica (WinAnsi: Deutsch geht, Emoji o. Ä. wird zu „?“); eigene Schrift mit @pdf-lib/fontkit, falls nötig.
export async function drawOnPdf(
  lib: typeof PdfLib,
  bytes: ArrayBuffer,
  pages: Page[],
  shapes: Shape[][],
): Promise<Uint8Array> {
  const {
    PDFDocument,
    StandardFonts,
    rgb,
    degrees,
    BlendMode,
    LineCapStyle,
    LineJoinStyle,
    pushGraphicsState,
    popGraphicsState,
    setLineJoin,
  } = lib

  const doc = await PDFDocument.load(bytes)
  const font = await doc.embedFont(StandardFonts.Helvetica)
  const knownCharacters = new Set(font.getCharacterSet())

  const sanitizeText = (str: string) =>
    [...str].map((char) => (knownCharacters.has(char.codePointAt(0)!) ? char : '?')).join('')

  shapes.forEach((shapeList, pageIndex) => {
    if (!shapeList.length) return

    const page = doc.getPage(pageIndex)
    const inverseTransform = invert(pages[pageIndex]!.t)
    const rotationAngle = (Math.atan2(inverseTransform[1], inverseTransform[0]) * 180) / Math.PI

    for (const shape of shapeList) {
      const [r, g, b] = rgbOf(shape.color)
      const color = rgb(r, g, b)

      if (shape.kind === 'text') {
        linesOf(shape).forEach((line, lineIndex) => {
          const [posX, posY] = apply(inverseTransform, [shape.x, baseline(shape, lineIndex)])
          page.drawText(sanitizeText(line), {
            x: posX,
            y: posY,
            size: shape.size,
            font,
            color,
            rotate: degrees(rotationAngle),
          })
        })
        continue
      }

      // drawSvgPath spiegelt y (SVG zeigt nach unten); daher die PDF-Punkte mit -y übergeben.
      const pathData = pathOf(shape, (point) => {
        const [x, y] = apply(inverseTransform, point)
        return [x, -y]
      })

      const isMarker = shape.kind === 'marker'
      page.pushOperators(pushGraphicsState(), setLineJoin(LineJoinStyle.Round))
      page.drawSvgPath(pathData, {
        x: 0,
        y: 0,
        borderColor: color,
        borderWidth: shape.width,
        borderLineCap: LineCapStyle.Round,
        borderOpacity: isMarker ? MARKER_ALPHA : 1,
        blendMode: isMarker ? BlendMode.Multiply : BlendMode.Normal,
      })
      page.pushOperators(popGraphicsState())
    }
  })

  return doc.save()
}

// ---------- Werkzeugleiste
export interface ToolItem {
  id: Tool
  label: string
  key: string
  icon: string
  sep?: boolean
}

export const TOOLS: ToolItem[] = [
  { id: 'select', label: 'Auswählen', key: 'V', icon: '#i-cursor', sep: true },
  { id: 'pen', label: 'Stift', key: 'P', icon: '#i-pen' },
  { id: 'marker', label: 'Textmarker', key: 'M', icon: '#i-marker', sep: true },
  { id: 'line', label: 'Linie', key: 'L', icon: '#i-line' },
  { id: 'arrow', label: 'Pfeil', key: 'A', icon: '#i-arrow' },
  { id: 'rect', label: 'Rechteck', key: 'R', icon: '#i-rect' },
  { id: 'ellipse', label: 'Ellipse', key: 'O', icon: '#i-ellipse', sep: true },
  { id: 'text', label: 'Text', key: 'T', icon: '#i-text' },
]

export const TOOL_KEYS: Record<string, Tool> = Object.fromEntries(
  TOOLS.map((tool) => [tool.key.toLowerCase(), tool.id]),
)

export const COLORS = [
  ['#e5484d', 'Rot'],
  ['#ffd60a', 'Gelb'],
  ['#30a46c', 'Grün'],
  ['#0090ff', 'Blau'],
  ['#1c1c1c', 'Schwarz'],
  ['#ffffff', 'Weiß'],
] as const

export const SIZES: { id: Size; label: string }[] = [
  { id: 's', label: 'Dünn' },
  { id: 'm', label: 'Mittel' },
  { id: 'l', label: 'Dick' },
]

export const SCALES = [1, 0.75, 0.5, 0.25]

// Nächste Zoomstufe in Prozent; zwischen zwei Stufen geht es zur nächsten in Richtung dir.
const ZOOM_STEPS = [10, 25, 33, 50, 67, 75, 90, 100, 125, 150, 200, 300, 400, 600, 800]

export function zoomStep(currentPercent: number, direction: 1 | -1): number {
  if (direction > 0) {
    return ZOOM_STEPS.find((step) => step > currentPercent + 0.5) ?? 800
  }
  return [...ZOOM_STEPS].reverse().find((step) => step < currentPercent - 0.5) ?? 10
}
