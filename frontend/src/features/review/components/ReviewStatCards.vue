<script setup lang="ts">
import { computed } from 'vue'
import type { Review, Task } from '@/api/client'
import { hm } from '@/lib/dates'

const props = defineProps<{
  review: Review
  planToDate: number
  weekTasksCount: number
  overdue: Task[]
}>()

const calendarMinutes = computed(
  () => props.review.days.reduce((s, d) => s + d.calendar_minutes, 0),
)

const trackedPct = computed(() =>
  props.planToDate
    ? Math.round(((props.review.tracked_minutes ?? 0) / props.planToDate) * 100)
    : 0,
)
</script>

<template>
  <!-- Kennzahlen als Karten wie auf der Startseite -->
  <div class="rv-stats">
    <section class="rv-card">
      <div class="rv-card-h">
        <span>{{ $t('Erfasst') }}</span
        ><span v-if="planToDate" class="rv-pill">{{ trackedPct }} %</span>
      </div>
      <span class="rv-big mono" :class="{ zero: !review.tracked_minutes }">{{
        hm(review.tracked_minutes)
      }}</span>
      <div class="rv-card-f">
        <template v-if="planToDate">
          <div class="rv-meter" aria-hidden="true">
            <span :style="{ width: `${Math.min(100, trackedPct)}%` }"></span>
          </div>
          <span>{{ $t('von {plan} geplant', { plan: hm(planToDate) }) }}</span>
        </template>
        <span v-else>{{ $t('Noch nichts geplant') }}</span>
      </div>
    </section>
    <section class="rv-card">
      <div class="rv-card-h">
        <span>{{ $t('Termine') }}</span>
      </div>
      <span class="rv-big mono" :class="{ zero: !calendarMinutes }">{{
        hm(calendarMinutes)
      }}</span>
      <div class="rv-card-f">
        <span>{{ $t('Kalenderzeit') }}</span>
      </div>
    </section>
    <section class="rv-card">
      <div class="rv-card-h">
        <span>{{ $t('Erledigt') }}</span
        ><span v-if="weekTasksCount" class="rv-pill"
          >{{ Math.round((review.completed_tasks / weekTasksCount) * 100) }} %</span
        >
      </div>
      <span class="rv-big mono" :class="{ zero: !review.completed_tasks }"
        >{{ review.completed_tasks
        }}<small v-if="weekTasksCount"> / {{ weekTasksCount }}</small></span
      >
      <div class="rv-card-f">
        <span>{{ $t('Aufgaben dieser Woche') }}</span>
      </div>
    </section>
    <section class="rv-card">
      <div class="rv-card-h">
        <span>{{ $t('Überfällig geworden') }}</span>
      </div>
      <span class="rv-big mono" :class="overdue.length ? 'od' : 'zero'">{{
        overdue.length
      }}</span>
      <div class="rv-card-f">
        <span v-if="overdue.length" class="rv-ell"
          >{{ overdue[0]!.title
          }}<template v-if="overdue.length > 1"> + {{ overdue.length - 1 }}</template></span
        >
        <span v-else>{{ $t('Nichts überfällig.') }}</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.rv-ell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Karten: gleiche Sprache wie .st-card auf der Startseite */
.rv-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 4px 0 16px;
}
.rv-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  padding: 14px 16px;
  border-radius: var(--r-m);
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  transition: border-color var(--dur) ease-out;
}
.rv-card:hover {
  border-color: var(--br-strong);
}
.rv-card-h {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 20px;
  font: 600 12px/1 var(--font-ui);
  color: var(--tx-secondary);
}
.rv-pill {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-subtle);
  font: 500 11.5px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-secondary);
}
.rv-big {
  font-size: 30px;
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: -0.02em;
  color: var(--tx-primary);
}
.rv-big small {
  font-size: 15px;
  letter-spacing: 0;
  color: var(--tx-muted);
}
.rv-big.zero {
  color: var(--tx-muted);
}
.rv-big.od {
  color: var(--a-red);
}
.rv-card-f {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  min-height: 18px;
  margin-top: auto;
  font: 400 11.5px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.rv-meter {
  flex: 1;
  max-width: 120px;
  height: 4px;
  border-radius: 2px;
  background: var(--br-default);
  overflow: hidden;
}
.rv-meter span {
  display: block;
  height: 100%;
  background: var(--a-blue-hi);
  border-radius: 2px;
}

@media (max-width: 640px) {
  .rv-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .rv-big {
    font-size: 24px;
  }
}
</style>
