// Rechenteile der Startseite: node --test test/dayPlan.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  blockEndMinutes,
  columns,
  doneSeconds,
  findSlot,
  gaps,
  minutesOfDay,
  stopwatch,
} from '../src/lib/dayPlan.ts'

test('Uhrzeit, Stoppuhr und erfasste Zeit', () => {
  assert.equal(minutesOfDay(Date.parse('2026-10-09T08:30:00Z'), 'Europe/Berlin'), 10 * 60 + 30)
  assert.equal(minutesOfDay(Date.parse('2026-10-09T23:15:00Z'), 'UTC'), 23 * 60 + 15)
  assert.equal(stopwatch(3725.9), '1:02:05')
  const at = (h: number) => `2026-10-09T${String(h).padStart(2, '0')}:00:00Z`
  assert.equal(
    doneSeconds(
      [
        { task_id: 'a', started_at: at(8), ended_at: at(9) },
        { task_id: 'a', started_at: at(10), ended_at: null }, // läuft noch
        { task_id: 'b', started_at: at(8), ended_at: at(12) },
      ],
      'a',
    ),
    3600,
  )
})

test('Überlappende Blöcke nebeneinander', () => {
  const pos = columns(
    [
      { key: 'a', start: 540, end: 600 },
      { key: 'b', start: 570, end: 630 },
      { key: 'c', start: 600, end: 615 }, // Spalte von a ist ab 600 frei
      { key: 'd', start: 700, end: 705 }, // eigene Gruppe, zählt 30 Minuten
      { key: 'e', start: 720, end: 750 },
    ],
    30,
  )
  assert.deepEqual(Object.fromEntries(pos), {
    a: { col: 0, cols: 2 },
    b: { col: 1, cols: 2 },
    c: { col: 0, cols: 2 },
    d: { col: 0, cols: 2 },
    e: { col: 1, cols: 2 },
  })
})

test('Zeitvorschlag: Lücke, Ränder, nichts frei', () => {
  const h = (x: number, m = 0) => x * 60 + m
  const busy = [
    { title: 'Mensa', start: h(12), end: h(13, 7) },
    { title: 'Labor', start: h(15), end: h(17) },
  ]
  // Lücke zwischen zwei Terminen, Ende der Mensa wird auf 5 Minuten aufgerundet
  const mid = findSlot(busy, h(12, 30), h(9), h(18), 60)!
  assert.deepEqual([mid.start, mid.prev?.title, mid.next?.title], [h(13, 10), 'Mensa', 'Labor'])
  // zu lang für die Lücke: nach dem letzten Termin
  const after = findSlot(busy, h(12, 30), h(9), h(18), 120)
  assert.equal(after, null) // 17:00 + 2h > 18:00
  const late = findSlot(busy, h(12, 30), h(9), h(20), 120)!
  assert.deepEqual([late.start, late.prev?.title, late.next], [h(17), 'Labor', undefined])
  // vor dem ersten Termin (jetzt liegt vor dem Fenster)
  const early = findSlot(busy, h(6), h(9), h(18), 60)!
  assert.deepEqual([early.start, early.prev, early.next?.title], [h(9), undefined, 'Mensa'])
  // jetzt mitten in einem Termin, leerer Tag
  assert.equal(findSlot(busy, h(15, 30), h(9), h(17, 30), 60), null)
  assert.deepEqual(findSlot([], h(10, 2), h(9), h(18), 30), {
    start: h(10, 5),
    prev: undefined,
    next: undefined,
  })
})

test('Block-Ende über Mitternacht', () => {
  const h = (x: number, m = 0) => x * 60 + m
  assert.equal(blockEndMinutes(h(10), h(11)), h(11)) // normal
  assert.equal(blockEndMinutes(h(23), h(1)), h(25)) // 23:00 → 01:00
  assert.equal(blockEndMinutes(h(23), 0), h(24)) // endet genau um Mitternacht
  assert.equal(blockEndMinutes(0, 0), h(24)) // ganztägig
})

test('freie Lücken', () => {
  const b = (start: number, end: number) => ({ start, end })
  assert.deepEqual(gaps([b(600, 660), b(630, 700), b(720, 740)], 540, 1020, 30), [
    b(540, 600),
    b(740, 1020),
  ])
  assert.deepEqual(gaps([], 540, 600, 30), [b(540, 600)])
  assert.deepEqual(gaps([b(500, 1100)], 540, 1020, 30), [])
  assert.deepEqual(
    gaps([b(1000, 1100)], 540, 600, 30),
    [b(540, 600)],
    'Blöcke nach dem Fenster kürzen nichts',
  )
})
