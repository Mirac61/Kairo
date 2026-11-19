<script setup lang="ts">
import { computed } from 'vue'
import { isOpen, type Task } from '@/api/client'
import { hm, ruleText } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'

const props = defineProps<{
  task: Task
  isSelected?: boolean
  isPicked?: boolean
  isFresh?: boolean
  isOpenDetail?: boolean
  projectName?: string
  subtasksProgress?: string
  isUrgent?: boolean
  isOverdue?: boolean
  whenText?: string
  whenTip?: string
}>()

const emit = defineEmits<{
  (e: 'toggleStatus', task: Task): void
  (e: 'openDetail', taskId: string): void
  (e: 'pick', event: MouseEvent, taskId: string): void
  (e: 'pointerDown', event: PointerEvent, taskId: string): void
  (e: 'pointerUp'): void
}>()

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: 'Backlog',
  PLANNED: 'Geplant',
  IN_PROGRESS: 'Läuft',
  PAUSED: 'Pausiert',
  COMPLETED: 'Erledigt',
  CANCELLED: 'Abgebrochen',
}

const PRIO_LABEL: Record<string, string> = {
  URGENT: 'Dringend',
  HIGH: 'Hoch',
  MEDIUM: 'Mittel',
  LOW: 'Niedrig',
}

const statusLabel = computed(() => t(STATUS_LABEL[props.task.status] ?? props.task.status))
const prioLabel = computed(() => t(PRIO_LABEL[props.task.priority] ?? props.task.priority))

const prioLevel = computed(() => {
  return ({ LOW: 1, MEDIUM: 2, HIGH: 3, URGENT: 3 } as const)[props.task.priority as 'LOW'] ?? 2
})
</script>

<template>
  <div
    class="task-row"
    :class="{
      done: !isOpen(task),
      running: task.status === 'IN_PROGRESS',
      selected: isSelected,
      picked: isPicked,
      fresh: isFresh,
    }"
    :style="task.project_id ? { '--pdot-c': projectColor(task.project_id) } : undefined"
    :data-task="task.id"
    @click.capture="emit('pick', $event, task.id)"
    @mousedown.shift.prevent
    @pointerdown="emit('pointerDown', $event, task.id)"
    @pointerup="emit('pointerUp')"
    @pointercancel="emit('pointerUp')"
  >
    <button
      type="button"
      class="cb sq"
      role="checkbox"
      :aria-checked="task.status === 'COMPLETED'"
      :aria-label="$t('{title} erledigt', { title: task.title })"
      :disabled="task.status === 'CANCELLED'"
      @click="emit('toggleStatus', task)"
    ></button>

    <span v-if="task.project_id" class="pdot" :title="projectName" :aria-label="projectName"></span>

    <button
      type="button"
      class="t row-title"
      :aria-expanded="isOpenDetail"
      @click="emit('openDetail', task.id)"
    >
      {{ task.title }}
    </button>

    <span v-if="task.status === 'IN_PROGRESS'" class="sr-only">{{ statusLabel }}</span>

    <span
      v-if="task.recurrence_rule"
      class="rec-i"
      role="img"
      :aria-label="$t('Wiederholt sich: {rule}', { rule: ruleText(task.recurrence_rule) })"
      :data-tip="ruleText(task.recurrence_rule)"
      >↻</span
    >

    <span
      v-if="subtasksProgress"
      class="badge mono"
      role="img"
      :aria-label="`${$t('Teilaufgaben erledigt')}: ${subtasksProgress}`"
    >
      {{ subtasksProgress }}
    </span>

    <span v-if="task.status === 'PAUSED'" class="status-l">
      <span class="cdot"></span>{{ statusLabel }}
    </span>

    <span class="pcol">
      <span v-if="task.project_id" class="chip">
        {{ projectName }}
      </span>
    </span>

    <span class="dur">
      {{ task.estimated_minutes ? hm(task.estimated_minutes) : '' }}
    </span>

    <span
      class="prio-i"
      :class="{ high: isUrgent, quiet: prioLevel < 3 }"
      :data-prio="task.priority"
      role="img"
      :aria-label="`${$t('Priorität')}: ${prioLabel}`"
    >
      <i v-for="n in 3" :key="n" :class="{ on: n <= prioLevel }"></i>
    </span>

    <span class="due" :class="{ od: isOverdue }" :data-tip="whenTip || undefined">
      {{ whenText }}
    </span>
  </div>
</template>

<style scoped>
.task-row.picked {
  background: var(--bg-selected);
}
.task-row.selected {
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-default);
}
.task-row:hover:not(.selected) {
  background: var(--bg-hover);
}
.row-title {
  text-align: left;
  background: none;
  border: 0;
  color: inherit;
  cursor: pointer;
}
.task-row.running .row-title {
  color: var(--a-run);
  font-weight: 500;
}
.task-row.fresh {
  background: var(--bg-selected);
  transition: background 1s;
}
</style>
