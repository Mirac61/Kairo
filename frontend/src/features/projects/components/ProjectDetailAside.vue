<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { isOpen, type Project, type Resource, type Task } from '@/api/client'
import { projectColor, PROJECT_COLORS } from '@/lib/projectColor'
import {
  PROJECT_ICONS,
  PROJECT_ICON_OPTIONS,
  projectIconKey,
  projectIconOf,
} from '@/lib/projectIcons'
import { dur } from '@/lib/dates'
import { t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'
import GitHubIssues from '@/components/GitHubIssues.vue'
import ResourceList from '@/components/ResourceList.vue'
import TaskActions from '@/features/tasks/components/TaskActions.vue'

const props = defineProps<{
  project: Project
  resources: Resource[]
  tasks: Task[]
  weekMinutes?: number
  totalMinutes?: number
  fractionText?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'setField', patch: Partial<Project>): void
  (e: 'addTask', title: string): void
  (e: 'toggleTask', task: Task): void
  (e: 'deleteProject'): void
  (e: 'reload'): void
  (e: 'run', action: () => Promise<unknown>): void
}>()

const router = useRouter()

const STATUS: Record<string, string> = {
  ACTIVE: t('Aktiv'),
  PAUSED: t('Pausiert'),
  COMPLETED: t('Abgeschlossen'),
  ARCHIVED: t('Archiviert'),
}

const RES_ICON = {
  URL: 'i-link',
  FILE: 'i-file',
  FOLDER: 'i-file',
  NOTE: 'i-file',
} as const

const showIconPicker = ref(false)
const newTaskTitle = ref('')

function resetIcon() {
  emit('setField', { icon: '' })
  showIconPicker.value = false
}

function selectIcon(iconKey: string) {
  emit('setField', { icon: iconKey })
  showIconPicker.value = false
}

watch(
  () => props.project.id,
  () => {
    showIconPicker.value = false
    newTaskTitle.value = ''
  },
)

const openTasks = computed(() =>
  props.tasks.filter((t) => t.project_id === props.project.id && isOpen(t)),
)

const projectResources = computed(() =>
  props.resources.filter((r) => r.project_id === props.project.id),
)

function getTaskResources(taskId: string): Resource[] {
  return props.resources.filter((r) => r.task_id === taskId)
}

function resourceHref(r: Resource): string | undefined {
  if (r.type === 'URL') return r.target
  if (r.type === 'NOTE') return `/notes?open=${encodeURIComponent(r.target)}`
  return undefined
}

function openInVSCodium() {
  window.location.href = `vscodium://kairo-local.kairo/open?project=${props.project.id}`
}

