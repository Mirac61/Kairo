<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { errorMessage, getReview, listHabits, listTasks, type Habit, type Review, type Task } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { daysAgo, ymd } from '@/lib/dates'
import { projectColor } from '@/lib/projectColor'

// Die API liefert completed_at, Task kennt es noch nicht.
type DoneTask = Task & { completed_at: string | null }

const WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So']
const weekStart = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate() - ((d.getDay() + 6) % 7))
const addDays = (d: Date, n: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n)
const hm = (m: number) => `${Math.floor(m / 60)}:${String(m % 60).padStart(2, '0')}`
const short = new Intl.DateTimeFormat('de-DE', { day: 'numeric', month: 'short' })
const today = () => ymd(new Date())

const monday = ref(weekStart(new Date()))
const review = ref<Review | null>(null)
const tasks = ref<DoneTask[]>([])
const habits = ref<Habit[]>([])
const error = ref('')

const from = computed(() => ymd(monday.value))
const to = computed(() => ymd(addDays(monday.value, 6)))
const isCurrent = computed(() => from.value === ymd(weekStart(new Date())))
const range = computed(() => `${short.format(monday.value)} – ${short.format(addDays(monday.value, 6))} ${addDays(monday.value, 6).getFullYear()}`)

async function load() {
  const f = from.value
  try {
    const [rv, ts, hs] = await Promise.all([getReview(f, to.value), listTasks(), listHabits()])
    if (f !== from.value) return // inzwischen andere Woche gewählt
    review.value = rv
    tasks.value = ts as DoneTask[]
    habits.value = hs
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function go(m: Date) {
  monday.value = m
  void load()
}

const maxMin = computed(() => Math.max(60, ...(review.value?.days ?? []).flatMap((d) => [d.tracked_minutes, d.calendar_minutes])))
const pct = (m: number) => `${m ? Math.max(2, (m / maxMin.value) * 100) : 0}%`
const calendarMinutes = computed(() => review.value?.days.reduce((s, d) => s + d.calendar_minutes, 0) ?? 0)
const dayText = (d: Review['days'][number], i: number) =>
  `${WD[i]} ${d.date.slice(8)}.${d.date.slice(5, 7)}.: erfasst ${hm(d.tracked_minutes)} h, Termine ${hm(d.calendar_minutes)} h, ${d.completed_tasks} erledigt`

const projects = computed(() => (review.value?.projects ?? []).filter((p) => p.tracked_minutes || p.completed_tasks))
const maxProject = computed(() => Math.max(1, ...projects.value.map((p) => p.tracked_minutes)))
const projectName = (id: string | null) => review.value?.projects.find((p) => p.project_id === id)?.name

const completed = computed(() => tasks.value
  .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) >= from.value && ymd(new Date(t.completed_at)) <= to.value)
  .sort((a, b) => a.completed_at!.localeCompare(b.completed_at!)))
const doneMeta = (t: DoneTask) =>
  [projectName(t.project_id), WD[(new Date(t.completed_at!).getDay() + 6) % 7]].filter(Boolean).join(' · ')

