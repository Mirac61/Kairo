<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createTask, deleteTask, isOpen, listProjects, listResources, listTasks, restoreTask, taskAction, updateTask,
  type Project, type Resource, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { addDays, dayLabel, daysAgo, dm, hhmm, hm, shortDay, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { projectColor } from '@/lib/projectColor'
import { parseQuickAdd, taskBody } from '@/lib/quickAdd'
import QuickHints from '@/components/QuickHints.vue'
import ResourceList from '@/components/ResourceList.vue'
import SearchField from '@/components/SearchField.vue'
import TaskActions from '@/components/TaskActions.vue'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: 'Backlog', PLANNED: 'Geplant', IN_PROGRESS: 'Läuft',
  PAUSED: 'Pausiert', COMPLETED: 'Erledigt', CANCELLED: 'Abgebrochen',
}
const PRIO_LABEL: Record<string, string> = { URGENT: 'Dringend', HIGH: 'Hoch', MEDIUM: 'Mittel', LOW: 'Niedrig' }
const FILTERS = [
  { id: 'alle', label: 'Offen' }, { id: 'heute', label: 'Heute' }, { id: 'woche', label: 'Diese Woche' },
  { id: 'ueber', label: 'Überfällig' }, { id: 'prio', label: 'Prio hoch' }, { id: 'erledigt', label: 'Erledigt' },
  { id: 'abgebrochen', label: 'Abgebrochen' },
]
// Per Auswahl setzbar; Läuft, Pausiert und Erledigt ergeben sich aus Timer und Häkchen.
const SETTABLE: Task['status'][] = ['BACKLOG', 'PLANNED', 'CANCELLED']

const SORTS = [['created', 'Angelegt'], ['day', 'Tag'], ['prio', 'Priorität'], ['title', 'Titel']] as const
const GROUPS = [['due', 'Fällig'], ['none', 'Keine'], ['project', 'Projekt'], ['day', 'Tag']] as const
const BY: Record<string, string> = { due: 'Fälligkeit', project: 'Projekt', day: 'Tag' }
const DUE_TITLE: Record<string, string> = { 0: 'Überfällig', 1: 'Früher', 3: 'Diese Woche', 4: 'Später', 5: 'Ohne Datum' }
const PRIO_RANK: Record<string, number> = { URGENT: 0, HIGH: 1, MEDIUM: 2, LOW: 3 }
const VIEW_KEY = 'kairo-tasks-view' // Sortierung und Gruppierung merkt sich der Browser

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])
const filter = ref('alle')
const projectId = ref('alle')
const openId = ref<string | null>(null)
const quick = ref('')
const subTitle = ref('')
const search = ref('')
const sel = ref<string | null>(null) // Auswahl für die Tastenkürzel
const quickEl = ref<HTMLInputElement>()
const view = ref({ sort: 'created', group: 'due' })
try { Object.assign(view.value, JSON.parse(store.get(VIEW_KEY) ?? '{}')) } catch { /* kaputter Wert */ }
watch(view, (v) => store.set(VIEW_KEY, JSON.stringify(v)), { deep: true })
const route = useRoute()
const router = useRouter()

const { offer, setDone } = useUndo()

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const todayStr = () => ymd(new Date())
const weekEnd = () => ymd(addDays(weekStart(), 7)) // Montag der nächsten Woche
// Der für die Liste maßgebliche Tag: geplanter Tag, sonst Fälligkeit.
const dayOf = (t: Task) => t.planned_date ?? (t.due_at ? ymd(new Date(t.due_at)) : null)

