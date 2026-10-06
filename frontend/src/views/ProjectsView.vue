<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api, createProject, createResource, deleteProject, deleteResource, errorMessage, listProjects, listResources,
  updateProject, type Project, type Resource,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { projectColor } from '@/lib/projectColor'
import { ymd } from '@/lib/dates'
import DeleteButton from '@/components/DeleteButton.vue'

const STATUS: Record<string, string> = { ACTIVE: 'Aktiv', PAUSED: 'Pausiert', COMPLETED: 'Abgeschlossen', ARCHIVED: 'Archiviert' }
const RES_ICON = { URL: 'i-link', FILE: 'i-file', FOLDER: 'i-file' } as const

interface Progress { project_id: string | null; done_tasks: number; total_tasks: number; tracked_minutes: number }

const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const progress = ref<Record<string, Progress>>({})
const error = ref('')
const dialog = ref(false)
const editId = ref<string | null>(null)
const form = ref({ name: '', description: '', local_path: '', status: 'ACTIVE' })
const resForm = ref({ type: 'URL' as Resource['type'], target: '', label: '' })

const editing = computed(() => projects.value.find((p) => p.id === editId.value))
const resOf = (id: string) => resources.value.filter((r) => r.project_id === id)

async function load() {
  try {
    const t = ymd(new Date())
    const [ps, rs, rv] = await Promise.all([
      listProjects(), listResources(), api<{ projects: Progress[] }>(`/review?from=${t}&to=${t}`),
    ])
    projects.value = ps
    resources.value = rs
    progress.value = Object.fromEntries(rv.projects.filter((p) => p.project_id).map((p) => [p.project_id!, p]))
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
    return false
  }
  await load()
  return true
}

function openDialog(p?: Project) {
  editId.value = p?.id ?? null
  form.value = { name: p?.name ?? '', description: p?.description ?? '', local_path: p?.local_path ?? '', status: p?.status ?? 'ACTIVE' }
  resForm.value = { type: 'URL', target: '', label: '' }
  dialog.value = true
}

async function save() {
  const f = form.value
  const name = f.name.trim()
  if (!name) return
  const ok = await run(() =>
    editId.value
      ? updateProject(editId.value, { name, description: f.description.trim(), local_path: f.local_path.trim(), status: f.status })
      : createProject({ name, description: f.description.trim(), local_path: f.local_path.trim() || null }),
  )
  if (ok) dialog.value = false
}

async function addResource() {
  const target = resForm.value.target.trim()
  if (!target || !editId.value) return
  const ok = await run(() => createResource({ project_id: editId.value, type: resForm.value.type, target, label: resForm.value.label.trim() }))
  if (ok) resForm.value = { type: resForm.value.type, target: '', label: '' }
}

