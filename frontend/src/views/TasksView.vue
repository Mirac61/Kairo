<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  createTask, deleteTask, errorMessage, listProjects, listTasks, taskAction, updateTask,
  type Project, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { hhmm, ymd } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'
import DeleteButton from '@/components/DeleteButton.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: 'Backlog', PLANNED: 'Geplant', IN_PROGRESS: 'Läuft',
  PAUSED: 'Pausiert', COMPLETED: 'Erledigt', CANCELLED: 'Abgebrochen',
}
const PRIO: Record<string, { label: string; color: string }> = {
  URGENT: { label: 'Dringend', color: 'var(--a-red)' }, HIGH: { label: 'Hoch', color: 'var(--a-orange)' },
  MEDIUM: { label: 'Mittel', color: 'var(--a-yellow)' }, LOW: { label: 'Niedrig', color: 'var(--a-neutral)' },
}
const FILTERS = [
  { id: 'alle', label: 'Alle' }, { id: 'heute', label: 'Heute' }, { id: 'woche', label: 'Diese Woche' },
  { id: 'ueber', label: 'Überfällig' }, { id: 'prio', label: 'Prio hoch' }, { id: 'erledigt', label: 'Erledigt' },
]

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const error = ref('')
const filter = ref('alle')
const projectId = ref('alle')
const openId = ref<string | null>(null)
const quick = ref('')

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const isOpen = (t: Task) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'
const todayStr = () => ymd(new Date())
const weekEnd = () => { const d = new Date(); d.setDate(d.getDate() + 7 - ((d.getDay() + 6) % 7)); return ymd(d) } // Montag der nächsten Woche
// Der für die Liste maßgebliche Tag: geplanter Tag, sonst Fälligkeit.
const dayOf = (t: Task) => t.planned_date ?? (t.due_at ? ymd(new Date(t.due_at)) : null)

const visible = computed(() => tasks.value.filter((t) => {
  if (projectId.value !== 'alle' && t.project_id !== projectId.value) return false
  const d = dayOf(t)
  switch (filter.value) {
    case 'erledigt': return t.status === 'COMPLETED'
    case 'heute': return isOpen(t) && d === todayStr()
    case 'woche': return isOpen(t) && d !== null && d >= todayStr() && d < weekEnd()
    case 'ueber': return isOpen(t) && d !== null && d < todayStr()
    case 'prio': return isOpen(t) && (t.priority === 'HIGH' || t.priority === 'URGENT')
    default: return isOpen(t)
  }
}))

