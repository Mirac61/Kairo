import { mathjax } from 'mathjax-full/js/mathjax.js'
import { TeX } from 'mathjax-full/js/input/tex.js'
import { SVG } from 'mathjax-full/js/output/svg.js'
import { liteAdaptor } from 'mathjax-full/js/adaptors/liteAdaptor.js'
import { RegisterHTMLHandler } from 'mathjax-full/js/handlers/html.js'
import { AllPackages } from 'mathjax-full/js/input/tex/AllPackages.js'

// LaTeX → eigenständiges SVG (Schrift als Pfade), wie es das Obsidian-Excalidraw-Plugin für Formeln erzeugt.
// Eigener Chunk: nur das Umwandeln von Obsidian-Zeichnungen und Lernkarten mit $…$ laden ihn.
const adaptor = liteAdaptor()
RegisterHTMLHandler(adaptor)
const doc = mathjax.document('', { InputJax: new TeX({ packages: AllPackages }), OutputJax: new SVG({ fontCache: 'none' }) })

export const texToSvg = (tex: string, display = true) => adaptor.innerHTML(doc.convert(tex, { display }))
