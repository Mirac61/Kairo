<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  api,
  createProject,
  createTask,
  deleteProject,
  listProjects,
  listResources,
  listTasks,
  isOpen,
  updateProject,
  updateTask,
  type Project,
  type Resource,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { projectColor } from '@/lib/projectColor'
import { projectIconOf } from '@/lib/projectIcons'
import { locale, t } from '@/lib/i18n'
import { daysAgo, hm, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import SearchField from '@/components/SearchField.vue'
import ProjectDetailAside from '@/components/ProjectDetailAside.vue'
import ProjectCreateDialog from '@/components/ProjectCreateDialog.vue'

const STATUS: Record<string, string> = {
  ACTIVE: t('Aktiv'),
  PAUSED: t('Pausiert'),
  COMPLETED: t('Abgeschlossen'),
  ARCHIVED: t('Archiviert'),
}

interface Progress {
  project_id: string | null
  done_tasks: number
  total_tasks: number
  tracked_minutes: number
}

interface ProjectStat {
  project_id: string
  week_minutes: number
  total_minutes: number
  last_active_at: string
}

const route = useRoute()
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const tasks = ref<Task[]>([])
const minutes = ref<Record<string, { week: number; total: number; last: number }>>({})
const openId = ref<string | null>(null)
const progress = ref<Record<string, Progress>>({})
const isCreateDialogOpen = ref(false)

const openProject = computed(() => projects.value.find((p) => p.id === openId.value))
const openTasks = (id: string) => tasks.value.filter((t) => t.project_id === id && isOpen(t))

const { error, load, run } = useLoader(async () => {
  const today = ymd(new Date())
  const [ps, rs, rv, ts, es] = await Promise.all([
    listProjects(),
    listResources(),
    api<{ projects: Progress[] }>(`/review?from=${today}&to=${today}`),
    listTasks(),
    api<ProjectStat[]>('/projects/stats'),
  ])
  projects.value = ps
  resources.value = rs
  tasks.value = ts
  minutes.value = Object.fromEntries(
    es.map((s) => [
      s.project_id,
      { week: s.week_minutes, total: s.total_minutes, last: Date.parse(s.last_active_at) },
    ])
  )
  progress.value = Object.fromEntries(
    rv.projects.filter((p) => p.project_id).map((p) => [p.project_id!, p])
  )
})

function openNewDialog() {
  error.value = ''
  isCreateDialogOpen.value = true
}

async function handleCreateProject(payload: Parameters<typeof createProject>[0]) {
  let created: Project | undefined
  const ok = await run(async () => {
    created = await createProject(payload)
  })
  if (ok && created) {
    isCreateDialogOpen.value = false
    await load()
    openId.value = created.id
  }
}

async function handleSetField(p: Project, patch: Partial<Project>) {
  const updated = await run(() => updateProject(p.id, patch))
  if (updated) {
    Object.assign(p, updated)
  }
}

async function handleToggleTask(t: Task) {
  const next = t.status === 'COMPLETED' ? 'PLANNED' : 'COMPLETED'
  await run(() => updateTask(t.id, { status: next }))
  await load()
}

async function handleAddTask(p: Project, title: string) {
  await run(() => createTask({ title, project_id: p.id, status: 'PLANNED' }))
  await load()
}

async function handleDeleteCurrentProject(p: Project) {
  const ok = await run(() => deleteProject(p.id))
  if (ok) {
    openId.value = null
    await load()
  }
}

const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q
    ? projects.value.filter((p) =>
        `${p.name} ${p.description} ${p.local_path ?? ''}`.toLowerCase().includes(q)
      )
    : projects.value
})

const LAYOUT_KEY = 'kairo-projects-layout'
const tab = ref('ACTIVE')
const sort = ref<'recent' | 'name'>('recent')
const layout = ref<'grid' | 'list'>('grid')

if (store.get(LAYOUT_KEY) === 'list') layout.value = 'list'
function setLayout(l: 'grid' | 'list') {
  layout.value = l
  store.set(LAYOUT_KEY, l)
}

const tabs = computed(() =>
  Object.entries(STATUS).map(([status, label]) => ({
    status,
    label,
    n: shown.value.filter((p) => p.status === status).length,
  }))
)

const list = computed(() =>
  shown.value
    .filter((p) => p.status === tab.value)
    .sort((a, b) =>
      sort.value === 'name'
        ? a.name.localeCompare(b.name, locale.value)
        : (minutes.value[b.id]?.last ?? 0) - (minutes.value[a.id]?.last ?? 0)
    )
)

