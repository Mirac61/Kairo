<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  createTask, deleteTask, errorMessage, listProjects, listResources, listTasks, restoreTask, taskAction, updateTask,
  type Project, type Resource, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useUndo } from '@/composables/useUndo'
import { hhmm, ymd } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'
import { parseQuickAdd } from '@/lib/quickAdd'
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

const SORTS = [['created', 'Angelegt'], ['day', 'Tag'], ['prio', 'Priorität'], ['title', 'Titel']] as const
const GROUPS = [['none', 'Keine'], ['project', 'Projekt'], ['day', 'Tag']] as const
const PRIO_RANK: Record<string, number> = { URGENT: 0, HIGH: 1, MEDIUM: 2, LOW: 3 }
const VIEW_KEY = 'kairo-tasks-view' // Sortierung und Gruppierung merkt sich der Browser

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const error = ref('')
const filter = ref('alle')
const projectId = ref('alle')
const openId = ref<string | null>(null)
const quick = ref('')
const subTitle = ref('')
const search = ref('')
const sel = ref<string | null>(null) // Auswahl für die Tastenkürzel
const quickEl = ref<HTMLInputElement>()
const view = ref({ sort: 'created', group: 'none' })
try { Object.assign(view.value, JSON.parse(localStorage.getItem(VIEW_KEY) ?? '{}')) } catch { /* privater Modus oder kaputter Wert */ }
watch(view, (v) => { try { localStorage.setItem(VIEW_KEY, JSON.stringify(v)) } catch { /* privater Modus */ } }, { deep: true })
const route = useRoute()

const { offer, setDone } = useUndo()

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const isOpen = (t: Task) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'
const todayStr = () => ymd(new Date())
const weekEnd = () => { const d = new Date(); d.setDate(d.getDate() + 7 - ((d.getDay() + 6) % 7)); return ymd(d) } // Montag der nächsten Woche
// Der für die Liste maßgebliche Tag: geplanter Tag, sonst Fälligkeit.
const dayOf = (t: Task) => t.planned_date ?? (t.due_at ? ymd(new Date(t.due_at)) : null)

