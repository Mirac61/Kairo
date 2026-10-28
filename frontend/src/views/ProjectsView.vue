<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  api, createProject, createTask, deleteProject, listProjects, listResources, listTasks,
  isOpen, updateProject, updateTask, type Project, type Resource, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { vDialog } from '@/lib/dialog'
import { PROJECT_COLORS, projectColor } from '@/lib/projectColor'
import { PROJECT_ICONS, PROJECT_ICON_OPTIONS, projectIconKey, projectIconOf } from '@/lib/projectIcons'
import { locale, t } from '@/lib/i18n'
import { daysAgo, dur, hm, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import DeleteButton from '@/components/DeleteButton.vue'
import GitHubIssues from '@/components/GitHubIssues.vue'
import ResourceList from '@/components/ResourceList.vue'
import SearchField from '@/components/SearchField.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS: Record<string, string> = { ACTIVE: t('Aktiv'), PAUSED: t('Pausiert'), COMPLETED: t('Abgeschlossen'), ARCHIVED: t('Archiviert') }
const RES_ICON = { URL: 'i-link', FILE: 'i-file', FOLDER: 'i-file', NOTE: 'i-file' } as const

interface Progress { project_id: string | null; done_tasks: number; total_tasks: number; tracked_minutes: number }

const router = useRouter()
const route = useRoute()
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const tasks = ref<Task[]>([])
const minutes = ref<Record<string, { week: number; total: number; last: number }>>({})
const openId = ref<string | null>(null)
const showIconPicker = ref(false)
const progress = ref<Record<string, Progress>>({})
const dialog = ref(false)
const form = ref({ name: '', description: '', local_path: '', github_repo: '', status: 'ACTIVE', color: '', icon: '' })
const newTaskTitle = ref('')

const open = computed(() => projects.value.find((p) => p.id === openId.value))
watch(openId, () => { showIconPicker.value = false })
const resOf = (id: string) => resources.value.filter((r) => r.project_id === id)
const taskRes = (id: string) => resources.value.filter((r) => r.task_id === id)
const openTasks = (id: string) => tasks.value.filter((t) => t.project_id === id && isOpen(t))

// Erfasste Zeit je Projekt, vom Server summiert (ein laufender Eintrag zählt bis jetzt).
interface ProjectStat { project_id: string; week_minutes: number; total_minutes: number; last_active_at: string }

const { error, load, run } = useLoader(async () => {
  const today = ymd(new Date())
  const [ps, rs, rv, ts, es] = await Promise.all([
    listProjects(), listResources(), api<{ projects: Progress[] }>(`/review?from=${today}&to=${today}`), listTasks(), api<ProjectStat[]>('/projects/stats'),
  ])
  projects.value = ps
  resources.value = rs
  tasks.value = ts
  minutes.value = Object.fromEntries(es.map((s) => [s.project_id, { week: s.week_minutes, total: s.total_minutes, last: Date.parse(s.last_active_at) }]))
  progress.value = Object.fromEntries(rv.projects.filter((p) => p.project_id).map((p) => [p.project_id!, p]))
})

function openNewDialog() {
  form.value = { name: '', description: '', local_path: '', github_repo: '', status: 'ACTIVE', color: '', icon: '' }
  error.value = ''
  dialog.value = true
}

async function saveNew() {
  const f = form.value
  const name = f.name.trim()
  if (!name) return
  let created: Project | undefined
  const ok = await run(async () => {
    created = await createProject({
      name,
      description: f.description.trim(),
      local_path: f.local_path.trim() || null,
      github_repo: f.github_repo.trim() || null,
      color: f.color || undefined,
      icon: f.icon || undefined,
    })
  })
  if (ok && created) {
    dialog.value = false
    await load()
    openId.value = created.id
  }
}

async function setField(p: Project, patch: Partial<Project>) {
  const updated = await run(() => updateProject(p.id, patch))
  if (updated) {
    Object.assign(p, updated)
  }
}

async function toggleTask(t: Task) {
  const next = t.status === 'COMPLETED' ? 'PLANNED' : 'COMPLETED'
  await run(() => updateTask(t.id, { status: next }))
  await load()
}

async function addTaskForProject(p: Project) {
  const title = newTaskTitle.value.trim()
  if (!title) return
  await run(() => createTask({ title, project_id: p.id, status: 'PLANNED' }))
  newTaskTitle.value = ''
  await load()
}

async function deleteCurrentProject(p: Project) {
  const ok = await run(() => deleteProject(p.id))
  if (ok) {
    openId.value = null
    await load()
  }
}

const valueOf = (e: Event) => (e.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value

const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? projects.value.filter((p) => `${p.name} ${p.description} ${p.local_path ?? ''}`.toLowerCase().includes(q)) : projects.value
})
const LAYOUT_KEY = 'kairo-projects-layout'
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
  .sort((a, b) => sort.value === 'name' ? a.name.localeCompare(b.name, locale.value) : (minutes.value[b.id]?.last ?? 0) - (minutes.value[a.id]?.last ?? 0)))