const inScope = (t: Task) => {
  if (projectId.value !== 'alle' && t.project_id !== projectId.value) return false
  const q = search.value.trim().toLowerCase()
  return !q || `${t.title} ${t.description}`.toLowerCase().includes(q)
}
const matches = (t: Task, f: string) => {
  const d = dayOf(t)
  switch (f) {
    case 'erledigt': return t.status === 'COMPLETED'
    case 'abgebrochen': return t.status === 'CANCELLED'
    case 'heute': return isOpen(t) && d === todayStr()
    case 'woche': return isOpen(t) && d !== null && d >= todayStr() && d < weekEnd()
    case 'ueber': return isOpen(t) && d !== null && d < todayStr()
    case 'prio': return isOpen(t) && (t.priority === 'HIGH' || t.priority === 'URGENT')
    default: return isOpen(t)
  }
}
const visible = computed(() => tasks.value.filter((t) => inScope(t) && matches(t, filter.value)))
const counts = computed(() => Object.fromEntries(FILTERS.map((f) => [f.id, tasks.value.filter((t) => inScope(t) && matches(t, f.id)).length])))
const stats = computed(() => {
  const open = tasks.value.filter(isOpen)
  return { open: open.length, today: open.filter((t) => dayOf(t) === todayStr()).length, overdue: open.filter(overdue).length, planned: open.filter((t) => t.planned_date).length }
})

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

// Gruppen in Anzeigereihenfolge: Fälligkeit nach Dringlichkeit, Tag aufsteigend, Projekt nach Name; „ohne“ steht zuletzt.
const todayLong = new Intl.DateTimeFormat('de-DE', { weekday: 'short', day: 'numeric', month: 'short' }).format(new Date()).replace('.,', '')
function dueBucket(t: Task) {
  const d = dayOf(t)
  if (!d) return '5'
  if (d < todayStr()) return isOpen(t) ? '0' : '1'
  return d === todayStr() ? '2' : d < weekEnd() ? '3' : '4'
}
const groups = computed(() => {
  const sorted = [...rows.value].sort(SORT[view.value.sort] ?? SORT.created!)
  const by = view.value.group
  if (!sorted.length) return []
  if (by !== 'project' && by !== 'day' && by !== 'due') return [{ key: '', title: '', rows: sorted, minutes: 0 }]
  const keyOf = (t: Task) => (by === 'project' ? t.project_id : by === 'due' ? dueBucket(t) : dayOf(t)) ?? ''
  const titleOf = (k: string) => by === 'due' ? (k === '2' ? `Heute · ${todayLong}` : DUE_TITLE[k]!)
    : by === 'project' ? (k ? projectName.value.get(k) ?? 'Projekt' : 'Ohne Projekt') : k ? dayLabel(k, todayStr()) : 'Ohne Tag'
  const m = new Map<string, Task[]>()
  for (const t of sorted) m.set(keyOf(t), [...(m.get(keyOf(t)) ?? []), t])
  return [...m]
    .sort(([a], [b]) => Number(a === '') - Number(b === '') || (by === 'project' ? titleOf(a).localeCompare(titleOf(b), 'de') : a.localeCompare(b)))
    .map(([key, rs]) => ({ key, title: titleOf(key), rows: rs, minutes: rs.reduce((n, t) => n + t.estimated_minutes, 0) }))
})
const flat = computed(() => groups.value.flatMap((g) => g.rows))
// Verschwindet die ausgewählte Task (erledigt, gefiltert), rückt die Auswahl auf die Nachbarin.
watch(flat, (now, before) => {
  if (!sel.value || now.some((t) => t.id === sel.value)) return
  sel.value = now[Math.min(before.findIndex((t) => t.id === sel.value), now.length - 1)]?.id ?? null
})

const open = computed(() => tasks.value.find((t) => t.id === openId.value))
watch(openId, () => (subTitle.value = ''))
const prioLevel = (t: Task) => ({ LOW: 1, MEDIUM: 2, HIGH: 3, URGENT: 3 })[t.priority as 'LOW'] ?? 2
const kidsOf = (id: string) => tasks.value.filter((t) => t.parent_task_id === id)
const progress = (id: string) => {
  const k = kidsOf(id).filter((c) => c.status !== 'CANCELLED')
  return k.length ? `${k.filter((c) => c.status === 'COMPLETED').length}/${k.length}` : ''
}
const resOf = (id: string) => resources.value.filter((r) => r.task_id === id)

const { error, load, run } = useLoader(async () => {
  ;[tasks.value, projects.value, resources.value] = await Promise.all([listTasks(), listProjects(), listResources()])
})

