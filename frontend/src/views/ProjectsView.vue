<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api, createProject, deleteProject, errorMessage, listProjects, listResources, listTasks,
  updateProject, type Project, type Resource, type Task, type TimeEntry,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { vDialog } from '@/lib/dialog'
import { PROJECT_COLORS, projectColor } from '@/lib/projectColor'
import { ymd } from '@/lib/dates'
import DeleteButton from '@/components/DeleteButton.vue'
import ResourceList from '@/components/ResourceList.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS: Record<string, string> = { ACTIVE: 'Aktiv', PAUSED: 'Pausiert', COMPLETED: 'Abgeschlossen', ARCHIVED: 'Archiviert' }
const RES_ICON = { URL: 'i-link', FILE: 'i-file', FOLDER: 'i-file' } as const

interface Progress { project_id: string | null; done_tasks: number; total_tasks: number; tracked_minutes: number }

const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const tasks = ref<Task[]>([])
const minutes = ref<Record<string, { week: number; total: number }>>({})
const openId = ref<string | null>(null)
const progress = ref<Record<string, Progress>>({})
const error = ref('')
const dialog = ref(false)
const editId = ref<string | null>(null)
const form = ref({ name: '', description: '', local_path: '', status: 'ACTIVE', color: '' }) // color '' = automatisch (neues Projekt)

const editing = computed(() => projects.value.find((p) => p.id === editId.value))
const resOf = (id: string) => resources.value.filter((r) => r.project_id === id)
const taskRes = (id: string) => resources.value.filter((r) => r.task_id === id)
const openTasks = (id: string) => tasks.value.filter((t) => t.project_id === id && t.status !== 'COMPLETED' && t.status !== 'CANCELLED')
const dur = (m = 0) => (m >= 60 ? `${Math.floor(m / 60)} Std${m % 60 ? ` ${m % 60} Min` : ''}` : `${m} Min`)

// Erfasste Zeit je Projekt aus allen Zeiteinträgen (ein laufender Eintrag zählt bis jetzt).
function sumMinutes(es: TimeEntry[]) {
  const monday = new Date()
  monday.setHours(0, 0, 0, 0)
  monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7))
  const out: Record<string, { week: number; total: number }> = {}
  for (const e of es) {
    if (!e.project_id) continue
    const m = Math.max(0, Math.floor(((e.ended_at ? Date.parse(e.ended_at) : Date.now()) - Date.parse(e.started_at)) / 6e4))
    const k = (out[e.project_id] ??= { week: 0, total: 0 })
    k.total += m
    if (Date.parse(e.started_at) >= monday.getTime()) k.week += m
  }
  return out
}

