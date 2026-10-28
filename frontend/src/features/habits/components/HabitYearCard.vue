<script setup lang="ts">
import { ref } from 'vue'
import type { Habit } from '@/api/client'
import { formatHabitDescription, type HabitCell } from '@/lib/habits'
import { iconOf } from '@/lib/habitIcons'
import { habitColor } from '@/lib/projectColor'

defineProps<{
  habit: Habit
  months: { col: number; label: string }[]
  streak: number
  rate30: number | null
  yearCount: number
  isDue: boolean
  isDone: boolean
  cells: HabitCell[]
  wdLabels: Record<string, string>
}>()

const emit = defineEmits<{
  (e: 'toggleToday', habit: Habit): void
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
  (e: 'edit', habit: Habit): void
  (e: 'toggleActive', habit: Habit): void
  (e: 'trash', habit: Habit): void
}>()

const moreMenuOpen = ref(false)

function onToggleActive(h: Habit) {
  moreMenuOpen.value = false
  emit('toggleActive', h)
}

function onTrash(h: Habit) {
  moreMenuOpen.value = false
  emit('trash', h)
}
</script>

<template>
  <article
    class="hk-card"
    :class="{ inactive: !habit.active }"
    :style="{ '--hc': habitColor(habit.id) }"
  >
    <header class="hk-head">
      <span class="hk-icon" aria-hidden="true">
        <svg class="ic"><use :href="`#${iconOf(habit)}`" /></svg>
      </span>
      <div class="hk-id">
        <span class="hk-name">{{ habit.name }}</span>
        <span class="hab-sub">
          <span class="hab-sched">
            {{ habit.description || formatHabitDescription(habit, wdLabels) }}
            <template v-if="!habit.active"> {{ $t('· inaktiv') }}</template>
          </span>
          <span class="acts">
            <button class="btn btn-ghost" type="button" @click="emit('edit', habit)">
              {{ $t('Bearbeiten') }}
            </button>
            <span class="menu-wrap" @keydown.esc="moreMenuOpen = false">
              <button
                class="btn btn-ghost hd-more"
                type="button"
                :aria-label="$t('Weitere Aktionen')"
                :aria-expanded="moreMenuOpen"
                @click="moreMenuOpen = !moreMenuOpen"
              >
                <svg class="ic"><use href="#i-more" /></svg>
              </button>
              <template v-if="moreMenuOpen">
                <span class="menu-scrim" @click="moreMenuOpen = false"></span>
                <span class="menu">
                  <button class="menu-i" type="button" @click="onToggleActive(habit)">
                    {{ habit.active ? $t('Deaktivieren') : $t('Aktivieren') }}
                  </button>
                  <button class="menu-i danger" type="button" @click="onTrash(habit)">
                    {{ $t('Löschen') }}
                  </button>
                </span>
              </template>
            </span>
          </span>
        </span>
      </div>
      <div class="hk-chips">
        <span v-if="habit.description" class="chip">{{
          formatHabitDescription(habit, wdLabels)
        }}</span>
        <span class="hk-kpi"
          ><b>{{ streak }}</b
          >{{ $t('Serie') }}</span
        >
        <span class="hk-kpi">
          <b>{{ rate30 ?? '–' }}<template v-if="rate30 !== null"> %</template></b>
          {{ $t('30 Tage') }}
        </span>
        <span class="hk-kpi"
          ><b>{{ yearCount }}</b
          >{{ $t('im Jahr') }}</span
        >
      </div>
      <button
        type="button"
        class="hk-check"
        role="checkbox"
        :aria-checked="isDone"
        :disabled="!isDue"
        :aria-label="$t('Heute abhaken: {name}', { name: habit.name })"
        :title="
          isDue ? $t('Heute abhaken: {name}', { name: habit.name }) : $t('heute nicht fällig')
        "
        @click="emit('toggleToday', habit)"
      >
        <svg class="ic" aria-hidden="true"><use href="#i-check" /></svg>
      </button>
    </header>

    <div
      class="hk-graph"
      role="group"
      :aria-label="$t('Letztes Jahr: {n} Tage erledigt', { n: yearCount })"
    >
      <div class="hk-months" aria-hidden="true">
        <span v-for="m in months" :key="m.col" :style="{ gridColumn: m.col }">{{ m.label }}</span>
      </div>
      <div class="hk-grid">
        <i
          v-for="c in cells"
          :key="c.s"
          :class="[c.state, { today: c.today }]"
          :title="c.state === 'future' || c.state === 'pre' ? undefined : c.label"
          :role="c.can ? 'checkbox' : undefined"
          :aria-checked="c.can ? c.isDone : undefined"
          :aria-label="c.can ? c.label : undefined"
          :aria-hidden="c.can ? undefined : 'true'"
          @click="emit('toggleCell', habit, c)"
        ></i>
      </div>
    </div>
  </article>
