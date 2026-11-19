<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { addDays, ymd } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'

const HEAT_DAYS = 42

const props = defineProps<{
  allDates: string[]
  todayStr: string
  activeDate: string
  streak: number
}>()

const emit = defineEmits<{
  (e: 'select', date: string): void
  (e: 'backfill', date: string): void
}>()

interface HeatCell {
  s: string
  dayNum: number
  monthStart: boolean
  isToday: boolean
  has: boolean
  future: boolean
  label: string
}

const heatLabel = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    }),
)
const wdHeaderFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short' }))
const monthShortFmt = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'short' }))

// Feste Wochentagsspalten Mo–So (2024-01-01 ist ein Montag)
const weekdays = computed(() =>
  Array.from({ length: 7 }, (_, i) => {
    const d = new Date(2024, 0, 1 + i)
    return wdHeaderFmt.value.format(d).replace(/\.$/, '').slice(0, 2)
  }),
)

const startDate = computed(() =>
  props.todayStr ? addDays(new Date(`${props.todayStr}T00:00:00`), -(HEAT_DAYS - 1)) : null,
)

// Spalte 1 = Montag … 7 = Sonntag für den ersten Tag des 6-Wochen-Fensters
const firstWeekdayCol = computed(() => {
  if (!startDate.value) return 1
  return ((startDate.value.getDay() + 6) % 7) + 1
})

const rangeLabel = computed(() => {
  if (!startDate.value || !props.todayStr) return t('Letzte 6 Wochen')
  const end = new Date(`${props.todayStr}T00:00:00`)
  const mStart = monthShortFmt.value.format(startDate.value).replace(/\.$/, '')
  const mEnd = monthShortFmt.value.format(end).replace(/\.$/, '')
  const year = end.getFullYear()
  return mStart === mEnd ? `${mEnd} ${year}` : `${mStart} – ${mEnd} ${year}`
})

const heatCells = computed<HeatCell[]>(() => {
  if (!props.todayStr || !startDate.value) return []
  const dateSet = new Set(props.allDates)
  const today = props.todayStr
  const start = startDate.value

  return Array.from({ length: HEAT_DAYS }, (_, i) => {
    const d = addDays(start, i)
    const s = ymd(d)
    const dayNum = d.getDate()
    return {
      s,
      dayNum,
      monthStart: dayNum === 1,
      isToday: s === today,
      has: dateSet.has(s),
      future: s > today,
      label: heatLabel.value.format(d),
    }
  })
})

const heatFocus = ref(props.todayStr)
watch(
  () => props.todayStr,
  (val) => {
    if (!heatFocus.value && val) heatFocus.value = val
  },
)

const heatCount = computed(() => heatCells.value.filter((c) => c.has).length)

function heatAction(c: HeatCell): string {
  return c.has ? t('Eintrag öffnen') : t('Eintrag nachtragen')
}

function heatTitle(c: HeatCell): string {
  return c.future ? c.label : `${c.label} · ${heatAction(c)}`
}

function heatAria(c: HeatCell): string {
  return c.future ? c.label : `${c.label}: ${heatAction(c)}`
}

function openHeat(c: HeatCell) {
  if (c.future) return
  if (c.s === props.todayStr) {
    emit('select', c.s)
    return
  }
  if (!c.has) {
    emit('backfill', c.s)
    return
  }
  emit('select', c.s)
}

function onHeatKey(e: KeyboardEvent, c: HeatCell) {
  const step =
    e.key === 'ArrowLeft'
      ? -1
      : e.key === 'ArrowRight'
        ? 1
        : e.key === 'ArrowUp'
          ? -7
          : e.key === 'ArrowDown'
            ? 7
            : 0
  if (!step) return
  e.preventDefault()

  const cells = heatCells.value
  const from = cells.findIndex((x) => x.s === c.s)
  let max = cells.length - 1
  while (max > 0 && cells[max]!.future) max--
  const to = Math.min(max, Math.max(0, from + step))
  heatFocus.value = cells[to]!.s

  const buttons = (
    e.currentTarget as HTMLElement
  ).parentElement?.querySelectorAll<HTMLButtonElement>('.jr-heat-cell')
  buttons?.[to]?.focus()
}
</script>

