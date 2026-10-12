import { fetchFile } from '@/api/client'

declare global {
  interface Window { EXCALIDRAW_ASSET_PATH?: string | string[] }
}

// Excalidraw (samt React) ist ein eigener Chunk, den erst eine Zeichnung lädt. Die Schriften liefert Kairo selbst aus
// (vite.config.ts kopiert sie nach dist/excalidraw/), damit es offline läuft; im Dev-Server kommen sie vom CDN.
export function loadExcalidraw() {
  window.EXCALIDRAW_ASSET_PATH = '/excalidraw/'
  return Promise.all([import('@excalidraw/excalidraw'), import('@excalidraw/excalidraw/index.css')]).then(([m]) => m)
}

// Vorschau eingebetteter Zeichnungen (![](x.excalidraw)): als SVG-Blob in einem <img>, dort führt SVG kein Skript aus.
// sanitize im Editor ist synchron, deshalb liegen fertige Bilder hier im Cache; render füllt ihn nach.
const cache = new Map<string, { mtime: string; url: string }>()
const running = new Map<string, Promise<boolean>>()

export const drawingSrc = (path: string) => cache.get(path)?.url

// Lädt die Zeichnung und rendert sie neu, wenn sich die Datei seit dem letzten Mal geändert hat. true = neues Bild.
export function renderDrawing(path: string): Promise<boolean> {
  let p = running.get(path)
  if (!p) {
    p = render(path).finally(() => running.delete(path))
    running.set(path, p)
  }
  return p
}

async function render(path: string) {
  const { blob, mtime } = await fetchFile(path)
  if (cache.get(path)?.mtime === mtime) return false
  const { exportToSvg } = await loadExcalidraw()
  const scene = JSON.parse(await blob.text())
  const svg = await exportToSvg({
    elements: scene.elements ?? [],
    appState: { ...scene.appState, exportBackground: false },
    files: scene.files ?? null,
    exportPadding: 8,
  })
  const url = URL.createObjectURL(new Blob([svg.outerHTML], { type: 'image/svg+xml' }))
  const old = cache.get(path)
  cache.set(path, { mtime, url })
  if (old) URL.revokeObjectURL(old.url)
  return true
}
