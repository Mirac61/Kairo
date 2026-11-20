<script setup lang="ts">
import { computed, ref } from 'vue'
import { hm as fmtDur } from '@/lib/dates'

export interface WeekDayBar {
  date: string
  tracked_minutes: number
  plan: number
  isToday: boolean
  short: string
  full: string
}

export interface ProjectBar {
  project_id: string | null
  name: string
  tracked_minutes: number
  pct: number
  primary: boolean
}

const props = defineProps<{
  trackedMinutes: number
  planToDate: number
  planPercent: number | null
  completedTasks: number
  weekTasksCount: number
  routineRate: number | null
  days: WeekDayBar[]
  maxMinutes: number
  projects: ProjectBar[]
  hoveredProjectId: string | null
  hoveredProjectName: string | null
}>()

const emit = defineEmits<{
  'update:hoveredProjectId': [id: string | null]
}>()

const hoveredDayIdx = ref<number | null>(null)
const hoveredDay = computed(() =>
  hoveredDayIdx.value !== null ? (props.days[hoveredDayIdx.value] ?? null) : null,
)
</script>

<template>
  <section class="pr-left">
    <div class="pr-hero">
      <div>
        <div class="pr-kicker">{{ $t('Diese Woche erfasst') }}</div>
        <div class="pr-hero-val">
          <span class="pr-big">{{ fmtDur(trackedMinutes) }}</span>
          <span v-if="planToDate > 0" class="pr-plan">
            / {{ fmtDur(planToDate) }} {{ $t('Plan') }}
            <template v-if="planPercent !== null"> · {{ planPercent }} %</template>
          </span>
        </div>
      </div>

      <div class="pr-sub-stats">
        <div>
          <div class="pr-stat-num">
            {{ completedTasks
            }}<span v-if="weekTasksCount" class="pr-muted"> / {{ weekTasksCount }}</span>
          </div>
          <div class="pr-stat-lbl">{{ $t('Aufgaben erledigt') }}</div>
        </div>
        <div v-if="routineRate !== null">
          <div class="pr-stat-num">{{ routineRate }} %</div>
          <div class="pr-stat-lbl">{{ $t('Routinen-Quote') }}</div>
        </div>
      </div>
    </div>

    <div class="pr-sec">
      <div class="pr-sec-head">
        <span class="pr-sec-title">{{ $t('Tagesverlauf') }}</span>
        <div class="pr-readout">
          <template v-if="hoveredDay">
            <span class="pr-rd-day">{{ hoveredDay.full }}</span>
            <span class="pr-rd-accent">{{ fmtDur(hoveredDay.tracked_minutes) }}</span>
            <span v-if="hoveredDay.plan > 0" class="pr-muted">
              / {{ fmtDur(hoveredDay.plan) }} {{ $t('Plan') }}
            </span>
          </template>
          <template v-else-if="hoveredProjectName">
            <span class="pr-rd-accent">{{ $t('Fokus') }}: {{ hoveredProjectName }}</span>
          </template>
          <template v-else>
            <span class="pr-leg"><i class="is-past"></i>{{ $t('Ist') }}</span>
            <span class="pr-leg is-today"><i class="is-today"></i>{{ $t('Heute') }}</span>
            <span class="pr-leg"><i class="is-plan"></i>{{ $t('Plan') }}</span>
          </template>
        </div>
      </div>

      <div class="pr-bars">
        <div
          v-for="(d, idx) in days"
          :key="d.date"
          class="pr-bar-col"
          tabindex="0"
          :aria-label="`${d.full}: ${fmtDur(d.tracked_minutes)}`"
          @mouseenter="hoveredDayIdx = idx"
          @mouseleave="hoveredDayIdx = null"
          @focus="hoveredDayIdx = idx"
          @blur="hoveredDayIdx = null"
        >
          <div class="pr-bar-pair">
            <span
              v-if="d.plan > 0"
              class="pr-bar-plan"
              :style="{ height: `${Math.max(4, Math.round((d.plan / maxMinutes) * 100))}%` }"
            ></span>
            <span
              v-if="d.tracked_minutes > 0"
              class="pr-bar-actual"
              :class="{ today: d.isToday }"
              :style="{
                height: `${Math.max(4, Math.round((d.tracked_minutes / maxMinutes) * 100))}%`,
              }"
            ></span>
            <span v-else class="pr-bar-empty"></span>
          </div>
          <div class="pr-bar-lbls">
            <span class="pr-day" :class="{ active: d.isToday || hoveredDayIdx === idx }">{{
              d.short
            }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="projects.length" class="pr-sec">
      <div class="pr-sec-head">
        <span class="pr-sec-title">{{ $t('Zeit nach Projekt') }}</span>
      </div>
      <div class="pr-proj-list">
        <div
          v-for="p in projects"
          :key="p.project_id ?? 'none'"
          class="pr-proj-row"
          :class="{
            primary: p.primary || (hoveredProjectId !== null && hoveredProjectId === p.project_id),
            dimmed: hoveredProjectId !== null && hoveredProjectId !== p.project_id,
          }"
          :tabindex="p.project_id ? 0 : undefined"
          @mouseenter="p.project_id && emit('update:hoveredProjectId', p.project_id)"
          @mouseleave="emit('update:hoveredProjectId', null)"
          @focus="p.project_id && emit('update:hoveredProjectId', p.project_id)"
          @blur="emit('update:hoveredProjectId', null)"
        >
          <div class="pr-proj-top">
            <span class="pr-proj-name">{{ p.name }}</span>
            <span class="pr-proj-dur">{{ fmtDur(p.tracked_minutes) }}</span>
          </div>
          <div class="pr-proj-track">
            <div class="pr-proj-fill" :style="{ width: `${p.pct}%` }"></div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.pr-left {
  display: flex;
  flex-direction: column;
  gap: 32px;
  min-width: 0;
}
.pr-hero {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--br-subtle);
}
.pr-kicker {
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
  margin-bottom: 8px;
}
.pr-hero-val {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 10px;
}
.pr-big {
  font: 600 34px/1 var(--font-ui);
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
  color: var(--tx-primary);
}
.pr-plan {
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.pr-sub-stats {
  display: flex;
  gap: 28px;
}
.pr-stat-num {
  font: 600 18px/1.1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-primary);
}
.pr-stat-lbl {
  margin-top: 4px;
  font: 400 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.pr-muted {
  color: var(--tx-muted);
}
.pr-sec {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.pr-sec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 18px;
}
.pr-sec-title {
  font: 500 11.5px/1 var(--font-ui);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.pr-readout {
  display: flex;
  align-items: center;
  gap: 10px;
  font: 400 12px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
}
.pr-rd-day {
  color: var(--tx-secondary);
}
.pr-rd-accent {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.pr-leg {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--tx-muted);
}
.pr-leg i {
  width: 7px;
  height: 7px;
  border-radius: 2px;
  display: inline-block;
}
.pr-leg i.is-past {
  background: var(--tx-secondary);
}
.pr-leg i.is-today {
  background: var(--a-blue-hi);
}
.pr-leg i.is-plan {
  background: var(--bg-3);
}
.pr-bars {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 10px;
  height: 148px;
  padding-top: 8px;
}
.pr-bar-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 0;
  border-radius: 6px;
}
.pr-bar-col:focus-visible {
  outline: 1.5px solid var(--a-focus);
  outline-offset: 2px;
}
.pr-bar-pair {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 3px;
}
.pr-bar-plan {
  width: 9px;
  border-radius: 3px 3px 1px 1px;
  background: var(--bg-3);
  transition: height 240ms var(--ease);
}
.pr-bar-actual {
  width: 14px;
  border-radius: 4px 4px 1px 1px;
  background: var(--tx-secondary);
  transition: height 240ms var(--ease);
}
.pr-bar-actual.today {
  background: var(--a-blue-hi);
}
.pr-bar-empty {
  width: 14px;
  height: 2px;
  border-radius: 2px;
  background: var(--br-default);
}
.pr-bar-lbls {
  display: flex;
  justify-content: center;
}
.pr-day {
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.pr-day.active {
  color: var(--tx-primary);
}
.pr-proj-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.pr-proj-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: opacity 160ms ease;
  border-radius: 6px;
}
.pr-proj-row:focus-visible {
  outline: 1.5px solid var(--a-focus);
  outline-offset: 4px;
}
.pr-proj-row.dimmed {
  opacity: 0.42;
}
.pr-proj-top {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  font: 400 13px/1.2 var(--font-ui);
}
.pr-proj-name {
  color: var(--tx-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pr-proj-row.primary .pr-proj-name {
  color: var(--tx-primary);
  font-weight: 500;
}
.pr-proj-dur {
  font-family: var(--font-mono);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
  flex-shrink: 0;
}
.pr-proj-track {
  height: 4px;
  border-radius: 99px;
  background: var(--bg-2);
  overflow: hidden;
}
.pr-proj-fill {
  height: 100%;
  border-radius: 99px;
  background: var(--tx-muted);
  transition:
    width 240ms var(--ease),
    background 160ms ease;
}
.pr-proj-row.primary .pr-proj-fill {
  background: var(--a-blue-hi);
}
</style>
