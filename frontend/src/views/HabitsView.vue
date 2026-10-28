<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  api,
  completeHabit,
  createHabit,
  createHabitPause,
  deleteHabit,
  deleteHabitPause,
  getToday,
  listHabitPauses,
  listHabits,
  listProjects,
  restoreHabit,
  uncompleteHabit,
  updateHabit,
  type Habit,
  type HabitPause,
  type Project,
  type TodayHabit,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useUndo } from '@/composables/useUndo'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { habitColor } from '@/lib/projectColor'
import { iconOf } from '@/lib/habitIcons'
import { locale, t } from '@/lib/i18n'
import {
  buildHabitCell,
  computeHabit30DayRate,
  computeLongestStreak,
  computeWeekdayRates,
  computeWeekProgress,
  formatHabitDescription,
  formatHabitRhythm,
  formatPauseSpan,
  getGridStartDate,
  getUpcomingPauses,
  getWeekdayLabels,
  isHabitPlanned,
  WEEKDAYS,
  WEEKS_IN_GRAPH,
  type HabitCell,
} from '@/lib/habits'
import SearchField from '@/components/SearchField.vue'
import HabitEditDialog from '@/components/HabitEditDialog.vue'
import HabitDetailAside from '@/components/HabitDetailAside.vue'

const wdLabels = computed(() => getWeekdayLabels(locale.value))

const habits = ref<Habit[]>([])
const todayHabits = ref<TodayHabit[]>([])
const done = ref<Record<string, Set<string>>>({})
const vals = ref<Record<string, Record<string, number | null>>>({})
const pauses = ref<HabitPause[]>([])
const streaks = ref<Record<string, number>>({})
const projects = ref<Project[]>([])

// Dialog State
const isEditDialogOpen = ref(false)
const editingHabit = ref<Habit | null>(null)

// View State
type View = 'day' | 'month' | 'year'
const VIEW_KEY = 'kairo-habits-view'
const view = ref<View>(
  (['day', 'month', 'year'] as const).find((v) => v === store.get(VIEW_KEY)) ?? 'day'
)
watch(view, (v) => store.set(VIEW_KEY, v))

const todayStr = () => ymd(new Date())
const gridStart = () => getGridStartDate()

const dayLabelFmt = new Intl.DateTimeFormat(locale.value, {
  weekday: 'short',
  day: '2-digit',
  month: '2-digit',
})
const dateShortFmt = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' })
const monthShortFmt = new Intl.DateTimeFormat(locale.value, { month: 'short' })
const wdShortFmt = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })

const { error, load, run } = useLoader(async () => {
  const currentToday = todayStr()
  const from = ymd(gridStart())

  const [hs, td, rv, ps, pz] = await Promise.all([
    listHabits(),
    getToday(currentToday),
    api<{ habits: { habit_id: string; streak: number }[] }>(`/review?from=${currentToday}&to=${currentToday}`),
    listProjects(),
    listHabitPauses(),
  ])

  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')

  const completions = await Promise.all(
    hs.map((h) =>
      api<{ date: string; value: number | null }[]>(
        `/habits/${h.id}/completions?from=${from}&to=${currentToday}`
      )
    )
  )

  habits.value = hs
  todayHabits.value = td.habits
  streaks.value = Object.fromEntries(rv.habits.map((h) => [h.habit_id, h.streak]))
  pauses.value = pz

  vals.value = Object.fromEntries(
    hs.map((h, i) => [
      h.id,
      Object.fromEntries(completions[i]!.map((c) => [c.date, c.value])),
    ])
  )

  // Mit Zielmenge ist ein Tag erst ab der Menge erledigt; ein Haken ohne Menge zählt voll.
  done.value = Object.fromEntries(
    hs.map((h, i) => [
      h.id,
      new Set(
        completions[i]!
          .filter(
            (c) =>
              h.target_value == null || c.value == null || c.value >= h.target_value
          )
          .map((c) => c.date)
      ),
    ])
  )
})

const { offer } = useUndo()

function trash(h: Habit) {
  void run(async () => {
    await deleteHabit(h.id)
    offer(t('„{title}“ gelöscht', { title: h.name }), () => run(() => restoreHabit(h.id)))
  })
}

function openNew() {
  editingHabit.value = null
  isEditDialogOpen.value = true
}

function openEdit(h: Habit) {
  editingHabit.value = h
  isEditDialogOpen.value = true
}

function handleSaveHabit({
  id,
  body,
}: {
  id?: string
  body: Parameters<typeof createHabit>[0]
}) {
  void run(async () => {
    if (id) {
      await updateHabit(id, body)
    } else {
      await createHabit(body)
    }
    isEditDialogOpen.value = false
  })
}

function cell(h: Habit, d: Date): HabitCell {
  return buildHabitCell(h, d, done.value[h.id], pauses.value, todayStr(), dayLabelFmt)
}

function graph(h: Habit): HabitCell[] {
  return Array.from({ length: WEEKS_IN_GRAPH * 7 }, (_, i) =>
    cell(h, addDays(gridStart(), i))
  )
}

const months = computed(() =>
  Array.from({ length: WEEKS_IN_GRAPH }, (_, w) => addDays(gridStart(), 7 * w))
    .map((d, w, all) => ({
      col: w + 1,
      label:
        w === 0 || d.getMonth() !== all[w - 1]!.getMonth() ? monthShortFmt.format(d) : '',
    }))
    .filter((m) => m.label && m.col < WEEKS_IN_GRAPH - 1)
    .filter((m, i, all) => !all[i + 1] || all[i + 1]!.col - m.col >= 3)
)

function toggleCell(h: Habit, c: HabitCell) {
  if (!c.can) return
  void run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))
}

function amount(h: Habit, s: string): number {
  return vals.value[h.id]?.[s] ?? (done.value[h.id]?.has(s) ? h.target_value ?? 1 : 0)
}