const lastActive = (id: string) => {
  const t = minutes.value[id]?.last
  return t ? daysAgo(ymd(new Date(t)), ymd(new Date())) : ''
}
const frac = (id: string) => { const p = progress.value[id]; return p?.total_tasks ? `${p.done_tasks}/${p.total_tasks}` : '' }

// Die Extension öffnet den Workspace des Projekts.
const inCode = (p: Project) => { window.location.href = `vscodium://kairo-local.kairo/open?project=${p.id}` }
// Notizen öffnen in Kairo selbst, URLs in neuem Tab.
const href = (r: Resource) => (r.type === 'URL' ? r.target : r.type === 'NOTE' ? `/notes?open=${encodeURIComponent(r.target)}` : undefined)

// Offene GitHub-Issues je Projekt
const issueCount = ref<Record<string, number>>({})
const loadIssueCounts = () => Promise.all(projects.value.filter((p) => p.local_path).map(async (p) => {
  try {
    const r = await api<{ repo: string; issues: unknown[] }>(`/projects/${p.id}/github/issues`)
    if (r.repo) issueCount.value[p.id] = r.issues.length
  } catch { /* kein gh oder GitHub nicht erreichbar */ }
}))

function focusFromRoute() {
  const p = projects.value.find((x) => x.id === route.query.project)
  if (!p) return
  tab.value = p.status
  search.value = ''
  openId.value = p.id
  void nextTick(() => document.querySelector(`[data-project="${p.id}"]`)?.scrollIntoView({ block: 'center' }))
}
watch(() => route.query.project, focusFromRoute)
onMounted(() => void load().then(() => { focusFromRoute(); return loadIssueCounts() }))
useLiveEvents(load)

function move(d: number) {
  const l = list.value
  if (!l.length) return
  const i = l.findIndex((p) => p.id === openId.value)
  const next = l[i < 0 ? (d > 0 ? 0 : l.length - 1) : Math.min(Math.max(i + d, 0), l.length - 1)]!
  openId.value = next.id
  void nextTick(() => document.querySelector(`[data-project="${next.id}"]`)?.scrollIntoView({ block: 'nearest' }))
}

useShortcuts((e) => {
  const el = e.target as HTMLElement
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') return
  if (e.key === 'j') move(1)
  else if (e.key === 'k') move(-1)
  else if (e.key === 'Escape') openId.value = null
})
</script>

