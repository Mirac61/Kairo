<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Chip from 'primevue/chip'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import {
  createProject, createResource, deleteProject, deleteResource, errorMessage, listProjects, listResources,
  updateProject, type Project, type Resource,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'

const statusOptions = [
  { value: 'ACTIVE', label: 'Aktiv' }, { value: 'PAUSED', label: 'Pausiert' },
  { value: 'COMPLETED', label: 'Abgeschlossen' }, { value: 'ARCHIVED', label: 'Archiviert' },
]
const typeOptions = ['URL', 'FILE', 'FOLDER']

const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const error = ref('')
const form = ref({ name: '', local_path: '' })
const resForm = ref<Record<string, { type: Resource['type']; target: string; label: string }>>({})
const resFor = (id: string) => (resForm.value[id] ??= { type: 'URL', target: '', label: '' })

async function load() {
  try {
    ;[projects.value, resources.value] = await Promise.all([listProjects(), listResources()])
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

function addResource(p: Project) {
  const f = resFor(p.id)
  const target = f.target.trim()
  if (!target) return
  void run(async () => {
    await createResource({ project_id: p.id, type: f.type, target, label: f.label.trim() })
    resForm.value[p.id] = { type: f.type, target: '', label: '' }
  })
}

// Name und Ordner speichern beim Verlassen des Felds.
const valueOf = (e: Event) => (e.target as HTMLInputElement).value.trim()

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="stack">
    <h1 class="title">Projekte</h1>
    <Message v-if="error" severity="error">{{ error }}</Message>

    <form class="row" @submit.prevent="add">
      <InputText v-model="form.name" placeholder="Neues Projekt" class="w-name" />
      <InputText v-model="form.local_path" placeholder="Ordner (optional, absoluter Pfad)" class="w-path" />
      <Button type="submit" label="Anlegen" />
    </form>

    <span v-if="!projects.length" class="muted">Keine Projekte.</span>
    <section v-for="p in projects" :key="p.id" class="card stack">
      <div class="row">
        <InputText :default-value="p.name" aria-label="Name" class="w-name" @change="(e: Event) => run(() => updateProject(p.id, { name: valueOf(e) || p.name }))" />
        <InputText :default-value="p.local_path ?? ''" aria-label="Ordner" placeholder="Ordner (absoluter Pfad)" class="w-path" @change="(e: Event) => run(() => updateProject(p.id, { local_path: valueOf(e) }))" />
        <Select :model-value="p.status" :options="statusOptions" option-label="label" option-value="value" class="w-status" @update:model-value="(v: string) => run(() => updateProject(p.id, { status: v }))" />
        <DeleteButton :text="`„${p.name}“ löschen?`" @confirm="run(() => deleteProject(p.id))" />
      </div>
      <div v-if="resources.some((x) => x.project_id === p.id)" class="row">
        <Chip v-for="r in resources.filter((x) => x.project_id === p.id)" :key="r.id" :label="`${r.label || r.target} · ${r.type}`" removable @remove="run(() => deleteResource(r.id))" />
      </div>
      <form class="row" @submit.prevent="addResource(p)">
        <Select v-model="resFor(p.id).type" :options="typeOptions" size="small" class="w-type" />
        <InputText v-model="resFor(p.id).target" size="small" placeholder="URL oder Pfad (~ erlaubt)" class="w-target" />
        <InputText v-model="resFor(p.id).label" size="small" placeholder="Name (optional)" class="w-label" />
        <Button type="submit" label="+ Ressource" size="small" severity="secondary" />
      </form>
    </section>
  </div>
</template>

<style scoped>
.w-name { width: 220px; }
.w-path { width: 300px; }
.w-status { width: 150px; }
.w-type { width: 100px; }
.w-target { width: 260px; }
.w-label { width: 160px; }
</style>
