<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api, completeHabit, createHabit, deleteHabit, errorMessage, getToday, listHabits, restoreHabit, uncompleteHabit, updateHabit,
  type FrequencyConfig, type Habit, type TodayHabit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useUndo } from '@/composables/useUndo'
import { ymd } from '@/lib/dates'

const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
const TYPE_LABEL: Record<Habit['frequency_type'], string> = {
  DAILY: 'Täglich', WEEKLY: 'Wöchentlich', SPECIFIC_WEEKDAYS: 'Bestimmte Wochentage', TIMES_PER_WEEK: 'X-mal pro Woche',
}
const DAYS = 28

const habits = ref<Habit[]>([])
const todayHabits = ref<TodayHabit[]>([])
const done = ref<Record<string, Set<string>>>({})
const streaks = ref<Record<string, number>>({})
const error = ref('')
const dialog = ref(false)
const form = ref({ name: '', type: 'DAILY' as Habit['frequency_type'], weekday: 'MO', weekdays: ['MO'] as string[], times: 3 })

const today = () => ymd(new Date())
// Die letzten 28 Tage, älteste zuerst; der letzte Eintrag ist heute.
const days = () => Array.from({ length: DAYS }, (_, i) => {
  const d = new Date()
  d.setDate(d.getDate() - (DAYS - 1 - i))
  return d
})

async function load() {
  try {
    const t = today()
    const from = ymd(days()[0]!)
    const [hs, td, rv] = await Promise.all([
      listHabits(), getToday(t), api<{ habits: { habit_id: string; streak: number }[] }>(`/review?from=${t}&to=${t}`),
    ])
    const comps = await Promise.all(hs.map((h) =>
      api<{ date: string }[]>(`/habits/${h.id}/completions?from=${from}&to=${t}`)))
    habits.value = hs
    todayHabits.value = td.habits
    streaks.value = Object.fromEntries(rv.habits.map((h) => [h.habit_id, h.streak]))
    done.value = Object.fromEntries(hs.map((h, i) => [h.id, new Set(comps[i]!.map((c) => c.date))]))
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

const { offer } = useUndo()

// Löschen legt in den Papierkorb; Rückgängig holt das Habit samt Abhak-Verlauf zurück.
const trash = ({ id, name }: Habit) =>
  void run(async () => {
    await deleteHabit(id)
    offer(`„${name}“ gelöscht`, () => run(() => restoreHabit(id)))
  })

const toggle = (h: TodayHabit) => run(() => (h.done ? uncompleteHabit(h.id, today()) : completeHabit(h.id, today())))

function config(): FrequencyConfig {
  const f = form.value
  if (f.type === 'WEEKLY') return { weekday: f.weekday }
  if (f.type === 'SPECIFIC_WEEKDAYS') return { weekdays: f.weekdays }
  if (f.type === 'TIMES_PER_WEEK') return { times: f.times || 1 }
  return {}
}

function add() {
  const name = form.value.name.trim()
  if (!name) return
  void run(async () => {
    await createHabit({ name, frequency_type: form.value.type, frequency_config: config() })
    form.value.name = ''
    dialog.value = false
  })
}

function toggleDay(d: string) {
  const l = form.value.weekdays
  form.value.weekdays = l.includes(d) ? l.filter((x) => x !== d) : [...l, d]
}

function describe(h: Habit) {
  const c = h.frequency_config
  if (h.frequency_type === 'WEEKLY') return `Wöchentlich (${c.weekday ?? 'Startdatum'})`
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (c.weekdays ?? []).join(', ')
  if (h.frequency_type === 'TIMES_PER_WEEK') return `${c.times}× pro Woche`
  return TYPE_LABEL[h.frequency_type]
}

// Geplant ist ein Tag nur bei festen Wochentagen eingeschränkt; sonst zählt jeder Tag.
function planned(h: Habit, d: Date) {
  const wd = WEEKDAYS[(d.getDay() + 6) % 7]!
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (h.frequency_config.weekdays ?? []).includes(wd)
  if (h.frequency_type === 'WEEKLY' && h.frequency_config.weekday) return h.frequency_config.weekday === wd
  return true
}

// start_date liefert die API, steht aber nicht in Habit.
const startOf = (h: Habit) => (h as Habit & { start_date?: string }).start_date ?? ''
const dayLabel = new Intl.DateTimeFormat('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit' })
// pre: vor start_date (leer, nicht klickbar). Bei X-mal pro Woche ist ein offener Tag neutral statt "verpasst".
const cells = (h: Habit) => days().map((d, i) => {
  const s = ymd(d)
  const pre = s < startOf(h)
  const isDone = !!done.value[h.id]?.has(s)
  const last = i === DAYS - 1
  const cls = [pre ? 'pre' : isDone ? 'done' : !planned(h, d) ? 'off' : last || h.frequency_type === 'TIMES_PER_WEEK' ? '' : 'miss']
  if (last) cls.push('today')
  return { s, pre, isDone, can: h.active && !pre, cls: cls.join(' '), label: `${dayLabel.format(d)}: ${isDone ? 'erledigt' : 'offen'}` }
})
// Klick auf ein Feld holt den Tag nach bzw. nimmt ihn zurück.
const toggleCell = (h: Habit, c: { s: string; isDone: boolean }) =>
  run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))
