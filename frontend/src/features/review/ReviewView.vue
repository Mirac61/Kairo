<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  ApiError,
  api,
  createGoal,
  createNote,
  deleteGoal,
  deleteNote,
  getReview,
  listGoals,
  listHabits,
  listProjects,
  listTasks,
  updateGoal,
  uploadFile,
  type Goal,
  type Habit,
  type Project,
  type Review,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { addDays, hm as fmtDur, isoWeek, weekStart, ymd } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import GoalEditDialog from '@/features/goals/components/GoalEditDialog.vue'
import GoalDetailModal from './components/GoalDetailModal.vue'
import ReviewGoalsPanel, { type GoalCardVm } from './components/ReviewGoalsPanel.vue'
import ReviewWeekMetrics from './components/ReviewWeekMetrics.vue'
import ReviewWeeklyRhythm from './components/ReviewWeeklyRhythm.vue'
import { goalPercent } from './goalUtils'
import type { DoneTask } from './types'

const IMAGE_DIR = 'Ziele'

const shortFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }),
)
const weekdayShort = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short' }))
const weekdayLong = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long' }))
const today = () => ymd(new Date())

const monday = ref(weekStart(new Date()))
const review = ref<Review | null>(null)
const tasks = ref<DoneTask[]>([])
const habits = ref<Habit[]>([])
const projects = ref<Project[]>([])
const goals = ref<Goal[]>([])
const habitWeekMap = ref<Record<string, Set<string>>>({})

const showReached = ref(false)
const hoveredProjectId = ref<string | null>(null)
const activeGoalId = ref<string | null>(null)
const editDialog = ref(false)
const editingGoal = ref<Goal | null>(null)

const from = computed(() => ymd(monday.value))
const to = computed(() => ymd(addDays(monday.value, 6)))
const isCurrent = computed(() => from.value >= ymd(weekStart(new Date())))
// Donnerstag der Woche, damit isoWeek auch bei Sonntag als Wochenstart die richtige KW liefert.
const kw = computed(() => isoWeek(addDays(monday.value, 3)))
const range = computed(
  () =>
    `${shortFmt.value.format(monday.value)} – ${shortFmt.value.format(addDays(monday.value, 6))}`,
)
const weekDateList = computed(() =>
  Array.from({ length: 7 }, (_, i) => ymd(addDays(monday.value, i))),
)

const { error, load, run } = useLoader(async () => {
  const f = from.value
  const tDate = to.value
  const [rv, ts, hs, ps, gs] = await Promise.all([
    getReview(f, tDate),
    listTasks(),
    listHabits(),
    listProjects(),
    listGoals(),
  ])
  if (f !== from.value) return
  const activeHabits = hs.filter((h) => h.active)
  const topHabits = activeHabits.slice(0, 5)
  const compLists = await Promise.all(
    topHabits.map((h) =>
      api<{ date: string; value: number | null }[]>(
        `/habits/${h.id}/completions?from=${f}&to=${tDate}`,
      ).catch(() => []),
    ),
  )
  if (f !== from.value) return // Wochenwechsel während der zweiten Anfrage-Runde
  review.value = rv
  tasks.value = ts as DoneTask[]
  habits.value = activeHabits
  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')
  goals.value = gs
  const map: Record<string, Set<string>> = {}
  topHabits.forEach((h, i) => {
    const target = h.target_value && h.target_value > 0 ? h.target_value : 1
    const set = new Set<string>()
    for (const c of compLists[i] ?? []) if (c.value == null || c.value >= target) set.add(c.date)
    map[h.id] = set
  })
  habitWeekMap.value = map
})

onMounted(load)
useLiveEvents(load)

function go(m: Date) {
  monday.value = m
  void load()
}

// --- Wochenwerte ---
const planOf = (day: string, calendar: number) =>
  calendar +
  tasks.value
    .filter((tk) => tk.planned_date === day && tk.status !== 'CANCELLED')
    .reduce((s, tk) => s + tk.estimated_minutes, 0)

