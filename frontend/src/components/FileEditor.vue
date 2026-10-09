<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ApiError, errorMessage, fetchFile, replaceFile } from '@/api/client'
import {
  COLORS, FONT, LINE, TOOL_KEYS, baseline, bounds, boxFrom, corners, linesOf, pathOf, resizeBox, sizeFor, snapped, transformed, zoomStep,
  type Box, type Page, type Pt, type Shape, type Size, type Stroke, type Text, type Tool,
} from '@/lib/annotate'
import { imageBlob, pdfBlob } from '@/lib/fileSave'
import { store } from '@/lib/storage'
import { t } from '@/lib/i18n'
import { useHistory } from '@/composables/useHistory'
import { usePdfRender } from '@/composables/usePdfRender'
import FileEditorBar from '@/components/FileEditorBar.vue'

// PDFs und Bilder ansehen und bezeichnen: Stift, Textmarker, Linie, Pfeil, Rechteck, Ellipse, Text; Zoom; Bilder auch verkleinern.
// Bis zum Speichern bleibt jede Form einzeln wählbar; Speichern brennt sie in die Datei ein (die alte Fassung landet in .trash).
// active: sichtbar und zuletzt benutzt; nur dann gelten die Tastenkürzel (geteilte Ansicht: die Notiz daneben tippt sonst Werkzeuge).
const props = defineProps<{ path: string; kind: 'pdf' | 'image'; active: boolean }>()
const emit = defineEmits<{ dirty: [boolean] }>()

const IMAGE_TYPES: Record<string, string> = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp' }
const imageType = IMAGE_TYPES[props.path.split('.').pop()!.toLowerCase()]
const editable = props.kind === 'pdf' || !!imageType // GIF kann der Browser nicht schreiben: nur ansehen


// ---------- Datei laden
const pages = ref<Page[]>([])
const shapes = ref<Shape[][]>([])
const failed = ref('')
const loading = ref(true)
const imgUrl = ref('')
let img: HTMLImageElement | null = null
let bytes: ArrayBuffer | null = null // Original-PDF für pdf-lib
let version = ''
let nextId = 1

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
    bytes = await blob.arrayBuffer()
    pages.value = await pdfRender.open(bytes)
  }
  shapes.value = pages.value.map(() => [])
  history.reset()
  outScale.value = 1
  sel.value = editing.value = null
  await nextTick()
  if (!keepView) {
    needsView = true
    applyView()
  }
  pdfRender.observe()
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
    failed.value = e instanceof ApiError ? errorMessage(e) : t('Die Datei lässt sich nicht anzeigen ({reason}).', { reason: e instanceof Error ? e.message : String(e) })
  } finally {
    loading.value = false
  }
}

// ---------- Verlauf (Rückgängig/Wiederholen) und Speichern
const history = useHistory(() => shapes.value, (v) => (shapes.value = v))
const { at, len: histLen, commit } = history
const outScale = ref(1) // Bild verkleinern beim Speichern
const dirty = computed(() => history.changed.value || outScale.value !== 1)
watch(dirty, (d) => emit('dirty', d))

function go(to: number) {
  commitText()
  if (history.go(to)) sel.value = null
}

