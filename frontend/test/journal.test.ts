// Journal-Notizen lesen und schreiben: node --test test/journal.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { journalPath, parseEntry, toMarkdown } from '../src/lib/journal.ts'

test('Journal hin und zurück', () => {
  const md = toMarkdown('2026-10-21', { answers: ['Sport gemacht', 'Früher anfangen\n- wirklich', ''], other: '' })
  assert.equal(md, '# Journal 2026-10-21\n\n## Was lief gut?\n\nSport gemacht\n\n## Was lerne ich daraus?\n\nFrüher anfangen\n- wirklich\n\n## Was ist morgen wichtig?\n')
  assert.deepEqual(parseEntry(md), { answers: ['Sport gemacht', 'Früher anfangen\n- wirklich', ''], other: '' })
  assert.equal(journalPath('2026-10-21'), 'Journal/2026-10-21.md')
})

test('eigene Abschnitte bleiben erhalten', () => {
  const e = parseEntry('# Journal\n\n## Was lief gut?\nviel\n\n## Dankbar für\nKaffee\n')
  assert.deepEqual(e, { answers: ['viel', '', ''], other: '## Dankbar für\nKaffee' })
  assert.match(toMarkdown('2026-10-21', e), /## Was ist morgen wichtig\?\n\n## Dankbar für\nKaffee\n$/)
})
