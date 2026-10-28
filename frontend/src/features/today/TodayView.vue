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
import { findSlot, stopwatch } from '@/lib/dayPlan'
import { dur, hm, shiftDate } from '@/lib/dates'
import { locale, t, tn } from '@/lib/i18n'
import { journalPath, parseEntry } from '@/lib/journal'
import { prefs } from '@/lib/prefs'
import { projectColor } from '@/lib/projectColor'
import { store } from '@/lib/storage'
import TodayDayPlanRail from './components/TodayDayPlanRail.vue'
import TodayQuickStrip from './components/TodayQuickStrip.vue'

// Startseite: oben eine Karte mit dem, was jetzt dran ist, darunter Gewohnheiten, Lernkarte und Ziel,
// dann die Aufgaben nach Überfällig/Heute/Demnächst; rechts der Zeitplan mit freien Lücken zum Einplanen.
const home = provideHome()
const {
  today,
  tasks,
  tasksById,
  projectNames,
  nowMin,
  error,
  blocks,
  allDay,
  freeGaps,
  runEntry,
  runningTaskId,
  pausedTask,
  runSecs,
  run,
  offer,
  setDone,
  scheduleAt,
} = home

const events = computed(() => blocks.value.filter((b) => b.kind === 'event'))

const upcomingEvent = computed(() => {
  const cur = events.value.find((b) => b.start <= nowMin.value && nowMin.value < b.end)
  const next = events.value.find((b) => b.start >= nowMin.value)
  if (cur) return t('Aktuell: {title} bis {time} Uhr', { title: cur.title, time: hm(cur.end) })
  if (next)
    return t('Als Nächstes im Kalender: {title} um {time} Uhr', {
      title: next.title,
      time: hm(next.start),
    })
  if (allDay.value.length) return allDay.value.map((b) => b.title).join(', ')
  return null
})

const todayFormatted = computed(() => {
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(new Date())
})

const freeMin = computed(() => freeGaps.value.reduce((s, g) => s + g.end - g.start, 0))
const estimate = (x: Task) => x.estimated_minutes || 30
const estimateLabel = (x: Task) => (x.estimated_minutes ? dur(x.estimated_minutes) : '')
const startOf = (x: Task) => blocks.value.find((b) => b.task?.id === x.id)?.start

// Aufgaben: Überfälliges, heute Offenes, Demnächst (nach Datum). Erledigtes von heute klappt unten auf.
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

const shortDay = (d: string | null) => {
  return d
    ? new Intl.DateTimeFormat(locale.value, {
        weekday: 'short',
        day: 'numeric',
        month: 'numeric',
      }).format(new Date(`${d}T12:00:00`))
    : ''
}

// Karte oben: laufende/pausierte Aufgabe, morgens ohne Plan die Auswahl, ab 18 Uhr der Rückblick,
// sonst die nächste offene Aufgabe von heute, und wenn nichts mehr offen ist „Alles erledigt“.
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

const fitGap = (x: Task) => freeGaps.value.find((g) => g.end - g.start >= estimate(x))

const heroFit = computed(() => {
  const x = heroTask.value
  if (!x) return ''
  const at = startOf(x)
  if (at !== undefined) return t('Eingeplant um {time} Uhr.', { time: hm(at) })
  const g = fitGap(x)
  if (!g) return t('Heute ist keine Lücke groß genug.')
  return g.start <= nowMin.value + 5
    ? t('Passt jetzt in deine freie Zeit.')
    : t('Passt in deine freie Lücke um {time} Uhr.', { time: hm(g.start) })
})

const runTitle = computed(
  () =>
    current.value?.title ??
    projectNames.value.get(runEntry.value?.project_id ?? '') ??
    t('Projektzeit'),
)
const runEst = computed(() => (current.value ? estimate(current.value) * 60 : 0))
const runPct = computed(() =>
  runEst.value ? Math.min(100, (runSecs.value / runEst.value) * 100) : 0,
)
const runRest = computed(() => {
  const over = runSecs.value - runEst.value
  return over > 0
    ? t('{n} min über der Schätzung', { n: Math.floor(over / 60) })
    : t('noch {n} min', { n: Math.ceil(-over / 60) })
})

