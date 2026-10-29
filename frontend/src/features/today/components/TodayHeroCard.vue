<script setup lang="ts">
import { computed, ref } from 'vue'
import { updateTask, type Task } from '@/api/client'
import { useHome } from '@/composables/useHome'
import { findSlot, stopwatch } from '@/lib/dayPlan'
import { dur, hm, shiftDate } from '@/lib/dates'
import { locale, t, tn } from '@/lib/i18n'
import { prefs } from '@/lib/prefs'
import { store } from '@/lib/storage'

const props = defineProps<{
  heroTask: Task | null
  overdue: Task[]
  todays: Task[]
  upcoming: Task[]
  doneToday: Task[]
  openToday: Task[]
}>()

const emit = defineEmits<{
  (e: 'setDone', task: Task): void
  (e: 'toggleTimer', task: Task): void
  (e: 'togglePlan', task: Task): void
  (e: 'skipPlan'): void
}>()

const home = useHome()
const {
  today,
  tasksById,
  projectNames,
  nowMin,
  blocks,
  freeGaps,
  runEntry,
  runningTaskId,
  pausedTask,
  runSecs,
  run,
  offer,
  scheduleAt,
} = home

const current = computed(
  () =>
    (runningTaskId.value ? tasksById.value.get(runningTaskId.value) : undefined) ??
    pausedTask.value,
)

const planSkip = ref(store.get('kairo-plan-skip'))
const candidates = computed(() =>
  [...props.overdue, ...props.todays, ...props.upcoming]
    .filter((x) => !x.planned_start_at)
    .slice(0, 4),
)

const estimate = (x: Task) => x.estimated_minutes || 30
const estimateLabel = (x: Task) => (x.estimated_minutes ? dur(x.estimated_minutes) : '')
const startOf = (x: Task) => blocks.value.find((b) => b.task?.id === x.id)?.start
const fitGap = (x: Task) => freeGaps.value.find((g) => g.end - g.start >= estimate(x))
const dayOf = (x: Task) => x.planned_date || x.due_at?.slice(0, 10) || null

function overdueLabel(x: Task): string {
  const d = dayOf(x)
  if (!d) return t('Überfällig')
  const wd = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
    .format(new Date(`${d}T12:00:00`))
    .replace(/\.$/, '')
  return t('überfällig seit {day}', { day: wd })
}

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
  return props.heroTask ? 'task' : 'done'
})

