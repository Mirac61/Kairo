<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  api, completeHabit, createHabit, createHabitPause, deleteHabit, deleteHabitPause, getToday, listHabitPauses, listHabits, listProjects,
  restoreHabit, uncompleteHabit, updateHabit, type FrequencyConfig, type Habit, type HabitPause, type Project, type TodayHabit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { vDialog } from '@/lib/dialog'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { habitColor, projectColor } from '@/lib/projectColor'
import { HABIT_ICONS, iconKey, iconOf } from '@/lib/habitIcons'
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
const vals = ref<Record<string, Record<string, number | null>>>({}) // erfasste Menge je Habit und Tag
const pauses = ref<HabitPause[]>([])
const streaks = ref<Record<string, number>>({})
const dialog = ref(false)
const blank = () => ({
  id: '', name: '', description: '', type: 'DAILY' as Rhythm, weekdays: ['MO'] as string[], times: 3,
  time: '', duration: 30, target: null as number | null, unit: '', icon: '', remind: false, start: ymd(new Date()), end: '', project: '',
})
const projects = ref<Project[]>([])
const projectMap = computed(() => new Map(projects.value.map((p) => [p.id, p])))
const projectName = (id?: string | null) => (id ? projectMap.value.get(id)?.name : undefined)
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
  const [hs, td, rv, ps, pz] = await Promise.all([
    listHabits(), getToday(t), api<{ habits: { habit_id: string; streak: number }[] }>(`/review?from=${t}&to=${t}`), listProjects(), listHabitPauses(),
  ])
  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')
  const comps = await Promise.all(hs.map((h) =>
    api<{ date: string; value: number | null }[]>(`/habits/${h.id}/completions?from=${from}&to=${t}`)))
  habits.value = hs
  todayHabits.value = td.habits
  streaks.value = Object.fromEntries(rv.habits.map((h) => [h.habit_id, h.streak]))
  pauses.value = pz
  vals.value = Object.fromEntries(hs.map((h, i) => [h.id, Object.fromEntries(comps[i]!.map((c) => [c.date, c.value]))]))
  // Mit Zielmenge ist ein Tag erst ab der Menge erledigt; ein Haken ohne Menge zählt voll (wie im Backend).
  done.value = Object.fromEntries(hs.map((h, i) => [h.id,
    new Set(comps[i]!.filter((c) => h.target_value == null || c.value == null || c.value >= h.target_value).map((c) => c.date))]))
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
    time: h.preferred_time ?? '', duration: h.duration_minutes, target: h.target_value, unit: h.unit, icon: h.icon ?? '', remind: h.remind,
    start: h.start_date, end: h.end_date ?? '', project: h.project_id ?? '',
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
    unit: f.unit, icon: f.icon, remind: f.remind && !!f.time, start_date: f.start, end_date: f.end, project_id: f.project || (f.id ? '' : null),
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

// Pausen: eigene und globale (Urlaub); an diesen Tagen ist nichts fällig.
const pausesOf = (h: Habit) => pauses.value.filter((p) => !p.habit_id || p.habit_id === h.id)
const pausedOn = (h: Habit, s: string) => pausesOf(h).some((p) => s >= p.start_date && s <= p.end_date)

const dayLabel = new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: '2-digit', month: '2-digit' })
// Zustand eines Tages: future/pre (außerhalb, nicht klickbar), done, open (geplant, offen), off (nicht geplant).
// Bei X-mal pro Woche zählt jeder Tag als geplant; offen ist dort kein Versäumnis.
type State = 'done' | 'open' | 'off' | 'pre' | 'future' | 'pause'
function cell(h: Habit, d: Date) {
  const s = ymd(d)
  const isDone = !!done.value[h.id]?.has(s)
  const state: State = s > today() ? 'future' : isDone ? 'done' : s < h.start_date || (h.end_date && s > h.end_date) ? 'pre'
    : !planned(h, d) ? 'off' : pausedOn(h, s) ? 'pause' : 'open'
  const can = h.active && state !== 'future' && state !== 'pre'
  return { s, state, isDone, can, today: s === today(), label: `${dayLabel.format(d)}: ${isDone ? t('erledigt') : state === 'pause' ? t('ausgesetzt') : t('offen')}` }
}
type Cell = ReturnType<typeof cell>
const graph = (h: Habit) => Array.from({ length: WEEKS * 7 }, (_, i) => cell(h, addDays(gridStart(), i)))
// Monatsname über der ersten Woche, die in einem neuen Monat beginnt.
const monthShort = new Intl.DateTimeFormat(locale.value, { month: 'short' })
const months = computed(() => Array.from({ length: WEEKS }, (_, w) => addDays(gridStart(), 7 * w))
  .map((d, w, all) => ({ col: w + 1, label: w === 0 || d.getMonth() !== all[w - 1]!.getMonth() ? monthShort.format(d) : '' }))
  .filter((m) => m.label && m.col < WEEKS - 1)
  .filter((m, i, all) => !all[i + 1] || all[i + 1]!.col - m.col >= 3)) // angeschnittener Anfangsmonat weicht, wenn der nächste direkt folgt („OktNov“)

// Klick auf ein Feld holt den Tag nach bzw. nimmt ihn zurück.
const toggleCell = (h: Habit, c: Cell) => c.can && run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))

// Menge eines Tages: erfasster Wert; ein Haken ohne Wert zählt als volle Zielmenge.
const amount = (h: Habit, s: string) => vals.value[h.id]?.[s] ?? (done.value[h.id]?.has(s) ? h.target_value ?? 1 : 0)
// 0 nimmt den Tag zurück; sonst wird die Menge gespeichert (erledigt ab Zielmenge).
function setAmount(h: Habit, c: Cell, v: number) {
  if (!c.can) return
  const had = c.s in (vals.value[h.id] ?? {})
  void run(() => (v > 0 ? completeHabit(h.id, c.s, v) : had ? uncompleteHabit(h.id, c.s) : Promise.resolve()))
}

// Aussetzen (ein Tag) und Pausieren (Zeitraum); global = Urlaub für alle Gewohnheiten.
const dayPause = (h: Habit, s: string) => pauses.value.find((p) => p.habit_id === h.id && p.start_date === s && p.end_date === s)
const toggleSkip = (h: Habit, s: string) => {
  const p = dayPause(h, s)
  void run(() => (p ? deleteHabitPause(p.id) : createHabitPause({ habit_id: h.id, start_date: s })))
}
// Offen ist das Formular mit habit_id (eine Gewohnheit) oder null (Urlaub); undefined = zu.
const pauseFor = ref<string | null | undefined>(undefined)
const more = ref<string | null>(null) // Habit, dessen „…“-Menü offen ist
const pauseFrom = ref(today())
const pauseUntil = ref(ymd(addDays(new Date(), 7)))
function openPause(habitID: string | null) {
  pauseFor.value = habitID
  pauseFrom.value = today()
  pauseUntil.value = ymd(addDays(new Date(), 7))
}
function savePause() {
  const habit_id = pauseFor.value ?? null
  if (pauseUntil.value < pauseFrom.value) return
  void run(async () => {
    await createHabitPause({ habit_id, start_date: pauseFrom.value, end_date: pauseUntil.value })
    pauseFor.value = undefined
  })
}
const dateShort = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' })
const fmtDay = (x: string) => dateShort.format(new Date(`${x}T12:00:00`))
const spanLabel = (p: HabitPause) => (p.start_date === p.end_date ? fmtDay(p.start_date) : `${fmtDay(p.start_date)} – ${fmtDay(p.end_date)}`)
// Laufende und künftige Pausen (vergangene sind nur noch Geschichte im Graphen).
const upcoming = (ps: HabitPause[]) => ps.filter((p) => p.end_date >= today())
const vacations = computed(() => upcoming(pauses.value.filter((p) => !p.habit_id)))

