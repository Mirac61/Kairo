<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PDFDocumentProxy, PDFPageProxy, RenderTask } from 'pdfjs-dist'
import { ApiError, errorMessage, fetchFile, replaceFile } from '@/api/client'
import {
  FONT, LINE, baseline, bounds, boxFrom, corners, drawOnCanvas, drawOnPdf, linesOf, pathOf, resizeBox, sizeFor, snapped, transformed,
  type Box, type Page, type Pt, type Shape, type Size, type Stroke, type Text, type Tool,
} from '@/lib/annotate'
import { filesUrl } from '@/lib/noteFiles'

// PDFs und Bilder ansehen und bezeichnen: Stift, Textmarker, Linie, Pfeil, Rechteck, Ellipse, Text; Zoom; Bilder auch verkleinern.
// Bis zum Speichern bleibt jede Form einzeln wählbar; Speichern brennt sie in die Datei ein (die alte Fassung landet in .trash).
const props = defineProps<{ path: string; kind: 'pdf' | 'image' }>()
const emit = defineEmits<{ dirty: [boolean] }>()

const IMAGE_TYPES: Record<string, string> = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp' }
const imageType = IMAGE_TYPES[props.path.split('.').pop()!.toLowerCase()]
const editable = props.kind === 'pdf' || !!imageType // GIF kann der Browser nicht schreiben: nur ansehen

const TOOLS: { id: Tool; label: string; key: string; icon: string; sep?: boolean }[] = [
  { id: 'select', label: 'Auswählen', key: 'V', icon: '#i-cursor', sep: true },
  { id: 'pen', label: 'Stift', key: 'P', icon: '#i-pen' },
  { id: 'marker', label: 'Textmarker', key: 'M', icon: '#i-marker', sep: true },
  { id: 'line', label: 'Linie', key: 'L', icon: '#i-line' },
  { id: 'arrow', label: 'Pfeil', key: 'A', icon: '#i-arrow' },
  { id: 'rect', label: 'Rechteck', key: 'R', icon: '#i-rect' },
  { id: 'ellipse', label: 'Ellipse', key: 'O', icon: '#i-ellipse', sep: true },
  { id: 'text', label: 'Text', key: 'T', icon: '#i-text' },
]
const KEYS = Object.fromEntries(TOOLS.map((t) => [t.key.toLowerCase(), t.id]))
const COLORS = [['#e5484d', 'Rot'], ['#ffd60a', 'Gelb'], ['#30a46c', 'Grün'], ['#0090ff', 'Blau'], ['#1c1c1c', 'Schwarz'], ['#ffffff', 'Weiß']] as const
const SIZES: { id: Size; label: string }[] = [{ id: 's', label: 'Dünn' }, { id: 'm', label: 'Mittel' }, { id: 'l', label: 'Dick' }]
const SCALES = [1, 0.75, 0.5, 0.25]

// ---------- Datei laden
const pages = ref<Page[]>([])
const shapes = ref<Shape[][]>([])
const failed = ref('')
const loading = ref(true)
const imgUrl = ref('')
let img: HTMLImageElement | null = null
let bytes: ArrayBuffer | null = null // Original-PDF für pdf-lib
let pdf: PDFDocumentProxy | null = null
let pdfPages: PDFPageProxy[] = []
let version = ''
let nextId = 1

async function loadPdfjs() {
  const [pdfjs, worker] = await Promise.all([import('pdfjs-dist'), import('pdfjs-dist/build/pdf.worker.min.mjs?url')])
  pdfjs.GlobalWorkerOptions.workerSrc = worker.default
  return pdfjs
}

// keepView: nach dem Speichern Zoom und Scrollposition behalten.
async function show(blob: Blob, keepView = false) {
  if (props.kind === 'image') {
    const url = URL.createObjectURL(blob)
    const next = new Image()
    next.src = url
    await next.decode()
    if (imgUrl.value) URL.revokeObjectURL(imgUrl.value)
    ;[img, imgUrl.value] = [next, url]
    pages.value = [{ w: next.naturalWidth, h: next.naturalHeight, t: [1, 0, 0, 1, 0, 0] }]
  } else {
    // ponytail: ohne cMaps, Standardschriften und WASM-Decoder von pdf.js (seltene CJK-/JPEG-2000-PDFs zeigen dann Lücken); bei Bedarf nach public/ kopieren.
    const pdfjs = await loadPdfjs()
    bytes = await blob.arrayBuffer()
    const doc = await pdfjs.getDocument({ data: new Uint8Array(bytes.slice(0)) }).promise
    const list = await Promise.all(Array.from({ length: doc.numPages }, (_, i) => doc.getPage(i + 1)))
    for (const i of tasks.keys()) cancel(i)
    void pdf?.loadingTask.destroy()
    ;[pdf, pdfPages] = [doc, list]
    rendered.clear()
    pages.value = list.map((p) => {
      const v = p.getViewport({ scale: 1 })
      return { w: v.width, h: v.height, t: v.transform }
    })
  }
  shapes.value = pages.value.map(() => [])
  hist = [snapshot()]
  histLen.value = 1
  at.value = savedAt.value = 0
  outScale.value = 1
  sel.value = editing.value = null
  await nextTick()
  if (!keepView) fit(true)
  observe()
  scheduleRender(0)
}

