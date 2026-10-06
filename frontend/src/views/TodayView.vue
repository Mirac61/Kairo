<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Checkbox from 'primevue/checkbox'
import Message from 'primevue/message'
import { completeHabit, errorMessage, getToday, uncompleteHabit, type Task, type Today } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import TaskActions from '@/components/TaskActions.vue'

const today = ref<Today | null>(null)
const error = ref('')
const now = ref(Date.now())
let tick: ReturnType<typeof setInterval> | undefined

async function load() {
  try {
    today.value = await getToday()
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
  }
  await load() // das /ws-Ereignis lädt ebenfalls, aber so ist die Anzeige auch ohne /ws aktuell
}

useLiveEvents(load)
onMounted(() => {
  tick = setInterval(() => (now.value = Date.now()), 1000)
})
onUnmounted(() => clearInterval(tick))

const fmt = (iso: string) =>
  new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit', timeZone: today.value?.timezone }).format(new Date(iso))
const hm = (min: number) => `${Math.floor(min / 60)}:${String(min % 60).padStart(2, '0')} h`

const runningTaskId = computed(() => today.value?.running_time_entry?.task_id ?? null)
const elapsed = computed(() => {
  const e = today.value?.running_time_entry
  if (!e) return ''
  const s = Math.max(0, Math.floor((now.value - Date.parse(e.started_at)) / 1000))
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(Math.floor(s / 3600))}:${p(Math.floor(s / 60) % 60)}:${p(s % 60)}`
})

type Item =
  | { kind: 'event'; at: string; key: string; title: string; sub: string; end: string }
  | { kind: 'task'; at: string; key: string; task: Task }

// Zeitleiste: Termine und Tasks mit Uhrzeit, chronologisch.
const timeline = computed<Item[]>(() => {
  const t = today.value
  if (!t) return []
  const items: Item[] = t.events.map((e) => ({
    kind: 'event', at: e.occurrence_start, key: `e${e.id}${e.occurrence_start}`,
    title: e.title, sub: e.location, end: e.occurrence_end,
  }))
  for (const task of t.tasks) {
    if (task.planned_start_at) items.push({ kind: 'task', at: task.planned_start_at, key: `t${task.id}`, task })
  }
  return items.sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
})
const groups = computed(() => [
  { title: 'Ohne Uhrzeit', tasks: today.value?.tasks.filter((t) => !t.planned_start_at) ?? [] },
  { title: 'Aktiv, nicht für heute geplant', tasks: today.value?.active_tasks ?? [] },
])
</script>

<template>
  <div class="stack">
    <h1 class="title">Heute <span class="muted">{{ today?.date }}</span></h1>
    <Message v-if="error" severity="error">{{ error }}</Message>

    <template v-if="today">
      <div class="row stats">
        <div><span class="muted">Geplant</span><b>{{ hm(today.planned_minutes) }}</b></div>
        <div><span class="muted">Termine</span><b>{{ hm(today.calendar_minutes) }}</b></div>
        <div><span class="muted">Erfasst</span><b>{{ hm(today.tracked_minutes) }}</b></div>
        <div><span class="muted">Frei</span><b>{{ hm(today.free_minutes) }}</b><span class="muted">von {{ hm(today.work_minutes) }}</span></div>
      </div>
      <Message v-if="today.overplanned_minutes" severity="warn">Überplant um {{ hm(today.overplanned_minutes) }}</Message>
      <Message v-if="today.running_time_entry" severity="success" class="running">Timer läuft: {{ elapsed }}</Message>

      <section class="card">
        <h2>Zeitleiste</h2>
        <span v-if="!timeline.length" class="muted">Keine Termine oder Tasks mit Uhrzeit.</span>
        <ul v-else class="list">
          <li v-for="i in timeline" :key="i.key">
            <span class="time">{{ fmt(i.at) }}</span>
            <span v-if="i.kind === 'event'" class="main">
              {{ i.title }}
              <span class="muted">bis {{ fmt(i.end) }}<template v-if="i.sub"> · {{ i.sub }}</template></span>
            </span>
            <template v-else>
              <span class="main" :class="{ done: i.task.status === 'COMPLETED' }">{{ i.task.title }}</span>
              <TaskActions :task="i.task" :running="runningTaskId === i.task.id" @run="run" />
            </template>
          </li>
        </ul>
      </section>

      <template v-for="g in groups" :key="g.title">
        <section v-if="g.tasks.length" class="card">
          <h2>{{ g.title }}</h2>
          <ul class="list">
            <li v-for="task in g.tasks" :key="task.id">
              <span class="main" :class="{ done: task.status === 'COMPLETED' }">{{ task.title }}</span>
              <TaskActions :task="task" :running="runningTaskId === task.id" @run="run" />
            </li>
          </ul>
        </section>
      </template>

      <section v-if="today.habits.length" class="card">
        <h2>Habits</h2>
        <ul class="list">
          <li v-for="h in today.habits" :key="h.id">
            <Checkbox
              :model-value="h.done" binary :input-id="`h${h.id}`"
              @update:model-value="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
            />
            <label :for="`h${h.id}`" class="main">
              {{ h.name }}
              <span v-if="h.week_progress" class="muted">{{ h.week_progress.done }}/{{ h.week_progress.target }} diese Woche</span>
            </label>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>

<style scoped>
.stats { gap: 32px; }
.stats div { display: flex; flex-direction: column; }
.stats b { font-size: 24px; font-weight: 500; }
</style>