<template>
  <section class="jr-meta" :aria-label="$t('Letzte 6 Wochen')">
    <div class="jr-meta-head">
      <div class="jr-meta-left">
        <span class="jr-meta-title">{{ rangeLabel }}</span>
        <span class="jr-meta-sub">{{ heatCount }}/{{ HEAT_DAYS }}</span>
      </div>
      <span v-if="streak" class="jr-streak">{{
        $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge')
      }}</span>
    </div>

    <div class="jr-cal-wd" aria-hidden="true">
      <span v-for="wd in weekdays" :key="wd">{{ wd }}</span>
    </div>

    <div
      class="jr-heat"
      role="group"
      :aria-label="
        $tn(
          heatCount,
          '{n} Eintrag in den letzten 6 Wochen',
          '{n} Einträge in den letzten 6 Wochen',
        )
      "
    >
      <button
        v-for="(c, i) in heatCells"
        :key="c.s"
        type="button"
        class="jr-heat-cell"
        :data-date="c.s"
        :style="i === 0 ? { gridColumnStart: firstWeekdayCol } : undefined"
        :class="{
          on: c.has,
          future: c.future,
          open: c.s === activeDate,
          today: c.isToday,
          mstart: c.monthStart,
        }"
        :disabled="c.future"
        :tabindex="c.s === heatFocus ? 0 : -1"
        :title="heatTitle(c)"
        :aria-label="heatAria(c)"
        @focus="heatFocus = c.s"
        @click="openHeat(c)"
        @keydown="onHeatKey($event, c)"
      >
        <span aria-hidden="true">{{ c.dayNum }}</span>
      </button>
    </div>

    <div class="jr-heat-leg">
      <span><i class="jr-leg-dot"></i>{{ $t('Ohne Eintrag') }}</span>
      <span><i class="jr-leg-dot on"></i>{{ $t('Tag mit Eintrag') }}</span>
      <span><i class="jr-leg-dot open"></i>{{ $t('Geöffnet') }}</span>
    </div>
  </section>
</template>

<style scoped>
.jr-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.jr-meta-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
.jr-meta-left {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}
.jr-meta-title {
  font: 500 12.5px/1.2 var(--font-ui);
  color: var(--tx-primary);
}
.jr-meta-sub {
  font: 400 11.5px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.jr-streak {
  flex: none;
  font: 500 11.5px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--a-blue-hi);
}
.jr-cal-wd {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 4px;
  text-align: center;
  font: 500 10.5px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.jr-heat {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 4px;
}
.jr-heat-cell {
  display: grid;
  place-items: center;
  width: 100%;
  aspect-ratio: 1;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  background: var(--bg-2);
  color: var(--tx-muted);
  font: 400 11px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  position: relative;
  transition:
    background-color var(--dur) ease-out,
    border-color var(--dur) ease-out,
    color var(--dur) ease-out;
}
.jr-heat-cell.mstart {
  border-color: var(--br-default);
}
.jr-heat-cell.on {
  background: color-mix(in oklab, var(--a-blue-hi) 30%, var(--bg-2));
  border-color: color-mix(in oklab, var(--a-blue-hi) 38%, transparent);
  color: var(--tx-primary);
  font-weight: 500;
}
.jr-heat-cell.today:not(.open) {
  border-color: color-mix(in oklab, var(--a-blue-hi) 70%, transparent);
  color: var(--tx-primary);
  font-weight: 600;
}
.jr-heat-cell.open {
  background: var(--a-blue-hi);
  border-color: var(--a-blue-hi);
  color: var(--tx-on-accent);
  font-weight: 600;
}
.jr-heat-cell.future {
  background: transparent;
  color: var(--tx-disabled);
  opacity: 0.4;
  cursor: default;
}
.jr-heat-cell:not(.future):hover {
  border-color: var(--br-strong);
  color: var(--tx-primary);
}
.jr-heat-cell.open:hover {
  border-color: var(--a-blue-hi);
  color: var(--tx-on-accent);
}
.jr-heat-cell:focus-visible {
  outline: 2px solid var(--a-focus);
  outline-offset: 1px;
}

.jr-heat-leg {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-top: 2px;
  font: 400 11px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.jr-heat-leg span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.jr-leg-dot {
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 2.5px;
  background: var(--bg-2);
  border: 1px solid var(--br-subtle);
  box-sizing: border-box;
}
.jr-leg-dot.on {
  background: color-mix(in oklab, var(--a-blue-hi) 35%, var(--bg-2));
  border-color: color-mix(in oklab, var(--a-blue-hi) 45%, transparent);
}
.jr-leg-dot.open {
  background: var(--a-blue-hi);
  border-color: var(--a-blue-hi);
}
</style>