async function load() {
  loading.value = true
  failed.value = ''
  try {
    const f = await fetchFile(props.path)
    version = f.mtime
    await show(f.blob)
  } catch (e) {
    failed.value = e instanceof ApiError ? errorMessage(e) : `Die Datei lässt sich nicht anzeigen (${e instanceof Error ? e.message : e}).`
  } finally {
    loading.value = false
  }
}

// ---------- Verlauf (Rückgängig/Wiederholen) und Speichern
let hist: string[] = []
const histLen = ref(0)
const at = ref(0)
const savedAt = ref(0)
const outScale = ref(1) // Bild verkleinern beim Speichern
const snapshot = () => JSON.stringify(shapes.value)
const dirty = computed(() => at.value !== savedAt.value || outScale.value !== 1)
watch(dirty, (d) => emit('dirty', d))

function commit() {
  hist = hist.slice(0, at.value + 1)
  if (savedAt.value > at.value) savedAt.value = -1 // gespeicherter Stand liegt im verworfenen Zweig
  hist.push(snapshot())
  histLen.value = hist.length
  at.value = hist.length - 1
}
function go(to: number) {
  if (to < 0 || to >= hist.length) return
  commitText()
  at.value = to
  shapes.value = JSON.parse(hist[to]!)
  sel.value = null
}

// Verkleinern in Halbschritten: ein großer Sprung lässt Browser Treppen und Moiré zeichnen.
function scaled(src: HTMLImageElement, w: number, h: number) {
  let cur: CanvasImageSource = src
  let [cw, ch] = [src.naturalWidth, src.naturalHeight]
  for (;;) {
    const half = cw / 2 >= w && ch / 2 >= h
    const c = document.createElement('canvas')
    ;[c.width, c.height] = half ? [Math.round(cw / 2), Math.round(ch / 2)] : [w, h]
    const ctx = c.getContext('2d')!
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(cur, 0, 0, c.width, c.height)
    ;[cur, cw, ch] = [c, c.width, c.height]
    if (!half) return c
  }
}

async function imageBlob(): Promise<Blob> {
  const pg = pages.value[0]!
  const [w, h] = [Math.max(1, Math.round(pg.w * outScale.value)), Math.max(1, Math.round(pg.h * outScale.value))]
  const c = scaled(img!, w, h)
  const ctx = c.getContext('2d')!
  ctx.scale(w / pg.w, h / pg.h)
  drawOnCanvas(ctx, shapes.value[0]!)
  const blob = await new Promise<Blob | null>((res) => c.toBlob(res, imageType, 0.92))
  if (!blob || blob.type !== imageType) throw new Error(`Dieser Browser kann ${imageType} nicht schreiben.`)
  return blob
}

async function save(force = false) {
  commitText()
  let blob: Blob
  if (props.kind === 'pdf') {
    try {
      blob = new Blob([(await drawOnPdf(await import('pdf-lib'), bytes!, pages.value, shapes.value)) as BlobPart], { type: 'application/pdf' })
    } catch {
      throw new Error('Kairo kann diese PDF nicht bearbeiten (verschlüsselt oder beschädigt).')
    }
  } else blob = await imageBlob()
  const resized = outScale.value !== 1
  version = (await replaceFile(props.path, blob, version, force)).mtime
  await show(blob, !resized)
}

defineExpose({ save, reload: load })

// ---------- Zoom
const scroller = ref<HTMLElement>()
const zoom = ref(1) // CSS-Pixel pro Seiteneinheit
const base = props.kind === 'pdf' ? 96 / 72 : 1 / window.devicePixelRatio // 100 %: PDF in Druckgröße, Bild Pixel für Pixel
const pct = computed(() => Math.round((zoom.value / base) * 100))
const STEPS = [10, 25, 33, 50, 67, 75, 90, 100, 125, 150, 200, 300, 400, 600, 800]

