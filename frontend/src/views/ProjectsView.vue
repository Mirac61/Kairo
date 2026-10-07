<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api, createProject, deleteProject, listProjects, listResources, listTasks,
  isOpen, updateProject, type Project, type Resource, type Task, type TimeEntry,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { vDialog } from '@/lib/dialog'
import { PROJECT_COLORS, projectColor } from '@/lib/projectColor'
import { daysAgo, dur, entryMinutes, hm, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import DeleteButton from '@/components/DeleteButton.vue'
import ResourceList from '@/components/ResourceList.vue'
import SearchField from '@/components/SearchField.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS: Record<string, string> = { ACTIVE: 'Aktiv', PAUSED: 'Pausiert', COMPLETED: 'Abgeschlossen', ARCHIVED: 'Archiviert' }
const RES_ICON = { URL: 'i-link', FILE: 'i-file', FOLDER: 'i-file' } as const

interface Progress { project_id: string | null; done_tasks: number; total_tasks: number; tracked_minutes: number }

const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const tasks = ref<Task[]>([])
const minutes = ref<Record<string, { week: number; total: number; last: number }>>({})
const openId = ref<string | null>(null)
const progress = ref<Record<string, Progress>>({})
const dialog = ref(false)
const editId = ref<string | null>(null)
const form = ref({ name: '', description: '', local_path: '', status: 'ACTIVE', color: '' }) // color '' = automatisch (neues Projekt)

const editing = computed(() => projects.value.find((p) => p.id === editId.value))
const resOf = (id: string) => resources.value.filter((r) => r.project_id === id)
const taskRes = (id: string) => resources.value.filter((r) => r.task_id === id)
const openTasks = (id: string) => tasks.value.filter((t) => t.project_id === id && isOpen(t))

// Erfasste Zeit je Projekt aus allen Zeiteinträgen (ein laufender Eintrag zählt bis jetzt).
function sumMinutes(es: TimeEntry[]) {
  const monday = weekStart().getTime()
  const out: Record<string, { week: number; total: number; last: number }> = {}
  for (const e of es) {
    if (!e.project_id) continue
    const m = entryMinutes(e)
    const k = (out[e.project_id] ??= { week: 0, total: 0, last: 0 })
    k.total += m
    k.last = Math.max(k.last, e.ended_at ? Date.parse(e.ended_at) : Date.now())
    if (Date.parse(e.started_at) >= monday) k.week += m
  }
  return out
}

const { error, load, run } = useLoader(async () => {
  const t = ymd(new Date())
  const [ps, rs, rv, ts, es] = await Promise.all([
    listProjects(), listResources(), api<{ projects: Progress[] }>(`/review?from=${t}&to=${t}`), listTasks(), api<TimeEntry[]>('/time-entries'),
  ])
  projects.value = ps
  resources.value = rs
  tasks.value = ts
  minutes.value = sumMinutes(es)
  progress.value = Object.fromEntries(rv.projects.filter((p) => p.project_id).map((p) => [p.project_id!, p]))
})

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
const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? projects.value.filter((p) => `${p.name} ${p.description} ${p.local_path ?? ''}`.toLowerCase().includes(q)) : projects.value
})
const LAYOUT_KEY = 'kairo-projects-layout' // Kacheln oder eine Spalte merkt sich der Browser
const tab = ref('ACTIVE')
const sort = ref<'recent' | 'name'>('recent')
const layout = ref<'grid' | 'list'>('grid')
if (store.get(LAYOUT_KEY) === 'list') layout.value = 'list'
const setLayout = (l: 'grid' | 'list') => {
  layout.value = l
  store.set(LAYOUT_KEY, l)
}
const tabs = computed(() => Object.entries(STATUS).map(([status, label]) => ({ status, label, n: shown.value.filter((p) => p.status === status).length })))
const list = computed(() => shown.value
  .filter((p) => p.status === tab.value)
  .sort((a, b) => sort.value === 'name' ? a.name.localeCompare(b.name, 'de') : (minutes.value[b.id]?.last ?? 0) - (minutes.value[a.id]?.last ?? 0)))
