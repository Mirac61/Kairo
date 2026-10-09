// Prüfung der Zeichen-Geometrie und des PDF-Exports: node test/annotate.test.ts
import assert from 'node:assert/strict'
import * as lib from 'pdf-lib'
import { apply, bounds, boxFrom, resizeBox, drawOnPdf, invert, pathOf, snapped, transformed, type Page, type Shape } from '../src/lib/annotate.ts'

const near = (a: number[], b: number[]) => a.forEach((v, i) => assert.ok(Math.abs(v - b[i]) < 1e-6, `${a} ≠ ${b}`))
const measure = (t: string, size: number) => t.length * size * 0.5

// Formen als Pfade
assert.equal(pathOf({ id: 1, color: '#000000', kind: 'rect', width: 1, x: 1, y: 2, w: 3, h: 4 }), 'M1 2L4 2L4 6L1 6Z')
assert.equal(pathOf({ id: 1, color: '#000000', kind: 'pen', width: 1, pts: [[0, 0], [2, 2], [4, 0]] }), 'M0 0Q2 2 3 1L4 0')
assert.match(pathOf({ id: 1, color: '#000000', kind: 'pen', width: 1, pts: [[5, 5]] }), /^M5 5L5\.01 5$/) // Punkt
assert.match(pathOf({ id: 1, color: '#000000', kind: 'ellipse', width: 1, x: 0, y: 0, w: 10, h: 4 }), /^M10 2C.*Z$/)
assert.equal(pathOf({ id: 1, color: '#000000', kind: 'line', width: 1, a: [0, 0], b: [1, 1] }, ([x, y]) => [x * 2, -y]), 'M0 0L2 -1')

// Box, Einrasten, Verschieben und Skalieren
assert.deepEqual(boxFrom([10, 10], [4, 2]), { x: 4, y: 2, w: 6, h: 8 })
assert.deepEqual(boxFrom([0, 0], [3, -5], true), { x: 0, y: -5, w: 5, h: 5 })
near(snapped([0, 0], [10, 1]), [Math.hypot(10, 1), 0])
const pen: Shape = { id: 2, color: '#000000', kind: 'pen', width: 2, pts: [[0, 0], [10, 20]] }
assert.deepEqual(bounds(pen, measure), { x: 0, y: 0, w: 10, h: 20 })
assert.deepEqual(transformed(pen, { x: 0, y: 0, w: 10, h: 20 }, { x: 5, y: 5, w: 20, h: 40 }).pts, [[5, 5], [25, 45]])
const text: Shape = { id: 3, color: '#000000', kind: 'text', size: 10, x: 0, y: 0, text: 'ab\nabcd' }
assert.deepEqual(bounds(text, measure), { x: 0, y: 0, w: 20, h: 25 })
assert.equal(transformed(text, bounds(text, measure), { x: 1, y: 1, w: 40, h: 50 }).size, 20)
const flat: Shape = { id: 4, color: '#000000', kind: 'line', width: 1, a: [0, 5], b: [10, 5] } // Höhe 0: kein Teilen durch 0
assert.deepEqual(transformed(flat, { x: 0, y: 5, w: 10, h: 0 }, { x: 0, y: 7, w: 10, h: 0 }).a, [0, 7])

assert.deepEqual(resizeBox({ x: 0, y: 0, w: 10, h: 5 }, 2, [30, 6], false), { x: 0, y: 0, w: 30, h: 6 })
assert.deepEqual(resizeBox({ x: 0, y: 0, w: 10, h: 5 }, 2, [30, 6], true), { x: 0, y: 0, w: 30, h: 15 }) // Verhältnis bleibt
assert.deepEqual(resizeBox({ x: 0, y: 0, w: 10, h: 5 }, 0, [-10, 2], false), { x: -10, y: 2, w: 20, h: 3 }) // Gegenecke (10,5) bleibt

// Seitentransformation: pdf.js viewport.transform einer um 90° gedrehten Seite mit verschobener CropBox
const t = [0, 1, 1, 0, -50, -20]
near(apply(invert(t), apply(t, [70, 300])), [70, 300])

// PDF-Export: Seite normal und gedreht, Umlaute bleiben, Emoji wird ersetzt
const src = await lib.PDFDocument.create()
src.addPage([300, 200])
src.addPage([300, 200]).setRotation(lib.degrees(90))
const bytes = await src.save()
const pages: Page[] = [{ w: 300, h: 200, t: [1, 0, 0, -1, 0, 200] }, { w: 200, h: 300, t: [0, 1, 1, 0, 0, 0] }]
const out = await drawOnPdf(lib, bytes.buffer as ArrayBuffer, pages, [
  [{ id: 1, color: '#e5484d', kind: 'rect', width: 2, x: 10, y: 10, w: 50, h: 30 }, { id: 2, color: '#1c1c1c', kind: 'text', size: 12, x: 20, y: 60, text: 'Größe 😀\nZeile 2' }],
  [{ id: 3, color: '#ffd60a', kind: 'marker', width: 10, pts: [[10, 10], [100, 10]] }, { id: 4, color: '#1c1c1c', kind: 'text', size: 12, x: 20, y: 20, text: 'quer' }],
])
const back = await lib.PDFDocument.load(out)
assert.equal(back.getPageCount(), 2)
assert.ok(out.length > bytes.length)
console.log('annotate ok')
