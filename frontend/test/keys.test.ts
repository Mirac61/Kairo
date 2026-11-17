// Kürzel-Anzeige je Plattform: node --test test/keys.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ariaKeys, keys } from '../src/lib/keys.ts'

test('Mac behält ⌘ ⇧ ⌥', () => {
  assert.equal(keys('⇧⌘Z', true), '⇧⌘Z')
  assert.equal(ariaKeys('⌘D', true), 'Meta+D')
})

test('Windows und Linux: Strg/Shift/Alt in üblicher Reihenfolge', () => {
  assert.equal(keys('⌘K', false), 'Strg+K')
  assert.equal(keys('⌘ K', false), 'Strg+K')
  assert.equal(keys('⇧⌘Z', false), 'Strg+Shift+Z')
  assert.equal(keys('⇧D', false), 'Shift+D')
  assert.equal(keys('Rückgängig (⌘Z)', false), 'Rückgängig (Strg+Z)')
  assert.equal(keys('⌘Enter fügt hinzu', false), 'Strg+Enter fügt hinzu')
  assert.equal(keys('F2', false), 'F2')
  assert.equal(ariaKeys('⌘D', false), 'Control+D')
  assert.equal(ariaKeys('⇧D', false), 'Shift+D')
  assert.equal(keys('⌥⌘N', false), 'Strg+Alt+N')
  assert.equal(keys('⌥⌘⌫', false), 'Strg+Alt+Backspace')
  assert.equal(ariaKeys('⌥⌘⌫', true), 'Alt+Meta+Backspace')
})
