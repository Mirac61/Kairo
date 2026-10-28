import assert from 'node:assert/strict'
import { test } from 'node:test'
import { leafFrame } from '../src/lib/pageTurn.ts'

const W = 500
const H = 600

test('Umblättern: Anfang und Ende liegen flach', () => {
  // Am Anfang ist die alte Seite ganz da und die Rückseite noch nicht zu sehen.
  assert.equal(leafFrame('cut', 0, W, H).clip, 'polygon(0px 0px, 500px 0px, 500px 600px, 0px 600px)')
  assert.equal(leafFrame('flap', 0, W, H).clip, 'polygon(0px 0px, 0px 0px, 0px 600px, 0px 600px)')
  // Am Ende liegt die neue Seite genau an ihrem Platz, die alte ist bis auf den Bund verschwunden.
  const end = leafFrame('flap', 1, W, H)
  assert.equal(end.transform, 'matrix(1, 0, 0, 1, 0, 0)')
  assert.equal(end.clip, 'polygon(0px 0px, 500px 0px, 500px 600px, 0px 600px)')
  assert.equal(leafFrame('cut', 1, W, H).clip, 'polygon(0px 0px, 0px 0px, 0px 600px, 0px 600px)')
})

test('Umblättern: die Rückseite liegt am Falz an', () => {
  // Halb umgeblättert: Der Falzpunkt in der Mitte gehört zur alten und zur neuen Seite zugleich.
  const p = 0.5
  const cut = leafFrame('cut', p, W, H)
  const flap = leafFrame('flap', p, W, H)
  const [a, b, c, d, e, f] = flap.transform.slice(7, -1).split(', ').map(Number)
  const fold = W * (1 - p) // Falz auf der rechten Seite, vom Bund aus
  // Rückseite: lokaler Punkt (W - fold, H/2) der linken Seite, die bei x = -W beginnt
  const x = a * (W - fold) + c * (H / 2) + e - W
  const y = b * (W - fold) + d * (H / 2) + f
  assert.ok(Math.abs(x - fold) < 0.5, `x ${x}`)
  assert.ok(Math.abs(y - H / 2) < 0.5, `y ${y}`)
  const [, top, bottom] = [...cut.clip.matchAll(/([\d.]+)px ([\d.]+)px/g)].map((m) => Number(m[1]))
  assert.ok(top! > fold && bottom! < fold, `untere Ecke läuft voraus: ${cut.clip}`)
})

test('Umblättern: zurück ist gespiegelt', () => {
  const fwd = leafFrame('cut', 0.3, W, H)
  const back = leafFrame('cut', 0.3, W, H, true)
  const xs = (clip: string) => [...clip.matchAll(/([\d.-]+)px ([\d.-]+)px/g)].map((m) => Number(m[1]))
  assert.deepEqual(xs(back.clip), xs(fwd.clip).map((x) => Math.round((W - x) * 100) / 100))
  assert.equal(leafFrame('flap', 1, W, H, true).transform, 'matrix(1, 0, 0, 1, 0, 0)')
})
