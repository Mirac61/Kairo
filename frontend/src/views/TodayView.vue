<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  completeHabit,
  getToday,
  taskAction,
  uncompleteHabit,
  type Task,
  type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'

const today = ref<Today | null>(null)
const error = ref('')
const now = ref(Date.now())
let tick: ReturnType<typeof setInterval> | undefined

async function load() {
  try {
    today.value = await getToday()
    error.value = ''
  } catch {
    error.value = 'Backend nicht erreichbar.'
  }
}

async function run(fn: () => Promise<unknown>) {
  try {
    await fn()
  } catch {
    error.value = 'Aktion fehlgeschlagen.'
  }
  await load() // das /ws-Ereignis lädt ebenfalls, aber so ist die Anzeige auch ohne /ws aktuell
}

useLiveEvents(load)
onMounted(() => {
  tick = setInterval(() => (now.value = Date.now()), 1000)
})
onUnmounted(() => clearInterval(tick))

const fmt = (iso: string) =>
  new Intl.DateTimeFormat('de-DE', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: today.value?.timezone,
  }).format(new Date(iso))

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
    kind: 'event',
    at: e.occurrence_start,
    key: `e${e.id}${e.occurrence_start}`,
    title: e.title,
    sub: e.location,
    end: e.occurrence_end,
  }))
  for (const task of t.tasks) {
    if (task.planned_start_at) items.push({ kind: 'task', at: task.planned_start_at, key: `t${task.id}`, task })
  }
  return items.sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
})
const untimed = computed(() => today.value?.tasks.filter((t) => !t.planned_start_at) ?? [])
</script>

<template>
  <section>
    <h1>Heute <small v-if="today">{{ today.date }}</small></h1>
    <p v-if="error" class="error">{{ error }}</p>

    <template v-if="today">
      <p class="summary">
        Geplant {{ hm(today.planned_minutes) }} · Termine {{ hm(today.calendar_minutes) }} · Erfasst
        {{ hm(today.tracked_minutes) }} · Frei {{ hm(today.free_minutes) }} von {{ hm(today.work_minutes) }}
      </p>
      <p v-if="today.overplanned_minutes" class="error">
        Überplant um {{ hm(today.overplanned_minutes) }}
      </p>
      <p v-if="today.running_time_entry" class="running">● Timer läuft: {{ elapsed }}</p>

      <h2>Zeitleiste</h2>
      <p v-if="!timeline.length" class="hint">Keine Termine oder Tasks mit Uhrzeit.</p>
      <ul class="list">
        <li v-for="i in timeline" :key="i.key">
          <span class="time">{{ fmt(i.at) }}</span>
          <template v-if="i.kind === 'event'">
            <span class="title">{{ i.title }} <small>bis {{ fmt(i.end) }}<template v-if="i.sub"> · {{ i.sub }}</template></small></span>
          </template>
          <template v-else>
            <span class="title" :class="{ done: i.task.status === 'COMPLETED' }">{{ i.task.title }}</span>
            <span class="actions">
              <button v-if="runningTaskId === i.task.id" @click="run(() => taskAction(i.task.id, 'pause'))">Pause</button>
              <button v-else-if="i.task.status !== 'COMPLETED'" @click="run(() => taskAction(i.task.id, 'start'))">Start</button>
              <button v-if="i.task.status !== 'COMPLETED'" @click="run(() => taskAction(i.task.id, 'complete'))">Fertig</button>
            </span>
          </template>
        </li>
      </ul>

      <template v-for="[title, tasks] in [['Ohne Uhrzeit', untimed], ['Aktiv, nicht für heute geplant', today.active_tasks]] as const" :key="title">
        <template v-if="tasks.length">
          <h2>{{ title }}</h2>
          <ul class="list">
            <li v-for="task in tasks" :key="task.id">
              <span class="title" :class="{ done: task.status === 'COMPLETED' }">{{ task.title }}</span>
              <span class="actions">
                <button v-if="runningTaskId === task.id" @click="run(() => taskAction(task.id, 'pause'))">Pause</button>
                <button v-else-if="task.status !== 'COMPLETED'" @click="run(() => taskAction(task.id, 'start'))">Start</button>
                <button v-if="task.status !== 'COMPLETED'" @click="run(() => taskAction(task.id, 'complete'))">Fertig</button>
              </span>
            </li>
          </ul>
        </template>
      </template>

      <template v-if="today.habits.length">
        <h2>Habits</h2>
        <ul class="list">
          <li v-for="h in today.habits" :key="h.id">
            <label class="title">
              <input
                type="checkbox"
                :checked="h.done"
                @change="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
              />
              {{ h.name }}
              <small v-if="h.week_progress">{{ h.week_progress.done }}/{{ h.week_progress.target }} diese Woche</small>
            </label>
          </li>
        </ul>
      </template>
    </template>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 8px; font-size: 24px; }
h1 small, .title small { color: var(--text-muted); font-size: 13px; font-weight: 400; }
h2 { margin: 24px 0 8px; font-size: 16px; }
.summary, .hint { margin: 0; color: var(--text-muted); }
.running { color: var(--ok); font-weight: 600; }
.error { color: var(--err); }
.list { list-style: none; margin: 0; padding: 0; }
.list li {
  display: flex; align-items: center; gap: 12px;
  padding: 8px 12px; margin-bottom: 4px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.time { width: 48px; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.title { flex: 1; }
.done { text-decoration: line-through; color: var(--text-muted); }
.actions { display: flex; gap: 6px; }
button {
  padding: 4px 10px; border: 1px solid var(--border); border-radius: 6px;
  background: var(--bg); color: var(--text); cursor: pointer;
}
button:hover { border-color: var(--accent); color: var(--accent); }
</style>
