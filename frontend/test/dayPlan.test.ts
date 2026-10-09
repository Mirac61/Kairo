// Rechenteile der Startseite: node --test test/dayPlan.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { columns, doneSeconds, minutesOfDay, stopwatch } from '../src/lib/dayPlan.ts'

test('Uhrzeit, Stoppuhr und erfasste Zeit', () => {
  assert.equal(minutesOfDay(Date.parse('2026-10-09T08:30:00Z'), 'Europe/Berlin'), 10 * 60 + 30)
  assert.equal(minutesOfDay(Date.parse('2026-10-09T23:15:00Z'), 'UTC'), 23 * 60 + 15)
  assert.equal(stopwatch(3725.9), '1:02:05')
  const at = (h: number) => `2026-10-09T${String(h).padStart(2, '0')}:00:00Z`
  assert.equal(doneSeconds([
    { task_id: 'a', started_at: at(8), ended_at: at(9) },
    { task_id: 'a', started_at: at(10), ended_at: null }, // läuft noch
    { task_id: 'b', started_at: at(8), ended_at: at(12) },
  ], 'a'), 3600)
})

test('Überlappende Blöcke nebeneinander', () => {
  const pos = columns([
    { key: 'a', start: 540, end: 600 },
    { key: 'b', start: 570, end: 630 },
    { key: 'c', start: 600, end: 615 }, // Spalte von a ist ab 600 frei
    { key: 'd', start: 700, end: 705 }, // eigene Gruppe, zählt 30 Minuten
    { key: 'e', start: 720, end: 750 },
  ], 30)
  assert.deepEqual(Object.fromEntries(pos), {
    a: { col: 0, cols: 2 }, b: { col: 1, cols: 2 }, c: { col: 0, cols: 2 },
    d: { col: 0, cols: 2 }, e: { col: 1, cols: 2 },
  })
})