<template>
  <div class="view-inner" :class="{ 'proj-split': !!open }">
    <div class="proj-left">
      <div class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ $t('Projekte') }}</h1>
          <div class="v-sub">{{ $t('Fortschritt und verknüpfte Ressourcen.') }}</div>
        </div>
        <button class="btn btn-primary" type="button" @click="openNewDialog()"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neues Projekt') }}</button>
      </div>
      <div v-if="error && !dialog" class="badge" role="alert">{{ error }}</div>

      <div class="filterbar">
        <div class="seg" role="group" :aria-label="$t('Filter')">
          <button v-for="t in tabs" :key="t.status" type="button" :aria-pressed="tab === t.status" :disabled="!t.n && tab !== t.status" @click="tab = t.status">{{ t.label }} <span class="count">{{ t.n }}</span></button>
        </div>
        <span class="spacer"></span>
        <select v-model="sort" class="input" :aria-label="$t('Sortieren nach')">
          <option value="recent">{{ $t('Zuletzt aktiv') }}</option>
          <option value="name">{{ $t('Name') }}</option>
        </select>
        <span class="seg" role="group" :aria-label="$t('Ansicht')">
          <button type="button" :aria-label="$t('Kacheln')" :aria-pressed="layout === 'grid'" @click="setLayout('grid')"><svg class="ic"><use href="#i-grid" /></svg></button>
          <button type="button" :aria-label="$t('Liste')" :aria-pressed="layout === 'list'" @click="setLayout('list')"><svg class="ic"><use href="#i-list" /></svg></button>
        </span>
        <SearchField v-model="search" :label="$t('Projekte durchsuchen')" />
      </div>

      <div v-if="!projects.length" class="empty-cta"><p class="v-sub">{{ $t('Noch keine Projekte. Ein Projekt bündelt Aufgaben, Notizen und die Zeit dafür.') }}</p><button class="btn" type="button" @click="openNewDialog()">{{ $t('Erstes Projekt anlegen') }}</button></div>
      <div v-else-if="!list.length" class="v-sub">{{ search ? $t('Keine Projekte für „{q}“ in „{tab}“.', { q: search, tab: STATUS[tab] ?? '' }) : $t('Keine Projekte in „{tab}“.', { tab: STATUS[tab] ?? '' }) }}</div>
      <div v-else class="proj-list" :class="layout">
        <article
          v-for="p in list"
          :key="p.id"
          :data-project="p.id"
          class="proj-card"
          :class="{ selected: openId === p.id }"
          :style="{ '--pc': projectColor(p.id) }"
          tabindex="0"
          role="button"
          :aria-expanded="openId === p.id"
          @click="openId = openId === p.id ? null : p.id"
          @keydown.enter.self="openId = openId === p.id ? null : p.id"
        >
          <div class="proj-head">
            <svg class="ic proj-ic" aria-hidden="true"><use :href="`#${projectIconOf(p)}`" /></svg>
            <div class="proj-title-wrap">
              <div class="proj-name">{{ p.name }}</div>
              <span v-if="lastActive(p.id)" class="proj-last">{{ $t('aktiv {when}', { when: lastActive(p.id) }) }}</span>
            </div>
            <button
              v-if="p.local_path"
              class="icon-btn act-code"
              type="button"
              :title="$t('In VSCodium öffnen: {name}', { name: p.name })"
              :aria-label="$t('In VSCodium öffnen: {name}', { name: p.name })"
              @click.stop="inCode(p)"
            >
              <svg class="ic"><use href="#i-external" /></svg>
            </button>
          </div>

          <div v-if="p.description" class="proj-desc">{{ p.description }}</div>

          <div class="proj-foot">
            <span v-if="frac(p.id)" class="proj-stat"><b class="mono">{{ frac(p.id) }}</b> {{ $t('erledigt') }}</span>
            <span v-else class="proj-stat"><b class="mono">{{ openTasks(p.id).length }}</b> {{ $t('offen') }}</span>
            <span v-if="issueCount[p.id]" class="proj-stat issues"><b class="mono">{{ issueCount[p.id] }}</b> {{ $t('GitHub-Issues') }}</span>
            <span v-if="minutes[p.id]?.week" class="proj-stat"><b class="mono">{{ hm(minutes[p.id]!.week) }}</b> {{ $t('diese Woche') }}</span>
          </div>
        </article>
      </div>
    </div>

    <!-- RECHTES DETAIL-PANEL -->
    <aside v-if="open" class="project-aside" role="region" :aria-label="$t('Projekt-Details')" :style="{ '--pc': projectColor(open.id) }">
      <div class="aside-head">
        <div class="aside-status-wrap">
          <select class="input status-select" :value="open.status" :aria-label="$t('Status')" @change="(e) => setField(open!, { status: valueOf(e) })">
            <option v-for="(l, v) in STATUS" :key="v" :value="v">{{ l }}</option>
          </select>
        </div>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="openId = null">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="proj-detail-body">
        <div class="pd-hero">
          <button
            type="button"
            class="pd-icon-btn"
            :title="$t('Symbol ändern')"
            :aria-label="$t('Symbol ändern')"
            @click="showIconPicker = !showIconPicker"
          >
            <svg class="ic" aria-hidden="true"><use :href="`#${projectIconOf(open)}`" /></svg>
          </button>
          <div class="pd-hero-text">
            <input
              class="td-title"
              :value="open.name"
              :placeholder="$t('Projektname')"
              :aria-label="$t('Projektname')"
              @change="(e) => setField(open!, { name: valueOf(e).trim() || open!.name })"
            />
          </div>
        </div>

        <!-- SYMBOL-AUSWAHL POPOVER -->
        <div v-if="showIconPicker" class="pd-icon-popover">
          <div class="icon-picker-head">
            <span class="lbl">{{ $t('Symbol wählen') }}</span>
            <button type="button" class="btn btn-ghost btn-xs" @click="setField(open!, { icon: '' }); showIconPicker = false">
              {{ $t('Automatisch') }}
            </button>
          </div>
          <div class="icon-picker-grid">
            <button
              v-for="[k, label] in PROJECT_ICON_OPTIONS"
              :key="k"
              type="button"
              class="icon-pick-cell"
              :class="{ active: (open.icon || projectIconKey(open)) === k }"
              :title="label"
              @click="setField(open!, { icon: k }); showIconPicker = false"
            >
              <svg class="ic"><use :href="`#${PROJECT_ICONS[k]}`" /></svg>
            </button>
          </div>
        </div>

        <textarea
          class="td-desc"
          rows="2"
          :placeholder="$t('Beschreibung hinzufügen')"
          :value="open.description"
          @change="(e) => setField(open!, { description: valueOf(e) })"
        ></textarea>

        <button
          v-if="open.local_path"
          class="btn btn-secondary vsc-btn"
          type="button"
          @click="inCode(open)"
        >
          <svg class="ic"><use href="#i-external" /></svg>
          {{ $t('In VSCodium öffnen') }}
        </button>

        <!-- EIGENSCHAFTEN -->
        <div class="td-props">
          <label class="td-row">
            <span>{{ $t('Symbol') }}</span>
            <select class="input" :value="open.icon ?? ''" @change="(e) => setField(open!, { icon: valueOf(e) })">
              <option value="">{{ $t('Automatisch') }}</option>
              <option v-for="[k, label] in PROJECT_ICON_OPTIONS" :key="k" :value="k">{{ label }}</option>
            </select>
          </label>
          <div class="td-row color-row">
            <span>{{ $t('Farbe') }}</span>
            <div class="swatches" role="radiogroup" :aria-label="$t('Farbe')">
              <label v-for="[c, label] in PROJECT_COLORS" :key="c" class="swatch" :title="$t(label)">
                <input
                  type="radio"
                  name="aside-p-color"
                  :value="c"
                  :checked="open.color === c"
                  :aria-label="label"
                  @change="setField(open!, { color: c })"
                />
                <span :style="{ background: `var(--p-${c})` }"></span>
              </label>
            </div>
          </div>
          <label class="td-row">
            <span>{{ $t('Ordner') }}</span>
            <input
              class="input mono"
              :value="open.local_path ?? ''"
              :placeholder="$t('Absoluter Pfad oder ~/… auf deinem Rechner (optional)')"
              @change="(e) => setField(open!, { local_path: valueOf(e).trim() || null })"
            />
          </label>
          <label class="td-row">
            <span>{{ $t('GitHub-Repo') }}</span>
            <input
              class="input mono"
              :value="open.github_repo ?? ''"
              :placeholder="$t('owner/name, sonst aus dem Ordner (optional)')"
              @change="(e) => setField(open!, { github_repo: valueOf(e).trim() || null })"
            />
          </label>
        </div>

        <!-- ZEITERFASSUNG -->
        <div class="td-sec">
          <div class="pd-time-meta">
            {{ $t('Erfasst: {week} diese Woche · {total} gesamt', { week: dur(minutes[open.id]?.week), total: dur(minutes[open.id]?.total) }) }}
            <template v-if="frac(open.id)"> · {{ $t('{n} Aufgaben erledigt', { n: frac(open.id) }) }}</template>
          </div>
        </div>

        <!-- OFFENE AUFGABEN -->
        <div class="td-sec sub-sec">
          <span class="lbl">{{ $t('Offene Aufgaben') }}<template v-if="openTasks(open.id).length"> · {{ openTasks(open.id).length }}</template></span>
          <div v-if="openTasks(open.id).length" class="task-rows">
            <div v-for="t in openTasks(open.id)" :key="t.id" class="sub-row" :class="{ done: !isOpen(t) }">
              <button
                type="button"
                class="cb"
                role="checkbox"
                :aria-checked="t.status === 'COMPLETED'"
                :aria-label="$t('{title} erledigt', { title: t.title })"
                :disabled="t.status === 'CANCELLED'"
                @click="toggleTask(t)"
              ></button>
              <button type="button" class="sub-task-title" @click="router.push({ path: '/tasks', query: { task: t.id } })">
                {{ t.title }}
              </button>
              <component :is="href(r) ? 'a' : 'span'" v-for="r in taskRes(t.id)" :key="r.id" class="res-mini" :href="href(r)" :target="r.type === 'URL' ? '_blank' : undefined" rel="noopener">
                <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>
              </component>
              <TaskActions class="row-act" :task="t" :running="t.status === 'IN_PROGRESS'" :note="taskRes(t.id).find((r) => r.type === 'NOTE')?.target" @run="run" />
            </div>
          </div>
          <div v-else class="pd-empty-note">{{ $t('Keine offenen Aufgaben für dieses Projekt.') }}</div>
          <form class="addrow" @submit.prevent="addTaskForProject(open)">
            <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
            <input v-model="newTaskTitle" type="text" :placeholder="$t('Aufgabe für dieses Projekt hinzufügen')" :aria-label="$t('Aufgabe für dieses Projekt hinzufügen')" />
          </form>
        </div>

        <!-- GITHUB-ISSUES -->
        <GitHubIssues
          v-if="open.local_path || open.github_repo"
          class="td-sec"
          :project-id="open.id"
          @open="(id) => router.push({ path: '/tasks', query: { task: id } })"
          @changed="load"
        />

        <!-- RESSOURCEN -->
        <ResourceList
          class="td-sec"
          :resources="resOf(open.id)"
          :project-id="open.id"
          @changed="load"
        />

        <!-- FOOTER / LÖSCHEN -->
        <div class="td-foot">
          <span></span>
          <DeleteButton
            :text="$t('„{name}“ löschen? Die Tasks bleiben ohne Projekt, ihre Zeiteinträge landen in „Sonstiges“.', { name: open.name })"
            @confirm="deleteCurrentProject(open)"
          />
        </div>
      </div>
    </aside>

    <!-- DIALOG FÜR NEUES PROJEKT -->
    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <div class="dialog" :aria-label="$t('Neues Projekt')">
        <form @submit.prevent="saveNew">
          <div class="dlg-head">
            <h3>{{ $t('Neues Projekt') }}</h3>
            <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
          </div>
          <div class="dlg-body">
            <div class="field"><label for="p-name">{{ $t('Name') }}</label><input id="p-name" v-model="form.name" class="input" :placeholder="$t('Projektname')" autofocus /></div>
            <div class="field"><label for="p-desc">{{ $t('Beschreibung') }}</label><input id="p-desc" v-model="form.description" class="input" /></div>
            <div v-if="error" class="badge" role="alert">{{ error }}</div>
            <div class="field"><label for="p-path">{{ $t('Ordner') }}</label><input id="p-path" v-model="form.local_path" class="input" :placeholder="$t('Absoluter Pfad oder ~/… auf deinem Rechner (optional)')" /></div>
            <div class="field"><label for="p-repo">{{ $t('GitHub-Repo') }}</label><input id="p-repo" v-model="form.github_repo" class="input" :placeholder="$t('owner/name, sonst aus dem Ordner (optional)')" /></div>
            <div class="field">
              <label for="p-icon">{{ $t('Symbol') }}</label>
              <select id="p-icon" v-model="form.icon" class="input">
                <option value="">{{ $t('Automatisch') }}</option>
                <option v-for="[k, label] in PROJECT_ICON_OPTIONS" :key="k" :value="k">{{ label }}</option>
              </select>
            </div>
            <div class="field">
              <label id="p-color-l">{{ $t('Farbe') }}</label>
              <div class="swatches" role="radiogroup" aria-labelledby="p-color-l">
                <label v-for="[c, label] in PROJECT_COLORS" :key="c" class="swatch" :title="$t(label)">
                  <input v-model="form.color" type="radio" name="p-color" :value="c" :aria-label="label" /><span :style="{ background: `var(--p-${c})` }"></span>
                </label>
              </div>
            </div>
          </div>
          <div class="dlg-foot">
            <span class="spacer"></span>
            <button class="btn btn-ghost" type="button" @click="dialog = false">{{ $t('Abbrechen') }}</button>
            <button class="btn btn-primary" type="submit">{{ $t('Anlegen') }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.spacer { flex: 1; }
.swatches { display: flex; flex-wrap: wrap; gap: 8px; }
.swatch { position: relative; display: inline-flex; cursor: pointer; }
.swatch input { position: absolute; inset: 0; opacity: 0; margin: 0; cursor: pointer; }
.swatch span { width: 22px; height: 22px; border-radius: 50%; border: 2px solid transparent; box-shadow: inset 0 0 0 2px var(--bg-1); }
.swatch input:checked + span { border-color: var(--tx-primary); }
.swatch input:focus-visible + span { outline: 2px solid var(--tx-primary); outline-offset: 2px; }

/* Kacheln (Richtung 2 & 3 Mix) */
.proj-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(290px, 1fr)); gap: 16px; margin-top: 16px; }
.proj-list.list { grid-template-columns: minmax(0, 1fr); }

