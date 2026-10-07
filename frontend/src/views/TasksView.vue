<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  createTask, deleteTask, errorMessage, listProjects, listResources, listTasks, updateTask,
  type Project, type Resource, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useUndo } from '@/composables/useUndo'
import { hhmm, ymd } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'
import DeleteButton from '@/components/DeleteButton.vue'
import ResourceList from '@/components/ResourceList.vue'
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
  { id: 'alle', label: 'Offen' }, { id: 'heute', label: 'Heute' }, { id: 'woche', label: 'Diese Woche' },
  { id: 'ueber', label: 'Überfällig' }, { id: 'prio', label: 'Prio hoch' }, { id: 'erledigt', label: 'Erledigt' },
  { id: 'abgebrochen', label: 'Abgebrochen' },
]
// Per Auswahl setzbar; Läuft, Pausiert und Erledigt ergeben sich aus Timer und Häkchen.
const SETTABLE: Task['status'][] = ['BACKLOG', 'PLANNED', 'CANCELLED']

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const error = ref('')
const filter = ref('alle')
const projectId = ref('alle')
const openId = ref<string | null>(null)
const quick = ref('')
const subTitle = ref('')

const { setDone } = useUndo()

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
    case 'abgebrochen': return t.status === 'CANCELLED'
    case 'heute': return isOpen(t) && d === todayStr()
    case 'woche': return isOpen(t) && d !== null && d >= todayStr() && d < weekEnd()
    case 'ueber': return isOpen(t) && d !== null && d < todayStr()
    case 'prio': return isOpen(t) && (t.priority === 'HIGH' || t.priority === 'URGENT')
    default: return isOpen(t)
  }
}))

// Teilaufgaben stehen im Detail ihrer Elterntask; eigene Zeile nur, wenn die Eltern nicht (als oberste Ebene) in der Liste stehen.
const rows = computed(() => {
  const byId = new Map(visible.value.map((t) => [t.id, t]))
  return visible.value.filter((t) => {
    const p = t.parent_task_id ? byId.get(t.parent_task_id) : undefined
    return !p || p.parent_task_id
  })
})
const kidsOf = (id: string) => tasks.value.filter((t) => t.parent_task_id === id)
const progress = (id: string) => {
  const k = kidsOf(id).filter((c) => c.status !== 'CANCELLED')
  return k.length ? `${k.filter((c) => c.status === 'COMPLETED').length}/${k.length}` : ''
}
const resOf = (id: string) => resources.value.filter((r) => r.task_id === id)

