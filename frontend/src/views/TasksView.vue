<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  createTask,
  deleteTask,
  errorMessage,
  listProjects,
  listTasks,
  taskAction,
  type Project,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: 'Backlog',
  PLANNED: 'Geplant',
  IN_PROGRESS: 'Läuft',
  PAUSED: 'Pausiert',
  COMPLETED: 'Erledigt',
  CANCELLED: 'Abgebrochen',
}

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const error = ref('')
const status = ref('')
const projectId = ref('')
const form = ref({ title: '', project_id: '', estimated_minutes: 0, planned_date: '' })

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
      title,
      project_id: f.project_id || null,
      estimated_minutes: f.estimated_minutes || 0,
      planned_date: f.planned_date || null,
    })
    form.value.title = ''
  })
}

function remove(t: Task) {
  if (confirm(`„${t.title}“ löschen?`)) void run(() => deleteTask(t.id))
}

const open = (t: Task) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <section>
    <h1>Tasks</h1>
    <p v-if="error" class="error">{{ error }}</p>

    <form class="row" @submit.prevent="add">
      <input v-model="form.title" placeholder="Neue Task" required />
      <select v-model="form.project_id">
        <option value="">Kein Projekt</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
      <input v-model.number="form.estimated_minutes" type="number" min="0" class="num" title="Schätzung in Minuten" />
      <input v-model="form.planned_date" type="date" />
      <button>Anlegen</button>
    </form>

    <div class="row filters">
      <select v-model="status" @change="load">
        <option value="">Alle Status</option>
        <option v-for="(label, s) in STATUS_LABEL" :key="s" :value="s">{{ label }}</option>
      </select>
      <select v-model="projectId" @change="load">
        <option value="">Alle Projekte</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </div>

    <p v-if="!tasks.length" class="hint">Keine Tasks.</p>
    <ul class="list">
      <li v-for="t in tasks" :key="t.id">
        <span class="title" :class="{ done: !open(t) }">
          {{ t.title }}
          <small>
            {{ STATUS_LABEL[t.status] }}
            <template v-if="t.project_id"> · {{ projectName.get(t.project_id) }}</template>
            <template v-if="t.estimated_minutes"> · {{ t.estimated_minutes }} min</template>
            <template v-if="t.planned_date"> · {{ t.planned_date }}</template>
          </small>
        </span>
        <span class="actions">
          <template v-if="open(t)">
            <button v-if="t.status === 'IN_PROGRESS'" @click="run(() => taskAction(t.id, 'pause'))">Pause</button>
            <button v-else @click="run(() => taskAction(t.id, 'start'))">Start</button>
            <button @click="run(() => taskAction(t.id, 'complete'))">Fertig</button>
          </template>
          <button @click="remove(t)">Löschen</button>
        </span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 16px; font-size: 24px; }
.hint { color: var(--text-muted); }
.error { color: var(--err); }
.row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.row input:first-child { flex: 1; min-width: 160px; }
.num { width: 80px; }
input, select, button {
  padding: 4px 10px; border: 1px solid var(--border); border-radius: 6px;
  background: var(--surface); color: var(--text); font: inherit;
}
button { background: var(--bg); cursor: pointer; }
button:hover { border-color: var(--accent); color: var(--accent); }
.list { list-style: none; margin: 0; padding: 0; }
.list li {
  display: flex; align-items: center; gap: 12px; padding: 8px 12px; margin-bottom: 4px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.title { flex: 1; }
.title small { display: block; color: var(--text-muted); font-size: 13px; }
.done { text-decoration: line-through; color: var(--text-muted); }
.actions { display: flex; gap: 6px; }
</style>