function valueOf(e: Event): string {
  return (e.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value
}

function handleAddTask() {
  const title = newTaskTitle.value.trim()
  if (!title) return
  emit('addTask', title)
  newTaskTitle.value = ''
}
</script>

<template>
  <aside
    class="project-aside"
    role="region"
    :aria-label="$t('Projekt-Details')"
    :style="{ '--pc': projectColor(project.id) }"
  >
    <div class="aside-head">
      <div class="aside-status-wrap">
        <select
          class="input status-select"
          :value="project.status"
          :aria-label="$t('Status')"
          @change="(e) => emit('setField', { status: valueOf(e) })"
        >
          <option v-for="(label, val) in STATUS" :key="val" :value="val">{{ label }}</option>
        </select>
      </div>
      <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
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
          <svg class="ic" aria-hidden="true"><use :href="`#${projectIconOf(project)}`" /></svg>
        </button>
        <div class="pd-hero-text">
          <input
            class="td-title"
            :value="project.name"
            :placeholder="$t('Projektname')"
            :aria-label="$t('Projektname')"
            @change="(e) => emit('setField', { name: valueOf(e).trim() || project.name })"
          />
        </div>
      </div>

      <!-- SYMBOL-AUSWAHL POPOVER -->
      <div v-if="showIconPicker" class="pd-icon-popover">
        <div class="icon-picker-head">
          <span class="lbl">{{ $t('Symbol wählen') }}</span>
          <button type="button" class="btn btn-ghost btn-xs" @click="resetIcon">
            {{ $t('Automatisch') }}
          </button>
        </div>
        <div class="icon-picker-grid">
          <button
            v-for="[k, label] in PROJECT_ICON_OPTIONS"
            :key="k"
            type="button"
            class="icon-pick-cell"
            :class="{ active: (project.icon || projectIconKey(project)) === k }"
            :title="label"
            @click="selectIcon(k)"
          >
            <svg class="ic"><use :href="`#${PROJECT_ICONS[k]}`" /></svg>
          </button>
        </div>
      </div>

      <textarea
        class="td-desc"
        rows="2"
        :placeholder="$t('Beschreibung hinzufügen')"
        :value="project.description"
        @change="(e) => emit('setField', { description: valueOf(e) })"
      ></textarea>

      <button
        v-if="project.local_path"
        class="btn btn-secondary vsc-btn"
        type="button"
        @click="openInVSCodium"
      >
        <svg class="ic"><use href="#i-external" /></svg>
        {{ $t('In VSCodium öffnen') }}
      </button>

      <!-- EIGENSCHAFTEN -->
      <div class="td-props">
        <label class="td-row">
          <span>{{ $t('Symbol') }}</span>
          <select
            class="input"
            :value="project.icon ?? ''"
            @change="(e) => emit('setField', { icon: valueOf(e) })"
          >
            <option value="">{{ $t('Automatisch') }}</option>
            <option v-for="[k, label] in PROJECT_ICON_OPTIONS" :key="k" :value="k">
              {{ label }}
            </option>
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
                :checked="project.color === c"
                :aria-label="label"
                @change="emit('setField', { color: c })"
              />
              <span :style="{ background: `var(--p-${c})` }"></span>
            </label>
          </div>
        </div>
        <label class="td-row">
          <span>{{ $t('Ordner') }}</span>
          <input
            class="input mono"
            :value="project.local_path ?? ''"
            :placeholder="$t('Absoluter Pfad oder ~/… auf deinem Rechner (optional)')"
            @change="(e) => emit('setField', { local_path: valueOf(e).trim() || null })"
          />
        </label>
        <label class="td-row">
          <span>{{ $t('GitHub-Repo') }}</span>
          <input
            class="input mono"
            :value="project.github_repo ?? ''"
            :placeholder="$t('owner/name, sonst aus dem Ordner (optional)')"
            @change="(e) => emit('setField', { github_repo: valueOf(e).trim() || null })"
          />
        </label>
      </div>

      <!-- ZEITERFASSUNG -->
      <div class="td-sec">
        <div class="pd-time-meta">
          {{
            $t('Erfasst: {week} diese Woche · {total} gesamt', {
              week: dur(weekMinutes),
              total: dur(totalMinutes),
            })
          }}
          <template v-if="fractionText">
            · {{ $t('{n} Aufgaben erledigt', { n: fractionText }) }}</template
          >
        </div>
      </div>

      <!-- OFFENE AUFGABEN -->
      <div class="td-sec sub-sec">
        <span class="lbl">
          {{ $t('Offene Aufgaben') }}
          <template v-if="openTasks.length"> · {{ openTasks.length }}</template>
        </span>
        <div v-if="openTasks.length" class="task-rows">
          <div
            v-for="task in openTasks"
            :key="task.id"
            class="sub-row"
            :class="{ done: !isOpen(task) }"
          >
            <button
              type="button"
              class="cb"
              role="checkbox"
              :aria-checked="task.status === 'COMPLETED'"
              :aria-label="$t('{title} erledigt', { title: task.title })"
              :disabled="task.status === 'CANCELLED'"
              @click="emit('toggleTask', task)"
            ></button>
            <button
              type="button"
              class="sub-task-title"
              @click="router.push({ path: '/tasks', query: { task: task.id } })"
            >
              {{ task.title }}
            </button>
            <component
              :is="resourceHref(r) ? 'a' : 'span'"
              v-for="r in getTaskResources(task.id)"
              :key="r.id"
              class="res-mini"
              :href="resourceHref(r)"
              :target="r.type === 'URL' ? '_blank' : undefined"
              rel="noopener"
            >
              <svg class="ic"><use :href="`#${RES_ICON[r.type]}`" /></svg>
            </component>
            <TaskActions
              class="row-act"
              :task="task"
              :running="task.status === 'IN_PROGRESS'"
              :note="getTaskResources(task.id).find((r) => r.type === 'NOTE')?.target"
              @run="(fn) => emit('run', fn)"
            />
          </div>
        </div>
        <div v-else class="pd-empty-note">
          {{ $t('Keine offenen Aufgaben für dieses Projekt.') }}
        </div>
        <form class="addrow" @submit.prevent="handleAddTask">
          <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
          <input
            v-model="newTaskTitle"
            type="text"
            :placeholder="$t('Aufgabe für dieses Projekt hinzufügen')"
            :aria-label="$t('Aufgabe für dieses Projekt hinzufügen')"
          />
        </form>
      </div>

      <!-- GITHUB-ISSUES -->
      <GitHubIssues
        v-if="project.local_path || project.github_repo"
        class="td-sec"
        :project-id="project.id"
        @open="(id) => router.push({ path: '/tasks', query: { task: id } })"
        @changed="emit('reload')"
      />

      <!-- RESSOURCEN -->
      <ResourceList
        class="td-sec"
        :resources="projectResources"
        :project-id="project.id"
        @changed="emit('reload')"
      />

      <!-- FOOTER / LÖSCHEN -->
      <div class="td-foot">
        <span></span>
        <DeleteButton
          :text="
            $t(
              '„{name}“ löschen? Die Tasks bleiben ohne Projekt, ihre Zeiteinträge landen in „Sonstiges“.',
              { name: project.name },
            )
          "
          @confirm="emit('deleteProject')"
        />
      </div>
    </div>
  </aside>
</template>

<style scoped>
.project-aside {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-width: none;
  box-sizing: border-box;
}
.project-aside::-webkit-scrollbar {
  display: none;
}

.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 0 20px;
  min-width: 0;
  box-sizing: border-box;
}
.aside-status-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.status-select {
  height: 28px;
  font-size: 12.5px;
  padding: 0 8px;
  border-radius: var(--r-s);
}