async function load() {
  try {
    ;[tasks.value, projects.value, resources.value] = await Promise.all([listTasks(), listProjects(), listResources()])
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
function addSub(t: Task) {
  const title = subTitle.value.trim()
  if (!title) return
  void run(async () => {
    await createTask({ title, parent_task_id: t.id, project_id: t.project_id })
    subTitle.value = ''
  })
}

const toggle = (t: Task) =>
  t.status === 'COMPLETED' ? run(() => updateTask(t.id, { status: t.planned_date ? 'PLANNED' : 'BACKLOG' })) : setDone(t, 'COMPLETED', run)

// Mittel ist der Standard und bleibt in der Zeile unbeschriftet.
const prio = (t: Task) => (t.priority === 'MEDIUM' ? undefined : PRIO[t.priority])
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
const setField = (t: Task, field: 'title' | 'description' | 'priority' | 'project_id' | 'status', value: string) => {
  if (field === 'title' && !value.trim()) return
  if (field === 'status' && value === 'CANCELLED') return setDone(t, 'CANCELLED', run)
  void run(() => updateTask(t.id, { [field]: value }))
}
// Fälligkeit als Tag; gespeichert wird das Ende dieses Tages in Ortszeit.
const dueDay = (t: Task) => (t.due_at ? ymd(new Date(t.due_at)) : '')
const setDue = (t: Task, day: string) => void run(() => updateTask(t.id, { due_at: day ? new Date(`${day}T23:59:00`).toISOString() : '' }))
const valueOf = (e: Event) => (e.target as HTMLInputElement).value

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">Aufgaben</h1>
      <div class="v-sub count">{{ rows.length }} {{ rows.length === 1 ? 'Aufgabe' : 'Aufgaben' }}</div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="filterbar">
      <button v-for="f in FILTERS" :key="f.id" type="button" class="fchip" :aria-pressed="filter === f.id" @click="filter = f.id">{{ f.label }}</button>
      <select v-model="projectId" class="input" aria-label="Nach Projekt filtern">
        <option value="alle">Alle Projekte</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </div>

    <div v-if="!rows.length" class="v-sub">Keine Aufgaben.</div>
    <div v-else class="card tasklist">
      <template v-for="t in rows" :key="t.id">
        <div class="task-row" :class="{ done: !isOpen(t) }">
          <button type="button" class="cb" role="checkbox" :aria-checked="t.status === 'COMPLETED'" :aria-label="`${t.title} erledigt`" :disabled="t.status === 'CANCELLED'" @click="toggle(t)"></button>
          <button type="button" class="t row-title" :aria-expanded="openId === t.id" @click="openId = openId === t.id ? null : t.id; subTitle = ''">{{ t.title }}</button>
          <span v-if="progress(t.id)" class="badge mono" role="img" :aria-label="`Teilaufgaben erledigt: ${progress(t.id)}`">{{ progress(t.id) }}</span>
          <span v-if="t.status === 'IN_PROGRESS' || t.status === 'PAUSED'" class="badge"><span class="cdot" :style="{ background: t.status === 'IN_PROGRESS' ? 'var(--a-green)' : 'var(--a-yellow)' }"></span>{{ STATUS_LABEL[t.status] }}</span>
          <span v-if="t.project_id" class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><span class="cdot"></span>{{ projectName.get(t.project_id) }}</span>
          <span class="due" :class="{ od: overdue(t) }">{{ `${fmtDay(dayOf(t))} ${startTime(t)}`.trim() }}</span>
          <span class="prio-l">{{ prio(t)?.label }}</span>
          <span class="pdot" :style="{ background: prio(t)?.color }"></span>
        </div>
        <div v-if="openId === t.id" class="task-detail">
          <label class="field wide"><span>Titel</span><input class="input" :value="t.title" @change="(e) => setField(t, 'title', valueOf(e))" /></label>
          <label class="field wide"><span>Beschreibung</span><textarea class="input" rows="3" :value="t.description" @change="(e) => setField(t, 'description', valueOf(e))"></textarea></label>
          <label class="field"><span>Priorität</span>
            <select class="input" :value="t.priority" @change="(e) => setField(t, 'priority', valueOf(e))">
              <option v-for="(p, key) in PRIO" :key="key" :value="key">{{ p.label }}</option>
            </select>
          </label>
          <label class="field"><span>Status</span>
            <select class="input" :value="t.status" @change="(e) => setField(t, 'status', valueOf(e))">
              <option v-for="st in SETTABLE" :key="st" :value="st">{{ STATUS_LABEL[st] }}</option>
              <option v-if="!SETTABLE.includes(t.status)" :value="t.status" disabled>{{ STATUS_LABEL[t.status] }}</option>
            </select>
          </label>
          <label class="field"><span>Projekt</span>
            <select class="input" :value="t.project_id ?? ''" @change="(e) => setField(t, 'project_id', valueOf(e))">
              <option value="">Kein Projekt</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </label>
          <label class="field"><span>Fällig</span><input class="input" type="date" :value="dueDay(t)" @change="(e) => setDue(t, valueOf(e))" /></label>
          <template v-if="isOpen(t)">
            <label class="field"><span>Datum</span><input class="input" type="date" :value="t.planned_date ?? ''" @change="(e) => setDate(t, valueOf(e))" /></label>
            <label class="field"><span>Uhrzeit</span><input class="input" type="time" :value="startTime(t)" :disabled="!t.planned_date" @change="(e) => setTime(t, valueOf(e))" /></label>
            <label class="field"><span>Minuten</span><input class="input" type="number" min="0" :value="t.estimated_minutes" @change="(e) => setEstimate(t, valueOf(e))" /></label>
            <button v-if="t.planned_date !== todayStr()" type="button" class="btn btn-secondary" @click="setDate(t, todayStr())">Heute</button>
            <TaskActions :task="t" :running="t.status === 'IN_PROGRESS'" @run="run" />
          </template>
          <DeleteButton :text="`„${t.title}“ löschen?`" @confirm="run(() => deleteTask(t.id))" />
          <div class="full sub-sec">
            <span class="lbl">Teilaufgaben<template v-if="progress(t.id)"> · {{ progress(t.id) }}</template></span>
            <div v-for="c in kidsOf(t.id)" :key="c.id" class="sub-row" :class="{ done: !isOpen(c) }">
              <button type="button" class="cb" role="checkbox" :aria-checked="c.status === 'COMPLETED'" :aria-label="`${c.title} erledigt`" :disabled="c.status === 'CANCELLED'" @click="toggle(c)"></button>
              <span>{{ c.title }}</span>
            </div>
            <form class="addrow" @submit.prevent="addSub(t)">
              <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
              <input v-model="subTitle" type="text" placeholder="Teilaufgabe hinzufügen" aria-label="Teilaufgabe hinzufügen" />
            </form>
          </div>
          <ResourceList class="full" :resources="resOf(t.id)" :task-id="t.id" @changed="load" />
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
.task-detail .field.wide, .task-detail .full { width: 100%; }
.sub-sec { display: grid; gap: 8px; }
.sub-sec .addrow { margin-top: 0; }
.sub-row { display: flex; align-items: center; gap: 10px; font: 400 14px/1.4 var(--font-ui); }
.sub-row.done span { text-decoration: line-through; opacity: .5; }
</style>
