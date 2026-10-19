// Bereichsauswahl in Listen: node --test test/selection.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pick } from '../src/lib/selection.ts'

const order = ['a', 'b', 'c', 'd', 'e']

test('Klick schaltet um', () => {
  assert.deepEqual(pick(order, [], null, 'b', false), ['b'])
  assert.deepEqual(pick(order, ['b', 'd'], 'b', 'b', false), ['d'])
})

test('Shift wählt den Bereich in beide Richtungen und behält die bisherige Auswahl', () => {
  assert.deepEqual(pick(order, ['b'], 'b', 'd', true), ['b', 'c', 'd'])
  assert.deepEqual(pick(order, ['d'], 'd', 'b', true), ['d', 'b', 'c'])
  assert.deepEqual(pick(order, ['a'], 'c', 'e', true), ['a', 'c', 'd', 'e'])
})

test('Shift ohne gültigen Anker schaltet nur um', () => {
  assert.deepEqual(pick(order, [], null, 'c', true), ['c'])
  assert.deepEqual(pick(order, [], 'x', 'c', true), ['c'])
})
