<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import type { Task } from '@/api/client'
import type { Block } from '@/composables/useHome'
import { columns } from '@/lib/dayPlan'
import { dur, hm } from '@/lib/dates'
import { vDialog } from '@/lib/dialog'
import { prefs } from '@/lib/prefs'
import { projectColor } from '@/lib/projectColor'

const props = defineProps<{
  blocks: Block[]
  freeGaps: { start: number; end: number }[]
  nowMin: number
  selectedTask?: Task
  allTasks: Task[]
  projectNames: Map<string, string>
  freeMin: number
}>()

const emit = defineEmits<{
  (e: 'scheduleAt', task: Task, minute: number): void
  (e: 'togglePlan', task: Task): void
  (e: 'clearSelectedTask'): void
}>()

const rangeStart = computed(() => prefs.dayStart * 60)
const rangeEnd = computed(() => Math.max(prefs.dayEnd * 60, rangeStart.value + 60))

// Prozentuale Lage eines Minutenwerts auf der Schiene
function relativeY(minute: number): number {
  const clamped = Math.min(Math.max(minute, rangeStart.value), rangeEnd.value)
  return ((clamped - rangeStart.value) / (rangeEnd.value - rangeStart.value)) * 100
}

const rail = ref<HTMLElement | null>(null)
const railHeight = ref(0)
const resizeObserver = new ResizeObserver(([entry]) => {
  if (entry) {
    railHeight.value = entry.contentRect.height
  }
})

watch(rail, (element, previous) => {
  if (previous) resizeObserver.unobserve(previous)
  if (element) resizeObserver.observe(element)
})

onUnmounted(() => {
  resizeObserver.disconnect()
})

function isShortBlock(block: { start: number; end: number }): boolean {
  const fraction = (block.end - block.start) / (rangeEnd.value - rangeStart.value)
  return fraction * railHeight.value < 44
}

const hours = computed(() => {
  const count = prefs.dayEnd - prefs.dayStart + 1
  return Array.from({ length: count }, (_, index) => prefs.dayStart + index)
})

const visibleBlocks = computed(() => {
  return props.blocks.filter(
    (b) => b.end > rangeStart.value && b.start < rangeEnd.value,
  )
})

// Überlappende Blöcke nebeneinander
const railColumns = computed(() => columns(visibleBlocks.value, 30))

function columnStyle(blockKey: string) {
  const colInfo = railColumns.value.get(blockKey) ?? { col: 0, cols: 1 }
  const widthCalc = `calc((100% - 52px) / ${colInfo.cols} - ${colInfo.cols > 1 ? 3 : 0}px)`
  return {
    left: `calc(52px + (100% - 52px) * ${colInfo.col / colInfo.cols})`,
    width: widthCalc,
  }
}

function estimate(task: Task): number {
  return task.estimated_minutes || 30
}

function estimateLabel(task: Task): string {
  return task.estimated_minutes ? dur(task.estimated_minutes) : ''
}

// Je Lücke ein Vorschlag: die erste noch nicht verplante offene Aufgabe, die hineinpasst
const gapSuggestions = computed(() => {
  const usedIds = new Set<string>()
  return props.freeGaps.map((gap) => {
    const gapDuration = gap.end - gap.start
    const foundTask = props.allTasks.find(
      (task) =>
        !task.planned_start_at &&
        !usedIds.has(task.id) &&
        estimate(task) <= gapDuration,
    )
    if (foundTask) {
      usedIds.add(foundTask.id)
    }
    return foundTask
  })
})

const gapToFill = ref<{ start: number; end: number } | null>(null)

function tasksForGap(gap: { start: number; end: number }): Task[] {
  const gapDuration = gap.end - gap.start
  return props.allTasks.filter(
    (task) => !task.planned_start_at && estimate(task) <= gapDuration,
  )
}

function onGapClick(gap: { start: number; end: number }) {
  if (props.selectedTask) {
    if (estimate(props.selectedTask) <= gap.end - gap.start) {
      emit('scheduleAt', props.selectedTask, gap.start)
      emit('clearSelectedTask')
    }
    return
  }
  gapToFill.value = gap
}

const draggedTask = ref<Task | null>(null)
const dropAtMinute = ref<number | null>(null)

function taskFitsInGap(gap: { start: number; end: number }): boolean {
  return Boolean(draggedTask.value && estimate(draggedTask.value) <= gap.end - gap.start)
}

function handleDrop(gap: { start: number; end: number }) {
  if (draggedTask.value && taskFitsInGap(gap)) {
    emit('scheduleAt', draggedTask.value, gap.start)
  }
  draggedTask.value = null
  dropAtMinute.value = null
}
</script>

