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
import { locale, t } from '@/lib/i18n'
import {
  buildHabitCell,
  computeHabit30DayRate,
  computeLongestStreak,
  computeWeekdayRates,
  computeWeekProgress,
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
import HabitEditDialog from './components/HabitEditDialog.vue'
import HabitDetailAside from './components/HabitDetailAside.vue'
import HabitYearCard from './components/HabitYearCard.vue'
import HabitMonthCard from './components/HabitMonthCard.vue'
import HabitWeekView from './components/HabitWeekView.vue'

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
  (['day', 'month', 'year'] as const).find((v) => v === store.get(VIEW_KEY)) ?? 'day',
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
    api<{ habits: { habit_id: string; streak: number }[] }>(
      `/review?from=${currentToday}&to=${currentToday}`,
    ),
    listProjects(),
    listHabitPauses(),
  ])

  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')

  const completions = await Promise.all(
    hs.map((h) =>
      api<{ date: string; value: number | null }[]>(
        `/habits/${h.id}/completions?from=${from}&to=${currentToday}`,
      ),
    ),
  )

  habits.value = hs
  todayHabits.value = td.habits
  streaks.value = Object.fromEntries(rv.habits.map((h) => [h.habit_id, h.streak]))
  pauses.value = pz

  vals.value = Object.fromEntries(
    hs.map((h, i) => [h.id, Object.fromEntries(completions[i]!.map((c) => [c.date, c.value]))]),
  )

  // Mit Zielmenge ist ein Tag erst ab der Menge erledigt; ein Haken ohne Menge zählt voll.
  done.value = Object.fromEntries(
    hs.map((h, i) => [
      h.id,
      new Set(
        completions[i]!.filter(
          (c) => h.target_value == null || c.value == null || c.value >= h.target_value,
        ).map((c) => c.date),
      ),
    ]),
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

function handleSaveHabit({ id, body }: { id?: string; body: Parameters<typeof createHabit>[0] }) {
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
  return Array.from({ length: WEEKS_IN_GRAPH * 7 }, (_, i) => cell(h, addDays(gridStart(), i)))
}

const months = computed(() =>
  Array.from({ length: WEEKS_IN_GRAPH }, (_, w) => addDays(gridStart(), 7 * w))
    .map((d, w, all) => ({
      col: w + 1,
      label: w === 0 || d.getMonth() !== all[w - 1]!.getMonth() ? monthShortFmt.format(d) : '',
    }))
    .filter((m) => m.label && m.col < WEEKS_IN_GRAPH - 1)
    .filter((m, i, all) => !all[i + 1] || all[i + 1]!.col - m.col >= 3),
)

function toggleCell(h: Habit, c: HabitCell) {
  if (!c.can) return
  void run(() => (c.isDone ? uncompleteHabit(h.id, c.s) : completeHabit(h.id, c.s)))
}

function amount(h: Habit, s: string): number {
  return vals.value[h.id]?.[s] ?? (done.value[h.id]?.has(s) ? (h.target_value ?? 1) : 0)
}

function setAmount(h: Habit, c: HabitCell, v: number) {
  if (!c.can) return
  const had = c.s in (vals.value[h.id] ?? {})
  void run(() =>
    v > 0 ? completeHabit(h.id, c.s, v) : had ? uncompleteHabit(h.id, c.s) : Promise.resolve(),
  )
}

// Pause Management
const pauseFor = ref<string | null | undefined>(undefined)

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
  const p = pauses.value.find((x) => x.habit_id === h.id && x.start_date === s && x.end_date === s)
  void run(() => (p ? deleteHabitPause(p.id) : createHabitPause({ habit_id: h.id, start_date: s })))
}

const vacations = computed(() =>
  getUpcomingPauses(
    pauses.value.filter((p) => !p.habit_id),
    todayStr(),
  ),
)

