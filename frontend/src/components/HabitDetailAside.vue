<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Habit, HabitPause, Project } from '@/api/client'
import { addDays, ymd } from '@/lib/dates'
import { habitColor, projectColor } from '@/lib/projectColor'
import { iconOf } from '@/lib/habitIcons'
import { locale } from '@/lib/i18n'
import {
  buildHabitCell,
  formatHabitDescription,
  formatHabitRhythm,
  formatPauseSpan,
  getPausesForHabit,
  getUpcomingPauses,
  getWeekdayLabels,
  WEEKDAYS,
  type HabitCell,
  type Weekday,
} from '@/lib/habits'

const props = defineProps<{
  habit: Habit
  streak: number
  longestStreak: number
  rate30Days: number | null
  yearCount: number
  doneDates?: Set<string>
  values?: Record<string, number | null>
  pauses: HabitPause[]
  projects: Project[]
  weekdayRates: { w: Weekday; pct: number | null }[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'edit', habit: Habit): void
  (e: 'trash', habit: Habit): void
  (e: 'toggleActive', habit: Habit): void
  (e: 'toggleCell', habit: Habit, cell: HabitCell): void
  (e: 'setAmount', habit: Habit, cell: HabitCell, value: number): void
  (e: 'toggleSkip', habit: Habit, dateStr: string): void
  (e: 'createPause', payload: { habit_id: string; start_date: string; end_date: string }): void
  (e: 'deletePause', pauseId: string): void
}>()

const todayStr = computed(() => ymd(new Date()))
const dayLabelFmt = new Intl.DateTimeFormat(locale.value, {
  weekday: 'short',
  day: '2-digit',
  month: '2-digit',
})
const dateShortFmt = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' })
const wdLabels = computed(() => getWeekdayLabels(locale.value))

const projectMap = computed(() => new Map(props.projects.map((p) => [p.id, p])))
const projectName = computed(() =>
  props.habit.project_id ? projectMap.value.get(props.habit.project_id)?.name : undefined
)

function getCell(d: Date): HabitCell {
  return buildHabitCell(
    props.habit,
    d,
    props.doneDates,
    props.pauses,
    todayStr.value,
    dayLabelFmt
  )
}

const todayCell = computed(() => getCell(new Date()))

function getAmount(s: string): number {
  return (
    props.values?.[s] ??
    (props.doneDates?.has(s) ? props.habit.target_value ?? 1 : 0)
  )
}

const habitPauses = computed(() => getPausesForHabit(props.pauses, props.habit.id))
const upcomingHabitPauses = computed(() =>
  getUpcomingPauses(habitPauses.value, todayStr.value)
)
const todayPause = computed(() =>
  props.pauses.find(
    (p) =>
      p.habit_id === props.habit.id &&
      p.start_date === todayStr.value &&
      p.end_date === todayStr.value
  )
)

// Pause Form
const isPauseFormOpen = ref(false)
const pauseFrom = ref(todayStr.value)
const pauseUntil = ref(ymd(addDays(new Date(), 7)))

function openPauseForm() {
  pauseFrom.value = todayStr.value
  pauseUntil.value = ymd(addDays(new Date(), 7))
  isPauseFormOpen.value = true
}

function submitPause() {
  if (pauseUntil.value < pauseFrom.value) return
  emit('createPause', {
    habit_id: props.habit.id,
    start_date: pauseFrom.value,
    end_date: pauseUntil.value,
  })
  isPauseFormOpen.value = false
}

// Month Navigation
const monthOffset = ref(0)
const monthStart = computed(() => {
  const n = new Date()
  return new Date(n.getFullYear(), n.getMonth() + monthOffset.value, 1)
})
const monthTitle = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(
    monthStart.value
  )
)
const monthDays = computed(() => {
  const m = monthStart.value
  return Array.from(
    { length: new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate() },
    (_, i) => new Date(m.getFullYear(), m.getMonth(), i + 1)
  )
})
const lead = computed(() => (monthStart.value.getDay() + 6) % 7)

