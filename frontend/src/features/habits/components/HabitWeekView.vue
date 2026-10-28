<script setup lang="ts">
import type { Habit } from '@/api/client'
import type { HabitCell } from '@/lib/habits'
import { habitColor } from '@/lib/projectColor'
import { iconOf } from '@/lib/habitIcons'
import { ymd } from '@/lib/dates'

export type GridHabitCell = HabitCell & {
  plan?: boolean
  p?: number
  label?: string
}

defineProps<{
  weekLabel: string
  thisWeek: boolean
  todayDueCount: number
  todayDoneCount: number
  week: Date[]
  todayStr: string
  wdShortFmt: Intl.DateTimeFormat
  gridGroups: { label: string; habits: Habit[] }[]
  activeHabits: Habit[]
  focus: Habit | null
  streaks: Record<string, number>
  streakAtRisk: (h: Habit) => boolean
  meta: (h: Habit) => string
  gridCell: (h: Habit, d: Date) => GridHabitCell
  weekRates: (number | null)[]
  trend: { s: string; cur: boolean; pct: number | null }[]
  dateShortFmt: Intl.DateTimeFormat
}>()

const emit = defineEmits<{
  (e: 'prevWeek'): void
  (e: 'nextWeek'): void
  (e: 'thisWeekClick'): void
  (e: 'pick', habit: Habit): void
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
  (e: 'openNew'): void
}>()
</script>

<template>
  <div class="hw" role="grid" :aria-label="$t('Woche {range}', { range: weekLabel })">
    <div class="hw-head" role="row">
      <span class="hw-nav">
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Vorige Woche')"
          @click="emit('prevWeek')"
        >
          <svg class="ic"><use href="#i-left" /></svg>
        </button>
        <span class="hw-range">{{ weekLabel }}</span>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Nächste Woche')"
          :disabled="thisWeek"
          @click="emit('nextWeek')"
        >
          <svg class="ic"><use href="#i-right" /></svg>
        </button>
        <button
          v-if="!thisWeek"
          class="btn btn-ghost hw-back"
          type="button"
          @click="emit('thisWeekClick')"
        >
          {{ $t('Diese Woche') }}
        </button>
        <span v-else-if="todayDueCount" class="hab-count">
          {{ $t('{done}/{n} heute', { done: todayDoneCount, n: todayDueCount }) }}
        </span>
      </span>

      <span
        v-for="d in week"
        :key="ymd(d)"
        class="hw-d"
        :class="{ today: ymd(d) === todayStr, future: ymd(d) > todayStr }"
        role="columnheader"
      >
        <small>{{ wdShortFmt.format(d).slice(0, 2) }}</small>
        <b>{{ d.getDate() }}</b>
      </span>

      <span class="hw-streak-th" :title="$t('Serie')" :aria-label="$t('Serie')" role="columnheader">
        <svg class="ic" aria-hidden="true"><use href="#i-flame" /></svg>
      </span>
    </div>

    <div v-for="g in gridGroups" :key="g.label" class="hw-group" role="rowgroup">
      <span class="hw-gl">{{ g.label }}</span>
      <div
        v-for="h in g.habits"
        :key="h.id"
        class="hd-row"
        role="row"
        :class="{ sel: focus?.id === h.id, inactive: !h.active }"
        :style="{ '--hc': habitColor(h.id) }"
      >
        <button
          type="button"
          class="hd-pick"
          :aria-pressed="focus?.id === h.id"
          @click="emit('pick', h)"
        >
          <span class="hd-icon" aria-hidden="true">
            <svg class="ic"><use :href="`#${iconOf(h)}`" /></svg>
          </span>
          <span class="hw-name">
            <b>{{ h.name }}</b>
            <small>{{ meta(h) }}</small>
          </span>
        </button>

        <button
          v-for="c in week.map((d) => gridCell(h, d))"
          :key="c.s"
          type="button"
          class="hw-c"
          :class="[
            c.state,
            {
              today: c.today,
              plan: c.plan,
              part: (c.p ?? 0) > 0,
              soft: h.frequency_type === 'TIMES_PER_WEEK',
            },
          ]"
          :style="c.p ? { '--p': c.p } : undefined"
          role="checkbox"
          :aria-checked="c.isDone"
          :aria-label="`${h.name}, ${c.label}`"
          :title="c.can ? c.label : undefined"
          :disabled="!c.can"
          @click="emit('toggleCell', h, c)"
        >
          <i>
            <svg v-if="c.isDone" class="ic" aria-hidden="true"><use href="#i-check" /></svg>
          </i>
        </button>

        <span class="hw-streak" :class="{ risk: streakAtRisk(h) }" :title="$t('Serie')">
          {{ streaks[h.id] || '' }}
        </span>
      </div>
    </div>

    <div v-if="activeHabits.length" class="hd-row hw-sum" role="row">
      <span class="hw-gl">{{ $t('Tag gesamt') }}</span>
      <span
        v-for="(r, i) in weekRates"
        :key="i"
        class="hw-ring"
        :class="{ full: r === 1 }"
        :style="r === null ? undefined : { '--p': r }"
        :title="r === null ? undefined : `${Math.round(r * 100)} %`"
      >
        <template v-if="r !== null">{{ Math.round(r * 100) }}</template>
      </span>
      <span></span>
    </div>

    <p v-if="activeHabits.length" class="hw-legend">
      <span><i class="done"></i>{{ $t('erledigt') }}</span>
      <span><i class="open"></i>{{ $t('offen') }}</span>
      <span><i class="part"></i>{{ $t('teilweise') }}</span>
      <span><i class="soft"></i>{{ $t('flexibel') }}</span>
      <span><i class="pause"></i>{{ $t('ausgesetzt') }}</span>
      <span><i class="plan"></i>{{ $t('geplant') }}</span>
      <em>{{ $t('Kreis anklicken hakt den Tag ab, auch rückwirkend') }}</em>
    </p>

    <section v-if="activeHabits.length" class="hw-trend" :aria-label="$t('Letzte 12 Wochen')">
      <div class="hw-trend-h">
        <span>{{ $t('Letzte 12 Wochen') }}</span>
        <b v-if="trend[11]!.pct !== null">{{ $t('{p} % diese Woche', { p: trend[11]!.pct! }) }}</b>
      </div>
      <div class="hw-bars">
        <span
          v-for="w in trend"
          :key="w.s"
          :class="{ cur: w.cur }"
          :title="
            w.pct === null
              ? undefined
              : `${dateShortFmt.format(new Date(w.s + 'T12:00:00'))}: ${w.pct} %`
          "
        >
          <i v-if="w.pct !== null" :style="{ '--h': w.pct / 100 }"></i>
          <i v-else class="none"></i>
          <small>{{ dateShortFmt.format(new Date(w.s + 'T12:00:00')) }}</small>
        </span>
      </div>
    </section>

    <button class="hw-add" type="button" @click="emit('openNew')">
      <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
      {{ $t('Gewohnheit hinzufügen') }}
    </button>
  </div>
