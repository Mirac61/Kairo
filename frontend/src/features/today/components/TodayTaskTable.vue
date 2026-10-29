<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Project, Task } from '@/api/client'
import type { Block } from '@/composables/useHome'
import { locale, t } from '@/lib/i18n'
import { projectIconOf } from '@/lib/projectIcons'

const props = defineProps<{
  tasks: Task[]
  doneToday: Task[]
  sel: string | null
  overdueTasks: Task[]
  todaysTasks: Task[]
  projectNames: Map<string, string>
  projectsById: Map<string, Project>
  blocks: Block[]
  todayDate?: string
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'setDone', task: Task): void
  (e: 'toggleTimer', task: Task): void
  (e: 'togglePlan', task: Task): void
  (e: 'reopen', task: Task): void
}>()

const showDone = ref(false)

const overdueIds = computed(() => new Set(props.overdueTasks.map((x) => x.id)))
const dayOf = (x: Task) => x.planned_date || x.due_at?.slice(0, 10) || null

function isTaskOverdue(x: Task): boolean {
  if (overdueIds.value.has(x.id)) return true
  const d = dayOf(x)
  return Boolean(d && props.todayDate && d < props.todayDate)
}

function shortDay(d: string | null) {
  return d
    ? new Intl.DateTimeFormat(locale.value, {
        weekday: 'short',
        day: 'numeric',
        month: 'numeric',
      }).format(new Date(`${d}T12:00:00`))
    : ''
}

function taskDueInfo(x: Task): {
  text: string
  isOverdue: boolean
  isToday: boolean
} {
  const d = dayOf(x)
  if (isTaskOverdue(x)) {
    return {
      text: d ? shortDay(d) : t('Überfällig'),
      isOverdue: true,
      isToday: false,
    }
  }
  const start = startOf(x)
  if (start !== undefined) {
    return {
      text: t('Heute {time}', { time: formatTime(start) }),
      isOverdue: false,
      isToday: true,
    }
  }
  if (d === props.todayDate || props.todaysTasks.some((t) => t.id === x.id)) {
    return {
      text: t('Heute'),
      isOverdue: false,
      isToday: true,
    }
  }
  if (d) {
    return {
      text: shortDay(d),
      isOverdue: false,
      isToday: false,
    }
  }
  return {
    text: '',
    isOverdue: false,
    isToday: false,
  }
}

function compactDur(mins = 0): string {
  if (!mins) return ''
  const h = Math.floor(mins / 60)
  const m = mins % 60
  if (h && m) return `${h}h ${m}m`
  if (h) return `${h}h`
  return `${m}m`
}

