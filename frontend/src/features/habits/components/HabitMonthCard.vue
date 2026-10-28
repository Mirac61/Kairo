<script setup lang="ts">
import type { Habit } from '@/api/client'
import { formatHabitRhythm, type HabitCell } from '@/lib/habits'
import { iconOf } from '@/lib/habitIcons'
import { habitColor } from '@/lib/projectColor'

defineProps<{
  habit: Habit
  stat: { done: number; goal: number }
  wdLabels: Record<string, string>
  weekdays: readonly string[]
  lead: number
  monthTitle: string
  cells: HabitCell[]
}>()

const emit = defineEmits<{
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
}>()
</script>

<template>
  <article
    class="hm-card"
    :class="{ inactive: !habit.active }"
    :style="{ '--hc': habitColor(habit.id) }"
  >
    <header class="hm-head">
      <span class="hk-icon sm" aria-hidden="true">
        <svg class="ic"><use :href="`#${iconOf(habit)}`" /></svg>
      </span>
      <span class="hk-id">
        <span class="hk-name">{{ habit.name }}</span>
        <span class="hab-sched">{{ formatHabitRhythm(habit, wdLabels) }}</span>
      </span>
      <span class="hm-stat">
        <b>{{ stat.done }}</b
        >/{{ stat.goal }}
      </span>
    </header>

    <div class="hm-grid" role="group" :aria-label="`${habit.name}, ${monthTitle}`">
      <span v-for="w in weekdays" :key="w" class="hm-wd" aria-hidden="true">
        {{ wdLabels[w]?.slice(0, 2) }}
      </span>
      <span v-for="i in lead" :key="`l${i}`" aria-hidden="true"></span>
      <button
        v-for="c in cells"
        :key="c.s"
        type="button"
        class="hm-day"
        :class="[c.state, { today: c.today }]"
        role="checkbox"
        :aria-checked="c.isDone"
        :aria-label="c.label"
        :title="c.label"
        :disabled="!c.can"
        @click="emit('toggleCell', habit, c)"
      >
        {{ Number(c.s.slice(8)) }}
      </button>
    </div>
  </article>
</template>

<style scoped>
.hm-card {
  padding: 0;
}
.hm-card.inactive {
  opacity: 0.55;
}
.hm-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.hk-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 9px;
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary));
  border: 0;
  background: color-mix(in srgb, var(--hc) 14%, var(--bg-2));
}
.hk-icon.sm {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  font-size: 13px;
}
.hk-icon .ic {
  width: 16px;
  height: 16px;
}
.hk-id {
  display: grid;
  gap: 3px;
  min-width: 0;
  flex: 1;
}
.hk-name {
  font: 500 15px/1.3 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hab-sched {
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hm-stat {
  font: 400 13px/1 var(--font-mono);
  color: var(--tx-muted);
}
.hm-stat b {
  font-size: 16px;
  color: var(--tx-primary);
}
.hm-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 4px;
}
.hm-wd {
  text-align: center;
  font: 500 11px/1.6 var(--font-ui);
  color: var(--tx-muted);
}
.hm-day {
  aspect-ratio: 1;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 7px;
  cursor: pointer;
  font: 500 11px/1 var(--font-ui);
  color: var(--tx-secondary);
  background: color-mix(in srgb, var(--hc) 10%, var(--bg-2));
}
.hm-day:hover:not(:disabled) {
  outline: 1.5px solid var(--tx-secondary);
}
.hm-day.done {
  background: color-mix(in srgb, var(--hc) 26%, var(--bg-1));
  color: var(--tx-primary);
  font-weight: 500;
  box-shadow: inset 0 -2px 0 var(--hc);
  text-decoration: none;
}
.hm-day.off {
  background: transparent;
  color: var(--tx-disabled);
}
.hm-day.pause {
  background: repeating-linear-gradient(
    135deg,
    color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px,
    transparent 2px 4px
  );
  color: var(--tx-muted);
}
.hm-day.pre,
.hm-day.future {
  background: transparent;
  color: var(--tx-disabled);
}
.hm-day.today {
  outline: 1.5px solid var(--tx-primary);
}
.hm-day:disabled {
  cursor: default;
}
</style>
