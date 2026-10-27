// Symbol einer Gewohnheit: node --test test/habitIcons.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { iconKey, iconOf } from '../src/lib/habitIcons.ts'

test('gewähltes Symbol gilt, sonst aus dem Namen geraten, sonst Standard', () => {
  assert.equal(iconKey({ name: 'Lesen', icon: 'bike' }), 'bike')
  assert.equal(iconKey({ name: 'Lesen · 20 min', icon: '' }), 'book')
  assert.equal(iconKey({ name: 'Wasser trinken' }), 'droplet')
  assert.equal(iconKey({ name: 'xyz', icon: 'gibtsnicht' }), 'repeat')
  assert.equal(iconOf({ name: 'Meditieren' }), 'hi-leaf')
})