const days = computed(() =>
  (review.value?.days ?? []).map((d) => {
    const dt = new Date(`${d.date}T00:00`)
    return {
      date: d.date,
      tracked_minutes: d.tracked_minutes,
      plan: planOf(d.date, d.calendar_minutes),
      isToday: d.date === today(),
      future: d.date > today(),
      short: weekdayShort.value.format(dt).replace(/\.$/, ''),
      full: weekdayLong.value.format(dt),
    }
  }),
)
const maxMinutes = computed(() =>
  days.value.reduce((m, d) => Math.max(m, d.plan, d.tracked_minutes), 60),
)
const planToDate = computed(() =>
  days.value.filter((d) => !d.future).reduce((s, d) => s + d.plan, 0),
)
const planPercent = computed(() =>
  planToDate.value && review.value
    ? Math.round((review.value.tracked_minutes / planToDate.value) * 100)
    : null,
)
const weekTasksCount = computed(
  () =>
    tasks.value.filter(
      (tk) =>
        tk.planned_date &&
        tk.planned_date >= from.value &&
        tk.planned_date <= to.value &&
        tk.status !== 'CANCELLED',
    ).length,
)
const routineRate = computed(() => {
  const hs = review.value?.habits ?? []
  if (!hs.length) return null
  const pastDays = Math.max(1, days.value.filter((d) => !d.future).length)
  const totalDone = hs.reduce((s, h) => s + h.done, 0)
  return Math.min(100, Math.round((totalDone / (hs.length * pastDays)) * 100))
})
const projectRows = computed(() => {
  const list = (review.value?.projects ?? []).filter((p) => p.tracked_minutes > 0)
  const total = list.reduce((s, p) => s + p.tracked_minutes, 0) || 1
  return list
    .slice()
    .sort((a, b) => b.tracked_minutes - a.tracked_minutes)
    .slice(0, 6)
    .map((p, idx) => ({
      project_id: p.project_id,
      name: p.name,
      tracked_minutes: p.tracked_minutes,
      pct: Math.max(4, Math.round((p.tracked_minutes / total) * 100)),
      primary: idx === 0,
    }))
})
const hoveredProjectName = computed(() =>
  hoveredProjectId.value
    ? (projects.value.find((p) => p.id === hoveredProjectId.value)?.name ?? null)
    : null,
)
const rhythmRows = computed(() =>
  habits.value.slice(0, 5).map((h) => {
    const set = habitWeekMap.value[h.id] ?? new Set<string>()
    const daysDone = weekDateList.value.map((d) => set.has(d))
    return { id: h.id, name: h.name, days: daysDone, count: daysDone.filter(Boolean).length }
  }),
)
const rhythmRange = computed(() => `${days.value[0]?.short ?? ''} – ${days.value[6]?.short ?? ''}`)

// --- Ziele ---
const stepsByParent = computed(() => {
  const m = new Map<string, Goal[]>()
  for (const g of goals.value) {
    if (!g.parent_id) continue
    const list = m.get(g.parent_id) ?? []
    list.push(g)
    m.set(g.parent_id, list)
  }
  for (const [k, list] of m)
    m.set(
      k,
      list.slice().sort((a, b) => Number(a.archived) - Number(b.archived)),
    )
  return m
})
const stepsOf = (id: string) => stepsByParent.value.get(id) ?? []
const activeGoals = computed(() => goals.value.filter((g) => !g.parent_id && !g.archived))
const reachedGoals = computed(() => goals.value.filter((g) => !g.parent_id && g.archived))
const activeGoal = computed(() => goals.value.find((g) => g.id === activeGoalId.value) ?? null)

function goalWeeklyText(g: Goal): string {
  if (g.project_id && review.value) {
    const p = review.value.projects.find((x) => x.project_id === g.project_id)
    if (p && p.tracked_minutes > 0) return `+${fmtDur(p.tracked_minutes)} ${t('diese Woche')}`
  }
  if (g.habit_id && review.value) {
    const h = review.value.habits.find((x) => x.habit_id === g.habit_id)
    if (h && h.streak > 0) return `${h.streak} ${t('Tage Serie')}`
    if (h && h.done > 0) return `${h.done}/7 ${t('diese Woche')}`
  }
  return ''
}

