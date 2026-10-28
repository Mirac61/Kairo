<script setup lang="ts">
import { ref } from 'vue'
import { isOpen, type Project, type Resource, type Task } from '@/api/client'
import { hhmm, ruleText, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'
import { RULE_PRESETS } from '@/lib/quickAdd'
import ResourceList from '@/components/ResourceList.vue'
import TaskActions from '@/components/TaskActions.vue'

const props = defineProps<{
  task: Task
  projects: Project[]
  resources: Resource[]
  subTasks: Task[]
  progressText: string
  editSlotHint?: { time?: string; text: string } | null
  todayStr: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updateField', field: 'title' | 'description' | 'priority' | 'project_id' | 'status' | 'recurrence_rule', value: string): void
  (e: 'setDate', date: string): void
  (e: 'setTime', time: string): void
  (e: 'setDue', day: string): void
  (e: 'setEstimate', minutes: string): void
  (e: 'toggleSubtask', subtask: Task): void
  (e: 'addSubtask', title: string): void
  (e: 'trash'): void
  (e: 'takeEditSlot', time: string): void
  (e: 'run', action: () => Promise<unknown>): void
  (e: 'reload'): void
}>()

const subTitleInput = ref('')

const STATUS_LABELS: Record<Task['status'], string> = {
  BACKLOG: t('Backlog'),
  PLANNED: t('Geplant'),
  IN_PROGRESS: t('Läuft'),
  PAUSED: t('Pausiert'),
  COMPLETED: t('Erledigt'),
  CANCELLED: t('Abgebrochen'),
}

const SETTABLE_STATUSES: Task['status'][] = ['BACKLOG', 'PLANNED', 'CANCELLED']

const PRIORITY_LABELS: Record<string, string> = {
  URGENT: t('Dringend'),
  HIGH: t('Hoch'),
  MEDIUM: t('Mittel'),
  LOW: t('Niedrig'),
}

const WEEKDAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']

function getStartTime(task: Task): string {
  return task.planned_start_at ? hhmm(new Date(task.planned_start_at)) : ''
}

function getDueDay(task: Task): string {
  return task.due_at ? ymd(new Date(task.due_at)) : ''
}

function computeRuleOptions(task: Task): string[] {
  const options = [...RULE_PRESETS.map(([rule]) => rule)]

  if (task.planned_date) {
    const dayOfWeek = new Date(`${task.planned_date}T12:00:00`).getDay()
    const code = WEEKDAY_CODES[dayOfWeek]
    options.push(`FREQ=WEEKLY;BYDAY=${code}`)
  }

  if (task.recurrence_rule) {
    options.push(task.recurrence_rule)
  }

  return [...new Set(options)]
}

function onValueChange(
  field: 'title' | 'description' | 'priority' | 'project_id' | 'status' | 'recurrence_rule',
  event: Event,
) {
  const target = event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
  emit('updateField', field, target.value)
}

function handleAddSubtask() {
  const trimmed = subTitleInput.value.trim()
  if (!trimmed) return
  emit('addSubtask', trimmed)
  subTitleInput.value = ''
}

function noteTarget(): string | undefined {
  return props.resources.find((r) => r.type === 'NOTE')?.target
}
</script>

<template>
  <aside class="task-aside" :aria-label="$t('Aufgabendetails')">
    <div class="aside-head">
      <span class="lbl">{{ $t('Aufgabe') }}</span>
      <button
        class="icon-btn"
        type="button"
        :aria-label="$t('Details schließen')"
        @click="emit('close')"
      >
        <svg class="ic"><use href="#i-x" /></svg>
      </button>
    </div>

    <div class="task-detail">
      <input
        class="td-title"
        :value="task.title"
        :aria-label="$t('Titel')"
        @change="onValueChange('title', $event)"
      />

      <div class="td-meta">
        <select
          class="input td-pill"
          :value="task.project_id ?? ''"
          :aria-label="$t('Projekt')"
          :style="{ '--chip-c': projectColor(task.project_id) }"
          @change="onValueChange('project_id', $event)"
        >
          <option value="">{{ $t('Kein Projekt') }}</option>
          <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>

        <select
          class="input td-pill plain"
          :value="task.status"
          :aria-label="$t('Status')"
          @change="onValueChange('status', $event)"
        >
          <option v-for="st in SETTABLE_STATUSES" :key="st" :value="st">
            {{ STATUS_LABELS[st] }}
          </option>
          <option v-if="!SETTABLE_STATUSES.includes(task.status)" :value="task.status" disabled>
            {{ STATUS_LABELS[task.status] }}
          </option>
        </select>
      </div>

      <TaskActions
        :task="task"
        :running="task.status === 'IN_PROGRESS'"
        :note="noteTarget()"
        @run="(fn) => emit('run', fn)"
      />

      <div class="td-props">
        <label class="td-row">
          <span>{{ $t('Fällig') }}</span>
          <input
            class="input"
            type="date"
            :value="getDueDay(task)"
            @change="(e) => emit('setDue', (e.target as HTMLInputElement).value)"
          />
        </label>

        <template v-if="isOpen(task)">
          <div class="td-row">
            <span>{{ $t('Geplant') }}</span>
            <input
              class="input"
              type="date"
              :value="task.planned_date ?? ''"
              :aria-label="$t('Datum')"
              @change="(e) => emit('setDate', (e.target as HTMLInputElement).value)"
            />
            <input
              class="input"
              type="time"
              :value="getStartTime(task)"
              :disabled="!task.planned_date"
              :aria-label="$t('Uhrzeit')"
              @change="(e) => emit('setTime', (e.target as HTMLInputElement).value)"
            />
          </div>

          <label class="td-row">
            <span>{{ $t('Dauer') }}</span>
            <input
              class="input"
              type="number"
              min="0"
              :value="task.estimated_minutes"
              :aria-label="$t('Minuten')"
              @change="(e) => emit('setEstimate', (e.target as HTMLInputElement).value)"
            />
            <em>min</em>
          </label>
        </template>

        <label class="td-row">
          <span>{{ $t('Priorität') }}</span>
          <select
            class="input"
            :value="task.priority"
            @change="onValueChange('priority', $event)"
          >
            <option v-for="(label, key) in PRIORITY_LABELS" :key="key" :value="key">
              {{ label }}
            </option>
          </select>
        </label>

        <label v-if="isOpen(task)" class="td-row">
          <span>{{ $t('Wiederholung') }}</span>
          <select
            class="input"
            :value="task.recurrence_rule ?? ''"
            @change="onValueChange('recurrence_rule', $event)"
          >
            <option value="">{{ $t('Nicht wiederholen') }}</option>
            <option v-for="r in computeRuleOptions(task)" :key="r" :value="r">
              {{ ruleText(r) }}
            </option>
          </select>
        </label>
      </div>

      <div v-if="isOpen(task)" class="td-quick">
        <button
          v-if="editSlotHint?.time && !task.planned_start_at"
          type="button"
          class="btn btn-secondary"
          @click="emit('takeEditSlot', editSlotHint.time)"
        >
          {{ editSlotHint.text }}
        </button>
        <span
          v-else-if="editSlotHint && !task.planned_start_at"
          class="qa-hint"
        >
          {{ editSlotHint.text }}
        </span>
        <button
          v-if="task.planned_date !== todayStr"
          type="button"
          class="btn btn-secondary"
          @click="emit('setDate', todayStr)"
        >
          {{ $t('Heute') }}
        </button>
      </div>

      <label class="td-sec">
        <span class="lbl">{{ $t('Beschreibung') }}</span>
        <textarea
          class="td-desc"
          rows="2"
          :placeholder="$t('Beschreibung hinzufügen')"
          :value="task.description"
          @change="onValueChange('description', $event)"
        ></textarea>
      </label>

      <div class="td-sec sub-sec">
        <span class="lbl">
          {{ $t('Teilaufgaben') }}
          <template v-if="progressText"> · {{ progressText }}</template>
        </span>
        <div
          v-for="sub in subTasks"
          :key="sub.id"
          class="sub-row"
          :class="{ done: !isOpen(sub) }"
        >
          <button
            type="button"
            class="cb"
            role="checkbox"
            :aria-checked="sub.status === 'COMPLETED'"
            :aria-label="$t('{title} erledigt', { title: sub.title })"
            :disabled="sub.status === 'CANCELLED'"
            @click="emit('toggleSubtask', sub)"
          ></button>
          <span>{{ sub.title }}</span>
        </div>

        <form class="addrow" @submit.prevent="handleAddSubtask">
          <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
          <input
            v-model="subTitleInput"
            type="text"
            :placeholder="$t('Teilaufgabe hinzufügen')"
            :aria-label="$t('Teilaufgabe hinzufügen')"
          />
        </form>
      </div>

      <ResourceList
        class="td-sec"
        :resources="resources"
        :task-id="task.id"
        @changed="emit('reload')"
      />

      <div class="td-foot">
        <span></span>
        <button type="button" class="btn btn-ghost" @click="emit('trash')">
          {{ $t('Löschen') }}
        </button>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.task-aside {
  display: flex;
  flex-direction: column;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  overflow-y: auto;
  scrollbar-width: none;
}
.task-aside::-webkit-scrollbar {
  display: none;
}
.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 0 24px;
}
.task-detail {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 12px 24px 18px;
}
.td-title {
  width: 100%;
  padding: 4px 0;
  border: 0;
  background: none;
  color: var(--tx-primary);
  font: 600 20px/1.3 var(--font-ui);
  letter-spacing: -.015em;
}
.td-title:focus-visible {
  outline: none;
  box-shadow: 0 1px 0 var(--a-focus);
}
.td-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 0 18px;
}
.td-pill {
  height: 24px;
  width: auto;
  max-width: 180px;
  padding: 0 22px 0 9px;
  border-radius: 7px;
  font: 500 12px/1 var(--font-ui);
  cursor: pointer;
  background-color: color-mix(in srgb, var(--chip-c, var(--tx-muted)) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--chip-c, var(--tx-muted)) 24%, transparent);
  color: color-mix(in srgb, var(--chip-c, var(--tx-muted)) 70%, var(--tx-primary));
}
.td-pill.plain {
  padding-left: 0;
  border-color: transparent;
  background-color: transparent;
  color: var(--tx-secondary);
  font-weight: 400;
}
.td-props {
  margin-top: 18px;
  border-top: 1px solid var(--br-subtle);
}
.td-row {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  border-bottom: 1px solid var(--br-subtle);
  font: 400 13px/1 var(--font-ui);
}
.td-row > span {
  flex: none;
  width: 104px;
  color: var(--tx-muted);
}
.td-row > em {
  font-style: normal;
  color: var(--tx-muted);
  font-size: 12px;
}
.td-row .input {
  flex: 1;
  min-width: 0;
  width: auto;
  height: 28px;
  padding: 0 6px;
  border: 0;
  background-color: transparent;
  color: var(--tx-primary);
  font: 400 13px/1 var(--font-ui);
  border-radius: 6px;
}
.td-row input[type="number"] {
  flex: none;
  width: 56px;
  font-variant-numeric: tabular-nums;
}
.td-row .input:hover {
  background-color: var(--bg-2);
}
.td-quick {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}
.td-quick:not(:has(*)) {
  display: none;
}
.td-sec {
  display: block;
  margin-top: 18px;
}
.td-sec > .lbl,
.sub-sec > .lbl {
  display: block;
  margin-bottom: 8px;
  font: 600 11px/1 var(--font-ui);
  letter-spacing: .08em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.td-desc {
  width: 100%;
  padding: 6px 8px;
  margin: 0 -8px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--tx-secondary);
  font: 400 13px/1.55 var(--font-ui);
  resize: vertical;
}
.td-desc:hover,
.td-desc:focus-visible {
  background: var(--bg-2);
}
.td-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.sub-sec {
  display: grid;
  gap: 8px;
}
.sub-sec .addrow {
  margin-top: 0;
}
.sub-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font: 400 14px/1.4 var(--font-ui);
}
.sub-row.done span {
  text-decoration: line-through;
  color: var(--tx-muted);
}

@media (min-width: 1001px) {
  .task-aside {
    position: sticky;
    top: 56px;
    align-self: start;
    height: calc(100dvh - 56px);
  }
}

@media (max-width: 1000px) {
  .task-aside {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 20;
    max-height: 65vh;
    border: 0;
    border-radius: var(--r-m) var(--r-m) 0 0;
    box-shadow: var(--shadow-pop);
  }
}

@media (max-width: 760px) {
  .task-aside {
    bottom: calc(56px + env(safe-area-inset-bottom));
    z-index: 55;
    max-height: calc(100dvh - 56px - env(safe-area-inset-bottom) - 48px);
  }
}
</style>
