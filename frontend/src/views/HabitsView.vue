<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api, completeHabit, createHabit, deleteHabit, getToday, listHabits, restoreHabit, uncompleteHabit, updateHabit,
  type FrequencyConfig, type Habit, type TodayHabit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useUndo } from '@/composables/useUndo'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { habitColor } from '@/lib/projectColor'
import { locale, t } from '@/lib/i18n'
import SearchField from '@/components/SearchField.vue'

const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
const TYPE_LABEL: Record<Habit['frequency_type'], string> = {
  DAILY: t('Täglich'), WEEKLY: t('Wöchentlich'), SPECIFIC_WEEKDAYS: t('Bestimmte Wochentage'), TIMES_PER_WEEK: t('X-mal pro Woche'),
}
const DAYS = 28

const habits = ref<Habit[]>([])
const todayHabits = ref<TodayHabit[]>([])
const done = ref<Record<string, Set<string>>>({})
const streaks = ref<Record<string, number>>({})
const dialog = ref(false)
const form = ref({ name: '', type: 'DAILY' as Habit['frequency_type'], weekday: 'MO', weekdays: ['MO'] as string[], times: 3 })

const today = () => ymd(new Date())
// Die letzten 28 Tage, älteste zuerst; der letzte Eintrag ist heute.
const days = () => Array.from({ length: DAYS }, (_, i) => {
  const d = new Date()
  d.setDate(d.getDate() - (DAYS - 1 - i))
  return d
})

const { error, load, run } = useLoader(async () => {
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
})

const { offer } = useUndo()

// Löschen legt in den Papierkorb; Rückgängig holt das Habit samt Abhak-Verlauf zurück.
const trash = ({ id, name }: Habit) =>
  void run(async () => {
    await deleteHabit(id)
    offer(t('„{title}“ gelöscht', { title: name }), () => run(() => restoreHabit(id)))
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
  if (h.frequency_type === 'WEEKLY') return `${t('Wöchentlich')} (${c.weekday ?? t('Startdatum')})`
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (c.weekdays ?? []).join(', ')
  if (h.frequency_type === 'TIMES_PER_WEEK') return t('{n}× pro Woche', { n: c.times ?? 0 })
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
const dayLabel = new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: '2-digit', month: '2-digit' })
// pre: vor start_date (leer, nicht klickbar). Bei X-mal pro Woche ist ein offener Tag neutral statt "verpasst".
const cells = (h: Habit) => days().map((d, i) => {
  const s = ymd(d)
  const pre = s < startOf(h)
  const isDone = !!done.value[h.id]?.has(s)
  const last = i === DAYS - 1
  const cls = [pre ? 'pre' : isDone ? 'done' : !planned(h, d) ? 'off' : last || h.frequency_type === 'TIMES_PER_WEEK' ? '' : 'miss']
  if (last) cls.push('today')
  return { s, pre, isDone, can: h.active && !pre, cls: cls.join(' '), label: `${dayLabel.format(d)}: ${isDone ? t('erledigt') : t('offen')}` }
})
// Klick auf ein Feld holt den Tag nach bzw. nimmt ihn zurück.
const toggleCell = (h: Habit, c: { s: string; isDone: boolean }) =>
  run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))
const doneCount = (h: Habit) => done.value[h.id]?.size ?? 0
const week = (h: Habit) => days().slice(-7).filter((d) => done.value[h.id]?.has(ymd(d))).length
// Letzte 7 Tage; bei X-mal pro Woche der Wochenfortschritt (das Backend liefert ihn nur, solange der Habit heute fällig ist; sonst aus den Tagen seit Montag).
const weekOf = (h: Habit) => {
  if (h.frequency_type !== 'TIMES_PER_WEEK') return { done: week(h), target: 7 }
  const p = todayHabits.value.find((x) => x.id === h.id)?.week_progress
  const sinceMonday = days().slice(-(((new Date().getDay() + 6) % 7) + 1)).filter((d) => done.value[h.id]?.has(ymd(d))).length
  return { done: p?.done ?? sinceMonday, target: p?.target ?? h.frequency_config.times ?? 1 }
}
const due = (h: Habit) => todayHabits.value.find((x) => x.id === h.id)
const cardSub = (h: Habit) => {
  const x = due(h)
  return x ? `${x.done ? t('erledigt') : t('offen')} · ${t('Serie')} ${streaks.value[h.id] ?? 0}` : t('heute nicht fällig')
}
const activeHabits = computed(() => habits.value.filter((h) => h.active))
const headline = computed(() =>
  t('{active} aktiv · heute {done} von {due} erledigt', { active: activeHabits.value.length, done: todayHabits.value.filter((h) => h.done).length, due: todayHabits.value.length }))
