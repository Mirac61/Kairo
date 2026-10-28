<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createTask,
  deleteTask,
  isOpen,
  listProjects,
  listResources,
  listTasks,
  taskAction,
  type Project,
  type Resource,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useTaskFilter } from '@/composables/useTaskFilter'
import { useTaskSelection } from '@/composables/useTaskSelection'
import { useUndo } from '@/composables/useUndo'
import { useFreeSlot } from '@/composables/useFreeSlot'
import { addDays, hm, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'
import SelectionBar from '@/components/SelectionBar.vue'
import TaskEditSheet from './components/TaskEditSheet.vue'
import TaskQuickAdd from './components/TaskQuickAdd.vue'
import TaskRow from './components/TaskRow.vue'
import TasksToolbar from './components/TasksToolbar.vue'
import TasksEmptyState from './components/TasksEmptyState.vue'
import TasksKeyboardHelp from './components/TasksKeyboardHelp.vue'
import TaskGithubDialog from './components/TaskGithubDialog.vue'
import { useTaskOperations } from './composables/useTaskOperations'

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])

const github = ref(false)
const openId = ref<string | null>(null)
const sel = ref<string | null>(null) // Fokus/Auswahl für Tastenkürzel

const quickAddEl = ref<InstanceType<typeof TaskQuickAdd>>()

const route = useRoute()
const router = useRouter()
const { offer, setDone } = useUndo()

// Filter- und Gruppierungslogik über Composable
const {
  filter,
  projectId,
  search,
  view,
  SORTS,
  GROUPS,
  MAIN_FILTER_IDS,
  projectNameMap,
  todayString,
  dayOf,
  inScope,
  matches,
  counts,
  tabs,
  extra,
  rows,
  groups,
  flat,
} = useTaskFilter(tasks, projects)

const { error, load, run } = useLoader(async () => {
  ;[tasks.value, projects.value, resources.value] = await Promise.all([
    listTasks(),
    listProjects(),
    listResources(),
  ])
})

const selection = useTaskSelection(
  () => flat.value,
  run,
  () => ymd(addDays(new Date(), 1)),
)
const { picked, onClick: pickRow, onPointerDown, onPointerUp } = selection

// Wenn die aktuell ausgewählte Aufgabe verschwindet, rückt die Auswahl auf den Nachbarn
watch(flat, (now, before) => {
  if (!sel.value || now.some((t) => t.id === sel.value)) return
  const prevIndex = before.findIndex((t) => t.id === sel.value)
  const targetIndex = Math.min(prevIndex, now.length - 1)
  sel.value = now[targetIndex]?.id ?? null
})

const openTask = computed(() => tasks.value.find((t) => t.id === openId.value))

function prioLevel(task: Task): number {
  return ({ LOW: 1, MEDIUM: 2, HIGH: 3, URGENT: 3 } as const)[task.priority as 'LOW'] ?? 2
}

function overdue(task: Task): boolean {
  const d = dayOf(task)
  return isOpen(task) && d !== null && d < todayString()
}

// Rot nur, wenn hohe Priorität auch heute fällig oder überfällig ist
function urgent(task: Task): boolean {
  return prioLevel(task) === 3 && (matches(task, 'heute') || matches(task, 'ueber'))
}

function kidsOf(id: string): Task[] {
  return tasks.value.filter((t) => t.parent_task_id === id)
}

function progress(id: string): string {
  const children = kidsOf(id).filter((c) => c.status !== 'CANCELLED')
  if (!children.length) return ''
  const completed = children.filter((c) => c.status === 'COMPLETED').length
  return `${completed}/${children.length}`
}

function resOf(id: string): Resource[] {
  return resources.value.filter((r) => r.task_id === id)
}

const stats = computed(() => {
  const openTasks = tasks.value.filter(isOpen)
  const totalMinutes = openTasks.reduce((sum, task) => sum + task.estimated_minutes, 0)
  const overdueCount = openTasks.filter(overdue).length

  return [
    t('{n} offen', { n: openTasks.length }),
    t('{n} überfällig', { n: overdueCount }),
    totalMinutes ? t('{time} geschätzt', { time: hm(totalMinutes) }) : '',
  ]
    .filter(Boolean)
    .join(' · ')
})

const defaultProject = () => (projectId.value === 'alle' ? null : projectId.value)

const freshCreatedIds = ref<string[]>([])

async function handleTasksCreated(list: Task[], notificationText: string) {
  await load()
  const first = list[0]!
  freshCreatedIds.value = list.map((task) => task.id)
  setTimeout(() => {
    freshCreatedIds.value = []
  }, 1800)

  const isFirstVisible = inScope(first) && matches(first, filter.value)

  offer(
    notificationText,
    () =>
      run(async () => {
        for (const task of list) {
          await deleteTask(task.id, true)
        }
      }),
    isFirstVisible ? undefined : () => focusTask(first.id),
  )
}

const editSlotDay = computed(() => {
  const t = openTask.value
  return t && t.planned_date && !t.planned_start_at ? t.planned_date : null
})

