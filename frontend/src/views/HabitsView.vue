<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  api, completeHabit, createHabit, deleteHabit, getToday, listHabits, restoreHabit, uncompleteHabit, updateHabit,
  type FrequencyConfig, type Habit, type TodayHabit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useUndo } from '@/composables/useUndo'
import { vDialog } from '@/lib/dialog'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { habitColor } from '@/lib/projectColor'
import { locale, t } from '@/lib/i18n'
import SearchField from '@/components/SearchField.vue'

const WEEKDAYS = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU']
// 01.01.2024 war ein Montag.
const wdFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
const WD_LABEL = Object.fromEntries(WEEKDAYS.map((d, i) => [d, wdFmt.format(new Date(2024, 0, 1 + i))]))
// WEEKLY bietet der Dialog nicht mehr an: ein einzelner Tag unter „Bestimmte Tage“ ist dasselbe.
type Rhythm = 'DAILY' | 'SPECIFIC_WEEKDAYS' | 'TIMES_PER_WEEK'
const RHYTHMS: Record<Rhythm, string> = { DAILY: t('Täglich'), SPECIFIC_WEEKDAYS: t('Bestimmte Tage'), TIMES_PER_WEEK: t('X-mal pro Woche') }
// Graph: 53 Wochen bis einschließlich dieser Woche, Montag oben (wie der GitHub-Beitragsgraph).
const WEEKS = 53

const habits = ref<Habit[]>([])
const todayHabits = ref<TodayHabit[]>([])
const done = ref<Record<string, Set<string>>>({})
const streaks = ref<Record<string, number>>({})
const dialog = ref(false)
const blank = () => ({
  id: '', name: '', description: '', type: 'DAILY' as Rhythm, weekdays: ['MO'] as string[], times: 3,
  time: '', duration: 30, target: null as number | null, unit: '', start: ymd(new Date()), end: '',
})
const form = ref(blank())

// Ansicht: Tag (Wochenstreifen und Karten), Monat (Kalender je Gewohnheit) oder Jahr (Beitragsgraph); je Browser gemerkt.
type View = 'day' | 'month' | 'year'
const VIEW_KEY = 'kairo-habits-view'
const view = ref<View>((['day', 'month', 'year'] as const).find((v) => v === store.get(VIEW_KEY)) ?? 'day')
watch(view, (v) => store.set(VIEW_KEY, v))

const today = () => ymd(new Date())
const gridStart = () => addDays(weekStart(), -7 * (WEEKS - 1))