function setAmount(h: Habit, c: HabitCell, v: number) {
  if (!c.can) return
  const had = c.s in (vals.value[h.id] ?? {})
  void run(() =>
    v > 0
      ? completeHabit(h.id, c.s, v)
      : had
        ? uncompleteHabit(h.id, c.s)
        : Promise.resolve()
  )
}

// Pause Management
const pauseFor = ref<string | null | undefined>(undefined)
const moreMenuOpen = ref<string | null>(null)
const pauseFrom = ref(todayStr())
const pauseUntil = ref(ymd(addDays(new Date(), 7)))

function openPause(habitId: string | null) {
  pauseFor.value = habitId
  pauseFrom.value = todayStr()
  pauseUntil.value = ymd(addDays(new Date(), 7))
}

function savePause() {
  const habit_id = pauseFor.value ?? null
  if (pauseUntil.value < pauseFrom.value) return
  void run(async () => {
    await createHabitPause({
      habit_id,
      start_date: pauseFrom.value,
      end_date: pauseUntil.value,
    })
    pauseFor.value = undefined
  })
}

function handleCreatePause(payload: { habit_id: string; start_date: string; end_date: string }) {
  void run(async () => {
    await createHabitPause(payload)
  })
}

function handleDeletePause(pauseId: string) {
  void run(() => deleteHabitPause(pauseId))
}

function toggleSkip(h: Habit, s: string) {
  const p = pauses.value.find(
    (x) => x.habit_id === h.id && x.start_date === s && x.end_date === s
  )
  void run(() =>
    p ? deleteHabitPause(p.id) : createHabitPause({ habit_id: h.id, start_date: s })
  )
}

const vacations = computed(() =>
  getUpcomingPauses(
    pauses.value.filter((p) => !p.habit_id),
    todayStr()
  )
)

const due = (h: Habit) => todayHabits.value.find((x) => x.id === h.id)
function toggleToday(h: Habit) {
  const x = due(h)
  if (!x) return
  void run(() =>
    x.done ? uncompleteHabit(h.id, todayStr()) : completeHabit(h.id, todayStr())
  )
}

const yearCount = (h: Habit) => done.value[h.id]?.size ?? 0
const habit30DayRate = (h: Habit) =>
  computeHabit30DayRate(h, done.value[h.id], pauses.value, todayStr())

const weekOf = (d: Date) => Array.from({ length: 7 }, (_, i) => addDays(weekStart(d), i))
const getWeekProgress = (h: Habit, days = weekOf(new Date())) =>
  computeWeekProgress(h, done.value[h.id], pauses.value, days)

function dueOn(h: Habit, d: Date) {
  const c = cell(h, d)
  if (!h.active || c.state === 'pre' || c.state === 'off' || (c.state === 'pause' && !c.isDone)) {
    return false
  }
  if (c.isDone || h.frequency_type !== 'TIMES_PER_WEEK') return true
  return getWeekProgress(h, weekOf(d)).done < (h.frequency_config.times ?? 1)
}

// Week grid
const selDay = ref(new Date())
const week = computed(() => weekOf(selDay.value))
const thisWeek = computed(() => ymd(weekStart(selDay.value)) >= ymd(weekStart()))
const weekLabel = computed(() => {
  const startDay = dateShortFmt.format(new Date(`${ymd(week.value[0]!)}T12:00:00`))
  const endDay = dateShortFmt.format(new Date(`${ymd(week.value[6]!)}T12:00:00`))
  return `${startDay} – ${endDay}`
})

const SLOTS = [
  [t('Morgens'), '00:00'],
  [t('Mittags'), '12:00'],
  [t('Abends'), '17:00'],
] as const

const activeHabits = computed(() => habits.value.filter((h) => h.active))
const search = ref('')
const sorted = computed(() => {
  const q = search.value.trim().toLowerCase()
  return habits.value
    .filter((h) => !q || h.name.toLowerCase().includes(q))
    .sort((a, b) => Number(b.active) - Number(a.active))
})

const gridGroups = computed(() => {
  const hs = [...sorted.value].sort((a, b) =>
    (a.preferred_time ?? '99').localeCompare(b.preferred_time ?? '99')
  )
  const slot = (h: Habit) => {
    if (!h.active) return t('Inaktiv')
    if (!h.preferred_time) return t('Jederzeit')
    return (
      [...SLOTS].reverse().find(([, from]) => h.preferred_time! >= from)?.[0] ??
      t('Jederzeit')
    )
  }
  return [...SLOTS.map(([l]) => l), t('Jederzeit'), t('Inaktiv')]
    .map((label) => ({ label, habits: hs.filter((h) => slot(h) === label) }))
    .filter((g) => g.habits.length)
})

function gridCell(h: Habit, d: Date) {
  const c = cell(h, d)
  const p =
    h.target_value && !c.isDone ? Math.min(1, amount(h, c.s) / h.target_value) : 0
  const isSoft = h.frequency_type === 'TIMES_PER_WEEK'
  let stateLabel = c.isDone ? t('erledigt') : c.state === 'pause' ? t('ausgesetzt') : t('offen')

  if (!c.isDone && p > 0 && h.target_value) {
    stateLabel = `${amount(h, c.s)}/${h.target_value} ${h.unit || ''} ${t('erledigt')}`.trim()
  } else if (!c.isDone && isSoft && c.state === 'open') {
    stateLabel = `${t('offen')} (${t('flexibel')})`
  }

  return {
    ...c,
    plan: c.state === 'future' && isHabitPlanned(h, d) && (!h.end_date || c.s <= h.end_date),
    p,
    label: `${dayLabelFmt.format(d)}: ${stateLabel}`,
  }
}

const todayCell = (h: Habit) => cell(h, new Date())
const streakAtRisk = (h: Habit) =>
  (streaks.value[h.id] ?? 0) > 0 && !!due(h) && !due(h)?.done