const toggleTimer = (x: Task) =>
  run(() => taskAction(x.id, runningTaskId.value === x.id ? 'pause' : 'start'))

const tracked = computed(() => today.value?.tracked_minutes ?? 0)
const pull = computed(() => upcoming.value[0])
const pullForward = (x: Task) => run(() => updateTask(x.id, { planned_date: today.value!.date }))
const openToday = computed(() => [...overdue.value, ...todays.value])

const reviewText = computed(() => {
  const [first, ...rest] = openToday.value
  if (!first) return t('Alles abgehakt. Zwei Sätze ins Journal, dann ist der Tag rund.')
  return rest.length
    ? t('„{title}“ ist offen geblieben, dazu {n} weitere.', { title: first.title, n: rest.length })
    : t('„{title}“ ist offen geblieben.', { title: first.title })
})

function moveOpen() {
  const moved = openToday.value.map(({ id, planned_date, planned_start_at }) => ({
    id,
    planned_date,
    planned_start_at,
  }))
  const tomorrow = shiftDate(today.value!.date, 1)
  void run(async () => {
    await Promise.all(
      moved.map((x) => updateTask(x.id, { planned_date: tomorrow, planned_start_at: '' })),
    )
    offer(tn(moved.length, '{n} Aufgabe auf morgen gelegt', '{n} Aufgaben auf morgen gelegt'), () =>
      run(() =>
        Promise.all(
          moved.map((x) =>
            updateTask(x.id, {
              planned_date: x.planned_date ?? '',
              planned_start_at: x.planned_start_at ?? '',
            }),
          ),
        ),
      ),
    )
  })
}

// Morgens ohne Plan: bis zu drei Aufgaben wählen, sie landen nacheinander in den ersten passenden Lücken.
const picks = ref(new Set<string>())
function togglePick(id: string) {
  const s = new Set(picks.value)
  if (s.has(id)) s.delete(id)
  else if (s.size < 3) s.add(id)
  picks.value = s
}

function planDay() {
  const busy = blocks.value.filter((b) => !b.done).map((b) => ({ start: b.start, end: b.end }))
  const from = Math.max(nowMin.value, prefs.dayStart * 60)
  const plan: { task: Task; at: number }[] = []
  for (const x of candidates.value.filter((c) => picks.value.has(c.id))) {
    const slot = findSlot(
      busy.sort((a, b) => a.start - b.start),
      from,
      prefs.dayStart * 60,
      prefs.dayEnd * 60,
      estimate(x),
    )
    if (!slot) continue
    plan.push({ task: x, at: slot.start })
    busy.push({ start: slot.start, end: slot.start + estimate(x) + 15 }) // 15 Minuten Luft zwischen den Blöcken
  }
  plan.forEach((p) => scheduleAt(p.task, p.at))
  picks.value = new Set()
}

function skipPlan() {
  planSkip.value = today.value?.date ?? ''
  store.set('kairo-plan-skip', planSkip.value)
}

// Einplanen/Ausplanen: ohne Uhrzeit in die erste passende Lücke, mit Uhrzeit wieder heraus.
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

// Liste mit Tastatur: J/K bewegen, X abhaken, S starten, P einplanen.
const heroId = computed(() =>
  kind.value === 'task' ? heroTask.value?.id : kind.value === 'run' ? current.value?.id : undefined,
)
const groups = computed(() =>
  [
    { key: 'over', label: t('Überfällig'), rows: overdue.value },
    { key: 'today', label: t('Heute'), rows: todays.value },
    { key: 'soon', label: t('Demnächst'), rows: upcoming.value },
  ]
    .map((g) => ({
      ...g,
      rows: g.rows
        .filter((x) => x.id !== heroId.value)
        .sort((a, b) => Number(Boolean(a.planned_start_at)) - Number(Boolean(b.planned_start_at))),
    }))
    .filter((g) => g.rows.length),
)