const { error, load, run } = useLoader(async () => {
  const t = today()
  const from = ymd(gridStart())
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


function config(): FrequencyConfig {
  const f = form.value
  if (f.type === 'SPECIFIC_WEEKDAYS') return { weekdays: f.weekdays }
  if (f.type === 'TIMES_PER_WEEK') return { times: f.times || 1 }
  return {}
}

function openNew() {
  form.value = blank()
  dialog.value = true
}

function openEdit(h: Habit) {
  const c = h.frequency_config
  form.value = {
    ...blank(), id: h.id, name: h.name, description: h.description,
    type: h.frequency_type === 'WEEKLY' ? 'SPECIFIC_WEEKDAYS' : h.frequency_type,
    weekdays: h.frequency_type === 'WEEKLY' ? [c.weekday ?? 'MO'] : c.weekdays ?? ['MO'], times: c.times ?? 3,
    time: h.preferred_time ?? '', duration: h.duration_minutes, target: h.target_value, unit: h.unit,
    start: h.start_date, end: h.end_date ?? '',
  }
  dialog.value = true
}

// Leere Felder: "" bzw. target_value 0 entfernen beim Bearbeiten den Wert, beim Anlegen gilt der Standard.
function save() {
  const f = form.value
  const name = f.name.trim()
  if (!name) return
  const body = {
    name, description: f.description.trim(), frequency_type: f.type, frequency_config: config(),
    preferred_time: f.time, duration_minutes: f.duration || 30, target_value: f.target || (f.id ? 0 : null),
    unit: f.unit, start_date: f.start, end_date: f.end,
  }
  void run(async () => {
    await (f.id ? updateHabit(f.id, body) : createHabit(body))
    dialog.value = false
  })
}

function toggleDay(d: string) {
  const l = form.value.weekdays
  form.value.weekdays = l.includes(d) ? l.filter((x) => x !== d) : [...l, d]
}

function rhythm(h: Habit) {
  const c = h.frequency_config
  if (h.frequency_type === 'WEEKLY') return c.weekday ? WD_LABEL[c.weekday] : t('Wöchentlich')
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (c.weekdays ?? []).map((d) => WD_LABEL[d]).join(', ')
  if (h.frequency_type === 'TIMES_PER_WEEK') return t('{n}× pro Woche', { n: c.times ?? 0 })
  return t('Täglich')
}
// z. B. „Mo, Do · 18:00 · 20 Seiten“
const describe = (h: Habit) =>
  [rhythm(h), h.preferred_time, h.target_value && `${h.target_value} ${h.unit}`.trim()].filter(Boolean).join(' · ')

// Geplant ist ein Tag nur bei festen Wochentagen eingeschränkt; sonst zählt jeder Tag.
function planned(h: Habit, d: Date) {
  const wd = WEEKDAYS[(d.getDay() + 6) % 7]!
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return (h.frequency_config.weekdays ?? []).includes(wd)
  if (h.frequency_type === 'WEEKLY' && h.frequency_config.weekday) return h.frequency_config.weekday === wd
  return true
}

const dayLabel = new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: '2-digit', month: '2-digit' })
// Zustand eines Tages: future/pre (außerhalb, nicht klickbar), done, open (geplant, offen), off (nicht geplant).
// Bei X-mal pro Woche zählt jeder Tag als geplant; offen ist dort kein Versäumnis.
type State = 'done' | 'open' | 'off' | 'pre' | 'future'
function cell(h: Habit, d: Date) {
  const s = ymd(d)
  const isDone = !!done.value[h.id]?.has(s)
  const state: State = s > today() ? 'future' : isDone ? 'done' : s < h.start_date || (h.end_date && s > h.end_date) ? 'pre' : planned(h, d) ? 'open' : 'off'
  const can = h.active && state !== 'future' && state !== 'pre'
  return { s, state, isDone, can, today: s === today(), label: `${dayLabel.format(d)}: ${isDone ? t('erledigt') : t('offen')}` }
}
type Cell = ReturnType<typeof cell>
const graph = (h: Habit) => Array.from({ length: WEEKS * 7 }, (_, i) => cell(h, addDays(gridStart(), i)))
// Monatsname über der ersten Woche, die in einem neuen Monat beginnt.
const monthShort = new Intl.DateTimeFormat(locale.value, { month: 'short' })
const months = computed(() => Array.from({ length: WEEKS }, (_, w) => addDays(gridStart(), 7 * w))
  .map((d, w, all) => ({ col: w + 1, label: w === 0 || d.getMonth() !== all[w - 1]!.getMonth() ? monthShort.format(d) : '' }))
  .filter((m) => m.label && m.col < WEEKS - 1))

// Klick auf ein Feld holt den Tag nach bzw. nimmt ihn zurück.
const toggleCell = (h: Habit, c: Cell) => c.can && run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))

const due = (h: Habit) => todayHabits.value.find((x) => x.id === h.id)
const toggleToday = (h: Habit) => { const x = due(h); if (x) void run(() => (x.done ? uncompleteHabit(h.id, today()) : completeHabit(h.id, today()))) }
const yearCount = (h: Habit) => done.value[h.id]?.size ?? 0
// Quote der letzten 30 Tage: erledigte durch geplante Tage (bei X-mal pro Woche das anteilige Ziel), höchstens 100 %.
function rate(h: Habit) {
  const days = Array.from({ length: 30 }, (_, i) => addDays(new Date(), -i)).filter((d) => ymd(d) >= h.start_date)
  if (!days.length) return null
  const hits = days.filter((d) => done.value[h.id]?.has(ymd(d))).length
  const goal = h.frequency_type === 'TIMES_PER_WEEK' ? ((h.frequency_config.times ?? 1) * days.length) / 7 : days.filter((d) => planned(h, d)).length
  return goal ? Math.min(100, Math.round((hits / goal) * 100)) : null
}

// Fortschritt einer Woche: erledigt gegen das Ziel (geplante Tage bzw. X bei X-mal pro Woche).
const weekOf = (d: Date) => Array.from({ length: 7 }, (_, i) => addDays(weekStart(d), i))
function weekProgress(h: Habit, days = weekOf(new Date())) {
  const target = h.frequency_type === 'TIMES_PER_WEEK' ? h.frequency_config.times ?? 1
    : days.filter((d) => planned(h, d) && ymd(d) >= h.start_date && (!h.end_date || ymd(d) <= h.end_date)).length
  return { done: days.filter((d) => done.value[h.id]?.has(ymd(d))).length, target }
}