const due = (h: Habit) => todayHabits.value.find((x) => x.id === h.id)
const toggleToday = (h: Habit) => { const x = due(h); if (x) void run(() => (x.done ? uncompleteHabit(h.id, today()) : completeHabit(h.id, today()))) }
const yearCount = (h: Habit) => done.value[h.id]?.size ?? 0
// Quote der letzten 30 Tage: erledigte durch geplante Tage (bei X-mal pro Woche das anteilige Ziel), höchstens 100 %.
function rate(h: Habit) {
  const days = Array.from({ length: 30 }, (_, i) => addDays(new Date(), -i)).filter((d) => ymd(d) >= h.start_date && !pausedOn(h, ymd(d)))
  if (!days.length) return null
  const hits = days.filter((d) => done.value[h.id]?.has(ymd(d))).length
  const goal = h.frequency_type === 'TIMES_PER_WEEK' ? ((h.frequency_config.times ?? 1) * days.length) / 7 : days.filter((d) => planned(h, d)).length
  return goal ? Math.min(100, Math.round((hits / goal) * 100)) : null
}

// Fortschritt einer Woche: erledigt gegen das Ziel (geplante Tage bzw. X bei X-mal pro Woche).
const weekOf = (d: Date) => Array.from({ length: 7 }, (_, i) => addDays(weekStart(d), i))
function weekProgress(h: Habit, days = weekOf(new Date())) {
  const target = h.frequency_type === 'TIMES_PER_WEEK' ? h.frequency_config.times ?? 1
    : days.filter((d) => planned(h, d) && ymd(d) >= h.start_date && (!h.end_date || ymd(d) <= h.end_date) && !pausedOn(h, ymd(d))).length
  return { done: days.filter((d) => done.value[h.id]?.has(ymd(d))).length, target }
}

// Tagesansicht: fällig ist ein Habit an geplanten Tagen im Zeitraum; bei X-mal pro Woche nur,
// solange das Wochenziel vor diesem Tag noch nicht erreicht ist (ein erledigter Tag bleibt sichtbar).
function dueOn(h: Habit, d: Date) {
  const c = cell(h, d)
  if (!h.active || c.state === 'pre' || c.state === 'off' || (c.state === 'pause' && !c.isDone)) return false
  if (c.isDone || h.frequency_type !== 'TIMES_PER_WEEK') return true
  return weekProgress(h, weekOf(d)).done < (h.frequency_config.times ?? 1)
}
// Wochenraster: eine Zeile je Gewohnheit, eine Spalte je Tag der gewählten Woche.
const selDay = ref(new Date())
const week = computed(() => weekOf(selDay.value))
const thisWeek = computed(() => ymd(weekStart(selDay.value)) >= ymd(weekStart()))
const weekLabel = computed(() => `${fmtDay(ymd(week.value[0]!))} – ${fmtDay(ymd(week.value[6]!))}`)
const wdShort = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
const SLOTS = [[t('Morgens'), '00:00'], [t('Mittags'), '12:00'], [t('Abends'), '17:00']] as const
// Nach Tageszeit gruppiert; ohne Uhrzeit unter „Jederzeit“, inaktive zuletzt.
const gridGroups = computed(() => {
  const hs = [...sorted.value].sort((a, b) => (a.preferred_time ?? '99').localeCompare(b.preferred_time ?? '99'))
  const slot = (h: Habit) => !h.active ? t('Inaktiv') : h.preferred_time ? [...SLOTS].reverse().find(([, from]) => h.preferred_time! >= from)![0] : t('Jederzeit')
  return [...SLOTS.map(([l]) => l), t('Jederzeit'), t('Inaktiv')].map((label) => ({ label, habits: hs.filter((h) => slot(h) === label) })).filter((g) => g.habits.length)
})
// Feld im Raster: künftige geplante Tage bekommen einen Haarpunkt, Mengen einen Teilring.
function gridCell(h: Habit, d: Date) {
  const c = cell(h, d)
  const p = h.target_value && !c.isDone ? Math.min(1, amount(h, c.s) / h.target_value) : 0
  const isSoft = h.frequency_type === 'TIMES_PER_WEEK'
  let stateLabel = c.isDone ? t('erledigt') : c.state === 'pause' ? t('ausgesetzt') : t('offen')
  if (!c.isDone && p > 0 && h.target_value) {
    stateLabel = `${amount(h, c.s)}/${h.target_value} ${h.unit || ''} ${t('erledigt')}`.trim()
  } else if (!c.isDone && isSoft && c.state === 'open') {
    stateLabel = `${t('offen')} (${t('flexibel')})`
  }
  return { ...c, plan: c.state === 'future' && planned(h, d) && (!h.end_date || c.s <= h.end_date), p, label: `${dayLabel.format(d)}: ${stateLabel}` }
}
const todayCell = (h: Habit) => cell(h, new Date())
const streakAtRisk = (h: Habit) => (streaks.value[h.id] ?? 0) > 0 && !!due(h) && !due(h)?.done
// Anteil erledigt an einem Tag über alle aktiven Gewohnheiten; null für künftige oder leere Tage.
function dayRate(d: Date) {
  if (ymd(d) > today()) return null
  const cs = activeHabits.value.map((h) => cell(h, d)).filter((c) => c.state === 'done' || c.state === 'open')
  return cs.length ? cs.filter((c) => c.isDone).length / cs.length : null
}
const weekRates = computed(() => week.value.map(dayRate))
// Verlauf der letzten 12 Wochen, älteste zuerst.
const trend = computed(() => Array.from({ length: 12 }, (_, i) => {
  const days = weekOf(addDays(new Date(), (i - 11) * 7))
  const rs = days.map(dayRate).filter((r): r is number => r !== null)
  return { s: ymd(days[0]!), cur: i === 11, pct: rs.length ? Math.round(rs.reduce((a, b) => a + b, 0) / rs.length * 100) : null }
}))
const todayDue = computed(() => activeHabits.value.filter((h) => dueOn(h, new Date())))
const todayDone = computed(() => todayDue.value.filter((h) => todayCell(h).isDone).length)

const meta = (h: Habit) => [h.preferred_time,
  h.frequency_type === 'TIMES_PER_WEEK' ? t('{done}/{target} diese Woche', weekProgress(h, week.value)) : rhythm(h)].filter(Boolean).join(' · ')
