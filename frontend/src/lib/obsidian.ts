import { decompressFromBase64 } from 'lz-string'
import { deleteNote, errorMessage, fetchFile, readNote, uploadFile } from '@/api/client'
import { t } from '@/lib/i18n'
import { fileKind, isObsidianDrawing, parseObsidianDrawing, resolveLink } from '@/lib/noteFiles'

const dataUrl = (blob: Blob) =>
  new Promise<string>((ok, fail) => {
    const r = new FileReader()
    r.onload = () => ok(r.result as string)
    r.onerror = () => fail(r.error)
    r.readAsDataURL(blob)
  })

// Wandelt jede „Name.excalidraw.md“ aus Obsidian in „Name.excalidraw“ um, damit ![[Name.excalidraw]] darauf zeigt.
// Eingefügte Bilder kommen als data-URL in files, Formeln ($$TeX$$) als SVG, ihr TeX bleibt in customData.latex.
// Das Original wandert erst in den Papierkorb, wenn die neue Datei geschrieben ist; bei jedem Fehler bleibt es liegen.
export async function convertObsidianDrawings(files: string[]) {
  const done: string[] = []
  const skipped: string[] = []
  for (const p of files.filter(isObsidianDrawing)) {
    try {
      await convert(p, files)
      done.push(p)
    } catch (e) {
      skipped.push(`${p}: ${errorMessage(e)}`)
    }
  }
  return { done, skipped }
}

async function convert(p: string, files: string[]) {
  const dest = p.slice(0, -'.md'.length)
  if (files.includes(dest)) throw new Error(t('„{name}“ existiert schon.', { name: dest }))
  const { scene, embedded } = parseObsidianDrawing((await readNote(p)).content, decompressFromBase64)
  const out: Record<string, unknown> = (scene.files ??= {})
  const images = new Map(scene.elements.filter((e) => e.type === 'image' && e.fileId).map((e) => [e.fileId!, e]))
  for (const [id, v] of embedded) {
    if (out[id]) continue
    let url: string
    if ('file' in v) {
      const f = resolveLink(v.file, p.slice(0, Math.max(0, p.lastIndexOf('/'))), files)
      if (fileKind(f) !== 'image') throw new Error(t('Eingebettete Datei „{name}“ fehlt.', { name: v.file }))
      url = await dataUrl((await fetchFile(f)).blob)
    } else if ('tex' in v) {
      const { texToSvg } = await import('@/lib/tex')
      url = await dataUrl(new Blob([texToSvg(v.tex)], { type: 'image/svg+xml' }))
      const el = images.get(id)
      if (el) el.customData = { ...el.customData, latex: v.tex }
    } else {
      throw new Error(t('Eingebettetes Objekt „{name}“ ist weder Bild noch Formel.', { name: v.other }))
    }
    out[id] = { id, mimeType: url.slice(5, url.indexOf(';')), dataURL: url, created: Date.now() }
  }
  for (const id of images.keys()) if (!out[id]) throw new Error(t('Bild {id} ohne Datei.', { id }))
  await uploadFile(dest, new Blob([JSON.stringify(scene)], { type: 'application/json' }))
  await deleteNote(p)
}