// Tagesansicht: fällig ist ein Habit an geplanten Tagen im Zeitraum; bei X-mal pro Woche nur,
// solange das Wochenziel vor diesem Tag noch nicht erreicht ist (ein erledigter Tag bleibt sichtbar).
function dueOn(h: Habit, d: Date) {
  const c = cell(h, d)
  if (!h.active || c.state === 'pre' || c.state === 'off') return false
  if (c.isDone || h.frequency_type !== 'TIMES_PER_WEEK') return true
  return weekProgress(h, weekOf(d)).done < (h.frequency_config.times ?? 1)
}
const selDay = ref(new Date())
const strip = computed(() => weekOf(selDay.value).map((d) => {
  const hs = activeHabits.value.filter((h) => dueOn(h, d))
  const n = hs.filter((h) => done.value[h.id]?.has(ymd(d))).length
  return { d, s: ymd(d), due: hs.length, done: n, pct: hs.length ? n / hs.length : 0 }
}))
const shiftWeek = (n: number) => (selDay.value = addDays(selDay.value, 7 * n))
const wdShort = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
const selLabel = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }).format(selDay.value))
const SLOTS = [[t('Morgens'), '00:00'], [t('Mittags'), '12:00'], [t('Abends'), '17:00']] as const
// Fällige Habits des gewählten Tags nach Tageszeit; ohne Uhrzeit unter „Jederzeit“.
const dayGroups = computed(() => {
  const hs = sorted.value.filter((h) => dueOn(h, selDay.value))
    .sort((a, b) => (a.preferred_time ?? '99').localeCompare(b.preferred_time ?? '99'))
  const slot = (h: Habit) => h.preferred_time ? [...SLOTS].reverse().find(([, from]) => h.preferred_time! >= from)![0] : t('Jederzeit')
  return [...SLOTS.map(([l]) => l), t('Jederzeit')].map((label) => ({ label, habits: hs.filter((h) => slot(h) === label) })).filter((g) => g.habits.length)
})
const notDue = computed(() => sorted.value.filter((h) => h.active && !dueOn(h, selDay.value)))
const selCell = (h: Habit) => cell(h, selDay.value)

// Monatsansicht: Kalender je Gewohnheit, höchstens zwölf Monate zurück (so weit reichen die geladenen Daten).
const monthOffset = ref(0)
const monthStart = computed(() => { const n = new Date(); return new Date(n.getFullYear(), n.getMonth() + monthOffset.value, 1) })
const monthTitle = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(monthStart.value))
const monthDays = computed(() => {
  const m = monthStart.value
  return Array.from({ length: new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate() }, (_, i) => new Date(m.getFullYear(), m.getMonth(), i + 1))
})
const lead = computed(() => (monthStart.value.getDay() + 6) % 7)
function monthStat(h: Habit) {
  const cs = monthDays.value.map((d) => cell(h, d))
  return { done: cs.filter((c) => c.isDone).length, goal: cs.filter((c) => c.state === 'open' || c.state === 'done').length }
}

