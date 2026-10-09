import type { Ref } from 'vue'
import type { PDFDocumentProxy, PDFPageProxy, RenderTask } from 'pdfjs-dist'
import type { Page } from '@/lib/annotate'

const MAX_PIXELS = 16e6 // größte Canvas-Fläche, die Safari noch zeichnet

async function loadPdfjs() {
  const [pdfjs, worker] = await Promise.all([import('pdfjs-dist'), import('pdfjs-dist/build/pdf.worker.min.mjs?url')])
  pdfjs.GlobalWorkerOptions.workerSrc = worker.default
  return pdfjs
}

// PDF-Seiten rendern: nur sichtbare (±1 Bildschirm), scharf für Zoom × Pixeldichte.
// Die Seiten im scroller tragen die Klasse .fe-page und data-i; ihre Canvas meldet setCanvas(i).
export function usePdfRender(zoom: Ref<number>, pages: Ref<Page[]>, scroller: Ref<HTMLElement | undefined>) {
  let pdf: PDFDocumentProxy | null = null
  let pdfPages: PDFPageProxy[] = []
  const canvases: (HTMLCanvasElement | null)[] = []
  const setCanvas = (i: number) => (el: unknown) => (canvases[i] = el as HTMLCanvasElement | null)
  const rendered = new Map<number, number>() // Seite → gerenderte Skala
  const tasks = new Map<number, RenderTask>()
  const visible = new Set<number>()
  let observer: IntersectionObserver | null = null
  let renderTimer = 0

  // Öffnet eine PDF, gibt die vorige frei und liefert die Seitenmaße.
  async function open(bytes: ArrayBuffer): Promise<Page[]> {
    // ponytail: ohne cMaps, Standardschriften und WASM-Decoder von pdf.js (seltene CJK-/JPEG-2000-PDFs zeigen dann Lücken); bei Bedarf nach public/ kopieren.
    const pdfjs = await loadPdfjs()
    const doc = await pdfjs.getDocument({ data: new Uint8Array(bytes.slice(0)) }).promise
    const list = await Promise.all(Array.from({ length: doc.numPages }, (_, i) => doc.getPage(i + 1)))
    for (const i of tasks.keys()) cancel(i)
    void pdf?.loadingTask.destroy()
    ;[pdf, pdfPages] = [doc, list]
    rendered.clear()
    return list.map((p) => {
      const v = p.getViewport({ scale: 1 })
      return { w: v.width, h: v.height, t: v.transform }
    })
  }

  function observe() {
    observer?.disconnect()
    if (!pdf || !scroller.value) return
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

  function dispose() {
    observer?.disconnect()
    clearTimeout(renderTimer)
    for (const i of tasks.keys()) cancel(i)
    void pdf?.loadingTask.destroy()
  }

  return { open, observe, scheduleRender, setCanvas, dispose }
}
