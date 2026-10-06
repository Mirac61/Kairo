<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NAlert, NButton, NDatePicker, NH1, NInput, NInputNumber, NList, NListItem, NSelect, NSpace, NText, NTimePicker,
} from 'naive-ui'
import {
  createTask, deleteTask, errorMessage, listProjects, listTasks, updateTask,
  type Project, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: 'Backlog', PLANNED: 'Geplant', IN_PROGRESS: 'Läuft',
  PAUSED: 'Pausiert', COMPLETED: 'Erledigt', CANCELLED: 'Abgebrochen',
}
const statusOptions = Object.entries(STATUS_LABEL).map(([value, label]) => ({ value, label }))

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const error = ref('')
const status = ref<string | null>(null)
const projectId = ref<string | null>(null)
const form = ref({ title: '', project_id: null as string | null, estimated_minutes: null as number | null, planned_date: null as string | null })

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const projectOptions = computed(() => projects.value.map((p) => ({ value: p.id, label: p.name })))

async function load() {
  const q = new URLSearchParams()
  if (status.value) q.set('status', status.value)
  if (projectId.value) q.set('project_id', projectId.value)
  try {
    ;[tasks.value, projects.value] = await Promise.all([listTasks(q.size ? `?${q}` : ''), listProjects()])
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function run(fn: () => Promise<unknown>) {
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
    return
  }
  await load()
}

function add() {
  const f = form.value
  const title = f.title.trim()
  if (!title) return
  void run(async () => {
    await createTask({
      title, project_id: f.project_id, estimated_minutes: f.estimated_minutes || 0, planned_date: f.planned_date,
    })
    form.value.title = ''
  })
}

const pad = (n: number) => String(n).padStart(2, '0')
const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
const startTime = (t: Task) => {
  if (!t.planned_start_at) return null
  const d = new Date(t.planned_start_at)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// Datum und Uhrzeit hängen zusammen: Ohne Datum gibt es keine Uhrzeit, ein neues Datum behält sie.
function setDate(t: Task, date: string | null) {
  const time = startTime(t)
  void run(() =>
    updateTask(t.id, {
      planned_date: date ?? '',
      planned_start_at: date && time ? new Date(`${date}T${time}`).toISOString() : '',
    }),
  )
}
function setTime(t: Task, time: string | null) {
  if (!t.planned_date) return
  void run(() => updateTask(t.id, { planned_start_at: time ? new Date(`${t.planned_date}T${time}`).toISOString() : '' }))
}
// Erst beim Verlassen des Felds speichern, nicht bei jeder Ziffer.
const setEstimate = (t: Task, e: FocusEvent) =>
  void run(() => updateTask(t.id, { estimated_minutes: Number((e.target as HTMLInputElement).value) || 0 }))

const open = (t: Task) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <n-space vertical :size="16">
    <n-h1 style="margin: 0">Tasks</n-h1>
    <n-alert v-if="error" type="error">{{ error }}</n-alert>

    <form @submit.prevent="add">
      <n-space>
        <n-input v-model:value="form.title" placeholder="Neue Task" style="width: 240px" />
        <n-select v-model:value="form.project_id" :options="projectOptions" clearable placeholder="Projekt" style="width: 160px" />
        <n-input-number v-model:value="form.estimated_minutes" :min="0" :show-button="false" placeholder="Min." style="width: 80px" />
        <n-date-picker v-model:formatted-value="form.planned_date" value-format="yyyy-MM-dd" type="date" clearable placeholder="Datum" style="width: 150px" />
        <n-button type="primary" attr-type="submit">Anlegen</n-button>
      </n-space>
    </form>

    <n-space>
      <n-select v-model:value="status" :options="statusOptions" clearable placeholder="Alle Status" style="width: 160px" @update:value="load" />
      <n-select v-model:value="projectId" :options="projectOptions" clearable placeholder="Alle Projekte" style="width: 180px" @update:value="load" />
    </n-space>

    <n-text v-if="!tasks.length" depth="3">Keine Tasks.</n-text>
    <n-list v-else bordered>
      <n-list-item v-for="t in tasks" :key="t.id">
        <n-space vertical :size="2">
          <n-text :delete="!open(t)">{{ t.title }}</n-text>
          <n-text depth="3" style="font-size: 13px">
            {{ STATUS_LABEL[t.status] }}<template v-if="t.project_id"> · {{ projectName.get(t.project_id) }}</template>
          </n-text>
        </n-space>
        <template #suffix>
          <n-space :size="8" align="center" :wrap="false">
            <template v-if="open(t)">
              <n-date-picker
                :formatted-value="t.planned_date" value-format="yyyy-MM-dd" type="date" clearable size="small" placeholder="Datum" style="width: 140px"
                @update:formatted-value="(v: string | null) => setDate(t, v)"
              />
              <n-time-picker
                :formatted-value="startTime(t)" format="HH:mm" value-format="HH:mm" clearable size="small" placeholder="Uhrzeit"
                :disabled="!t.planned_date" style="width: 100px" @update:formatted-value="(v: string | null) => setTime(t, v)"
              />
              <n-input-number
                :value="t.estimated_minutes" :min="0" :show-button="false" size="small" style="width: 64px"
                @blur="(e: FocusEvent) => setEstimate(t, e)"
              />
              <n-button v-if="t.planned_date !== today()" size="small" @click="setDate(t, today())">Heute</n-button>
            </template>
            <task-actions :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
            <delete-button :text="`„${t.title}“ löschen?`" @confirm="run(() => deleteTask(t.id))" />
          </n-space>
        </template>
      </n-list-item>
    </n-list>
  </n-space>
</template>