const activeHabits = computed(() => habits.value.filter((h) => h.active))
const dueToday = computed(() => todayHabits.value.length)
const doneToday = computed(() => todayHabits.value.filter((h) => h.done).length)
const best = computed(() => activeHabits.value.reduce<Habit | null>((a, h) => ((streaks.value[h.id] ?? 0) > (a ? streaks.value[a.id] ?? 0 : 0) ? h : a), null))
const avgRate = computed(() => {
  const rs = activeHabits.value.map(rate).filter((r): r is number => r !== null)
  return rs.length ? Math.round(rs.reduce((a, b) => a + b, 0) / rs.length) : null
})
const weekTotal = computed(() => activeHabits.value.map((h) => weekProgress(h)).reduce((a, p) => ({ done: a.done + p.done, target: a.target + p.target }), { done: 0, target: 0 }))
const dayHead = new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())
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
        <div class="v-sub">{{ dayHead }}</div>
      </div>
      <button class="btn btn-primary" type="button" @click="openNew"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neue Gewohnheit') }}</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="hk-stats">
      <div class="hk-stat">
        <span class="lbl">{{ $t('Heute') }}</span>
        <b>{{ doneToday }}<small>/{{ dueToday }}</small></b>
        <div class="pbar" aria-hidden="true"><div class="pbar-fill" :style="{ width: (dueToday ? (doneToday / dueToday) * 100 : 0) + '%' }"></div></div>
      </div>
      <div class="hk-stat">
        <span class="lbl">{{ $t('Diese Woche') }}</span>
        <b>{{ weekTotal.done }}<small>/{{ weekTotal.target }}</small></b>
        <div class="pbar" aria-hidden="true"><div class="pbar-fill" :style="{ width: (weekTotal.target ? Math.min(100, (weekTotal.done / weekTotal.target) * 100) : 0) + '%' }"></div></div>
      </div>
      <div class="hk-stat">
        <span class="lbl">{{ $t('Längste Serie') }}</span>
        <b>{{ best ? streaks[best.id] : 0 }}</b>
        <span class="hk-note">{{ best?.name ?? $t('noch keine') }}</span>
      </div>
      <div class="hk-stat">
        <span class="lbl">{{ $t('Quote 30 Tage') }}</span>
        <b>{{ avgRate ?? '–' }}<small v-if="avgRate !== null"> %</small></b>
        <span class="hk-note">{{ $t('{n} aktiv', { n: activeHabits.length }) }}</span>
      </div>
    </div>

    <div class="filterbar">
      <div class="seg" role="group" :aria-label="$t('Ansicht')">
        <button type="button" :aria-pressed="view === 'day'" @click="view = 'day'">{{ $t('Tag') }}</button>
        <button type="button" :aria-pressed="view === 'month'" @click="view = 'month'">{{ $t('Monat') }}</button>
        <button type="button" :aria-pressed="view === 'year'" @click="view = 'year'">{{ $t('Jahr') }}</button>
      </div>
      <span class="spacer"></span>
      <SearchField v-model="search" :label="$t('Gewohnheiten durchsuchen')" />
    </div>
    <div v-if="!habits.length" class="v-sub">{{ $t('Keine Gewohnheiten.') }}</div>
    <div v-else-if="!sorted.length" class="v-sub">{{ $t('Keine Treffer für „{q}“.', { q: search }) }}</div>

    <div v-else-if="view === 'year'" class="hk-list">
      <article v-for="h in sorted" :key="h.id" class="card hk-card" :class="{ inactive: !h.active }" :style="{ '--hc': habitColor(h.id) }">
        <header class="hk-head">
          <span class="hk-icon" aria-hidden="true">{{ h.name.slice(0, 1).toUpperCase() }}</span>
          <div class="hk-id">
            <span class="hk-name">{{ h.name }}</span>
            <span class="hab-sub">
              <span class="hab-sched">{{ h.description || describe(h) }}<template v-if="!h.active"> {{ $t('· inaktiv') }}</template></span>
              <span class="acts">
                <button class="btn btn-ghost" type="button" @click="openEdit(h)">{{ $t('Bearbeiten') }}</button>
                <button class="btn btn-ghost" type="button" @click="run(() => updateHabit(h.id, { active: !h.active }))">{{ h.active ? $t('Deaktivieren') : $t('Aktivieren') }}</button>
                <button type="button" class="btn btn-ghost" @click="trash(h)">{{ $t('Löschen') }}</button>
              </span>
            </span>
          </div>
          <div class="hk-chips">
            <span v-if="h.description" class="chip">{{ describe(h) }}</span>
            <span class="hk-kpi"><b>{{ streaks[h.id] ?? 0 }}</b>{{ $t('Serie') }}</span>
            <span class="hk-kpi"><b>{{ rate(h) ?? '–' }}<template v-if="rate(h) !== null"> %</template></b>{{ $t('30 Tage') }}</span>
            <span class="hk-kpi"><b>{{ yearCount(h) }}</b>{{ $t('im Jahr') }}</span>
          </div>
          <button
            type="button" class="hk-check" role="checkbox" :aria-checked="!!due(h)?.done" :disabled="!due(h)"
            :aria-label="$t('Heute abhaken: {name}', { name: h.name })" :title="due(h) ? $t('Heute abhaken: {name}', { name: h.name }) : $t('heute nicht fällig')" @click="toggleToday(h)"
          ><svg class="ic" aria-hidden="true"><use href="#i-check" /></svg></button>
        </header>
        <div class="hk-graph" role="group" :aria-label="$t('Letztes Jahr: {n} Tage erledigt', { n: yearCount(h) })">
          <div class="hk-months" aria-hidden="true"><span v-for="m in months" :key="m.col" :style="{ gridColumn: m.col }">{{ m.label }}</span></div>
          <div class="hk-grid">
            <i
              v-for="c in graph(h)" :key="c.s" :class="[c.state, { today: c.today }]" :title="c.state === 'future' || c.state === 'pre' ? undefined : c.label"
              :role="c.can ? 'checkbox' : undefined" :aria-checked="c.can ? c.isDone : undefined" :aria-label="c.can ? c.label : undefined" :aria-hidden="c.can ? undefined : 'true'"
              @click="toggleCell(h, c)"
            ></i>
          </div>
        </div>
      </article>
      <div class="hab-foot">
        <span class="hab-legend"><i class="done"></i>{{ $t('erledigt') }}</span>
        <span class="hab-legend"><i></i>{{ $t('offen') }}</span>
        <span class="hab-legend"><i class="off"></i>{{ $t('nicht geplant') }}</span>
        <span class="hab-hint">{{ $t('Klick auf ein Feld trägt nach oder nimmt zurück') }}</span>
      </div>
    </div>

    <div v-else-if="view === 'day'" class="hd">
      <div class="hd-strip">
        <button class="icon-btn" type="button" :aria-label="$t('Vorige Woche')" @click="shiftWeek(-1)"><svg class="ic"><use href="#i-left" /></svg></button>
        <button
          v-for="x in strip" :key="x.s" type="button" class="hd-day" :class="{ sel: x.s === ymd(selDay), today: x.s === today(), future: x.s > today() }"
          :aria-pressed="x.s === ymd(selDay)" :aria-label="`${wdShort.format(x.d)} ${x.d.getDate()}: ${x.done}/${x.due}`" @click="selDay = x.d"
        >
          <small>{{ wdShort.format(x.d).replace('.', '') }}</small>
          <span class="hd-ring" :class="{ full: x.due && x.done === x.due }" :style="{ '--p': x.pct }"><b>{{ x.d.getDate() }}</b></span>
          <span class="hd-cnt">{{ x.due ? `${x.done}/${x.due}` : '–' }}</span>
        </button>
        <button class="icon-btn" type="button" :aria-label="$t('Nächste Woche')" :disabled="weekStart(selDay) >= weekStart()" @click="shiftWeek(1)"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
      <h2 class="hd-title">{{ ymd(selDay) === today() ? $t('Heute') : selLabel }}</h2>
      <div v-if="!dayGroups.length" class="v-sub">{{ $t('Für diesen Tag ist nichts geplant.') }}</div>
      <section v-for="g in dayGroups" :key="g.label" class="hd-group">
        <span class="lbl">{{ g.label }}</span>
        <div class="hd-cards">
          <div
            v-for="h in g.habits" :key="h.id" class="hd-card" :class="{ done: selCell(h).isDone, locked: !selCell(h).can }" :style="{ '--hc': habitColor(h.id) }"
            role="checkbox" :aria-checked="selCell(h).isDone" :aria-disabled="!selCell(h).can" :tabindex="selCell(h).can ? 0 : -1"
            @click="toggleCell(h, selCell(h))" @keydown.enter.prevent="toggleCell(h, selCell(h))" @keydown.space.prevent="toggleCell(h, selCell(h))"
          >
            <span class="hk-icon" aria-hidden="true">{{ h.name.slice(0, 1).toUpperCase() }}</span>
            <span class="hk-id">
              <span class="hk-name">{{ h.name }}</span>
              <span class="hab-sched">
                {{ [h.preferred_time, h.target_value && `${h.target_value} ${h.unit}`.trim(), h.frequency_type === 'TIMES_PER_WEEK' && $t('{done}/{target} diese Woche', weekProgress(h, weekOf(selDay)))].filter(Boolean).join(' · ') || rhythm(h) }}
              </span>
            </span>
            <span class="hd-streak" :class="{ cold: !streaks[h.id] }" :title="$t('Serie')"><b>{{ streaks[h.id] ?? 0 }}</b>{{ $t('Serie') }}</span>
            <span class="hd-check" aria-hidden="true"><svg class="ic"><use href="#i-check" /></svg></span>
          </div>
        </div>
      </section>
      <div v-if="notDue.length" class="hd-rest">{{ $t('Nicht geplant') }}: {{ notDue.map((h) => h.name).join(', ') }}</div>
    </div>

    <div v-else class="hm">
      <div class="hm-nav">
        <button class="icon-btn" type="button" :aria-label="$t('Voriger Monat')" :disabled="monthOffset <= -11" @click="monthOffset--"><svg class="ic"><use href="#i-left" /></svg></button>
        <h2 class="hd-title">{{ monthTitle }}</h2>
        <button class="icon-btn" type="button" :aria-label="$t('Nächster Monat')" :disabled="monthOffset >= 0" @click="monthOffset++"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
      <div class="hm-cards">
        <article v-for="h in sorted" :key="h.id" class="card hm-card" :class="{ inactive: !h.active }" :style="{ '--hc': habitColor(h.id) }">
          <header class="hm-head">
            <span class="hk-icon sm" aria-hidden="true">{{ h.name.slice(0, 1).toUpperCase() }}</span>
            <span class="hk-id"><span class="hk-name">{{ h.name }}</span><span class="hab-sched">{{ rhythm(h) }}</span></span>
            <span class="hm-stat"><b>{{ monthStat(h).done }}</b>/{{ monthStat(h).goal }}</span>
          </header>
          <div class="hm-grid" role="group" :aria-label="`${h.name}, ${monthTitle}`">
            <span v-for="w in WEEKDAYS" :key="w" class="hm-wd" aria-hidden="true">{{ WD_LABEL[w]!.slice(0, 2) }}</span>
            <span v-for="i in lead" :key="`l${i}`" aria-hidden="true"></span>
            <button
              v-for="c in monthDays.map((d) => cell(h, d))" :key="c.s" type="button" class="hm-day" :class="[c.state, { today: c.today }]"
              role="checkbox" :aria-checked="c.isDone" :aria-label="c.label" :title="c.label" :disabled="!c.can" @click="toggleCell(h, c)"
            >{{ Number(c.s.slice(8)) }}</button>
          </div>
        </article>
      </div>
    </div>

    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <form class="dialog" :aria-label="form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit')" @submit.prevent="save">
        <div class="dlg-head">
          <h3>{{ form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="field"><label for="h-name">{{ $t('Name') }}</label><input id="h-name" v-model="form.name" class="input" :placeholder="$t('Neue Gewohnheit')" autofocus required /></div>
          <div class="field"><label for="h-desc">{{ $t('Beschreibung') }}</label><input id="h-desc" v-model="form.description" class="input" :placeholder="$t('optional')" /></div>
          <div class="field">
            <label id="h-type">{{ $t('Rhythmus') }}</label>
            <div class="seg" role="group" aria-labelledby="h-type">
              <button v-for="(l, v) in RHYTHMS" :key="v" type="button" :aria-pressed="form.type === v" @click="form.type = v">{{ l }}</button>
            </div>
          </div>
          <div v-if="form.type === 'SPECIFIC_WEEKDAYS'" class="seg" role="group" :aria-label="$t('Wochentage')">
            <button v-for="d in WEEKDAYS" :key="d" type="button" :aria-pressed="form.weekdays.includes(d)" @click="toggleDay(d)">{{ WD_LABEL[d] }}</button>
          </div>
          <div v-if="form.type === 'TIMES_PER_WEEK'" class="field">
            <label for="h-times">{{ $t('Pro Woche') }}</label>
            <input id="h-times" v-model.number="form.times" class="input" type="number" min="1" max="7" />
          </div>
          <div class="dlg-row">
            <div class="field"><label for="h-time">{{ $t('Uhrzeit') }}</label><input id="h-time" v-model="form.time" class="input" type="time" /></div>
            <div class="field"><label for="h-dur">{{ $t('Dauer (Min.)') }}</label><input id="h-dur" v-model.number="form.duration" class="input" type="number" min="5" max="1440" step="5" :disabled="!form.time" /></div>
          </div>
          <div class="v-sub">{{ $t('Mit Uhrzeit erscheint die Gewohnheit als Block im Kalender.') }}</div>
          <div class="dlg-row">
            <div class="field"><label for="h-target">{{ $t('Ziel') }}</label><input id="h-target" v-model.number="form.target" class="input" type="number" min="1" :placeholder="$t('optional')" /></div>
            <div class="field"><label for="h-unit">{{ $t('Einheit') }}</label><input id="h-unit" v-model="form.unit" class="input" :placeholder="$t('z. B. Seiten')" /></div>
          </div>
          <div class="dlg-row">
            <div class="field"><label for="h-start">{{ $t('Beginnt am') }}</label><input id="h-start" v-model="form.start" class="input" type="date" required /></div>
            <div class="field"><label for="h-end">{{ $t('Endet am') }}</label><input id="h-end" v-model="form.end" class="input" type="date" :min="form.start" /></div>
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" @click="dialog = false">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit" :disabled="form.type === 'SPECIFIC_WEEKDAYS' && !form.weekdays.length">{{ form.id ? $t('Speichern') : $t('Anlegen') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.dlg-row { display: flex; gap: 12px; }
.dlg-row .field { flex: 1; }

/* Kennzahlen */
.hk-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 24px; }
.hk-stat { display: grid; align-content: start; gap: 8px; padding: 16px 18px; border: 1px solid var(--br-default); border-radius: var(--r-m); background: var(--bg-1); }
.hk-stat b { font: 600 28px/1 var(--font-ui); letter-spacing: -0.02em; color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.hk-stat small { font-size: 15px; font-weight: 500; color: var(--tx-muted); }
.hk-note { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hk-stat .pbar-fill { background: var(--tx-primary); }

/* Graph-Karten */
.hk-list { display: grid; gap: 14px; }
.hk-card { padding: 18px 20px 16px; }
.hk-head { display: flex; align-items: center; gap: 14px; margin-bottom: 14px; }
.hk-icon { flex: none; display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; font: 600 16px/1 var(--font-ui);
  color: var(--hc); background: color-mix(in srgb, var(--hc) 16%, transparent); }
.hk-icon.sm { width: 30px; height: 30px; border-radius: 8px; font-size: 13px; }
.hk-id { display: grid; gap: 3px; min-width: 0; flex: 1; }
.hk-name { font: 600 15px/1.3 var(--font-ui); color: var(--tx-primary); }
.hk-chips { display: flex; align-items: center; gap: 22px; margin-right: 8px; }
.hk-kpi { display: grid; justify-items: end; gap: 3px; font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hk-kpi b { font: 600 17px/1 var(--font-ui); color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.hk-check { flex: none; display: grid; place-items: center; width: 40px; height: 40px; border-radius: 10px; cursor: pointer;
  border: 1.5px solid color-mix(in srgb, var(--hc) 55%, transparent); background: transparent; color: transparent; transition: background var(--dur) ease-out; }
.hk-check:hover:not(:disabled) { background: color-mix(in srgb, var(--hc) 14%, transparent); color: color-mix(in srgb, var(--hc) 60%, transparent); }
.hk-check[aria-checked="true"] { background: var(--hc); border-color: var(--hc); color: var(--bg-0); }
.hk-check:disabled { cursor: default; opacity: .35; border-style: dashed; }
.hk-check .ic { width: 20px; height: 20px; }
.hk-graph { display: grid; gap: 4px; }
.hk-months, .hk-grid { display: grid; grid-template-columns: repeat(53, minmax(0, 1fr)); column-gap: 3px; }
.hk-months { font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hk-months span { white-space: nowrap; }
.hk-grid { grid-template-rows: repeat(7, auto); grid-auto-flow: column; row-gap: 3px; }
.hk-grid i { aspect-ratio: 1; border-radius: 3px; background: color-mix(in srgb, var(--hc) 9%, var(--bg-2)); }
.hk-grid i.done { background: var(--hc); }
.hk-grid i.off { background: var(--bg-1); }
.hk-grid i.pre { background: var(--bg-2); opacity: .5; }
.hk-grid i.future { background: transparent; }
.hk-grid i.today { outline: 1.5px solid var(--tx-primary); outline-offset: 0; }
.hk-grid i[role="checkbox"] { cursor: pointer; }
.hk-grid i[role="checkbox"]:hover { outline: 1.5px solid var(--tx-secondary); }

/* Tagesansicht */
.hd-strip { display: flex; align-items: center; gap: 6px; margin-bottom: 22px; }
.hd-day { flex: 1; display: grid; justify-items: center; gap: 6px; padding: 10px 0 8px; border: 1px solid transparent; border-radius: var(--r-m);
  background: transparent; color: var(--tx-muted); cursor: pointer; font: 500 11px/1 var(--font-ui); }
.hd-day:hover { background: var(--bg-1); }
.hd-day.sel { background: var(--bg-1); border-color: var(--br-default); color: var(--tx-primary); }
.hd-day.today small { color: var(--tx-primary); }
.hd-day.future { opacity: .5; }
.hd-ring { --p: 0; display: grid; place-items: center; width: 44px; height: 44px; border-radius: 50%;
  background: radial-gradient(closest-side, var(--bg-0) 82%, transparent 84%), conic-gradient(var(--tx-primary) calc(var(--p) * 1turn), var(--bg-3) 0); }
.hd-day.sel .hd-ring { background: radial-gradient(closest-side, var(--bg-1) 82%, transparent 84%), conic-gradient(var(--tx-primary) calc(var(--p) * 1turn), var(--bg-3) 0); }
.hd-ring.full { background: var(--a-green, var(--tx-primary)); }
.hd-ring.full b { color: var(--bg-0); }
.hd-day.sel .hd-ring.full b { color: var(--tx-primary); } /* gewählt: Ring mit dunkler Mitte */
.hd-ring b { font: 600 15px/1 var(--font-ui); color: var(--tx-primary); }
.hd-cnt { font: 400 11px/1 var(--font-mono); color: var(--tx-muted); }
.hd-title { margin: 0 0 14px; font: 600 18px/1.2 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }
.hd-group { display: grid; gap: 10px; margin-bottom: 22px; }
.hd-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 10px; }
.hd-card { display: flex; align-items: center; gap: 14px; padding: 14px 16px; border: 1px solid var(--br-default); border-radius: var(--r-m);
  background: var(--bg-1); cursor: pointer; transition: background var(--dur) ease-out, border-color var(--dur) ease-out; }
.hd-card:hover { border-color: color-mix(in srgb, var(--hc) 50%, var(--br-default)); }
.hd-card:focus-visible { outline: 2px solid var(--a-focus); outline-offset: 1px; }
.hd-card.done { background: color-mix(in srgb, var(--hc) 13%, var(--bg-1)); border-color: color-mix(in srgb, var(--hc) 45%, transparent); }
/* Erledigt: gedimmt statt durchgestrichen, der Haken sagt es schon. */
.hd-card.done .hk-name, .hd-card.done .hab-sub { opacity: .6; }
.hd-card.locked { cursor: default; opacity: .6; }
.hd-streak { display: grid; justify-items: end; gap: 3px; font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hd-streak b { font: 600 16px/1 var(--font-ui); color: var(--tx-primary); }
.hd-streak.cold b { color: var(--tx-disabled); }
.hd-check { flex: none; display: grid; place-items: center; width: 38px; height: 38px; border-radius: 50%;
  border: 2px solid color-mix(in srgb, var(--hc) 60%, transparent); color: transparent; }
.hd-card:hover:not(.locked) .hd-check { color: color-mix(in srgb, var(--hc) 60%, transparent); }
.hd-card.done .hd-check { background: var(--hc); border-color: var(--hc); color: var(--bg-0); }
.hd-check .ic { width: 18px; height: 18px; }
.hd-rest { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }

/* Monatsansicht */
.hm-nav { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
.hm-nav .hd-title { margin: 0; min-width: 170px; text-align: center; }
.hm-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 14px; }
.hm-card { padding: 16px; }
.hm-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.hm-stat { font: 400 13px/1 var(--font-mono); color: var(--tx-muted); }
.hm-stat b { font-size: 16px; color: var(--tx-primary); }
.hm-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 4px; }
.hm-wd { text-align: center; font: 500 10px/1.6 var(--font-ui); color: var(--tx-muted); }
.hm-day { aspect-ratio: 1; display: grid; place-items: center; border: 0; border-radius: 7px; cursor: pointer;
  font: 500 11px/1 var(--font-ui); color: var(--tx-secondary); background: color-mix(in srgb, var(--hc) 10%, var(--bg-2)); }
.hm-day:hover:not(:disabled) { outline: 1.5px solid var(--tx-secondary); }
.hm-day.done { background: var(--hc); color: var(--bg-0); font-weight: 600; text-decoration: none; } /* styles.css streicht .done global durch */
.hm-day.off { background: transparent; color: var(--tx-disabled); }
.hm-day.pre, .hm-day.future { background: transparent; color: var(--tx-disabled); }
.hm-day.today { outline: 1.5px solid var(--tx-primary); }
.hm-day:disabled { cursor: default; }

.inactive { opacity: .55; }
/* Beim Überfahren ersetzen die Aktionen die Rhythmus-Zeile. */
.hab-sub { display: grid; align-items: center; min-height: 20px; }
.hab-sub > * { grid-area: 1 / 1; }
.acts { display: inline-flex; gap: 2px; margin-left: -8px; opacity: 0; pointer-events: none; transition: opacity var(--dur) ease-out; }
.acts .btn { min-height: 0; height: 20px; padding: 0 8px; font-size: 12px; }
.hk-card:hover .acts, .hk-card:focus-within .acts { opacity: 1; pointer-events: auto; }
.hk-card:hover .hab-sched, .hk-card:focus-within .hab-sched { opacity: 0; }
.hab-foot { border-top: 0; padding-top: 4px; }

@media (max-width: 900px) {
  .hk-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .hk-chips { display: none; }
  .hd-ring { width: 36px; height: 36px; }
}
/* Handy: Kennzahlen als eine schmale Zeile, damit die Liste zum Abhaken oben bleibt. */
@media (max-width: 640px) {
  .hk-stats { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; margin-bottom: 16px; }
  .hk-stat { gap: 4px; padding: 8px 10px; }
  .hk-stat .lbl { font-size: 10px; line-height: 1.2; }
  .hk-stat b { font-size: 18px; }
  .hk-stat small { font-size: 12px; }
  .hk-stat .pbar, .hk-note { display: none; }
}
</style>