.proj-detail-body {
  display: grid;
  gap: 20px;
  padding: 16px 20px 32px;
  min-width: 0;
  box-sizing: border-box;
}

.pd-hero {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.pd-icon-btn {
  flex: none;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: var(--r-m);
  border: 1px solid var(--br-default);
  background: color-mix(in srgb, var(--pc) 14%, var(--bg-2));
  color: color-mix(in srgb, var(--pc) 85%, var(--tx-primary));
  cursor: pointer;
  transition:
    transform var(--dur) ease-out,
    border-color var(--dur) ease-out;
}
.pd-icon-btn:hover {
  transform: scale(1.05);
  border-color: var(--br-strong);
}
.pd-icon-btn .ic {
  width: 22px;
  height: 22px;
}
.pd-hero-text {
  flex: 1;
  min-width: 0;
}

.td-title {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  font: 600 20px/1.25 var(--font-ui);
  letter-spacing: -0.015em;
  color: var(--tx-primary);
  background: none;
  border: 0;
  padding: 0;
  outline: none;
}
.td-title:focus {
  box-shadow: 0 1px 0 var(--br-strong);
}

.pd-icon-popover {
  padding: 12px;
  border-radius: var(--r-m);
  border: 1px solid var(--br-default);
  background: var(--bg-2);
  box-shadow: var(--shadow-pop);
}
.icon-picker-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.icon-picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(32px, 1fr));
  gap: 4px;
}
.icon-pick-cell {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: var(--r-s);
  border: 1px solid transparent;
  background: none;
  color: var(--tx-secondary);
  cursor: pointer;
}
.icon-pick-cell:hover {
  background: var(--bg-hover);
  color: var(--tx-primary);
}
.icon-pick-cell.active {
  background: color-mix(in srgb, var(--pc) 20%, transparent);
  border-color: var(--pc);
  color: var(--tx-primary);
}
.icon-pick-cell .ic {
  width: 16px;
  height: 16px;
}