const goalCards = computed<GoalCardVm[]>(() =>
  (showReached.value ? reachedGoals.value : activeGoals.value).map((g) => {
    const steps = stepsOf(g.id)
    return {
      goal: g,
      percent: goalPercent(g, steps),
      weeklyText: goalWeeklyText(g),
      dueShort: g.due_date ? shortFmt.value.format(new Date(`${g.due_date}T00:00`)) : '',
      doneSteps: steps.filter((s) => s.archived).length,
      totalSteps: steps.length,
      nextStepTitle: steps.find((s) => !s.archived)?.title ?? null,
    }
  }),
)

// --- Aktionen (useLoader.run lädt danach selbst neu) ---
const dropImage = (path: string | null) => (path ? deleteNote(path).catch(() => {}) : undefined)

async function uploadMediaFile(id: string, file: File) {
  await createNote(IMAGE_DIR, true).catch((e) => {
    if (!(e instanceof ApiError && e.status === 409)) throw e
  })
  const ext = file.name.split('.').pop()?.toLowerCase() || 'jpg'
  const path = `${IMAGE_DIR}/${id}-${Date.now()}.${ext}`
  await uploadFile(path, file)
  return path
}

function openNewGoal() {
  editingGoal.value = null
  editDialog.value = true
}
function openEditFromModal(g: Goal) {
  editingGoal.value = g
  editDialog.value = true
}
const handleSaveWhy = (g: Goal, why: string) => void run(() => updateGoal(g.id, { why }))
const handleToggleStep = (step: Goal, archived: boolean) =>
  void run(() => updateGoal(step.id, { archived }))
const handleAddStep = (g: Goal, title: string) =>
  void run(() => createGoal({ title, parent_id: g.id }))
const handleDeleteStep = (step: Goal) =>
  void run(async () => {
    await deleteGoal(step.id)
    await dropImage(step.image_path)
  })
const handleUploadMedia = (g: Goal, file: File) =>
  void run(async () => {
    const old = g.image_path
    const path = await uploadMediaFile(g.id, file)
    await updateGoal(g.id, { image_path: path })
    if (old && old !== path) await dropImage(old)
  })
const handleClearMedia = (g: Goal) =>
  void run(async () => {
    const old = g.image_path
    await updateGoal(g.id, { image_path: null })
    await dropImage(old)
  })
const handleArchiveGoal = (g: Goal, archived: boolean) =>
  void run(async () => {
    await updateGoal(g.id, { archived })
    editDialog.value = false
    if (archived && activeGoalId.value === g.id) activeGoalId.value = null
  })
const handleDeleteGoal = (g: Goal) =>
  void run(async () => {
    const images = [g.image_path, ...stepsOf(g.id).map((s) => s.image_path)]
    await deleteGoal(g.id)
    await Promise.all(images.map(dropImage))
    editDialog.value = false
    if (activeGoalId.value === g.id) activeGoalId.value = null
  })
const handleSaveDialog = (payload: { id?: string; body: Partial<Goal>; file: File | null }) =>
  void run(async () => {
    const { id, body, file } = payload
    const old = id ? (goals.value.find((x) => x.id === id)?.image_path ?? null) : null
    const g = id ? await updateGoal(id, body) : await createGoal(body)
    if (file) await updateGoal(g.id, { image_path: await uploadMediaFile(g.id, file) })
    // Altes Bild nur löschen, wenn es tatsächlich ersetzt oder entfernt wurde.
    const replaced = !!file || ('image_path' in body && body.image_path !== old)
    if (old && replaced) await dropImage(old)
    editDialog.value = false
  })
</script>