const lastActive = (id: string) => {
  const t = minutes.value[id]?.last
  return t ? daysAgo(ymd(new Date(t)), ymd(new Date())) : ''
}

const fractionText = (id: string) => {
  const p = progress.value[id]
  return p?.total_tasks ? `${p.done_tasks}/${p.total_tasks}` : ''
}

// Die Extension öffnet den Workspace des Projekts.
const inCode = (p: Project) => {
  window.location.href = `vscodium://kairo-local.kairo/open?project=${p.id}`
}

// Offene GitHub-Issues je Projekt
const issueCount = ref<Record<string, number>>({})
const loadIssueCounts = () =>
  Promise.all(
    projects.value
      .filter((p) => p.local_path)
      .map(async (p) => {
        try {
          const r = await api<{ repo: string; issues: unknown[] }>(
            `/projects/${p.id}/github/issues`
          )
          if (r.repo) issueCount.value[p.id] = r.issues.length
        } catch {
          // kein gh oder GitHub nicht erreichbar
        }
      })
  )

function focusFromRoute() {
  const p = projects.value.find((x) => x.id === route.query.project)
  if (!p) return
  tab.value = p.status
  search.value = ''
  openId.value = p.id
  void nextTick(() =>
    document.querySelector(`[data-project="${p.id}"]`)?.scrollIntoView({ block: 'center' })
  )
}

watch(() => route.query.project, focusFromRoute)

onMounted(() =>
  void load().then(() => {
    focusFromRoute()
    return loadIssueCounts()
  })
)
useLiveEvents(load)

function moveSelection(direction: number) {
  const l = list.value
  if (!l.length) return
  const i = l.findIndex((p) => p.id === openId.value)
  const next =
    l[
      i < 0
        ? direction > 0
          ? 0
          : l.length - 1
        : Math.min(Math.max(i + direction, 0), l.length - 1)
    ]!
  openId.value = next.id
  void nextTick(() =>
    document
      .querySelector(`[data-project="${next.id}"]`)
      ?.scrollIntoView({ block: 'nearest' })
  )
}

useShortcuts((e) => {
  const el = e.target as HTMLElement
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') return
  if (e.key === 'j') moveSelection(1)
  else if (e.key === 'k') moveSelection(-1)
  else if (e.key === 'Escape') openId.value = null
})
</script>

