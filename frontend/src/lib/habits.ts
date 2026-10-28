import type { Habit, HabitPause } from '@/api/client'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'

export const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'] as const
export type Weekday = (typeof WEEKDAYS)[number]

export const WEEKS_IN_GRAPH = 53

export type HabitRhythm = 'DAILY' | 'SPECIFIC_WEEKDAYS' | 'TIMES_PER_WEEK'

export function getWeekdayLabels(localeStr: string): Record<string, string> {
  const wdFmt = new Intl.DateTimeFormat(localeStr, { weekday: 'short' })
  // 01.01.2024 war ein Montag.
  return Object.fromEntries(WEEKDAYS.map((d, i) => [d, wdFmt.format(new Date(2024, 0, 1 + i))]))
}

export function getGridStartDate(): Date {
  return addDays(weekStart(), -7 * (WEEKS_IN_GRAPH - 1))
}

export function isHabitPlanned(habit: Habit, date: Date): boolean {
  const wd = WEEKDAYS[(date.getDay() + 6) % 7]!
  if (habit.frequency_type === 'SPECIFIC_WEEKDAYS') {
    return (habit.frequency_config.weekdays ?? []).includes(wd)
  }
  if (habit.frequency_type === 'WEEKLY' && habit.frequency_config.weekday) {
    return habit.frequency_config.weekday === wd
  }
  return true
}

export function getPausesForHabit(pauses: HabitPause[], habitId?: string): HabitPause[] {
  return pauses.filter((p) => !p.habit_id || (habitId && p.habit_id === habitId))
}

export function isHabitPausedOn(pauses: HabitPause[], habitId: string, dateStr: string): boolean {
  const activePauses = getPausesForHabit(pauses, habitId)
  return activePauses.some((p) => dateStr >= p.start_date && dateStr <= p.end_date)
}

export type HabitCellState = 'done' | 'open' | 'off' | 'pre' | 'future' | 'pause'

export interface HabitCell {
  s: string
  state: HabitCellState
  isDone: boolean
  can: boolean
  today: boolean
  label: string
}

export function buildHabitCell(
  habit: Habit,
  date: Date,
  doneSet: Set<string> | undefined,
  pauses: HabitPause[],
  todayStr: string,
  dayLabelFmt: Intl.DateTimeFormat,
): HabitCell {
  const s = ymd(date)
  const isDone = !!doneSet?.has(s)
  let state: HabitCellState

  if (s > todayStr) {
    state = 'future'
  } else if (isDone) {
    state = 'done'
  } else if (s < habit.start_date || (habit.end_date && s > habit.end_date)) {
    state = 'pre'
  } else if (!isHabitPlanned(habit, date)) {
    state = 'off'
  } else if (isHabitPausedOn(pauses, habit.id, s)) {
    state = 'pause'
  } else {
    state = 'open'
  }

  const can = habit.active && state !== 'future' && state !== 'pre'
  const stateLabel = isDone ? t('erledigt') : state === 'pause' ? t('ausgesetzt') : t('offen')

  return {
    s,
    state,
    isDone,
    can,
    today: s === todayStr,
    label: `${dayLabelFmt.format(date)}: ${stateLabel}`,
  }
}

export function formatHabitRhythm(habit: Habit, wdLabel: Record<string, string>): string {
  const c = habit.frequency_config
  if (habit.frequency_type === 'WEEKLY') {
    return c.weekday ? (wdLabel[c.weekday] ?? t('Wöchentlich')) : t('Wöchentlich')
  }
  if (habit.frequency_type === 'SPECIFIC_WEEKDAYS') {
    return (c.weekdays ?? []).map((d) => wdLabel[d] ?? d).join(', ')
  }
  if (habit.frequency_type === 'TIMES_PER_WEEK') {
    return t('{n}× pro Woche', { n: c.times ?? 0 })
  }
  return t('Täglich')
}

export function formatHabitDescription(habit: Habit, wdLabel: Record<string, string>): string {
  const parts = [
    formatHabitRhythm(habit, wdLabel),
    habit.preferred_time,
    habit.target_value ? `${habit.target_value} ${habit.unit}`.trim() : null,
  ]
  return parts.filter(Boolean).join(' · ')
}

export function formatPauseSpan(pause: HabitPause, dateShortFmt: Intl.DateTimeFormat): string {
  const fmt = (iso: string) => dateShortFmt.format(new Date(`${iso}T12:00:00`))
  return pause.start_date === pause.end_date
    ? fmt(pause.start_date)
    : `${fmt(pause.start_date)} – ${fmt(pause.end_date)}`
}

export function getUpcomingPauses(pauses: HabitPause[], todayStr: string): HabitPause[] {
  return pauses.filter((p) => p.end_date >= todayStr)
}

export function computeHabit30DayRate(
  habit: Habit,
  doneSet: Set<string> | undefined,
  pauses: HabitPause[],
  todayStr: string = ymd(new Date()),
): number | null {
  const todayDate = new Date(`${todayStr}T12:00:00`)
  const days = Array.from({ length: 30 }, (_, i) => addDays(todayDate, -i)).filter(
    (d) => ymd(d) >= habit.start_date && !isHabitPausedOn(pauses, habit.id, ymd(d)),
  )
  if (!days.length) return null

  const hits = days.filter((d) => doneSet?.has(ymd(d))).length
  const goal =
    habit.frequency_type === 'TIMES_PER_WEEK'
      ? ((habit.frequency_config.times ?? 1) * days.length) / 7
      : days.filter((d) => isHabitPlanned(habit, d)).length

  return goal > 0 ? Math.min(100, Math.round((hits / goal) * 100)) : null
}

export function computeWeekProgress(
  habit: Habit,
  doneSet: Set<string> | undefined,
  pauses: HabitPause[],
  weekDays: Date[],
): { done: number; target: number } {
  const done = weekDays.filter((d) => doneSet?.has(ymd(d))).length
  if (habit.frequency_type === 'TIMES_PER_WEEK') {
    return { done, target: habit.frequency_config.times ?? 1 }
  }
  const target = weekDays.filter((d) => {
    const s = ymd(d)
    return (
      isHabitPlanned(habit, d) &&
      s >= habit.start_date &&
      (!habit.end_date || s <= habit.end_date) &&
      !isHabitPausedOn(pauses, habit.id, s)
    )
  }).length
  return { done, target }
}

export function computeLongestStreak(cells: HabitCell[], currentStreak: number): number {
  let best = 0
  let cur = 0
  for (const c of cells) {
    if (c.state === 'done') {
      cur++
      best = Math.max(best, cur)
    } else if (c.state === 'open' && !c.today) {
      cur = 0
    }
  }
  return Math.max(best, currentStreak)
}

export function computeWeekdayRates(cells: HabitCell[]): { w: Weekday; pct: number | null }[] {
  return WEEKDAYS.map((w, i) => {
    const weekdayCells = cells.filter(
      (c, j) => j % 7 === i && (c.state === 'done' || (c.state === 'open' && !c.today)),
    )
    if (!weekdayCells.length) return { w, pct: null }
    const doneCount = weekdayCells.filter((c) => c.isDone).length
    return { w, pct: Math.round((doneCount / weekdayCells.length) * 100) }
  })
}
