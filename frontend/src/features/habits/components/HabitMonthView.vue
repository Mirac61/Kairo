<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Habit } from '@/api/client'
import { WEEKDAYS, type HabitCell } from '@/lib/habits'
import { locale } from '@/lib/i18n'
import HabitMonthCard from './HabitMonthCard.vue'

const props = defineProps<{
  habits: Habit[]
  cell: (h: Habit, d: Date) => HabitCell
  wdLabels: Record<string, string>
}>()

const emit = defineEmits<{
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
}>()

const monthOffset = ref(0)

const monthStart = computed(() => {
  const n = new Date()
  return new Date(n.getFullYear(), n.getMonth() + monthOffset.value, 1)
})

const monthTitle = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(
    monthStart.value,
  ),
)

const monthDays = computed(() => {
  const m = monthStart.value
  return Array.from(
    { length: new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate() },
    (_, i) => new Date(m.getFullYear(), m.getMonth(), i + 1),
  )
})

const lead = computed(() => (monthStart.value.getDay() + 6) % 7)

function monthStat(h: Habit) {
  const cs = monthDays.value.map((d) => props.cell(h, d))
  return {
    done: cs.filter((c) => c.isDone).length,
    goal: cs.filter((c) => c.state === 'open' || c.state === 'done').length,
  }
}
</script>

<template>
  <div class="hm">
    <div class="hm-nav">
      <button
        class="icon-btn"
        type="button"
        :aria-label="$t('Voriger Monat')"
        :disabled="monthOffset <= -11"
        @click="monthOffset--"
      >
        <svg class="ic"><use href="#i-left" /></svg>
      </button>
      <h2 class="hd-title">{{ monthTitle }}</h2>
      <button
        class="icon-btn"
        type="button"
        :aria-label="$t('Nächster Monat')"
        :disabled="monthOffset >= 0"
        @click="monthOffset++"
      >
        <svg class="ic"><use href="#i-right" /></svg>
      </button>
    </div>

    <div class="hm-cards">
      <HabitMonthCard
        v-for="h in habits"
        :key="h.id"
        :habit="h"
        :stat="monthStat(h)"
        :wd-labels="wdLabels"
        :weekdays="WEEKDAYS"
        :lead="lead"
        :month-title="monthTitle"
        :cells="monthDays.map((d) => cell(h, d))"
        @toggle-cell="(habit, c) => emit('toggleCell', habit, c)"
      />
    </div>
  </div>
</template>

<style scoped>
.hd-title {
  margin: 0 0 14px;
  font: 600 18px/1.2 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}
.hm-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.hm-nav .hd-title {
  margin: 0;
  min-width: 170px;
  text-align: center;
}
.hm-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 36px 40px;
}
</style>