async function save(force = false) {
  commitText()
  const blob = props.kind === 'pdf'
    ? await pdfBlob(bytes!, pages.value, shapes.value)
    : await imageBlob(img!, pages.value[0]!, shapes.value[0]!, outScale.value, imageType!)
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

// Zoomt um einen Bildschirmpunkt (Mauszeiger, sonst Mitte); der Punkt auf der Seite bleibt dabei unter dem Zeiger.
// Selbst gezoomt (user) heißt: nicht mehr automatisch an die Breite anpassen.
function zoomTo(z: number, cx?: number, cy?: number, user = true) {
  const el = scroller.value
  if (!el) return
  if (user) fitted = false
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

const step = (dir: 1 | -1) => zoomTo((zoomStep(pct.value, dir) / 100) * base)

// PDF: Seitenbreite; Bild: ganz sichtbar, aber nie größer als 100 %.
function fit(top = false) {
  const el = scroller.value
  if (!el?.clientWidth || !pages.value.length) return // ausgeblendet: beim Einblenden passt der ResizeObserver an
  const [w, h] = [el.clientWidth - 48, el.clientHeight - 48]
  const first = pages.value[0]!
  const z = props.kind === 'pdf' ? w / Math.max(...pages.value.map((p) => p.w)) : Math.min(w / first.w, h / first.h, base)
  zoomTo(z, undefined, undefined, false)
  fitted = true
  if (top) void nextTick(() => el.scrollTo(0, 0))
}

// ---------- Ausschnitt merken: je Datei Zoom und Scrollposition, damit man nach einem Wechsel dort weiterliest
const VIEWS = 'kairo-notes-views'
type View = { z: number; x: number; y: number } // z = 0: eingepasst; x, y als Anteil, passt so auch bei anderer Fensterbreite
let fitted = true // Zoom folgt der Breite, bis man selbst zoomt
let needsView = false // nach dem Laden: gemerkten Ausschnitt anwenden, sobald der Editor sichtbar ist
let lastView: { x: number; y: number } | null = null
let viewTimer = 0
let wasHidden = false

function readViews(): Record<string, View> {
  try { return JSON.parse(store.get(VIEWS) ?? '{}') } catch { return {} }
}
function saveView() {
  if (!lastView) return
  const all = readViews()
  delete all[props.path] // ans Ende: von den 50 zuletzt benutzten bleiben die neuesten
  all[props.path] = { z: fitted ? 0 : zoom.value, ...lastView }
  store.set(VIEWS, JSON.stringify(Object.fromEntries(Object.entries(all).slice(-50))))
}
function onScroll() {
  const el = scroller.value
  if (!el?.clientWidth) return // ausgeblendet: der Browser setzt die Position zurück, die alte gilt weiter
  lastView = { x: el.scrollLeft / el.scrollWidth, y: el.scrollTop / el.scrollHeight }
  clearTimeout(viewTimer)
  viewTimer = window.setTimeout(saveView, 300)
}
function scrollToView(v: { x: number; y: number }) {
  const el = scroller.value!
  void nextTick(() => {
    el.scrollTop = v.y * el.scrollHeight
    el.scrollLeft = v.x * el.scrollWidth
  })
}
function applyView() {
  const el = scroller.value
  if (!needsView || !el?.clientWidth || !pages.value.length) return
  needsView = false
  const v = readViews()[props.path]
  if (!v) return fit(true)
  if (v.z) {
    zoom.value = v.z
    fitted = false
    scheduleRender(0)
  } else fit()
  scrollToView(v)
}
// Breite ändert sich (geteilte Ansicht, Trenner, Fenster) oder der Editor wird wieder eingeblendet.
const resize = new ResizeObserver(() => {
  if (!scroller.value?.clientWidth) {
    wasHidden = true
    return
  }
  if (needsView) return applyView()
  if (fitted) fit()
  if (wasHidden && lastView) scrollToView(lastView)
  wasHidden = false
})
watch(zoom, onScroll)

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

// ---------- PDF-Seiten rendern
const pdfRender = usePdfRender(zoom, pages, scroller)
const { scheduleRender, setCanvas } = pdfRender

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
  if (!props.active || (e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')) return
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
    const t = TOOL_KEYS[k]
    if (t) pickTool(t)
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  const el = scroller.value!
  el.addEventListener('wheel', onWheel, { passive: false })
  for (const t of ['gesturestart', 'gesturechange', 'gestureend']) el.addEventListener(t, onGesture)
  resize.observe(el)
  void load()
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  clearTimeout(viewTimer)
  saveView()
  resize.disconnect()
  pdfRender.dispose()
  if (imgUrl.value) URL.revokeObjectURL(imgUrl.value)
})
</script>

<template>
  <div class="fe">
    <FileEditorBar
      v-model:out-scale="outScale" :kind="kind" :path="path" :editable="editable" :tool="tool" :color="activeColor" :size="size"
      :can-undo="at > 0" :can-redo="at < histLen - 1" :can-remove="!!sel" :page="pages[0]" :pct="pct"
      @tool="pickTool" @color="pickColor" @size="pickSize" @undo="go(at - 1)" @redo="go(at + 1)" @remove="removeSelected" @zoom="step" @fit="fit()"
    />

    <div ref="scroller" class="fe-scroll" @scroll.passive="onScroll">
      <p v-if="failed" class="fe-msg" role="alert">{{ failed }}</p>
      <p v-else-if="loading && !pages.length" class="fe-msg">{{ $t('Lädt …') }}</p>
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
            spellcheck="false" :aria-label="$t('Text')" @blur="commitText" @keydown.esc.prevent="commitText" @keydown.meta.enter.prevent="commitText"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fe { flex: 1; min-height: 0; display: flex; flex-direction: column; }

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