// Weekday Rate Filter: only show if difference >= 15
const visibleWeekdayRates = computed(() => {
  const values = props.weekdayRates.flatMap((x) => (x.pct === null ? [] : [x.pct]))
  return values.length > 1 && Math.max(...values) - Math.min(...values) >= 15
    ? props.weekdayRates
    : []
})

function formatDayStr(d: string): string {
  return dateShortFmt.format(new Date(`${d}T12:00:00`))
}
</script>

<template>
  <aside class="hab-aside" :aria-label="$t('Gewohnheit')" :style="{ '--hc': habitColor(habit.id) }">
    <div class="aside-head">
      <span class="lbl">{{ $t('Gewohnheit') }}</span>
      <button class="icon-btn" type="button" :aria-label="$t('Details schließen')" @click="emit('close')">
        <svg class="ic"><use href="#i-x" /></svg>
      </button>
    </div>

    <div class="ha-body">
      <header class="ha-head">
        <span class="hd-icon" aria-hidden="true">
          <svg class="ic"><use :href="`#${iconOf(habit)}`" /></svg>
        </span>
        <span class="hd-name">
          <b>{{ habit.name }}</b>
          <small>
            {{ habit.description || formatHabitDescription(habit, wdLabels) }}
            <template v-if="!habit.active"> {{ $t('· inaktiv') }}</template>
          </small>
        </span>

        <span
          v-if="habit.active && habit.target_value"
          class="hd-qty"
          role="group"
          :aria-label="$t('Menge: {name}', { name: habit.name })"
        >
          <button
            type="button"
            class="icon-btn"
            :aria-label="$t('Weniger')"
            :disabled="!todayCell.can || !getAmount(todayStr)"
            @click="emit('setAmount', habit, todayCell, getAmount(todayStr) - 1)"
          >
            <svg class="ic"><use href="#i-minus" /></svg>
          </button>
          <span
            class="hd-qring"
            :class="{ full: todayCell.isDone }"
            :style="{ '--p': Math.min(1, getAmount(todayStr) / habit.target_value) }"
          >
            <b>{{ getAmount(todayStr) }}</b>
            <small>/{{ habit.target_value }}</small>
          </span>
          <button
            type="button"
            class="icon-btn"
            :aria-label="$t('Mehr')"
            :disabled="!todayCell.can"
            @click="emit('setAmount', habit, todayCell, getAmount(todayStr) + 1)"
          >
            <svg class="ic"><use href="#i-plus" /></svg>
          </button>
        </span>

        <button
          v-else-if="habit.active"
          type="button"
          class="hd-check"
          role="checkbox"
          :aria-checked="todayCell.isDone"
          :disabled="!todayCell.can"
          :aria-label="$t('Heute abhaken: {name}', { name: habit.name })"
          :title="$t('Heute abhaken: {name}', { name: habit.name })"
          @click="emit('toggleCell', habit, todayCell)"
        >
          <svg class="ic" aria-hidden="true"><use href="#i-check" /></svg>
        </button>
      </header>

      <p class="ha-stats">
        <template v-if="yearCount">
          {{ $t('Serie {n}', { n: streak }) }} · {{ $t('Rekord {n}', { n: longestStreak }) }}
          <template v-if="rate30Days !== null"> · {{ $t('{p} % in 30 Tagen', { p: rate30Days }) }}</template>
        </template>
        <template v-else>{{ $t('Noch nichts erledigt. Heute abhaken startet die Serie.') }}</template>
      </p>

      <form v-if="isPauseFormOpen" class="hd-pause" @submit.prevent="submitPause">
        <label>{{ $t('von') }} <input v-model="pauseFrom" class="input" type="date" required /></label>
        <label>{{ $t('bis') }} <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required /></label>
        <button class="btn btn-primary" type="submit">{{ $t('Pausieren') }}</button>
        <button class="btn btn-ghost" type="button" @click="isPauseFormOpen = false">{{ $t('Abbrechen') }}</button>
      </form>

      <ul v-if="upcomingHabitPauses.length" class="hd-chips">
        <li v-for="p in upcomingHabitPauses" :key="p.id">
          {{ p.habit_id ? $t('Pause') : $t('Urlaub') }} · {{ formatPauseSpan(p, dateShortFmt) }}
          <button type="button" class="icon-btn" :aria-label="$t('Pause entfernen')" @click="emit('deletePause', p.id)">
            <svg class="ic"><use href="#i-x" /></svg>
          </button>
        </li>
      </ul>

      <dl class="ha-props">
        <div>
          <dt>{{ $t('Rhythmus') }}</dt>
          <dd>{{ formatHabitRhythm(habit, wdLabels) }}</dd>
        </div>
        <div v-if="habit.preferred_time">
          <dt>{{ $t('Uhrzeit') }}</dt>
          <dd>{{ habit.preferred_time }} · {{ habit.duration_minutes }} min</dd>
        </div>
        <div v-if="habit.target_value">
          <dt>{{ $t('Ziel') }}</dt>
          <dd>{{ habit.target_value }} {{ habit.unit }}</dd>
        </div>
        <div v-if="projectName">
          <dt>{{ $t('Projekt') }}</dt>
          <dd>
            <span class="chip" :style="{ '--chip-c': projectColor(habit.project_id) }">
              {{ projectName }}
            </span>
          </dd>
        </div>
        <div>
          <dt>{{ $t('Seit') }}</dt>
          <dd>
            {{ formatDayStr(habit.start_date) }}
            <template v-if="yearCount"> · {{ $t('{n}× im Jahr', { n: yearCount }) }}</template>
          </dd>
        </div>
      </dl>

      <section class="ha-month">
        <div class="hm-nav">
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Voriger Monat')"
            :disabled="monthOffset <= -11"
            @click="monthOffset--"
          >
            <svg class="ic"><use href="#i-left" /></svg>
          </button>
          <span class="lbl">{{ monthTitle }}</span>
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Nächster Monat')"
            :disabled="monthOffset >= 0"
            @click="monthOffset++"
          >
            <svg class="ic"><use href="#i-right" /></svg>
          </button>
        </div>

        <div class="hm-grid" role="group" :aria-label="`${habit.name}, ${monthTitle}`">
          <span v-for="w in WEEKDAYS" :key="w" class="hm-wd" aria-hidden="true">
            {{ wdLabels[w]?.slice(0, 2) }}
          </span>
          <span v-for="i in lead" :key="`l${i}`" aria-hidden="true"></span>
          <button
            v-for="c in monthDays.map(getCell)"
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
      </section>

      <div v-if="visibleWeekdayRates.length" class="hd-wd">
        <span class="lbl">{{ $t('Quote je Wochentag') }}</span>
        <div class="hd-wdbars">
          <span
            v-for="x in visibleWeekdayRates"
            :key="x.w"
            :title="`${wdLabels[x.w]}: ${x.pct ?? '–'} %`"
          >
            <small class="hd-pct">{{ x.pct === null ? '–' : `${x.pct}%` }}</small>
            <i :style="{ '--h': (x.pct ?? 0) / 100 }"></i>
            <small>{{ wdLabels[x.w]?.slice(0, 2) }}</small>
          </span>
        </div>
      </div>

      <div class="ha-acts">
        <button
          v-if="habit.active && !todayCell.isDone"
          type="button"
          @click="emit('toggleSkip', habit, todayStr)"
        >
          <svg class="ic" aria-hidden="true"><use href="#i-today" /></svg>
          {{ todayPause ? $t('Nicht mehr aussetzen') : $t('Heute aussetzen') }}
        </button>
        <button v-if="habit.active" type="button" @click="openPauseForm">
          <svg class="ic" aria-hidden="true"><use href="#i-clock" /></svg>
          {{ $t('Pausieren …') }}
        </button>
        <button type="button" @click="emit('edit', habit)">
          <svg class="ic" aria-hidden="true"><use href="#i-pen" /></svg>
          {{ $t('Bearbeiten') }}
        </button>
        <button type="button" @click="emit('toggleActive', habit)">
          <svg class="ic" aria-hidden="true"><use href="#i-moon" /></svg>
          {{ habit.active ? $t('Deaktivieren') : $t('Aktivieren') }}
        </button>
        <button class="danger" type="button" @click="emit('trash', habit)">
          <svg class="ic" aria-hidden="true"><use href="#i-trash" /></svg>
          {{ $t('Löschen') }}
        </button>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.hab-aside {
  display: flex;
  flex-direction: column;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  overflow-y: auto;
  scrollbar-width: none;
}
.hab-aside::-webkit-scrollbar {
  display: none;
}

