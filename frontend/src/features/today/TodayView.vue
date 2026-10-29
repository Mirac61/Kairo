<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  completeHabit,
  dueCards,
  goalToday,
  isOpen,
  readNote,
  reviewCard,
  taskAction,
  uncompleteHabit,
  updateTask,
  type Card,
  type Goal,
  type Task,
} from '@/api/client'
import { provideHome } from '@/composables/useHome'
import { useShortcuts } from '@/composables/useShortcuts'
import { dur, shiftDate } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { journalPath, parseEntry } from '@/lib/journal'
import { store } from '@/lib/storage'
import TodayHeroCard from './components/TodayHeroCard.vue'
import TodayQuickStrip from './components/TodayQuickStrip.vue'
import TodayTaskTable from './components/TodayTaskTable.vue'

// Startseite: links Hero-Karte mit dem, was jetzt dran ist, darunter Aufgaben-Tabelle;
// rechts kompakte Tagesübersicht (Termine, Gewohnheiten, Fokus-Ziel und Lernkarte).
const home = provideHome()
const {
  today,
  tasks,
  tasksById,
  projectNames,
  projectsById,
  nowMin,
  error,
  blocks,
  allDay,
  freeGaps,
  runEntry,
  runningTaskId,
  pausedTask,
  run,
  setDone,
  scheduleAt,
} = home

const events = computed(() => blocks.value.filter((b) => b.kind === 'event'))
const eventCount = computed(() => events.value.length + allDay.value.length)
const freeMin = computed(() => freeGaps.value.reduce((s, g) => s + g.end - g.start, 0))

const todayFormatted = computed(() => {
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(new Date())
})

// Aufgaben: Überfälliges, heute Offenes, Demnächst (nach Datum).
const overdue = computed(() => (today.value?.overdue ?? []).filter(isOpen))
const todays = computed(() => (today.value?.tasks ?? []).filter(isOpen))
const doneToday = computed(() => (today.value?.tasks ?? []).filter((x) => x.status === 'COMPLETED'))
const dayOf = (x: Task) => x.planned_date || x.due_at?.slice(0, 10) || null

const upcoming = computed(() => {
  const shown = new Set([...overdue.value, ...todays.value].map((x) => x.id))
  return tasks.value
    .filter((x) => isOpen(x) && !x.parent_task_id && !shown.has(x.id))
    .sort((a, b) => (dayOf(a) ?? '~').localeCompare(dayOf(b) ?? '~'))
    .slice(0, 6)
})

const current = computed(
  () =>
    (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined) ??
    pausedTask.value,
)
const heroTask = computed(() => [...overdue.value, ...todays.value][0] ?? null)
const planSkip = ref(store.get('kairo-plan-skip'))
const candidates = computed(() =>
  [...overdue.value, ...todays.value, ...upcoming.value]
    .filter((x) => !x.planned_start_at)
    .slice(0, 4),
)

const kind = computed(() => {
  const day = today.value?.date
  if (current.value || runEntry.value) return 'run'
  if (nowMin.value >= 18 * 60) return 'review'
  if (
    nowMin.value < 12 * 60 &&
    planSkip.value !== day &&
    candidates.value.length &&
    !blocks.value.some((b) => b.kind === 'task')
  ) {
    return 'plan'
  }
  return heroTask.value ? 'task' : 'done'
})

const heroId = computed(() =>
  kind.value === 'task' ? heroTask.value?.id : kind.value === 'run' ? current.value?.id : undefined,
)

const toggleTimer = (x: Task) =>
  run(() => taskAction(x.id, runningTaskId.value === x.id ? 'pause' : 'start'))

const fitGap = (x: Task) =>
  freeGaps.value.find((g) => g.end - g.start >= (x.estimated_minutes || 30))

function togglePlan(x: Task, at?: number) {
  if (at === undefined && x.planned_start_at)
    return run(() => updateTask(x.id, { planned_start_at: '' }))
  const start = at ?? fitGap(x)?.start
  if (start === undefined) error.value = t('Heute ist keine Lücke groß genug.')
  else scheduleAt(x, start)
}

// Gewohnheiten, Lernkarte, Ziel
const habits = computed(() => today.value?.habits ?? [])
const handleToggleHabit = (id: string, done: boolean) =>
  run(() => (done ? uncompleteHabit(id, today.value!.date) : completeHabit(id, today.value!.date)))

const cards = ref<Card[] | null>(null)
const goal = ref<Goal | null>(null)

onMounted(() => {
  dueCards().then(
    (cs) => (cards.value = cs),
    () => (cards.value = []),
  )
  goalToday().then(
    (g) => (goal.value = g),
    () => {},
  )
})

function handleGradeCard(gradeValue: number) {
  const currentCard = cards.value?.[0]
  if (!currentCard) return
  void run(async () => {
    await reviewCard(currentCard.key, gradeValue)
    cards.value = cards.value!.slice(1)
  })
}

// Aufgaben-Tabelle
const tableTasks = computed(() => {
  const seen = new Set<string>()
  const list: Task[] = []
  for (const x of [...overdue.value, ...todays.value, ...upcoming.value]) {
    if (x.id === heroId.value || seen.has(x.id)) continue
    seen.add(x.id)
    list.push(x)
  }
  return list
})

const ids = computed(() => tableTasks.value.map((x) => x.id))
const sel = ref<string | null>(null)

