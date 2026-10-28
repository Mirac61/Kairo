<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getReview, isOpen } from '@/api/client'
import { useHome } from '@/composables/useHome'
import { hm, shiftDate } from '@/lib/dates'
import { locale } from '@/lib/i18n'
import { prefs } from '@/lib/prefs'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const { today, tasks } = useHome()
const calendar = ref(new Map<string, number>())

// Montag bis Sonntag der laufenden Woche.
const days = computed(() => {
  const d = today.value?.date
  if (!d) return []
  const dow = (new Date(`${d}T12:00:00Z`).getUTCDay() + 6) % 7
  return Array.from({ length: 7 }, (_, i) => shiftDate(d, i - dow))
})
watch(
  days,
  async (ds) => {
    if (!ds.length) return
    const r = await getReview(ds[0]!, ds[6]!).catch(() => null)
    calendar.value = new Map((r?.days ?? []).map((x) => [x.date, x.calendar_minutes]))
  },
  { immediate: true },
)

// Auslastung je Tag: Termine plus geplante offene Aufgaben (ohne Schätzung 30 Minuten), gemessen am sichtbaren Tagesbereich.
const load = computed(() =>
  days.value.map((d) => {
    const planned = tasks.value
      .filter((x) => x.planned_date === d && isOpen(x))
      .reduce((s, x) => s + (x.estimated_minutes || 30), 0)
    const min = (calendar.value.get(d) ?? 0) + planned
    return { d, min, pct: Math.min(100, (min / ((prefs.dayEnd - prefs.dayStart) * 60)) * 100) }
  }),
)
const total = computed(() => load.value.reduce((s, x) => s + x.min, 0))
const weekday = computed(
  () => new Intl.DateTimeFormat(locale.value, { weekday: 'narrow', timeZone: 'UTC' }),
)
// Kalenderwoche nach ISO 8601: die Woche mit dem Donnerstag.
const kw = computed(() => {
  const thu = days.value[3]
  if (!thu) return ''
  const d = new Date(`${thu}T00:00:00Z`)
  return Math.ceil(((d.getTime() - Date.UTC(d.getUTCFullYear(), 0, 1)) / 864e5 + 1) / 7)
})
</script>

<template>
  <section class="wg wg-week" :aria-label="$t('Woche')">
    <div class="wg-h">
      <router-link to="/review"
        ><svg class="ic"><use href="#i-bars" /></svg>{{ $t('Woche') }}</router-link
      ><span class="mono">{{ $t('KW {n}', { n: kw }) }}</span>
    </div>
    <div v-if="size === 'M'" class="ww-top">
      <span class="wg-big">{{ hm(total) }}</span
      ><span class="wg-meta">{{ $t('verplant') }}</span>
    </div>
    <div class="ww-bars">
      <span
        v-for="x in load"
        :key="x.d"
        class="ww-bar"
        :class="{ today: x.d === today?.date }"
        :title="`${x.d}: ${hm(x.min)}`"
        ><i :style="{ height: Math.max(x.pct, 6) + '%' }"></i
      ></span>
    </div>
    <div class="ww-days mono" aria-hidden="true">
      <span v-for="x in load" :key="x.d" :class="{ today: x.d === today?.date }">{{
        weekday.format(new Date(`${x.d}T12:00:00Z`))
      }}</span>
    </div>
  </section>
</template>

<style scoped>
.ww-bars {
  flex: 1;
  min-height: 56px;
  display: flex;
  gap: 8px;
  align-items: stretch;
}
.ww-bar {
  flex: 1;
  display: flex;
  align-items: flex-end;
}
.ww-bar i {
  display: block;
  width: 100%;
  border-radius: 3px;
  background: var(--bg-3);
}
.ww-bar.today i {
  background: var(--br-strong);
}
.ww-top {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.ww-days {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--tx-muted);
}
.ww-days span {
  flex: 1;
  text-align: center;
}
.ww-days .today {
  color: var(--tx-primary);
}
</style>