// Längste Serie im geladenen Jahr: offene, vergangene Plantage brechen sie, nicht geplante Tage zählen nicht.
// shortcut: bei X-mal pro Woche zählt jeder offene Tag als Bruch; genauer wäre die Wochenlogik wie im Backend.
function longest(h: Habit) {
  let best = 0, cur = 0
  for (const c of graph(h)) if (c.state === 'done') best = Math.max(best, ++cur); else if (c.state === 'open' && !c.today) cur = 0
  return Math.max(best, streaks.value[h.id] ?? 0)
}
// Anteil erledigter Plantage je Wochentag (Spalten des Graphen beginnen montags).
const wdRate = (h: Habit) => WEEKDAYS.map((w, i) => {
  const cs = graph(h).filter((c, j) => j % 7 === i && (c.state === 'done' || (c.state === 'open' && !c.today)))
  return { w, pct: cs.length ? Math.round((cs.filter((c) => c.isDone).length / cs.length) * 100) : null }
})
const focusId = ref<string | null>(null)
const focus = computed(() => habits.value.find((h) => h.id === focusId.value) ?? null)
const pick = (h: Habit) => (focusId.value = focusId.value === h.id ? null : h.id)
// Quote je Wochentag nur zeigen, wenn sich die Tage deutlich unterscheiden (≥ 15 Punkte); sonst löst sie keine Entscheidung aus.
const focusWd = computed(() => {
  const r = focus.value ? wdRate(focus.value) : []
  const p = r.flatMap((x) => (x.pct === null ? [] : [x.pct]))
  return p.length > 1 && Math.max(...p) - Math.min(...p) >= 15 ? r : []
})

