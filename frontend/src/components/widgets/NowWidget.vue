<script setup lang="ts">
import { computed } from 'vue'
import { isOpen, taskAction } from '@/api/client'
import { useHome } from '@/composables/useHome'
import { stopwatch } from '@/lib/dayPlan'
import { dur, hm } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const { today, tasksById, projectNames, blocks, nowMin, runEntry, runningTaskId, pausedTask, runSecs, fmt, run, setDone } = useHome()

// Vorrang: laufende oder pausierte Aufgabe, dann ein Termin, der gerade läuft, dann das Nächste.
const runningTask = computed(() => (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined))
const task = computed(() => runningTask.value ?? pausedTask.value)
const project = computed(() => task.value?.project_id ?? runEntry.value?.project_id ?? null)
const goal = computed(() => task.value?.estimated_minutes || 0)
const timerPct = computed(() => (goal.value ? Math.min(100, (runSecs.value / 60 / goal.value) * 100) : 0))

const liveEvent = computed(() => blocks.value.find((b) => b.kind === 'event' && b.start <= nowMin.value && nowMin.value < b.end))
const next = computed(() => {
  const ev = blocks.value.find((b) => b.kind === 'event' && b.start > nowMin.value)
  const tk = (today.value?.tasks ?? []).find((x) => isOpen(x) && x.id !== task.value?.id)
  const tStart = tk?.planned_start_at ? blocks.value.find((b) => b.task?.id === tk.id)?.start ?? null : null
  if (ev && (!tk || (tStart !== null && ev.start < tStart))) return { title: ev.title, when: `${hm(ev.start)}–${hm(ev.end)}`, start: ev.start, task: undefined }
  if (!tk) return null
  return { title: tk.title, when: tStart !== null ? hm(tStart) : '', start: tStart, task: tk }
})
const lead = computed(() => (next.value?.start != null && next.value.start > nowMin.value ? t('in {time}', { time: dur(next.value.start - nowMin.value) }) : ''))
</script>

<template>
  <section class="wg wg-now" :class="{ sm: size === 'S' }" :aria-label="$t('Jetzt')">
    <template v-if="task || runEntry">
      <div class="wg-h"><span class="wg-lbl"><i class="run-dot" :class="{ off: !runEntry }"></i>{{ runEntry ? $t('Läuft') : $t('Pausiert') }}</span><span v-if="runEntry" class="mono">{{ $t('seit {time}', { time: fmt(runEntry.started_at) }) }}</span></div>
      <div class="wn-row">
        <div class="wn-main">
          <div class="wn-t">{{ task?.title ?? projectNames.get(project ?? '') ?? $t('Projektzeit') }}</div>
          <div class="wn-s"><span class="cdot" :style="{ background: projectColor(project) }"></span>{{ projectNames.get(project ?? '') ?? $t('Ohne Projekt') }}<template v-if="goal"> · {{ $t('Ziel') }} {{ hm(goal) }}</template></div>
        </div>
        <span class="wn-timer mono">{{ stopwatch(runSecs) }}</span>
      </div>
      <!-- Fortschritt gegen das Ziel füllt die Fläche, die sonst leer zwischen Titel und Knöpfen läge -->
      <div v-if="goal" class="wn-goal">
        <span class="wn-bar"><i :style="{ width: timerPct + '%' }"></i></span>
        <span class="mono">{{ $t('{a} von {b}', { a: hm(Math.floor(runSecs / 60)), b: hm(goal) }) }}</span>
      </div>
      <div class="wn-foot">
        <template v-if="task">
          <button v-if="runningTask" type="button" class="btn btn-secondary" @click="run(() => taskAction(runningTask!.id, 'pause'))">{{ $t('Pause') }}</button>
          <button v-else type="button" class="btn btn-primary" @click="run(() => taskAction(task!.id, 'start'))">{{ $t('Fortsetzen') }}</button>
          <button type="button" class="btn btn-secondary" @click="setDone(task, 'COMPLETED', run)">{{ $t('Fertig') }}</button>
        </template>
      </div>
    </template>
    <template v-else-if="liveEvent">
      <div class="wg-h"><span class="wg-lbl"><i class="run-dot"></i>{{ $t('Jetzt') }}</span><span class="mono">{{ $t('noch {time}', { time: dur(liveEvent.end - nowMin) }) }}</span></div>
      <div class="wn-t">{{ liveEvent.title }}</div>
      <div class="wn-s">{{ hm(liveEvent.start) }}–{{ hm(liveEvent.end) }}<template v-if="liveEvent.sub"> · {{ liveEvent.sub }}</template></div>
      <div class="wn-foot"><span class="wn-bar"><i :style="{ width: ((nowMin - liveEvent.start) / (liveEvent.end - liveEvent.start)) * 100 + '%' }"></i></span></div>
    </template>
    <template v-else-if="next">
      <div class="wg-h"><span class="wg-lbl"><i class="run-dot off"></i>{{ $t('Als Nächstes') }}</span><span v-if="lead" class="mono">{{ lead }}</span></div>
      <div class="wn-t">{{ next.title }}</div>
      <div v-if="next.when" class="wn-s">{{ next.when }}</div>
      <div v-if="next.task" class="wn-foot">
        <button type="button" class="btn btn-secondary" @click="run(() => taskAction(next!.task!.id, 'start'))">{{ $t('Timer starten') }}</button>
      </div>
    </template>
    <template v-else>
      <div class="wg-h"><span class="wg-lbl"><i class="run-dot off"></i>{{ $t('Jetzt') }}</span></div>
      <div class="wn-t is-free">{{ $t('Es läuft gerade nichts.') }}</div>
      <div class="wn-s">{{ $t('Starte eine Aufgabe, um Zeit zu erfassen.') }}</div>
    </template>
  </section>
