import { drawOnCanvas, drawOnPdf, type Page, type Shape } from '@/lib/annotate'
import { t } from '@/lib/i18n'

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

// Bild mit eingebrannten Formen, auf outScale verkleinert, im Format type.
export async function imageBlob(img: HTMLImageElement, pg: Page, shapes: Shape[], outScale: number, type: string): Promise<Blob> {
  const [w, h] = [Math.max(1, Math.round(pg.w * outScale)), Math.max(1, Math.round(pg.h * outScale))]
  const c = scaled(img, w, h)
  const ctx = c.getContext('2d')!
  ctx.scale(w / pg.w, h / pg.h)
  drawOnCanvas(ctx, shapes)
  const blob = await new Promise<Blob | null>((res) => c.toBlob(res, type, 0.92))
  if (!blob || blob.type !== type) throw new Error(t('Dieser Browser kann {type} nicht schreiben.', { type }))
  return blob
}

// PDF mit eingebrannten Formen; pdf-lib wird erst hier geladen.
export async function pdfBlob(bytes: ArrayBuffer, pages: Page[], shapes: Shape[][]): Promise<Blob> {
  try {
    return new Blob([(await drawOnPdf(await import('pdf-lib'), bytes, pages, shapes)) as BlobPart], { type: 'application/pdf' })
  } catch {
    throw new Error(t('Kairo kann diese PDF nicht bearbeiten (verschlüsselt oder beschädigt).'))
  }
}