// Zoomt um einen Bildschirmpunkt (Mauszeiger, sonst Mitte); der Punkt auf der Seite bleibt dabei unter dem Zeiger.
function zoomTo(z: number, cx?: number, cy?: number) {
  const el = scroller.value
  if (!el) return
  z = Math.min(Math.max(z, 0.1 * base), 8 * base)
  const r = el.getBoundingClientRect()
  ;[cx, cy] = [cx ?? r.left + el.clientWidth / 2, cy ?? r.top + el.clientHeight / 2]
  const pageEls = [...el.querySelectorAll<HTMLElement>('.fe-page')]
  const anchor = pageEls.find((p) => p.getBoundingClientRect().bottom >= cy!) ?? pageEls.at(-1)
  const ar = anchor?.getBoundingClientRect()
  const pt = ar && [(cx - ar.left) / zoom.value, (cy - ar.top) / zoom.value]
  zoom.value = z
  void nextTick(() => {
    if (!anchor || !pt) return
    const nr = anchor.getBoundingClientRect()
    el.scrollLeft += nr.left + pt[0]! * z - cx!
    el.scrollTop += nr.top + pt[1]! * z - cy!
  })
  scheduleRender()
}

function step(dir: 1 | -1) {
  const p = pct.value
  const next = dir > 0 ? STEPS.find((s) => s > p + 0.5) ?? 800 : [...STEPS].reverse().find((s) => s < p - 0.5) ?? 10
  zoomTo((next / 100) * base)
}

// PDF: Seitenbreite; Bild: ganz sichtbar, aber nie größer als 100 %.
function fit(top = false) {
  const el = scroller.value
  if (!el || !pages.value.length) return
  const [w, h] = [el.clientWidth - 48, el.clientHeight - 48]
  const first = pages.value[0]!
  const z = props.kind === 'pdf' ? w / Math.max(...pages.value.map((p) => p.w)) : Math.min(w / first.w, h / first.h, base)
  zoomTo(z)
  if (top) void nextTick(() => el.scrollTo(0, 0))
}

// ⌘/Strg + Mausrad und Trackpad-Pinch (Chrome/Firefox); Safari meldet Pinch als gesture*-Ereignisse.
let gesture = 0
function onWheel(e: WheelEvent) {
  if (!(e.ctrlKey || e.metaKey) || gesture) return
  e.preventDefault()
  zoomTo(zoom.value * Math.exp(-Math.max(-25, Math.min(25, e.deltaY)) * 0.01), e.clientX, e.clientY)
}
type GestureEvent = UIEvent & { scale: number; clientX: number; clientY: number }
function onGesture(e: Event) {
  e.preventDefault()
  const g = e as GestureEvent
  if (e.type === 'gesturestart') gesture = zoom.value
  else if (e.type === 'gestureend') gesture = 0
  else if (gesture) zoomTo(gesture * g.scale, g.clientX, g.clientY)
}

// ---------- PDF-Seiten rendern: nur sichtbare (±1 Bildschirm), scharf für Zoom × Pixeldichte
const canvases: (HTMLCanvasElement | null)[] = []
const setCanvas = (i: number) => (el: unknown) => (canvases[i] = el as HTMLCanvasElement | null)
const rendered = new Map<number, number>() // Seite → gerenderte Skala
const tasks = new Map<number, RenderTask>()
const visible = new Set<number>()
let observer: IntersectionObserver | null = null
let renderTimer = 0
const MAX_PIXELS = 16e6 // größte Canvas-Fläche, die Safari noch zeichnet

function observe() {
  observer?.disconnect()
  if (props.kind !== 'pdf' || !scroller.value) return
  observer = new IntersectionObserver((entries) => {
    for (const en of entries) {
      const i = Number((en.target as HTMLElement).dataset.i)
      if (en.isIntersecting) visible.add(i)
      else {
        visible.delete(i)
        free(i)
      }
    }
    scheduleRender(0)
  }, { root: scroller.value, rootMargin: '100% 0px' })
  scroller.value.querySelectorAll('.fe-page').forEach((el) => observer!.observe(el))
}

function scheduleRender(delay = 150) {
  clearTimeout(renderTimer)
  renderTimer = window.setTimeout(() => visible.forEach((i) => void renderPage(i)), delay)
}

async function renderPage(i: number) {
  const [page, canvas, pg] = [pdfPages[i], canvases[i], pages.value[i]]
  if (!page || !canvas || !pg) return
  const scale = Math.min(zoom.value * window.devicePixelRatio, Math.sqrt(MAX_PIXELS / (pg.w * pg.h)))
  if (Math.abs((rendered.get(i) ?? 0) - scale) < 1e-3) return
  cancel(i)
  // Erst offscreen rendern, dann umkopieren: beim Zoomen bleibt das alte Bild stehen, statt weiß zu blitzen.
  const off = document.createElement('canvas')
  const viewport = page.getViewport({ scale })
  ;[off.width, off.height] = [Math.ceil(viewport.width), Math.ceil(viewport.height)]
  const task = page.render({ canvas: off, viewport })
  tasks.set(i, task)
  try {
    await task.promise
  } catch {
    return // abgebrochen
  }
  if (tasks.get(i) !== task) return
  tasks.delete(i)
  ;[canvas.width, canvas.height] = [off.width, off.height]
  canvas.getContext('2d')!.drawImage(off, 0, 0)
  rendered.set(i, scale)
}