const dayHead = new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'short' }).format(new Date())
// Vier Wochenblöcke à 7 Tage, der letzte endet heute.
const monthShort = new Intl.DateTimeFormat(locale.value, { month: 'short' })
const weekLabels = Array.from({ length: 4 }, (_, g) => {
  const a = days()[g * 7]!
  const b = days()[g * 7 + 6]!
  return a.getMonth() === b.getMonth()
    ? `${a.getDate()}.–${b.getDate()}. ${monthShort.format(b)}`
    : `${a.getDate()}. ${monthShort.format(a)}–${b.getDate()}. ${monthShort.format(b)}`
})
const weeks = (h: Habit) => { const c = cells(h); return [0, 1, 2, 3].map((g) => c.slice(g * 7, g * 7 + 7)) }
const search = ref('')
const sorted = computed(() => {
  const q = search.value.trim().toLowerCase()
  return habits.value.filter((h) => !q || h.name.toLowerCase().includes(q)).sort((a, b) => Number(b.active) - Number(a.active))
})

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">{{ $t('Gewohnheiten') }}</h1>
        <div class="v-sub">{{ headline }}</div>
      </div>
      <button class="btn btn-primary" type="button" @click="dialog = true"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neue Gewohnheit') }}</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <section class="hab-today">
      <h2 class="hab-day">{{ $t('Heute') }} · {{ dayHead }}</h2>
      <div class="hab-cards">
        <button v-for="h in activeHabits" :key="h.id" type="button" class="hab-chip" role="checkbox" :aria-checked="!!due(h)?.done" :disabled="!due(h)" @click="toggle(due(h)!)">
          <span class="cb sq" :style="{ '--rc': habitColor(h.id) }" aria-hidden="true" :aria-checked="!!due(h)?.done"></span>
          <span class="hc-text"><span class="hc-name">{{ h.name }}</span><span class="hc-sub">{{ cardSub(h) }}</span></span>
        </button>
        <div v-if="!activeHabits.length" class="v-sub">{{ $t('Keine aktive Gewohnheit.') }}</div>
      </div>
    </section>

    <div class="filterbar"><SearchField v-model="search" :label="$t('Gewohnheiten durchsuchen')" /></div>
    <div v-if="!habits.length" class="v-sub">{{ $t('Keine Gewohnheiten.') }}</div>
    <div v-else-if="!sorted.length" class="v-sub">{{ $t('Keine Treffer für „{q}“.', { q: search }) }}</div>
    <div v-else class="card hab-card">
      <div class="hab-row hab-head" aria-hidden="true">
        <span class="lbl">{{ $t('Gewohnheit') }}</span>
        <div class="hab-weeks"><span v-for="l in weekLabels" :key="l" class="lbl">{{ l }}</span></div>
        <span class="lbl hab-r">{{ $t('Serie') }}</span>
        <span class="lbl hab-r">{{ $t('7 Tage') }}</span>
      </div>
      <div v-for="h in sorted" :key="h.id" class="hab-row hab-item" :class="{ inactive: !h.active }" :style="{ '--hc': habitColor(h.id) }">
        <div class="hab-id">
          <span class="hab-name"><i class="hab-dot"></i>{{ h.name }}</span>
          <span class="hab-sub">
            <span class="hab-sched">{{ describe(h) }}<template v-if="!h.active"> {{ $t('· inaktiv') }}</template></span>
            <span class="acts">
              <button class="btn btn-ghost" type="button" @click="run(() => updateHabit(h.id, { active: !h.active }))">{{ h.active ? 'Deaktivieren' : 'Aktivieren' }}</button>
              <button type="button" class="btn btn-ghost" @click="trash(h)">{{ $t('Löschen') }}</button>
            </span>
          </span>
        </div>
        <div class="hab-weeks hab-track" role="group" :aria-label="$t('Letzte 28 Tage: {n} Tage erledigt', { n: doneCount(h) })">
          <div v-for="(wk, g) in weeks(h)" :key="g" class="hab-wk">
            <i
              v-for="c in wk" :key="c.s" :class="c.cls" :title="c.pre ? undefined : c.label" :aria-hidden="c.pre || undefined"
              :role="c.can ? 'checkbox' : undefined" :aria-checked="c.can ? c.isDone : undefined" :aria-label="c.can ? c.label : undefined" :tabindex="c.can ? 0 : undefined"
              @click="c.can && toggleCell(h, c)" @keydown.enter.prevent="c.can && toggleCell(h, c)" @keydown.space.prevent="c.can && toggleCell(h, c)"
            ></i>
          </div>
        </div>
        <div class="hab-num hab-r"><span class="n" :class="{ cold: !streaks[h.id] }">{{ streaks[h.id] ?? 0 }}</span></div>
        <div class="hab-wkn hab-r" :title="h.frequency_type === 'TIMES_PER_WEEK' ? $t('Diese Woche') : $t('Letzte 7 Tage')">
          <span class="mono">{{ weekOf(h).done }}/{{ weekOf(h).target }}</span>
          <div class="pbar" aria-hidden="true"><div class="pbar-fill" :style="{ width: Math.min(100, (weekOf(h).done / weekOf(h).target) * 100) + '%' }"></div></div>
        </div>
      </div>
      <div class="hab-foot">
        <span class="hab-legend"><i class="done"></i>{{ $t('erledigt') }}</span>
        <span class="hab-legend"><i></i>{{ $t('offen') }}</span>
        <span class="hab-legend"><i class="off"></i>{{ $t('nicht geplant') }}</span>
        <span class="hab-legend"><i class="today"></i>{{ $t('heute') }}</span>
        <span class="hab-hint">{{ $t('Klick auf ein Feld trägt nach oder nimmt zurück') }}</span>
      </div>
    </div>

    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <form class="dialog" :aria-label="$t('Neue Gewohnheit')" @submit.prevent="add">
        <div class="dlg-head">
          <h3>{{ $t('Neue Gewohnheit') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="field"><label for="h-name">{{ $t('Name') }}</label><input id="h-name" v-model="form.name" class="input" :placeholder="$t('Neue Gewohnheit')" /></div>
          <div class="field">
            <label for="h-type">{{ $t('Rhythmus') }}</label>
            <select id="h-type" v-model="form.type" class="input">
              <option v-for="(l, v) in TYPE_LABEL" :key="v" :value="v">{{ l }}</option>
            </select>
          </div>
          <div v-if="form.type === 'WEEKLY'" class="field">
            <label for="h-day">{{ $t('Wochentag') }}</label>
            <select id="h-day" v-model="form.weekday" class="input"><option v-for="d in WEEKDAYS" :key="d">{{ d }}</option></select>
          </div>
          <div v-if="form.type === 'SPECIFIC_WEEKDAYS'" class="seg">
            <button v-for="d in WEEKDAYS" :key="d" type="button" :aria-pressed="form.weekdays.includes(d)" @click="toggleDay(d)">{{ d }}</button>
          </div>
          <div v-if="form.type === 'TIMES_PER_WEEK'" class="field">
            <label for="h-times">{{ $t('Pro Woche') }}</label>
            <input id="h-times" v-model.number="form.times" class="input" type="number" min="1" max="7" />
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" @click="dialog = false">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit">{{ $t('Anlegen') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.inactive .hab-name, .inactive .hab-num { opacity: .5; }
/* Beim Überfahren ersetzen die Aktionen die Rhythmus-Zeile. */
.hab-sub { display: grid; align-items: center; min-height: 20px; }
.hab-sub > * { grid-area: 1 / 1; }
.acts { display: inline-flex; gap: 2px; margin-left: -8px; opacity: 0; pointer-events: none; transition: opacity var(--dur) ease-out; }
.acts .btn { min-height: 0; height: 20px; padding: 0 8px; font-size: 12px; }
.hab-item:hover .acts, .hab-item:focus-within .acts { opacity: 1; pointer-events: auto; }
.hab-item:hover .hab-sched, .hab-item:focus-within .hab-sched { opacity: 0; }
.hab-track i.pre { visibility: hidden; }
.hab-track i[role="checkbox"] { cursor: pointer; }
.hab-track i[role="checkbox"]:hover { border-color: var(--tx-primary); }
</style>
