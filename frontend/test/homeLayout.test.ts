// Anordnung der Startseite lesen und umsortieren: node --test test/homeLayout.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DEFAULTS, moveSlot, parseLayout } from '../src/lib/homeLayout.ts'

test('leer oder kaputt → Standard des Geräts', () => {
  assert.deepEqual(parseLayout('', 'desktop'), DEFAULTS.desktop)
  assert.deepEqual(parseLayout('{', 'mobile'), DEFAULTS.mobile)
  assert.deepEqual(parseLayout('{"id":"now"}', 'mobile'), DEFAULTS.mobile)
  assert.deepEqual(parseLayout('[]', 'desktop'), [], 'alles entfernt bleibt leer')
})

test('Unbekanntes, falsche Größen und Doppeltes fallen weg', () => {
  const json = JSON.stringify([
    { id: 'now', size: 'M' },
    { id: 'now', size: 'M' },
    { id: 'inbox', size: 'S' },
    { id: 'cards', size: 'L' },
    { id: 'toString', size: 'S' },
    null,
    { id: 'agenda', size: 'L' },
  ])
  assert.deepEqual(parseLayout(json, 'desktop'), [
    { id: 'now', size: 'M' },
    { id: 'agenda', size: 'L' },
  ])
})

test('Standard nicht verändern', () => {
  parseLayout('', 'desktop')[0]!.size = 'M'
  assert.equal(DEFAULTS.desktop[0]!.size, 'L')
})

test('verschieben', () => {
  assert.deepEqual(moveSlot(['a', 'b', 'c'], 0, 2), ['b', 'c', 'a'])
  assert.deepEqual(moveSlot(['a', 'b', 'c'], 2, 1), ['a', 'c', 'b'])
  assert.deepEqual(moveSlot(['a', 'b'], 0, -1), ['a', 'b'])
  assert.deepEqual(moveSlot(['a', 'b'], 1, 2), ['a', 'b'])
})
