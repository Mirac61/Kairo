<script setup lang="ts">
import { computed } from 'vue'
import type { Goal } from '@/api/client'
import { filesUrl } from '@/lib/noteFiles'
import { prefs } from '@/lib/prefs'
import { isVideoPath } from '../goalUtils'

export interface GoalCardVm {
  goal: Goal
  percent: number
  weeklyText: string
  dueShort: string
  doneSteps: number
  totalSteps: number
  nextStepTitle: string | null
}

defineProps<{
  cards: GoalCardVm[]
  activeCount: number
  reachedCount: number
  showReached: boolean
  hoveredProjectId: string | null
}>()

const emit = defineEmits<{
  'update:showReached': [val: boolean]
  'update:hoveredProjectId': [id: string | null]
  select: [id: string]
}>()

const RING_SIZE = 38
const RING_R = (RING_SIZE - 4.5) / 2
const RING_C = 2 * Math.PI * RING_R
const ringOffset = (pct: number) => RING_C * (1 - pct / 100)

const reducedMotion = computed(
  () =>
    prefs.motion ||
    (typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches),
)
</script>

<template>
  <div class="pr-goals-block">
    <div class="pr-sec-head">
      <div class="pr-goals-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          class="pr-tab"
          :class="{ on: !showReached }"
          :aria-selected="!showReached"
          @click="emit('update:showReached', false)"
        >
          {{ $t('Aktive Ziele') }}
          <span class="pr-tab-cnt">{{ activeCount }}</span>
        </button>
        <button
          type="button"
          role="tab"
          class="pr-tab"
          :class="{ on: showReached }"
          :aria-selected="showReached"
          @click="emit('update:showReached', true)"
        >
          {{ $t('Erreichte Ziele') }}
          <span class="pr-tab-cnt">{{ reachedCount }}</span>
        </button>
      </div>
    </div>

    <div v-if="cards.length" class="pr-goal-list">
      <div
        v-for="c in cards"
        :key="c.goal.id"
        class="pr-goal"
        :class="{
          linked: hoveredProjectId !== null && hoveredProjectId === c.goal.project_id,
          dimmed: hoveredProjectId !== null && c.goal.project_id !== hoveredProjectId,
        }"
        role="button"
        tabindex="0"
        @click="emit('select', c.goal.id)"
        @keydown.enter.prevent="emit('select', c.goal.id)"
        @keydown.space.prevent="emit('select', c.goal.id)"
        @mouseenter="c.goal.project_id && emit('update:hoveredProjectId', c.goal.project_id)"
        @mouseleave="emit('update:hoveredProjectId', null)"
        @focus="c.goal.project_id && emit('update:hoveredProjectId', c.goal.project_id)"
        @blur="emit('update:hoveredProjectId', null)"
      >
        <div class="pr-goal-top">
          <div class="pr-ring">
            <svg :width="RING_SIZE" :height="RING_SIZE" class="pr-ring-svg" aria-hidden="true">
              <circle :cx="RING_SIZE / 2" :cy="RING_SIZE / 2" :r="RING_R" class="pr-ring-track" />
              <circle
                :cx="RING_SIZE / 2"
                :cy="RING_SIZE / 2"
                :r="RING_R"
                class="pr-ring-arc"
                :stroke-dasharray="RING_C"
                :stroke-dashoffset="ringOffset(c.percent)"
              />
            </svg>
            <span class="pr-ring-num" :aria-label="`${c.percent} %`">{{ c.percent }}</span>
          </div>

          <div class="pr-goal-info">
            <div class="pr-goal-trow">
              <span class="pr-goal-title">{{ c.goal.title }}</span>
              <div class="pr-goal-right-meta">
                <div v-if="c.goal.image_path" class="pr-thumb">
                  <video
                    v-if="isVideoPath(c.goal.image_path)"
                    :src="filesUrl(c.goal.image_path)"
                    :autoplay="!reducedMotion"
                    loop
                    muted
                    playsinline
                  ></video>
                  <img v-else :src="filesUrl(c.goal.image_path)" alt="" />
                </div>
                <span class="pr-arrow" aria-hidden="true">↗</span>
              </div>
            </div>
            <div class="pr-goal-meta">
              <span v-if="c.weeklyText" class="pr-weekly">{{ c.weeklyText }}</span>
              <span v-if="c.weeklyText && c.dueShort" class="pr-sep">·</span>
              <span v-if="c.dueShort">{{ c.dueShort }}</span>
              <span v-if="c.totalSteps && (c.weeklyText || c.dueShort)" class="pr-sep">·</span>
              <span v-if="c.totalSteps">{{ c.doneSteps }}/{{ c.totalSteps }}</span>
            </div>
          </div>
        </div>

        <div v-if="c.nextStepTitle" class="pr-next-step">
          <span class="pr-muted">→</span> {{ c.nextStepTitle }}
        </div>
      </div>
    </div>

    <p v-else class="pr-empty">
      {{
        showReached
          ? $t('Noch keine erreichten Ziele.')
          : $t(
              'Noch keine Ziele. Lege dein erstes Ziel an – Fortschritt und Schritte erscheinen hier.',
            )
      }}
    </p>
  </div>
