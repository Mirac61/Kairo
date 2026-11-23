import assert from 'node:assert/strict'
import { test } from 'node:test'
import { goalPace, spreadDue, topicLines } from '../src/lib/goals.ts'

const g = (percent: number, current: number, target: number, extra = {}) => ({
  kind: 'PROJECT_TASKS' as const,
  start_date: '2026-01-01',
  due_date: '2026-01-29',
  progress: { current, target, percent },
  ...extra,
})

test('Ziele: Soll-Stand und Zustand aus Start, Frist und Fortschritt', () => {
  // Halbzeit (14 von 28 Tagen): Soll 50 %.
  assert.deepEqual(goalPace(g(60, 12, 20), '2026-01-15'), {
    pace: 50,
    state: 'ok',
    daysLeft: 14,
    hint: '2 Aufgaben voraus',
  })
  assert.equal(goalPace(g(45, 9, 20), '2026-01-15')?.state, 'tight')
  // 16 offen, 2 Wochen übrig: 8 pro Woche.
  assert.deepEqual(goalPace(g(20, 4, 20), '2026-01-15'), {
    pace: 50,
    state: 'off',
    daysLeft: 14,
    hint: '8 Aufgaben pro Woche nötig',
  })
  assert.equal(goalPace(g(20, 4, 20), '2026-02-10')?.hint, 'Frist vorbei')
  assert.equal(goalPace(g(100, 20, 20), '2026-02-10')?.state, 'ok')
  assert.equal(goalPace(g(40, 4, 10, { kind: 'HABIT_COUNT' }), '2026-01-08')?.hint, '1-mal voraus')
  // Ohne Frist oder Fortschritt kein Soll.
  assert.equal(goalPace(g(50, 1, 2, { due_date: null }), '2026-01-15'), null)
  assert.equal(goalPace({ ...g(0, 0, 0), progress: null }, '2026-01-15'), null)
})

test('spreadDue verteilt Teilziele bis drei Tage vor der Frist', () => {
  assert.deepEqual(spreadDue('2026-11-24', '2026-12-17', 4), [
    '2026-11-29',
    '2026-12-04',
    '2026-12-09',
    '2026-12-14',
  ])
  assert.deepEqual(spreadDue('2026-11-24', '2026-12-17', 1), ['2026-12-14'])
  // Über die Zeitumstellung und den Jahreswechsel hinweg
  assert.deepEqual(spreadDue('2026-10-20', '2027-01-04', 2), ['2026-11-26', '2027-01-01'])
  // Frist näher als der Puffer: alles heute
  assert.deepEqual(spreadDue('2026-11-24', '2026-11-26', 2), ['2026-11-24', '2026-11-24'])
})

test('topicLines entfernt Leerzeilen und Listenzeichen', () => {
  assert.deepEqual(
    topicLines('Folgen\n- Reihen\n\n  * Grenzwerte \n1. Integrale\n2) Taylor\n-Minus bleibt'),
    ['Folgen', 'Reihen', 'Grenzwerte', 'Integrale', 'Taylor', '-Minus bleibt'],
  )
})