function formatTime(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

const startOf = (x: Task) => props.blocks.find((b) => b.task?.id === x.id)?.start

const projectIcon = (id: string | null) => {
  if (!id) return 'i-proj'
  const p = props.projectsById.get(id)
  return projectIconOf(p ?? { name: props.projectNames.get(id) ?? '' })
}
</script>

<template>
  <div class="st-table-wrap">
    <div v-if="tasks.length || doneToday.length" class="st-table-card">
      <div class="st-tbl-head" aria-hidden="true">
        <span class="st-th-cb"></span>
        <span class="st-c-t">{{ $t('Aufgabe') }}</span>
        <span class="st-c-p">{{ $t('Projekt') }}</span>
        <span class="st-c-d">{{ $t('Dauer') }}</span>
        <span class="st-c-f">{{ $t('Fällig') }}</span>
      </div>

      <div class="st-tbody">
        <div
          v-for="x in tasks"
          :key="x.id"
          class="st-row"
          :class="{ sel: sel === x.id }"
          @click="emit('select', x.id)"
        >
          <button
            type="button"
            class="cb ring"
            role="checkbox"
            aria-checked="false"
            :aria-label="$t('{title} erledigt', { title: x.title })"
            @click.stop="emit('setDone', x)"
          ></button>
          <div class="st-row-t">
            <span class="t">{{ x.title }}</span>
          </div>
          <div class="st-col-p">
            <span v-if="x.project_id" class="st-proj-item" :title="projectNames.get(x.project_id)">
              <svg class="ic st-proj-ic" aria-hidden="true">
                <use :href="`#${projectIcon(x.project_id)}`" />
              </svg>
              <span class="st-proj-name">{{ projectNames.get(x.project_id) }}</span>
            </span>
          </div>
          <div class="st-col-d">
            <span v-if="compactDur(x.estimated_minutes)" class="st-dur-val">
              {{ compactDur(x.estimated_minutes) }}
            </span>
          </div>
          <div class="st-col-f">
            <span
              v-if="taskDueInfo(x).text"
              class="st-due-val"
              :class="{
                overdue: taskDueInfo(x).isOverdue,
                today: taskDueInfo(x).isToday,
              }"
            >
              {{ taskDueInfo(x).text }}
            </span>
          </div>
          <div class="st-row-acts">
            <button type="button" class="btn btn-secondary" @click.stop="emit('toggleTimer', x)">
              {{ $t('Starten') }}
            </button>
            <button type="button" class="btn btn-secondary" @click.stop="emit('togglePlan', x)">
              {{ x.planned_start_at ? $t('Aus dem Plan') : $t('Einplanen') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Erledigte Aufgaben (einklappbar) -->
      <div v-if="doneToday.length" class="st-done-section">
        <button
          type="button"
          class="st-done-btn"
          :aria-expanded="showDone"
          @click="showDone = !showDone"
        >
          <svg class="ic" :class="{ open: showDone }" viewBox="0 0 12 12" aria-hidden="true">
            <path d="M4.5 3 7.5 6 4.5 9" />
          </svg>
          <span>{{ $t('Erledigt') }}</span>
          <span class="count">{{ doneToday.length }}</span>
        </button>
        <div v-if="showDone" class="st-tbody st-done-body">
          <div v-for="x in doneToday" :key="x.id" class="st-row done">
            <button
              type="button"
              class="cb ring"
              role="checkbox"
              aria-checked="true"
              :aria-label="$t('Wieder öffnen')"
              :title="$t('Wieder öffnen')"
              @click="emit('reopen', x)"
            ></button>
            <div class="st-row-t">
              <span class="t">{{ x.title }}</span>
            </div>
            <div class="st-col-p">
              <span v-if="x.project_id" class="st-proj-item">
                <svg class="ic st-proj-ic" aria-hidden="true">
                  <use :href="`#${projectIcon(x.project_id)}`" />
                </svg>
                <span class="st-proj-name">{{ projectNames.get(x.project_id) }}</span>
              </span>
            </div>
            <span class="st-col-d"></span>
            <div class="st-col-f"></div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="!tasks.length && !doneToday.length" class="st-empty">
      {{ $t('Nichts für heute geplant.') }}
    </div>
  </div>
</template>

<style scoped>
.st-table-wrap {
  --st-cols: 20px minmax(0, 1fr) minmax(0, 140px) 52px 96px;
  margin-top: 32px;
}
/* Listenansicht ohne Kasten, wie die Aufgabenliste */
.st-table-card {
  font-variant-numeric: tabular-nums;
}
.st-tbl-head {
  display: grid;
  grid-template-columns: var(--st-cols);
  align-items: center;
  gap: 12px;
  height: 34px;
  padding: 0 12px;
  border-bottom: 1px solid var(--br-default);
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
  user-select: none;
}
.st-th-cb {
  width: 20px;
}
.st-tbl-head .st-c-t {
  min-width: 0;
}
.st-tbl-head .st-c-p {
  min-width: 0;
}
.st-tbl-head .st-c-d {
  text-align: right;
  white-space: nowrap;
}
.st-tbl-head .st-c-f {
  text-align: right;
}

.st-tbody {
  display: flex;
  flex-direction: column;
}
.st-row {
  position: relative;
  display: grid;
  grid-template-columns: var(--st-cols);
  align-items: center;
  gap: 12px;
  min-height: 46px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--br-subtle);
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  cursor: pointer;
  background: transparent;
  transition:
    background var(--dur) ease,
    box-shadow var(--dur) ease;
}
.st-tbody .st-row:last-child {
  border-bottom: none;
}
.st-row:hover {
  background: var(--bg-hover);
}
.st-row.sel {
  background: var(--bg-2);
  box-shadow: inset 2px 0 0 var(--a-blue);
}
.st-row.done {
  opacity: 0.55;
}
.st-row.done .t {
  text-decoration: line-through;
  color: var(--tx-muted);
}
.st-row .cb.ring {
  width: 18px;
  height: 18px;
  margin: 0;
  justify-self: center;
  border: 1.5px solid var(--br-strong);
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  cursor: pointer;
  transition:
    border-color var(--dur) ease,
    background var(--dur) ease;
}
.st-row:hover .cb.ring {
  border-color: var(--tx-muted);
}
.st-row .cb.ring:hover {
  border-color: var(--tx-secondary);
}
.st-row-t {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.st-row-t .t {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
  color: var(--tx-primary);
  letter-spacing: -0.005em;
}
.st-col-p {
  min-width: 0;
  overflow: hidden;
}
.st-proj-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  color: var(--tx-secondary);
  font-size: 12.5px;
  transition: color var(--dur) ease;
}
.st-row:hover .st-proj-item {
  color: var(--tx-primary);
}
.st-proj-ic {
  width: 14px;
  height: 14px;
  flex: none;
  color: var(--tx-muted);
  transition: color var(--dur) ease;
}
.st-row:hover .st-proj-ic {
  color: var(--tx-secondary);
}
.st-proj-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-col-d {
  text-align: right;
  white-space: nowrap;
}
.st-dur-val {
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.st-col-f {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.st-due-val {
  font-size: 12.5px;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.st-due-val.today {
  color: var(--tx-secondary);
}
.st-due-val.overdue {
  color: var(--a-red);
  font-weight: 500;
}
.st-row-acts {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  opacity: 0;
  pointer-events: none;
  background: var(--bg-hover);
  padding: 4px 6px 4px 16px;
  border-radius: var(--r-s);
  box-shadow: -16px 0 16px var(--bg-hover);
  transition: opacity var(--dur) ease;
}
.st-row:hover .st-row-acts {
  opacity: 1;
  pointer-events: auto;
}
.st-row.sel .st-row-acts {
  background: var(--bg-2);
  box-shadow: -16px 0 16px var(--bg-2);
}
.st-row-acts .btn {
  height: 26px;
  padding: 0 9px;
  font-size: 11.5px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border-radius: 4px;
  background: var(--bg-2);
  border: 1px solid var(--br-default);
  color: var(--tx-secondary);
  transition: all var(--dur) ease;
}
.st-row-acts .btn:hover {
  background: var(--bg-3);
  border-color: var(--br-strong);
  color: var(--tx-primary);
}
.st-done-section {
  border-top: 1px solid var(--br-default);
}
.st-done-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 38px;
  padding: 0 12px;
  background: transparent;
  border: 0;
  cursor: pointer;
  color: var(--tx-muted);
  font: 500 12.5px/1 var(--font-ui);
  transition:
    background var(--dur) ease,
    color var(--dur) ease;
}
.st-done-btn:hover {
  background: var(--bg-hover);
  color: var(--tx-secondary);
}
.st-done-btn .ic {
  width: 10px;
  height: 10px;
  fill: currentColor;
  transition: transform var(--dur) ease;
}
.st-done-btn .ic.open {
  transform: rotate(90deg);
}
.st-done-btn .count {
  font-size: 12px;
  font-weight: 400;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.st-done-body {
  border-top: 1px solid var(--br-subtle);
}
.st-empty {
  padding: 24px 0;
  text-align: center;
  color: var(--tx-muted);
  font: 400 13px/1.4 var(--font-ui);
}

@media (max-width: 680px) {
  .st-table-wrap {
    --st-cols: 20px minmax(0, 1fr) auto;
  }
  .st-tbl-head .st-c-p,
  .st-tbl-head .st-c-d,
  .st-col-p,
  .st-col-d {
    display: none;
  }
}
</style>