.td-desc {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 8px 10px;
  font: 400 13px/1.45 var(--font-ui);
  color: var(--tx-primary);
  background: var(--bg-2);
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-s);
  resize: vertical;
}
.td-desc:focus {
  border-color: var(--br-strong);
  outline: none;
}

.vsc-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  align-self: start;
  max-width: 100%;
}
.vsc-btn .ic {
  width: 14px;
  height: 14px;
}

.td-props {
  display: grid;
  border-top: 1px solid var(--br-subtle);
  min-width: 0;
}
.td-row {
  display: flex;
  align-items: center;
  min-height: 38px;
  padding: 4px 0;
  border-bottom: 1px solid var(--br-subtle);
  font: 400 13px/1.3 var(--font-ui);
  gap: 12px;
  min-width: 0;
  box-sizing: border-box;
}
.td-row > span {
  flex: none;
  width: 85px;
  color: var(--tx-muted);
}
.td-row .input {
  flex: 1;
  min-width: 0;
  width: 0;
}

.color-row .swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.swatch {
  position: relative;
  display: inline-flex;
  cursor: pointer;
}
.swatch input {
  position: absolute;
  inset: 0;
  opacity: 0;
  margin: 0;
  cursor: pointer;
}
.swatch span {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid transparent;
  box-shadow: inset 0 0 0 2px var(--bg-1);
}
.swatch input:checked + span {
  border-color: var(--tx-primary);
}

.td-sec {
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
  min-width: 0;
}
.pd-time-meta {
  font: 400 12.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
  word-break: break-word;
}

.sub-sec {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.task-rows {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.sub-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 2px 6px;
  border-radius: var(--r-s);
  transition: background var(--dur) ease-out;
  min-width: 0;
  box-sizing: border-box;
}
.sub-row:hover {
  background: var(--bg-hover);
}
.sub-task-title {
  flex: 1;
  min-width: 0;
  font: 400 13px/1.3 var(--font-ui);
  color: var(--tx-primary);
  text-align: left;
  border: 0;
  background: none;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub-task-title:hover {
  text-decoration: underline;
}
.sub-row.done .sub-task-title {
  color: var(--tx-muted);
  text-decoration: line-through;
}
.sub-row :deep(.row-act) {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}
.sub-row :deep(.row-act .btn) {
  padding: 2px 7px;
  font-size: 11.5px;
  height: 24px;
  line-height: 1;
}
.sub-row :deep(.row-act .act-code) {
  padding: 2px 6px;
  font-size: 11.5px;
  height: 24px;
}
.res-mini {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  color: var(--tx-muted);
  text-decoration: none;
  flex: none;
}
.res-mini:hover {
  color: var(--tx-primary);
  background: var(--bg-hover);
}
.res-mini .ic {
  width: 12px;
  height: 12px;
}
.pd-empty-note {
  font: 400 12.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
}

.addrow {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  border-radius: var(--r-s);
  min-width: 0;
  box-sizing: border-box;
}
.addrow .ic {
  width: 14px;
  height: 14px;
  color: var(--tx-muted);
  flex: none;
}
.addrow input {
  flex: 1;
  min-width: 0;
  width: 0;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-primary);
  background: none;
  border: 0;
  outline: none;
}
.addrow input::placeholder {
  color: var(--tx-muted);
}

.td-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
  min-width: 0;
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