useShortcuts((e) => {
  const k = e.key.toLowerCase()
  const i = sel.value ? ids.value.indexOf(sel.value) : -1
  const x = sel.value ? tasksById.value.get(sel.value) : undefined
  const hero = current.value ?? heroTask.value

  if (k === 'j' || k === 'arrowdown') {
    sel.value = ids.value[Math.min(ids.value.length - 1, i + 1)] ?? null
  } else if (k === 'k' || k === 'arrowup') {
    sel.value = ids.value[Math.max(0, i - 1)] ?? null
  } else if (k === 'x' && x) {
    sel.value = ids.value[i + 1] ?? ids.value[i - 1] ?? null
    setDone(x, 'COMPLETED', run)
  } else if (k === 's' && (x || hero)) {
    void toggleTimer((x ?? hero)!)
  } else if (k === 'p' && (x || heroTask.value)) {
    void togglePlan((x ?? heroTask.value)!)
  } else if (k === 'escape' && sel.value) {
    sel.value = null
  } else {
    return
  }
  e.preventDefault()
})

const openToday = computed(() => [...overdue.value, ...todays.value])

// Vorhaben von gestern (dritte Journal-Frage) als Erinnerung mit Link auf den Eintrag.
const yesterday = ref<{ date: string; intention: string } | null>(null)
watch(
  () => today.value?.date,
  (day) => {
    if (!day) return
    const prev = shiftDate(day, -1)
    readNote(journalPath(prev)).then(
      (n) => {
        const intention = parseEntry(n.content).answers[2]?.trim()
        yesterday.value = intention ? { date: prev, intention } : null
      },
      () => (yesterday.value = null),
    )
  },
  { immediate: true },
)
</script>

<template>
  <div class="view-inner st">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="v-head st-head">
      <h1 class="v-title">{{ todayFormatted }}</h1>
      <p class="st-sub">
        <template v-if="overdue.length">
          <span class="st-sub-od">{{ $t('{n} überfällig', { n: overdue.length }) }}</span>
          <span class="st-sub-sep" aria-hidden="true">·</span>
        </template>
        <template v-if="eventCount">
          <span>{{ $tn(eventCount, '{n} Termin', '{n} Termine') }}</span>
          <span class="st-sub-sep" aria-hidden="true">·</span>
        </template>
        <span>{{ $t('{time} frei', { time: dur(freeMin) }) }}</span>
      </p>

      <RouterLink
        v-if="yesterday"
        class="st-journal"
        :to="{ name: 'journal', query: { date: yesterday.date } }"
      >
        <span>{{ $t('Gestern hast du dir vorgenommen:') }}</span>
        <b>{{ yesterday.intention }}</b>
      </RouterLink>
    </div>

    <div class="st-grid">
      <div class="st-main">
        <!-- Ausgelagerte Hero-Karte -->
        <TodayHeroCard
          :hero-task="heroTask"
          :overdue="overdue"
          :todays="todays"
          :upcoming="upcoming"
          :done-today="doneToday"
          :open-today="openToday"
          @set-done="setDone($event, 'COMPLETED', run)"
          @toggle-timer="toggleTimer"
          @toggle-plan="togglePlan"
          @skip-plan="planSkip = today?.date ?? ''"
        />

        <!-- Ausgelagerte Aufgaben-Tabelle -->
        <TodayTaskTable
          :tasks="tableTasks"
          :done-today="doneToday"
          :sel="sel"
          :overdue-tasks="overdue"
          :todays-tasks="todays"
          :project-names="projectNames"
          :projects-by-id="projectsById"
          :blocks="blocks"
          :today-date="today?.date"
          @select="sel = $event"
          @set-done="setDone($event, 'COMPLETED', run)"
          @toggle-timer="toggleTimer"
          @toggle-plan="togglePlan"
          @reopen="run(() => updateTask($event.id, { status: 'PLANNED' }))"
        />

        <div class="st-keys" aria-hidden="true">
          <span><kbd>J</kbd><kbd>K</kbd>{{ $t('bewegen') }}</span>
          <span><kbd>X</kbd>{{ $t('erledigt') }}</span>
          <span><kbd>S</kbd>{{ $t('starten') }}</span>
          <span><kbd>P</kbd>{{ $t('einplanen') }}</span>
        </div>
      </div>

      <!-- Rechte Spalte: Kompakte Tagesübersicht (Termine, Gewohnheiten, Fokus-Ziel, Lernkarte) -->
      <TodayQuickStrip
        :habits="habits"
        :cards="cards"
        :goal="goal"
        :events="events"
        :all-day="allDay"
        :now-min="nowMin"
        @toggle-habit="handleToggleHabit"
        @grade-card="handleGradeCard"
      />
    </div>
  </div>
</template>

<style scoped>
.st {
  max-width: 1360px;
  padding-top: 36px;
}
.st-head {
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  margin-bottom: 24px;
}
.st-sub {
  margin: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.st-sub-od {
  color: var(--a-red);
}
.st-sub-sep {
  color: var(--br-strong);
}
.st-journal {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-top: 6px;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
  text-decoration: none;
}
.st-journal b {
  min-width: 0;
  font-weight: 500;
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-journal:hover b {
  text-decoration: underline;
}
.st-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 44px;
  align-items: start;
}
.st-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
kbd {
  display: inline-grid;
  place-items: center;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  box-sizing: border-box;
  border-radius: 4px;
  background: color-mix(in oklab, currentColor 10%, transparent);
  font: 500 10.5px/1 var(--font-ui);
}
.st-keys {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 28px;
  padding-top: 12px;
  border-top: 1px solid var(--br-subtle);
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
  opacity: 0.65;
}
.st-keys span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 960px) {
  .st-grid {
    grid-template-columns: 1fr;
    gap: 32px;
  }
}
</style>