function cancel(i: number) {
  tasks.get(i)?.cancel()
  tasks.delete(i)
}
function free(i: number) {
  cancel(i)
  const c = canvases[i]
  if (c) c.width = c.height = 0
  rendered.delete(i)
}

// ---------- Werkzeuge
const tool = ref<Tool>('select')
const color = ref<string>(COLORS[0][0])
const markerColor = ref<string>(COLORS[1][0])
const size = ref<Size>('m')
const sel = ref<{ page: number; id: number } | null>(null)
const draft = ref<{ page: number; shape: Shape } | null>(null)

const find = (page: number, id: number) => shapes.value[page]?.find((s) => s.id === id)
const selShape = computed(() => (sel.value ? find(sel.value.page, sel.value.id) : undefined))
const activeColor = computed(() => selShape.value?.color ?? (tool.value === 'marker' ? markerColor.value : color.value))

const measureCtx = document.createElement('canvas').getContext('2d')!
function measure(text: string, px: number) {
  measureCtx.font = `${px}px ${FONT}`
  return measureCtx.measureText(text).width
}

function pickTool(t: Tool) {
  commitText()
  tool.value = t
  if (t !== 'select') sel.value = null
}

// Farbe und Stärke gelten für das nächste Zeichnen und, falls etwas gewählt ist, auch dafür.
function pickColor(c: string) {
  const s = selShape.value
  ;(s?.kind === 'marker' || (!s && tool.value === 'marker') ? markerColor : color).value = c
  if (s && s.color !== c) {
    s.color = c
    commit()
  }
}
function pickSize(z: Size) {
  size.value = z
  const s = selShape.value
  if (!s || !sel.value) return
  const v = sizeFor(s.kind, z, pages.value[sel.value.page]!)
  if (s.kind === 'text') s.size = v
  else s.width = v
  commit()
}

function removeSelected() {
  if (!sel.value) return
  const list = shapes.value[sel.value.page]!
  list.splice(list.findIndex((s) => s.id === sel.value!.id), 1)
  sel.value = null
  commit()
}

function newShape(t: Exclude<Tool, 'select' | 'text'>, p: Pt, page: number): Stroke {
  const base = { id: nextId++, color: t === 'marker' ? markerColor.value : color.value, width: sizeFor(t, size.value, pages.value[page]!) }
  if (t === 'pen' || t === 'marker') return { ...base, kind: t, pts: [p] }
  if (t === 'line' || t === 'arrow') return { ...base, kind: t, a: p, b: p }
  return { ...base, kind: t, x: p[0], y: p[1], w: 0, h: 0 }
}

// ---------- Zeiger: zeichnen, wählen, verschieben, skalieren
type Drag =
  | { mode: 'draw'; page: number; start: Pt }
  | { mode: 'move'; page: number; start: Pt; orig: Shape; box: Box }
  | { mode: 'resize'; page: number; corner: number; grab: Pt; orig: Shape; box: Box }
  | { mode: 'end'; page: number; end: 'a' | 'b'; grab: Pt; orig: Shape & { a: Pt; b: Pt } }
let drag: Drag | null = null
const clone = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) // Formen sind reaktive Proxys; structuredClone mag die nicht
let changed = false

function pointOf(e: MouseEvent, svg: Element): Pt {
  const r = svg.getBoundingClientRect()
  return [(e.clientX - r.left) / zoom.value, (e.clientY - r.top) / zoom.value]
}

