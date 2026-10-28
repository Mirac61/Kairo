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
    })
)

const heatCells = computed<HeatCell[]>(() => {
  if (!props.todayStr) return []
  const dateSet = new Set(props.allDates)
  const today = props.todayStr
  const start = addDays(new Date(`${today}T00:00:00`), -(HEAT_DAYS - 1))

  return Array.from({ length: HEAT_DAYS }, (_, i) => {
    const d = addDays(start, i)
    const s = ymd(d)
    return {
      s,
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
  }
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
  const step = e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0
  if (!step) return
  e.preventDefault()

  const cells = heatCells.value
  const from = cells.findIndex((x) => x.s === c.s)
  let max = cells.length - 1
  while (max > 0 && cells[max]!.future) max--
  const to = Math.min(max, Math.max(0, from + step))
  heatFocus.value = cells[to]!.s

  const buttons = (e.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>(
    '.jr-heat-cell'
  )
  buttons?.[to]?.focus()
}
</script>

<template>
  <div class="jr-meta">
    <span v-if="streak" class="jr-streak">{{
      $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge')
    }}</span>
    <div
      class="jr-heat"
      role="group"
      :aria-label="
        $tn(
          heatCount,
          '{n} Eintrag in den letzten 6 Wochen',
          '{n} Einträge in den letzten 6 Wochen'
        )
      "
    >
      <button
        v-for="c in heatCells"
        :key="c.s"
        type="button"
        class="jr-heat-cell"
        :data-date="c.s"
        :class="{ on: c.has, future: c.future, open: c.s === activeDate }"
        :disabled="c.future"
        :tabindex="c.s === heatFocus ? 0 : -1"
        :title="heatTitle(c)"
        :aria-label="heatAria(c)"
        @focus="heatFocus = c.s"
        @click="openHeat(c)"
        @keydown="onHeatKey($event, c)"
      ></button>
    </div>
    <span class="sr-only">{{ $t('Tag mit Eintrag') }}</span>
  </div>
</template>

<style scoped>
.jr-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 16px;
}
.jr-streak {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-heat {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.jr-heat-cell {
  width: 6px;
  height: 6px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: var(--tx-muted);
  opacity: 0.3;
  cursor: pointer;
  position: relative;
  transition:
    opacity var(--dur) ease-out,
    transform var(--dur) ease-out;
}
.jr-heat-cell::before {
  content: '';
  position: absolute;
  inset: -3px;
}
.jr-heat-cell.on {
  background: var(--p-orange);
  opacity: 0.9;
}
.jr-heat-cell.open {
  outline: 1px solid var(--tx-secondary);
  outline-offset: 2px;
  opacity: 1;
}
.jr-heat-cell.future {
  background: var(--br-subtle);
  opacity: 0.15;
  cursor: default;
}
.jr-heat-cell:not(.future):hover {
  opacity: 1;
  transform: scale(1.3);
}
.jr-heat-cell:focus-visible {
  outline: 2px solid var(--tx-secondary);
  outline-offset: 2px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@media (max-width: 640px) {
  .jr-meta {
    gap: 12px;
  }
  .jr-heat-cell:nth-child(-n + 14) {
    display: none;
  }
}
</style>