.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 0 24px;
}

.ha-body {
  display: grid;
  gap: 22px;
  align-content: start;
  padding: 12px 24px 24px;
}

.ha-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hd-icon {
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
.hd-icon .ic {
  width: 52%;
  height: 52%;
  stroke-width: 1.9;
}

.hd-name {
  flex: 1;
  min-width: 0;
  display: grid;
  gap: 3px;
}
.hd-name b {
  font: 600 18px/1.25 var(--font-ui);
  letter-spacing: -0.015em;
  color: var(--tx-primary);
}
.hd-name small {
  font: 400 12.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
}

.hd-check {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  margin: 0;
  border-radius: 50%;
  border: 1.5px solid var(--br-strong);
  background: transparent;
  color: transparent;
  cursor: pointer;
  transition:
    background var(--dur) ease-out,
    border-color var(--dur) ease-out,
    transform var(--dur) ease-out;
}
.hd-check:hover:not(:disabled) {
  border-color: var(--tx-secondary);
}
.hd-check[aria-checked='true'] {
  background: var(--tx-primary);
  border-color: var(--tx-primary);
  color: var(--bg-0);
}
.hd-check:active:not(:disabled) {
  transform: scale(0.92);
}
.hd-check:disabled {
  cursor: default;
  opacity: 0.4;
}
.hd-check .ic {
  width: 13px;
  height: 13px;
  stroke-width: 3;
}

.hd-qty {
  flex: none;
  display: flex;
  align-items: center;
  gap: 2px;
  margin: 0;
}
.hd-qty .icon-btn {
  width: 26px;
  height: 26px;
}
.hd-qty .icon-btn .ic {
  width: 14px;
  height: 14px;
}

.hd-qring {
  --p: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background:
    radial-gradient(closest-side, var(--bg-0) 84%, transparent 86%),
    conic-gradient(var(--tx-primary) calc(var(--p) * 1turn), var(--br-default) 0);
}
.hd-qring.full {
  background: var(--tx-primary);
}
.hd-qring b {
  font: 600 13px/1 var(--font-ui);
  color: var(--tx-primary);
  font-variant-numeric: tabular-nums;
}
.hd-qring small {
  margin-top: 2px;
  font: 400 10px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-qring.full b,
.hd-qring.full small {
  color: var(--bg-0);
}

.ha-stats {
  margin: -8px 0 0;
  font: 400 13px/1.5 var(--font-ui);
  color: var(--tx-secondary);
  font-variant-numeric: tabular-nums;
}

.hd-pause {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.hd-pause label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-pause .input {
  width: auto;
  height: 30px;
}

.hd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: -8px 0 0;
  padding: 0;
  list-style: none;
}
.hd-chips li {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 4px 2px 10px;
  border-radius: 99px;
  font: 500 12px/1.6 var(--font-ui);
  color: var(--tx-secondary);
  background: var(--bg-2);
}
.hd-chips .icon-btn {
  width: 20px;
  height: 20px;
}
.hd-chips .icon-btn .ic {
  width: 12px;
  height: 12px;
}

.ha-props {
  margin: 0;
  border-top: 1px solid var(--br-subtle);
}
.ha-props div {
  display: flex;
  align-items: center;
  min-height: 38px;
  border-bottom: 1px solid var(--br-subtle);
  font: 400 13px/1.3 var(--font-ui);
}
.ha-props dt {
  flex: none;
  width: 96px;
  color: var(--tx-muted);
}
.ha-props dd {
  margin: 0;
  color: var(--tx-primary);
}

.chip {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 500;
  background: color-mix(in srgb, var(--chip-c) 15%, transparent);
  color: color-mix(in srgb, var(--chip-c) 80%, var(--tx-primary));
  white-space: nowrap;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ha-month {
  display: grid;
  gap: 10px;
}
.ha-month .hm-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0;
}
.ha-month .hm-nav .icon-btn {
  width: 24px;
  height: 24px;
}
.ha-month .hm-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 3px;
}
.ha-month .hm-wd {
  text-align: center;
  font: 500 11px/1.6 var(--font-ui);
  color: var(--tx-muted);
}
.ha-month .hm-day {
  justify-self: center;
  width: 30px;
  height: 30px;
  aspect-ratio: auto;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-secondary);
  cursor: pointer;
}
.ha-month .hm-day:hover:not(:disabled) {
  outline: 1.5px solid var(--tx-secondary);
}
.ha-month .hm-day.done {
  background: color-mix(in srgb, var(--hc) 22%, transparent);
  color: var(--tx-primary);
  box-shadow: inset 0 0 0 1.5px var(--hc);
}
.ha-month .hm-day.open {
  box-shadow: inset 0 0 0 1.5px var(--br-default);
}
.ha-month .hm-day.today {
  outline: 0;
  color: var(--tx-primary);
  font-weight: 600;
}
.ha-month .hm-day.off,
.ha-month .hm-day.pre,
.ha-month .hm-day.future {
  background: transparent;
  color: var(--tx-disabled);
}
.ha-month .hm-day.pause {
  background: repeating-linear-gradient(
    135deg,
    color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px,
    transparent 2px 4px
  );
  color: var(--tx-muted);
}
.ha-month .hm-day:disabled {
  cursor: default;
}

