<script setup lang="ts">
import type { Habit } from '@/api/client'

defineProps<{
  habit: Habit
  icon: string
  ratio: number
  isCounted: boolean
  stat: string
  days: { d: string; on: boolean }[]
  ringSize: number
  ringRadius: number
  ringCirc: number
  dashOffset: number
  busy: boolean
}>()

const emit = defineEmits<{
  click: [e: MouseEvent]
  decrement: []
  edit: []
}>()
</script>

<template>
  <div class="rr-item">
    <button
      type="button"
      class="rr-btn"
      :class="{
        'is-done': ratio >= 1,
        'is-part': ratio > 0 && ratio < 1,
      }"
      :disabled="busy"
      :aria-pressed="!isCounted ? ratio >= 1 : undefined"
      :aria-label="`${habit.name} (${stat})`"
      :title="isCounted ? $t('Linksklick +1 · Rechtsklick / Shift −1') : habit.name"
      @click="emit('click', $event)"
      @contextmenu.prevent="emit('decrement')"
    >
      <span class="rr-ring" :style="{ width: `${ringSize}px`, height: `${ringSize}px` }">
        <span v-if="ratio >= 1" class="rr-glow" aria-hidden="true"></span>
        <svg :width="ringSize" :height="ringSize" class="rr-svg" aria-hidden="true">
          <circle :cx="ringSize / 2" :cy="ringSize / 2" :r="ringRadius" class="rr-track" />
          <circle
            v-if="ratio > 0"
            :cx="ringSize / 2"
            :cy="ringSize / 2"
            :r="ringRadius"
            class="rr-arc"
            :stroke-dasharray="ringCirc"
            :stroke-dashoffset="dashOffset"
          />
        </svg>
        <svg class="rr-ic" aria-hidden="true"><use :href="`#${icon}`" /></svg>
      </span>
      <span class="rr-meta">
        <span class="rr-name">{{ habit.name }}</span>
        <span class="rr-stat">{{ stat }}</span>
      </span>
      <span class="rr-dashes" aria-hidden="true">
        <i v-for="day in days" :key="day.d" :class="{ on: day.on }"></i>
      </span>
    </button>
    <button
      type="button"
      class="rr-edit"
      :aria-label="$t('„{name}“ bearbeiten', { name: habit.name })"
      @click.stop="emit('edit')"
    >
      <svg class="ic" aria-hidden="true"><use href="#i-pen" /></svg>
    </button>
  </div>
</template>

<style scoped>
.rr-item {
  position: relative;
  flex-shrink: 0;
  width: 112px;
  display: flex;
  justify-content: center;
}
:global(.rr-grid) .rr-item {
  width: auto;
}
.rr-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  padding: 2px 4px;
  color: inherit;
  cursor: pointer;
  min-width: 92px;
  text-decoration: none;
}
.rr-btn:focus-visible {
  outline: 1px solid var(--a-blue-hi);
  outline-offset: 4px;
  border-radius: 8px;
}
.rr-ring {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: transform 140ms ease;
}
.rr-btn:hover .rr-ring {
  transform: scale(1.04);
}
.rr-btn:active .rr-ring {
  transform: scale(0.96);
}
.rr-glow {
  position: absolute;
  inset: 4px;
  border-radius: 50%;
  background: var(--a-blue-hi);
  opacity: 0.14;
  pointer-events: none;
}
.rr-svg {
  position: absolute;
  inset: 0;
  transform: rotate(-90deg);
}
.rr-track {
  fill: none;
  stroke: var(--br-default);
  stroke-width: 2.2;
}
.rr-arc {
  fill: none;
  stroke: var(--a-blue-hi);
  stroke-width: 2.2;
  stroke-linecap: round;
  transition: stroke-dashoffset 280ms cubic-bezier(0.16, 1, 0.3, 1);
}
.rr-ic {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
  color: var(--tx-secondary);
  transition: color 140ms ease;
  position: relative;
  z-index: 1;
}
.rr-btn.is-part .rr-ic {
  color: var(--tx-primary);
}
.rr-btn.is-done .rr-ic {
  color: var(--a-blue-hi);
}
.rr-meta {
  display: flex;
  align-items: baseline;
  gap: 5px;
  max-width: 112px;
}
.rr-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--tx-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.rr-btn.is-done .rr-name {
  color: var(--tx-primary);
}
.rr-stat {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.rr-btn.is-done .rr-stat {
  color: var(--a-blue-hi);
}
.rr-dashes {
  display: flex;
  align-items: center;
  gap: 3px;
}
.rr-dashes i {
  display: block;
  width: 6px;
  height: 2px;
  border-radius: 1px;
  background: var(--br-default);
}
.rr-dashes i.on {
  background: var(--a-blue-hi);
  opacity: 0.85;
}
.rr-edit {
  position: absolute;
  top: -2px;
  right: 8px;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  border: 1px solid var(--br-default);
  background: var(--bg-1);
  color: var(--tx-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  cursor: pointer;
  transition:
    opacity 140ms ease,
    color 140ms ease;
}
.rr-item:hover .rr-edit,
.rr-edit:focus-visible {
  opacity: 1;
}
@media (hover: none) {
  .rr-edit {
    opacity: 1;
  }
}
.rr-edit:hover {
  color: var(--tx-primary);
}
.rr-edit .ic {
  width: 11px;
  height: 11px;
}
</style>