function dayRate(d: Date): number | null {
  if (ymd(d) > todayStr()) return null
  const cs = activeHabits.value
    .map((h) => cell(h, d))
    .filter((c) => c.state === 'done' || c.state === 'open')
  return cs.length ? cs.filter((c) => c.isDone).length / cs.length : null
}

const weekRates = computed(() => week.value.map(dayRate))

const trend = computed(() =>
  Array.from({ length: 12 }, (_, i) => {
    const days = weekOf(addDays(new Date(), (i - 11) * 7))
    const rs = days.map(dayRate).filter((r): r is number => r !== null)
    return {
      s: ymd(days[0]!),
      cur: i === 11,
      pct: rs.length ? Math.round((rs.reduce((a, b) => a + b, 0) / rs.length) * 100) : null,
    }
  })
)

const todayDue = computed(() => activeHabits.value.filter((h) => dueOn(h, new Date())))
const todayDone = computed(
  () => todayDue.value.filter((h) => todayCell(h).isDone).length
)

function meta(h: Habit): string {
  const sub =
    h.frequency_type === 'TIMES_PER_WEEK'
      ? t('{done}/{target} diese Woche', getWeekProgress(h, week.value))
      : formatHabitRhythm(h, wdLabels.value)
  return [h.preferred_time, sub].filter(Boolean).join(' · ')
}

const longest = (h: Habit) => computeLongestStreak(graph(h), streaks.value[h.id] ?? 0)
const wdRate = (h: Habit) => computeWeekdayRates(graph(h))

// Selection / Detail
const focusId = ref<string | null>(null)
const focus = computed(() => habits.value.find((h) => h.id === focusId.value) ?? null)
function pick(h: Habit) {
  focusId.value = focusId.value === h.id ? null : h.id
}

// Month View
const monthOffset = ref(0)
const monthStart = computed(() => {
  const n = new Date()
  return new Date(n.getFullYear(), n.getMonth() + monthOffset.value, 1)
})
const monthTitle = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(
    monthStart.value
  )
)
const monthDays = computed(() => {
  const m = monthStart.value
  return Array.from(
    { length: new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate() },
    (_, i) => new Date(m.getFullYear(), m.getMonth(), i + 1)
  )
})
const lead = computed(() => (monthStart.value.getDay() + 6) % 7)

function monthStat(h: Habit) {
  const cs = monthDays.value.map((d) => cell(h, d))
  return {
    done: cs.filter((c) => c.isDone).length,
    goal: cs.filter((c) => c.state === 'open' || c.state === 'done').length,
  }
}