.hd-wd {
  display: grid;
  gap: 10px;
}
.hd-wdbars {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 12px;
  height: 84px;
}
.hd-wdbars span {
  display: grid;
  grid-template-rows: auto 1fr auto;
  justify-items: center;
  gap: 5px;
}
.hd-wdbars i {
  align-self: end;
  width: 100%;
  max-width: 18px;
  height: calc(3px + var(--h) * 44px);
  border-radius: 3px;
  background: color-mix(in srgb, var(--hc) calc(25% + var(--h) * 60%), var(--bg-2));
}
.hd-wdbars small {
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-wdbars .hd-pct {
  align-self: end;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
  font-size: 11px;
}

.ha-acts {
  display: grid;
  margin: 0 -10px;
  padding-top: 8px;
  border-top: 1px solid var(--br-subtle);
}
.ha-acts button {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 34px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--r-s);
  background: none;
  color: var(--tx-secondary);
  font: 400 13px/1 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.ha-acts button:hover {
  background: var(--bg-hover);
  color: var(--tx-primary);
}
.ha-acts .ic {
  width: 15px;
  height: 15px;
  color: var(--tx-muted);
}
.ha-acts .danger {
  margin-top: 6px;
  color: var(--a-red);
}
.ha-acts .danger .ic {
  color: currentColor;
}

@media (max-width: 1000px) {
  .hab-aside {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 20;
    max-height: 65vh;
    border: 0;
    border-radius: var(--r-m) var(--r-m) 0 0;
    box-shadow: var(--shadow-pop);
  }
}
@media (max-width: 760px) {
  .hab-aside {
    bottom: calc(56px + env(safe-area-inset-bottom));
    z-index: 55;
    max-height: calc(100dvh - 56px - env(safe-area-inset-bottom) - 48px);
  }
}
</style>
