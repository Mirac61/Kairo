<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useHome, type Block } from '@/composables/useHome'
import { columns } from '@/lib/dayPlan'
import { hm } from '@/lib/dates'
import { prefs } from '@/lib/prefs'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const { today, blocks, allDay, freeGaps, nowMin, tasksById, run, setDone, scheduleAt, planAt } = useHome()

// Stundenhöhe: mindestens 44 px, sonst so viel, dass der Tag die Höhe des Widgets füllt.
const boxH = ref(0)
const HOUR = computed(() => Math.max(44, (boxH.value - 20) / (prefs.dayEnd - prefs.dayStart)))
const start = computed(() => prefs.dayStart * 60)
const end = computed(() => prefs.dayEnd * 60)
const hours = computed(() => Array.from({ length: prefs.dayEnd - prefs.dayStart + 1 }, (_, i) => prefs.dayStart + i))
const y = (m: number) => ((m - start.value) / 60) * HOUR.value
// Kürzere Blöcke zeichnet das Raster mit Mindesthöhe (22 px = 30 Minuten); überlappende stehen nebeneinander.
const layout = computed(() => columns(blocks.value, 30))
const style = (b: Block) => {
  const top = Math.max(y(b.start), 0)
  const { col, cols } = layout.value.get(b.key) ?? { col: 0, cols: 1 }
  return {
    top: `${top}px`, height: `${Math.max(y(Math.min(b.end, end.value)) - top, 22)}px`, '--acc': b.color,
    left: `calc((100% - 2px) * ${col / cols})`, width: `calc((100% - 2px) / ${cols} - 2px)`,
  }
}
const gapStyle = (g: { start: number; end: number }) => ({ top: `${y(g.start) + 2}px`, height: `${y(g.end) - y(g.start) - 4}px` })
const nowTop = computed(() => (nowMin.value >= start.value && nowMin.value <= end.value ? y(nowMin.value) : null))

// Beim Öffnen steht die Jetzt-Linie im oberen Drittel; nach Tagesende das Ende des Tages.
const scroller = ref<HTMLElement>()
const ro = new ResizeObserver(([e]) => (boxH.value = e!.contentRect.height))
onUnmounted(() => ro.disconnect())
onMounted(() => {
  ro.observe(scroller.value!)
  boxH.value = scroller.value!.clientHeight
  if (nowTop.value !== null) scroller.value!.scrollTop = nowTop.value - scroller.value!.clientHeight / 3
  else if (nowMin.value > end.value) scroller.value!.scrollTop = scroller.value!.scrollHeight
})

// Aufgaben im Plan per Ziehen verschieben, auf 15 Minuten gerastert.
const drag = (e: DragEvent, b: Block) => {
  e.dataTransfer!.setData('text/plain', b.task!.id)
  e.dataTransfer!.effectAllowed = 'move'
}
function drop(e: DragEvent) {
  const task = tasksById.value.get(e.dataTransfer?.getData('text/plain') ?? '')
  if (!task) return
  const top = (e.currentTarget as HTMLElement).getBoundingClientRect().top
  const snapped = start.value + Math.round(((e.clientY - top) / HOUR.value) * 4) * 15
  scheduleAt(task, Math.min(Math.max(snapped, start.value), end.value - 15))
}
</script>

<template>
  <section class="wg wg-dayplan" :aria-label="$t('Tagesplan')">
    <div class="wg-h">
      <router-link to="/calendar"><svg class="ic"><use href="#i-cal" /></svg>{{ $t('Tagesplan') }}</router-link>
      <span v-if="today" class="mono">{{ $t('{time} frei', { time: hm(today.free_minutes) }) }}</span>
    </div>
    <div v-if="allDay.length" class="wg-allday"><span class="cdot"></span>{{ allDay.map((b) => b.title).join(', ') }}</div>
    <div ref="scroller" class="wd-scroll">
      <div class="wd-body">
        <div class="hourcol" :style="{ '--hour': HOUR + 'px' }">
          <div v-for="h in hours" :key="h" class="hl"><span class="mono">{{ String(h).padStart(2, '0') }}</span></div>
        </div>
        <div class="wd-grid" :style="{ height: (prefs.dayEnd - prefs.dayStart) * HOUR + 'px' }" @dragover.prevent @drop.prevent="drop">
          <div v-for="h in hours" :key="h" class="hline" :style="{ top: (h - prefs.dayStart) * HOUR + 'px' }"></div>
          <div v-for="g in freeGaps" :key="g.start" class="wd-gap" :style="gapStyle(g)">
            <span>{{ $t('frei bis {time}', { time: hm(g.end) }) }}</span>
            <button type="button" class="wg-link" @click="planAt = g.start">{{ $t('+ planen') }}</button>
          </div>
          <div
            v-for="b in blocks" :key="b.key" class="hauch" :class="{ done: b.done, run: b.running, short: b.end - b.start <= 45 }" :style="style(b)"
            :draggable="b.kind === 'task'" @dragstart="drag($event, b)"
          >
            <div class="hauch-t">
              <button
                v-if="b.task" type="button" class="cb ring" :style="{ '--rc': b.color }" role="checkbox" :aria-checked="b.done"
                :disabled="b.done" :aria-label="$t('{title} erledigt', { title: b.title })" @click="setDone(b.task, 'COMPLETED', run)"
              ></button>
              <span class="hauch-n">{{ b.title }}</span>
              <span v-if="b.end - b.start <= 45" class="hauch-m mono">{{ hm(b.start) }}</span>
            </div>
            <div v-if="b.end - b.start > 45" class="hauch-m">{{ hm(b.start) }}–{{ hm(b.end) }}<template v-if="b.sub"> · {{ b.sub }}</template></div>
          </div>
          <div v-if="nowTop !== null" class="wd-now" :style="{ top: nowTop + 'px' }"></div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.wd-scroll { flex: 1; min-height: 0; overflow-y: auto; margin: 0 -4px; padding: 0 4px; }
.wd-body { display: grid; grid-template-columns: 34px minmax(0, 1fr); padding: 8px 0 12px; }
.wd-grid { position: relative; }
.wd-now { position: absolute; left: -34px; right: 0; height: 1px; background: var(--a-blue-hi); z-index: 3; pointer-events: none; }
.wd-gap { position: absolute; left: 0; right: 2px; display: flex; align-items: center; justify-content: center; gap: 6px; min-height: 18px; border: 1px dashed var(--br-default); border-radius: 11px; font: 400 12.5px/1.2 var(--font-ui); color: var(--tx-muted); overflow: hidden; }
.hauch[draggable='true'] { cursor: grab; }
</style>