useShortcuts((e) => {
  if (isEditDialogOpen.value) return
  if (e.key === 'n') {
    openNew()
  } else if (view.value !== 'day') {
    return
  } else if (e.key === 'j' || e.key === 'k') {
    const list = gridGroups.value.flatMap((g) => g.habits)
    const i = list.findIndex((h) => h.id === focus.value?.id)
    const next = list[Math.min(list.length - 1, Math.max(0, i + (e.key === 'j' ? 1 : -1)))]
    if (next) focusId.value = next.id
  } else if (e.key === 'x' && focus.value) {
    void toggleCell(focus.value, todayCell(focus.value))
  } else if (e.key === 'Escape' && focus.value) {
    focusId.value = null
  } else if (e.key === 'ArrowLeft') {
    selDay.value = addDays(selDay.value, -7)
  } else if (e.key === 'ArrowRight' && !thisWeek.value) {
    selDay.value = addDays(selDay.value, 7)
  } else {
    return
  }
  e.preventDefault()
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
          <button type="button" :aria-pressed="view === 'day'" @click="view = 'day'">
            {{ $t('Woche') }}
          </button>
          <button type="button" :aria-pressed="view === 'month'" @click="view = 'month'">
            {{ $t('Monat') }}
          </button>
          <button type="button" :aria-pressed="view === 'year'" @click="view = 'year'">
            {{ $t('Jahr') }}
          </button>
        </div>
        <span class="spacer"></span>
        <button
          class="btn btn-ghost hab-vac"
          type="button"
          :aria-expanded="pauseFor === null"
          @click="pauseFor === null ? (pauseFor = undefined) : openPause(null)"
        >
          {{ $t('Urlaub') }}
        </button>
        <SearchField v-model="search" :label="$t('Gewohnheiten durchsuchen')" />
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Neue Gewohnheit')"
          :title="`${$t('Neue Gewohnheit')} (N)`"
          @click="openNew"
        >
          <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
        </button>
      </div>

      <form v-if="pauseFor === null" class="hd-pause card" @submit.prevent="savePause">
        <span class="hd-pause-t">{{ $t('Alle Gewohnheiten pausieren') }}</span>
        <label>{{ $t('von') }} <input v-model="pauseFrom" class="input" type="date" required /></label>
        <label>{{ $t('bis') }} <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required /></label>
        <button class="btn btn-primary" type="submit">{{ $t('Pausieren') }}</button>
        <button class="btn btn-ghost" type="button" @click="pauseFor = undefined">
          {{ $t('Abbrechen') }}
        </button>
        <ul v-if="vacations.length" class="hd-chips">
          <li v-for="p in vacations" :key="p.id">
            {{ $t('Urlaub') }} · {{ formatPauseSpan(p, dateShortFmt) }}
            <button
              type="button"
              class="icon-btn"
              :aria-label="$t('Pause entfernen')"
              @click="handleDeletePause(p.id)"
            >
              <svg class="ic"><use href="#i-x" /></svg>
            </button>
          </li>
        </ul>
      </form>

      <div v-if="!habits.length" class="empty-cta">
        <p class="v-sub">
          {{ $t('Noch keine Gewohnheiten. Klein anfangen hilft, etwa zehn Minuten Lesen am Morgen.') }}
        </p>
        <button class="btn" type="button" @click="openNew">
          {{ $t('Erste Gewohnheit anlegen') }}
        </button>
      </div>
      <div v-else-if="!sorted.length" class="v-sub">
        {{ $t('Keine Treffer für „{q}“.', { q: search }) }}
      </div>

      <!-- Jahresansicht (Beitragsgraph) -->
      <div v-else-if="view === 'year'" class="hk-list">
        <article
          v-for="h in sorted"
          :key="h.id"
          class="hk-card"
          :class="{ inactive: !h.active }"
          :style="{ '--hc': habitColor(h.id) }"
        >
          <header class="hk-head">
            <span class="hk-icon" aria-hidden="true">
              <svg class="ic"><use :href="`#${iconOf(h)}`" /></svg>
            </span>
            <div class="hk-id">
              <span class="hk-name">{{ h.name }}</span>
              <span class="hab-sub">
                <span class="hab-sched">
                  {{ h.description || formatHabitDescription(h, wdLabels) }}
                  <template v-if="!h.active"> {{ $t('· inaktiv') }}</template>
                </span>
                <span class="acts">
                  <button class="btn btn-ghost" type="button" @click="openEdit(h)">
                    {{ $t('Bearbeiten') }}
                  </button>
                  <span class="menu-wrap" @keydown.esc="moreMenuOpen = null">
                    <button
                      class="btn btn-ghost hd-more"
                      type="button"
                      :aria-label="$t('Weitere Aktionen')"
                      :aria-expanded="moreMenuOpen === h.id"
                      @click="moreMenuOpen = moreMenuOpen === h.id ? null : h.id"
                    >
                      <svg class="ic"><use href="#i-more" /></svg>
                    </button>
                    <template v-if="moreMenuOpen === h.id">
                      <span class="menu-scrim" @click="moreMenuOpen = null"></span>
                      <span class="menu">
                        <button
                          class="menu-i"
                          type="button"
                          @click="moreMenuOpen = null; run(() => updateHabit(h.id, { active: !h.active }))"
                        >
                          {{ h.active ? $t('Deaktivieren') : $t('Aktivieren') }}
                        </button>
                        <button class="menu-i danger" type="button" @click="moreMenuOpen = null; trash(h)">
                          {{ $t('Löschen') }}
                        </button>
                      </span>
                    </template>
                  </span>
                </span>
              </span>
            </div>
            <div class="hk-chips">
              <span v-if="h.description" class="chip">{{ formatHabitDescription(h, wdLabels) }}</span>
              <span class="hk-kpi"><b>{{ streaks[h.id] ?? 0 }}</b>{{ $t('Serie') }}</span>
              <span class="hk-kpi">
                <b>{{ habit30DayRate(h) ?? '–' }}<template v-if="habit30DayRate(h) !== null"> %</template></b>
                {{ $t('30 Tage') }}
              </span>
              <span class="hk-kpi"><b>{{ yearCount(h) }}</b>{{ $t('im Jahr') }}</span>
            </div>
            <button
              type="button"
              class="hk-check"
              role="checkbox"
              :aria-checked="!!due(h)?.done"
              :disabled="!due(h)"
              :aria-label="$t('Heute abhaken: {name}', { name: h.name })"
              :title="due(h) ? $t('Heute abhaken: {name}', { name: h.name }) : $t('heute nicht fällig')"
              @click="toggleToday(h)"
            >
              <svg class="ic" aria-hidden="true"><use href="#i-check" /></svg>
            </button>
          </header>

          <div
            class="hk-graph"
            role="group"
            :aria-label="$t('Letztes Jahr: {n} Tage erledigt', { n: yearCount(h) })"
          >
            <div class="hk-months" aria-hidden="true">
              <span v-for="m in months" :key="m.col" :style="{ gridColumn: m.col }">{{ m.label }}</span>
            </div>
            <div class="hk-grid">
              <i
                v-for="c in graph(h)"
                :key="c.s"
                :class="[c.state, { today: c.today }]"
                :title="c.state === 'future' || c.state === 'pre' ? undefined : c.label"
                :role="c.can ? 'checkbox' : undefined"
                :aria-checked="c.can ? c.isDone : undefined"
                :aria-label="c.can ? c.label : undefined"
                :aria-hidden="c.can ? undefined : 'true'"
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

      <!-- Wochenansicht -->
      <div
        v-else-if="view === 'day'"
        class="hw"
        role="grid"
        :aria-label="$t('Woche {range}', { range: weekLabel })"
      >
        <div class="hw-head" role="row">
          <span class="hw-nav">
            <button
              class="icon-btn"
              type="button"
              :aria-label="$t('Vorige Woche')"
              @click="selDay = addDays(selDay, -7)"
            >
              <svg class="ic"><use href="#i-left" /></svg>
            </button>
            <span class="hw-range">{{ weekLabel }}</span>
            <button
              class="icon-btn"
              type="button"
              :aria-label="$t('Nächste Woche')"
              :disabled="thisWeek"
              @click="selDay = addDays(selDay, 7)"
            >
              <svg class="ic"><use href="#i-right" /></svg>
            </button>
            <button
              v-if="!thisWeek"
              class="btn btn-ghost hw-back"
              type="button"
              @click="selDay = new Date()"
            >
              {{ $t('Diese Woche') }}
            </button>
            <span v-else-if="todayDue.length" class="hab-count">
              {{ $t('{done}/{n} heute', { done: todayDone, n: todayDue.length }) }}
            </span>
          </span>

          <span
            v-for="d in week"
            :key="ymd(d)"
            class="hw-d"
            :class="{ today: ymd(d) === todayStr(), future: ymd(d) > todayStr() }"
            role="columnheader"
          >
            <small>{{ wdShortFmt.format(d).slice(0, 2) }}</small>
            <b>{{ d.getDate() }}</b>
          </span>

          <span
            class="hw-streak-th"
            :title="$t('Serie')"
            :aria-label="$t('Serie')"
            role="columnheader"
          >
            <svg class="ic" aria-hidden="true"><use href="#i-flame" /></svg>
          </span>
        </div>

        <div v-for="g in gridGroups" :key="g.label" class="hw-group" role="rowgroup">
          <span class="hw-gl">{{ g.label }}</span>
          <div
            v-for="h in g.habits"
            :key="h.id"
            class="hd-row"
            role="row"
            :class="{ sel: focus?.id === h.id, inactive: !h.active }"
            :style="{ '--hc': habitColor(h.id) }"
          >
            <button type="button" class="hd-pick" :aria-pressed="focus?.id === h.id" @click="pick(h)">
              <span class="hd-icon" aria-hidden="true">
                <svg class="ic"><use :href="`#${iconOf(h)}`" /></svg>
              </span>
              <span class="hw-name">
                <b>{{ h.name }}</b>
                <small>{{ meta(h) }}</small>
              </span>
            </button>

            <button
              v-for="c in week.map((d) => gridCell(h, d))"
              :key="c.s"
              type="button"
              class="hw-c"
              :class="[
                c.state,
                {
                  today: c.today,
                  plan: c.plan,
                  part: c.p > 0,
                  soft: h.frequency_type === 'TIMES_PER_WEEK',
                },
              ]"
              :style="c.p ? { '--p': c.p } : undefined"
              role="checkbox"
              :aria-checked="c.isDone"
              :aria-label="`${h.name}, ${c.label}`"
              :title="c.can ? c.label : undefined"
              :disabled="!c.can"
              @click="toggleCell(h, c)"
            >
              <i>
                <svg v-if="c.isDone" class="ic" aria-hidden="true"><use href="#i-check" /></svg>
              </i>
            </button>

            <span class="hw-streak" :class="{ risk: streakAtRisk(h) }" :title="$t('Serie')">
              {{ streaks[h.id] || '' }}
            </span>
          </div>
        </div>

        <div v-if="activeHabits.length" class="hd-row hw-sum" role="row">
          <span class="hw-gl">{{ $t('Tag gesamt') }}</span>
          <span
            v-for="(r, i) in weekRates"
            :key="i"
            class="hw-ring"
            :class="{ full: r === 1 }"
            :style="r === null ? undefined : { '--p': r }"
            :title="r === null ? undefined : `${Math.round(r * 100)} %`"
          >
            <template v-if="r !== null">{{ Math.round(r * 100) }}</template>
          </span>
          <span></span>
        </div>

        <p v-if="activeHabits.length" class="hw-legend">
          <span><i class="done"></i>{{ $t('erledigt') }}</span>
          <span><i class="open"></i>{{ $t('offen') }}</span>
          <span><i class="part"></i>{{ $t('teilweise') }}</span>
          <span><i class="soft"></i>{{ $t('flexibel') }}</span>
          <span><i class="pause"></i>{{ $t('ausgesetzt') }}</span>
          <span><i class="plan"></i>{{ $t('geplant') }}</span>
          <em>{{ $t('Kreis anklicken hakt den Tag ab, auch rückwirkend') }}</em>
        </p>

        <section v-if="activeHabits.length" class="hw-trend" :aria-label="$t('Letzte 12 Wochen')">
          <div class="hw-trend-h">
            <span>{{ $t('Letzte 12 Wochen') }}</span>
            <b v-if="trend[11]!.pct !== null">{{ $t('{p} % diese Woche', { p: trend[11]!.pct! }) }}</b>
          </div>
          <div class="hw-bars">
            <span
              v-for="w in trend"
              :key="w.s"
              :class="{ cur: w.cur }"
              :title="w.pct === null ? undefined : `${dateShortFmt.format(new Date(w.s + 'T12:00:00'))}: ${w.pct} %`"
            >
              <i v-if="w.pct !== null" :style="{ '--h': w.pct / 100 }"></i>
              <i v-else class="none"></i>
              <small>{{ dateShortFmt.format(new Date(w.s + 'T12:00:00')) }}</small>
            </span>
          </div>
        </section>

        <button class="hw-add" type="button" @click="openNew">
          <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
          {{ $t('Gewohnheit hinzufügen') }}
        </button>
      </div>

      <!-- Monatsansicht -->
      <div v-else class="hm">
        <div class="hm-nav">
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Voriger Monat')"
            :disabled="monthOffset <= -11"
            @click="monthOffset--"
          >
            <svg class="ic"><use href="#i-left" /></svg>
          </button>
          <h2 class="hd-title">{{ monthTitle }}</h2>
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Nächster Monat')"
            :disabled="monthOffset >= 0"
            @click="monthOffset++"
          >
            <svg class="ic"><use href="#i-right" /></svg>
          </button>
        </div>

        <div class="hm-cards">
          <article
            v-for="h in sorted"
            :key="h.id"
            class="hm-card"
            :class="{ inactive: !h.active }"
            :style="{ '--hc': habitColor(h.id) }"
          >
            <header class="hm-head">
              <span class="hk-icon sm" aria-hidden="true">
                <svg class="ic"><use :href="`#${iconOf(h)}`" /></svg>
              </span>
              <span class="hk-id">
                <span class="hk-name">{{ h.name }}</span>
                <span class="hab-sched">{{ formatHabitRhythm(h, wdLabels) }}</span>
              </span>
              <span class="hm-stat">
                <b>{{ monthStat(h).done }}</b>/{{ monthStat(h).goal }}
              </span>
            </header>

            <div class="hm-grid" role="group" :aria-label="`${h.name}, ${monthTitle}`">
              <span v-for="w in WEEKDAYS" :key="w" class="hm-wd" aria-hidden="true">
                {{ wdLabels[w]?.slice(0, 2) }}
              </span>
              <span v-for="i in lead" :key="`l${i}`" aria-hidden="true"></span>
              <button
                v-for="c in monthDays.map((d) => cell(h, d))"
                :key="c.s"
                type="button"
                class="hm-day"
                :class="[c.state, { today: c.today }]"
                role="checkbox"
                :aria-checked="c.isDone"
                :aria-label="c.label"
                :title="c.label"
                :disabled="!c.can"
                @click="toggleCell(h, c)"
              >
                {{ Number(c.s.slice(8)) }}
              </button>
            </div>
          </article>
        </div>
      </div>
    </div>

    <!-- Detailansicht rechts in der Wochenansicht -->
    <HabitDetailAside
      v-if="view === 'day' && focus"
      :habit="focus"
      :streak="streaks[focus.id] ?? 0"
      :longest-streak="longest(focus)"
      :rate30-days="habit30DayRate(focus)"
      :year-count="yearCount(focus)"
      :done-dates="done[focus.id]"
      :values="vals[focus.id]"
      :pauses="pauses"
      :projects="projects"
      :weekday-rates="wdRate(focus)"
      @close="focusId = null"
      @edit="openEdit"
      @trash="(h) => { focusId = null; trash(h) }"
      @toggle-active="(h) => run(() => updateHabit(h.id, { active: !h.active }))"
      @toggle-cell="toggleCell"
      @set-amount="setAmount"
      @toggle-skip="toggleSkip"
      @create-pause="handleCreatePause"
      @delete-pause="handleDeletePause"
    />

    <!-- Erstellen/Bearbeiten Dialog -->
    <HabitEditDialog
      :open="isEditDialogOpen"
      :habit="editingHabit"
      :projects="projects"
      @close="isEditDialogOpen = false"
      @save="handleSaveHabit"
    />
  </div>
</template>

<style scoped>
/* Kennzahlen */
.hk-note {
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Graph-Karten */
.hk-list {
  display: grid;
}
.hk-card {
  padding: 20px 0 18px;
  border-bottom: 1px solid var(--br-subtle);
}
.hk-head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 14px;
}
.hk-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 9px;
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary));
  border: 0;
  background: color-mix(in srgb, var(--hc) 14%, var(--bg-2));
}
.hk-icon.sm {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  font-size: 13px;
}
.hk-id {
  display: grid;
  gap: 3px;
  min-width: 0;
  flex: 1;
}
.hk-name {
  font: 500 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
}
.hk-chips {
  display: flex;
  align-items: center;
  gap: 22px;
  margin-right: 8px;
}
.hk-kpi {
  display: grid;
  justify-items: end;
  gap: 3px;
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hk-kpi b {
  font: 300 20px/1 var(--font-ui);
  letter-spacing: -0.02em;
  color: var(--tx-primary);
  font-variant-numeric: tabular-nums;
}
.hk-check {
  flex: none;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  cursor: pointer;
  border: 1.5px solid var(--br-strong);
  background: transparent;
  color: transparent;
  transition: background var(--dur) ease-out;
}
.hk-check:hover:not(:disabled) {
  border-color: var(--tx-secondary);
  color: var(--tx-muted);
}
.hk-check[aria-checked='true'] {
  background: var(--tx-primary);
  border-color: var(--tx-primary);
  color: var(--bg-0);
}
.hk-check:disabled {
  cursor: default;
  opacity: 0.35;
  border-style: dashed;
}
.hk-check .ic {
  width: 14px;
  height: 14px;
  stroke-width: 3;
}
.hk-graph {
  display: grid;
  gap: 4px;
}
.hk-months,
.hk-grid {
  display: grid;
  grid-template-columns: repeat(53, minmax(0, 1fr));
  column-gap: 3px;
}
.hk-months {
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hk-months span {
  white-space: nowrap;
}
.hk-grid {
  grid-template-rows: repeat(7, auto);
  grid-auto-flow: column;
  row-gap: 3px;
}
.hk-grid i {
  aspect-ratio: 1;
  border-radius: 2px;
  background: color-mix(in srgb, var(--hc) 7%, var(--bg-2));
}
.hk-grid i.done {
  background: color-mix(in srgb, var(--hc) 82%, var(--bg-0));
}
.hk-grid i.off {
  background: var(--bg-1);
}
.hk-grid i.pause {
  background: repeating-linear-gradient(
    135deg,
    color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px,
    transparent 2px 4px
  );
}
.hk-grid i.pre {
  background: var(--bg-2);
  opacity: 0.5;
}
.hk-grid i.future {
  background: transparent;
}
.hk-grid i.today {
  outline: 1.5px solid var(--tx-primary);
  outline-offset: 0;
}
.hk-grid i[role='checkbox'] {
  cursor: pointer;
}
.hk-grid i[role='checkbox']:hover {
  outline: 1.5px solid var(--tx-secondary);
}

/* Überschrift der Monatsnavigation */
.hd-title {
  margin: 0 0 14px;
  font: 600 18px/1.2 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}

/* Monatsansicht */
.hm-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.hm-nav .hd-title {
  margin: 0;
  min-width: 170px;
  text-align: center;
}
.hm-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 36px 40px;
}
.hm-card {
  padding: 0;
}
.hm-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.hm-stat {
  font: 400 13px/1 var(--font-mono);
  color: var(--tx-muted);
}
.hm-stat b {
  font-size: 16px;
  color: var(--tx-primary);
}
.hm-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 4px;
}
.hm-wd {
  text-align: center;
  font: 500 11px/1.6 var(--font-ui);
  color: var(--tx-muted);
}
.hm-day {
  aspect-ratio: 1;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 7px;
  cursor: pointer;
  font: 500 11px/1 var(--font-ui);
  color: var(--tx-secondary);
  background: color-mix(in srgb, var(--hc) 10%, var(--bg-2));
}
.hm-day:hover:not(:disabled) {
  outline: 1.5px solid var(--tx-secondary);
}
.hm-day.done {
  background: color-mix(in srgb, var(--hc) 26%, var(--bg-1));
  color: var(--tx-primary);
  font-weight: 500;
  box-shadow: inset 0 -2px 0 var(--hc);
  text-decoration: none;
}
.hm-day.off {
  background: transparent;
  color: var(--tx-disabled);
}
.hm-day.pause {
  background: repeating-linear-gradient(
    135deg,
    color-mix(in srgb, var(--hc, var(--tx-muted)) 35%, transparent) 0 2px,
    transparent 2px 4px
  );
  color: var(--tx-muted);
}
.hm-day.pre,
.hm-day.future {
  background: transparent;
  color: var(--tx-disabled);
}
.hm-day.today {
  outline: 1.5px solid var(--tx-primary);
}
.hm-day:disabled {
  cursor: default;
}

