import assert from 'node:assert/strict'
import { test } from 'node:test'
import { projectIconKey, projectIconOf } from '../src/lib/projectIcons.ts'

test('Projektsymbole: gewähltes Symbol gilt, sonst Name geraten, sonst Ordner', () => {
  // Explizit gewählt
  assert.equal(projectIconKey({ name: 'Beliebig', icon: 'code' }), 'code')
  assert.equal(projectIconOf({ name: 'Beliebig', icon: 'code' }), 'i-code')

  // Aus Namen geraten
  assert.equal(projectIconKey({ name: 'Uni Informatik' }), 'book')
  assert.equal(projectIconOf({ name: 'Uni Informatik' }), 'hi-book')

  assert.equal(projectIconKey({ name: 'activitytracker' }), 'code')
  assert.equal(projectIconOf({ name: 'activitytracker' }), 'i-code')

  assert.equal(projectIconKey({ name: 'SmartHome' }), 'sparkles')
  assert.equal(projectIconOf({ name: 'SmartHome' }), 'hi-sparkles')

  assert.equal(projectIconKey({ name: 'Bewerbungen' }), 'note')
  assert.equal(projectIconOf({ name: 'Bewerbungen' }), 'i-note')

  assert.equal(projectIconKey({ name: 'Finanzen' }), 'wallet')
  assert.equal(projectIconOf({ name: 'Finanzen' }), 'hi-wallet')

  // Unbekannt / Fallback
  assert.equal(projectIconKey({ name: 'Sonstiges Projekt' }), 'folder')
  assert.equal(projectIconOf({ name: 'Sonstiges Projekt' }), 'i-proj')
})