const doneCount = (h: Habit) => done.value[h.id]?.size ?? 0
const week = (h: Habit) => days().slice(-7).filter((d) => done.value[h.id]?.has(ymd(d))).length
// Wochenfortschritt: das Backend liefert ihn nur, solange der Habit heute fällig ist; sonst aus den Tagen seit Montag.
const weekProgress = (h: Habit) => {
  const p = todayHabits.value.find((x) => x.id === h.id)?.week_progress
  const sinceMonday = days().slice(-(((new Date().getDay() + 6) % 7) + 1)).filter((d) => done.value[h.id]?.has(ymd(d))).length
  return `${p?.done ?? sinceMonday}/${p?.target ?? h.frequency_config.times ?? 1}`
}
const todayState = (h: Habit) => {
  const t = todayHabits.value.find((x) => x.id === h.id)
  return t ? (t.done ? 'Heute erledigt' : 'Heute offen') : 'Heute nicht fällig'
}
const sorted = computed(() => [...habits.value].sort((a, b) => Number(b.active) - Number(a.active)))

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">Gewohnheiten</h1>
        <div class="v-sub">Die letzten 28 Tage. Gefüllt heisst erledigt, gestrichelt heisst nicht geplant. Ein Klick auf ein Feld trägt den Tag nach oder nimmt ihn zurück.</div>
      </div>
      <button class="btn btn-secondary" type="button" @click="dialog = true"><svg class="ic"><use href="#i-plus" /></svg>Neue Gewohnheit</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="card hab-today">
      <span class="lbl">Heute abhaken</span>
      <div class="hab-chips">
        <button v-for="h in todayHabits" :key="h.id" type="button" class="hab-chip" role="checkbox" :aria-checked="h.done" @click="toggle(h)">
          <span class="cb cb-orange" aria-hidden="true"></span>
          <span>{{ h.name }}</span>
          <span v-if="h.done" class="hc-done">erledigt</span>
        </button>
        <div v-if="!todayHabits.length" class="v-sub">Heute ist keine Gewohnheit fällig.</div>
      </div>
    </div>

    <div class="hab-list">
      <div v-if="!habits.length" class="v-sub">Keine Gewohnheiten.</div>
      <div v-for="h in sorted" :key="h.id" class="hab-item" :class="{ inactive: !h.active }">
        <div class="hab-id">
          <span class="hab-name">{{ h.name }}</span>
          <span class="hab-sched">{{ describe(h) }}<template v-if="!h.active"> · inaktiv</template></span>
        </div>
        <div class="hab-num">
          <span class="n" :class="{ cold: !streaks[h.id] }">{{ streaks[h.id] ?? 0 }}</span>
          <span class="u">Tage Serie</span>
        </div>
        <div class="hab-track" role="group" :aria-label="`Letzte 28 Tage: ${doneCount(h)} Tage erledigt`">
          <i
            v-for="c in cells(h)" :key="c.s" :class="c.cls" :title="c.pre ? undefined : c.label" :aria-hidden="c.pre || undefined"
            :role="c.can ? 'checkbox' : undefined" :aria-checked="c.can ? c.isDone : undefined" :aria-label="c.can ? c.label : undefined" :tabindex="c.can ? 0 : undefined"
            @click="c.can && toggleCell(h, c)" @keydown.enter.prevent="c.can && toggleCell(h, c)" @keydown.space.prevent="c.can && toggleCell(h, c)"
          ></i>
        </div>
        <div class="hab-meta">
          <span v-if="h.frequency_type === 'TIMES_PER_WEEK'"><span class="mono">{{ weekProgress(h) }}</span> diese Woche · {{ todayState(h) }}</span>
          <span v-else>Letzte 7 Tage <span class="mono">{{ week(h) }}/7</span> · {{ todayState(h) }}</span>
          <span class="acts">
            <button class="btn btn-ghost" type="button" @click="run(() => updateHabit(h.id, { active: !h.active }))">{{ h.active ? 'Deaktivieren' : 'Aktivieren' }}</button>
            <button type="button" class="btn btn-danger" @click="trash(h)">Löschen</button>
          </span>
        </div>
      </div>
    </div>

    <div class="overlay" :class="{ open: dialog }" @click.self="dialog = false" @keydown.esc="dialog = false">
      <form class="dialog" role="dialog" aria-label="Neue Gewohnheit" @submit.prevent="add">
        <div class="dlg-head">
          <h3>Neue Gewohnheit</h3>
          <button class="icon-btn" type="button" aria-label="Schließen" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="field"><label for="h-name">Name</label><input id="h-name" v-model="form.name" class="input" placeholder="Neue Gewohnheit" /></div>
          <div class="field">
            <label for="h-type">Rhythmus</label>
            <select id="h-type" v-model="form.type" class="input">
              <option v-for="(l, v) in TYPE_LABEL" :key="v" :value="v">{{ l }}</option>
            </select>
          </div>
          <div v-if="form.type === 'WEEKLY'" class="field">
            <label for="h-day">Wochentag</label>
            <select id="h-day" v-model="form.weekday" class="input"><option v-for="d in WEEKDAYS" :key="d">{{ d }}</option></select>
          </div>
          <div v-if="form.type === 'SPECIFIC_WEEKDAYS'" class="seg">
            <button v-for="d in WEEKDAYS" :key="d" type="button" :aria-pressed="form.weekdays.includes(d)" @click="toggleDay(d)">{{ d }}</button>
          </div>
          <div v-if="form.type === 'TIMES_PER_WEEK'" class="field">
            <label for="h-times">Pro Woche</label>
            <input id="h-times" v-model.number="form.times" class="input" type="number" min="1" max="7" />
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" @click="dialog = false">Abbrechen</button>
          <button class="btn btn-primary" type="submit">Anlegen</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.inactive .hab-name, .inactive .hab-num { opacity: .5; }
.acts { display: inline-flex; gap: 4px; }
.hab-track i.pre { visibility: hidden; }
.hab-track i[role="checkbox"] { cursor: pointer; }
.hab-track i[role="checkbox"]:hover { border-color: var(--a-orange); }
</style>