function down(e: PointerEvent, page: number) {
  if (e.button !== 0 || !editable) return
  const svg = e.currentTarget as SVGSVGElement
  const p = pointOf(e, svg)
  const target = e.target as Element
  const handle = target.closest('[data-h]')?.getAttribute('data-h')
  const hit = find(page, Number(target.closest('[data-id]')?.getAttribute('data-id')))
  const wasEditing = !!editing.value
  commitText()
  e.preventDefault() // kein Fokuswechsel und keine Textauswahl beim Ziehen
  if (tool.value === 'text') {
    if (wasEditing) return // erster Klick daneben beendet nur das Schreiben
    if (hit?.kind === 'text') return startEdit(page, hit.id)
    const px = sizeFor('text', size.value, pages.value[page]!)
    const s: Text = { id: nextId++, kind: 'text', color: color.value, size: px, x: p[0], y: p[1] - (px * LINE) / 2, text: '' }
    shapes.value[page]!.push(s)
    return startEdit(page, s.id, true)
  }
  if (tool.value === 'select') {
    const s = selShape.value
    if (handle && s && sel.value?.page === page) {
      if ((handle === 'a' || handle === 'b') && (s.kind === 'line' || s.kind === 'arrow')) {
        drag = { mode: 'end', page, end: handle, grab: [s[handle][0] - p[0], s[handle][1] - p[1]], orig: clone(s) }
      } else {
        const box = bounds(s, measure)
        const c = corners(box)[Number(handle)]!
        drag = { mode: 'resize', page, corner: Number(handle), grab: [c[0] - p[0], c[1] - p[1]], orig: clone(s), box }
      }
    } else if (hit) {
      sel.value = { page, id: hit.id }
      drag = { mode: 'move', page, start: p, orig: clone(hit), box: bounds(hit, measure) }
    } else {
      sel.value = null
      return
    }
  } else {
    sel.value = null
    draft.value = { page, shape: newShape(tool.value, p, page) }
    drag = { mode: 'draw', page, start: p }
  }
  changed = false
  svg.setPointerCapture(e.pointerId)
}

function move(e: PointerEvent, page: number) {
  if (!drag || drag.page !== page) return
  const svg = e.currentTarget as Element
  const p = pointOf(e, svg)
  if (drag.mode === 'draw') {
    const s = draft.value!.shape
    if (s.kind === 'pen' || s.kind === 'marker') {
      for (const ce of e.getCoalescedEvents?.() ?? [e]) { // alle Zwischenpunkte: glatte Striche auch bei schnellen Bewegungen
        const q = pointOf(ce, svg), last = s.pts[s.pts.length - 1]!
        if (Math.hypot(q[0] - last[0], q[1] - last[1]) * zoom.value >= 1.5) s.pts.push(q)
      }
    } else if (s.kind === 'line' || s.kind === 'arrow') s.b = e.shiftKey ? snapped(s.a, p) : p
    else if (s.kind === 'rect' || s.kind === 'ellipse') Object.assign(s, boxFrom(drag.start, p, e.shiftKey))
    return
  }
  const d = drag
  const list = shapes.value[page]!
  const i = list.findIndex((s) => s.id === d.orig.id)
  if (i < 0) return
  changed = true
  if (d.mode === 'move') {
    list[i] = transformed(d.orig, d.box, { ...d.box, x: d.box.x + p[0] - d.start[0], y: d.box.y + p[1] - d.start[1] })
  } else if (d.mode === 'end') {
    const q: Pt = [p[0] + d.grab[0], p[1] + d.grab[1]]
    const other = d.orig[d.end === 'a' ? 'b' : 'a']
    list[i] = { ...d.orig, [d.end]: e.shiftKey ? snapped(other, q) : q }
  } else {
    const q: Pt = [p[0] + d.grab[0], p[1] + d.grab[1]]
    list[i] = transformed(d.orig, d.box, resizeBox(d.box, d.corner, q, e.shiftKey || d.orig.kind === 'text'))
  }
}

function up(page: number) {
  if (!drag || drag.page !== page) return
  const d = drag
  drag = null
  if (d.mode !== 'draw') {
    if (changed) commit()
    return
  }
  const s = draft.value!.shape
  draft.value = null
  const b = bounds(s, measure)
  if (Math.max(b.w, b.h) * zoom.value < 3 && s.kind !== 'pen' && s.kind !== 'marker') return // Klick ohne Ziehen
  shapes.value[page]!.push(s)
  commit()
}

function dblclick(e: MouseEvent, page: number) {
  const hit = find(page, Number((e.target as Element).closest('[data-id]')?.getAttribute('data-id')))
  if (editable && hit?.kind === 'text') startEdit(page, hit.id)
}

// Formen einer Seite, dazu die gerade gezeichnete.
const inkOf = (page: number) => (draft.value?.page === page ? [...shapes.value[page]!, draft.value.shape] : shapes.value[page]!)
const markersOf = (page: number) => inkOf(page).filter((s) => s.kind === 'marker') as Stroke[]

// Auswahlrahmen etwas größer als die Form, Griffe immer gleich groß auf dem Bildschirm.
const selBox = computed(() => {
  const s = selShape.value
  if (!s || s.kind === 'line' || s.kind === 'arrow') return null
  const pad = (s.kind === 'text' ? 0 : s.width / 2) + 4 / zoom.value
  const b = bounds(s, measure)
  return { box: { x: b.x - pad, y: b.y - pad, w: b.w + 2 * pad, h: b.h + 2 * pad }, handles: corners(b) }
})
const handleSize = computed(() => 9 / zoom.value)