const currentMinuteOfDay = () => {
  const d = new Date()
  return d.getHours() * 60 + d.getMinutes()
}

const { hint: editSlotHint } = useFreeSlot(
  () => editSlotDay.value,
  () => openTask.value?.estimated_minutes ?? 0,
  todayString,
  currentMinuteOfDay,
)

function takeEditSlot(time: string) {
  if (openTask.value) {
    setTime(openTask.value, time)
  }
}

const {
  formatTaskWhen,
  setDate,
  setTime,
  setEstimate,
  setField,
  setDue,
  toggleTaskStatus,
  trashTask,
} = useTaskOperations(run, offer, setDone, todayString, dayOf, overdue, openId)

function moveSelection(direction: number) {
  const list = flat.value
  if (!list.length) return

  const currentIndex = list.findIndex((t) => t.id === sel.value)
  const nextIndex =
    currentIndex < 0
      ? direction > 0
        ? 0
        : list.length - 1
      : Math.min(Math.max(currentIndex + direction, 0), list.length - 1)

  sel.value = list[nextIndex]!.id
  if (openId.value) {
    openId.value = sel.value
  }

  void nextTick(() => {
    document.querySelector(`[data-task="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' })
  })
}

useShortcuts((event) => {
  const targetEl = event.target as HTMLElement
  const currentTask = flat.value.find((x) => x.id === sel.value)

  if (event.key === 'n') {
    event.preventDefault()
    quickAddEl.value?.focus()
  } else if (event.key === 'j') {
    moveSelection(1)
  } else if (event.key === 'k') {
    moveSelection(-1)
  } else if (event.key === 'Enter' && currentTask && !targetEl.closest('button, a')) {
    openId.value = openId.value === currentTask.id ? null : currentTask.id
  } else if (event.key === 'x' && currentTask) {
    void toggleTaskStatus(currentTask)
  } else if (event.key === 's' && currentTask && isOpen(currentTask)) {
    void run(() =>
      taskAction(currentTask.id, currentTask.status === 'IN_PROGRESS' ? 'pause' : 'start'),
    )
  } else if (event.key === 't' && currentTask && isOpen(currentTask)) {
    setDate(currentTask, todayString())
  } else if (/^[1-7]$/.test(event.key)) {
    filter.value = MAIN_FILTER_IDS[Number(event.key) - 1]!
  }
})

function focusTask(id: unknown) {
  const target = tasks.value.find((x) => x.id === id)
  if (!target) return

  filter.value =
    target.status === 'COMPLETED'
      ? 'erledigt'
      : target.status === 'CANCELLED'
        ? 'abgebrochen'
        : 'alle'
  projectId.value = 'alle'
  search.value = ''
  openId.value = target.id
  sel.value = target.id

  void nextTick(() => {
    document.querySelector(`[data-task="${target.id}"]`)?.scrollIntoView({ block: 'center' })
  })
}

function toggleOpenDetail(id: string) {
  openId.value = openId.value === id ? null : id
  sel.value = id
}

function focusNewFromRoute() {
  if (!route.query.new) return
  if (typeof route.query.text === 'string') {
    quickAddEl.value?.setQuick(route.query.text)
  }
  void nextTick(() => quickAddEl.value?.focus())
  void router.replace({ query: {} })
}
watch(() => route.query.new, focusNewFromRoute)

function filterFromRoute() {
  const queryFilter = route.query.filter
  if (typeof queryFilter === 'string') {
    filter.value = queryFilter
  }
}
watch(() => route.query.filter, filterFromRoute, { immediate: true })

onMounted(() => {
  focusNewFromRoute()
  void load().then(() => {
    if (route.query.task) {
      focusTask(route.query.task)
    }
  })
})

useLiveEvents(load)
</script>

<template>
  <div class="view-inner" :class="{ 'tk-split': openTask }">
    <div class="tk-left">
      <div class="v-head v-head-row tk-head">
        <h1 class="v-title">{{ $t('Aufgaben') }}</h1>
        <span class="spacer"></span>
        <p class="tk-stats">{{ stats }}</p>
      </div>

      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <!-- Quick Add Formular -->
      <TaskQuickAdd
        ref="quickAddEl"
        :projects="projects"
        :tasks="tasks"
        :default-project-id="defaultProject()"
        :today-string="todayString()"
        @created="handleTasksCreated"
        @error="error = $event"
      />

      <!-- Toolbar & Filter -->
      <TasksToolbar
        :tabs="tabs"
        :extra="extra"
        :filter="filter"
        :counts="counts"
        :search="search"
        :projects="projects"
        :project-id="projectId"
        :view="view"
        :groups="GROUPS"
        :sorts="SORTS"
        :selection-mode="selection.mode.value"
        @update:filter="filter = $event"
        @update:search="search = $event"
        @update:project-id="projectId = $event"
        @update:group="view.group = $event as any"
        @update:sort="view.sort = $event as any"
        @open-github="github = true"
        @toggle-selection="selection.mode.value = !selection.mode.value"
      />

      <!-- Aufgabenliste -->
      <div class="tasks-layout" :class="{ 'with-aside': openTask }">
        <div class="tasks-main" role="tabpanel" :aria-labelledby="`tk-tab-${filter}`">
          <TasksEmptyState
            v-if="!rows.length"
            :filter="filter"
            :search="search"
            :project-id="projectId"
            @select-example="quickAddEl?.setQuick($event)"
          />

          <div v-if="rows.length" class="tk-cols" aria-hidden="true">
            <span class="c-t">{{ $t('Aufgabe') }}</span>
            <span class="c-p">{{ $t('Projekt') }}</span>
            <span class="c-d">{{ $t('Dauer') }}</span>
            <span class="c-i">{{ $t('Prio') }}</span>
            <span class="c-f">{{ $t('Fällig') }}</span>
          </div>

          <div v-for="group in groups" :key="group.key" class="tgroup">
            <span
              v-if="group.title"
              class="lbl"
              :class="{ od: view.group === 'due' && group.key === '0' }"
            >
              {{ group.title }}
              <span class="count">{{ group.rows.length }}</span>
              <span v-if="group.minutes" class="g-sum">
                {{ $t('{time} geschätzt', { time: hm(group.minutes) }) }}
              </span>
            </span>

            <div class="tasklist">
              <TaskRow
                v-for="task in group.rows"
                :key="task.id"
                :task="task"
                :is-selected="sel === task.id"
                :is-picked="picked.includes(task.id)"
                :is-fresh="freshCreatedIds.includes(task.id)"
                :is-open-detail="openId === task.id"
                :project-name="task.project_id ? projectNameMap.get(task.project_id) : undefined"
                :subtasks-progress="progress(task.id)"
                :is-urgent="urgent(task)"
                :is-overdue="overdue(task)"
                :when-text="formatTaskWhen(task).text"
                :when-tip="formatTaskWhen(task).tip"
                @toggle-status="toggleTaskStatus"
                @open-detail="toggleOpenDetail"
                @pick="pickRow"
                @pointer-down="onPointerDown"
                @pointer-up="onPointerUp"
              />
            </div>
          </div>
        </div>
      </div>

      <SelectionBar
        v-if="picked.length || selection.mode.value"
        :count="picked.length"
        :projects="projects.filter((p) => p.status !== 'ARCHIVED')"
        @tomorrow="selection.toTomorrow()"
        @project="selection.toProject"
        @trash="selection.trash"
        @clear="selection.clear"
      />

      <TasksKeyboardHelp v-if="!rows.length" />
    </div>

    <!-- Modale Detailansicht als ausgelagerte Komponente -->
    <TaskEditSheet
      v-if="openTask"
      :task="openTask"
      :projects="projects"
      :resources="resOf(openTask.id)"
      :sub-tasks="kidsOf(openTask.id)"
      :progress-text="progress(openTask.id)"
      :edit-slot-hint="editSlotHint"
      :today-str="todayString()"
      @close="openId = null"
      @update-field="(field, val) => setField(openTask!, field, val)"
      @set-date="(d) => setDate(openTask!, d)"
      @set-time="(t) => setTime(openTask!, t)"
      @set-due="(d) => setDue(openTask!, d)"
      @set-estimate="(m) => setEstimate(openTask!, m)"
      @toggle-subtask="toggleTaskStatus"
      @add-subtask="
        (title) => {
          void run(async () => {
            await createTask({
              title,
              parent_task_id: openTask!.id,
              project_id: openTask!.project_id,
            })
          })
        }
      "
      @trash="trashTask(openTask)"
      @take-edit-slot="takeEditSlot"
      @run="run"
      @reload="load"
    />

    <!-- GitHub-Dialog -->
    <TaskGithubDialog
      :open="github"
      :projects="projects"
      @close="github = false"
      @open-task="
        (id) => {
          github = false
          void load().then(() => focusTask(id))
        }
      "
      @changed="load"
    />
  </div>
</template>

<style scoped>
.rec-i {
  color: var(--tx-muted);
  font-size: 13px;
  line-height: 1;
}

.tasks-layout {
  margin-top: 16px;
}

@media (min-width: 1001px) {
  .view-inner.tk-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 380px;
    max-width: none;
    padding: 0;
  }
  .tk-split .tk-left {
    min-width: 0;
    padding: 32px 40px 64px 32px;
  }
  .tk-cols {
    position: sticky;
    top: 56px;
    z-index: 2;
  }
}

.tk-stats {
  margin: 0 4px 3px 0;
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.tk-sum {
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}

@media (max-width: 640px) {
  .tk-sum {
    display: none;
  }
  .tk-head {
    flex-wrap: wrap;
  }
  .tk-stats {
    flex-basis: 100%;
    order: 3;
    margin: 6px 0 0;
    font-size: 14px;
    color: var(--tx-secondary);
  }
  .v-head-row .btn-primary {
    display: none;
  }
}

.btn-primary .key {
  margin-left: 6px;
  background: transparent;
  color: inherit;
  border-color: currentColor;
  opacity: 0.85;
}
.tgroup > .lbl.od {
  color: var(--a-red);
}
.g-sum {
  margin-left: auto;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
</style>
