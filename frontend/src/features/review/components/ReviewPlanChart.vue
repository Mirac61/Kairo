<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { dm, hm, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'
import type { ReviewDay } from '../types'

const props = defineProps<{
  days: ReviewDay[]
}>()

const WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map((d) => t(d))
const today = () => ymd(new Date())

// Skala in ganzen Stunden-Schritten, damit die Hilfslinien glatte Werte haben.
const step = computed(() =>
  Math.max(...props.days.flatMap((d) => [d.tracked_minutes, d.plan])) > 600 ? 240 : 120,
)
const maxMin = computed(() =>
  Math.max(
    240,
    Math.ceil(Math.max(...props.days.flatMap((d) => [d.tracked_minutes, d.plan])) / step.value) *
      step.value,
  ),
)
const ticks = computed(() =>
  Array.from({ length: maxMin.value / step.value + 1 }, (_, i) => i * step.value),
)
const pct = (m: number) => `${m ? Math.max(2, (m / maxMin.value) * 100) : 0}%`

const dayText = (d: ReviewDay) =>
  `${WD[d.i]} ${dm(d.date)}: ` +
  t('erfasst {tracked} h, geplant {plan} h, {done} erledigt', {
    tracked: hm(d.tracked_minutes),
    plan: hm(d.plan),
    done: d.completed_tasks,
  })
</script>

<template>
  <section class="rv-hero">
    <div class="rv-legend">
      <h2 class="rv-h">{{ $t('Plan und Ist pro Tag') }}</h2>
      <span class="spacer"></span> <span><i class="k-plan"></i>{{ $t('Geplant') }}</span
      ><span><i class="k-trk"></i>{{ $t('Erfasst') }}</span
      ><span><i class="k-soon"></i>{{ $t('Kommt noch') }}</span>
    </div>
    <div class="rv-plot">
      <div class="rv-grid" aria-hidden="true">
        <span v-for="m in ticks" :key="m" :style="{ bottom: pct(m) }">{{ m / 60 }} h</span>
      </div>
      <div class="rv-days">
        <RouterLink
          v-for="d in days"
          :key="d.date"
          class="rv-day"
          :class="{ now: d.date === today(), future: d.future }"
          :to="{ name: 'calendar', query: { date: d.date } }"
          :aria-label="dayText(d)"
          :title="dayText(d)"
        >
          <div class="rv-bars" aria-hidden="true">
            <i :class="d.future ? 'k-soon' : 'k-plan'" :style="{ height: pct(d.plan) }"></i>
            <i v-if="!d.future" class="k-trk" :style="{ height: pct(d.tracked_minutes) }"></i>
          </div>
          <span class="rv-dl"
            >{{ WD[d.i] }} <b>{{ Number(d.date.slice(8)) }}</b></span
          >
          <span v-if="d.future" class="rv-n">{{ d.plan ? hm(d.plan) : $t('frei') }}</span>
          <span v-else class="rv-n"
            ><b>{{ hm(d.tracked_minutes) }}</b
            ><span class="rv-of"> / {{ hm(d.plan) }}</span></span
          >
          <span class="rv-done">{{ d.completed_tasks ? `✓ ${d.completed_tasks}` : '' }}</span>
        </RouterLink>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Diagramm in der hervorgehobenen Karte (wie .st-hero) */
.rv-hero {
  margin-bottom: 40px;
  padding: 18px 20px 12px;
  border-radius: 14px;
  background: linear-gradient(180deg, var(--bg-3), var(--bg-2) 60%);
  box-shadow: inset 0 0 0 1px var(--br-subtle);
}
.rv-h {
  margin: 0;
  font: 600 14.5px/1.3 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}
.rv-legend {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 14px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.rv-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.rv-legend i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  box-sizing: border-box;
}
.spacer {
  flex: 1;
}
.k-plan {
  background: color-mix(in srgb, var(--tx-primary) 13%, transparent);
}
.k-trk {
  background: var(--a-blue-hi);
}
.k-soon {
  background: transparent;
  border: 1px dashed var(--br-strong);
}
.rv-plot {
  position: relative;
  padding-left: 30px;
}
.rv-grid {
  position: absolute;
  left: 0;
  right: 0;
  top: 8px;
  height: 140px;
  pointer-events: none;
}
.rv-grid span {
  position: absolute;
  left: 0;
  right: 0;
  height: 0;
  font: 400 10.5px/1 var(--font-mono);
  color: var(--tx-muted);
  transform: translateY(-5px);
}
.rv-grid span::after {
  content: '';
  position: absolute;
  left: 30px;
  right: 0;
  top: 5px;
  border-top: 1px dashed var(--br-subtle);
}
.rv-grid span:first-child::after {
  border-top: 1px solid var(--br-default);
}
.rv-days {
  position: relative;
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 6px;
}
.rv-day {
  display: grid;
  justify-items: center;
  gap: 4px;
  padding: 8px 4px 6px;
  border-radius: var(--r-s);
  color: inherit;
  text-decoration: none;
  transition: background var(--dur) ease-out;
}
.rv-day:hover {
  background: color-mix(in oklab, var(--tx-primary) 4%, transparent);
}
.rv-day.now {
  background: color-mix(in oklab, var(--tx-primary) 6%, transparent);
}
.rv-day:focus-visible {
  outline: 2px solid var(--a-blue-hi);
  outline-offset: 1px;
}
/* Ein Balken je Tag: Plan als Spur, Erfasstes füllt sie von unten. */
.rv-bars {
  position: relative;
  height: 140px;
  width: min(34px, 64%);
  margin-bottom: 6px;
}
.rv-bars i {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  border-radius: 5px 5px 2px 2px;
  box-sizing: border-box;
  transition: height var(--dur) ease-out;
}
.rv-bars .k-soon {
  border-bottom: 0;
}
.rv-dl {
  font: 500 12px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.rv-dl b {
  color: var(--tx-primary);
  font-weight: 600;
}
.rv-day.now .rv-dl b {
  display: inline-grid;
  place-items: center;
  min-width: 20px;
  height: 20px;
  padding: 0 3px;
  border-radius: 10px;
  background: var(--a-blue-hi);
  color: var(--bg-0);
}
.rv-n {
  font: 400 12px/1.2 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
  white-space: nowrap;
}
.rv-n b {
  color: var(--tx-primary);
  font-weight: 500;
}
.rv-done {
  min-height: 11px;
  font: 500 11px/1 var(--font-ui);
  color: var(--tx-muted);
}

@media (max-width: 640px) {
  .rv-hero {
    padding: 14px 10px 8px;
  }
  .rv-plot {
    padding-left: 22px;
  }
  .rv-grid span::after {
    left: 22px;
  }
  .rv-days {
    gap: 2px;
  }
  .rv-bars {
    width: min(24px, 70%);
  }
  .rv-dl {
    font-size: 10px;
    white-space: nowrap;
  }
  .rv-n {
    font-size: 10.5px;
  }
  .rv-of {
    display: none;
  } /* Plan steht im Tooltip und als Balken */
  .rv-legend {
    flex-wrap: wrap;
    row-gap: 10px;
  }
  .rv-legend .spacer {
    flex-basis: 100%;
  }
}
</style>