const pct = (id: string) => {
  const p = progress.value[id]
  return p && p.total_tasks ? Math.round((p.done_tasks / p.total_tasks) * 100) : 0
}
const meta = (p: Project) => {
  const pr = progress.value[p.id]
  const parts = [STATUS[p.status] ?? p.status, pr ? `${pr.done_tasks} von ${pr.total_tasks} Aufgaben erledigt` : '']
  if (p.local_path) parts.push(p.local_path)
  return parts.filter(Boolean).join(' · ')
}
// Datei und Ordner öffnet der Browser nicht; nur URLs sind echte Links.
const href = (r: Resource) => (r.type === 'URL' ? r.target : undefined)

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">Projekte</h1>
        <div class="v-sub">Fortschritt und verknüpfte Ressourcen.</div>
      </div>
      <button class="btn btn-secondary" type="button" @click="openDialog()"><svg class="ic"><use href="#i-plus" /></svg>Neues Projekt</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="proj-list">
      <div v-if="!projects.length" class="v-sub">Keine Projekte.</div>
      <article v-for="p in projects" :key="p.id" class="card proj-card" :style="{ '--pc': projectColor(p.id) }">
        <div class="proj-head">
          <span class="cdot"></span>
          <span class="proj-name">{{ p.name }}</span>
          <span class="proj-pct">{{ pct(p.id) }}%</span>
          <button class="btn btn-ghost" type="button" @click="openDialog(p)">Bearbeiten</button>
        </div>
        <div class="pbar" role="progressbar" :aria-valuenow="pct(p.id)" aria-valuemin="0" aria-valuemax="100"><div class="pbar-fill" :style="{ width: pct(p.id) + '%' }"></div></div>
        <div v-if="p.description" class="proj-desc">{{ p.description }}</div>
        <div class="proj-meta">{{ meta(p) }}</div>
        <div v-if="resOf(p.id).length" class="proj-res">
          <component :is="href(r) ? 'a' : 'span'" v-for="r in resOf(p.id)" :key="r.id" class="res" :href="href(r)" target="_blank" rel="noopener">
            <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>{{ r.label || r.target }}
          </component>
        </div>
      </article>
    </div>

    <div class="overlay" :class="{ open: dialog }" @click.self="dialog = false" @keydown.esc="dialog = false">
      <div class="dialog" role="dialog" :aria-label="editing ? 'Projekt bearbeiten' : 'Neues Projekt'">
        <form @submit.prevent="save">
          <div class="dlg-head">
            <h3>{{ editing ? 'Projekt bearbeiten' : 'Neues Projekt' }}</h3>
            <button class="icon-btn" type="button" aria-label="Schließen" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
          </div>
          <div class="dlg-body">
            <div class="field"><label for="p-name">Name</label><input id="p-name" v-model="form.name" class="input" placeholder="Projektname" /></div>
            <div class="field"><label for="p-desc">Beschreibung</label><input id="p-desc" v-model="form.description" class="input" /></div>
            <div class="field"><label for="p-path">Ordner</label><input id="p-path" v-model="form.local_path" class="input" placeholder="Absoluter Pfad (optional)" /></div>
            <div v-if="editing" class="field">
              <label for="p-status">Status</label>
              <select id="p-status" v-model="form.status" class="input"><option v-for="(l, v) in STATUS" :key="v" :value="v">{{ l }}</option></select>
            </div>
          </div>
          <div class="dlg-foot">
            <DeleteButton v-if="editing" :text="`„${editing.name}“ löschen?`" @confirm="run(() => deleteProject(editing!.id)).then((ok) => ok && (dialog = false))" />
            <span class="spacer"></span>
            <button class="btn btn-ghost" type="button" @click="dialog = false">Abbrechen</button>
            <button class="btn btn-primary" type="submit">{{ editing ? 'Speichern' : 'Anlegen' }}</button>
          </div>
        </form>

        <div v-if="editing" class="dlg-body res-edit">
          <span class="lbl">Ressourcen</span>
          <div v-for="r in resOf(editing.id)" :key="r.id" class="res-line">
            <span class="res-t">{{ r.label || r.target }} <span class="muted">· {{ r.type }}</span></span>
            <button class="icon-btn" type="button" aria-label="Ressource entfernen" @click="run(() => deleteResource(r.id))"><svg class="ic"><use href="#i-trash" /></svg></button>
          </div>
          <form class="res-add" @submit.prevent="addResource">
            <select v-model="resForm.type" class="input" aria-label="Typ"><option>URL</option><option>FILE</option><option>FOLDER</option></select>
            <input v-model="resForm.target" class="input" placeholder="URL oder Pfad (~ erlaubt)" aria-label="Ziel" />
            <input v-model="resForm.label" class="input" placeholder="Name (optional)" aria-label="Name" />
            <button class="btn btn-secondary" type="submit">Hinzufügen</button>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.spacer { flex: 1; }
.res-edit { border-top: 1px solid var(--br-subtle); display: grid; gap: 8px; }
.res-line { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
.res-t { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.res-add { display: grid; grid-template-columns: 90px 1fr; gap: 8px; }
.res-add .btn { grid-column: 1 / -1; justify-self: start; }
</style>