.proj-card {
  display: flex;
  flex-direction: column;
  padding: 16px 18px;
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  border-radius: var(--r-m);
  cursor: pointer;
  transition: background var(--dur) ease-out, border-color var(--dur) ease-out, box-shadow var(--dur) ease-out;
  outline: none;
}
.proj-card:hover {
  background: var(--bg-hover, var(--bg-2));
  border-color: var(--br-strong);
}
.proj-card.selected {
  background: var(--bg-2);
  border-color: var(--br-strong);
  box-shadow: inset 0 0 0 1px var(--br-strong);
}
.proj-card:focus-visible {
  box-shadow: 0 0 0 2px var(--a-focus);
}

.proj-head { display: flex; align-items: center; gap: 10px; }
.proj-ic {
  width: 17px;
  height: 17px;
  flex: none;
  color: var(--pc, var(--tx-secondary));
}

.pd-icon-btn {
  display: inline-grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: var(--r-s);
  border: 1px solid transparent;
  background: none;
  color: var(--pc, var(--tx-primary));
  cursor: pointer;
  transition: background var(--dur), border-color var(--dur);
}
.pd-icon-btn:hover {
  background: var(--bg-2);
  border-color: var(--br-default);
}
.pd-icon-btn .ic { width: 20px; height: 20px; }

