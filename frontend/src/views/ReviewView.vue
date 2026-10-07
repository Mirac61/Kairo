<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getReview, isOpen, listHabits, listProjects, listTasks, updateTask, type Habit, type Review, type Task } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { addDays, daysAgo, hm, weekStart, ymd } from '@/lib/dates'
import { habitColor, projectColor } from '@/lib/projectColor'

// Die API liefert completed_at, Task kennt es noch nicht.
type DoneTask = Task & { completed_at: string | null }

const WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So']
const short = new Intl.DateTimeFormat('de-DE', { day: 'numeric', month: 'short' })
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
const range = computed(() => `${short.format(monday.value)} – ${short.format(addDays(monday.value, 6))} ${addDays(monday.value, 6).getFullYear()}`)

const { error, load, run } = useLoader(async () => {
  const f = from.value
  const [rv, ts, hs] = await Promise.all([getReview(f, to.value), listTasks(), listHabits(), listProjects()]) // Projekte für die Farben
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
  calendar + tasks.value.filter((t) => t.planned_date === day && t.status !== 'CANCELLED').reduce((s, t) => s + t.estimated_minutes, 0)
const days = computed(() => (review.value?.days ?? []).map((d, i) => ({ ...d, i, plan: planOf(d.date, d.calendar_minutes), future: d.date > today() })))
const maxMin = computed(() => Math.max(240, ...days.value.flatMap((d) => [d.tracked_minutes, d.plan])))
const pct = (m: number) => `${m ? Math.max(2, (m / maxMin.value) * 100) : 0}%`
const calendarMinutes = computed(() => review.value?.days.reduce((s, d) => s + d.calendar_minutes, 0) ?? 0)
const planToDate = computed(() => days.value.filter((d) => !d.future).reduce((s, d) => s + d.plan, 0))
const trackedPct = computed(() => (planToDate.value ? Math.round(((review.value?.tracked_minutes ?? 0) / planToDate.value) * 100) : 0))
const weekTasks = computed(() => tasks.value.filter((t) => t.planned_date && t.planned_date >= from.value && t.planned_date <= to.value && t.status !== 'CANCELLED'))
const dayCaption = (d: (typeof days.value)[number]) => (d.future ? (d.plan ? `${hm(d.plan)} geplant` : 'frei') : `${hm(d.tracked_minutes)} / ${hm(d.plan)}`)
const dayText = (d: (typeof days.value)[number]) =>
  `${WD[d.i]} ${d.date.slice(8)}.${d.date.slice(5, 7)}.: erfasst ${hm(d.tracked_minutes)} h, geplant ${hm(d.plan)} h, ${d.completed_tasks} erledigt`

const projects = computed(() => (review.value?.projects ?? []).filter((p) => p.tracked_minutes || p.completed_tasks))
const maxProject = computed(() => Math.max(1, ...projects.value.map((p) => p.tracked_minutes)))
const projectTotal = computed(() => projects.value.reduce((s, p) => s + p.tracked_minutes, 0) || 1)
const projectName = (id: string | null) => review.value?.projects.find((p) => p.project_id === id)?.name

const completed = computed(() => tasks.value
  .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) >= from.value && ymd(new Date(t.completed_at)) <= to.value)
  .sort((a, b) => a.completed_at!.localeCompare(b.completed_at!)))
const doneMeta = (t: DoneTask) =>
  [projectName(t.project_id), WD[(new Date(t.completed_at!).getDay() + 6) % 7]].filter(Boolean).join(' · ')

// Für einen Tag dieser Woche geplant, noch offen und der Tag liegt vor heute (wie Überfällig in /api/today).
const overdue = computed(() => tasks.value
  .filter((t) => t.planned_date && t.planned_date >= from.value && t.planned_date <= to.value && t.planned_date < today()
    && isOpen(t))
  .sort((a, b) => a.planned_date!.localeCompare(b.planned_date!)))

const plan = (t: Task) => run(() => updateTask(t.id, { planned_date: today() }))

