<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import {
  createTask, deleteTask, errorMessage, listProjects, listTasks, updateTask,
  type Project, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { dateOf, hhmm, single, timeOf, ymd } from '@/lib/dates'
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
const form = ref({ title: '', project_id: null as string | null, estimated_minutes: null as number | null, planned_date: null as Date | null })

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))

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
      title, project_id: f.project_id, estimated_minutes: f.estimated_minutes || 0,
      planned_date: f.planned_date ? ymd(f.planned_date) : null,
    })
    form.value.title = ''
  })
}

const startTime = (t: Task) => (t.planned_start_at ? hhmm(new Date(t.planned_start_at)) : null)

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
function setTime(t: Task, d: Date | null) {
  if (!t.planned_date) return
  void run(() => updateTask(t.id, { planned_start_at: d ? new Date(`${t.planned_date}T${hhmm(d)}`).toISOString() : '' }))
}
// Erst beim Verlassen des Felds speichern, nicht bei jeder Ziffer.
const setEstimate = (t: Task, value: string) => void run(() => updateTask(t.id, { estimated_minutes: Number(value) || 0 }))

const open = (t: Task) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="stack">
    <h1 class="title">Tasks</h1>
    <Message v-if="error" severity="error">{{ error }}</Message>

    <form class="row" @submit.prevent="add">
      <InputText v-model="form.title" placeholder="Neue Task" class="w-title" />
      <Select v-model="form.project_id" :options="projects" option-label="name" option-value="id" show-clear placeholder="Projekt" class="w-select" />
      <InputNumber v-model="form.estimated_minutes" :min="0" :use-grouping="false" placeholder="Min." input-class="w-num" />
      <DatePicker v-model="form.planned_date" show-icon placeholder="Datum" date-format="dd.mm.yy" show-button-bar class="w-date" />
      <Button type="submit" label="Anlegen" />
    </form>

    <div class="row">
      <Select v-model="status" :options="statusOptions" option-label="label" option-value="value" show-clear placeholder="Alle Status" class="w-select" @change="load" />
      <Select v-model="projectId" :options="projects" option-label="name" option-value="id" show-clear placeholder="Alle Projekte" class="w-select" @change="load" />
    </div>

    <span v-if="!tasks.length" class="muted">Keine Tasks.</span>
    <ul v-else class="list card">
      <li v-for="t in tasks" :key="t.id">
        <span class="main">
          <span :class="{ done: !open(t) }">{{ t.title }}</span>
          <span class="muted">{{ STATUS_LABEL[t.status] }}<template v-if="t.project_id"> · {{ projectName.get(t.project_id) }}</template></span>
        </span>
        <div class="row nowrap">
          <template v-if="open(t)">
            <DatePicker
              :model-value="dateOf(t.planned_date)" placeholder="Datum" date-format="dd.mm.yy" show-button-bar class="w-date"
              @update:model-value="(v) => setDate(t, single(v) ? ymd(single(v)!) : null)"
            />
            <DatePicker
              :model-value="timeOf(startTime(t))" time-only hour-format="24" placeholder="Uhrzeit" :disabled="!t.planned_date" class="w-time"
              @update:model-value="(v) => setTime(t, single(v))"
            />
            <InputNumber
              :model-value="t.estimated_minutes" :min="0" :use-grouping="false" input-class="w-num" suffix=" min"
              @blur="(e) => setEstimate(t, e.value)"
            />
            <Button v-if="t.planned_date !== ymd(new Date())" label="Heute" size="small" severity="secondary" @click="setDate(t, ymd(new Date()))" />
          </template>
          <TaskActions :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
          <DeleteButton :text="`„${t.title}“ löschen?`" @confirm="run(() => deleteTask(t.id))" />
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.w-title { width: 220px; }
.w-select { width: 170px; }
.w-date { width: 150px; }
.w-time { width: 100px; }
:deep(.w-num) { width: 80px; }
</style>