// Archivierte Projekte sind für #name nicht wählbar.
const quickParsed = computed(() => parseQuickAdd(quick.value, projects.value.filter((p) => p.status !== 'ARCHIVED'), todayStr()))
function add() {
  const q = quickParsed.value
  if (!q.title) return
  void run(async () => {
    await createTask({ project_id: projectId.value === 'alle' ? null : projectId.value, ...taskBody(q), title: q.title })
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

const startTime = (t: Task) => (t.planned_start_at ? hhmm(new Date(t.planned_start_at)) : '')
// Eine Spalte, ein Format: Datum (+ Uhrzeit), „fällig 12.10.“ ohne Plan, überfällig als „vor 2 Tagen“ (Datum im Tooltip).
const when = (t: Task) => {
  const d = dayOf(t)
  if (!d) return { text: '', tip: '' }
  if (overdue(t)) return { text: daysAgo(d, todayStr()), tip: `Fällig ${shortDay(d)}` }
  if (!t.planned_date) return { text: `fällig ${dm(d)}`, tip: '' }
  return { text: `${dayLabel(d, todayStr())} ${startTime(t)}`.trim(), tip: '' }
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
  if (openId.value) openId.value = sel.value // das Detail folgt der Auswahl
  void nextTick(() => document.querySelector(`[data-task="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' }))
}
useShortcuts((e) => {
  const el = e.target as HTMLElement
  const t = flat.value.find((x) => x.id === sel.value)
  if (e.key === 'n') { e.preventDefault(); quickEl.value?.focus() }
  else if (e.key === 'j') move(1)
  else if (e.key === 'k') move(-1)
  else if (e.key === 'Enter' && t && !el.closest('button, a')) openId.value = openId.value === t.id ? null : t.id
  else if (e.key === 'x' && t) void toggle(t)
  else if (e.key === 's' && t && isOpen(t)) void run(() => taskAction(t.id, t.status === 'IN_PROGRESS' ? 'pause' : 'start'))
  else if (e.key === 't' && t && isOpen(t)) setDate(t, todayStr())
})

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

// „Neu …“ / ⌘K aus der Sidebar: /tasks?new=… fokussiert die Schnelleingabe.
function focusNew() {
  if (!route.query.new) return
  void nextTick(() => quickEl.value?.focus())
  void router.replace({ query: {} })
}
watch(() => route.query.new, focusNew)

onMounted(() => {
  focusNew()
  void load().then(focusFromRoute)
})
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">Aufgaben</h1>
        <div class="v-sub tk-stats">
          <span><b>{{ stats.open }}</b> offen</span><span><b>{{ stats.today }}</b> heute</span>
          <span :class="{ od: stats.overdue }"><b>{{ stats.overdue }}</b> überfällig</span><span><b>{{ stats.planned }}</b> eingeplant</span>
        </div>
      </div>
      <button class="btn btn-primary" type="button" aria-keyshortcuts="n" @click="quickEl?.focus()"><svg class="ic"><use href="#i-plus" /></svg>Neue Aufgabe<kbd class="key" aria-hidden="true">N</kbd></button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div class="filterbar tk-bar">
      <div class="seg tk-tabs" role="group" aria-label="Filter">
        <button v-for="f in FILTERS" :key="f.id" type="button" :aria-pressed="filter === f.id" @click="filter = f.id">{{ f.label }} <span class="count">{{ counts[f.id] }}</span></button>
      </div>
      <span class="spacer"></span>
      <SearchField v-model="search" label="Aufgaben durchsuchen" />
    </div>

    <form class="addrow" @submit.prevent="add">
      <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
      <input ref="quickEl" v-model="quick" type="text" placeholder="Neue Aufgabe, z. B. Sport 30m @morgen #Kairo !hoch" aria-label="Neue Aufgabe hinzufügen" aria-keyshortcuts="n" />
      <span class="qa-syntax" aria-hidden="true"><kbd class="key">30m</kbd><kbd class="key">@datum</kbd><kbd class="key">#projekt</kbd><kbd class="key">!prio</kbd></span>
    </form>
    <QuickHints :q="quickParsed" :today="todayStr()" />
    <div class="tk-opts">
      <span class="tk-by">{{ view.group === 'none' ? 'Nicht gruppiert' : `Gruppiert nach ${BY[view.group] ?? ''}` }}</span>
      <span class="spacer"></span>
      <select v-model="projectId" class="input" aria-label="Nach Projekt filtern">
        <option value="alle">Projekt: Alle</option>
        <option v-for="p in projects" :key="p.id" :value="p.id">Projekt: {{ p.name }}</option>
      </select>
      <select v-model="view.group" class="input" aria-label="Gruppieren nach">
        <option v-for="[v, l] in GROUPS" :key="v" :value="v">Gruppe: {{ l }}</option>
      </select>
      <select v-model="view.sort" class="input" aria-label="Sortieren nach">
        <option v-for="[v, l] in SORTS" :key="v" :value="v">Sortierung: {{ l }}</option>
      </select>
    </div>
    <div class="tasks-layout" :class="{ 'with-aside': open }">
    <div class="tasks-main">
    <div v-if="!rows.length" class="v-sub">Keine Aufgaben.</div>
    <div v-for="g in groups" :key="g.key" class="tgroup">
      <span v-if="g.title" class="lbl" :class="{ od: view.group === 'due' && g.key === '0' }">{{ g.title }}<span class="count">{{ g.rows.length }}</span><span v-if="g.minutes" class="g-sum">{{ hm(g.minutes) }} geschätzt</span></span>
      <div class="tasklist">
      <template v-for="t in g.rows" :key="t.id">
        <div class="task-row" :class="{ done: !isOpen(t), selected: sel === t.id, running: t.status === 'IN_PROGRESS' }" :data-task="t.id">
          <button type="button" class="cb ring" :style="{ '--rc': projectColor(t.project_id) }" role="checkbox" :aria-checked="t.status === 'COMPLETED'" :aria-label="`${t.title} erledigt`" :disabled="t.status === 'CANCELLED'" @click="toggle(t)"></button>
          <button type="button" class="t row-title" :aria-expanded="openId === t.id" @click="openId = openId === t.id ? null : t.id; sel = t.id; subTitle = ''">{{ t.title }}</button>
          <span v-if="progress(t.id)" class="badge mono" role="img" :aria-label="`Teilaufgaben erledigt: ${progress(t.id)}`">{{ progress(t.id) }}</span>
          <span v-if="t.status === 'IN_PROGRESS' || t.status === 'PAUSED'" class="status-l" :class="{ run: t.status === 'IN_PROGRESS' }"><span class="cdot"></span>{{ STATUS_LABEL[t.status] }}</span>
          <span class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><template v-if="t.project_id"><span class="cdot"></span>{{ projectName.get(t.project_id) }}</template></span>
          <span class="dur">{{ t.estimated_minutes ? hm(t.estimated_minutes) : '' }}</span>
          <span class="due" :class="{ od: overdue(t) }" :data-tip="when(t).tip || undefined">{{ when(t).text }}</span>
          <span class="prio-i" :class="{ high: prioLevel(t) === 3 }" role="img" :aria-label="`Priorität: ${PRIO_LABEL[t.priority] ?? t.priority}`"><i v-for="n in 3" :key="n" :class="{ on: n <= prioLevel(t) }"></i></span>
        </div>
      </template>
      </div>
    </div>
    </div>
    <aside v-if="open" class="task-aside" aria-label="Aufgabendetails">
      <div class="aside-head"><span class="lbl">Aufgabe</span><button class="icon-btn" type="button" aria-label="Details schließen" @click="openId = null"><svg class="ic"><use href="#i-x" /></svg></button></div>
      <template v-for="t in [open]" :key="t.id"><!-- t: die geöffnete Task, wie in der Zeile -->
        <div class="task-detail">
          <label class="field wide"><span>Titel</span><input class="input" :value="t.title" @change="(e) => setField(t, 'title', valueOf(e))" /></label>
          <label class="field wide"><span>Beschreibung</span><textarea class="input" rows="3" :value="t.description" @change="(e) => setField(t, 'description', valueOf(e))"></textarea></label>
          <label class="field"><span>Priorität</span>
            <select class="input" :value="t.priority" @change="(e) => setField(t, 'priority', valueOf(e))">
              <option v-for="(label, key) in PRIO_LABEL" :key="key" :value="key">{{ label }}</option>
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
          <button type="button" class="btn btn-secondary" @click="trash(t)">Löschen</button>
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
    </aside>
    </div>

    <div class="keys" aria-label="Tastenkürzel">
      <span><kbd class="key">n</kbd>Neu</span>
      <span><kbd class="key">j</kbd><kbd class="key">k</kbd>Wählen</span>
      <span><kbd class="key">x</kbd>Erledigt</span>
      <span><kbd class="key">Enter</kbd>Öffnen</span>
      <span><kbd class="key">s</kbd>Start/Pause</span>
      <span><kbd class="key">t</kbd>Heute</span>
    </div>
  </div>
</template>

<style scoped>
.keys { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 20px; margin-top: 16px; font: 400 12px/1 var(--font-ui); color: var(--tx-secondary); }
.keys span { display: inline-flex; align-items: center; gap: 6px; }
.keys .key { min-width: 20px; text-align: center; color: var(--tx-secondary); }
.task-row.selected { background: var(--bg-2); box-shadow: inset 0 0 0 1px var(--br-default); }
.task-row.running::before { content: ''; position: absolute; left: 0; top: 6px; bottom: 6px; width: 3px; border-radius: 2px; background: var(--a-run); }
.task-row:hover:not(.selected) { background: var(--bg-hover); }
.row-title { text-align: left; background: none; border: 0; color: inherit; cursor: pointer; }
/* Detail rechts neben der Liste; auf schmalen Fenstern als Sheet am unteren Rand. */
.tasks-layout { margin-top: 16px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 0 32px; align-items: start; }
.tasks-layout.with-aside { grid-template-columns: minmax(0, 1fr) 340px; }
.task-aside {
  position: sticky; top: 16px; max-height: calc(100vh - 32px); overflow: auto;
  background: var(--bg-1); border: 1px solid var(--br-default); border-radius: var(--r-m);
}
.aside-head { display: flex; align-items: center; justify-content: space-between; padding: 12px 12px 0 16px; }
.task-detail { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; padding: 8px 16px 16px; }
.task-detail .field { width: calc(50% - 6px); }
@media (max-width: 1000px) {
  .tasks-layout.with-aside { grid-template-columns: minmax(0, 1fr); }
  .task-aside { position: fixed; inset: auto 0 0 0; z-index: 20; max-height: 65vh; border-radius: var(--r-m) var(--r-m) 0 0; box-shadow: var(--shadow-pop); }
}
.tk-tabs { flex-wrap: wrap; border-radius: 10px; }
.tk-tabs button { display: inline-flex; align-items: center; gap: 6px; border-radius: 8px; }
.tk-opts { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 20px; }
.tk-opts select { height: 28px; width: auto; padding: 0 26px 0 10px; font-size: 12px; border-radius: 999px; }
.tk-by { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.spacer { flex: 1; }
.tk-stats { display: flex; flex-wrap: wrap; gap: 4px 14px; }
.tk-stats b { font-weight: 500; color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.tk-stats .od, .tk-stats .od b { color: var(--a-red); }
.qa-syntax { display: inline-flex; gap: 6px; }
.btn-primary .key { margin-left: 6px; background: transparent; color: inherit; border-color: currentColor; opacity: .6; }
.tgroup > .lbl.od { color: var(--a-red); }
.g-sum { margin-left: auto; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.task-detail .field.wide, .task-detail .full { width: 100%; }
.sub-sec { display: grid; gap: 8px; }
.sub-sec .addrow { margin-top: 0; }
.sub-row { display: flex; align-items: center; gap: 10px; font: 400 14px/1.4 var(--font-ui); }
.sub-row.done span { text-decoration: line-through; opacity: .5; }
</style>
