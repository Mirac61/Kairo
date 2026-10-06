<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  createHabit,
  deleteHabit,
  errorMessage,
  listHabits,
  updateHabit,
  type FrequencyConfig,
  type Habit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'

const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
const TYPE_LABEL: Record<Habit['frequency_type'], string> = {
  DAILY: 'Täglich',
  WEEKLY: 'Wöchentlich',
  SPECIFIC_WEEKDAYS: 'Bestimmte Wochentage',
  TIMES_PER_WEEK: 'X-mal pro Woche',
}

const habits = ref<Habit[]>([])
const error = ref('')
const form = ref({
  name: '',
  type: 'DAILY' as Habit['frequency_type'],
  weekday: 'MO',
  weekdays: ['MO'] as string[],
  times: 3,
})

async function load() {
  try {
    habits.value = await listHabits()
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

function config(): FrequencyConfig {
  const f = form.value
  if (f.type === 'WEEKLY') return { weekday: f.weekday }
  if (f.type === 'SPECIFIC_WEEKDAYS') return { weekdays: f.weekdays }
  if (f.type === 'TIMES_PER_WEEK') return { times: f.times }
  return {}
}

function add() {
  const name = form.value.name.trim()
  if (!name) return
  void run(async () => {
    await createHabit({ name, frequency_type: form.value.type, frequency_config: config() })
    form.value.name = ''
  })
}

function describe(h: Habit) {
  const c = h.frequency_config
  if (h.frequency_type === 'WEEKLY') return `Wöchentlich (${c.weekday ?? 'Startdatum'})`
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (c.weekdays ?? []).join(', ')
  if (h.frequency_type === 'TIMES_PER_WEEK') return `${c.times}× pro Woche`
  return TYPE_LABEL[h.frequency_type]
}

function remove(h: Habit) {
  if (confirm(`„${h.name}“ löschen?`)) void run(() => deleteHabit(h.id))
}

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <section>
    <h1>Habits</h1>
    <p v-if="error" class="error">{{ error }}</p>

    <form class="row" @submit.prevent="add">
      <input v-model="form.name" placeholder="Neues Habit" required />
      <select v-model="form.type">
        <option v-for="(label, t) in TYPE_LABEL" :key="t" :value="t">{{ label }}</option>
      </select>
      <select v-if="form.type === 'WEEKLY'" v-model="form.weekday">
        <option v-for="d in WEEKDAYS" :key="d">{{ d }}</option>
      </select>
      <span v-if="form.type === 'SPECIFIC_WEEKDAYS'" class="days">
        <label v-for="d in WEEKDAYS" :key="d"><input v-model="form.weekdays" type="checkbox" :value="d" /> {{ d }}</label>
      </span>
      <input v-if="form.type === 'TIMES_PER_WEEK'" v-model.number="form.times" type="number" min="1" max="7" class="num" />
      <button>Anlegen</button>
    </form>

    <p v-if="!habits.length" class="hint">Keine Habits.</p>
    <ul class="list">
      <li v-for="h in habits" :key="h.id">
        <span class="title" :class="{ off: !h.active }">
          {{ h.name }} <small>{{ describe(h) }}<template v-if="!h.active"> · inaktiv</template></small>
        </span>
        <button @click="run(() => updateHabit(h.id, { active: !h.active }))">{{ h.active ? 'Deaktivieren' : 'Aktivieren' }}</button>
        <button @click="remove(h)">Löschen</button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 16px; font-size: 24px; }
.hint { color: var(--text-muted); }
.error { color: var(--err); }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 12px; }
.row > input:first-child { flex: 1; min-width: 160px; }
.num { width: 64px; }
.days { display: flex; gap: 8px; font-size: 13px; }
input, select, button {
  padding: 4px 10px; border: 1px solid var(--border); border-radius: 6px;
  background: var(--surface); color: var(--text); font: inherit;
}
.days input { padding: 0; }
button { background: var(--bg); cursor: pointer; }
button:hover { border-color: var(--accent); color: var(--accent); }
.list { list-style: none; margin: 0; padding: 0; }
.list li {
  display: flex; align-items: center; gap: 12px; padding: 8px 12px; margin-bottom: 4px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.title { flex: 1; }
.title small { color: var(--text-muted); font-size: 13px; }
.off { color: var(--text-muted); }
</style>
