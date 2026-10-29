<script setup lang="ts">
import { computed } from 'vue'
import type { Review } from '@/api/client'
import { hm } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'

const props = defineProps<{
  projects: Review['projects']
  trackedMinutes: number
}>()

const projectsList = computed(() => (props.projects ?? []).filter((p) => p.tracked_minutes))
const maxProject = computed(() => Math.max(1, ...projectsList.value.map((p) => p.tracked_minutes)))
const projectTotal = computed(
  () => projectsList.value.reduce((s, p) => s + p.tracked_minutes, 0) || 1,
)
</script>

<template>
  <section class="rv-group">
    <h2 class="rv-gh">
      {{ $t('Zeit pro Projekt') }}
      <span class="count">{{ hm(trackedMinutes) }}</span>
    </h2>
    <div v-for="p in projectsList" :key="p.project_id ?? ''" class="rv-row rv-proj">
      <span class="chip" :style="{ '--chip-c': projectColor(p.project_id) }"
        ><span class="cdot"></span><span class="rv-ell">{{ p.name }}</span></span
      >
      <div class="rv-track" aria-hidden="true">
        <span
          :style="{
            width: `${(p.tracked_minutes / maxProject) * 100}%`,
            background: projectColor(p.project_id),
          }"
        ></span>
      </div>
      <span class="rv-n mono">{{ hm(p.tracked_minutes) }}</span>
      <span class="rv-n rv-pct"
        >{{ Math.round((p.tracked_minutes / projectTotal) * 100) }} %</span
      >
    </div>
    <div v-if="!projectsList.length" class="rv-empty">
      <b>{{ $t('Keine Zeit erfasst.') }}</b>
      {{ $t('Starte einen Timer an einer Aufgabe, dann erscheint die Zeit hier.') }}
    </div>
  </section>
</template>

<style scoped>
.rv-ell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rv-gh {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  margin: 0 0 4px;
  font: 600 14.5px/1.3 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}
.rv-gh .count {
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.rv-row {
  display: grid;
  align-items: center;
  column-gap: 14px;
  min-height: 42px;
  padding: 6px 12px;
  margin: 0 -12px;
  border-radius: var(--r-s);
  transition: background var(--dur) ease-out;
}
.rv-row:hover {
  background: color-mix(in oklab, var(--tx-primary) 4%, transparent);
}
.rv-row .chip {
  min-width: 0;
  justify-content: flex-end;
}
.rv-proj {
  grid-template-columns: minmax(110px, 190px) minmax(0, 1fr) 44px 38px;
}
.rv-proj .chip {
  justify-content: flex-start;
  font-weight: 500;
  color: var(--tx-primary);
}
.rv-track {
  display: block;
  height: 6px;
  border-radius: 3px;
  background: var(--br-subtle);
  overflow: hidden;
}
.rv-track span {
  display: block;
  height: 100%;
  border-radius: 3px;
}
.rv-n {
  font: 400 12px/1.2 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
  white-space: nowrap;
}
.rv-proj .rv-n.mono {
  color: var(--tx-secondary);
  text-align: right;
}
.rv-pct {
  text-align: right;
}
.rv-empty {
  padding: 10px 0;
  font: 400 13px/1.5 var(--font-ui);
  color: var(--tx-muted);
}
.rv-empty b {
  color: var(--tx-secondary);
  font-weight: 500;
}

@media (max-width: 640px) {
  .rv-proj {
    grid-template-columns: minmax(0, 1fr) 64px 40px 34px;
    column-gap: 10px;
  }
}
</style>