const heroFit = computed(() => {
  const x = props.heroTask
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

const tracked = computed(() => today.value?.tracked_minutes ?? 0)
const freeMin = computed(() => freeGaps.value.reduce((s, g) => s + g.end - g.start, 0))
const pull = computed(() => props.upcoming[0])
const pullForward = (x: Task) => run(() => updateTask(x.id, { planned_date: today.value!.date }))

const reviewText = computed(() => {
  const [first, ...rest] = props.openToday
  if (!first) return t('Alles abgehakt. Zwei Sätze ins Journal, dann ist der Tag rund.')
  return rest.length
    ? t('„{title}“ ist offen geblieben, dazu {n} weitere.', { title: first.title, n: rest.length })
    : t('„{title}“ ist offen geblieben.', { title: first.title })
})

function moveOpen() {
  const moved = props.openToday.map(({ id, planned_date, planned_start_at }) => ({
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
    busy.push({ start: slot.start, end: slot.start + estimate(x) + 15 })
  }
  plan.forEach((p) => scheduleAt(p.task, p.at))
  picks.value = new Set()
}

function skipPlan() {
  planSkip.value = today.value?.date ?? ''
  store.set('kairo-plan-skip', planSkip.value)
  emit('skipPlan')
}

defineExpose({
  kind,
  current,
})
</script>

<template>
  <section class="st-hero" :aria-label="$t('Jetzt')">
    <template v-if="kind === 'run'">
      <div class="st-eyebrow">
        <span class="st-dot" :class="{ off: !runEntry }"></span>
        <b>{{ runEntry ? $t('Läuft') : $t('Pausiert') }}</b>
        <template v-if="current?.project_id">
          <span>·</span>
          <span>{{ projectNames.get(current.project_id) }}</span>
        </template>
      </div>
      <div class="st-title">{{ runTitle }}</div>
      <div class="st-clock">{{ stopwatch(runSecs) }}</div>
      <template v-if="current">
        <div class="st-prog-wrap">
          <div class="st-prog" aria-hidden="true">
            <i :style="{ width: runPct + '%' }"></i>
          </div>
          <div class="st-progt">
            <span>{{
              $t('{n} von {m} min', { n: Math.floor(runSecs / 60), m: estimate(current) })
            }}</span>
            <span>{{ runRest }}</span>
          </div>
        </div>
        <div class="st-acts">
          <button type="button" class="btn btn-primary" @click="emit('setDone', current!)">
            {{ $t('Fertig') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="emit('toggleTimer', current!)">
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
          <span v-if="x.project_id" class="st-pick-proj">
            {{ projectNames.get(x.project_id) }}
          </span>
          <span class="st-est">{{ estimateLabel(x) }}</span>
        </button>
      </div>
      <div class="st-acts">
        <button type="button" class="btn btn-primary" :disabled="!picks.size" @click="planDay">
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
        <RouterLink :to="{ name: 'journal', query: { date: today?.date } }" class="btn btn-primary">
          {{ $t('Journal schreiben') }}
        </RouterLink>
        <button v-if="openToday.length" type="button" class="btn btn-secondary" @click="moveOpen">
          {{ $t('Offenes auf morgen') }}
        </button>
      </div>
    </template>

    <template v-else-if="kind === 'task' && heroTask">
      <div class="st-eyebrow">
        <b>{{ $t('Als Nächstes') }}</b>
        <template v-if="heroTask.project_id">
          <span>·</span>
          <span>{{ projectNames.get(heroTask.project_id) }}</span>
        </template>
        <template v-if="estimateLabel(heroTask)">
          <span>·</span>
          <span>{{ estimateLabel(heroTask) }}</span>
        </template>
        <span>·</span>
        <span :class="{ 'st-od': overdue.includes(heroTask) }">
          {{ overdue.includes(heroTask) ? overdueLabel(heroTask) : $t('Heute fällig') }}
        </span>
      </div>
      <div class="st-title">{{ heroTask.title }}</div>
      <p class="st-text">{{ heroFit }}</p>
      <div class="st-acts">
        <button type="button" class="btn btn-primary" @click="emit('toggleTimer', heroTask)">
          <svg class="ic" viewBox="0 0 12 12" aria-hidden="true">
            <path d="M3 1.8v8.4L10 6z" fill="currentColor" />
          </svg>
          {{ $t('Starten') }}<kbd aria-hidden="true">S</kbd>
        </button>
        <button type="button" class="btn btn-secondary" @click="emit('togglePlan', heroTask)">
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
</template>

<style scoped>
kbd {
  display: inline-grid;
  place-items: center;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  box-sizing: border-box;
  border-radius: 5px;
  background: color-mix(in oklab, currentColor 12%, transparent);
  font: 500 11px/1 var(--font-ui);
}
.btn kbd {
  margin-left: 4px;
  opacity: 0.8;
}
.st-hero {
  position: relative;
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
  font: 500 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.st-eyebrow b {
  font-weight: 500;
  color: var(--tx-secondary);
}
.st-od {
  color: var(--a-red);
}
.st-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--a-run);
}
.st-dot.off {
  background: var(--tx-muted);
}
.st-title {
  margin: 12px 0 0;
  font: 600 24px/1.2 var(--font-ui);
  letter-spacing: -0.02em;
  color: var(--tx-primary);
}
.st-clock {
  margin: 12px 0 0;
  font: 600 40px/1 var(--font-ui);
  letter-spacing: -0.025em;
  color: var(--tx-primary);
  font-variant-numeric: tabular-nums;
}
.st-prog-wrap {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.st-prog {
  height: 4px;
  border-radius: 9999px;
  background: var(--bg-2);
  overflow: hidden;
}
.st-prog i {
  display: block;
  height: 100%;
  border-radius: 9999px;
  background: var(--tx-secondary);
  transition: width 0.3s ease;
}
.st-progt {
  display: flex;
  justify-content: space-between;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
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
.st-pick-proj {
  font-size: 12px;
  color: var(--tx-muted);
}
.st-est {
  color: var(--tx-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
</style>
