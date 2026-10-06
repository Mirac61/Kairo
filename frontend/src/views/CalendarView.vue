<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { createEvent, deleteEvent, errorMessage, getToday, type Today } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { single, ymd } from '@/lib/dates'
import DeleteButton from '@/components/DeleteButton.vue'

// Wochenansicht: pro Tag ein /api/today?date=…, damit Serientermine als
// einzelne Vorkommen erscheinen. Neue Termine nutzen die Zeitzone des Browsers.
function monday(d: Date) {
  const m = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  m.setDate(m.getDate() - ((m.getDay() + 6) % 7))
  return m
}

const weekStart = ref(monday(new Date()))
const days = ref<Today[]>([])
const error = ref('')
const form = ref({ title: '', start: null as Date | null, end: null as Date | null, location: '' })

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
  if (!f.title.trim() || !f.start || !f.end) return
  const [start, end] = [f.start, f.end]
  void run(async () => {
    await createEvent({ title: f.title.trim(), start_at: start.toISOString(), end_at: end.toISOString(), location: f.location })
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
  <div class="stack">
    <h1 class="title">Kalender</h1>
    <Message v-if="error" severity="error">{{ error }}</Message>

    <form class="row" @submit.prevent="add">
      <InputText v-model="form.title" placeholder="Neuer Termin" class="w-title" />
      <DatePicker v-model="form.start" show-time hour-format="24" date-format="dd.mm.yy" placeholder="Beginn" class="w-dt" @update:model-value="(v) => (form.start = single(v))" />
      <DatePicker v-model="form.end" show-time hour-format="24" date-format="dd.mm.yy" placeholder="Ende" class="w-dt" />
      <InputText v-model="form.location" placeholder="Ort" class="w-loc" />
      <Button type="submit" label="Anlegen" />
    </form>

    <div class="row">
      <Button label="←" severity="secondary" @click="shift(-1)" />
      <b>{{ range }}</b>
      <Button label="→" severity="secondary" @click="shift(1)" />
    </div>

    <section v-for="d in days" :key="d.date" class="card">
      <h2>{{ weekday(d.date) }}</h2>
      <span v-if="!d.events.length" class="muted">–</span>
      <ul v-else class="list">
        <li v-for="e in d.events" :key="e.id + e.occurrence_start">
          <span class="time">{{ time(e.occurrence_start, d.timezone) }}–{{ time(e.occurrence_end, d.timezone) }}</span>
          <span class="main">{{ e.title }} <span v-if="e.location" class="muted">{{ e.location }}</span></span>
          <DeleteButton :text="`„${e.title}“ löschen? Bei Serien entfällt die ganze Serie.`" @confirm="run(() => deleteEvent(e.id))" />
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.w-title { width: 200px; }
.w-dt { width: 190px; }
.w-loc { width: 140px; }
</style>
