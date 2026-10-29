<script setup lang="ts">
import HabitWeekView, { type GridHabitCell } from './HabitWeekView.vue'
import type { Habit } from '@/api/client'
import type { HabitCell } from '@/lib/habits'

export type { GridHabitCell }

defineProps<{
  weekLabel: string
  thisWeek: boolean
  todayDueCount: number
  todayDoneCount: number
  week: Date[]
  todayStr: string
  wdShortFmt: Intl.DateTimeFormat
  gridGroups: { label: string; habits: Habit[] }[]
  activeHabits: Habit[]
  focus: Habit | null
  streaks: Record<string, number>
  streakAtRisk: (h: Habit) => boolean
  meta: (h: Habit) => string
  gridCell: (h: Habit, d: Date) => GridHabitCell
  weekRates: (number | null)[]
  trend: { s: string; cur: boolean; pct: number | null }[]
  dateShortFmt: Intl.DateTimeFormat
}>()

const emit = defineEmits<{
  (e: 'prevWeek'): void
  (e: 'nextWeek'): void
  (e: 'thisWeekClick'): void
  (e: 'pick', habit: Habit): void
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
  (e: 'openNew'): void
}>()
</script>

<template>
  <HabitWeekView
    :week-label="weekLabel"
    :this-week="thisWeek"
    :today-due-count="todayDueCount"
    :today-done-count="todayDoneCount"
    :week="week"
    :today-str="todayStr"
    :wd-short-fmt="wdShortFmt"
    :grid-groups="gridGroups"
    :active-habits="activeHabits"
    :focus="focus"
    :streaks="streaks"
    :streak-at-risk="streakAtRisk"
    :meta="meta"
    :grid-cell="gridCell"
    :week-rates="weekRates"
    :trend="trend"
    :date-short-fmt="dateShortFmt"
    @prev-week="emit('prevWeek')"
    @next-week="emit('nextWeek')"
    @this-week-click="emit('thisWeekClick')"
    @pick="(h) => emit('pick', h)"
    @toggle-cell="(h, c) => emit('toggleCell', h, c)"
    @open-new="emit('openNew')"
  />
</template>
