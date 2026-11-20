import { mathjax } from 'mathjax-full/js/mathjax.js'
import { TeX } from 'mathjax-full/js/input/tex.js'
import { SVG } from 'mathjax-full/js/output/svg.js'
import { liteAdaptor } from 'mathjax-full/js/adaptors/liteAdaptor.js'
import { RegisterHTMLHandler } from 'mathjax-full/js/handlers/html.js'
import { AllPackages } from 'mathjax-full/js/input/tex/AllPackages.js'

// LaTeX → eigenständiges SVG (Schrift als Pfade), wie es das Obsidian-Excalidraw-Plugin für Formeln erzeugt.
// Eigener Chunk: nur das Umwandeln von Obsidian-Zeichnungen und Lernkarten mit $…$ laden ihn.
// html/action ausgeschlossen, damit \href{javascript:...} oder \style in importierten Karten kein XSS einschleusen.
const SAFE_PACKAGES = AllPackages.filter((p) => p !== 'html' && p !== 'action')

const adaptor = liteAdaptor()
RegisterHTMLHandler(adaptor)
const doc = mathjax.document('', {
  InputJax: new TeX({ packages: SAFE_PACKAGES }),
  OutputJax: new SVG({ fontCache: 'none' }),
})

function escapeHtml(s: string) {
  return s
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

export function texToSvg(tex: string, display = true): string {
  try {
    return adaptor.innerHTML(doc.convert(tex, { display }))
  } catch {
    return `<code>${escapeHtml(tex)}</code>`
  }
}