// ---------- Text schreiben
const editing = ref<{ page: number; id: number } | null>(null)
const editText = ref('')
let editFresh = false
let textarea: HTMLTextAreaElement | null = null
const setTextarea = (el: unknown) => (textarea = el as HTMLTextAreaElement | null)
const editShape = computed(() => (editing.value ? (find(editing.value.page, editing.value.id) as Text | undefined) : undefined))

function startEdit(page: number, id: number, fresh = false) {
  const s = find(page, id) as Text
  editing.value = { page, id }
  editText.value = s.text
  editFresh = fresh
  sel.value = null
  void nextTick(() => textarea?.focus())
}

function commitText() {
  const ed = editing.value
  if (!ed) return
  editing.value = null
  const list = shapes.value[ed.page]!
  const i = list.findIndex((s) => s.id === ed.id)
  const s = list[i] as Text | undefined
  if (!s) return
  const text = editText.value.replace(/\s+$/, '')
  if (!text.trim()) {
    list.splice(i, 1)
    if (!editFresh) commit() // vorhandener Text geleert = gelöscht
  } else if (text !== s.text) {
    s.text = text
    commit()
  }
}

const taStyle = computed(() => {
  const s = editShape.value
  if (!s) return {}
  const lines = editText.value.split('\n')
  const w = Math.max(...lines.map((l) => measure(l, s.size)), s.size * 2) + s.size
  return {
    left: `${s.x * zoom.value}px`, top: `${s.y * zoom.value}px`, width: `${w * zoom.value}px`, height: `${lines.length * LINE * s.size * zoom.value}px`,
    fontSize: `${s.size * zoom.value}px`, lineHeight: String(LINE), color: s.color, fontFamily: FONT,
  }
})

