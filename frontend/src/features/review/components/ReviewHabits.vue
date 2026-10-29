<script setup lang="ts">
import type { Habit, Review } from '@/api/client'
import { habitColor } from '@/lib/projectColor'
import { iconOf } from '@/lib/habitIcons'

const props = defineProps<{
  reviewHabits: Review['habits']
  allHabits: Habit[]
}>()

// Wochenziel pro Rhythmus; 0 = unbekannt.
function target(id: string) {
  const h = props.allHabits.find((x) => x.id === id)
  if (!h) return 0
  if (h.frequency_type === 'DAILY') return 7
  if (h.frequency_type === 'WEEKLY') return 1
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return h.frequency_config.weekdays?.length ?? 0
  return h.frequency_config.times ?? 0
}

const habitIcon = (id: string, name: string) =>
  iconOf(props.allHabits.find((x) => x.id === id) ?? { name })

// Segmente wie beim Fokus-Ziel auf der Startseite; bei mehr als 7 Zielen pro Woche reicht ein Balken.
const segs = (id: string) => {
  const n = target(id)
  return n > 0 && n <= 7 ? n : 0
}
</script>

<template>
  <section class="rv-group">
    <h2 class="rv-gh">
      {{ $t('Gewohnheiten') }} <span class="count">{{ reviewHabits.length || '' }}</span>
    </h2>
    <div
      v-for="h in reviewHabits"
      :key="h.habit_id"
      class="rv-row rv-habit"
      :class="{ met: target(h.habit_id) && h.done >= target(h.habit_id) }"
      :style="{ '--hc': habitColor(h.habit_id) }"
    >
      <span class="rv-hicon" aria-hidden="true"
        ><svg class="ic"><use :href="`#${habitIcon(h.habit_id, h.name)}`" /></svg
      ></span>
      <span class="rv-ht">
        <span class="t">{{ h.name }}</span>
        <span class="rv-hs"
          >{{
            target(h.habit_id)
              ? $t('{done} von {target}', { done: h.done, target: target(h.habit_id) })
              : h.done
          }}<template v-if="h.streak">
            · <svg class="ic" aria-hidden="true"><use href="#i-flame" /></svg
            >{{ $t('{n} Tage Serie', { n: h.streak }) }}</template
          ></span
        >
      </span>
      <span v-if="segs(h.habit_id)" class="rv-segs" aria-hidden="true"
        ><i v-for="n in segs(h.habit_id)" :key="n" :class="{ on: n <= h.done }"></i
      ></span>
      <span v-else class="rv-track rv-htrack" aria-hidden="true"
        ><span
          :style="{
            width: `${target(h.habit_id) ? Math.min(100, (h.done / target(h.habit_id)) * 100) : 0}%`,
          }"
        ></span
      ></span>
    </div>
    <div v-if="!reviewHabits.length" class="rv-empty">
      {{ $t('Keine aktiven Gewohnheiten.') }}
    </div>
  </section>
</template>

<style scoped>
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
.rv-row .t {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font: 500 14px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.rv-habit {
  grid-template-columns: 30px minmax(0, 1fr) auto;
}
.rv-hicon {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary));
  background: color-mix(in srgb, var(--hc) 14%, var(--bg-2));
}
.rv-hicon .ic {
  width: 52%;
  height: 52%;
  stroke-width: 1.9;
}
.rv-ht {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.rv-hs {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font: 400 12px/1.2 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.rv-hs .ic {
  width: 12px;
  height: 12px;
  margin-left: 2px;
}
.rv-segs {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: 14px;
  gap: 3px;
}
.rv-segs i {
  height: 6px;
  border-radius: 3px;
  background: var(--br-default);
}
.rv-segs i.on {
  background: var(--hc);
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
.rv-htrack {
  width: 116px;
}
.rv-htrack span {
  background: var(--hc);
}
.rv-empty {
  padding: 10px 0;
  font: 400 13px/1.5 var(--font-ui);
  color: var(--tx-muted);
}
</style>
