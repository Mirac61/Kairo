<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { completeHabit, habitsDue, uncompleteHabit } from '@/api/client'
import { useHome } from '@/composables/useHome'
import { shiftDate } from '@/lib/dates'
import { locale } from '@/lib/i18n'
import { habitColor } from '@/lib/projectColor'
import type { Size } from '@/lib/homeLayout'

const props = defineProps<{ size: Size }>()
const { today, run } = useHome()
const habits = computed(() => today.value?.habits ?? [])
const shown = computed(() => habits.value.slice(0, props.size === 'S' ? 4 : 5))
const done = computed(() => habits.value.filter((h) => h.done).length)
const toggle = (id: string, isDone: boolean) => run(() => (isDone ? uncompleteHabit(id, today.value!.date) : completeHabit(id, today.value!.date)))

// Montag bis Sonntag; je Gewohnheit und Tag: erledigt, fällig oder nicht dran. S zeigt nur die Liste, sonst werden die Namen unlesbar.
const days = computed(() => {
  const d = today.value?.date
  if (!d || props.size === 'S') return []
  const dow = (new Date(`${d}T12:00:00Z`).getUTCDay() + 6) % 7
  return Array.from({ length: 7 }, (_, i) => shiftDate(d, i - dow))
})
const week = ref(new Map<string, boolean>()) // `${habit}|${date}` → erledigt
watch([days, () => habits.value.map((h) => `${h.id}${h.done}`).join()], async ([ds]) => {
  if (!ds.length) return
  const due = await habitsDue(ds[0]!, ds.at(-1)!).catch(() => [])
  week.value = new Map(due.map((x) => [`${x.habit_id}|${x.date}`, x.done]))
}, { immediate: true })
const dayState = (id: string, d: string) => {
  const v = week.value.get(`${id}|${d}`)
  return v === undefined ? 'off' : v ? 'done' : d < (today.value?.date ?? '') ? 'missed' : 'due'
}
const weekday = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'narrow', timeZone: 'UTC' }))

// Ring: Anteil heute erledigt.
const R = 34
const C = 2 * Math.PI * R
const dash = computed(() => (habits.value.length ? (done.value / habits.value.length) * C : 0))
</script>

<template>
  <section class="wg wg-habits" :class="{ sm: size === 'S' }" :aria-label="$t('Gewohnheiten')">
    <div class="wg-h"><router-link to="/habits"><svg class="ic"><use href="#i-habits" /></svg>{{ $t('Gewohnheiten') }}</router-link><span v-if="size === 'S' && habits.length" class="mono">{{ done }}/{{ habits.length }}</span><span v-else>{{ $t('heute') }}</span></div>
    <div v-if="habits.length" class="wh-body">
      <div v-if="size !== 'S'" class="wh-ring" role="img" :aria-label="$t('{n} von {m} erledigt', { n: done, m: habits.length })">
        <svg viewBox="0 0 80 80" aria-hidden="true">
          <circle cx="40" cy="40" :r="R" class="wh-track" />
          <circle cx="40" cy="40" :r="R" class="wh-fill" :stroke-dasharray="`${dash} ${C}`" />
        </svg>
        <span class="wh-n">{{ done }}<small>/{{ habits.length }}</small></span>
      </div>
      <div class="wh-list">
        <div v-if="days.length" class="wh-days" aria-hidden="true"><span></span><i v-for="d in days" :key="d" :class="{ today: d === today?.date }">{{ weekday.format(new Date(`${d}T12:00:00Z`)) }}</i></div>
        <div v-for="h in shown" :key="h.id" class="wh-row">
          <button type="button" class="wh-item" role="checkbox" :aria-checked="h.done" @click="toggle(h.id, h.done)">
            <span class="cb ring" :style="{ '--rc': habitColor(h.id) }" :aria-checked="h.done" aria-hidden="true"></span>
            <span class="wh-name">{{ h.name }}</span>
          </button>
          <i v-for="d in days" :key="d" class="wh-dot" :class="dayState(h.id, d)" :style="{ '--rc': habitColor(h.id) }" aria-hidden="true"></i>
        </div>
        <router-link v-if="size !== 'S' && habits.length > shown.length" to="/habits" class="wg-more">{{ $t('+{n} weitere', { n: habits.length - shown.length }) }}</router-link>
      </div>
    </div>
    <div v-else-if="today" class="wg-empty">{{ $t('Heute keine Gewohnheiten.') }}</div>
  </section>
</template>

<style scoped>
.wh-body { flex: 1; min-height: 0; display: flex; align-items: center; gap: 22px; }
.wh-ring { position: relative; flex: none; width: 96px; height: 96px; }
.wh-ring svg { width: 100%; height: 100%; transform: rotate(-90deg); }
.wh-track { fill: none; stroke: var(--br-subtle); stroke-width: 6; }
.wh-fill { fill: none; stroke: var(--a-run); stroke-width: 6; stroke-linecap: round; transition: stroke-dasharray var(--dur) ease-out; }
.wh-n { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; font: 400 36px/1 var(--font-serif); color: var(--tx-primary); }
.wh-n small { font: 400 14px/1 var(--font-ui); color: var(--tx-muted); margin: 10px 0 0 2px; }
.wh-list { flex: 1; min-width: 0; display: grid; gap: 6px; }
.wh-days, .wh-row { display: grid; grid-template-columns: minmax(18px, 1fr) repeat(7, 14px); column-gap: 6px; align-items: center; }
.wh-days i { font: 400 12px/1 var(--font-ui); font-style: normal; color: var(--tx-muted); text-align: center; }
.wh-days i.today { color: var(--tx-primary); }
.wh-item { display: flex; align-items: center; gap: 10px; min-height: 32px; min-width: 0; padding: 0; background: none; border: 0; text-align: left; font: 500 15px/1.2 var(--font-ui); color: var(--tx-primary); }
.wh-item .cb { width: 18px; height: 18px; flex: none; pointer-events: none; }
.wh-item[aria-checked='true'] .wh-name { color: var(--tx-muted); }
.wh-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wh-dot { width: 10px; height: 10px; justify-self: center; border-radius: 50%; }
.wh-dot.done { background: var(--rc); }
.wh-dot.due { box-shadow: inset 0 0 0 1.5px var(--rc); }
.wh-dot.missed { box-shadow: inset 0 0 0 1.5px var(--br-strong); }
.sm .wh-body { align-items: flex-start; }
.sm .wh-row { grid-template-columns: minmax(0, 1fr); }
/* In der 150px-Kachel am Handy passen drei Zeilen; den Rest zeigt die Zahl im Kopf. */
@media (max-width: 640px) { .sm .wh-row:nth-child(n + 4) { display: none; } }
.wh-dot.off { width: 4px; height: 4px; background: var(--br-default); }
</style>