// ---------- Tastatur
function onKey(e: KeyboardEvent) {
  if ((e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')) return
  const k = e.key.toLowerCase()
  if (e.metaKey || e.ctrlKey) {
    const act = { '+': () => step(1), '=': () => step(1), '-': () => step(-1), '0': () => fit(), z: () => go(at.value + (e.shiftKey ? 1 : -1)), y: () => go(at.value + 1) }[k]
    if (act && (editable || !'zy'.includes(k))) {
      e.preventDefault()
      act()
    }
    return
  }
  if (e.altKey || !editable) return
  if ((k === 'backspace' || k === 'delete') && sel.value) {
    e.preventDefault()
    removeSelected()
  } else if (k === 'escape') {
    sel.value = null
    tool.value = 'select'
  } else {
    const t = KEYS[k]
    if (t) pickTool(t)
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  const el = scroller.value!
  el.addEventListener('wheel', onWheel, { passive: false })
  for (const t of ['gesturestart', 'gesturechange', 'gestureend']) el.addEventListener(t, onGesture)
  void load()
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  observer?.disconnect()
  clearTimeout(renderTimer)
  for (const i of tasks.keys()) cancel(i)
  void pdf?.loadingTask.destroy()
  if (imgUrl.value) URL.revokeObjectURL(imgUrl.value)
})
</script>

<template>
  <div class="fe">
    <div class="fe-bar" role="toolbar" aria-label="Bearbeiten">
      <template v-if="editable">
        <template v-for="t in TOOLS" :key="t.id">
          <button
            type="button" class="icon-btn" :class="{ on: tool === t.id }" :aria-pressed="tool === t.id"
            :aria-label="t.label" :data-tip="`${t.label} (${t.key})`" @click="pickTool(t.id)"
          ><svg class="ic"><use :href="t.icon" /></svg></button>
          <i v-if="t.sep" class="sep" />
        </template>
        <i class="sep" />
        <button
          v-for="[c, name] in COLORS" :key="c" type="button" class="fe-sw" :class="{ on: activeColor === c }"
          :aria-pressed="activeColor === c" :aria-label="name" :data-tip="name" @click="pickColor(c)"
        ><i :style="{ background: c }" /></button>
        <i class="sep" />
        <button
          v-for="(z, k) in SIZES" :key="z.id" type="button" class="icon-btn fe-dot" :class="{ on: size === z.id }"
          :aria-pressed="size === z.id" :aria-label="z.label" :data-tip="z.label" @click="pickSize(z.id)"
        ><i :style="{ width: `${4 + k * 3}px`, height: `${4 + k * 3}px` }" /></button>
        <i class="sep" />
        <button type="button" class="icon-btn" aria-label="Rückgängig" data-tip="Rückgängig (⌘Z)" :disabled="at === 0" @click="go(at - 1)"><svg class="ic"><use href="#i-undo" /></svg></button>
        <button type="button" class="icon-btn" aria-label="Wiederholen" data-tip="Wiederholen (⇧⌘Z)" :disabled="at >= histLen - 1" @click="go(at + 1)"><svg class="ic"><use href="#i-redo" /></svg></button>
        <button type="button" class="icon-btn" aria-label="Auswahl löschen" data-tip="Löschen (⌫)" :disabled="!sel" @click="removeSelected"><svg class="ic"><use href="#i-trash" /></svg></button>
        <template v-if="kind === 'image' && pages[0]">
          <i class="sep" />
          <label class="fe-scale" :data-tip="`Beim Speichern: ${Math.round(pages[0].w * outScale)} × ${Math.round(pages[0].h * outScale)} px`">
            <span>Größe</span>
            <select v-model.number="outScale" aria-label="Größe beim Speichern">
              <option v-for="k in SCALES" :key="k" :value="k">{{ k * 100 }} %</option>
            </select>
          </label>
        </template>
      </template>
      <span class="fe-zoom">
        <button type="button" class="icon-btn" aria-label="Verkleinern" data-tip="Verkleinern (⌘−)" @click="step(-1)"><svg class="ic"><use href="#i-minus" /></svg></button>
        <button type="button" class="fe-pct" aria-label="Einpassen" data-tip="Einpassen (⌘0)" @click="fit()">{{ pct }} %</button>
        <button type="button" class="icon-btn" aria-label="Vergrößern" data-tip="Vergrößern (⌘+)" @click="step(1)"><svg class="ic"><use href="#i-plus" /></svg></button>
        <template v-if="kind === 'pdf'">
          <i class="sep" />
          <a class="icon-btn" :href="filesUrl(path)" target="_blank" rel="noopener" aria-label="Im PDF-Viewer des Browsers öffnen" data-tip="Im PDF-Viewer öffnen (Suche, Drucken)"><svg class="ic"><use href="#i-external" /></svg></a>
        </template>
      </span>
    </div>

    <div ref="scroller" class="fe-scroll">
      <p v-if="failed" class="fe-msg" role="alert">{{ failed }}</p>
      <p v-else-if="loading && !pages.length" class="fe-msg">Lädt …</p>
      <div class="fe-pages" :class="{ single: kind === 'image' }">
        <div
          v-for="(pg, i) in pages" :key="i" class="fe-page" :data-i="i"
          :style="{ width: `${pg.w * zoom}px`, height: `${pg.h * zoom}px` }"
        >
          <img v-if="kind === 'image'" :src="imgUrl" alt="" draggable="false" />
          <canvas v-else :ref="setCanvas(i)" />
          <!-- Textmarker liegen in einer eigenen Ebene, die multipliziert: Schrift darunter bleibt schwarz. -->
          <svg class="fe-mark" :viewBox="`0 0 ${pg.w} ${pg.h}`" aria-hidden="true">
            <path v-for="s in markersOf(i)" :key="s.id" :d="pathOf(s)" :stroke="s.color" :stroke-width="s.width" />
          </svg>
          <svg
            class="fe-ink" :class="`t-${editable ? tool : 'none'}`" :viewBox="`0 0 ${pg.w} ${pg.h}`"
            @pointerdown="down($event, i)" @pointermove="move($event, i)" @pointerup="up(i)" @pointercancel="up(i)" @dblclick="dblclick($event, i)"
          >
            <g v-for="s in inkOf(i)" :key="s.id" :data-id="s.id" :class="{ hidden: editing?.id === s.id }">
              <template v-if="s.kind === 'text'">
                <rect class="hit" v-bind="bounds(s, measure)" />
                <text :fill="s.color" :font-size="s.size" :font-family="FONT">
                  <tspan v-for="(l, k) in linesOf(s)" :key="k" :x="s.x" :y="baseline(s, k)">{{ l || ' ' }}</tspan>
                </text>
              </template>
              <template v-else>
                <path class="hit" :d="pathOf(s)" :stroke-width="Math.max(s.width, 12 / zoom)" />
                <path v-if="s.kind !== 'marker'" :d="pathOf(s)" :stroke="s.color" :stroke-width="s.width" />
              </template>
            </g>
            <g v-if="sel?.page === i && selShape" class="sel">
              <template v-if="selShape.kind === 'line' || selShape.kind === 'arrow'">
                <rect v-for="h in (['a', 'b'] as const)" :key="h" class="handle move" :data-h="h" :x="selShape[h][0] - handleSize / 2" :y="selShape[h][1] - handleSize / 2" :width="handleSize" :height="handleSize" />
              </template>
              <template v-else-if="selBox">
                <rect class="box" v-bind="selBox.box" />
                <rect
                  v-for="(c, k) in selBox.handles" :key="k" class="handle" :class="k % 2 ? 'nesw' : 'nwse'" :data-h="k"
                  :x="c[0] - handleSize / 2" :y="c[1] - handleSize / 2" :width="handleSize" :height="handleSize"
                />
              </template>
            </g>
          </svg>
          <textarea
            v-if="editing?.page === i" :ref="setTextarea" v-model="editText" class="fe-ta" :style="taStyle"
            spellcheck="false" aria-label="Text" @blur="commitText" @keydown.esc.prevent="commitText" @keydown.meta.enter.prevent="commitText"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fe { flex: 1; min-height: 0; display: flex; flex-direction: column; }
/* wie die Werkzeugleiste des Markdown-Editors */
.fe-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 2px; min-height: 40px; padding: 4px 16px; border-bottom: 1px solid var(--br-subtle); background: var(--bg-0); }
.fe-bar [data-tip]:hover::after { bottom: auto; top: calc(100% + 8px); } /* Tooltip unter der Leiste, nicht über dem Pfad */
.fe-bar .icon-btn.on { background: var(--bg-selected); color: var(--a-blue-hi); }
.fe-bar .icon-btn:disabled { color: var(--tx-disabled); background: none; cursor: default; }
.sep { flex: none; width: 1px; height: 16px; margin: 0 6px; background: var(--br-default); }
.fe-sw { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 50%; transition: box-shadow var(--dur) ease-out; }
.fe-sw i { width: 12px; height: 12px; border-radius: 50%; box-shadow: inset 0 0 0 1px rgb(127 127 127 / .45); }
.fe-sw:hover { box-shadow: 0 0 0 1px var(--br-strong); }
.fe-sw.on { box-shadow: 0 0 0 1.5px var(--tx-primary); }
.fe-dot i { border-radius: 50%; background: currentColor; }
.fe-scale { display: inline-flex; align-items: center; gap: 6px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.fe-scale select { height: 26px; padding: 0 6px; border: 1px solid var(--br-default); border-radius: var(--r-s); background: var(--bg-1); color: var(--tx-primary); font: 400 12px/1 var(--font-ui); font-variant-numeric: tabular-nums; }
.fe-zoom { display: flex; align-items: center; gap: 2px; margin-left: auto; } /* bricht die Leiste um, bleibt der Zoom rechts */
.fe-pct { min-width: 52px; height: 28px; border-radius: var(--r-s); font: 400 12px/1 var(--font-mono); font-variant-numeric: tabular-nums; color: var(--tx-secondary); transition: background var(--dur) ease-out, color var(--dur) ease-out; }
.fe-pct:hover { background: var(--bg-2); color: var(--tx-primary); }

.fe-scroll { position: relative; flex: 1; min-height: 0; overflow: auto; background: var(--bg-2); }
.fe-pages { display: flex; flex-direction: column; align-items: center; gap: 16px; box-sizing: border-box; width: max-content; min-width: 100%; padding: 24px; }
.fe-pages.single { min-height: 100%; justify-content: center; }
.fe-page { position: relative; flex: none; background: #fff; box-shadow: 0 0 0 1px var(--br-default), 0 2px 12px rgb(0 0 0 / .18); }
.single .fe-page { background: none; }
.fe-page > canvas, .fe-page > img { position: absolute; inset: 0; display: block; width: 100%; height: 100%; user-select: none; -webkit-user-drag: none; }
.fe-msg { position: absolute; inset: 0; display: grid; place-items: center; padding: 24px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); text-align: center; }

.fe-mark, .fe-ink { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.fe-mark { mix-blend-mode: multiply; pointer-events: none; }
.fe-mark path { opacity: .4; }
.fe-mark path, .fe-ink path { fill: none; stroke-linecap: round; stroke-linejoin: round; }
.fe-ink:not(.t-select):not(.t-none) { touch-action: none; cursor: crosshair; }
.fe-ink.t-text { cursor: text; }
.fe-ink .hit { stroke: transparent; fill: transparent; pointer-events: none; }
.fe-ink.t-select .hit, .fe-ink.t-text rect.hit { pointer-events: all; }
.fe-ink.t-select path.hit { pointer-events: stroke; }
.fe-ink.t-select .hit { cursor: move; }
.fe-ink text { white-space: pre; pointer-events: none; user-select: none; }
.fe-ink g.hidden { visibility: hidden; }
.sel .box { fill: none; stroke: var(--a-blue-hi); stroke-width: 1; stroke-dasharray: 4 3; vector-effect: non-scaling-stroke; pointer-events: none; }
.sel .handle { fill: #fff; stroke: var(--a-blue-hi); stroke-width: 1.5; vector-effect: non-scaling-stroke; }
.sel .nwse { cursor: nwse-resize; }
.sel .nesw { cursor: nesw-resize; }
.sel .move { cursor: move; }
.fe-ta { position: absolute; z-index: 1; box-sizing: content-box; margin: 0; padding: 0; border: 0; outline: 1px dashed var(--a-blue-hi); outline-offset: 3px; background: transparent; resize: none; overflow: hidden; white-space: pre; }
</style>