// Wochenziel pro Rhythmus; 0 = unbekannt.
function target(id: string) {
  const h = habits.value.find((x) => x.id === id)
  if (!h) return 0
  if (h.frequency_type === 'DAILY') return 7
  if (h.frequency_type === 'WEEKLY') return 1
  if (h.frequency_type === 'SPECIFIC_WEEKDAYS') return h.frequency_config.weekdays?.length ?? 0
  return h.frequency_config.times ?? 0
}

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">Woche</h1>
        <div class="v-sub">Rückblick · {{ range }} · KW {{ kw }}</div>
      </div>
      <div class="rv-nav">
        <button class="icon-btn" type="button" aria-label="Vorherige Woche" @click="go(addDays(monday, -7))"><svg class="ic"><use href="#i-left" /></svg></button>
        <button class="btn btn-ghost" type="button" :disabled="isCurrent" @click="go(weekStart(new Date()))">Diese Woche</button>
        <button class="icon-btn" type="button" aria-label="Nächste Woche" :disabled="isCurrent" @click="go(addDays(monday, 7))"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="review">
      <div class="rv-stats">
        <div><span class="lbl">Erfasst</span><span class="rv-big mono">{{ hm(review.tracked_minutes) }}</span><span v-if="planToDate" class="rv-note">{{ trackedPct }} % von {{ hm(planToDate) }} bis heute geplant</span></div>
        <div><span class="lbl">Termine</span><span class="rv-big mono">{{ hm(calendarMinutes) }}</span><span class="rv-note">Kalenderzeit</span></div>
        <div><span class="lbl">Erledigt</span><span class="rv-big mono">{{ review.completed_tasks }}<small v-if="weekTasks.length"> / {{ weekTasks.length }}</small></span><span class="rv-note">Aufgaben dieser Woche</span></div>
        <div>
          <span class="lbl">Überfällig geworden</span><span class="rv-big mono" :class="{ od: overdue.length }">{{ overdue.length }}</span>
          <span v-if="overdue.length" class="rv-note">{{ overdue[0]!.title }}<template v-if="overdue.length > 1"> + {{ overdue.length - 1 }}</template></span>
        </div>
      </div>

      <div class="card rv-chart">
        <div class="rv-legend"><h2 class="col-title">Plan und Ist pro Tag</h2><span class="spacer"></span><span><i class="k-plan"></i>Geplant</span><span><i class="k-trk"></i>Erfasst</span><span><i class="k-soon"></i>Kommt noch</span></div>
        <div class="rv-days" role="img" :aria-label="days.map(dayText).join('; ')">
          <div v-for="d in days" :key="d.date" class="rv-day" :class="{ now: d.date === today() }" :title="dayText(d)">
            <div class="rv-bars">
              <i :class="d.future ? 'k-soon' : 'k-plan'" :style="{ height: pct(d.plan) }"></i>
              <i v-if="!d.future" class="k-trk" :style="{ height: pct(d.tracked_minutes) }"></i>
            </div>
            <span class="lbl">{{ WD[d.i] }} <b>{{ Number(d.date.slice(8)) }}</b></span>
            <span class="rv-n mono">{{ dayCaption(d) }}</span>
          </div>
        </div>
      </div>

      <div class="start-grid">
        <div class="start-col">
          <div class="col-head"><h2 class="col-title">Zeit pro Projekt</h2></div>
          <div class="tasklist">
            <div v-for="p in projects" :key="p.project_id ?? ''" class="rv-proj">
              <span class="rv-name"><span class="pdot" :style="{ background: projectColor(p.project_id) }"></span>{{ p.name }}</span>
              <div class="dayload" aria-hidden="true"><span :style="{ width: `${(p.tracked_minutes / maxProject) * 100}%`, background: projectColor(p.project_id) }"></span></div>
              <span class="rv-n mono">{{ hm(p.tracked_minutes) }}</span>
              <span class="rv-n mono rv-pct">{{ Math.round((p.tracked_minutes / projectTotal) * 100) }} %</span>
            </div>
            <div v-if="!projects.length" class="v-sub rv-empty">Keine Zeit erfasst.</div>
          </div>

          <div class="col-head rv-gap"><h2 class="col-title">Gewohnheiten</h2></div>
          <div class="tasklist">
            <div v-for="h in review.habits" :key="h.habit_id" class="task-row">
              <span class="pdot" :style="{ background: habitColor(h.habit_id) }"></span>
              <span class="t">{{ h.name }}</span>
              <span class="rv-n mono">{{ h.done }}<template v-if="target(h.habit_id)">/{{ target(h.habit_id) }}</template></span>
              <span class="rv-n">{{ h.streak }} Tage Serie</span>
            </div>
            <div v-if="!review.habits.length" class="v-sub rv-empty">Keine aktiven Gewohnheiten.</div>
          </div>
        </div>

        <div class="start-col">
          <div class="col-head"><h2 class="col-title">Erledigt</h2></div>
          <div class="tasklist">
            <div v-for="t in completed" :key="t.id" class="task-row">
              <span class="rv-ck" :style="{ background: projectColor(t.project_id) }" aria-hidden="true"><svg viewBox="0 0 10 10"><path d="M2 5.2l2 2 4-4.4" /></svg></span>
              <span class="t">{{ t.title }}</span>
              <span class="due">{{ doneMeta(t) }}</span>
            </div>
            <div v-if="!completed.length" class="v-sub rv-empty">Nichts erledigt.</div>
          </div>

          <div class="col-head rv-gap"><h2 class="col-title">Überfällig geworden</h2></div>
          <div class="tasklist">
            <div v-for="t in overdue" :key="t.id" class="task-row rv-od">
              <span class="t">{{ t.title }}</span>
              <span class="due od">{{ projectName(t.project_id) ? `${projectName(t.project_id)} · ` : '' }}{{ daysAgo(t.planned_date!, today()) }}</span>
              <button type="button" class="btn btn-ghost" :aria-label="`Einplanen: ${t.title}`" @click="plan(t)">Einplanen</button>
            </div>
            <div v-if="!overdue.length" class="v-sub rv-empty">Nichts überfällig.</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.rv-nav { display: flex; align-items: center; gap: 4px; }
