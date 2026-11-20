<script setup lang="ts">
import { computed } from 'vue'
import type { Goal } from '@/api/client'
import DeleteButton from '@/components/DeleteButton.vue'
import { formatDeadline } from '../goalUtils'

const props = defineProps<{
  goal: Goal
  progressPercent: number
  doneStepsCount: number
  totalStepsCount: number
  weeklyText: string
  projectName: string
  habitName: string
}>()

const emit = defineEmits<{
  edit: [goal: Goal]
  archive: [goal: Goal, archived: boolean]
  delete: [goal: Goal]
}>()

const RING_SIZE = 44
const RING_R = (RING_SIZE - 5) / 2
const RING_C = 2 * Math.PI * RING_R
const ringOffset = computed(() => RING_C * (1 - props.progressPercent / 100))
const deadlineText = computed(() => formatDeadline(props.goal.due_date))
</script>

<template>
  <aside class="gdm-rail">
    <div class="gdm-rail-top">
      <div class="gdm-ring-head">
        <div class="gdm-ring-wrap">
          <svg :width="RING_SIZE" :height="RING_SIZE" class="gdm-ring-svg" aria-hidden="true">
            <circle :cx="RING_SIZE / 2" :cy="RING_SIZE / 2" :r="RING_R" class="gdm-ring-track" />
            <circle
              :cx="RING_SIZE / 2"
              :cy="RING_SIZE / 2"
              :r="RING_R"
              class="gdm-ring-arc"
              :stroke-dasharray="RING_C"
              :stroke-dashoffset="ringOffset"
            />
          </svg>
          <span class="gdm-ring-num" :aria-label="`${progressPercent} %`">{{
            progressPercent
          }}</span>
        </div>
        <div>
          <div class="gdm-ring-val">
            {{
              totalStepsCount
                ? `${doneStepsCount} / ${totalStepsCount} ${$t('Schritte')}`
                : `${progressPercent} %`
            }}
          </div>
          <div class="gdm-ring-sub">{{ $t('abgeschlossen') }}</div>
        </div>
      </div>

      <dl class="gdm-props">
        <div v-if="weeklyText">
          <dt>{{ $t('Diese Woche') }}</dt>
          <dd class="is-accent">{{ weeklyText }}</dd>
        </div>
        <div>
          <dt>{{ $t('Frist') }}</dt>
          <dd class="is-mono">{{ deadlineText }}</dd>
        </div>
        <div>
          <dt>{{ $t('Projekt') }}</dt>
          <dd>{{ projectName }}</dd>
        </div>
        <div>
          <dt>{{ $t('Gewohnheit') }}</dt>
          <dd>{{ habitName }}</dd>
        </div>
      </dl>
    </div>

    <div class="gdm-rail-actions">
      <button type="button" class="gdm-rail-btn" @click="emit('edit', goal)">
        {{ $t('Eigenschaften bearbeiten') }}
      </button>
      <button type="button" class="gdm-rail-btn" @click="emit('archive', goal, !goal.archived)">
        {{ goal.archived ? $t('Wieder aktivieren') : $t('Als erreicht markieren') }}
      </button>
      <DeleteButton
        ghost
        class="gdm-del-btn"
        :label="$t('Ziel löschen')"
        :text="
          totalStepsCount
            ? $t('„{title}“ samt Teilzielen löschen?', { title: goal.title })
            : $t('„{title}“ löschen?', { title: goal.title })
        "
        @confirm="emit('delete', goal)"
      />
    </div>
  </aside>
</template>

<style scoped>
.gdm-rail {
  width: 268px;
  flex-shrink: 0;
  border-left: 1px solid var(--br-subtle);
  background: var(--bg-0);
  padding: 24px 20px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 24px;
}
.gdm-ring-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.gdm-ring-wrap {
  position: relative;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.gdm-ring-svg {
  position: absolute;
  inset: 0;
  transform: rotate(-90deg);
}
.gdm-ring-track {
  fill: none;
  stroke: var(--br-default);
  stroke-width: 2.4;
}
.gdm-ring-arc {
  fill: none;
  stroke: var(--a-blue-hi);
  stroke-width: 2.4;
  stroke-linecap: round;
  transition: stroke-dashoffset 280ms var(--ease);
}
.gdm-ring-num {
  font: 600 12px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-primary);
}
.gdm-ring-val {
  font: 600 14px/1.25 var(--font-ui);
  color: var(--tx-primary);
}
.gdm-ring-sub {
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.gdm-props {
  margin: 0;
  border-top: 1px solid var(--br-subtle);
}
.gdm-props > div {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  padding: 9px 0;
  border-bottom: 1px solid var(--br-subtle);
  font: 400 12.5px/1.35 var(--font-ui);
}
.gdm-props dt {
  color: var(--tx-muted);
  flex-shrink: 0;
}
.gdm-props dd {
  margin: 0;
  color: var(--tx-primary);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
}
.gdm-props dd.is-accent {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.gdm-props dd.is-mono {
  font-family: var(--font-mono);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.gdm-rail-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.gdm-rail-btn {
  width: 100%;
  height: 32px;
  padding: 0 12px;
  border-radius: var(--r-s);
  border: 1px solid var(--br-default);
  background: var(--bg-1);
  color: var(--tx-secondary);
  font: 500 12.5px/1 var(--font-ui);
  text-align: left;
  cursor: pointer;
  transition:
    background var(--dur) var(--ease),
    color var(--dur) var(--ease),
    border-color var(--dur) var(--ease);
}
.gdm-rail-btn:hover {
  background: var(--bg-2);
  color: var(--tx-primary);
  border-color: var(--br-strong);
}
.gdm-del-btn {
  width: 100%;
  justify-content: flex-start;
}
@media (max-width: 780px) {
  .gdm-rail {
    width: auto;
    border-left: 0;
    border-top: 1px solid var(--br-subtle);
    padding: 18px 20px;
  }
}
</style>
