<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Draggable } from '@fullcalendar/interaction'
import type { Project, Task } from '@/api/client'
import { hm, hhmm } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'
import SelectionBar from '@/components/SelectionBar.vue'

export interface NextUpItem {
  id: string
  title: string
  start: Date
  end: Date
  proj: string
  color: string
  task?: Task
}

const props = defineProps<{
  tasks: Task[]
  projects: Project[]
  isDay: boolean
  isOpen: boolean
  nextUp: NextUpItem | null
  pickedIds: string[]
  selectionMode: boolean
}>()

const emit = defineEmits<{
  (e: 'toggleOpen'): void
  (e: 'pickRow', event: MouseEvent, taskId: string): void
  (e: 'pointerDown', event: PointerEvent, taskId: string): void
  (e: 'pointerUp', event: PointerEvent): void
  (e: 'startNextUp', task: Task): void
  (e: 'pauseNextUp', task: Task): void
  (e: 'completeNextUp', task: Task): void
  (e: 'postponeNextUp', task: Task): void
  (e: 'toTomorrow'): void
  (e: 'toProject', projectId: string): void
  (e: 'trashSelection'): void
  (e: 'clearSelection'): void
}>()

const unplannedEl = ref<HTMLElement>()
let draggableInstance: Draggable | undefined

const DEFAULT_TASK_MINUTES = 30

onMounted(() => {
  if (unplannedEl.value) {
    draggableInstance = new Draggable(unplannedEl.value, {
      itemSelector: '.up-item',
      eventData: (elem) => ({
        create: false,
        duration: { minutes: Number(elem.dataset.min) || DEFAULT_TASK_MINUTES },
      }),
    })
  }
})

onBeforeUnmount(() => {
  draggableInstance?.destroy()
})

function formatInMinutes(d: Date): string {
  const m = Math.max(0, Math.round((d.getTime() - Date.now()) / 60_000))
  return m < 60 ? `${m} min` : `${hm(m)} h`
}

function projectColorName(projectId: string | null | undefined): string {
  if (!projectId) return 'blue'
  const p = props.projects.find((proj) => proj.id === projectId)
  return p?.color ?? 'blue'
}
</script>

