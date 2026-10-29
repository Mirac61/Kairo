<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  getReview,
  isOpen,
  listHabits,
  listProjects,
  listTasks,
  updateTask,
  type Habit,
  type Review,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { locale } from '@/lib/i18n'
import type { DoneTask, ReviewDay } from './types'
import ReviewStatCards from './components/ReviewStatCards.vue'
import ReviewPlanChart from './components/ReviewPlanChart.vue'
import ReviewProjectTimes from './components/ReviewProjectTimes.vue'
import ReviewHabits from './components/ReviewHabits.vue'
import ReviewTasksList from './components/ReviewTasksList.vue'

const short = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' })
const today = () => ymd(new Date())

const monday = ref(weekStart(new Date()))
const review = ref<Review | null>(null)
const tasks = ref<DoneTask[]>([])
const habits = ref<Habit[]>([])

const from = computed(() => ymd(monday.value))
const to = computed(() => ymd(addDays(monday.value, 6)))
const isCurrent = computed(() => from.value === ymd(weekStart(new Date())))
// ISO-Kalenderwoche: die Woche, in der der Donnerstag liegt.
const kw = computed(() => {
  const thu = addDays(monday.value, 3)
  return Math.floor(Math.round((+thu - +new Date(thu.getFullYear(), 0, 1)) / 864e5) / 7) + 1
})
const range = computed(
  () =>
    `${short.format(monday.value)} – ${short.format(addDays(monday.value, 6))} ${addDays(monday.value, 6).getFullYear()}`,
)

const { error, load, run } = useLoader(async () => {
  const f = from.value
  const [rv, ts, hs] = await Promise.all([
    getReview(f, to.value),
    listTasks(),
    listHabits(),
    listProjects(),
  ]) // Projekte für die Farben
  if (f !== from.value) return // inzwischen andere Woche gewählt
  review.value = rv
  tasks.value = ts as DoneTask[]
  habits.value = hs
})

function go(m: Date) {
  monday.value = m
  void load()
}

// Plan je Tag: Termine plus geschätzte Minuten der für den Tag geplanten Aufgaben.
const planOf = (day: string, calendar: number) =>
  calendar +
  tasks.value
    .filter((t) => t.planned_date === day && t.status !== 'CANCELLED')
    .reduce((s, t) => s + t.estimated_minutes, 0)

const days = computed<ReviewDay[]>(() =>
  (review.value?.days ?? []).map((d, i) => ({
    ...d,
    i,
    plan: planOf(d.date, d.calendar_minutes),
    future: d.date > today(),
  })),
)

const planToDate = computed(() =>
  days.value.filter((d) => !d.future).reduce((s, d) => s + d.plan, 0),
)

const weekTasks = computed(() =>
  tasks.value.filter(
    (t) =>
      t.planned_date &&
      t.planned_date >= from.value &&
      t.planned_date <= to.value &&
      t.status !== 'CANCELLED',
  ),
)

const completed = computed(() =>
  tasks.value
    .filter(
      (t) =>
        t.status === 'COMPLETED' &&
        t.completed_at &&
        ymd(new Date(t.completed_at)) >= from.value &&
        ymd(new Date(t.completed_at)) <= to.value,
    )
    .sort((a, b) => a.completed_at!.localeCompare(b.completed_at!)),
)

// Für einen Tag dieser Woche geplant, noch offen und der Tag liegt vor heute (wie Überfällig in /api/today).
const overdue = computed(() =>
  tasks.value
    .filter(
      (t) =>
        t.planned_date &&
        t.planned_date >= from.value &&
        t.planned_date <= to.value &&
        t.planned_date < today() &&
        isOpen(t),
    )
    .sort((a, b) => a.planned_date!.localeCompare(b.planned_date!)),
)

const plan = (t: Task) => run(() => updateTask(t.id, { planned_date: today() }))

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">{{ $t('Woche') }}</h1>
        <div class="v-sub">{{ $t('Rückblick') }} · {{ range }} · {{ $t('KW') }} {{ kw }}</div>
      </div>
      <div class="rv-nav">
        <button
          v-if="!isCurrent"
          class="btn btn-ghost"
          type="button"
          @click="go(weekStart(new Date()))"
        >
          {{ $t('Diese Woche') }}
        </button>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Vorherige Woche')"
          @click="go(addDays(monday, -7))"
        >
          <svg class="ic"><use href="#i-left" /></svg>
        </button>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Nächste Woche')"
          :disabled="isCurrent"
          @click="go(addDays(monday, 7))"
        >
          <svg class="ic"><use href="#i-right" /></svg>
        </button>
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="review">
      <ReviewStatCards
        :review="review"
        :plan-to-date="planToDate"
        :week-tasks-count="weekTasks.length"
        :overdue="overdue"
      />

      <ReviewPlanChart :days="days" />

      <div class="start-grid">
        <div class="start-col">
          <ReviewProjectTimes
            :projects="review.projects"
            :tracked-minutes="review.tracked_minutes"
          />
          <ReviewHabits :review-habits="review.habits" :all-habits="habits" />
        </div>

        <div class="start-col">
          <ReviewTasksList
            :completed="completed"
            :overdue="overdue"
            :projects="review.projects"
            @plan="plan"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.rv-nav {
  display: flex;
  align-items: center;
  gap: 4px;
}
.rv-nav :disabled {
  opacity: 0.35;
  cursor: default;
}

.start-col :deep(.rv-group + .rv-group) {
  margin-top: 32px;
}
</style>