<template>
  <div class="view-inner pr-wrap">
    <header class="pr-head">
      <div class="pr-head-left">
        <h1 class="pr-title">{{ $t('Fortschritt') }}</h1>
        <div class="pr-week-nav">
          <button
            type="button"
            class="pr-nav-btn"
            :aria-label="$t('Vorherige Woche')"
            @click="go(addDays(monday, -7))"
          >
            ‹
          </button>
          <span class="pr-range">{{ range }} · {{ $t('KW') }} {{ kw }}</span>
          <button
            type="button"
            class="pr-nav-btn"
            :disabled="isCurrent"
            :aria-label="$t('Nächste Woche')"
            @click="go(addDays(monday, 7))"
          >
            ›
          </button>
          <button
            v-if="!isCurrent"
            type="button"
            class="pr-today-btn"
            @click="go(weekStart(new Date()))"
          >
            {{ $t('Diese Woche') }}
          </button>
        </div>
      </div>
      <button type="button" class="pr-add-btn" @click="openNewGoal">
        + {{ $t('Neues Ziel') }}
      </button>
    </header>

    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div v-if="review" class="pr-split">
      <ReviewWeekMetrics
        v-model:hovered-project-id="hoveredProjectId"
        :tracked-minutes="review.tracked_minutes"
        :plan-to-date="planToDate"
        :plan-percent="planPercent"
        :completed-tasks="review.completed_tasks"
        :week-tasks-count="weekTasksCount"
        :routine-rate="routineRate"
        :days="days"
        :max-minutes="maxMinutes"
        :projects="projectRows"
        :hovered-project-name="hoveredProjectName"
      />
      <section class="pr-right">
        <ReviewGoalsPanel
          v-model:show-reached="showReached"
          v-model:hovered-project-id="hoveredProjectId"
          :cards="goalCards"
          :active-count="activeGoals.length"
          :reached-count="reachedGoals.length"
          @select="activeGoalId = $event"
        />
        <ReviewWeeklyRhythm :rows="rhythmRows" :range-label="rhythmRange" />
      </section>
    </div>

    <GoalDetailModal
      :goal="activeGoal"
      :steps="activeGoal ? stepsOf(activeGoal.id) : []"
      :projects="projects"
      :habits="habits"
      :weekly-text="activeGoal ? goalWeeklyText(activeGoal) : ''"
      @close="activeGoalId = null"
      @save-why="handleSaveWhy"
      @toggle-step="handleToggleStep"
      @add-step="handleAddStep"
      @delete-step="handleDeleteStep"
      @upload-media="handleUploadMedia"
      @clear-media="handleClearMedia"
      @edit="openEditFromModal"
      @archive="handleArchiveGoal"
      @delete="handleDeleteGoal"
    />

    <GoalEditDialog
      :open="editDialog"
      :goal="editingGoal"
      :goals="goals"
      :projects="projects"
      :habits="habits"
      @close="editDialog = false"
      @save="handleSaveDialog"
      @delete="handleDeleteGoal"
      @toggle-archive="handleArchiveGoal"
    />
  </div>
</template>

<style scoped>
.pr-wrap {
  width: 100%;
  max-width: 1440px;
  margin: 0 auto;
}
.pr-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 28px;
}
.pr-title {
  margin: 0 0 8px;
  font: 600 24px/1.2 var(--font-ui);
  letter-spacing: -0.02em;
}
.pr-week-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-secondary);
}
.pr-range {
  font-variant-numeric: tabular-nums;
}
.pr-nav-btn,
.pr-today-btn,
.pr-add-btn {
  height: 28px;
  padding: 0 10px;
  border-radius: var(--r-s);
  border: 1px solid var(--br-default);
  background: var(--bg-1);
  color: var(--tx-secondary);
  font: 500 12.5px/1 var(--font-ui);
  cursor: pointer;
}
.pr-nav-btn:hover:not(:disabled),
.pr-today-btn:hover,
.pr-add-btn:hover {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.pr-nav-btn:disabled {
  opacity: 0.4;
  cursor: default;
}
.pr-split {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  gap: 48px;
  align-items: start;
}
.pr-right {
  display: flex;
  flex-direction: column;
  gap: 32px;
  min-width: 0;
}
@media (max-width: 960px) {
  .pr-split {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }
}
</style>