<template>
  <aside class="st-side" :aria-label="$t('Zeitplan')">
    <div class="st-side-h">
      <span>{{ $t('Zeitplan') }}</span>
      <span class="st-pill">{{ $t('{time} frei', { time: dur(freeMin) }) }}</span>
    </div>

    <div ref="rail" class="st-rail" :style="{ '--mins': rangeEnd - rangeStart }">
      <!-- Stundengitter -->
      <div
        v-for="h in hours"
        :key="h"
        class="st-hour"
        :style="{ top: relativeY(h * 60) + '%' }"
      >
        <span class="mono">{{ String(h).padStart(2, '0') }}:00</span>
        <i></i>
      </div>

      <!-- Freie Lücken -->
      <div
        v-for="(gap, i) in freeGaps"
        :key="gap.start"
        class="st-gap"
        :class="{
          can: taskFitsInGap(gap) || (selectedTask && estimate(selectedTask) <= gap.end - gap.start),
          on: dropAtMinute === gap.start && taskFitsInGap(gap),
        }"
        :style="{
          top: relativeY(gap.start) + '%',
          height: relativeY(gap.end) - relativeY(gap.start) + '%',
        }"
        @dragover="(e) => {
          if (taskFitsInGap(gap)) {
            e.preventDefault()
            dropAtMinute = gap.start
          }
        }"
        @dragleave="dropAtMinute = null"
        @drop.prevent="handleDrop(gap)"
        @click="onGapClick(gap)"
      >
        <div>
          <span>
            {{
              draggedTask
                ? (taskFitsInGap(gap) ? $t('Hier ablegen') : $t('Zu kurz'))
                : selectedTask
                  ? (estimate(selectedTask) <= gap.end - gap.start ? $t('Hier einplanen') : $t('Zu kurz'))
                  : $t('{time} frei', { time: dur(gap.end - gap.start) })
            }}
          </span>
          <button
            v-if="!draggedTask && !selectedTask"
            type="button"
            class="st-gap-add"
            :title="
              gapSuggestions[i]
                ? $t('{title} um {time} einplanen', {
                    title: gapSuggestions[i]!.title,
                    time: hm(gap.start),
                  })
                : $t('Aufgabe einplanen')
            "
            @click.stop="gapToFill = gap"
          >
            + {{ $t('Einplanen') }}
          </button>
        </div>
      </div>

      <!-- Belegte Blöcke -->
      <div
        v-for="block in visibleBlocks"
        :key="block.key"
        class="st-block"
        :class="[
          block.kind,
          {
            short: isShortBlock(block),
            past: block.end <= nowMin,
            cur: block.start <= nowMin && nowMin < block.end,
          },
        ]"
        :style="{
          ...columnStyle(block.key),
          top: relativeY(block.start) + '%',
          height: relativeY(block.end) - relativeY(block.start) + '%',
          '--c': block.kind === 'event' ? 'var(--a-event)' : block.color,
        }"
      >
        <div>
          <span class="st-block-t" :class="{ done: block.done }">{{ block.title }}</span>
          <span class="st-block-m mono">
            {{
              isShortBlock(block)
                ? hm(block.start)
                : block.start <= nowMin && nowMin < block.end
                  ? $t('bis {time} · noch {left}', {
                      time: hm(block.end),
                      left: dur(block.end - nowMin),
                    })
                  : `${hm(block.start)}–${hm(block.end)}`
            }}
          </span>
          <button
            v-if="block.task && !block.done && block.start >= nowMin"
            type="button"
            class="st-block-x"
            :aria-label="$t('Aus dem Plan nehmen')"
            :title="$t('Aus dem Plan nehmen')"
            @click="emit('togglePlan', block.task)"
          >
            ×
          </button>
        </div>
      </div>

      <!-- Jetzt-Markierung -->
      <div
        v-if="nowMin >= rangeStart && nowMin <= rangeEnd"
        class="st-now"
        :style="{ top: relativeY(nowMin) + '%' }"
      >
        <span class="mono">{{ hm(nowMin) }}</span>
      </div>
    </div>
  </aside>

  <!-- Dialog zur Lückenauswahl (Mobil & Touch) -->
  <div
    v-if="gapToFill"
    v-dialog="() => gapToFill = null"
    class="overlay open"
    @click.self="gapToFill = null"
  >
    <div class="dialog st-gap-dialog">
      <div class="dlg-head">
        <h3>{{ $t('Aufgabe einplanen') }}</h3>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Schließen')"
          @click="gapToFill = null"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <div class="st-gap-dialog-info">
          <span class="mono">{{ hm(gapToFill.start) }} – {{ hm(gapToFill.end) }}</span>
          <span>· {{ $t('{time} frei', { time: dur(gapToFill.end - gapToFill.start) }) }}</span>
        </div>
        <div v-if="tasksForGap(gapToFill).length" class="st-gap-tasks">
          <button
            v-for="task in tasksForGap(gapToFill)"
            :key="task.id"
            type="button"
            class="st-gap-task-item"
            @click="emit('scheduleAt', task, gapToFill!.start); gapToFill = null"
          >
            <span class="t">{{ task.title }}</span>
            <span
              v-if="task.project_id"
              class="chip"
              :style="{ '--chip-c': projectColor(task.project_id) }"
            >
              <span class="cdot"></span>{{ projectNames.get(task.project_id) }}
            </span>
            <span class="dur mono">{{ estimateLabel(task) }}</span>
          </button>
        </div>
        <div v-else class="st-empty">
          {{ $t('Keine passende offene Aufgabe für diese Lücke.') }}
        </div>
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" @click="gapToFill = null">
          {{ $t('Abbrechen') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.st-side {
  position: sticky;
  top: 72px;
  display: flex;
  flex-direction: column;
}
.st-side-h {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-secondary);
}
.st-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-subtle);
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.st-rail {
  position: relative;
  height: clamp(520px, calc(100vh - 140px), 760px);
  padding: 0 0 0 52px;
  border-radius: var(--r-m);
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  overflow: hidden;
}
.st-hour {
  position: absolute;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  pointer-events: none;
}
.st-hour > span {
  width: 44px;
  padding-left: 8px;
  font-size: 11px;
  color: var(--tx-muted);
}
.st-hour > i {
  flex: 1;
  height: 1px;
  background: var(--br-subtle);
}
.st-gap {
  position: absolute;
  left: 52px;
  right: 0;
  display: flex;
  align-items: center;
  padding: 0 12px;
  border-left: 2px dashed var(--br-subtle);
  background: transparent;
  color: var(--tx-muted);
  font: 400 12px/1 var(--font-ui);
  cursor: pointer;
  transition: background var(--dur), border-color var(--dur);
}
.st-gap:hover {
  background: var(--bg-hover);
}
.st-gap.can {
  border-left-color: var(--a-blue);
  background: color-mix(in srgb, var(--a-blue) 4%, transparent);
}
.st-gap.on {
  background: color-mix(in srgb, var(--a-blue) 12%, transparent);
}
.st-gap > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
.st-gap-add {
  display: none;
  height: 22px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: var(--bg-2);
  color: var(--tx-secondary);
  font: 500 11px/1 var(--font-ui);
  cursor: pointer;
}
.st-gap:hover .st-gap-add {
  display: inline-flex;
  align-items: center;
}
.st-block {
  position: absolute;
  border-radius: 6px;
  background: color-mix(in srgb, var(--c, var(--tx-muted)) 12%, var(--bg-1));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--c, var(--tx-muted)) 30%, transparent);
  border-left: 3px solid var(--c, var(--tx-muted));
  padding: 4px 8px;
  overflow: hidden;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-primary);
  transition: box-shadow var(--dur);
}
.st-block > div {
  display: grid;
  height: 100%;
  align-content: start;
}
.st-block-t {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-block-t.done {
  text-decoration: line-through;
  opacity: .7;
}
.st-block-m {
  font-size: 11px;
  color: var(--tx-muted);
}
.st-block-x {
  position: absolute;
  top: 4px;
  right: 4px;
  display: none;
  width: 18px;
  height: 18px;
  border: 0;
  border-radius: 4px;
  background: var(--bg-1);
  color: var(--tx-muted);
  font-size: 13px;
  cursor: pointer;
}
.st-block:hover .st-block-x {
  display: grid;
  place-items: center;
}
.st-block.short {
  display: flex;
  align-items: center;
  padding: 0 8px;
}
.st-block.short > div {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}
.st-block.short .st-block-m {
  margin-left: auto;
}
.st-now {
  position: absolute;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  pointer-events: none;
  z-index: 2;
}
.st-now > span {
  width: 44px;
  padding-left: 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--a-red);
}
.st-now::after {
  content: '';
  flex: 1;
  height: 2px;
  background: var(--a-red);
}

.st-gap-dialog {
  width: 440px;
}
.st-gap-dialog-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-secondary);
}
.st-gap-tasks {
  display: grid;
  gap: 6px;
  max-height: 320px;
  overflow-y: auto;
}
.st-gap-task-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: var(--bg-1);
  box-shadow: inset 0 0 0 1px var(--br-default);
  color: var(--tx-primary);
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
  text-align: left;
}
.st-gap-task-item:hover {
  background: var(--bg-hover);
}
.st-gap-task-item .t {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-gap-task-item .dur {
  color: var(--tx-muted);
  font-size: 11.5px;
}
.st-empty {
  padding: 24px 0;
  text-align: center;
  color: var(--tx-muted);
  font-size: 13px;
}
</style>