.inactive {
  opacity: 0.55;
}
/* Beim Überfahren ersetzen die Aktionen die Rhythmus-Zeile. */
.hab-sub {
  display: grid;
  align-items: center;
  min-height: 20px;
}
.hab-sub > * {
  grid-area: 1 / 1;
}
.acts {
  display: inline-flex;
  gap: 2px;
  margin-left: -8px;
  opacity: 0;
  pointer-events: none;
  transition: opacity var(--dur) ease-out;
}
.acts .btn {
  min-height: 0;
  height: 20px;
  padding: 0 8px;
  font-size: 12px;
}
.hk-card:hover .acts,
.hk-card:focus-within .acts,
.acts:has([aria-expanded='true']) {
  opacity: 1;
  pointer-events: auto;
}
.hd-more {
  width: 28px;
  justify-content: center;
  padding: 0 !important;
}
.hd-more .ic {
  width: 16px;
  height: 16px;
}
.hk-card:hover .hab-sched,
.hk-card:focus-within .hab-sched {
  opacity: 0;
}
.hab-foot {
  border-top: 0;
  padding-top: 4px;
}

@media (max-width: 900px) {
  .hk-chips {
    display: none;
  }
}

.hd-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 9px;
  color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary));
  border: 0;
  background: color-mix(in srgb, var(--hc) 14%, var(--bg-2));
}
.hd-icon .ic,
.hk-icon .ic {
  width: 52%;
  height: 52%;
  stroke-width: 1.9;
}