</template>

<style scoped>
.hk-card {
  padding: 20px 0 18px;
  border-bottom: 1px solid var(--br-subtle);
}
.hk-card.inactive {
  opacity: 0.55;
}
.hk-head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 14px;
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
.hk-icon .ic {
  width: 18px;
  height: 18px;
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
.hab-sub {
  position: relative;
  display: flex;
  align-items: center;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
  min-height: 24px;
}
.hab-sched {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: opacity 120ms ease;
}
.acts {
  position: absolute;
  left: 0;
  top: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease;
}
.hk-card:hover .acts,
.hk-card:focus-within .acts {
  opacity: 1;
  pointer-events: auto;
}
.hk-card:hover .hab-sched,
.hk-card:focus-within .hab-sched {
  opacity: 0;
}
.acts .btn {
  height: 24px;
  padding: 0 8px;
  font-size: 11px;
}
.acts .hd-more {
  width: 24px;
  padding: 0;
  display: inline-grid;
  place-items: center;
}
.acts .hd-more .ic {
  width: 14px;
  height: 14px;
}
.hk-chips {
  display: flex;
  align-items: center;
  gap: 16px;
  flex: none;
}
.hk-chips .chip {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hk-kpi {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font: 400 11px/1.2 var(--font-ui);
  color: var(--tx-muted);
  min-width: 44px;
}
.hk-kpi b {
  font: 600 15px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-primary);
}
.hk-check {
  flex: none;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  padding: 0;
  border-radius: 50%;
  border: 1px solid var(--br-default);
  background: var(--bg-1);
  color: var(--tx-muted);
  cursor: pointer;
  transition: all var(--dur);
}
.hk-check:hover:not(:disabled) {
  border-color: var(--hc);
  color: var(--hc);
  background: color-mix(in srgb, var(--hc) 10%, transparent);
}
.hk-check[aria-checked='true'] {
  background: var(--hc);
  border-color: var(--hc);
  color: var(--bg-1);
}
.hk-check:disabled {
  opacity: 0.35;
  cursor: default;
}
.hk-check .ic {
  width: 16px;
  height: 16px;
}
.hk-graph {
  display: grid;
  gap: 4px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.hk-months,
.hk-grid {
  display: grid;
  grid-template-rows: repeat(7, 10px);
  grid-auto-flow: column;
  grid-auto-columns: 10px;
  gap: 3px;
}
.hk-months {
  grid-template-rows: auto;
  margin-bottom: 2px;
}
.hk-months span {
  font: 400 10px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hk-grid {
  grid-template-rows: repeat(7, 10px);
}
.hk-grid i {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background: var(--bg-2);
}
.hk-grid i.done {
  background: var(--hc);
}
.hk-grid i.off {
  background: transparent;
}
.hk-grid i.pause {
  background: repeating-linear-gradient(
    45deg,
    var(--bg-2),
    var(--bg-2) 2px,
    var(--br-default) 2px,
    var(--br-default) 4px
  );
}
.hk-grid i.pre {
  background: transparent;
}
.hk-grid i.future {
  background: transparent;
}
.hk-grid i.today {
  box-shadow: 0 0 0 1.5px var(--tx-primary);
}
.hk-grid i[role='checkbox'] {
  cursor: pointer;
}
.hk-grid i[role='checkbox']:hover {
  outline: 1.5px solid var(--hc);
}
</style>
