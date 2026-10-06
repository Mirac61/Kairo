<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  NAlert, NCard, NCheckbox, NH1, NList, NListItem, NSpace, NStatistic, NText,
} from 'naive-ui'
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
  <n-space vertical :size="16">
    <n-h1 style="margin: 0">Heute <n-text depth="3" style="font-size: 14px">{{ today?.date }}</n-text></n-h1>
    <n-alert v-if="error" type="error">{{ error }}</n-alert>

    <template v-if="today">
      <n-space :size="32">
        <n-statistic label="Geplant" :value="hm(today.planned_minutes)" />
        <n-statistic label="Termine" :value="hm(today.calendar_minutes)" />
        <n-statistic label="Erfasst" :value="hm(today.tracked_minutes)" />
        <n-statistic label="Frei" :value="hm(today.free_minutes)">
          <template #suffix><n-text depth="3" style="font-size: 14px">von {{ hm(today.work_minutes) }}</n-text></template>
        </n-statistic>
      </n-space>
      <n-alert v-if="today.overplanned_minutes" type="warning">Überplant um {{ hm(today.overplanned_minutes) }}</n-alert>
      <n-alert v-if="today.running_time_entry" class="running" type="success">Timer läuft: {{ elapsed }}</n-alert>

      <n-card title="Zeitleiste" size="small">
        <n-text v-if="!timeline.length" depth="3">Keine Termine oder Tasks mit Uhrzeit.</n-text>
        <n-list v-else>
          <n-list-item v-for="i in timeline" :key="i.key">
            <template #prefix><n-text depth="3">{{ fmt(i.at) }}</n-text></template>
            <template v-if="i.kind === 'event'">
              {{ i.title }}
              <n-text depth="3">bis {{ fmt(i.end) }}<template v-if="i.sub"> · {{ i.sub }}</template></n-text>
            </template>
            <n-text v-else :delete="i.task.status === 'COMPLETED'">{{ i.task.title }}</n-text>
            <template v-if="i.kind === 'task'" #suffix>
              <task-actions :task="i.task" :running="runningTaskId === i.task.id" @run="run" />
            </template>
          </n-list-item>
        </n-list>
      </n-card>

      <template v-for="g in groups" :key="g.title">
        <n-card v-if="g.tasks.length" :title="g.title" size="small">
          <n-list>
            <n-list-item v-for="task in g.tasks" :key="task.id">
              <n-text :delete="task.status === 'COMPLETED'">{{ task.title }}</n-text>
              <template #suffix><task-actions :task="task" :running="runningTaskId === task.id" @run="run" /></template>
            </n-list-item>
          </n-list>
        </n-card>
      </template>

      <n-card v-if="today.habits.length" title="Habits" size="small">
        <n-list>
          <n-list-item v-for="h in today.habits" :key="h.id">
            <n-checkbox
              :checked="h.done"
              @update:checked="run(() => (h.done ? uncompleteHabit(h.id, today!.date) : completeHabit(h.id, today!.date)))"
            >
              {{ h.name }}
              <n-text v-if="h.week_progress" depth="3">{{ h.week_progress.done }}/{{ h.week_progress.target }} diese Woche</n-text>
            </n-checkbox>
          </n-list-item>
        </n-list>
      </n-card>
    </template>
  </n-space>
</template>
