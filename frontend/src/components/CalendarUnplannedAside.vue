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
</script>

<template>
  <aside
    class="unplanned"
    :class="{ shut: !isOpen, wide: isDay }"
    aria-labelledby="up-title"
  >
    <!-- Als Nächstes (nur in der Tagesansicht) -->
    <section v-if="isDay && nextUp && isOpen" class="next" :aria-label="$t('Als Nächstes')">
      <div class="next-head">
        <span>{{ $t('Als Nächstes') }} · {{ $t('in {time}', { time: formatInMinutes(nextUp.start) }) }}</span>
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
        <button
          type="button"
          class="btn btn-ghost"
          @click="emit('postponeNextUp', nextUp.task)"
        >
          {{ $t('Verschieben') }}
        </button>
      </div>
    </section>

    <div class="up-head">
      <h2 id="up-title">
        <span class="up-lbl">{{ $t('Ungeplant') }} </span>
        <span class="up-count">{{ tasks.length }}</span>
      </h2>
      <span v-if="isDay && isOpen" class="up-hint">{{ $t('in den Plan ziehen') }}</span>
      <button
        type="button"
        class="icon-btn"
        :aria-expanded="isOpen"
        :aria-label="isOpen ? 'Ungeplant einklappen' : 'Ungeplant ausklappen'"
        @click="emit('toggleOpen')"
      >
        <svg class="ic"><use :href="isOpen ? '#i-right' : '#i-left'" /></svg>
      </button>
    </div>

    <div v-show="isOpen" ref="unplannedEl" class="up-list">
      <div
        v-for="t in tasks"
        :key="t.id"
        class="up-item"
        :class="{ picked: pickedIds.includes(t.id) }"
        :data-id="t.id"
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
  width: 240px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px;
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  border-radius: var(--r-m);
}
.up-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.unplanned.wide:not(.shut) {
  width: 300px;
}
.unplanned.shut {
  width: 44px;
  padding: 12px 6px;
  align-items: center;
}
.unplanned.shut .up-head {
  flex-direction: column;
}
.unplanned.shut .up-lbl {
  display: none;
}
.unplanned h2,
.unplanned h3 {
  margin: 0;
  font: 600 14px/1.3 var(--font-ui);
  color: var(--tx-primary);
}
.up-count {
  margin-left: 6px;
  font: 400 12px/1 var(--font-mono);
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
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  text-align: right;
}
.up-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  cursor: grab;
  border: 1px dashed var(--br-strong);
  border-radius: 8px;
  font: 400 13px/1.3 var(--font-ui);
  color: var(--tx-primary);
}
.up-item:hover {
  background: var(--bg-hover);
}
.up-item.picked {
  background: var(--bg-selected);
  box-shadow: inset 0 0 0 1px var(--a-blue-hi);
}
.up-dot {
  flex: none;
  width: 11px;
  height: 11px;
  box-sizing: border-box;
  border: 1.5px solid var(--acc);
  border-radius: 50%;
}
.up-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.up-min {
  flex: none;
  font: 400 12px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-secondary);
}
.next {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  margin-bottom: 16px;
  background: var(--bg-0);
  border: 1px solid var(--br-subtle);
  border-radius: 12px;
}
.next-head {
  display: flex;
  justify-content: space-between;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.next h3 {
  margin: 0;
  font: 600 17px/1.3 var(--font-ui);
  color: var(--tx-primary);
  text-wrap: pretty;
}
.next-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 12px;
  margin-top: -6px;
  font: 400 13px/1.3 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-secondary);
}
.next-meta span {
  display: flex;
  align-items: center;
  gap: 6px;
}
.next-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

@media (max-width: 900px) {
  .unplanned,
  .unplanned.wide:not(.shut) {
    width: auto;
    max-height: 180px;
    order: 2;
  }
}

@media (max-width: 760px) {
  .unplanned.shut {
    width: auto;
    flex-direction: row;
    padding: 4px 8px 4px 16px;
  }
  .unplanned.shut .up-head {
    flex-direction: row;
    flex: 1;
  }
  .unplanned.shut .up-lbl {
    display: inline;
  }
  .unplanned.shut .up-count {
    margin-left: 6px;
  }
  .unplanned,
  .unplanned.wide:not(.shut) {
    max-height: none;
  }
}
</style>