.chip {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 11.5px;
  font-weight: 500;
  background: color-mix(in srgb, var(--chip-c) 15%, transparent);
  color: color-mix(in srgb, var(--chip-c) 80%, var(--tx-primary));
  white-space: nowrap;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chip .cdot {
  display: none;
}

.hab-foot .hab-legend i.pause {
  border: 0;
  background: repeating-linear-gradient(135deg, var(--tx-muted) 0 2px, transparent 2px 4px);
}

/* Pausen */
.hd-pause {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.hd-pause label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-pause .input {
  width: auto;
  height: 30px;
}
.hd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: -8px 0 0;
  padding: 0;
  list-style: none;
}
.hd-chips li {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 4px 2px 10px;
  border-radius: 99px;
  font: 500 12px/1.6 var(--font-ui);
  color: var(--tx-secondary);
  background: var(--bg-2);
}
.hd-chips .icon-btn {
  width: 20px;
  height: 20px;
}
.hd-chips .icon-btn .ic {
  width: 12px;
  height: 12px;
}
.hd-pause.card {
  margin: -6px 0 14px;
  padding: 12px 14px;
}
.hd-pause-t {
  font: 500 13px/1.3 var(--font-ui);
  color: var(--tx-primary);
  margin-right: 4px;
}
.hab-count {
  margin-left: 8px;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  margin-right: 4px;
}

/* Wochenraster */
.hw {
  --cols: minmax(200px, 320px) repeat(7, minmax(44px, 1fr)) 56px;
  display: grid;
  margin-top: 4px;
}
.hab-vac {
  height: 28px;
  padding: 0 10px;
  font-size: 13px;
  color: var(--tx-secondary);
}
.hw-head,
.hd-row {
  display: grid;
  grid-template-columns: var(--cols);
  align-items: center;
}
.hw-head {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 6px 0 10px;
  background: var(--bg-0);
  border-bottom: 1px solid var(--br-subtle);
}
.hw-nav {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
}
.hw-nav .icon-btn {
  width: 26px;
  height: 26px;
}
.hw-range {
  padding: 0 6px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.hw-back {
  height: 24px;
  padding: 0 8px;
  font-size: 12px;
}
.hw-d {
  display: grid;
  justify-items: center;
  gap: 4px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.hw-d small {
  font: 400 11.5px/1 var(--font-ui);
}
.hw-d b {
  font: 400 15px/1 var(--font-ui);
}
.hw-d.today {
  color: var(--tx-primary);
}
.hw-d.today b {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  margin: -3px 0;
  border-radius: 50%;
  background: var(--a-now);
  color: var(--bg-0);
  font-weight: 600;
}
.hw-d.future {
  opacity: 0.5;
}
.hw-group {
  display: grid;
  padding-top: 14px;
}
.hw-gl {
  padding: 0 12px 8px;
  font: 500 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-row {
  min-height: 56px;
  border-radius: var(--r-s);
  transition: background var(--dur) ease-out;
}
.hd-row:hover:not(.sel) {
  background: var(--bg-hover);
}
.hd-row.sel {
  background: var(--bg-2);
}
.hd-pick {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  height: 100%;
  padding: 0 12px 0 8px;
  border: 0;
  background: none;
  text-align: left;
  color: inherit;
  cursor: pointer;
  border-radius: var(--r-s);
}
.hw-name {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.hd-pick .hd-icon {
  width: 32px;
  height: 32px;
  border-radius: 9px;
}
.hd-row.inactive .hd-icon {
  filter: grayscale(1);
  opacity: 0.5;
}
.hd-pick b {
  font: 500 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hd-pick small {
  flex: none;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
  white-space: nowrap;
}
.hd-pick:focus-visible,
.hw-c:focus-visible {
  outline: 2px solid var(--a-focus);
  outline-offset: -2px;
}
.hw-c {
  display: grid;
  place-items: center;
  height: 56px;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}
.hw-c > i {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  box-sizing: border-box;
  color: var(--bg-0);
  transition:
    transform var(--dur) ease-out,
    background-color var(--dur) ease-out,
    border-color var(--dur) ease-out;
}
.hw-c > i .ic {
  width: 15px;
  height: 15px;
  stroke-width: 3;
}
.hw-c.open > i {
  border: 1.5px solid var(--tx-disabled);
}
.hw-c.open.today > i {
  border: 2px solid var(--tx-secondary);
}
.hw-c.open.soft > i {
  border-style: dashed;
}
.hw-c.done > i {
  background: var(--tx-primary);
}
.hw-c.part > i {
  border: 1.5px solid var(--tx-disabled);
  background: conic-gradient(
    color-mix(in srgb, var(--tx-primary) 75%, transparent) calc(var(--p) * 1turn),
    transparent 0
  );
}
.hw-c.pause > i {
  background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 4px);
}
.hw-c.plan > i {
  width: 5px;
  height: 5px;
  background: var(--tx-disabled);
}
.hw-c:hover:not(:disabled) > i {
  border: 1.5px solid var(--tx-secondary);
  background-color: color-mix(in srgb, var(--tx-primary) 15%, transparent);
}
.hw-c.done:hover:not(:disabled) > i {
  background-color: color-mix(in srgb, var(--tx-primary) 80%, var(--bg-0));
}
.hw-c:active:not(:disabled) > i {
  transform: scale(0.9);
}
.hw-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 18px;
  padding: 12px 12px 0;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hw-legend span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.hw-legend i {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  box-sizing: border-box;
}
.hw-legend .done {
  background: var(--tx-secondary);
}
.hw-legend .open {
  border: 1.5px solid var(--tx-disabled);
}
.hw-legend .part {
  border: 1.5px solid var(--tx-disabled);
  background: conic-gradient(var(--tx-secondary) 180deg, transparent 0);
}
.hw-legend .soft {
  border: 1.5px dashed var(--tx-disabled);
}
.hw-legend .pause {
  background: repeating-linear-gradient(135deg, var(--tx-disabled) 0 1.5px, transparent 1.5px 3px);
}
.hw-legend .plan {
  width: 5px;
  height: 5px;
  background: var(--tx-disabled);
}
.hw-legend em {
  margin-left: auto;
  font-style: normal;
}
.hw-c:disabled {
  cursor: default;
}
.hw-streak-th {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 12px;
  color: var(--tx-muted);
}
.hw-streak-th .ic {
  width: 14px;
  height: 14px;
}
.hw-streak {
  padding-right: 12px;
  text-align: right;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.hw-streak.risk {
  color: var(--tx-primary);
  font-weight: 600;
}
.hw-sum {
  min-height: 52px;
  margin-top: 10px;
  border-top: 1px solid var(--br-subtle);
  border-radius: 0;
}
.hw-sum:hover {
  background: none !important;
}
.hw-sum .hw-gl {
  padding: 0 12px;
}
.hw-ring {
  --p: 0;
  justify-self: center;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  font: 500 10.5px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  background:
    radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%),
    conic-gradient(var(--tx-secondary) calc(var(--p) * 1turn), var(--bg-3) 0);
}
.hw-ring:empty {
  background: none;
}
.hw-ring.full {
  color: var(--tx-primary);
  background: radial-gradient(closest-side, var(--bg-0) 78%, transparent 80%), var(--a-now);
}
.hw-trend {
  margin-top: 28px;
  padding: 18px 12px 0;
  border-top: 1px solid var(--br-subtle);
}
.hw-trend-h {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 14px;
  font: 500 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hw-trend-h b {
  font-weight: 500;
  color: var(--tx-secondary);
  font-variant-numeric: tabular-nums;
}
.hw-bars {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: 8px;
  height: 96px;
}
.hw-bars span {
  display: grid;
  grid-template-rows: 1fr auto;
  justify-items: center;
  gap: 6px;
}
.hw-bars i {
  align-self: end;
  width: 100%;
  max-width: 36px;
  height: calc(3px + var(--h) * 70px);
  border-radius: 4px;
  background: var(--bg-3);
}
.hw-bars .cur i {
  background: var(--a-now);
}
.hw-bars i.none {
  height: 0;
}
.hw-bars small {
  font: 400 10.5px/1 var(--font-ui);
  color: var(--tx-muted);
  white-space: nowrap;
}
.hw-add {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  justify-self: start;
  margin-top: 14px;
  padding: 8px 12px;
  border: 0;
  border-radius: var(--r-s);
  background: none;
  color: var(--tx-muted);
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
}
.hw-add:hover {
  background: var(--bg-hover);
  color: var(--tx-secondary);
}
.hw-add .ic {
  width: 14px;
  height: 14px;
}

@media (min-width: 1001px) {
  .view-inner.hab-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 380px;
    max-width: none;
    padding: 0;
  }
  .hab-split .hab-main {
    min-width: 0;
    padding: 32px 40px 64px 32px;
  }
  .hab-split :deep(.hab-aside) {
    position: sticky;
    top: 56px;
    align-self: start;
    height: calc(100dvh - 56px);
  }
}

@media (max-width: 760px) {
  .hw {
    --cols: minmax(0, 1fr) repeat(7, 32px) 0;
    max-width: none;
  }
  .hw-streak {
    display: none;
  }
  .hd-pick {
    gap: 10px;
  }
  .hw-name {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .hw-bars small {
    display: none;
  }
  .hw-range {
    display: none;
  }
  .hw-c {
    height: 52px;
  }
  .hw-c > i {
    width: 24px;
    height: 24px;
  }
  .hw-legend em {
    display: none;
  }
  .hab-count {
    display: none;
  }
  .hd-pick b {
    max-width: 100%;
  }
}
</style>
