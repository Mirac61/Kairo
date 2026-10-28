import assert from 'node:assert/strict'
import { test } from 'node:test'
import { goalPace } from '../src/lib/goals.ts'

const g = (percent: number, current: number, target: number, extra = {}) =>
  ({ kind: 'PROJECT_TASKS' as const, start_date: '2026-01-01', due_date: '2026-01-29', progress: { current, target, percent }, ...extra })

test('Ziele: Soll-Stand und Zustand aus Start, Frist und Fortschritt', () => {
  // Halbzeit (14 von 28 Tagen): Soll 50 %.
  assert.deepEqual(goalPace(g(60, 12, 20), '2026-01-15'), { pace: 50, state: 'ok', daysLeft: 14, hint: '2 Aufgaben voraus' })
  assert.equal(goalPace(g(45, 9, 20), '2026-01-15')?.state, 'tight')
  // 16 offen, 2 Wochen übrig: 8 pro Woche.
  assert.deepEqual(goalPace(g(20, 4, 20), '2026-01-15'), { pace: 50, state: 'off', daysLeft: 14, hint: '8 Aufgaben pro Woche nötig' })
  assert.equal(goalPace(g(20, 4, 20), '2026-02-10')?.hint, 'Frist vorbei')
  assert.equal(goalPace(g(100, 20, 20), '2026-02-10')?.state, 'ok')
  assert.equal(goalPace(g(40, 4, 10, { kind: 'HABIT_COUNT' }), '2026-01-08')?.hint, '1-mal voraus')
  // Ohne Frist oder Fortschritt kein Soll.
  assert.equal(goalPace(g(50, 1, 2, { due_date: null }), '2026-01-15'), null)
  assert.equal(goalPace({ ...g(0, 0, 0), progress: null }, '2026-01-15'), null)
})
