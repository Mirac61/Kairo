<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import {
  createHabit, deleteHabit, errorMessage, listHabits, updateHabit, type FrequencyConfig, type Habit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'

const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
const TYPE_LABEL: Record<Habit['frequency_type'], string> = {
  DAILY: 'Täglich', WEEKLY: 'Wöchentlich', SPECIFIC_WEEKDAYS: 'Bestimmte Wochentage', TIMES_PER_WEEK: 'X-mal pro Woche',
}
const typeOptions = Object.entries(TYPE_LABEL).map(([value, label]) => ({ value, label }))

const habits = ref<Habit[]>([])
const error = ref('')
const form = ref({ name: '', type: 'DAILY' as Habit['frequency_type'], weekday: 'MO', weekdays: ['MO'] as string[], times: 3 as number | null })

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
  if (f.type === 'TIMES_PER_WEEK') return { times: f.times ?? 1 }
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

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="stack">
    <h1 class="title">Habits</h1>
    <Message v-if="error" severity="error">{{ error }}</Message>

    <form class="row" @submit.prevent="add">
      <InputText v-model="form.name" placeholder="Neues Habit" class="w-name" />
      <Select v-model="form.type" :options="typeOptions" option-label="label" option-value="value" class="w-type" />
      <Select v-if="form.type === 'WEEKLY'" v-model="form.weekday" :options="WEEKDAYS" class="w-day" />
      <template v-if="form.type === 'SPECIFIC_WEEKDAYS'">
        <label v-for="d in WEEKDAYS" :key="d" class="row day"><Checkbox v-model="form.weekdays" :value="d" /> {{ d }}</label>
      </template>
      <InputNumber v-if="form.type === 'TIMES_PER_WEEK'" v-model="form.times" :min="1" :max="7" input-class="w-day" />
      <Button type="submit" label="Anlegen" />
    </form>

    <span v-if="!habits.length" class="muted">Keine Habits.</span>
    <ul v-else class="list card">
      <li v-for="h in habits" :key="h.id">
        <span class="main">
          <span :class="{ done: !h.active }">{{ h.name }}</span>
          <span class="muted">{{ describe(h) }}<template v-if="!h.active"> · inaktiv</template></span>
        </span>
        <div class="row nowrap">
          <Button :label="h.active ? 'Deaktivieren' : 'Aktivieren'" size="small" severity="secondary" @click="run(() => updateHabit(h.id, { active: !h.active }))" />
          <DeleteButton :text="`„${h.name}“ löschen?`" @confirm="run(() => deleteHabit(h.id))" />
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.w-name { width: 220px; }
.w-type { width: 220px; }
.w-day { width: 90px; }
.day { gap: 4px; font-size: 13px; }
</style>