</template>

<style scoped>
.wn-row { display: flex; align-items: flex-end; justify-content: space-between; gap: 14px; }
.wn-main { min-width: 0; }
.wn-t { font: 400 32px/1.15 var(--font-serif); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wn-t.is-free { font-size: 28px; color: var(--tx-secondary); }
.wn-s { display: flex; align-items: center; gap: 6px; margin-top: 4px; font: 400 13px/1.3 var(--font-ui); color: var(--tx-secondary); min-width: 0; }
.wn-s .cdot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
.wn-timer { font: 500 34px/1 var(--font-mono); font-variant-numeric: tabular-nums; letter-spacing: -.02em; color: var(--tx-primary); white-space: nowrap; }
.wn-foot { display: flex; align-items: center; gap: 8px; margin-top: auto; }
.wn-foot .btn { height: 30px; }
.wn-goal { display: grid; gap: 6px; margin-top: auto; font-size: 12px; color: var(--tx-secondary); }
.wn-goal .wn-bar { height: 6px; border-radius: 3px; }
.wn-goal .wn-bar i { height: 6px; border-radius: 3px; }
.wn-goal + .wn-foot { margin-top: 4px; }
.wn-bar { flex: 1; height: 3px; border-radius: 2px; background: var(--br-subtle); }
.wn-bar i { display: block; height: 3px; border-radius: 2px; background: var(--a-run); }
/* S: Titel und Uhr untereinander, Nebenzeile und Fortschrittsbalken neben Knöpfen fallen weg */
.sm .wn-row { flex-direction: column; align-items: flex-start; gap: 2px; }
.sm .wn-row .wn-main { max-width: 100%; }
.sm .wn-t { font-size: 20px; }
.sm .wn-t.is-free { font-size: 18px; }
.sm .wn-s { display: none; }
.sm .wn-timer { font-size: 26px; }
.sm .wn-bar:not(:only-child), .sm .wn-goal { display: none; }
/* Handy: Titel über die volle Breite (sonst nur „Laborübung (…“), Uhr und Knöpfe teilen sich die Zeile darunter – passt in die 150px-Zeile */
@media (max-width: 640px) {
  .wg-now:has(.wn-row) { display: grid; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: auto auto 1fr; align-items: end; column-gap: 12px; }
  .wg-now .wg-h, .wn-main { grid-column: 1 / -1; align-self: start; }
  .wn-row { display: contents; }
  .wn-t { font-size: 28px; }
  .wn-timer { font-size: 28px; }
  .wn-goal { display: none; }
}
.run-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--a-run); }
.run-dot.off { background: var(--br-strong); }
</style>