<template>
  <aside class="unplanned" :class="{ shut: !isOpen, wide: isDay }" aria-labelledby="up-title">
    <!-- Als Nächstes (nur in der Tagesansicht) -->
    <section v-if="isDay && nextUp && isOpen" class="next" :aria-label="$t('Als Nächstes')">
      <div class="next-head">
        <span
          >{{ $t('Als Nächstes') }} ·
          {{ $t('in {time}', { time: formatInMinutes(nextUp.start) }) }}</span
        >
        <span>{{ nextUp.task ? $t('Aufgabe') : $t('Termin') }}</span>
      </div>
      <h3>{{ nextUp.title }}</h3>
      <div class="next-meta">
        <span>
          <i class="next-dot" :style="{ background: nextUp.color }" />
          {{ nextUp.proj || $t('Ohne Projekt') }}
        </span>
        <span>{{ hhmm(nextUp.start) }}–{{ hhmm(nextUp.end) }}</span>
        <span>{{ hm(Math.round((nextUp.end.getTime() - nextUp.start.getTime()) / 60_000)) }}</span>
      </div>
      <div v-if="nextUp.task" class="row nowrap">
        <button
          v-if="nextUp.task.status === 'IN_PROGRESS'"
          type="button"
          class="btn btn-primary"
          @click="emit('pauseNextUp', nextUp.task)"
        >
          {{ $t('Pause') }}
        </button>
        <button
          v-else
          type="button"
          class="btn btn-primary"
          @click="emit('startNextUp', nextUp.task)"
        >
          {{ $t('Start') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          @click="emit('completeNextUp', nextUp.task)"
        >
          {{ $t('Fertig') }}
        </button>
        <button type="button" class="btn btn-ghost" @click="emit('postponeNextUp', nextUp.task)">
          {{ $t('Verschieben') }}
        </button>
      </div>
    </section>

    <!-- Eingeklappte Seitenleiste: Klares Inbox-Icon mit Badge -->
    <div v-if="!isOpen" class="up-shut-wrap">
      <button
        type="button"
        class="up-tray-btn"
        :aria-expanded="false"
        :aria-label="$t('Ungeplant ausklappen')"
        :title="`${$t('Ungeplant')} (${tasks.length})`"
        :data-tip="`${$t('Ungeplant')} (${tasks.length})`"
        @click="emit('toggleOpen')"
      >
        <div class="up-tray-icon-wrap">
          <svg class="ic"><use href="#i-inbox" /></svg>
          <span v-if="tasks.length" class="up-tray-badge">{{ tasks.length }}</span>
        </div>
        <span class="up-tray-lbl">{{ $t('Ungeplant') }}</span>
      </button>
    </div>

    <!-- Ausgeklappte Seitenleiste -->
    <div v-else class="up-head">
      <h2 id="up-title">
        <span class="up-lbl">{{ $t('Ungeplant') }} </span>
        <span class="up-count">{{ tasks.length }}</span>
      </h2>
      <span v-if="isDay && isOpen" class="up-hint">{{ $t('in den Plan ziehen') }}</span>
      <button
        type="button"
        class="icon-btn"
        :aria-expanded="true"
        :aria-label="$t('Ungeplant einklappen')"
        :title="$t('Ungeplant einklappen')"
        :data-tip="$t('Ungeplant einklappen')"
        @click="emit('toggleOpen')"
      >
        <svg class="ic"><use href="#i-right" /></svg>
      </button>
    </div>

    <div v-show="isOpen" ref="unplannedEl" class="up-list">
      <div
        v-for="t in tasks"
        :key="t.id"
        class="up-item"
        :class="{ picked: pickedIds.includes(t.id) }"
        :data-id="t.id"
        :data-color="projectColorName(t.project_id)"
        :data-min="t.estimated_minutes || ''"
        :style="{ '--acc': projectColor(t.project_id) }"
        @click.capture="emit('pickRow', $event, t.id)"
        @pointerdown="emit('pointerDown', $event, t.id)"
        @pointerup="emit('pointerUp', $event)"
        @pointercancel="emit('pointerUp', $event)"
      >
        <span class="up-dot" />
        <span class="up-name">{{ t.title }}</span>
        <small v-if="t.estimated_minutes" class="up-min">{{ hm(t.estimated_minutes) }}</small>
      </div>
    </div>

    <SelectionBar
      v-if="pickedIds.length || selectionMode"
      :count="pickedIds.length"
      :projects="projects.filter((p) => p.status !== 'ARCHIVED')"
      @tomorrow="emit('toTomorrow')"
      @project="(pid) => emit('toProject', pid)"
      @trash="emit('trashSelection')"
      @clear="emit('clearSelection')"
    />
  </aside>
</template>

<style scoped>
.unplanned {
  flex: none;
  width: 250px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px;
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-m);
  box-sizing: border-box;
}
.unplanned.wide:not(.shut) {
  width: 320px;
}
.up-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.unplanned.shut {
  width: 40px;
  padding: 10px 4px;
  align-items: center;
}
.up-shut-wrap {
  display: flex;
  justify-content: center;
  width: 100%;
}
.up-tray-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: var(--r-s);
  background: transparent;
  color: var(--tx-secondary);
  cursor: pointer;
  transition:
    background var(--dur) ease-out,
    color var(--dur) ease-out;
}
.up-tray-btn:hover {
  background: var(--bg-hover);
  color: var(--tx-primary);
}
.up-tray-icon-wrap {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
}
.up-tray-icon-wrap .ic {
  width: 16px;
  height: 16px;
}
.up-tray-badge {
  position: absolute;
  top: -4px;
  right: -6px;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  border-radius: 7px;
  background: var(--a-blue);
  color: var(--tx-on-accent);
  font: 600 9.5px/14px var(--font-mono);
  font-variant-numeric: tabular-nums;
  text-align: center;
  box-sizing: border-box;
  box-shadow: 0 0 0 1.5px var(--bg-1);
}
.up-tray-lbl {
  display: none;
}
.unplanned h2,
.unplanned h3 {
  margin: 0;
  font: 600 13px/1.3 var(--font-ui);
  color: var(--tx-primary);
}
.up-count {
  margin-left: 5px;
  font: 500 11px/1 var(--font-mono);
  color: var(--tx-muted);
}
.unplanned.shut .up-count {
  margin-left: 0;
}
.up-list {
  margin-top: 10px;
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.up-hint {
  flex: 1;
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
  text-align: right;
}
.up-item {
  --ev-acc: var(--p-blue);

  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  cursor: grab;
  border: 1px solid color-mix(in srgb, var(--ev-acc) 22%, var(--br-subtle));
  border-left: 3px solid var(--ev-acc);
  border-radius: 6px;
  background: color-mix(in srgb, var(--ev-acc) 8%, var(--bg-1));
  font: 450 12.5px/1.35 var(--font-ui);
  color: var(--tx-primary);
  transition:
    background var(--dur) ease-out,
    border-color var(--dur) ease-out;
}
.up-item:hover {
  background: color-mix(in srgb, var(--ev-acc) 14%, var(--bg-1));
  border-color: color-mix(in srgb, var(--ev-acc) 38%, var(--br-default));
  border-left-color: var(--ev-acc);
}
.up-item.picked {
  background: color-mix(in srgb, var(--a-blue) 18%, var(--bg-1));
  box-shadow: inset 0 0 0 1px var(--a-blue);
}
.up-dot {
  display: none;
}
.up-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--tx-primary);
  font-weight: 500;
  font-size: 12.5px;
}
.up-min {
  flex: none;
  font: 400 11.5px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.up-item[data-color='blue'],
.up-item:not([data-color]) {
  --ev-acc: var(--p-blue);
}
.up-item[data-color='green'] {
  --ev-acc: var(--p-green);
}
.up-item[data-color='aqua'] {
  --ev-acc: var(--p-aqua);
}
.up-item[data-color='orange'],
.up-item[data-color='yellow'] {
  --ev-acc: var(--p-orange);
}
.up-item[data-color='violet'],
.up-item[data-color='indigo'] {
  --ev-acc: var(--p-violet);
}
.up-item[data-color='red'] {
  --ev-acc: var(--p-red);
}
.up-item[data-color='pink'] {
  --ev-acc: var(--p-pink);
}
.next {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px;
  margin-bottom: 12px;
  background: var(--bg-2);
  border: 1px solid var(--br-subtle);
  border-radius: 10px;
}
.next-head {
  display: flex;
  justify-content: space-between;
  font: 400 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.next h3 {
  margin: 0;
  font: 600 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  text-wrap: pretty;
}
.next-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
  margin-top: -4px;
  font: 400 12px/1.3 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-secondary);
}
.next-meta span {
  display: flex;
  align-items: center;
  gap: 5px;
}
.next-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.next .row {
  gap: 6px;
}
.next .btn {
  padding: 4px 10px;
  font-size: 12px;
}

@media (max-width: 1200px) {
  .unplanned:not(.shut) {
    width: 220px;
  }
  .unplanned.wide:not(.shut) {
    width: 280px;
  }
}

@media (max-width: 900px) {
  .unplanned:not(.shut) {
    width: auto;
    max-height: 200px;
    order: 2;
  }
}

@media (max-width: 760px) {
  .unplanned.shut {
    width: auto;
    flex-direction: row;
    padding: 6px 12px;
  }
  .up-shut-wrap {
    justify-content: flex-start;
  }
  .up-tray-btn {
    width: auto;
    height: auto;
    padding: 4px 8px;
    gap: 10px;
  }
  .up-tray-lbl {
    display: inline;
    font: 500 13px/1 var(--font-ui);
    color: var(--tx-primary);
  }
  .unplanned,
  .unplanned.wide:not(.shut) {
    max-height: none;
  }
}
</style>