// Für einen Tag dieser Woche geplant, noch offen und der Tag liegt vor heute (wie Überfällig in /api/today).
const overdue = computed(() => tasks.value
  .filter((t) => t.planned_date && t.planned_date >= from.value && t.planned_date <= to.value && t.planned_date < today()
    && t.status !== 'COMPLETED' && t.status !== 'CANCELLED')
  .sort((a, b) => a.planned_date!.localeCompare(b.planned_date!)))

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
        <div class="v-sub">Rückblick auf {{ range }}</div>
      </div>
      <div class="rv-nav">
        <button class="icon-btn" type="button" aria-label="Vorherige Woche" @click="go(addDays(monday, -7))"><svg class="ic"><use href="#i-left" /></svg></button>
        <button class="btn btn-ghost" type="button" :disabled="isCurrent" @click="go(weekStart(new Date()))">Diese Woche</button>
        <button class="icon-btn" type="button" aria-label="Nächste Woche" :disabled="isCurrent" @click="go(addDays(monday, 7))"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="review">
      <div class="focus-stats rv-stats">
        <div class="fstat"><span class="n">{{ hm(review.tracked_minutes) }}<span class="u">h</span></span><span class="c">Erfasst</span></div>
        <div class="fstat"><span class="n">{{ hm(calendarMinutes) }}<span class="u">h</span></span><span class="c">Termine</span></div>
        <div class="fstat"><span class="n">{{ review.completed_tasks }}</span><span class="c">Erledigt</span></div>
        <div class="fstat"><span class="n">{{ overdue.length }}</span><span class="c">Überfällig geworden</span></div>
      </div>

      <div class="card rv-chart">
        <div class="rv-legend"><span><i class="k-cal"></i>Termine</span><span><i class="k-trk"></i>Erfasst</span></div>
        <div class="rv-days" role="img" :aria-label="review.days.map(dayText).join('; ')">
          <div v-for="(d, i) in review.days" :key="d.date" class="rv-day" :title="dayText(d, i)">
            <div class="rv-bars">
              <i class="k-cal" :style="{ height: pct(d.calendar_minutes) }"></i>
              <i class="k-trk" :style="{ height: pct(d.tracked_minutes) }"></i>
            </div>
            <span class="rv-n mono">{{ hm(d.tracked_minutes) }}</span>
            <span class="lbl">{{ WD[i] }}</span>
          </div>
        </div>
      </div>

      <div class="start-grid">
        <div class="start-col">
          <div class="col-head"><h2 class="col-title">Zeit pro Projekt</h2></div>
          <div class="card tasklist">
            <div v-for="p in projects" :key="p.project_id ?? ''" class="rv-proj">
              <span class="rv-name">{{ p.name }}</span>
              <span class="rv-n mono">{{ hm(p.tracked_minutes) }} h<template v-if="p.completed_tasks"> · {{ p.completed_tasks }} erledigt</template></span>
              <div class="dayload" aria-hidden="true"><span class="dl-busy" :style="{ width: `${(p.tracked_minutes / maxProject) * 100}%`, '--acc': projectColor(p.project_id) }"></span></div>
            </div>
            <div v-if="!projects.length" class="v-sub rv-empty">Keine Zeit erfasst.</div>
          </div>

          <div class="col-head rv-gap"><h2 class="col-title">Gewohnheiten</h2></div>
          <div class="card tasklist">
            <div v-for="h in review.habits" :key="h.habit_id" class="task-row">
              <span class="t">{{ h.name }}</span>
              <span class="rv-n mono">{{ h.done }}<template v-if="target(h.habit_id)">/{{ target(h.habit_id) }}</template></span>
              <span class="rv-n">{{ h.streak }} Tage Serie</span>
            </div>
            <div v-if="!review.habits.length" class="v-sub rv-empty">Keine aktiven Gewohnheiten.</div>
          </div>
        </div>

        <div class="start-col">
          <div class="col-head"><h2 class="col-title">Erledigt</h2></div>
          <div class="card tasklist">
            <div v-for="t in completed" :key="t.id" class="task-row">
              <span class="t">{{ t.title }}</span>
              <span class="due">{{ doneMeta(t) }}</span>
            </div>
            <div v-if="!completed.length" class="v-sub rv-empty">Nichts erledigt.</div>
          </div>

          <div class="col-head rv-gap"><h2 class="col-title">Überfällig geworden</h2></div>
          <div class="card tasklist">
            <div v-for="t in overdue" :key="t.id" class="task-row">
              <span class="t">{{ t.title }}</span>
              <span class="due od">{{ daysAgo(t.planned_date!, today()) }}</span>
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
.rv-stats { margin-bottom: 24px; }
.rv-chart { padding: 18px 20px; margin-bottom: 32px; }
.rv-legend { display: flex; gap: 16px; margin-bottom: 14px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.rv-legend span { display: inline-flex; align-items: center; gap: 6px; }
.rv-legend i { width: 8px; height: 8px; border-radius: 2px; }
.k-cal { background: var(--a-blue); }
.k-trk { background: var(--a-green); }
.rv-days { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 8px; }
.rv-day { display: grid; justify-items: center; gap: 6px; }
.rv-bars { display: flex; align-items: flex-end; justify-content: center; gap: 4px; height: 120px; width: 100%; }
.rv-bars i { width: min(18px, 40%); border-radius: 2px 2px 0 0; }
.rv-n { font: 400 12px/1.2 var(--font-mono); color: var(--tx-secondary); white-space: nowrap; }
.rv-proj { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px 12px; padding: 10px 16px; border-bottom: 1px solid var(--br-subtle); }
.rv-proj:last-child { border-bottom: 0; }
.rv-proj .dayload { grid-column: 1 / -1; }
.rv-name { min-width: 0; font: 500 14px/1.4 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rv-gap { margin-top: 28px; }
.rv-empty { padding: 12px 16px; margin: 0; }
</style>