<template>
  <div class="view-inner" :class="{ 'proj-split': !!openProject }">
    <div class="proj-left">
      <div class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ $t('Projekte') }}</h1>
          <div class="v-sub">{{ $t('Fortschritt und verknüpfte Ressourcen.') }}</div>
        </div>
        <button class="btn btn-primary" type="button" @click="openNewDialog()">
          <svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neues Projekt') }}
        </button>
      </div>

      <div v-if="error && !isCreateDialogOpen" class="badge" role="alert">{{ error }}</div>

      <div class="filterbar">
        <div class="seg" role="group" :aria-label="$t('Filter')">
          <button
            v-for="t in tabs"
            :key="t.status"
            type="button"
            :aria-pressed="tab === t.status"
            :disabled="!t.n && tab !== t.status"
            @click="tab = t.status"
          >
            {{ t.label }} <span class="count">{{ t.n }}</span>
          </button>
        </div>

        <span class="spacer"></span>

        <select v-model="sort" class="input" :aria-label="$t('Sortieren nach')">
          <option value="recent">{{ $t('Zuletzt aktiv') }}</option>
          <option value="name">{{ $t('Name') }}</option>
        </select>

        <span class="seg" role="group" :aria-label="$t('Ansicht')">
          <button
            type="button"
            :aria-label="$t('Kacheln')"
            :aria-pressed="layout === 'grid'"
            @click="setLayout('grid')"
          >
            <svg class="ic"><use href="#i-grid" /></svg>
          </button>
          <button
            type="button"
            :aria-label="$t('Liste')"
            :aria-pressed="layout === 'list'"
            @click="setLayout('list')"
          >
            <svg class="ic"><use href="#i-list" /></svg>
          </button>
        </span>

        <SearchField v-model="search" :label="$t('Projekte durchsuchen')" />
      </div>

      <div v-if="!projects.length" class="empty-cta">
        <p class="v-sub">
          {{ $t('Noch keine Projekte. Ein Projekt bündelt Aufgaben, Notizen und die Zeit dafür.') }}
        </p>
        <button class="btn" type="button" @click="openNewDialog()">
          {{ $t('Erstes Projekt anlegen') }}
        </button>
      </div>

      <div v-else-if="!list.length" class="v-sub">
        {{
          search
            ? $t('Keine Projekte für „{q}“ in „{tab}“.', { q: search, tab: STATUS[tab] ?? '' })
            : $t('Keine Projekte in „{tab}“.', { tab: STATUS[tab] ?? '' })
        }}
      </div>

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
              <span v-if="lastActive(p.id)" class="proj-last">
                {{ $t('aktiv {when}', { when: lastActive(p.id) }) }}
              </span>
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
            <span v-if="fractionText(p.id)" class="proj-stat">
              <b class="mono">{{ fractionText(p.id) }}</b> {{ $t('erledigt') }}
            </span>
            <span v-else class="proj-stat">
              <b class="mono">{{ openTasks(p.id).length }}</b> {{ $t('offen') }}
            </span>
            <span v-if="issueCount[p.id]" class="proj-stat issues">
              <b class="mono">{{ issueCount[p.id] }}</b> {{ $t('GitHub-Issues') }}
            </span>
            <span v-if="minutes[p.id]?.week" class="proj-stat">
              <b class="mono">{{ hm(minutes[p.id]!.week) }}</b> {{ $t('diese Woche') }}
            </span>
          </div>
        </article>
      </div>
    </div>

    <!-- RECHTES DETAIL-PANEL -->
    <ProjectDetailAside
      v-if="openProject"
      :project="openProject"
      :resources="resources"
      :tasks="tasks"
      :week-minutes="minutes[openProject.id]?.week"
      :total-minutes="minutes[openProject.id]?.total"
      :fraction-text="fractionText(openProject.id)"
      @close="openId = null"
      @set-field="(patch) => handleSetField(openProject!, patch)"
      @add-task="(title) => handleAddTask(openProject!, title)"
      @toggle-task="handleToggleTask"
      @delete-project="handleDeleteCurrentProject(openProject!)"
      @reload="load"
      @run="run"
    />

    <!-- DIALOG FÜR NEUES PROJEKT -->
    <ProjectCreateDialog
      :open="isCreateDialogOpen"
      :error="error"
      @close="isCreateDialogOpen = false"
      @save="handleCreateProject"
    />
  </div>
</template>

<style scoped>
.spacer {
  flex: 1;
}

/* Kacheln */
.proj-list.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.proj-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-m);
  cursor: pointer;
  transition:
    transform var(--dur) ease-out,
    border-color var(--dur) ease-out,
    box-shadow var(--dur) ease-out;
}
.proj-card:hover {
  border-color: var(--br-default);
  box-shadow: var(--shadow-sm);
}
.proj-card.selected {
  border-color: var(--pc);
  background: color-mix(in srgb, var(--pc) 4%, var(--bg-1));
}

.proj-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.proj-ic {
  flex: none;
  width: 20px;
  height: 20px;
  color: var(--pc);
}
.proj-title-wrap {
  flex: 1;
  min-width: 0;
  display: grid;
  gap: 2px;
}
.proj-name {
  font: 600 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.proj-last {
  font: 400 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.act-code {
  flex: none;
  width: 26px;
  height: 26px;
  color: var(--tx-muted);
}
.act-code:hover {
  color: var(--tx-primary);
}
.act-code .ic {
  width: 14px;
  height: 14px;
}

.proj-desc {
  font: 400 13px/1.45 var(--font-ui);
  color: var(--tx-secondary);
  margin-bottom: 12px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.proj-foot {
  margin-top: auto;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.proj-stat {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
}
.proj-stat b {
  color: var(--tx-primary);
  font-weight: 500;
}
.proj-stat.issues b {
  color: var(--p-orange);
}

/* Liste */
.proj-list.list {
  display: grid;
  gap: 6px;
}
.proj-list.list .proj-card {
  flex-direction: row;
  align-items: center;
  gap: 16px;
  padding: 10px 16px;
}
.proj-list.list .proj-head {
  margin-bottom: 0;
  flex: 1;
  min-width: 0;
}
.proj-list.list .proj-desc {
  display: none;
}
.proj-list.list .proj-foot {
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
  flex: none;
}

@media (min-width: 1001px) {
  .view-inner.proj-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 380px;
    max-width: none;
    padding: 0;
  }
  .proj-split .proj-left {
    min-width: 0;
    padding: 32px 40px 64px 32px;
  }
  .proj-split :deep(.project-aside) {
    position: sticky;
    top: 56px;
    align-self: start;
    height: calc(100dvh - 56px);
  }
}
</style>
