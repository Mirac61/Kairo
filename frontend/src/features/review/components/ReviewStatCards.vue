<script setup lang="ts">
import type { Review, Task } from '@/api/client'
import { hm } from '@/lib/dates'

defineProps<{
  review: Review
  planToDate: number
  weekTasksCount: number
  overdue: Task[]
}>()
</script>

<template>
  <!-- Kennzahlen als eine Zeile Typografie, ohne Karten -->
  <p class="rv-stats">
    <span class="rv-stat">
      <span class="rv-l">{{ $t('Erfasst') }}</span>
      <b :class="{ zero: !review.tracked_minutes }">{{ hm(review.tracked_minutes) }}</b>
      <span v-if="planToDate" class="rv-l">{{
        $t('von {plan} geplant', { plan: hm(planToDate) })
      }}</span>
      <span v-else class="rv-l">{{ $t('Noch nichts geplant') }}</span>
    </span>
    <span class="rv-stat">
      <span class="rv-l">{{ $t('Erledigt') }}</span>
      <b :class="{ zero: !review.completed_tasks }">{{ review.completed_tasks }}</b>
      <span v-if="weekTasksCount" class="rv-l">{{
        $t('von {n} Aufgaben', { n: weekTasksCount })
      }}</span>
    </span>
    <!-- Nichts überfällig: weglassen statt eine Null zu zeigen -->
    <span v-if="overdue.length" class="rv-stat">
      <span class="rv-l">{{ $t('Überfällig geworden') }}</span>
      <b class="od">{{ overdue.length }}</b>
      <span class="rv-l rv-ell"
        >{{ overdue[0]!.title
        }}<template v-if="overdue.length > 1"> + {{ overdue.length - 1 }}</template></span
      >
    </span>
  </p>
</template>

<style scoped>
.rv-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 36px;
  margin: 8px 0 20px;
  padding: 0;
}
.rv-stat {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}
.rv-stat b {
  font: 500 22px/1.2 var(--font-ui);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}
.rv-stat b.zero {
  color: var(--tx-muted);
}
.rv-stat b.od {
  color: var(--a-red);
}
.rv-l {
  font: 400 13px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.rv-ell {
  min-width: 0;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