.rv-nav :disabled { opacity: .4; cursor: default; }
.rv-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin-bottom: 28px; border-block: 1px solid var(--br-subtle); }
.rv-stats > div { display: grid; gap: 4px; padding: 14px 18px; align-content: start; border-left: 1px solid var(--br-subtle); }
.rv-stats > div:first-child { border-left: 0; padding-left: 0; }
.rv-big { font: 500 28px/1.1 var(--font-mono); color: var(--tx-primary); }
.rv-big.od { color: var(--a-red); }
.rv-note { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rv-day { padding: 8px 0; border-radius: var(--r-s); }
.rv-day.now { background: var(--bg-2); }
.rv-day .lbl b { color: var(--tx-primary); font-weight: 600; }
.pdot { display: inline-block; flex: none; width: 8px; height: 8px; border-radius: 50%; }
.rv-od { border-radius: var(--r-s); background: color-mix(in srgb, var(--a-red) 7%, transparent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--a-red) 35%, transparent); }
.rv-chart { padding: 18px 20px; margin-bottom: 32px; }
.rv-legend { display: flex; align-items: center; gap: 16px; margin-bottom: 14px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.rv-legend span { display: inline-flex; align-items: center; gap: 6px; }
.rv-legend i { width: 9px; height: 9px; border-radius: 3px; box-sizing: border-box; }
.rv-legend .col-title { margin: 0; color: var(--tx-primary); }
.spacer { flex: 1; }
/* Neutral: gefüllt heißt erfasst, schraffiert heißt Termine */
.k-plan { background: var(--br-strong); }
.k-trk { background: var(--a-blue-hi); }
.k-soon { background: transparent; border: 1px dashed var(--br-strong); }
.rv-days { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 8px; }
.rv-day { display: grid; justify-items: center; gap: 6px; }
.rv-bars { display: flex; align-items: flex-end; justify-content: center; gap: 4px; height: 120px; width: 100%; }
.rv-bars i { width: min(22px, 40%); border-radius: 4px 4px 0 0; box-sizing: border-box; }
.rv-bars .k-soon { border-bottom: 0; }
.rv-big small { font-size: 15px; font-weight: 500; color: var(--tx-muted); }
.rv-ck { display: grid; place-items: center; flex: none; width: 16px; height: 16px; border-radius: 50%; }
.rv-ck svg { width: 9px; height: 9px; fill: none; stroke: var(--bg-0); stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.rv-od .btn { color: var(--a-blue-tx); }
.rv-od .btn:hover { background: transparent; text-decoration: underline; }
.rv-n { font: 400 12px/1.2 var(--font-mono); color: var(--tx-secondary); white-space: nowrap; }
.rv-proj { display: grid; grid-template-columns: 120px minmax(0, 1fr) 44px 40px; align-items: center; gap: 12px; padding: 8px 0; }
.rv-pct { color: var(--tx-muted); text-align: right; }
.rv-name { display: flex; align-items: center; gap: 8px; min-width: 0; font: 500 14px/1.4 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rv-gap { margin-top: 28px; }
.rv-empty { padding: 12px 16px; margin: 0; }
</style>