async function load() {
  try {
    ;[tasks.value, projects.value] = await Promise.all([listTasks(), listProjects()])
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
  const title = quick.value.trim()
  if (!title) return
  void run(async () => {
    await createTask({ title, project_id: projectId.value === 'alle' ? null : projectId.value })
    quick.value = ''
  })
}

const toggle = (t: Task) =>
  run(() => (t.status === 'COMPLETED' ? updateTask(t.id, { status: 'PLANNED' }) : taskAction(t.id, 'complete')))

const startTime = (t: Task) => (t.planned_start_at ? hhmm(new Date(t.planned_start_at)) : '')
const fmtDay = (d: string | null) => {
  if (!d) return ''
  if (d === todayStr()) return 'Heute'
  return new Intl.DateTimeFormat('de-DE', { day: 'numeric', month: 'short' }).format(new Date(`${d}T00:00:00`))
}
const overdue = (t: Task) => isOpen(t) && dayOf(t) !== null && dayOf(t)! < todayStr()

// Datum und Uhrzeit hängen zusammen: Ohne Datum gibt es keine Uhrzeit, ein neues Datum behält sie.
function setDate(t: Task, date: string) {
  const time = startTime(t)
  void run(() => updateTask(t.id, { planned_date: date, planned_start_at: date && time ? new Date(`${date}T${time}`).toISOString() : '' }))
}
function setTime(t: Task, time: string) {
  if (!t.planned_date) return
  void run(() => updateTask(t.id, { planned_start_at: time ? new Date(`${t.planned_date}T${time}`).toISOString() : '' }))
}
const setEstimate = (t: Task, value: string) => void run(() => updateTask(t.id, { estimated_minutes: Number(value) || 0 }))
const valueOf = (e: Event) => (e.target as HTMLInputElement).value

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">Aufgaben</h1>
      <div class="v-sub count">{{ visible.length }} {{ visible.length === 1 ? 'Aufgabe' : 'Aufgaben' }}</div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="filterbar">
      <button v-for="f in FILTERS" :key="f.id" type="button" class="fchip" :aria-pressed="filter === f.id" @click="filter = f.id">{{ f.label }}</button>
      <select v-model="projectId" class="input" aria-label="Nach Projekt filtern">
        <option value="alle">Alle Projekte</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </div>

    <div v-if="!visible.length" class="v-sub">Keine Aufgaben.</div>
    <div v-else class="card tasklist">
      <template v-for="t in visible" :key="t.id">
        <div class="task-row" :class="{ done: !isOpen(t) }">
          <button type="button" class="cb" role="checkbox" :aria-checked="t.status === 'COMPLETED'" :aria-label="`${t.title} erledigt`" :disabled="t.status === 'CANCELLED'" @click="toggle(t)"></button>
          <button type="button" class="t row-title" :aria-expanded="openId === t.id" @click="openId = openId === t.id ? null : t.id">{{ t.title }}</button>
          <span v-if="t.status === 'IN_PROGRESS' || t.status === 'PAUSED'" class="badge"><span class="cdot" :style="{ background: t.status === 'IN_PROGRESS' ? 'var(--a-green)' : 'var(--a-yellow)' }"></span>{{ STATUS_LABEL[t.status] }}</span>
          <span v-if="t.project_id" class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><span class="cdot"></span>{{ projectName.get(t.project_id) }}</span>
          <span class="due" :class="{ od: overdue(t) }">{{ fmtDay(dayOf(t)) }}<template v-if="startTime(t)"> {{ startTime(t) }}</template></span>
          <span class="prio-l">{{ PRIO[t.priority]?.label }}</span>
          <span class="pdot" :style="{ background: PRIO[t.priority]?.color }"></span>
        </div>
        <div v-if="openId === t.id" class="task-detail">
          <template v-if="isOpen(t)">
            <label class="field"><span>Datum</span><input class="input" type="date" :value="t.planned_date ?? ''" @change="(e) => setDate(t, valueOf(e))" /></label>
            <label class="field"><span>Uhrzeit</span><input class="input" type="time" :value="startTime(t)" :disabled="!t.planned_date" @change="(e) => setTime(t, valueOf(e))" /></label>
            <label class="field"><span>Minuten</span><input class="input" type="number" min="0" :value="t.estimated_minutes" @change="(e) => setEstimate(t, valueOf(e))" /></label>
            <button v-if="t.planned_date !== todayStr()" type="button" class="btn btn-secondary" @click="setDate(t, todayStr())">Heute</button>
            <TaskActions :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
          </template>
          <DeleteButton :text="`„${t.title}“ löschen?`" @confirm="run(() => deleteTask(t.id))" />
        </div>
      </template>
    </div>

    <form class="addrow" @submit.prevent="add">
      <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
      <input v-model="quick" type="text" placeholder="Neue Aufgabe hinzufügen — mit Eingabetaste bestätigen" aria-label="Neue Aufgabe hinzufügen" />
    </form>
  </div>
</template>

<style scoped>
.row-title { text-align: left; background: none; border: 0; color: inherit; cursor: pointer; }
.task-detail {
  display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; padding: 12px 16px 14px 44px;
  background: var(--bg-2); border-bottom: 1px solid var(--br-subtle);
}
.task-detail .field { width: 140px; }
</style>