</template>

<style scoped>
.pr-goals-block {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.pr-sec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.pr-goals-tabs {
  display: flex;
  align-items: center;
  gap: 16px;
}
.pr-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 0 6px;
  border: 0;
  border-bottom: 1.5px solid transparent;
  background: none;
  color: var(--tx-muted);
  font: 500 12.5px/1 var(--font-ui);
  cursor: pointer;
  transition:
    color var(--dur) var(--ease),
    border-color var(--dur) var(--ease);
}
.pr-tab:hover {
  color: var(--tx-secondary);
}
.pr-tab.on {
  color: var(--tx-primary);
  border-bottom-color: var(--tx-primary);
}
.pr-tab-cnt {
  font-family: var(--font-mono);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.pr-goal-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.pr-goal {
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--br-subtle);
  background: var(--bg-card);
  cursor: pointer;
  transition:
    background 150ms ease,
    border-color 150ms ease,
    opacity 150ms ease;
}
.pr-goal:hover,
.pr-goal.linked {
  background: color-mix(in oklab, var(--tx-primary) 4%, var(--bg-card));
  border-color: var(--br-default);
}
.pr-goal.linked {
  border-color: color-mix(in oklab, var(--a-blue-hi) 45%, var(--br-default));
}
.pr-goal.dimmed {
  opacity: 0.45;
}
.pr-goal:focus-visible {
  outline: 1.5px solid var(--a-focus);
  outline-offset: 2px;
}
.pr-goal-top {
  display: flex;
  align-items: center;
  gap: 14px;
}
.pr-ring {
  position: relative;
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.pr-ring-svg {
  position: absolute;
  inset: 0;
  transform: rotate(-90deg);
}
.pr-ring-track {
  fill: none;
  stroke: var(--br-default);
  stroke-width: 2.2;
}
.pr-ring-arc {
  fill: none;
  stroke: var(--a-blue-hi);
  stroke-width: 2.2;
  stroke-linecap: round;
  transition: stroke-dashoffset 260ms var(--ease);
}
.pr-ring-num {
  font: 600 11px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-primary);
}
.pr-goal-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.pr-goal-trow {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.pr-goal-title {
  font: 500 14px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pr-goal-right-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.pr-thumb {
  width: 34px;
  height: 22px;
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid var(--br-subtle);
}
.pr-thumb img,
.pr-thumb video {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.pr-arrow {
  color: var(--tx-muted);
  font-size: 13px;
  transition:
    transform 140ms ease,
    color 140ms ease;
}
.pr-goal:hover .pr-arrow {
  color: var(--tx-primary);
  transform: translate(1px, -1px);
}
.pr-goal-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font: 400 12px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.pr-weekly {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.pr-sep {
  opacity: 0.5;
}
.pr-next-step {
  margin-top: 10px;
  padding-top: 9px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12.5px/1.35 var(--font-ui);
  color: var(--tx-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pr-muted {
  color: var(--tx-muted);
}
.pr-empty {
  margin: 0;
  padding: 20px 0;
  font: 400 13px/1.55 var(--font-ui);
  color: var(--tx-muted);
}
</style>
