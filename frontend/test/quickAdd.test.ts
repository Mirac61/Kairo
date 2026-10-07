// Prüfung der Schnelleingabe: node test/quickAdd.test.ts (Node ≥ 22.18 führt TypeScript direkt aus)
import assert from 'node:assert/strict'
import { parseQuickAdd as p } from '../src/lib/quickAdd.ts'

const projects = [{ id: 'k', name: 'Kairo' }, { id: 's', name: 'SmartHome' }, { id: 'c', name: 'code-stats.nvim' }]
const today = '2026-10-07' // Mittwoch
const f = (s: string) => p(s, projects, today)

assert.deepEqual(f('30 min Sport'), { title: 'Sport', minutes: 30, date: null, time: null, project: null, priority: null }) // wie bisher
assert.equal(f('Sport 45m').title, 'Sport')
assert.equal(f('Sport 45m').minutes, 45)
assert.equal(f('3D Druck 30 min').title, '3D Druck') // Ziffer im Titel bleibt
assert.equal(f('Mathe lernen').minutes, 0)
assert.equal(f('Bericht 1h').minutes, 60)
assert.equal(f('Bericht 1,5h').minutes, 90)
assert.equal(f('Bericht 1h30').minutes, 90)
assert.equal(f('Bericht 2 std').minutes, 120)
assert.equal(f('Kapitel 3').minutes, 0) // Zahl ohne Einheit ist keine Dauer
assert.equal(f('Bericht 99h').minutes, 0) // über 24 h ignoriert
assert.equal(f('Bericht 99h').title, 'Bericht 99h')
assert.equal(f('Lampen @heute').date, '2026-10-07')
assert.equal(f('Lampen @Morgen').date, '2026-10-08')
assert.equal(f('Lampen @übermorgen').date, '2026-10-09')
assert.equal(f('Lampen @fr').date, '2026-10-09')
assert.equal(f('Lampen @mittwoch').date, '2026-10-14') // gleicher Wochentag = nächste Woche
assert.equal(f('Lampen @12.10.').date, '2026-10-12')
assert.equal(f('Lampen @5.10').date, '2027-10-05') // vergangen = nächstes Jahr
assert.equal(f('Lampen @31.2.').title, 'Lampen @31.2.') // kein echtes Datum
assert.deepEqual([f('x @14:30').date, f('x @14:30').time], ['2026-10-07', '14:30'])
assert.deepEqual([f('x @morgen @9:05').date, f('x @morgen @9:05').time], ['2026-10-08', '09:05'])
assert.equal(f('x @25:00').title, 'x @25:00')
assert.equal(f('Lampen #smart').project?.id, 's') // Anfang reicht
assert.equal(f('Lampen #SmartHome').project?.id, 's')
assert.equal(f('Lampen #codestats').project?.id, 'c') // Satzzeichen egal
assert.equal(f('Lampen #unbekannt').title, 'Lampen #unbekannt') // kein Treffer bleibt im Titel
assert.equal(f('Lampen !hoch').priority, 'HIGH')
assert.equal(f('Fertig!').priority, null)
assert.deepEqual(f('Lampen kaufen 1h @fr 18:00 #SmartHome !dringend'), { title: 'Lampen kaufen 18:00', minutes: 60, date: '2026-10-09', time: null, project: projects[1], priority: 'URGENT' })
assert.equal(f('#Kairo').title, '') // nur Token: kein Titel, die Ansicht legt nichts an
console.log('quickAdd: ok')