async function load() {
  try {
    const t = ymd(new Date())
    const [ps, rs, rv, ts, es] = await Promise.all([
      listProjects(), listResources(), api<{ projects: Progress[] }>(`/review?from=${t}&to=${t}`), listTasks(), api<TimeEntry[]>('/time-entries'),
    ])
    projects.value = ps
    resources.value = rs
    tasks.value = ts
    minutes.value = sumMinutes(es)
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
  form.value = { name: p?.name ?? '', description: p?.description ?? '', local_path: p?.local_path ?? '', status: p?.status ?? 'ACTIVE', color: p?.color ?? '' }
  error.value = ''
  dialog.value = true
}

async function save() {
  const f = form.value
  const name = f.name.trim()
  if (!name) return
  const ok = await run(() =>
    editId.value
      ? updateProject(editId.value, { name, description: f.description.trim(), local_path: f.local_path.trim(), status: f.status, color: f.color })
      : createProject({ name, description: f.description.trim(), local_path: f.local_path.trim() || null, color: f.color || undefined }),
  )
  if (ok) dialog.value = false
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
// Die Extension öffnet den Workspace des Projekts.
const inCode = (p: Project) => { window.location.href = `vscodium://kairo-local.kairo/open?project=${p.id}` }
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
    <div v-if="error && !dialog" class="badge" role="alert">{{ error }}</div>

    <div class="proj-list">
      <div v-if="!projects.length" class="v-sub">Keine Projekte.</div>
      <article v-for="p in projects" :key="p.id" class="card proj-card" :style="{ '--pc': projectColor(p.id) }">
        <div class="proj-head">
          <span class="cdot"></span>
          <button type="button" class="proj-name proj-toggle" :aria-expanded="openId === p.id" @click="openId = openId === p.id ? null : p.id">{{ p.name }}</button>
          <span class="proj-pct">{{ pct(p.id) }}%</span>
          <button v-if="p.local_path" class="btn btn-ghost" type="button" :aria-label="`In VSCodium öffnen: ${p.name}`" @click="inCode(p)">In VSCodium öffnen</button>
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
        <div v-if="openId === p.id" class="proj-more">
          <div class="proj-meta">Erfasst: {{ dur(minutes[p.id]?.week) }} diese Woche · {{ dur(minutes[p.id]?.total) }} gesamt</div>
          <span class="lbl">Offene Aufgaben<span v-if="openTasks(p.id).length" class="count"> · {{ openTasks(p.id).length }}</span></span>
          <div v-if="openTasks(p.id).length" class="card tasklist">
            <div v-for="t in openTasks(p.id)" :key="t.id" class="task-row">
              <span class="t">{{ t.title }}</span>
              <component :is="href(r) ? 'a' : 'span'" v-for="r in taskRes(t.id)" :key="r.id" class="res" :href="href(r)" target="_blank" rel="noopener">
                <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>{{ r.label || r.target }}
              </component>
              <TaskActions :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
            </div>
          </div>
          <div v-else class="proj-meta">Keine offenen Aufgaben.</div>
        </div>
      </article>
    </div>

    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <div class="dialog" :aria-label="editing ? 'Projekt bearbeiten' : 'Neues Projekt'">
        <form @submit.prevent="save">
          <div class="dlg-head">
            <h3>{{ editing ? 'Projekt bearbeiten' : 'Neues Projekt' }}</h3>
            <button class="icon-btn" type="button" aria-label="Schließen" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
          </div>
          <div class="dlg-body">
            <div class="field"><label for="p-name">Name</label><input id="p-name" v-model="form.name" class="input" placeholder="Projektname" /></div>
            <div class="field"><label for="p-desc">Beschreibung</label><input id="p-desc" v-model="form.description" class="input" /></div>
            <div v-if="error" class="badge" role="alert">{{ error }}</div>
            <div class="field"><label for="p-path">Ordner</label><input id="p-path" v-model="form.local_path" class="input" placeholder="Absoluter Pfad oder ~/…, muss existieren (optional)" /></div>
            <div class="field">
              <label id="p-color-l">Farbe</label>
              <div class="swatches" role="radiogroup" aria-labelledby="p-color-l">
                <label v-for="[c, label] in PROJECT_COLORS" :key="c" class="swatch" :title="label">
                  <input v-model="form.color" type="radio" name="p-color" :value="c" :aria-label="label" /><span :style="{ background: `var(--a-${c})` }"></span>
                </label>
              </div>
            </div>
            <div v-if="editing" class="field">
              <label for="p-status">Status</label>
              <select id="p-status" v-model="form.status" class="input"><option v-for="(l, v) in STATUS" :key="v" :value="v">{{ l }}</option></select>
            </div>
          </div>
          <div class="dlg-foot">
            <DeleteButton v-if="editing" :text="`„${editing.name}“ löschen? Die Tasks bleiben ohne Projekt, ihre Zeiteinträge landen in „Sonstiges“.`" @confirm="run(() => deleteProject(editing!.id)).then((ok) => ok && (dialog = false))" />
            <span class="spacer"></span>
            <button class="btn btn-ghost" type="button" @click="dialog = false">Abbrechen</button>
            <button class="btn btn-primary" type="submit">{{ editing ? 'Speichern' : 'Anlegen' }}</button>
          </div>
        </form>

        <ResourceList v-if="editing" class="dlg-body dlg-res" :resources="resOf(editing.id)" :project-id="editing.id" @changed="load" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.spacer { flex: 1; }
.swatches { display: flex; flex-wrap: wrap; gap: 8px; }
.swatch { position: relative; display: inline-flex; cursor: pointer; }
.swatch input { position: absolute; inset: 0; opacity: 0; margin: 0; cursor: pointer; }
.swatch span { width: 24px; height: 24px; border-radius: 50%; border: 2px solid transparent; box-shadow: inset 0 0 0 2px var(--bg-1); }
.swatch input:checked + span { border-color: var(--tx-primary); }
.swatch input:focus-visible + span { outline: 2px solid var(--tx-primary); outline-offset: 2px; }
.dlg-res { border-top: 1px solid var(--br-subtle); }
.proj-toggle { text-align: left; background: none; border: 0; color: inherit; cursor: pointer; }
.proj-more { display: grid; gap: 8px; border-top: 1px solid var(--br-subtle); padding-top: 10px; }
</style>
