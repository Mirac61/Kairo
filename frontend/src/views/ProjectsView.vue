<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NH1, NInput, NSelect, NSpace, NTag, NText } from 'naive-ui'
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
const typeOptions = ['URL', 'FILE', 'FOLDER'].map((v) => ({ value: v, label: v }))

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

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <n-space vertical :size="16">
    <n-h1 style="margin: 0">Projekte</n-h1>
    <n-alert v-if="error" type="error">{{ error }}</n-alert>

    <form @submit.prevent="add">
      <n-space>
        <n-input v-model:value="form.name" placeholder="Neues Projekt" style="width: 220px" />
        <n-input v-model:value="form.local_path" placeholder="Ordner (optional, absoluter Pfad)" style="width: 320px" />
        <n-button type="primary" attr-type="submit">Anlegen</n-button>
      </n-space>
    </form>

    <n-text v-if="!projects.length" depth="3">Keine Projekte.</n-text>
    <n-card v-for="p in projects" :key="p.id" size="small">
      <n-space vertical>
        <n-space align="center">
          <n-input :default-value="p.name" style="width: 220px" @change="(v: string) => run(() => updateProject(p.id, { name: v.trim() || p.name }))" />
          <n-input
            :default-value="p.local_path ?? ''" placeholder="Ordner (absoluter Pfad)" style="width: 320px"
            @change="(v: string) => run(() => updateProject(p.id, { local_path: v.trim() }))"
          />
          <n-select
            :value="p.status" :options="statusOptions" style="width: 150px"
            @update:value="(v: string) => run(() => updateProject(p.id, { status: v }))"
          />
          <delete-button :text="`„${p.name}“ löschen?`" @confirm="run(() => deleteProject(p.id))" />
        </n-space>
        <n-space align="center" :size="6">
          <n-tag v-for="r in resources.filter((x) => x.project_id === p.id)" :key="r.id" closable class="chip" @close="run(() => deleteResource(r.id))">
            {{ r.label || r.target }}<n-text depth="3"> · {{ r.type }}</n-text>
          </n-tag>
        </n-space>
        <form @submit.prevent="addResource(p)">
          <n-space>
            <n-select v-model:value="resFor(p.id).type" :options="typeOptions" size="small" style="width: 100px" />
            <n-input v-model:value="resFor(p.id).target" size="small" placeholder="URL oder Pfad (~ erlaubt)" style="width: 260px" />
            <n-input v-model:value="resFor(p.id).label" size="small" placeholder="Name (optional)" style="width: 160px" />
            <n-button size="small" attr-type="submit">+ Ressource</n-button>
          </n-space>
        </form>
      </n-space>
    </n-card>
  </n-space>
</template>
