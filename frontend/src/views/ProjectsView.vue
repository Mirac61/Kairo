<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  createProject,
  deleteProject,
  errorMessage,
  listProjects,
  updateProject,
  type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'

const STATUS_LABEL: Record<string, string> = {
  ACTIVE: 'Aktiv',
  PAUSED: 'Pausiert',
  COMPLETED: 'Abgeschlossen',
  ARCHIVED: 'Archiviert',
}

const projects = ref<Project[]>([])
const error = ref('')
const form = ref({ name: '', local_path: '' })

async function load() {
  try {
    projects.value = await listProjects()
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
  const name = form.value.name.trim()
  if (!name) return
  void run(async () => {
    await createProject({ name, local_path: form.value.local_path.trim() || null })
    form.value = { name: '', local_path: '' }
  })
}

function remove(p: Project) {
  if (confirm(`„${p.name}“ löschen?`)) void run(() => deleteProject(p.id))
}

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <section>
    <h1>Projekte</h1>
    <p v-if="error" class="error">{{ error }}</p>

    <form class="row" @submit.prevent="add">
      <input v-model="form.name" placeholder="Neues Projekt" required />
      <input v-model="form.local_path" placeholder="Ordner (optional, z. B. /Users/…/repo)" />
      <button>Anlegen</button>
    </form>

    <p v-if="!projects.length" class="hint">Keine Projekte.</p>
    <ul class="list">
      <li v-for="p in projects" :key="p.id">
        <span class="title">
          {{ p.name }}
          <small v-if="p.local_path">{{ p.local_path }}</small>
        </span>
        <select :value="p.status" @change="run(() => updateProject(p.id, { status: ($event.target as HTMLSelectElement).value }))">
          <option v-for="(label, s) in STATUS_LABEL" :key="s" :value="s">{{ label }}</option>
        </select>
        <button @click="remove(p)">Löschen</button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 16px; font-size: 24px; }
.hint { color: var(--text-muted); }
.error { color: var(--err); }
.row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.row input { flex: 1; min-width: 160px; }
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
</style>