const visible = computed(() => tasks.value.filter((t) => {
  if (projectId.value !== 'alle' && t.project_id !== projectId.value) return false
  const q = search.value.trim().toLowerCase()
  if (q && !`${t.title} ${t.description}`.toLowerCase().includes(q)) return false
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
const SORT: Record<string, (a: Task, b: Task) => number> = {
  created: () => 0, // Reihenfolge der API
  day: (a, b) => (dayOf(a) ?? '9').localeCompare(dayOf(b) ?? '9') || (a.planned_start_at ?? '').localeCompare(b.planned_start_at ?? ''),
  prio: (a, b) => (PRIO_RANK[a.priority] ?? 2) - (PRIO_RANK[b.priority] ?? 2),
  title: (a, b) => a.title.localeCompare(b.title, 'de'),
}

// Gruppen in Anzeigereihenfolge: Tag aufsteigend, Projekt nach Name; „ohne“ steht zuletzt.
const groups = computed(() => {
  const sorted = [...rows.value].sort(SORT[view.value.sort] ?? SORT.created!)
  const by = view.value.group
  if (!sorted.length) return []
  if (by !== 'project' && by !== 'day') return [{ key: '', title: '', rows: sorted }]
  const keyOf = (t: Task) => (by === 'project' ? t.project_id : dayOf(t)) ?? ''
  const titleOf = (k: string) => (by === 'project' ? (k ? projectName.value.get(k) ?? 'Projekt' : 'Ohne Projekt') : k ? fmtDay(k) : 'Ohne Tag')
  const m = new Map<string, Task[]>()
  for (const t of sorted) m.set(keyOf(t), [...(m.get(keyOf(t)) ?? []), t])
  return [...m]
    .sort(([a], [b]) => Number(a === '') - Number(b === '') || (by === 'day' ? a.localeCompare(b) : titleOf(a).localeCompare(titleOf(b), 'de')))
    .map(([key, rs]) => ({ key, title: titleOf(key), rows: rs }))
})
const flat = computed(() => groups.value.flatMap((g) => g.rows))
// Verschwindet die ausgewählte Task (erledigt, gefiltert), rückt die Auswahl auf die Nachbarin.
watch(flat, (now, before) => {
  if (!sel.value || now.some((t) => t.id === sel.value)) return
  sel.value = now[Math.min(before.findIndex((t) => t.id === sel.value), now.length - 1)]?.id ?? null
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
  const { title, minutes } = parseQuickAdd(quick.value)
  if (!title) return
  void run(async () => {
    await createTask({ title, estimated_minutes: minutes, project_id: projectId.value === 'alle' ? null : projectId.value })
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

// Löschen legt in den Papierkorb; Rückgängig holt die Task samt Teilaufgaben zurück.
const trash = ({ id, title }: Task) =>
  void run(async () => {
    await deleteTask(id)
    offer(`„${title}“ gelöscht`, () => run(() => restoreTask(id)))
  })

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

// Tastenkürzel: n neu · j/k wählen · x erledigen · Enter öffnen · s Start/Pause · t heute. Nicht beim Tippen, nicht in Dialogen.
function move(d: number) {
  const l = flat.value
  if (!l.length) return
  const i = l.findIndex((t) => t.id === sel.value)
  sel.value = l[i < 0 ? (d > 0 ? 0 : l.length - 1) : Math.min(Math.max(i + d, 0), l.length - 1)]!.id
  void nextTick(() => document.querySelector(`[data-task="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' }))
}
function onKey(e: KeyboardEvent) {
  const el = e.target as HTMLElement
  if (e.ctrlKey || e.metaKey || e.altKey || el.closest('input, textarea, select, [contenteditable], .overlay')) return
  const t = flat.value.find((x) => x.id === sel.value)
  if (e.key === 'n') { e.preventDefault(); quickEl.value?.focus() }
  else if (e.key === 'j') move(1)
  else if (e.key === 'k') move(-1)
  else if (e.key === 'Enter' && t && !el.closest('button, a')) openId.value = openId.value === t.id ? null : t.id
  else if (e.key === 'x' && t) void toggle(t)
  else if (e.key === 's' && t && isOpen(t)) void run(() => taskAction(t.id, t.status === 'IN_PROGRESS' ? 'pause' : 'start'))
  else if (e.key === 't' && t && isOpen(t)) setDate(t, todayStr())
}

// Aus dem Kalender: /tasks?task=ID öffnet die Task, auch wenn sie erledigt oder abgebrochen ist.
function focusFromRoute() {
  const t = tasks.value.find((x) => x.id === route.query.task)
  if (!t) return
  filter.value = t.status === 'COMPLETED' ? 'erledigt' : t.status === 'CANCELLED' ? 'abgebrochen' : 'alle'
  projectId.value = 'alle'
  search.value = ''
  openId.value = sel.value = t.id
  void nextTick(() => document.querySelector(`[data-task="${t.id}"]`)?.scrollIntoView({ block: 'center' }))
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  void load().then(focusFromRoute)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
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
      <input v-model="search" type="search" class="input" placeholder="Suchen …" aria-label="Aufgaben durchsuchen" />
      <select v-model="view.group" class="input" aria-label="Gruppieren nach">
        <option v-for="[v, l] in GROUPS" :key="v" :value="v">Gruppe: {{ l }}</option>
      </select>
      <select v-model="view.sort" class="input" aria-label="Sortieren nach">
        <option v-for="[v, l] in SORTS" :key="v" :value="v">Sortierung: {{ l }}</option>
      </select>
    </div>

    <div v-if="!rows.length" class="v-sub">Keine Aufgaben.</div>
    <div v-for="g in groups" :key="g.key" class="tgroup">
      <span v-if="g.title" class="lbl">{{ g.title }}<span class="count">{{ g.rows.length }}</span></span>
      <div class="card tasklist">
      <template v-for="t in g.rows" :key="t.id">
        <div class="task-row" :class="{ done: !isOpen(t), selected: sel === t.id }" :data-task="t.id">
          <button type="button" class="cb" role="checkbox" :aria-checked="t.status === 'COMPLETED'" :aria-label="`${t.title} erledigt`" :disabled="t.status === 'CANCELLED'" @click="toggle(t)"></button>
          <button type="button" class="t row-title" :aria-expanded="openId === t.id" @click="openId = openId === t.id ? null : t.id; sel = t.id; subTitle = ''">{{ t.title }}</button>
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
          <button type="button" class="btn btn-danger" @click="trash(t)">Löschen</button>
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
    </div>

    <form class="addrow" @submit.prevent="add">
      <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
      <input ref="quickEl" v-model="quick" type="text" placeholder="Neue Aufgabe, z. B. 30 min Sport — mit Eingabetaste bestätigen" aria-label="Neue Aufgabe hinzufügen" />
    </form>
    <p class="v-sub keys">Tastenkürzel: n neu · j/k wählen · x erledigt · Enter öffnen · s Start/Pause · t heute</p>
  </div>
</template>

<style scoped>
.keys { margin-top: 10px; }
.task-row.selected { background: var(--bg-2); box-shadow: inset 2px 0 0 var(--a-blue); }
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
