// Prüfung der Termin-Formularlogik und der Datumshelfer: npm test (Node ≥ 22.18 führt TypeScript direkt aus)
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { dur, entryMinutes, weekStart, ymd } from '../src/lib/dates.ts'
import { editForm, isAllDay, newForm, ruleOf } from '../src/lib/eventForm.ts'

const ev = (over: object) => ({
  id: 'e1', title: 'Vorlesung', location: 'H 1.02', occurrence_start: '', occurrence_end: '', project_id: null,
  start_at: new Date(2026, 9, 7, 10, 0).toISOString(), end_at: new Date(2026, 9, 7, 11, 30).toISOString(), recurrence_rule: null, ...over,
})

test('Serienregel: Dialog -> RRULE -> Dialog', () => {
  const f = newForm(new Date(2026, 9, 7, 10, 0), new Date(2026, 9, 7, 11, 30), false) // Mittwoch
  assert.deepEqual(f.days, ['WE'])
  Object.assign(f, { repeat: true, days: ['MO', 'WE'], until: '2026-12-31' })
  const rule = ruleOf(f)
  assert.equal(rule, 'FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261231')
  const back = editForm(ev({ recurrence_rule: rule }), '2026-10-07')
  assert.deepEqual([back.repeat, back.days, back.until, back.day, back.customRule], [true, ['MO', 'WE'], '2026-12-31', '2026-10-07', null])
})

test('Eigene Regeln bleiben unverändert', () => {
  const f = editForm(ev({ recurrence_rule: 'FREQ=MONTHLY;BYMONTHDAY=1' }), '2026-10-07')
  assert.deepEqual([f.repeat, f.customRule], [false, 'FREQ=MONTHLY;BYMONTHDAY=1'])
})

test('Ohne Wiederholung keine Regel; ohne Wochentage gilt der Starttag', () => {
  const f = newForm(new Date(2026, 9, 7, 10, 0), new Date(2026, 9, 7, 11, 0), false)
  assert.equal(ruleOf(f), '')
  assert.equal(ruleOf({ ...f, repeat: true, days: [] }), 'FREQ=WEEKLY;BYDAY=WE')
})

test('Ganztägig: Mitternacht bis Mitternacht, Ende einschließlich', () => {
  const [s, e] = [new Date(2026, 9, 7), new Date(2026, 9, 10)]
  assert.equal(isAllDay(s, e), true)
  assert.equal(isAllDay(new Date(2026, 9, 7, 10), new Date(2026, 9, 7, 11)), false)
  const f = editForm(ev({ start_at: s.toISOString(), end_at: e.toISOString() }), '')
  assert.deepEqual([f.allDay, f.date, f.endDate, f.keepTimes], [true, '2026-10-07', '2026-10-09', false])
})

test('Mehrtägiger Termin mit Uhrzeit behält seine Zeiten', () => {
  const f = editForm(ev({ end_at: new Date(2026, 9, 8, 9, 0).toISOString() }), '')
  assert.deepEqual([f.allDay, f.keepTimes], [false, true])
})

test('Datumshelfer', () => {
  assert.equal(ymd(weekStart(new Date(2026, 9, 7))), '2026-10-05') // Mittwoch -> Montag
  assert.equal(ymd(weekStart(new Date(2026, 9, 11))), '2026-10-05') // Sonntag -> Montag davor
  assert.equal(dur(45), '45 Min')
  assert.equal(dur(90), '1 Std 30 Min')
  assert.equal(entryMinutes({ started_at: '2026-10-07T10:00:00Z', ended_at: '2026-10-07T11:15:30Z' }), 75)
  assert.equal(entryMinutes({ started_at: '2026-10-07T10:00:00Z', ended_at: null }, Date.parse('2026-10-07T10:20:00Z')), 20)
})
