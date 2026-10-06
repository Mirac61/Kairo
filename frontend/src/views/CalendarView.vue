<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NDatePicker, NH1, NInput, NList, NListItem, NSpace, NText } from 'naive-ui'
import { createEvent, deleteEvent, errorMessage, getToday, type Today } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'

// Wochenansicht: pro Tag ein /api/today?date=…, damit Serientermine als
// einzelne Vorkommen erscheinen. Neue Termine nutzen die Zeitzone des Browsers.
const pad = (n: number) => String(n).padStart(2, '0')
const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

function monday(d: Date) {
  const m = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  m.setDate(m.getDate() - ((m.getDay() + 6) % 7))
  return m
}

const weekStart = ref(monday(new Date()))
const days = ref<Today[]>([])
const error = ref('')
const form = ref({ title: '', start: null as number | null, end: null as number | null, location: '' })

const range = computed(() => {
  const end = new Date(weekStart.value)
  end.setDate(end.getDate() + 6)
  return `${ymd(weekStart.value)} – ${ymd(end)}`
})

async function load() {
  const dates = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(weekStart.value)
    d.setDate(d.getDate() + i)
    return ymd(d)
  })
  try {
    days.value = await Promise.all(dates.map((d) => getToday(d)))
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function shift(weeks: number) {
  const d = new Date(weekStart.value)
  d.setDate(d.getDate() + weeks * 7)
  weekStart.value = d
  void load()
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
  const f = form.value
  if (!f.title.trim() || f.start === null || f.end === null) return
  const [start, end] = [f.start, f.end]
  void run(async () => {
    await createEvent({
      title: f.title.trim(), start_at: new Date(start).toISOString(), end_at: new Date(end).toISOString(), location: f.location,
    })
    form.value = { title: '', start: null, end: null, location: '' }
  })
}

const time = (iso: string, tz: string) =>
  new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit', timeZone: tz }).format(new Date(iso))
const weekday = (date: string) =>
  new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'numeric' }).format(new Date(`${date}T12:00:00`))

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <n-space vertical :size="16">
    <n-h1 style="margin: 0">Kalender</n-h1>
    <n-alert v-if="error" type="error">{{ error }}</n-alert>

    <form @submit.prevent="add">
      <n-space>
        <n-input v-model:value="form.title" placeholder="Neuer Termin" style="width: 200px" />
        <n-date-picker v-model:value="form.start" type="datetime" placeholder="Beginn" style="width: 200px" />
        <n-date-picker v-model:value="form.end" type="datetime" placeholder="Ende" style="width: 200px" />
        <n-input v-model:value="form.location" placeholder="Ort" style="width: 140px" />
        <n-button type="primary" attr-type="submit">Anlegen</n-button>
      </n-space>
    </form>

    <n-space align="center">
      <n-button @click="shift(-1)">←</n-button>
      <n-text strong>{{ range }}</n-text>
      <n-button @click="shift(1)">→</n-button>
    </n-space>

    <n-card v-for="d in days" :key="d.date" :title="weekday(d.date)" size="small">
      <n-text v-if="!d.events.length" depth="3">–</n-text>
      <n-list v-else>
        <n-list-item v-for="e in d.events" :key="e.id + e.occurrence_start">
          <template #prefix><n-text depth="3">{{ time(e.occurrence_start, d.timezone) }}–{{ time(e.occurrence_end, d.timezone) }}</n-text></template>
          {{ e.title }} <n-text v-if="e.location" depth="3">{{ e.location }}</n-text>
          <template #suffix>
            <delete-button :text="`„${e.title}“ löschen? Bei Serien entfällt die ganze Serie.`" @confirm="run(() => deleteEvent(e.id))" />
          </template>
        </n-list-item>
      </n-list>
    </n-card>
  </n-space>
</template>