const due = (h: Habit) => todayHabits.value.find((x) => x.id === h.id)
function toggleToday(h: Habit) {
  const x = due(h)
  if (!x) return
  void run(() => (x.done ? uncompleteHabit(h.id, todayStr()) : completeHabit(h.id, todayStr())))
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
    (a.preferred_time ?? '99').localeCompare(b.preferred_time ?? '99'),
  )
  const slot = (h: Habit) => {
    if (!h.active) return t('Inaktiv')
    if (!h.preferred_time) return t('Jederzeit')
    return [...SLOTS].reverse().find(([, from]) => h.preferred_time! >= from)?.[0] ?? t('Jederzeit')
  }
  return [...SLOTS.map(([l]) => l), t('Jederzeit'), t('Inaktiv')]
    .map((label) => ({ label, habits: hs.filter((h) => slot(h) === label) }))
    .filter((g) => g.habits.length)
})

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

  return {
    ...c,
    plan: c.state === 'future' && isHabitPlanned(h, d) && (!h.end_date || c.s <= h.end_date),
    p,
    label: `${dayLabelFmt.format(d)}: ${stateLabel}`,
  }
}

const todayCell = (h: Habit) => cell(h, new Date())
const streakAtRisk = (h: Habit) => (streaks.value[h.id] ?? 0) > 0 && !!due(h) && !due(h)?.done

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
  }),
)

const todayDue = computed(() => activeHabits.value.filter((h) => dueOn(h, new Date())))
const todayDone = computed(() => todayDue.value.filter((h) => todayCell(h).isDone).length)

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
    monthStart.value,
  ),
)
const monthDays = computed(() => {
  const m = monthStart.value
  return Array.from(
    { length: new Date(m.getFullYear(), m.getMonth() + 1, 0).getDate() },
    (_, i) => new Date(m.getFullYear(), m.getMonth(), i + 1),
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
        <label
          >{{ $t('von') }} <input v-model="pauseFrom" class="input" type="date" required
        /></label>
        <label
          >{{ $t('bis') }}
          <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required
        /></label>
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
          {{
            $t('Noch keine Gewohnheiten. Klein anfangen hilft, etwa zehn Minuten Lesen am Morgen.')
          }}
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
        <HabitYearCard
          v-for="h in sorted"
          :key="h.id"
          :habit="h"
          :months="months"
          :streak="streaks[h.id] ?? 0"
          :rate30="habit30DayRate(h)"
          :year-count="yearCount(h)"
          :is-due="Boolean(due(h))"
          :is-done="Boolean(due(h)?.done)"
          :cells="graph(h)"
          :wd-labels="wdLabels"
          @toggle-today="toggleToday"
          @toggle-cell="toggleCell"
          @edit="openEdit"
          @toggle-active="(h) => run(() => updateHabit(h.id, { active: !h.active }))"
          @trash="trash"
        />

        <div class="hab-foot">
          <span class="hab-legend"><i class="done"></i>{{ $t('erledigt') }}</span>
          <span class="hab-legend"><i></i>{{ $t('offen') }}</span>
          <span class="hab-legend"><i class="off"></i>{{ $t('nicht geplant') }}</span>
          <span class="hab-legend"><i class="pause"></i>{{ $t('ausgesetzt') }}</span>
          <span class="hab-hint">{{ $t('Klick auf ein Feld trägt nach oder nimmt zurück') }}</span>
        </div>
      </div>

      <!-- Wochenansicht -->
      <HabitWeekView
        v-else-if="view === 'day'"
        :week-label="weekLabel"
        :this-week="thisWeek"
        :today-due-count="todayDue.length"
        :today-done-count="todayDone"
        :week="week"
        :today-str="todayStr()"
        :wd-short-fmt="wdShortFmt"
        :grid-groups="gridGroups"
        :active-habits="activeHabits"
        :focus="focus"
        :streaks="streaks"
        :streak-at-risk="streakAtRisk"
        :meta="meta"
        :grid-cell="gridCell"
        :week-rates="weekRates"
        :trend="trend"
        :date-short-fmt="dateShortFmt"
        @prev-week="selDay = addDays(selDay, -7)"
        @next-week="selDay = addDays(selDay, 7)"
        @this-week-click="selDay = new Date()"
        @pick="pick"
        @toggle-cell="toggleCell"
        @open-new="openNew"
      />

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
          <HabitMonthCard
            v-for="h in sorted"
            :key="h.id"
            :habit="h"
            :stat="monthStat(h)"
            :wd-labels="wdLabels"
            :weekdays="WEEKDAYS"
            :lead="lead"
            :month-title="monthTitle"
            :cells="monthDays.map((d) => cell(h, d))"
            @toggle-cell="toggleCell"
          />
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
      @trash="
        (h) => {
          focusId = null
          trash(h)
        }
      "
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
</style>