.pd-icon-popover {
  margin: 10px 0 6px;
  padding: 12px;
  background: var(--bg-2);
  border: 1px solid var(--br-default);
  border-radius: var(--r-s);
}
.icon-picker-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.icon-picker-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 4px;
}
.icon-pick-cell {
  display: inline-grid;
  place-items: center;
  height: 34px;
  border-radius: var(--r-s);
  border: 1px solid transparent;
  background: none;
  color: var(--tx-secondary);
  cursor: pointer;
}
.icon-pick-cell:hover {
  background: var(--bg-3);
  color: var(--tx-primary);
}
.icon-pick-cell.active {
  background: var(--bg-3);
  border-color: var(--br-strong);
  color: var(--pc, var(--tx-primary));
}
.icon-pick-cell .ic { width: 17px; height: 17px; }

.proj-title-wrap { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.proj-name {
  font: 600 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.proj-last { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }

.act-code {
  opacity: 0;
  transition: opacity var(--dur) ease-out;
}
.proj-card:hover .act-code, .proj-card:focus-within .act-code {
  opacity: 1;
}

.proj-desc {
  font: 400 13px/1.5 var(--font-ui);
  color: var(--tx-secondary);
  margin-top: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.proj-foot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 14px;
  margin-top: auto;
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.proj-stat b { font-weight: 500; color: var(--tx-primary); }
.proj-stat.issues { color: var(--a-orange); }
.proj-stat.issues b { color: var(--a-orange); }

/* Master-Detail Split Layout */
@media (min-width: 1001px) {
  .view-inner.proj-split { display: grid; grid-template-columns: minmax(0, 1fr) 380px; max-width: none; padding: 0; }
  .proj-split .proj-left { min-width: 0; padding: 32px 40px 64px 32px; }
  .proj-split .project-aside { position: sticky; top: 56px; align-self: start; height: calc(100dvh - 56px); }
}

.project-aside {
  display: flex;
  flex-direction: column;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  overflow-y: auto;
  scrollbar-width: none;
}
.project-aside::-webkit-scrollbar { display: none; }

.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 0 24px;
}
.status-select {
  height: 26px;
  padding: 0 10px;
  font: 500 12px/1 var(--font-ui);
  border-radius: var(--r-s);
  background: var(--bg-2);
  border: 1px solid var(--br-default);
  cursor: pointer;
}

.proj-detail-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 12px 24px 24px;
}

.pd-hero { display: flex; align-items: center; gap: 12px; margin-top: 6px; }
.pd-hero-text { flex: 1; min-width: 0; }

.td-title {
  width: 100%;
  padding: 4px 0;
  border: 0;
  background: none;
  color: var(--tx-primary);
  font: 600 20px/1.3 var(--font-ui);
  letter-spacing: -.015em;
}
.td-title:focus-visible { outline: none; box-shadow: 0 1px 0 var(--a-focus); }

.td-desc {
  width: 100%;
  padding: 8px;
  margin: 10px -8px 0;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--tx-secondary);
  font: 400 13px/1.55 var(--font-ui);
  resize: vertical;
}
.td-desc:hover, .td-desc:focus-visible { background: var(--bg-2); }