</template>

<style scoped>
/* Wochenraster */
.hw {
  --cols: minmax(200px, 320px) repeat(7, minmax(44px, 1fr)) 56px;
  display: grid;
  margin-top: 4px;
}
.hw-head,
.hd-row {
  display: grid;
  grid-template-columns: var(--cols);
  align-items: center;
}
.hw-head {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 6px 0 10px;
  background: var(--bg-0);
  border-bottom: 1px solid var(--br-subtle);
}
.hw-nav {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
}
.hw-nav .icon-btn {
  width: 26px;
  height: 26px;
}
.hw-range {
  padding: 0 6px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.hw-back {
  font-size: 11px;
  padding: 2px 6px;
  height: 22px;
}
.hab-count {
  margin-left: 8px;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.hw-d {
  display: grid;
  justify-items: center;
  gap: 4px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.hw-d small {
  font: 400 11.5px/1 var(--font-ui);
}
.hw-d b {
  font: 400 15px/1 var(--font-ui);
}
.hw-d.today {
  color: var(--tx-primary);
}
.hw-d.today b {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  margin: -3px 0;
  border-radius: 50%;
  background: var(--a-now);
  color: var(--bg-0);
  font-weight: 600;
}
.hw-d.future {
  opacity: 0.5;
}
.hw-group {
  display: grid;
  padding-top: 14px;
}
.hw-gl {
  padding: 0 12px 8px;
  font: 500 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-row {
  min-height: 56px;
  border-radius: var(--r-s);
  transition: background var(--dur) ease-out;
}
.hd-row:hover:not(.sel) {
  background: var(--bg-hover);
}
.hd-row.sel {
  background: var(--bg-2);
}
.hd-row.inactive {
  opacity: 0.5;
}
.hd-pick {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  height: 100%;
  padding: 0 12px 0 8px;
  border: 0;
  background: none;
  text-align: left;
  color: inherit;
  cursor: pointer;
  border-radius: var(--r-s);
}
.hd-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: color-mix(in srgb, var(--hc) 14%, var(--bg-2));
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary));
}
.hd-icon .ic {
  width: 15px;
  height: 15px;
}
.hd-row.inactive .hd-icon {
  filter: grayscale(1);
  opacity: 0.5;
}
.hw-name {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.hd-pick b {
  font: 500 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hd-pick small {
  flex: none;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
  white-space: nowrap;
}
.hd-pick:focus-visible,
.hw-c:focus-visible {
  outline: 2px solid var(--a-focus);
  outline-offset: -2px;
}
.hw-c {
  display: grid;
  place-items: center;
  height: 56px;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}
.hw-c > i {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  box-sizing: border-box;
  color: var(--bg-0);
  transition:
    transform var(--dur) ease-out,
    background-color var(--dur) ease-out,
    border-color var(--dur) ease-out;
}
.hw-c > i .ic {
  width: 15px;
  height: 15px;
  stroke-width: 3;
}
.hw-c.open > i {
  border: 1.5px solid var(--tx-disabled);
}
.hw-c.open.today > i {
  border: 2px solid var(--tx-secondary);
}
.hw-c.open.soft > i {
  border-style: dashed;
}
.hw-c.done > i {
  background: var(--tx-primary);
}
.hw-c.part > i {
  border: 1.5px solid var(--tx-disabled);
  background: conic-gradient(
    color-mix(in srgb, var(--tx-primary) 75%, transparent) calc(var(--p) * 1turn),
    transparent 0
  );
}
.hw-c.pause > i {
  background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 4px);
}
.hw-c.plan > i {
  width: 5px;
  height: 5px;
  background: var(--tx-disabled);
}
.hw-c:hover:not(:disabled) > i {
  border: 1.5px solid var(--tx-secondary);
  background-color: color-mix(in srgb, var(--tx-primary) 15%, transparent);
}
.hw-c.done:hover:not(:disabled) > i {
  background-color: color-mix(in srgb, var(--tx-primary) 80%, var(--bg-0));
}
.hw-c:active:not(:disabled) > i {
  transform: scale(0.9);
}
.hw-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 18px;
  padding: 12px 12px 0;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hw-legend span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.hw-legend i {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  box-sizing: border-box;
}
.hw-legend .done {
  background: var(--tx-secondary);
}
.hw-legend .open {
  border: 1.5px solid var(--tx-disabled);
}
.hw-legend .part {
  border: 1.5px solid var(--tx-disabled);
  background: conic-gradient(var(--tx-secondary) 180deg, transparent 0);
}
.hw-legend .soft {
  border: 1.5px dashed var(--tx-disabled);
}
.hw-legend .pause {
  background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 3px);
}
.hw-legend .plan {
  width: 5px;
  height: 5px;
  background: var(--tx-disabled);
}
.hw-legend em {
  margin-left: auto;
  font-style: normal;
}
.hw-c:disabled {
  cursor: default;
}
.hw-streak-th {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 12px;
  color: var(--tx-muted);
}
.hw-streak-th .ic {
  width: 14px;
  height: 14px;
}
.hw-streak {
  padding-right: 12px;
  text-align: right;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.hw-streak.risk {
  color: var(--tx-primary);
  font-weight: 600;
}
.hw-sum {
  min-height: 52px;
  margin-top: 10px;
  border-top: 1px solid var(--br-subtle);
  border-radius: 0;
}
.hw-sum:hover {
  background: none !important;
}
.hw-sum .hw-gl {
  padding: 0 12px;
}
.hw-ring {
  --p: 0;
  justify-self: center;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  font: 500 10.5px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  background:
    radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%),
    conic-gradient(var(--tx-secondary) calc(var(--p) * 1turn), var(--bg-3) 0);
}
.hw-ring:empty {
  background: none;
}
.hw-ring.full {
  color: var(--tx-primary);
  background: radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%), var(--a-now);
}
.hw-trend {
  margin-top: 28px;
  padding: 18px 12px 0;
  border-top: 1px solid var(--br-subtle);
}
.hw-trend-h {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 14px;
  font: 500 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hw-trend-h b {
  font-weight: 500;
  color: var(--tx-secondary);
  font-variant-numeric: tabular-nums;
}
.hw-bars {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: 8px;
  height: 96px;
}
.hw-bars span {
  display: grid;
  grid-template-rows: 1fr auto;
  justify-items: center;
  gap: 6px;
}
.hw-bars i {
  align-self: end;
  width: 100%;
  max-width: 36px;
  height: calc(3px + var(--h) * 70px);
  border-radius: 4px;
  background: var(--bg-3);
}
.hw-bars .cur i {
  background: var(--a-now);
}
.hw-bars i.none {
  height: 0;
}
.hw-bars small {
  font: 400 10.5px/1 var(--font-ui);
  color: var(--tx-muted);
  white-space: nowrap;
}
.hw-add {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  justify-self: start;
  margin-top: 14px;
  padding: 8px 12px;
  border: 0;
  border-radius: var(--r-s);
  background: none;
  color: var(--tx-muted);
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
}
.hw-add:hover {
  background: var(--bg-hover);
  color: var(--tx-secondary);
}
.hw-add .ic {
  width: 14px;
  height: 14px;
}

@media (max-width: 760px) {
  .hw {
    --cols: minmax(0, 1fr) repeat(7, 32px) 0;
    max-width: none;
  }
  .hw-streak {
    display: none;
  }
  .hd-pick {
    gap: 10px;
  }
  .hw-name {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .hw-bars small {
    display: none;
  }
  .hw-range {
    display: none;
  }
  .hw-c {
    height: 52px;
  }
  .hw-c > i {
    width: 24px;
    height: 24px;
  }
  .hw-legend em {
    display: none;
  }
  .hab-count {
    display: none;
  }
  .hd-pick b {
    max-width: 100%;
  }
}
</style>