// Wochenansicht: j/k wählt, x hakt heute ab, ←/→ wechselt die Woche, Esc schließt das Detail, n legt an (Übersicht in lib/shortcuts.ts).
useShortcuts((e) => {
  if (dialog.value) return
  if (e.key === 'n') openNew()
  else if (view.value !== 'day') return
  else if (e.key === 'j' || e.key === 'k') {
    const list = gridGroups.value.flatMap((g) => g.habits)
    const i = list.findIndex((h) => h.id === focus.value?.id)
    const next = list[Math.min(list.length - 1, Math.max(0, i + (e.key === 'j' ? 1 : -1)))]
    if (next) focusId.value = next.id
  } else if (e.key === 'x' && focus.value) void toggleCell(focus.value, todayCell(focus.value))
  else if (e.key === 'Escape' && focus.value) focusId.value = null
  else if (e.key === 'ArrowLeft') selDay.value = addDays(selDay.value, -7)
  else if (e.key === 'ArrowRight' && !thisWeek.value) selDay.value = addDays(selDay.value, 7)
  else return
  e.preventDefault()
})

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
const search = ref('')
const sorted = computed(() => {
  const q = search.value.trim().toLowerCase()
  return habits.value.filter((h) => !q || h.name.toLowerCase().includes(q)).sort((a, b) => Number(b.active) - Number(a.active))
})

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner" :class="{ 'hab-split': view === 'day' && focus }">
   <div class="hab-main">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="filterbar">
      <div class="seg" role="group" :aria-label="$t('Ansicht')">
        <button type="button" :aria-pressed="view === 'day'" @click="view = 'day'">{{ $t('Woche') }}</button>
        <button type="button" :aria-pressed="view === 'month'" @click="view = 'month'">{{ $t('Monat') }}</button>
        <button type="button" :aria-pressed="view === 'year'" @click="view = 'year'">{{ $t('Jahr') }}</button>
      </div>
      <span class="spacer"></span>
      <button class="btn btn-ghost hab-vac" type="button" :aria-expanded="pauseFor === null" @click="pauseFor === null ? (pauseFor = undefined) : openPause(null)">{{ $t('Urlaub') }}</button>
      <SearchField v-model="search" :label="$t('Gewohnheiten durchsuchen')" />
      <button class="icon-btn" type="button" :aria-label="$t('Neue Gewohnheit')" :title="`${$t('Neue Gewohnheit')} (N)`" @click="openNew"><svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg></button>
    </div>
    <form v-if="pauseFor === null" class="hd-pause card" @submit.prevent="savePause">
      <span class="hd-pause-t">{{ $t('Alle Gewohnheiten pausieren') }}</span>
      <label>{{ $t('von') }} <input v-model="pauseFrom" class="input" type="date" required /></label>
      <label>{{ $t('bis') }} <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required /></label>
      <button class="btn btn-primary" type="submit">{{ $t('Pausieren') }}</button>
      <button class="btn btn-ghost" type="button" @click="pauseFor = undefined">{{ $t('Abbrechen') }}</button>
      <ul v-if="vacations.length" class="hd-chips">
        <li v-for="p in vacations" :key="p.id">
          {{ $t('Urlaub') }} · {{ spanLabel(p) }}
          <button type="button" class="icon-btn" :aria-label="$t('Pause entfernen')" @click="run(() => deleteHabitPause(p.id))"><svg class="ic"><use href="#i-x" /></svg></button>
        </li>
      </ul>
    </form>
    <div v-if="!habits.length" class="empty-cta"><p class="v-sub">{{ $t('Noch keine Gewohnheiten. Klein anfangen hilft, etwa zehn Minuten Lesen am Morgen.') }}</p><button class="btn" type="button" @click="openNew">{{ $t('Erste Gewohnheit anlegen') }}</button></div>
    <div v-else-if="!sorted.length" class="v-sub">{{ $t('Keine Treffer für „{q}“.', { q: search }) }}</div>

    <div v-else-if="view === 'year'" class="hk-list">
      <article v-for="h in sorted" :key="h.id" class="hk-card" :class="{ inactive: !h.active }" :style="{ '--hc': habitColor(h.id) }">
        <header class="hk-head">
          <span class="hk-icon" aria-hidden="true"><svg class="ic"><use :href="`#${iconOf(h)}`" /></svg></span>
          <div class="hk-id">
            <span class="hk-name">{{ h.name }}</span>
            <span class="hab-sub">
              <span class="hab-sched">{{ h.description || describe(h) }}<template v-if="!h.active"> {{ $t('· inaktiv') }}</template></span>
              <span class="acts">
                <button class="btn btn-ghost" type="button" @click="openEdit(h)">{{ $t('Bearbeiten') }}</button>
                <span class="menu-wrap" @keydown.esc="more = null">
                  <button class="btn btn-ghost hd-more" type="button" :aria-label="$t('Weitere Aktionen')" :aria-expanded="more === h.id" @click="more = more === h.id ? null : h.id"><svg class="ic"><use href="#i-more" /></svg></button>
                  <template v-if="more === h.id">
                    <span class="menu-scrim" @click="more = null"></span>
                    <span class="menu">
                      <button class="menu-i" type="button" @click="more = null; run(() => updateHabit(h.id, { active: !h.active }))">{{ h.active ? $t('Deaktivieren') : $t('Aktivieren') }}</button>
                      <button class="menu-i danger" type="button" @click="more = null; trash(h)">{{ $t('Löschen') }}</button>
                    </span>
                  </template>
                </span>
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
              <span class="hab-legend"><i class="pause"></i>{{ $t('ausgesetzt') }}</span>
        <span class="hab-hint">{{ $t('Klick auf ein Feld trägt nach oder nimmt zurück') }}</span>
      </div>
    </div>

    <div v-else-if="view === 'day'" class="hw" role="grid" :aria-label="$t('Woche {range}', { range: weekLabel })">
      <div class="hw-head" role="row">
        <span class="hw-nav">
          <button class="icon-btn" type="button" :aria-label="$t('Vorige Woche')" @click="selDay = addDays(selDay, -7)"><svg class="ic"><use href="#i-left" /></svg></button>
          <span class="hw-range">{{ weekLabel }}</span>
          <button class="icon-btn" type="button" :aria-label="$t('Nächste Woche')" :disabled="thisWeek" @click="selDay = addDays(selDay, 7)"><svg class="ic"><use href="#i-right" /></svg></button>
          <button v-if="!thisWeek" class="btn btn-ghost hw-back" type="button" @click="selDay = new Date()">{{ $t('Diese Woche') }}</button>
          <span v-else-if="todayDue.length" class="hab-count">{{ $t('{done}/{n} heute', { done: todayDone, n: todayDue.length }) }}</span>
        </span>
        <span v-for="d in week" :key="ymd(d)" class="hw-d" :class="{ today: ymd(d) === today(), future: ymd(d) > today() }" role="columnheader">
          <small>{{ wdShort.format(d).slice(0, 2) }}</small><b>{{ d.getDate() }}</b>
        </span>
        <span class="hw-streak-th" :title="$t('Serie')" :aria-label="$t('Serie')" role="columnheader"><svg class="ic" aria-hidden="true"><use href="#i-flame" /></svg></span>
      </div>
      <div v-for="g in gridGroups" :key="g.label" class="hw-group" role="rowgroup">
        <span class="hw-gl">{{ g.label }}</span>
        <div
          v-for="h in g.habits" :key="h.id" class="hd-row" role="row" :class="{ sel: focus?.id === h.id, inactive: !h.active }"
          :style="{ '--hc': habitColor(h.id) }"
        >
          <button type="button" class="hd-pick" :aria-pressed="focus?.id === h.id" @click="pick(h)">
            <span class="hd-icon" aria-hidden="true"><svg class="ic"><use :href="`#${iconOf(h)}`" /></svg></span>
            <span class="hw-name"><b>{{ h.name }}</b><small>{{ meta(h) }}</small></span>
          </button>
          <button
            v-for="c in week.map((d) => gridCell(h, d))" :key="c.s" type="button" class="hw-c"
            :class="[c.state, { today: c.today, plan: c.plan, part: c.p > 0, soft: h.frequency_type === 'TIMES_PER_WEEK' }]" :style="c.p ? { '--p': c.p } : undefined"
            role="checkbox" :aria-checked="c.isDone" :aria-label="`${h.name}, ${c.label}`" :title="c.can ? c.label : undefined" :disabled="!c.can" @click="toggleCell(h, c)"
          ><i><svg v-if="c.isDone" class="ic" aria-hidden="true"><use href="#i-check" /></svg></i></button>
          <span class="hw-streak" :class="{ risk: streakAtRisk(h) }" :title="$t('Serie')">{{ streaks[h.id] || '' }}</span>
        </div>
      </div>
      <div v-if="activeHabits.length" class="hd-row hw-sum" role="row">
        <span class="hw-gl">{{ $t('Tag gesamt') }}</span>
        <span v-for="(r, i) in weekRates" :key="i" class="hw-ring" :class="{ full: r === 1 }" :style="r === null ? undefined : { '--p': r }" :title="r === null ? undefined : `${Math.round(r * 100)} %`">
          <template v-if="r !== null">{{ Math.round(r * 100) }}</template>
        </span>
        <span></span>
      </div>
      <p v-if="activeHabits.length" class="hw-legend">
        <span><i class="done"></i>{{ $t('erledigt') }}</span><span><i class="open"></i>{{ $t('offen') }}</span>
        <span><i class="part"></i>{{ $t('teilweise') }}</span><span><i class="soft"></i>{{ $t('flexibel') }}</span>
        <span><i class="pause"></i>{{ $t('ausgesetzt') }}</span><span><i class="plan"></i>{{ $t('geplant') }}</span>
        <em>{{ $t('Kreis anklicken hakt den Tag ab, auch rückwirkend') }}</em>
      </p>
      <section v-if="activeHabits.length" class="hw-trend" :aria-label="$t('Letzte 12 Wochen')">
        <div class="hw-trend-h"><span>{{ $t('Letzte 12 Wochen') }}</span><b v-if="trend[11]!.pct !== null">{{ $t('{p} % diese Woche', { p: trend[11]!.pct! }) }}</b></div>
        <div class="hw-bars">
          <span v-for="w in trend" :key="w.s" :class="{ cur: w.cur }" :title="w.pct === null ? undefined : `${fmtDay(w.s)}: ${w.pct} %`">
            <i v-if="w.pct !== null" :style="{ '--h': w.pct / 100 }"></i><i v-else class="none"></i><small>{{ fmtDay(w.s) }}</small>
          </span>
        </div>
      </section>
      <button class="hw-add" type="button" @click="openNew"><svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>{{ $t('Gewohnheit hinzufügen') }}</button>
    </div>

    <div v-else class="hm">
      <div class="hm-nav">
        <button class="icon-btn" type="button" :aria-label="$t('Voriger Monat')" :disabled="monthOffset <= -11" @click="monthOffset--"><svg class="ic"><use href="#i-left" /></svg></button>
        <h2 class="hd-title">{{ monthTitle }}</h2>
        <button class="icon-btn" type="button" :aria-label="$t('Nächster Monat')" :disabled="monthOffset >= 0" @click="monthOffset++"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
      <div class="hm-cards">
        <article v-for="h in sorted" :key="h.id" class="hm-card" :class="{ inactive: !h.active }" :style="{ '--hc': habitColor(h.id) }">
          <header class="hm-head">
            <span class="hk-icon sm" aria-hidden="true"><svg class="ic"><use :href="`#${iconOf(h)}`" /></svg></span>
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

   </div>

    <aside v-if="view === 'day' && focus" class="hab-aside" :aria-label="$t('Gewohnheit')" :style="{ '--hc': habitColor(focus.id) }">
      <template v-for="f in [focus]" :key="f.id">
        <div class="aside-head">
          <span class="lbl">{{ $t('Gewohnheit') }}</span>
          <button class="icon-btn" type="button" :aria-label="$t('Details schließen')" @click="focusId = null"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="ha-body">
          <header class="ha-head">
            <span class="hd-icon" aria-hidden="true"><svg class="ic"><use :href="`#${iconOf(f)}`" /></svg></span>
            <span class="hd-name">
              <b>{{ f.name }}</b>
              <small>{{ f.description || describe(f) }}<template v-if="!f.active"> {{ $t('· inaktiv') }}</template></small>
            </span>
            <span v-if="f.active && f.target_value" class="hd-qty" role="group" :aria-label="$t('Menge: {name}', { name: f.name })">
              <button type="button" class="icon-btn" :aria-label="$t('Weniger')" :disabled="!todayCell(f).can || !amount(f, today())" @click="setAmount(f, todayCell(f), amount(f, today()) - 1)"><svg class="ic"><use href="#i-minus" /></svg></button>
              <span class="hd-qring" :class="{ full: todayCell(f).isDone }" :style="{ '--p': Math.min(1, amount(f, today()) / f.target_value) }">
                <b>{{ amount(f, today()) }}</b><small>/{{ f.target_value }}</small>
              </span>
              <button type="button" class="icon-btn" :aria-label="$t('Mehr')" :disabled="!todayCell(f).can" @click="setAmount(f, todayCell(f), amount(f, today()) + 1)"><svg class="ic"><use href="#i-plus" /></svg></button>
            </span>
            <button
              v-else-if="f.active" type="button" class="hd-check" role="checkbox" :aria-checked="todayCell(f).isDone" :disabled="!todayCell(f).can"
              :aria-label="$t('Heute abhaken: {name}', { name: f.name })" :title="$t('Heute abhaken: {name}', { name: f.name })" @click="toggleCell(f, todayCell(f))"
            ><svg class="ic" aria-hidden="true"><use href="#i-check" /></svg></button>
          </header>

          <p class="ha-stats">
            <template v-if="yearCount(f)">
              {{ $t('Serie {n}', { n: streaks[f.id] ?? 0 }) }} · {{ $t('Rekord {n}', { n: longest(f) }) }}<template v-if="rate(f) !== null"> · {{ $t('{p} % in 30 Tagen', { p: rate(f)! }) }}</template>
            </template>
            <template v-else>{{ $t('Noch nichts erledigt. Heute abhaken startet die Serie.') }}</template>
          </p>

          <form v-if="pauseFor === f.id" class="hd-pause" @submit.prevent="savePause">
            <label>{{ $t('von') }} <input v-model="pauseFrom" class="input" type="date" required /></label>
            <label>{{ $t('bis') }} <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required /></label>
            <button class="btn btn-primary" type="submit">{{ $t('Pausieren') }}</button>
            <button class="btn btn-ghost" type="button" @click="pauseFor = undefined">{{ $t('Abbrechen') }}</button>
          </form>
          <ul v-if="upcoming(pausesOf(f)).length" class="hd-chips">
            <li v-for="p in upcoming(pausesOf(f))" :key="p.id">
              {{ p.habit_id ? $t('Pause') : $t('Urlaub') }} · {{ spanLabel(p) }}
              <button type="button" class="icon-btn" :aria-label="$t('Pause entfernen')" @click="run(() => deleteHabitPause(p.id))"><svg class="ic"><use href="#i-x" /></svg></button>
            </li>
          </ul>

          <dl class="ha-props">
            <div><dt>{{ $t('Rhythmus') }}</dt><dd>{{ rhythm(f) }}</dd></div>
            <div v-if="f.preferred_time"><dt>{{ $t('Uhrzeit') }}</dt><dd>{{ f.preferred_time }} · {{ f.duration_minutes }} min</dd></div>
            <div v-if="f.target_value"><dt>{{ $t('Ziel') }}</dt><dd>{{ f.target_value }} {{ f.unit }}</dd></div>
            <div v-if="projectName(f.project_id)"><dt>{{ $t('Projekt') }}</dt><dd><span class="chip" :style="{ '--chip-c': projectColor(f.project_id) }">{{ projectName(f.project_id) }}</span></dd></div>
            <div><dt>{{ $t('Seit') }}</dt><dd>{{ fmtDay(f.start_date) }}<template v-if="yearCount(f)"> · {{ $t('{n}× im Jahr', { n: yearCount(f) }) }}</template></dd></div>
          </dl>

          <section class="ha-month">
            <div class="hm-nav">
              <button class="icon-btn" type="button" :aria-label="$t('Voriger Monat')" :disabled="monthOffset <= -11" @click="monthOffset--"><svg class="ic"><use href="#i-left" /></svg></button>
              <span class="lbl">{{ monthTitle }}</span>
              <button class="icon-btn" type="button" :aria-label="$t('Nächster Monat')" :disabled="monthOffset >= 0" @click="monthOffset++"><svg class="ic"><use href="#i-right" /></svg></button>
            </div>
            <div class="hm-grid" role="group" :aria-label="`${f.name}, ${monthTitle}`">
              <span v-for="w in WEEKDAYS" :key="w" class="hm-wd" aria-hidden="true">{{ WD_LABEL[w]!.slice(0, 2) }}</span>
              <span v-for="i in lead" :key="`l${i}`" aria-hidden="true"></span>
              <button
                v-for="c in monthDays.map((d) => cell(f, d))" :key="c.s" type="button" class="hm-day" :class="[c.state, { today: c.today }]"
                role="checkbox" :aria-checked="c.isDone" :aria-label="c.label" :title="c.label" :disabled="!c.can" @click="toggleCell(f, c)"
              >{{ Number(c.s.slice(8)) }}</button>
            </div>
          </section>

          <div v-if="focusWd.length" class="hd-wd">
            <span class="lbl">{{ $t('Quote je Wochentag') }}</span>
            <div class="hd-wdbars">
              <span v-for="x in focusWd" :key="x.w" :title="`${WD_LABEL[x.w]}: ${x.pct ?? '–'} %`">
                <small class="hd-pct">{{ x.pct === null ? '–' : `${x.pct}%` }}</small><i :style="{ '--h': (x.pct ?? 0) / 100 }"></i><small>{{ WD_LABEL[x.w]!.slice(0, 2) }}</small>
              </span>
            </div>
          </div>

          <div class="ha-acts">
            <button v-if="f.active && !todayCell(f).isDone" type="button" @click="toggleSkip(f, today())">
              <svg class="ic" aria-hidden="true"><use href="#i-today" /></svg>{{ dayPause(f, today()) ? $t('Nicht mehr aussetzen') : $t('Heute aussetzen') }}
            </button>
            <button v-if="f.active" type="button" @click="openPause(f.id)"><svg class="ic" aria-hidden="true"><use href="#i-clock" /></svg>{{ $t('Pausieren …') }}</button>
            <button type="button" @click="openEdit(f)"><svg class="ic" aria-hidden="true"><use href="#i-pen" /></svg>{{ $t('Bearbeiten') }}</button>
            <button type="button" @click="run(() => updateHabit(f.id, { active: !f.active }))"><svg class="ic" aria-hidden="true"><use href="#i-moon" /></svg>{{ f.active ? $t('Deaktivieren') : $t('Aktivieren') }}</button>
            <button class="danger" type="button" @click="focusId = null; trash(f)"><svg class="ic" aria-hidden="true"><use href="#i-trash" /></svg>{{ $t('Löschen') }}</button>
          </div>
        </div>
      </template>
    </aside>

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
            <label id="h-icon">{{ $t('Symbol') }}</label>
            <div class="hd-icons" role="radiogroup" aria-labelledby="h-icon" :style="{ '--hc': form.id ? habitColor(form.id) : 'var(--a-blue-hi)' }">
              <button
                v-for="(id, k) in HABIT_ICONS" :key="k" type="button" role="radio" :aria-checked="iconKey(form) === k" :aria-label="k"
                @click="form.icon = k"
              ><svg class="ic" aria-hidden="true"><use :href="`#${id}`" /></svg></button>
            </div>
          </div>
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
            <div class="field"><label for="h-dur">{{ $t('Dauer (Min.)') }}</label><input id="h-dur" v-model.number="form.duration" class="input" type="number" min="5" max="1440" step="5" :disabled="!form.time && !form.project" /></div>
          </div>
          <div class="v-sub">{{ $t('Mit Uhrzeit erscheint die Gewohnheit als Block im Kalender.') }}</div>
          <label class="lbl"><input v-model="form.remind" type="checkbox" :disabled="!form.time" />{{ $t('Zur Uhrzeit erinnern, solange offen (ntfy)') }}</label>
          <div class="field"><label for="h-proj">{{ $t('Aus Zeiterfassung') }}</label>
            <select id="h-proj" v-model="form.project" class="input">
              <option value="">{{ $t('Nur von Hand abhaken') }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
          <div v-if="form.project" class="v-sub">{{ $t('Erledigt sich selbst, sobald an einem Tag so viele Minuten im Projekt erfasst sind wie die Dauer.') }}</div>
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
.hk-note { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Graph-Karten */
.hk-list { display: grid; }
.hk-card { padding: 20px 0 18px; border-bottom: 1px solid var(--br-subtle); }
.hk-head { display: flex; align-items: center; gap: 14px; margin-bottom: 14px; }
.hk-icon { flex: none; display: grid; place-items: center; width: 36px; height: 36px; border-radius: 9px;
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary)); border: 0; background: color-mix(in srgb, var(--hc) 14%, var(--bg-2)); }
.hk-icon.sm { width: 30px; height: 30px; border-radius: 8px; font-size: 13px; }
.hk-id { display: grid; gap: 3px; min-width: 0; flex: 1; }
.hk-name { font: 500 15px/1.3 var(--font-ui); color: var(--tx-primary); }
.hk-chips { display: flex; align-items: center; gap: 22px; margin-right: 8px; }
.hk-kpi { display: grid; justify-items: end; gap: 3px; font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hk-kpi b { font: 300 20px/1 var(--font-ui); letter-spacing: -.02em; color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.hk-check { flex: none; display: grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; cursor: pointer;
  border: 1.5px solid var(--br-strong); background: transparent; color: transparent; transition: background var(--dur) ease-out; }
.hk-check:hover:not(:disabled) { border-color: var(--tx-secondary); color: var(--tx-muted); }
.hk-check[aria-checked="true"] { background: var(--tx-primary); border-color: var(--tx-primary); color: var(--bg-0); }
.hk-check:disabled { cursor: default; opacity: .35; border-style: dashed; }
.hk-check .ic { width: 14px; height: 14px; stroke-width: 3; }
.hk-graph { display: grid; gap: 4px; }
.hk-months, .hk-grid { display: grid; grid-template-columns: repeat(53, minmax(0, 1fr)); column-gap: 3px; }
.hk-months { font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hk-months span { white-space: nowrap; }
.hk-grid { grid-template-rows: repeat(7, auto); grid-auto-flow: column; row-gap: 3px; }
.hk-grid i { aspect-ratio: 1; border-radius: 2px; background: color-mix(in srgb, var(--hc) 7%, var(--bg-2)); }
.hk-grid i.done { background: color-mix(in srgb, var(--hc) 82%, var(--bg-0)); }
.hk-grid i.off { background: var(--bg-1); }
.hk-grid i.pause { background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px, transparent 2px 4px); }
.hk-grid i.pre { background: var(--bg-2); opacity: .5; }
.hk-grid i.future { background: transparent; }
.hk-grid i.today { outline: 1.5px solid var(--tx-primary); outline-offset: 0; }
.hk-grid i[role="checkbox"] { cursor: pointer; }
.hk-grid i[role="checkbox"]:hover { outline: 1.5px solid var(--tx-secondary); }

/* Überschrift der Monatsnavigation */
.hd-title { margin: 0 0 14px; font: 600 18px/1.2 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }

/* Monatsansicht */
.hm-nav { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
.hm-nav .hd-title { margin: 0; min-width: 170px; text-align: center; }
.hm-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 36px 40px; }
.hm-card { padding: 0; }
.hm-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.hm-stat { font: 400 13px/1 var(--font-mono); color: var(--tx-muted); }
.hm-stat b { font-size: 16px; color: var(--tx-primary); }
.hm-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 4px; }
.hm-wd { text-align: center; font: 500 11px/1.6 var(--font-ui); color: var(--tx-muted); }
.hm-day { aspect-ratio: 1; display: grid; place-items: center; border: 0; border-radius: 7px; cursor: pointer;
  font: 500 11px/1 var(--font-ui); color: var(--tx-secondary); background: color-mix(in srgb, var(--hc) 10%, var(--bg-2)); }
.hm-day:hover:not(:disabled) { outline: 1.5px solid var(--tx-secondary); }
.hm-day.done { background: color-mix(in srgb, var(--hc) 26%, var(--bg-1)); color: var(--tx-primary); font-weight: 500; box-shadow: inset 0 -2px 0 var(--hc); text-decoration: none; } /* styles.css streicht .done global durch */
.hm-day.off { background: transparent; color: var(--tx-disabled); }
.hm-day.pause { background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px, transparent 2px 4px); color: var(--tx-muted); }
.hm-day.pre, .hm-day.future { background: transparent; color: var(--tx-disabled); }
.hm-day.today { outline: 1.5px solid var(--tx-primary); }
.hm-day:disabled { cursor: default; }

