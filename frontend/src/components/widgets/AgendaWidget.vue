<script setup lang="ts">
import { computed } from 'vue'
import { useHome, type Block } from '@/composables/useHome'
import { dur, hm } from '@/lib/dates'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const { today, blocks, allDay, freeGaps, unscheduled, nowMin, run, setDone, planAt } = useHome()

// Zeiten und Lücken in einer Liste, nach Beginn sortiert; Vergangenes ist leiser.
type Row = { key: string; start: number; b?: Block; gap?: { start: number; end: number } }
const rows = computed<Row[]>(() =>
  [
    ...blocks.value.map((b) => ({ key: b.key, start: b.start, b })),
    ...freeGaps.value.map((g) => ({ key: `g${g.start}`, start: g.start, gap: g })),
  ].sort((a, b) => a.start - b.start),
)
const live = (b: Block) => b.kind === 'event' && b.start <= nowMin.value && nowMin.value < b.end
const past = (b: Block) => b.end <= nowMin.value
const pct = (b: Block) => ((nowMin.value - b.start) / (b.end - b.start)) * 100
</script>

<template>
  <section class="wg wg-agenda" :aria-label="$t('Agenda „Heute“')">
    <div class="wg-h">
      <router-link to="/calendar"
        ><svg class="ic"><use href="#i-today" /></svg>{{ $t('Heute') }}</router-link
      ><span v-if="today" class="mono">{{
        $t('{time} frei', { time: hm(today.free_minutes) })
      }}</span>
    </div>
    <div class="wa-list">
      <div v-if="allDay.length" class="wg-allday">
        <span class="cdot"></span>{{ allDay.map((b) => b.title).join(', ') }}
      </div>
      <template v-for="r in rows" :key="r.key">
        <div v-if="r.b" class="wa-row" :class="{ past: past(r.b) && !live(r.b) }">
          <div class="wa-tm mono">
            {{ hm(r.b.start)
            }}<template v-if="r.b.kind === 'event'"><br />{{ hm(r.b.end) }}</template>
          </div>
          <div class="wa-main">
            <div class="wa-t">
              <button
                v-if="r.b.task"
                type="button"
                class="cb ring"
                role="checkbox"
                :aria-checked="r.b.done"
                :disabled="r.b.done"
                :aria-label="$t('{title} erledigt', { title: r.b.title })"
                @click="setDone(r.b.task, 'COMPLETED', run)"
              ></button>
              <span v-else class="cdot" :style="{ background: r.b.color }"></span>
              <span class="wa-n" :class="{ done: r.b.done }">{{ r.b.title }}</span>
              <span v-if="r.b.task?.estimated_minutes" class="wa-d mono">{{
                hm(r.b.task.estimated_minutes)
              }}</span>
            </div>
            <div v-if="r.b.sub" class="wa-sub">{{ r.b.sub }}</div>
            <div v-if="live(r.b)" class="wa-live">
              <span class="wa-bar"><i :style="{ width: pct(r.b) + '%' }"></i></span>
              <span class="mono">{{ $t('noch {time}', { time: dur(r.b.end - nowMin) }) }}</span>
            </div>
          </div>
        </div>
        <div v-else-if="r.gap" class="wa-row wa-gap">
          <div class="wa-tm mono">{{ hm(r.gap.start) }}</div>
          <div class="wa-main">
            {{ $t('frei bis {time}', { time: hm(r.gap.end) }) }} ·
            {{ dur(r.gap.end - r.gap.start) }}
          </div>
          <button type="button" class="wg-link" @click="planAt = r.gap.start">
            {{ $t('+ planen') }}
          </button>
        </div>
      </template>
      <div v-for="x in unscheduled" :key="x.id" class="wa-row">
        <div class="wa-tm mono">–</div>
        <div class="wa-main">
          <div class="wa-t">
            <button
              type="button"
              class="cb ring"
              role="checkbox"
              aria-checked="false"
              :aria-label="$t('{title} erledigt', { title: x.title })"
              @click="setDone(x, 'COMPLETED', run)"
            ></button>
            <span class="wa-n">{{ x.title }}</span>
            <span v-if="x.estimated_minutes" class="wa-d mono">{{ hm(x.estimated_minutes) }}</span>
          </div>
        </div>
      </div>
      <div v-if="today && !rows.length && !unscheduled.length" class="wg-empty">
        {{ $t('Nichts geplant für heute.') }}
      </div>
    </div>
  </section>
</template>

<style scoped>
.wa-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  margin: 0 -4px;
  padding: 0 4px;
}
.wa-row {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  padding: 11px 0;
  border-bottom: 1px solid var(--br-subtle);
}
.wa-row:last-child {
  border-bottom: none;
}
.wa-row.past .wa-t {
  color: var(--tx-secondary);
}
.wa-tm {
  width: 40px;
  flex: none;
  font-size: 12px;
  line-height: 1.5;
  color: var(--tx-secondary);
}
.wa-main {
  flex: 1;
  min-width: 0;
}
.wa-t {
  display: flex;
  align-items: center;
  gap: 8px;
  font: 500 14px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.wa-t .cdot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex: none;
}
.wa-t .cb.ring {
  width: 15px;
  height: 15px;
  flex: none;
}
.wa-n {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.wa-n.done {
  text-decoration: line-through;
  color: var(--tx-muted);
}
.wa-d {
  margin-left: auto;
  font-size: 11px;
  color: var(--tx-muted);
}
.wa-sub {
  margin: 2px 0 0 14px;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.wa-live {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0 0 14px;
  font-size: 11px;
  color: var(--tx-muted);
}
.wa-bar {
  flex: 1;
  height: 2px;
  border-radius: 1px;
  background: var(--br-subtle);
}
.wa-bar i {
  display: block;
  height: 2px;
  border-radius: 1px;
  background: var(--a-run);
}
.wa-gap {
  align-items: center;
  font: 400 13px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.wa-gap .wa-tm {
  color: var(--tx-muted);
}
</style>