.vsc-btn {
  width: 100%;
  margin-top: 14px;
  gap: 8px;
  justify-content: center;
}

.td-props { margin-top: 18px; border-top: 1px solid var(--br-subtle); }
.td-row { display: flex; align-items: center; gap: 8px; height: 40px; border-bottom: 1px solid var(--br-subtle); font: 400 13px/1 var(--font-ui); }
.td-row > span { flex: none; width: 100px; color: var(--tx-muted); }
.td-row .input {
  flex: 1;
  min-width: 0;
  width: auto;
  height: 28px;
  padding: 0 6px;
  border: 0;
  background-color: transparent;
  color: var(--tx-primary);
  font: 400 13px/1 var(--font-ui);
  border-radius: 6px;
}
.td-row .input:hover, .td-row .input:focus-visible { background-color: var(--bg-2); }
.td-row.color-row { height: auto; padding: 10px 0; }

.td-sec { display: block; margin-top: 20px; }
.td-sec > .lbl, .sub-sec > .lbl { display: block; margin-bottom: 8px; font: 600 11px/1 var(--font-ui); letter-spacing: .08em; text-transform: uppercase; color: var(--tx-muted); }

.pd-time-meta { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }
.pd-empty-note { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); padding: 4px 0 8px; }

/* Sub-Tasks */
.task-rows { display: flex; flex-direction: column; gap: 2px; }
.sub-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 4px 6px;
  border-radius: var(--r-s);
  transition: background var(--dur);
}
.sub-row:hover { background: var(--bg-2); }
.sub-row.done .sub-task-title { color: var(--tx-muted); text-decoration: line-through; }
.sub-row .row-act { opacity: 0; margin-left: auto; transition: opacity var(--dur) ease-out; }
.sub-row:hover .row-act, .sub-row:focus-within .row-act { opacity: 1; }
.sub-row .row-act :deep(.btn) { height: 24px; padding: 0 7px; font-size: 11px; border-radius: 4px; }
.sub-task-title {
  flex: 1;
  min-width: 0;
  text-align: left;
  background: none;
  border: 0;
  color: var(--tx-primary);
  font: 400 13px/1.4 var(--font-ui);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub-task-title:hover { color: var(--a-blue); }
.res-mini {
  display: inline-flex;
  align-items: center;
  color: var(--tx-muted);
  padding: 2px 4px;
}
.res-mini:hover { color: var(--tx-primary); }

.addrow {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  padding: 6px 8px;
  border-radius: var(--r-s);
  color: var(--tx-muted);
}
.addrow:focus-within { background: var(--bg-2); color: var(--tx-primary); }
.addrow input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  color: var(--tx-primary);
  font: 400 13px/1.4 var(--font-ui);
  outline: none;
}
.addrow input::placeholder { color: var(--tx-muted); }

.td-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 20px;
  border-top: 1px solid var(--br-subtle);
}

@media (max-width: 1000px) {
  .project-aside {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 20;
    max-height: 70vh;
    border: 0;
    border-radius: var(--r-m) var(--r-m) 0 0;
    box-shadow: var(--shadow-pop);
  }
}
@media (max-width: 760px) {
  .project-aside {
    bottom: calc(56px + env(safe-area-inset-bottom));
    z-index: 55;
    max-height: calc(100dvh - 56px - env(safe-area-inset-bottom) - 48px);
  }
}
</style>
