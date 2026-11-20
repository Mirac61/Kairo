import test from 'node:test'
import assert from 'node:assert/strict'
import { hasMath, splitMath } from '../src/lib/math.ts'

test('erkennt Inline- und Block-LaTeX', () => {
  assert.equal(hasMath('Fläche $A = \\pi r^2$'), true)
  assert.equal(hasMath('$$\\int_0^1 x\\,dx$$'), true)
  assert.deepEqual(splitMath('A $x+1$ B'), [
    { kind: 'text', value: 'A ' },
    { kind: 'math', expr: 'x+1', display: false },
    { kind: 'text', value: ' B' },
  ])
})

test('ignoriert Währungen, escaped \\$ und Code-Spans', () => {
  assert.equal(hasMath('Kostet $5 oder $10 heute'), false)
  assert.equal(hasMath('Preis \\$5 bis \\$10'), false)
  assert.equal(hasMath('`$PATH` und `$HOME`', { skipCode: true }), false)
  assert.equal(hasMath('```\n$a + b$\n```', { skipCode: true }), false)
})
