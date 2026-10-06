<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NAlert, NButton, NCheckbox, NCheckboxGroup, NH1, NInput, NInputNumber, NList, NListItem, NSelect, NSpace, NText,
} from 'naive-ui'
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
const weekdayOptions = WEEKDAYS.map((v) => ({ value: v, label: v }))

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
  <n-space vertical :size="16">
    <n-h1 style="margin: 0">Habits</n-h1>
    <n-alert v-if="error" type="error">{{ error }}</n-alert>

    <form @submit.prevent="add">
      <n-space align="center">
        <n-input v-model:value="form.name" placeholder="Neues Habit" style="width: 220px" />
        <n-select v-model:value="form.type" :options="typeOptions" style="width: 200px" />
        <n-select v-if="form.type === 'WEEKLY'" v-model:value="form.weekday" :options="weekdayOptions" style="width: 90px" />
        <n-checkbox-group v-if="form.type === 'SPECIFIC_WEEKDAYS'" v-model:value="form.weekdays">
          <n-space :size="8"><n-checkbox v-for="d in WEEKDAYS" :key="d" :value="d" :label="d" /></n-space>
        </n-checkbox-group>
        <n-input-number v-if="form.type === 'TIMES_PER_WEEK'" v-model:value="form.times" :min="1" :max="7" style="width: 90px" />
        <n-button type="primary" attr-type="submit">Anlegen</n-button>
      </n-space>
    </form>

    <n-text v-if="!habits.length" depth="3">Keine Habits.</n-text>
    <n-list v-else bordered>
      <n-list-item v-for="h in habits" :key="h.id">
        <n-space vertical :size="2">
          <n-text :depth="h.active ? 1 : 3">{{ h.name }}</n-text>
          <n-text depth="3" style="font-size: 13px">{{ describe(h) }}<template v-if="!h.active"> · inaktiv</template></n-text>
        </n-space>
        <template #suffix>
          <n-space :size="6" :wrap="false">
            <n-button size="small" @click="run(() => updateHabit(h.id, { active: !h.active }))">
              {{ h.active ? 'Deaktivieren' : 'Aktivieren' }}
            </n-button>
            <delete-button :text="`„${h.name}“ löschen?`" @confirm="run(() => deleteHabit(h.id))" />
          </n-space>
        </template>
      </n-list-item>
    </n-list>
  </n-space>
</template>
