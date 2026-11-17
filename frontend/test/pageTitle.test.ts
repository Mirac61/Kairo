import assert from 'node:assert/strict'
import { test } from 'node:test'
import { setDocTitle, setViewTitle } from '../src/lib/pageTitle.ts'

test('Tab-Titel: Ansicht, Dokument ohne Ordner und Endung, zurück zur Ansicht', () => {
  const doc = { title: '' }
  ;(globalThis as { document?: unknown }).document = doc
  setViewTitle('Notizen')
  assert.equal(doc.title, 'Notizen · Kairo')
  setDocTitle('AlgoDat/Vorlesungen/2026-11-16 -K7-Graphen.md')
  assert.equal(doc.title, '2026-11-16 -K7-Graphen · Kairo')
  setDocTitle(null)
  assert.equal(doc.title, 'Notizen · Kairo')
})
