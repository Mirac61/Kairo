<script setup lang="ts">
import { computed } from 'vue'
import type { Habit, TodayHabit } from '@/api/client'
import { addDays } from '@/lib/dates'
import { getGridStartDate, WEEKS_IN_GRAPH, type HabitCell } from '@/lib/habits'
import { locale } from '@/lib/i18n'
import HabitYearCard from './HabitYearCard.vue'

const props = defineProps<{
  habits: Habit[]
  streaks: Record<string, number>
  rate30: (h: Habit) => number | null
  yearCount: (h: Habit) => number
  due: (h: Habit) => TodayHabit | undefined
  graph: (h: Habit) => HabitCell[]
  wdLabels: Record<string, string>
}>()

const emit = defineEmits<{
  (e: 'toggleToday', habit: Habit): void
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
  (e: 'edit', habit: Habit): void
  (e: 'toggleActive', habit: Habit): void
  (e: 'trash', habit: Habit): void
}>()

const monthShortFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { month: 'short' }),
)

const months = computed(() => {
  const gStart = getGridStartDate()
  const fmt = monthShortFmt.value
  return Array.from({ length: WEEKS_IN_GRAPH }, (_, w) => addDays(gStart, 7 * w))
    .map((d, w, all) => ({
      col: w + 1,
      label: w === 0 || d.getMonth() !== all[w - 1]!.getMonth() ? fmt.format(d) : '',
    }))
    .filter((m) => m.label && m.col < WEEKS_IN_GRAPH - 1)
    .filter((m, i, all) => !all[i + 1] || all[i + 1]!.col - m.col >= 3)
})
</script>

<template>
  <div class="hk-list">
    <HabitYearCard
      v-for="h in habits"
      :key="h.id"
      :habit="h"
      :months="months"
      :streak="streaks[h.id] ?? 0"
      :rate30="rate30(h)"
      :year-count="yearCount(h)"
      :is-due="Boolean(due(h))"
      :is-done="Boolean(due(h)?.done)"
      :cells="graph(h)"
      :wd-labels="props.wdLabels"
      @toggle-today="(habit) => emit('toggleToday', habit)"
      @toggle-cell="(habit, cell) => emit('toggleCell', habit, cell)"
      @edit="(habit) => emit('edit', habit)"
      @toggle-active="(habit) => emit('toggleActive', habit)"
      @trash="(habit) => emit('trash', habit)"
    />

    <div class="hab-foot">
      <span class="hab-legend"><i class="done"></i>{{ $t('erledigt') }}</span>
      <span class="hab-legend"><i></i>{{ $t('offen') }}</span>
      <span class="hab-legend"><i class="off"></i>{{ $t('nicht geplant') }}</span>
      <span class="hab-legend"><i class="pause"></i>{{ $t('ausgesetzt') }}</span>
      <span class="hab-hint">{{ $t('Klick auf ein Feld trägt nach oder nimmt zurück') }}</span>
    </div>
  </div>
</template>

<style scoped>
.hk-list {
  display: grid;
}
.hab-foot {
  border-top: 0;
  padding-top: 4px;
}
.hab-foot .hab-legend i.pause {
  border: 0;
  background: repeating-linear-gradient(135deg, var(--tx-muted) 0 2px, transparent 2px 4px);
}
</style>