const ids = computed(() => groups.value.flatMap((g) => g.rows.map((x) => x.id)))
const sel = ref<string | null>(null)
const showDone = ref(false)
const dueLabel = (key: string, x: Task) =>
  x.planned_start_at ? '' : key === 'today' ? '' : shortDay(dayOf(x))

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

const selectedTask = computed(() => (sel.value ? tasksById.value.get(sel.value) : undefined))
const allOpenAndUpcoming = computed(() => openToday.value.concat(upcoming.value))
</script>

<template>
  <div class="view-inner st">
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="st-grid">
      <div class="st-main">
        <div class="v-head">
          <h1 class="v-title">{{ todayFormatted }}</h1>
        </div>

        <div v-if="upcomingEvent" class="st-event-alert">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-cal" /></svg>
          <span>{{ upcomingEvent }}</span>
        </div>

        <RouterLink
          v-if="yesterday"
          class="st-journal"
          :to="{ name: 'journal', query: { date: yesterday.date } }"
        >
          <span>{{ $t('Gestern hast du dir vorgenommen:') }}</span>
          <b>{{ yesterday.intention }}</b>
        </RouterLink>

        <!-- Hero Karte (Jetzt, Planen, Rückblick, Nächste Aufgabe) -->
        <section class="st-hero" :aria-label="$t('Jetzt')">
          <template v-if="kind === 'run'">
            <div class="st-eyebrow">
              <span class="st-dot" :class="{ off: !runEntry }"></span>
              <b>{{ runEntry ? $t('Läuft') : $t('Pausiert') }}</b>
              <template v-if="current?.project_id">
                <span>·</span>
                <span class="chip" :style="{ '--chip-c': projectColor(current.project_id) }">
                  <span class="cdot"></span>{{ projectNames.get(current.project_id) }}
                </span>
              </template>
              <span v-if="current" class="st-pill">
                {{ $t('Schätzung {time}', { time: estimateLabel(current) }) }}
              </span>
            </div>
            <div class="st-title">{{ runTitle }}</div>
            <div class="st-clock mono">{{ stopwatch(runSecs) }}</div>
            <template v-if="current">
              <div class="st-prog" aria-hidden="true">
                <i :style="{ width: runPct + '%' }"></i>
              </div>
              <div class="st-progt">
                <span>{{
                  $t('{n} von {m} min', { n: Math.floor(runSecs / 60), m: estimate(current) })
                }}</span>
                <span>{{ runRest }}</span>
              </div>
              <div class="st-acts">
                <button
                  type="button"
                  class="btn btn-primary"
                  @click="setDone(current!, 'COMPLETED', run)"
                >
                  {{ $t('Fertig') }}
                </button>
                <button type="button" class="btn btn-secondary" @click="toggleTimer(current!)">
                  {{ runEntry ? $t('Pause') : $t('Fortsetzen') }}<kbd aria-hidden="true">S</kbd>
                </button>
              </div>
            </template>
          </template>

          <template v-else-if="kind === 'plan'">
            <div class="st-eyebrow">
              <b>{{ $t('Noch kein Plan') }}</b>
            </div>
            <div class="st-title">{{ $t('Was soll heute passieren?') }}</div>
            <p class="st-text">
              {{ $t('Wähle bis zu drei Aufgaben, Kairo legt sie in die Lücken.') }}
            </p>
            <div class="st-picks">
              <button
                v-for="x in candidates"
                :key="x.id"
                type="button"
                class="st-pick"
                role="checkbox"
                :aria-checked="picks.has(x.id)"
                @click="togglePick(x.id)"
              >
                <span class="cb" :aria-checked="picks.has(x.id)"></span>
                <span class="st-pick-t">{{ x.title }}</span>
                <span
                  v-if="x.project_id"
                  class="chip"
                  :style="{ '--chip-c': projectColor(x.project_id) }"
                >
                  <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
                </span>
                <span class="st-est">{{ estimateLabel(x) }}</span>
              </button>
            </div>
            <div class="st-acts">
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!picks.size"
                @click="planDay"
              >
                {{ picks.size ? $t('Tag planen · {n}', { n: picks.size }) : $t('Tag planen') }}
              </button>
              <button type="button" class="btn btn-secondary" @click="skipPlan">
                {{ $t('Ohne Plan beginnen') }}
              </button>
            </div>
          </template>

          <template v-else-if="kind === 'review'">
            <div class="st-eyebrow">
              <b>{{ $t('Tagesrückblick · {time}', { time: hm(nowMin) }) }}</b>
            </div>
            <div class="st-title">
              {{ $t('{n} erledigt, {time} erfasst', { n: doneToday.length, time: dur(tracked) }) }}
            </div>
            <p class="st-text">{{ reviewText }}</p>
            <div class="st-acts">
              <RouterLink
                :to="{ name: 'journal', query: { date: today?.date } }"
                class="btn btn-primary"
              >
                {{ $t('Journal schreiben') }}
              </RouterLink>
              <button
                v-if="openToday.length"
                type="button"
                class="btn btn-secondary"
                @click="moveOpen"
              >
                {{ $t('Offenes auf morgen') }}
              </button>
            </div>
          </template>

          <template v-else-if="kind === 'task' && heroTask">
            <div class="st-eyebrow">
              <b>{{ $t('Als Nächstes') }}</b>
              <template v-if="heroTask.project_id">
                <span>·</span>
                <span class="chip" :style="{ '--chip-c': projectColor(heroTask.project_id) }">
                  <span class="cdot"></span>{{ projectNames.get(heroTask.project_id) }}
                </span>
              </template>
              <span>·</span><span>{{ estimateLabel(heroTask) }}</span>
              <span class="st-pill">{{
                overdue.includes(heroTask) ? $t('Überfällig') : $t('Heute fällig')
              }}</span>
            </div>
            <div class="st-title">{{ heroTask.title }}</div>
            <p class="st-text">{{ heroFit }}</p>
            <div class="st-acts">
              <button type="button" class="btn btn-primary" @click="toggleTimer(heroTask)">
                <svg class="ic" viewBox="0 0 12 12" aria-hidden="true">
                  <path d="M3 1.8v8.4L10 6z" fill="currentColor" />
                </svg>
                {{ $t('Starten') }}<kbd aria-hidden="true">S</kbd>
              </button>
              <button type="button" class="btn btn-secondary" @click="togglePlan(heroTask)">
                {{
                  startOf(heroTask) !== undefined
                    ? $t('Geplant {time}', { time: hm(startOf(heroTask)!) })
                    : $t('In Lücke legen')
                }}
                <kbd aria-hidden="true">P</kbd>
              </button>
            </div>
          </template>

          <template v-else>
            <div class="st-eyebrow">
              <b>{{ $t('Heute') }}</b>
            </div>
            <div class="st-title">{{ $t('Alles erledigt für heute.') }}</div>
            <p class="st-text">
              {{
                $t('{n} abgehakt, {time} erfasst, noch {free} frei.', {
                  n: doneToday.length,
                  time: dur(tracked),
                  free: dur(freeMin),
                })
              }}
            </p>
            <div v-if="pull" class="st-acts">
              <button type="button" class="btn btn-secondary" @click="pullForward(pull)">
                {{ $t('„{title}“ vorziehen', { title: pull.title }) }}
              </button>
            </div>
          </template>
        </section>

        <!-- Aufgaben nach Gruppen -->
        <div class="st-groups">
          <div v-for="g in groups" :key="g.key" class="tgroup">
            <div class="lbl">
              <span>{{ g.label }}</span>
              <span class="count">{{ g.rows.length }}</span>
            </div>
            <div class="tasklist">
              <div
                v-for="x in g.rows"
                :key="x.id"
                class="st-row"
                :class="{ sel: sel === x.id }"
                @click="sel = x.id"
              >
                <button
                  type="button"
                  class="cb ring"
                  role="checkbox"
                  aria-checked="false"
                  :aria-label="$t('{title} erledigt', { title: x.title })"
                  @click.stop="setDone(x, 'COMPLETED', run)"
                ></button>
                <div class="st-row-t">
                  <span class="t">{{ x.title }}</span>
                  <span v-if="startOf(x) !== undefined" class="st-planned mono">
                    {{ hm(startOf(x)!) }}
                  </span>
                </div>
                <span class="pcol">
                  <span
                    v-if="x.project_id"
                    class="chip"
                    :style="{ '--chip-c': projectColor(x.project_id) }"
                  >
                    <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
                  </span>
                </span>
                <span class="dur">{{ x.estimated_minutes ? dur(x.estimated_minutes) : '' }}</span>
                <span class="due">{{ dueLabel(g.key, x) }}</span>
                <div class="st-row-acts">
                  <button type="button" class="btn btn-secondary" @click.stop="toggleTimer(x)">
                    <kbd aria-hidden="true">S</kbd>{{ $t('Starten') }}
                  </button>
                  <button type="button" class="btn btn-secondary" @click.stop="togglePlan(x)">
                    <kbd aria-hidden="true">P</kbd
                    >{{ x.planned_start_at ? $t('Aus dem Plan') : $t('Einplanen') }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div v-if="doneToday.length" class="tgroup">
            <button
              type="button"
              class="lbl st-done-btn"
              :aria-expanded="showDone"
              @click="showDone = !showDone"
            >
              <svg class="ic" :class="{ open: showDone }" viewBox="0 0 12 12" aria-hidden="true">
                <path d="M4.5 3 7.5 6 4.5 9" />
              </svg>
              <span>{{ $t('Erledigt') }}</span>
              <span class="count">{{ doneToday.length }}</span>
            </button>
            <div v-if="showDone" class="tasklist">
              <div v-for="x in doneToday" :key="x.id" class="st-row done">
                <button
                  type="button"
                  class="cb ring"
                  role="checkbox"
                  aria-checked="true"
                  :aria-label="$t('Wieder öffnen')"
                  :title="$t('Wieder öffnen')"
                  @click="run(() => updateTask(x.id, { status: 'PLANNED' }))"
                ></button>
                <div class="st-row-t">
                  <span class="t">{{ x.title }}</span>
                </div>
                <span class="pcol">
                  <span
                    v-if="x.project_id"
                    class="chip"
                    :style="{ '--chip-c': projectColor(x.project_id) }"
                  >
                    <span class="cdot"></span>{{ projectNames.get(x.project_id) }}
                  </span>
                </span>
                <span class="dur"></span>
                <span class="due"></span>
              </div>
            </div>
          </div>

          <div v-if="!groups.length && !doneToday.length" class="st-empty">
            {{ $t('Nichts für heute geplant.') }}
          </div>
        </div>

        <!-- Ausgelagerte Quick-Strip Komponente (Habits, Cards, Goal) -->
        <TodayQuickStrip
          :habits="habits"
          :cards="cards"
          :goal="goal"
          @toggle-habit="handleToggleHabit"
          @grade-card="handleGradeCard"
        />

        <div class="st-keys" aria-hidden="true">
          <span><kbd>J</kbd><kbd>K</kbd>{{ $t('bewegen') }}</span>
          <span><kbd>X</kbd>{{ $t('erledigt') }}</span>
          <span><kbd>S</kbd>{{ $t('starten') }}</span>
          <span><kbd>P</kbd>{{ $t('einplanen') }}</span>
        </div>
      </div>

      <!-- Ausgelagerte Tagesplan-Schiene -->
      <TodayDayPlanRail
        :blocks="blocks"
        :free-gaps="freeGaps"
        :now-min="nowMin"
        :selected-task="selectedTask"
        :all-tasks="allOpenAndUpcoming"
        :project-names="projectNames"
        :free-min="freeMin"
        @schedule-at="scheduleAt"
        @toggle-plan="togglePlan"
        @clear-selected-task="sel = null"
      />
    </div>
  </div>
</template>

<style scoped>
.st {
  max-width: 1240px;
  padding-top: 36px;
}
.st-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 310px;
  gap: 48px;
  align-items: start;
}
.st-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.st .v-sub {
  margin-top: 8px;
  font-variant-numeric: tabular-nums;
}
kbd {
  display: inline-grid;
  place-items: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  box-sizing: border-box;
  border-radius: 5px;
  background: color-mix(in oklab, currentColor 12%, transparent);
  font: 500 11px/1 var(--font-mono);
}
.st .btn kbd {
  margin-left: 4px;
  opacity: 0.8;
}
.st-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-subtle);
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.st-journal {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-top: 10px;
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
.st-event-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px 12px;
  border-radius: var(--r-s);
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.st-event-alert .ic {
  color: var(--a-blue);
  flex: none;
}
.st-hero {
  position: relative;
  margin-top: 20px;
  padding: 24px 28px;
  border-radius: var(--r-l);
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
}
.st-eyebrow {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font: 500 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.st-eyebrow b {
  color: var(--tx-secondary);
}
.st-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--a-green);
}
.st-dot.off {
  background: var(--tx-muted);
}
.st-title {
  margin: 10px 0 0;
  font: 600 24px/1.2 var(--font-ui);
  letter-spacing: -0.015em;
  color: var(--tx-primary);
}
.st-clock {
  margin: 12px 0 0;
  font-size: 40px;
  font-weight: 600;
  color: var(--tx-primary);
  line-height: 1;
}
.st-prog {
  height: 6px;
  margin: 16px 0 0;
  border-radius: 3px;
  background: var(--bg-2);
  overflow: hidden;
}
.st-prog i {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--a-green);
  transition: width 0.3s ease;
}
.st-progt {
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.st-text {
  margin: 8px 0 0;
  font: 400 14px/1.5 var(--font-ui);
  color: var(--tx-secondary);
}
.st-acts {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 20px;
}
.st-picks {
  display: grid;
  gap: 8px;
  margin: 14px 0 0;
}
.st-pick {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border: 0;
  border-radius: 8px;
  background: var(--bg-2);
  color: var(--tx-primary);
  font: 400 13px/1.4 var(--font-ui);
  cursor: pointer;
  text-align: left;
}
.st-pick:hover {
  background: var(--bg-hover);
}
.st-pick-t {
  flex: 1;
}
.st-est {
  color: var(--tx-muted);
  font-size: 12px;
}

.st-groups {
  margin-top: 32px;
  display: grid;
  gap: 24px;
}
.st-groups .lbl {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font: 600 11px/1 var(--font-ui);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.st-groups .tasklist {
  display: grid;
  gap: 2px;
}
.st-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  padding: 0 10px;
  border-radius: 8px;
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  cursor: pointer;
  transition: background var(--dur);
}
.st-row:hover {
  background: var(--bg-hover);
}
.st-row.sel {
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-default);
}
.st-row.done {
  opacity: 0.6;
}
.st-row.done .t {
  text-decoration: line-through;
}
.st-row-t {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.st-row-t .t {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-planned {
  font-size: 11px;
  color: var(--a-blue);
  background: color-mix(in srgb, var(--a-blue) 10%, transparent);
  padding: 1px 5px;
  border-radius: 4px;
}
.st-row .dur,
.st-row .due {
  flex: none;
  font-size: 12px;
  color: var(--tx-muted);
}
.st-row .dur {
  width: 60px;
  text-align: right;
}
.st-row .due {
  width: 90px;
  text-align: right;
}
.st-row-acts {
  display: none;
  gap: 6px;
  margin-left: 8px;
}
.st-row:hover .st-row-acts {
  display: flex;
}
.st-row-acts .btn {
  height: 24px;
  padding: 0 8px;
  font-size: 11px;
}
.st-done-btn {
  background: none;
  border: 0;
  cursor: pointer;
  padding: 0;
}
.st-done-btn .ic {
  width: 10px;
  height: 10px;
  transition: transform var(--dur);
}
.st-done-btn .ic.open {
  transform: rotate(90deg);
}

.st-empty {
  padding: 24px 0;
  text-align: center;
  color: var(--tx-muted);
  font: 400 13px/1.4 var(--font-ui);
}
.st-keys {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 32px;
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
  font: 400 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
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