.inactive { opacity: .55; }
/* Beim Überfahren ersetzen die Aktionen die Rhythmus-Zeile. */
.hab-sub { display: grid; align-items: center; min-height: 20px; }
.hab-sub > * { grid-area: 1 / 1; }
.acts { display: inline-flex; gap: 2px; margin-left: -8px; opacity: 0; pointer-events: none; transition: opacity var(--dur) ease-out; }
.acts .btn { min-height: 0; height: 20px; padding: 0 8px; font-size: 12px; }
.hk-card:hover .acts, .hk-card:focus-within .acts, .acts:has([aria-expanded="true"]) { opacity: 1; pointer-events: auto; }
.hd-more { width: 28px; justify-content: center; padding: 0 !important; }
.hd-more .ic { width: 16px; height: 16px; }
.hk-card:hover .hab-sched, .hk-card:focus-within .hab-sched { opacity: 0; }
.hab-foot { border-top: 0; padding-top: 4px; }

@media (max-width: 900px) {
  .hk-chips { display: none; }
}
/* Handy: Kennzahlen als eine schmale Zeile, damit die Liste zum Abhaken oben bleibt. */
@media (max-width: 640px) {
}

.hd-icon { flex: none; display: grid; place-items: center; width: 34px; height: 34px; border-radius: 9px; color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary)); border: 0; background: color-mix(in srgb, var(--hc) 14%, var(--bg-2)); }
.hd-icon .ic, .hk-icon .ic { width: 52%; height: 52%; stroke-width: 1.9; }
.hd-icons { display: grid; grid-template-columns: repeat(auto-fill, minmax(36px, 1fr)); gap: 6px; }
.hd-icons button { display: grid; place-items: center; aspect-ratio: 1; border: 1px solid var(--br-default); border-radius: 9px; background: var(--bg-1); color: var(--tx-secondary); cursor: pointer; }
.hd-icons button:hover { border-color: var(--br-strong); color: var(--tx-primary); }
.hd-icons button[aria-checked='true'] { border-color: var(--hc); color: var(--hc); background: color-mix(in srgb, var(--hc) 14%, transparent); }
.hd-icons .ic { width: 18px; height: 18px; }
.chip { display: inline-flex; align-items: center; height: 20px; padding: 0 8px; border-radius: 6px; font-size: 11.5px; font-weight: 500; background: color-mix(in srgb, var(--chip-c) 15%, transparent); color: color-mix(in srgb, var(--chip-c) 80%, var(--tx-primary)); white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
.chip .cdot { display: none; }
.hd-check { flex: none; display: grid; place-items: center; width: 24px; height: 24px; margin-right: 10px; border-radius: 50%; border: 1.5px solid var(--br-strong); background: transparent; color: transparent; cursor: pointer; transition: background var(--dur) ease-out, border-color var(--dur) ease-out, transform var(--dur) ease-out; }
.hd-check:hover:not(:disabled) { border-color: var(--tx-secondary); }
.hd-check[aria-checked='true'] { background: var(--tx-primary); border-color: var(--tx-primary); color: var(--bg-0); }
.hd-check:active:not(:disabled) { transform: scale(0.92); }
.hd-check:disabled { cursor: default; opacity: .4; }
.hd-check .ic { width: 13px; height: 13px; stroke-width: 3; }
.hd-wd { display: grid; gap: 10px; }
.hd-wdbars { display: grid; grid-template-columns: repeat(7, 1fr); gap: 12px; height: 84px; }
.hd-wdbars span { display: grid; grid-template-rows: auto 1fr auto; justify-items: center; gap: 5px; }
.hd-wdbars i { align-self: end; width: 100%; max-width: 18px; height: calc(3px + var(--h) * 44px); border-radius: 3px; background: color-mix(in srgb, var(--hc) calc(25% + var(--h) * 60%), var(--bg-2)); }
.hd-wdbars small { font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.hd-wdbars .hd-pct { align-self: end; font-variant-numeric: tabular-nums; color: var(--tx-muted); font-size: 11px; }
.hab-foot .hab-legend i.pause { border: 0; background: repeating-linear-gradient(135deg, var(--tx-muted) 0 2px, transparent 2px 4px); }
/* Mengen: −  Ring mit Zahl  + */
.hd-qty { flex: none; display: flex; align-items: center; gap: 2px; margin-right: 6px; }
.hd-qty .icon-btn { width: 26px; height: 26px; }
.hd-qty .icon-btn .ic { width: 14px; height: 14px; }
.hd-qring { --p: 0; display: flex; align-items: center; justify-content: center; width: 38px; height: 38px; border-radius: 50%;
  background: radial-gradient(closest-side, var(--bg-0) 84%, transparent 86%), conic-gradient(var(--tx-primary) calc(var(--p) * 1turn), var(--br-default) 0); }
.hd-qring.full { background: var(--tx-primary); }
.hd-qring b { font: 600 13px/1 var(--font-ui); color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.hd-qring small { margin-top: 2px; font: 400 10px/1 var(--font-ui); color: var(--tx-muted); }
.hd-qring.full b, .hd-qring.full small { color: var(--bg-0); }
/* Pausen */
.hd-pause { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; }
.hd-pause label { display: inline-flex; align-items: center; gap: 6px; font: 400 12.5px/1 var(--font-ui); color: var(--tx-muted); }
.hd-pause .input { width: auto; height: 30px; }
.hd-chips { display: flex; flex-wrap: wrap; gap: 6px; margin: -8px 0 0; padding: 0; list-style: none; }
.hd-chips li { display: inline-flex; align-items: center; gap: 4px; padding: 2px 4px 2px 10px; border-radius: 99px; font: 500 12px/1.6 var(--font-ui); color: var(--tx-secondary); background: var(--bg-2); }
.hd-chips .icon-btn { width: 20px; height: 20px; }
.hd-chips .icon-btn .ic { width: 12px; height: 12px; }
.hd-pause.card { margin: -6px 0 14px; padding: 12px 14px; }
.hd-pause-t { font: 500 13px/1.3 var(--font-ui); color: var(--tx-primary); margin-right: 4px; }
.hab-count { margin-left: 8px; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); font-variant-numeric: tabular-nums; margin-right: 4px; }

/* Wochenraster: Name | sieben Tage | Serie. Nur drei leise Zustände: Punkt (erledigt), Ring (offen), nichts. */
.hw { --cols: minmax(200px, 320px) repeat(7, minmax(44px, 1fr)) 56px; display: grid; margin-top: 4px; }
.hab-vac { height: 28px; padding: 0 10px; font-size: 13px; color: var(--tx-secondary); }
.hw-head, .hd-row { display: grid; grid-template-columns: var(--cols); align-items: center; }
.hw-head { position: sticky; top: 0; z-index: 1; padding: 6px 0 10px; background: var(--bg-0); border-bottom: 1px solid var(--br-subtle); }
.hw-nav { display: flex; align-items: center; gap: 2px; min-width: 0; }
.hw-nav .icon-btn { width: 26px; height: 26px; }
.hw-range { padding: 0 6px; font: 500 13px/1 var(--font-ui); color: var(--tx-secondary); white-space: nowrap; font-variant-numeric: tabular-nums; }
.hw-back { height: 24px; padding: 0 8px; font-size: 12px; }
.hw-d { display: grid; justify-items: center; gap: 4px; font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.hw-d small { font: 400 11.5px/1 var(--font-ui); }
.hw-d b { font: 400 15px/1 var(--font-ui); }
.hw-d.today { color: var(--tx-primary); }
.hw-d.today b { display: grid; place-items: center; width: 22px; height: 22px; margin: -3px 0; border-radius: 50%; background: var(--a-now); color: var(--bg-0); font-weight: 600; }
.hw-d.future { opacity: .5; }
.hw-group { display: grid; padding-top: 14px; }
.hw-gl { padding: 0 12px 8px; font: 500 12.5px/1 var(--font-ui); color: var(--tx-muted); }
.hd-row { min-height: 56px; border-radius: var(--r-s); transition: background var(--dur) ease-out; }
.hd-row:hover:not(.sel) { background: var(--bg-hover); }
.hd-row.sel { background: var(--bg-2); }
.hd-pick { display: flex; align-items: center; gap: 12px; min-width: 0; height: 100%; padding: 0 12px 0 8px; border: 0; background: none; text-align: left; color: inherit; cursor: pointer; border-radius: var(--r-s); }
.hw-name { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
.hd-pick .hd-icon { width: 32px; height: 32px; border-radius: 9px; }
.hd-row.inactive .hd-icon { filter: grayscale(1); opacity: .5; }
.hd-pick b { font: 500 15px/1.3 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hd-pick small { flex: none; font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); white-space: nowrap; }
.hd-pick:focus-visible, .hw-c:focus-visible { outline: 2px solid var(--a-focus); outline-offset: -2px; }
.hw-c { display: grid; place-items: center; height: 56px; padding: 0; border: 0; background: none; cursor: pointer; }
.hw-c > i { display: grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; box-sizing: border-box; color: var(--bg-0); transition: transform var(--dur) ease-out, background-color var(--dur) ease-out, border-color var(--dur) ease-out; }
.hw-c > i .ic { width: 15px; height: 15px; stroke-width: 3; }
.hw-c.open > i { border: 1.5px solid var(--tx-disabled); }
.hw-c.open.today > i { border: 2px solid var(--tx-secondary); }
.hw-c.open.soft > i { border-style: dashed; }
.hw-c.done > i { background: var(--tx-primary); }
.hw-c.part > i { border: 1.5px solid var(--tx-disabled); background: conic-gradient(color-mix(in srgb, var(--tx-primary) 75%, transparent) calc(var(--p) * 1turn), transparent 0); }
.hw-c.pause > i { background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 4px); }
.hw-c.plan > i { width: 5px; height: 5px; background: var(--tx-disabled); }
/* Überfahren zeigt, was ein Klick tut: auch an nicht geplanten Tagen lässt sich nachtragen. */
.hw-c:hover:not(:disabled) > i { border: 1.5px solid var(--tx-secondary); background-color: color-mix(in srgb, var(--tx-primary) 15%, transparent); }
.hw-c.done:hover:not(:disabled) > i { background-color: color-mix(in srgb, var(--tx-primary) 80%, var(--bg-0)); }
.hw-c:active:not(:disabled) > i { transform: scale(.9); }
.hw-legend { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 18px; padding: 12px 12px 0; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.hw-legend span { display: inline-flex; align-items: center; gap: 7px; }
.hw-legend i { display: inline-block; width: 12px; height: 12px; border-radius: 50%; box-sizing: border-box; }
.hw-legend .done { background: var(--tx-secondary); }
.hw-legend .open { border: 1.5px solid var(--tx-disabled); }
.hw-legend .part { border: 1.5px solid var(--tx-disabled); background: conic-gradient(var(--tx-secondary) 180deg, transparent 0); }
.hw-legend .soft { border: 1.5px dashed var(--tx-disabled); }
.hw-legend .pause { background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 3px); }
.hw-legend .plan { width: 5px; height: 5px; background: var(--tx-disabled); }
.hw-legend em { margin-left: auto; font-style: normal; }
.hw-c:disabled { cursor: default; }
.hw-streak-th { display: flex; align-items: center; justify-content: flex-end; padding-right: 12px; color: var(--tx-muted); }
.hw-streak-th .ic { width: 14px; height: 14px; }
.hw-streak { padding-right: 12px; text-align: right; font: 500 13px/1 var(--font-ui); color: var(--tx-muted); font-variant-numeric: tabular-nums; }
.hw-streak.risk { color: var(--tx-primary); font-weight: 600; }
.hw-sum { min-height: 52px; margin-top: 10px; border-top: 1px solid var(--br-subtle); border-radius: 0; }
.hw-sum:hover { background: none !important; }
.hw-sum .hw-gl { padding: 0 12px; }
.hw-ring { --p: 0; justify-self: center; display: grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; font: 500 10.5px/1 var(--font-ui); color: var(--tx-muted); font-variant-numeric: tabular-nums;
  background: radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%), conic-gradient(var(--tx-secondary) calc(var(--p) * 1turn), var(--bg-3) 0); }
.hw-ring:empty { background: none; }
.hw-ring.full { color: var(--tx-primary); background: radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%), var(--a-now); }
.hw-trend { margin-top: 28px; padding: 18px 12px 0; border-top: 1px solid var(--br-subtle); }
.hw-trend-h { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 14px; font: 500 12px/1 var(--font-ui); color: var(--tx-muted); }
.hw-trend-h b { font-weight: 500; color: var(--tx-secondary); font-variant-numeric: tabular-nums; }
.hw-bars { display: grid; grid-template-columns: repeat(12, 1fr); gap: 8px; height: 96px; }
.hw-bars span { display: grid; grid-template-rows: 1fr auto; justify-items: center; gap: 6px; }
.hw-bars i { align-self: end; width: 100%; max-width: 36px; height: calc(3px + var(--h) * 70px); border-radius: 4px; background: var(--bg-3); }
.hw-bars .cur i { background: var(--a-now); }
.hw-bars i.none { height: 0; }
.hw-bars small { font: 400 10.5px/1 var(--font-ui); color: var(--tx-muted); white-space: nowrap; }
.hw-add { display: inline-flex; align-items: center; gap: 8px; justify-self: start; margin-top: 14px; padding: 8px 12px; border: 0; border-radius: var(--r-s); background: none; color: var(--tx-muted); font: 400 13px/1 var(--font-ui); cursor: pointer; }
.hw-add:hover { background: var(--bg-hover); color: var(--tx-secondary); }
.hw-add .ic { width: 14px; height: 14px; }

/* Detail rechts wie bei Aufgaben: angedockte Spalte, unter 1000px als Sheet. */
@media (min-width: 1001px) {
  .view-inner.hab-split { display: grid; grid-template-columns: minmax(0, 1fr) 380px; max-width: none; padding: 0; }
  .hab-split .hab-main { min-width: 0; padding: 32px 40px 64px 32px; }
  .hab-split .hab-aside { position: sticky; top: 56px; align-self: start; height: calc(100dvh - 56px); }
}
.hab-aside { display: flex; flex-direction: column; background: var(--bg-1); border-left: 1px solid var(--br-subtle); overflow-y: auto; scrollbar-width: none; }
.hab-aside::-webkit-scrollbar { display: none; }
.aside-head { display: flex; align-items: center; justify-content: space-between; padding: 18px 20px 0 24px; }
.ha-body { display: grid; gap: 22px; align-content: start; padding: 12px 24px 24px; }
.ha-head { display: flex; align-items: center; gap: 12px; }
.ha-head .hd-icon { width: 36px; height: 36px; }
.hd-name { flex: 1; min-width: 0; display: grid; gap: 3px; }
.hd-name b { font: 600 18px/1.25 var(--font-ui); letter-spacing: -.015em; color: var(--tx-primary); }
.hd-name small { font: 400 12.5px/1.4 var(--font-ui); color: var(--tx-muted); }
.ha-head .hd-check { width: 32px; height: 32px; margin: 0; }
.ha-head .hd-check[aria-checked='true'] { background: var(--tx-primary); border-color: var(--tx-primary); }
.ha-head .hd-qty { margin: 0; }
.ha-stats { margin: -8px 0 0; font: 400 13px/1.5 var(--font-ui); color: var(--tx-secondary); font-variant-numeric: tabular-nums; }
.ha-props { margin: 0; border-top: 1px solid var(--br-subtle); }
.ha-props div { display: flex; align-items: center; min-height: 38px; border-bottom: 1px solid var(--br-subtle); font: 400 13px/1.3 var(--font-ui); }
.ha-props dt { flex: none; width: 96px; color: var(--tx-muted); }
.ha-props dd { margin: 0; color: var(--tx-primary); }
.ha-month { display: grid; gap: 10px; }
.ha-month .hm-nav { justify-content: space-between; margin: 0; }
.ha-month .hm-nav .icon-btn { width: 24px; height: 24px; }
.ha-month .hm-grid { gap: 3px; }
.ha-month .hm-day { justify-self: center; width: 30px; height: 30px; aspect-ratio: auto; border-radius: 50%; background: transparent; font-size: 11.5px; }
.ha-month .hm-day.done { background: color-mix(in srgb, var(--hc) 22%, transparent); color: var(--tx-primary); box-shadow: inset 0 0 0 1.5px var(--hc); }
.ha-month .hm-day.open { box-shadow: inset 0 0 0 1.5px var(--br-default); }
.ha-month .hm-day.today { outline: 0; color: var(--tx-primary); font-weight: 600; }
.ha-acts { display: grid; margin: 0 -10px; padding-top: 8px; border-top: 1px solid var(--br-subtle); }
.ha-acts button { display: flex; align-items: center; gap: 10px; height: 34px; padding: 0 10px; border: 0; border-radius: var(--r-s); background: none; color: var(--tx-secondary); font: 400 13px/1 var(--font-ui); text-align: left; cursor: pointer; }
.ha-acts button:hover { background: var(--bg-hover); color: var(--tx-primary); }
.ha-acts .ic { width: 15px; height: 15px; color: var(--tx-muted); }
.ha-acts .danger { margin-top: 6px; color: var(--a-red); }
.ha-acts .danger .ic { color: currentColor; }
.hab-aside .hd-chips { margin: -8px 0 0; }
@media (max-width: 1000px) {
  .hab-aside { position: fixed; inset: auto 0 0 0; z-index: 20; max-height: 65vh; border: 0; border-radius: var(--r-m) var(--r-m) 0 0; box-shadow: var(--shadow-pop); }
}
@media (max-width: 760px) {
  .hab-aside { bottom: calc(56px + env(safe-area-inset-bottom)); z-index: 55; max-height: calc(100dvh - 56px - env(safe-area-inset-bottom) - 48px); }
  .hw { --cols: minmax(0, 1fr) repeat(7, 32px) 0; max-width: none; }
  .hw-streak { display: none; }
  .hd-pick { gap: 10px; }
  .hw-name { flex-direction: column; align-items: flex-start; gap: 2px; }
  .hw-bars small { display: none; }
  .hw-range { display: none; }
  .hw-c { height: 52px; }
  .hw-c > i { width: 24px; height: 24px; }
  .hw-legend em { display: none; }
  .hab-count { display: none; }
  .hd-pick b { max-width: 100%; }
}
</style>