const lastActive = (id: string) => {
  const t = minutes.value[id]?.last
  return t ? daysAgo(ymd(new Date(t)), ymd(new Date())) : ''
}
const frac = (id: string) => { const p = progress.value[id]; return p?.total_tasks ? `${p.done_tasks}/${p.total_tasks}` : '' }
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
      <button class="btn btn-primary" type="button" @click="openDialog()"><svg class="ic"><use href="#i-plus" /></svg>Neues Projekt</button>
    </div>
    <div v-if="error && !dialog" class="badge" role="alert">{{ error }}</div>

    <div class="filterbar">
      <button v-for="t in tabs" :key="t.status" type="button" class="fchip" :aria-pressed="tab === t.status" :disabled="!t.n && tab !== t.status" @click="tab = t.status">{{ t.label }} <span class="count">{{ t.n }}</span></button>
      <SearchField v-model="search" label="Projekte durchsuchen" />
      <span class="spacer"></span>
      <select v-model="sort" class="input" aria-label="Sortieren nach">
        <option value="recent">Zuletzt aktiv</option>
        <option value="name">Name</option>
      </select>
      <span class="seg" role="group" aria-label="Ansicht">
        <button type="button" aria-label="Kacheln" :aria-pressed="layout === 'grid'" @click="setLayout('grid')"><svg class="ic"><use href="#i-grid" /></svg></button>
        <button type="button" aria-label="Liste" :aria-pressed="layout === 'list'" @click="setLayout('list')"><svg class="ic"><use href="#i-list" /></svg></button>
      </span>
    </div>
    <div v-if="!projects.length" class="v-sub">Keine Projekte.</div>
    <div v-else-if="!list.length" class="v-sub">Keine Projekte<template v-if="search"> für „{{ search }}“</template> in „{{ STATUS[tab] }}“.</div>
    <div v-else class="proj-list" :class="layout">
      <article v-for="p in list" :key="p.id" class="proj-card" :class="{ open: openId === p.id }" :style="{ '--pc': projectColor(p.id) }">
        <div class="proj-head">
          <span class="cdot"></span>
          <button type="button" class="proj-name proj-toggle" :aria-expanded="openId === p.id" @click="openId = openId === p.id ? null : p.id">{{ p.name }}</button>
          <span class="proj-end">
            <span class="proj-last">{{ lastActive(p.id) }}</span>
            <button class="btn btn-ghost proj-act" type="button" @click="openDialog(p)">Bearbeiten</button>
          </span>
        </div>
        <div class="proj-desc" :class="{ none: !p.description }">{{ p.description || 'Keine Beschreibung' }}</div>
        <div class="proj-meta mono">{{ p.local_path ?? '' }}</div>
        <div v-if="resOf(p.id).length" class="proj-res">
          <component :is="href(r) ? 'a' : 'span'" v-for="r in resOf(p.id)" :key="r.id" class="res" :href="href(r)" target="_blank" rel="noopener">
            <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>{{ r.label || r.target }}
          </component>
        </div>
        <div class="proj-prog">
          <div class="pbar" role="progressbar" :aria-label="`Fortschritt ${p.name}`" :aria-valuenow="pct(p.id)" aria-valuemin="0" aria-valuemax="100"><div class="pbar-fill" :style="{ width: pct(p.id) + '%' }"></div></div>
          <span class="proj-pct">{{ pct(p.id) }} %</span>
        </div>
        <div class="proj-foot">
          <span class="proj-stat"><b class="mono">{{ openTasks(p.id).length }}</b> offen</span>
          <span class="proj-stat"><b class="mono">{{ hm(minutes[p.id]?.week ?? 0) }}</b> diese Woche</span>
          <span class="spacer"></span>
          <button v-if="p.local_path" class="btn btn-ghost" type="button" :aria-label="`In VSCodium öffnen: ${p.name}`" @click="inCode(p)">VSCodium ↗</button>
        </div>
        <div v-if="openId === p.id" class="proj-more">
          <div class="proj-meta">Erfasst: {{ dur(minutes[p.id]?.week) }} diese Woche · {{ dur(minutes[p.id]?.total) }} gesamt<template v-if="frac(p.id)"> · {{ frac(p.id) }} Aufgaben erledigt</template></div>
          <span class="lbl">Offene Aufgaben<span v-if="openTasks(p.id).length" class="count"> · {{ openTasks(p.id).length }}</span></span>
          <div v-if="openTasks(p.id).length" class="card tasklist">
            <div v-for="t in openTasks(p.id)" :key="t.id" class="task-row">
              <span class="t">{{ t.title }}</span>
              <component :is="href(r) ? 'a' : 'span'" v-for="r in taskRes(t.id)" :key="r.id" class="res" :href="href(r)" target="_blank" rel="noopener">
                <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>{{ r.label || r.target }}
              </component>
              <TaskActions class="row-act" :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
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
                  <input v-model="form.color" type="radio" name="p-color" :value="c" :aria-label="label" /><span :style="{ background: `var(--p-${c})` }"></span>
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
