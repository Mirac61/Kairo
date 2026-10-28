import assert from 'node:assert/strict'
import { test } from 'node:test'
import { weekStart, ymd } from '../src/lib/dates.ts'
import { prefs } from '../src/lib/prefs.ts'

test('weekStart beachtet prefs.weekStart', () => {
  // 2026-10-28 ist ein Mittwoch
  const wed = new Date(2026, 9, 28) // Monat 9 = Oktober (0-basiert)

  // Standard / Montag (weekStart = 1)
  prefs.weekStart = 1
  const mon = weekStart(wed)
  assert.equal(ymd(mon), '2026-10-26') // Montag der Woche

  // Sonntag (weekStart = 0)
  prefs.weekStart = 0
  const sun = weekStart(wed)
  assert.equal(ymd(sun), '2026-10-25') // Sonntag der Woche

  // Test an einem Sonntag (2026-11-01)
  const sundayDate = new Date(2026, 10, 1)
  prefs.weekStart = 1
  assert.equal(ymd(weekStart(sundayDate)), '2026-10-26') // Gehört bei Mo-Start zur vorherigen Woche

  prefs.weekStart = 0
  assert.equal(ymd(weekStart(sundayDate)), '2026-11-01') // Ist bei So-Start der Wochenbeginn

  // Zurücksetzen auf Standard
  prefs.weekStart = 1
})
