// Rechenteile der Lernkarten: node --test test/cards.test.ts
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  accuracy,
  forecast,
  intervalText,
  lapses,
  parseCardList,
  perDay,
  streak,
} from '../src/lib/cards.ts'

test('Liste einfügen', () => {
  const r = parseCardList(
    '- ATP :: Energieträger\nScrum\tPO, SM, Devs\n\nNur Notiz\nA :: B :: C\n :: leer',
  )
  assert.deepEqual(r.cards, [
    { question: 'ATP', answer: 'Energieträger' },
    { question: 'Scrum', answer: 'PO, SM, Devs' },
    { question: 'A', answer: 'B :: C' },
  ])
  assert.equal(r.skipped, 2)
})

test('Vorschau, Serie, Trefferquote, oft vergessen', () => {
  const today = new Date(2026, 9, 26, 12)
  const card = (due: boolean, due_date: string | null) => ({ due, due_date }) as never
  assert.deepEqual(
    forecast(
      [
        card(true, null),
        card(true, '2026-10-20'),
        card(false, '2026-10-27'),
        card(false, '2026-11-30'),
      ],
      today,
      3,
    ),
    [2, 1, 0],
  )
  const at = (d: number, grade: number, key = 'a') => ({
    key,
    grade,
    at: new Date(2026, 9, d, 9).toISOString(),
  })
  const log = [at(22, 2), at(24, 0), at(24, 0, 'b'), at(25, 3), at(25, 0)]
  const days = perDay(log)
  assert.equal(days.get('2026-10-24'), 2)
  assert.equal(streak(days, today), 2) // heute noch nichts: 24. und 25.
  assert.equal(streak(new Map([...days, ['2026-10-26', 1]]), today), 3)
  assert.equal(accuracy(log), 40)
  assert.equal(accuracy([]), null)
  assert.deepEqual(
    [...lapses(log)],
    [
      ['a', 2],
      ['b', 1],
    ],
  )
  assert.deepEqual([0, 1, 12, 45, 400].map(intervalText), [
    'gleich',
    '1 T',
    '12 T',
    '2 Mon.',
    '1.1 J.',
  ])
})
