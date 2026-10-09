// Jeder Text in t('…'), $t('…') und tn(n, '…', '…') hat eine englische Übersetzung: node --test test/i18n.test.ts
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'
import en from '../src/lib/en.ts'
import { lang, t, tn } from '../src/lib/i18n.ts'

const files = (dir: string): string[] =>
  readdirSync(dir, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? files(join(dir, e.name)) : /\.(vue|ts)$/.test(e.name) ? [join(dir, e.name)] : []))
const STR = `'((?:[^'\\\\]|\\\\.)*)'`
const keys = new Set<string>()
for (const f of files(new URL('../src', import.meta.url).pathname)) {
  const src = readFileSync(f, 'utf8')
  for (const m of src.matchAll(new RegExp(`(?<![\\w.])\\$?t\\(${STR}`, 'g'))) keys.add(m[1]!.replace(/\\'/g, "'"))
  for (const m of src.matchAll(new RegExp(`(?<![\\w.])\\$?tn\\([^,]+,\\s*${STR},\\s*${STR}`, 'g'))) keys.add(m[1]!).add(m[2]!)
}

test('alle Schlüssel übersetzt', () => {
  const missing = [...keys].filter((k) => !(k in en)).sort()
  assert.deepEqual(missing, [], `fehlt in en.ts:\n${missing.map((k) => JSON.stringify(k)).join('\n')}`)
  // Platzhalter bleiben gleich, sonst fehlen Werte im Text.
  for (const [k, v] of Object.entries(en)) assert.deepEqual(v.match(/\{\w+\}/g)?.sort(), k.match(/\{\w+\}/g)?.sort(), k)
})

test('t und tn', () => {
  lang.value = 'en'
  assert.equal(t('Speichern'), 'Save')
  assert.equal(t('nicht übersetzt'), 'nicht übersetzt')
  assert.equal(tn(1, '{n} Eintrag', '{n} Einträge'), '1 entry')
  assert.equal(tn(3, '{n} Eintrag', '{n} Einträge'), '3 entries')
  lang.value = 'de'
  assert.equal(t('„{title}“ gelöscht', { title: 'X' }), '„X“ gelöscht')
})
