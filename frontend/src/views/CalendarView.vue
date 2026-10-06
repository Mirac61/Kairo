<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  createEvent,
  deleteEvent,
  errorMessage,
  getToday,
  type Today,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'

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
const form = ref({ title: '', start: '', end: '', location: '' })

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
  void run(async () => {
    await createEvent({
      title: f.title.trim(),
      start_at: new Date(f.start).toISOString(),
      end_at: new Date(f.end).toISOString(),
      location: f.location,
    })
    form.value = { title: '', start: '', end: '', location: '' }
  })
}

function remove(id: string, title: string) {
  if (confirm(`„${title}“ löschen? Bei Serien entfällt die ganze Serie.`)) void run(() => deleteEvent(id))
}

const time = (iso: string, tz: string) =>
  new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit', timeZone: tz }).format(new Date(iso))
const weekday = (date: string) =>
  new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'numeric' }).format(new Date(`${date}T12:00:00`))

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <section>
    <h1>Kalender</h1>
    <p v-if="error" class="error">{{ error }}</p>

    <form class="row" @submit.prevent="add">
      <input v-model="form.title" placeholder="Neuer Termin" required />
      <input v-model="form.start" type="datetime-local" required />
      <input v-model="form.end" type="datetime-local" required />
      <input v-model="form.location" placeholder="Ort" />
      <button>Anlegen</button>
    </form>

    <div class="nav">
      <button @click="shift(-1)">←</button>
      <strong>{{ range }}</strong>
      <button @click="shift(1)">→</button>
    </div>

    <div v-for="d in days" :key="d.date" class="day">
      <h2>{{ weekday(d.date) }}</h2>
      <p v-if="!d.events.length" class="hint">–</p>
      <ul class="list">
        <li v-for="e in d.events" :key="e.id + e.occurrence_start">
          <span class="time">{{ time(e.occurrence_start, d.timezone) }}–{{ time(e.occurrence_end, d.timezone) }}</span>
          <span class="title">{{ e.title }} <small v-if="e.location">{{ e.location }}</small></span>
          <button @click="remove(e.id, e.title)">Löschen</button>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 16px; font-size: 24px; }
h2 { margin: 16px 0 6px; font-size: 15px; }
.hint { margin: 0; color: var(--text-muted); }
.error { color: var(--err); }
.row, .nav { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 12px; }
.row > input:first-child { flex: 1; min-width: 160px; }
input, button {
  padding: 4px 10px; border: 1px solid var(--border); border-radius: 6px;
  background: var(--surface); color: var(--text); font: inherit;
}
button { background: var(--bg); cursor: pointer; }
button:hover { border-color: var(--accent); color: var(--accent); }
.list { list-style: none; margin: 0; padding: 0; }
.list li {
  display: flex; align-items: center; gap: 12px; padding: 8px 12px; margin-bottom: 4px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.time { width: 110px; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.title { flex: 1; }
.title small { margin-left: 8px; color: var(--text-muted); font-size: 13px; }
</style>
