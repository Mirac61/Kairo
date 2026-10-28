<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getReview, isOpen, listHabits, listProjects, listTasks, updateTask, type Habit, type Review, type Task } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { addDays, daysAgo, dm, hm, weekStart, ymd } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { iconOf } from '@/lib/habitIcons'
import { habitColor, projectColor } from '@/lib/projectColor'

// Die API liefert completed_at, Task kennt es noch nicht.
type DoneTask = Task & { completed_at: string | null }

const WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map((d) => t(d))
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
// Skala in ganzen Stunden-Schritten, damit die Hilfslinien glatte Werte haben.
const step = computed(() => (Math.max(...days.value.flatMap((d) => [d.tracked_minutes, d.plan])) > 600 ? 240 : 120))
const maxMin = computed(() => Math.max(240, Math.ceil(Math.max(...days.value.flatMap((d) => [d.tracked_minutes, d.plan])) / step.value) * step.value))
const ticks = computed(() => Array.from({ length: maxMin.value / step.value + 1 }, (_, i) => i * step.value))
const pct = (m: number) => `${m ? Math.max(2, (m / maxMin.value) * 100) : 0}%`
const calendarMinutes = computed(() => review.value?.days.reduce((s, d) => s + d.calendar_minutes, 0) ?? 0)
const planToDate = computed(() => days.value.filter((d) => !d.future).reduce((s, d) => s + d.plan, 0))
const trackedPct = computed(() => (planToDate.value ? Math.round(((review.value?.tracked_minutes ?? 0) / planToDate.value) * 100) : 0))
const weekTasks = computed(() => tasks.value.filter((t) => t.planned_date && t.planned_date >= from.value && t.planned_date <= to.value && t.status !== 'CANCELLED'))
const dayText = (d: (typeof days.value)[number]) =>
  `${WD[d.i]} ${dm(d.date)}: ` + t('erfasst {tracked} h, geplant {plan} h, {done} erledigt', { tracked: hm(d.tracked_minutes), plan: hm(d.plan), done: d.completed_tasks })

const projects = computed(() => (review.value?.projects ?? []).filter((p) => p.tracked_minutes))
const maxProject = computed(() => Math.max(1, ...projects.value.map((p) => p.tracked_minutes)))
const projectTotal = computed(() => projects.value.reduce((s, p) => s + p.tracked_minutes, 0) || 1)
const projectName = (id: string | null) => review.value?.projects.find((p) => p.project_id === id)?.name

const completed = computed(() => tasks.value
  .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) >= from.value && ymd(new Date(t.completed_at)) <= to.value)
  .sort((a, b) => a.completed_at!.localeCompare(b.completed_at!)))

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
const habitIcon = (id: string, name: string) => iconOf(habits.value.find((x) => x.id === id) ?? { name })
// Segmente wie beim Fokus-Ziel auf der Startseite; bei mehr als 7 Zielen pro Woche reicht ein Balken.
const segs = (id: string) => { const n = target(id); return n > 0 && n <= 7 ? n : 0 }

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
        <button v-if="!isCurrent" class="btn btn-ghost" type="button" @click="go(weekStart(new Date()))">{{ $t('Diese Woche') }}</button>
        <button class="icon-btn" type="button" :aria-label="$t('Vorherige Woche')" @click="go(addDays(monday, -7))"><svg class="ic"><use href="#i-left" /></svg></button>
        <button class="icon-btn" type="button" :aria-label="$t('Nächste Woche')" :disabled="isCurrent" @click="go(addDays(monday, 7))"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <template v-if="review">
      <!-- Kennzahlen als Karten wie auf der Startseite -->
      <div class="rv-stats">
        <section class="rv-card">
          <div class="rv-card-h"><span>{{ $t('Erfasst') }}</span><span v-if="planToDate" class="rv-pill">{{ trackedPct }} %</span></div>
          <span class="rv-big mono" :class="{ zero: !review.tracked_minutes }">{{ hm(review.tracked_minutes) }}</span>
          <div class="rv-card-f">
            <template v-if="planToDate">
              <div class="rv-meter" aria-hidden="true"><span :style="{ width: `${Math.min(100, trackedPct)}%` }"></span></div>
              <span>{{ $t('von {plan} geplant', { plan: hm(planToDate) }) }}</span>
            </template>
            <span v-else>{{ $t('Noch nichts geplant') }}</span>
          </div>
        </section>
        <section class="rv-card">
          <div class="rv-card-h"><span>{{ $t('Termine') }}</span></div>
          <span class="rv-big mono" :class="{ zero: !calendarMinutes }">{{ hm(calendarMinutes) }}</span>
          <div class="rv-card-f"><span>{{ $t('Kalenderzeit') }}</span></div>
        </section>
        <section class="rv-card">
          <div class="rv-card-h"><span>{{ $t('Erledigt') }}</span><span v-if="weekTasks.length" class="rv-pill">{{ Math.round((review.completed_tasks / weekTasks.length) * 100) }} %</span></div>
          <span class="rv-big mono" :class="{ zero: !review.completed_tasks }">{{ review.completed_tasks }}<small v-if="weekTasks.length"> / {{ weekTasks.length }}</small></span>
          <div class="rv-card-f"><span>{{ $t('Aufgaben dieser Woche') }}</span></div>
        </section>
        <section class="rv-card">
          <div class="rv-card-h"><span>{{ $t('Überfällig geworden') }}</span></div>
          <span class="rv-big mono" :class="overdue.length ? 'od' : 'zero'">{{ overdue.length }}</span>
          <div class="rv-card-f">
            <span v-if="overdue.length" class="rv-ell">{{ overdue[0]!.title }}<template v-if="overdue.length > 1"> + {{ overdue.length - 1 }}</template></span>
            <span v-else>{{ $t('Nichts überfällig.') }}</span>
          </div>
        </section>
      </div>

      <section class="rv-hero">
        <div class="rv-legend">
          <h2 class="rv-h">{{ $t('Plan und Ist pro Tag') }}</h2><span class="spacer"></span>
          <span><i class="k-plan"></i>{{ $t('Geplant') }}</span><span><i class="k-trk"></i>{{ $t('Erfasst') }}</span><span><i class="k-soon"></i>{{ $t('Kommt noch') }}</span>
        </div>
        <div class="rv-plot">
          <div class="rv-grid" aria-hidden="true"><span v-for="m in ticks" :key="m" :style="{ bottom: pct(m) }">{{ m / 60 }} h</span></div>
          <div class="rv-days">
            <RouterLink
              v-for="d in days" :key="d.date" class="rv-day" :class="{ now: d.date === today(), future: d.future }"
              :to="{ name: 'calendar', query: { date: d.date } }" :aria-label="dayText(d)" :title="dayText(d)"
            >
              <div class="rv-bars" aria-hidden="true">
                <i :class="d.future ? 'k-soon' : 'k-plan'" :style="{ height: pct(d.plan) }"></i>
                <i v-if="!d.future" class="k-trk" :style="{ height: pct(d.tracked_minutes) }"></i>
              </div>
              <span class="rv-dl">{{ WD[d.i] }} <b>{{ Number(d.date.slice(8)) }}</b></span>
              <span v-if="d.future" class="rv-n">{{ d.plan ? hm(d.plan) : $t('frei') }}</span>
              <span v-else class="rv-n"><b>{{ hm(d.tracked_minutes) }}</b><span class="rv-of"> / {{ hm(d.plan) }}</span></span>
              <span class="rv-done">{{ d.completed_tasks ? `✓ ${d.completed_tasks}` : '' }}</span>
            </RouterLink>
          </div>
        </div>
      </section>

      <div class="start-grid">
        <div class="start-col">
          <section class="rv-group">
            <h2 class="rv-gh">{{ $t('Zeit pro Projekt') }} <span class="count">{{ hm(review.tracked_minutes) }}</span></h2>
            <div v-for="p in projects" :key="p.project_id ?? ''" class="rv-row rv-proj">
              <span class="chip" :style="{ '--chip-c': projectColor(p.project_id) }"><span class="cdot"></span><span class="rv-ell">{{ p.name }}</span></span>
              <div class="rv-track" aria-hidden="true"><span :style="{ width: `${(p.tracked_minutes / maxProject) * 100}%`, background: projectColor(p.project_id) }"></span></div>
              <span class="rv-n mono">{{ hm(p.tracked_minutes) }}</span>
              <span class="rv-n rv-pct">{{ Math.round((p.tracked_minutes / projectTotal) * 100) }} %</span>
            </div>
            <div v-if="!projects.length" class="rv-empty"><b>{{ $t('Keine Zeit erfasst.') }}</b> {{ $t('Starte einen Timer an einer Aufgabe, dann erscheint die Zeit hier.') }}</div>
          </section>

          <section class="rv-group">
            <h2 class="rv-gh">{{ $t('Gewohnheiten') }} <span class="count">{{ review.habits.length || '' }}</span></h2>
            <div v-for="h in review.habits" :key="h.habit_id" class="rv-row rv-habit" :class="{ met: target(h.habit_id) && h.done >= target(h.habit_id) }" :style="{ '--hc': habitColor(h.habit_id) }">
              <span class="rv-hicon" aria-hidden="true"><svg class="ic"><use :href="`#${habitIcon(h.habit_id, h.name)}`" /></svg></span>
              <span class="rv-ht">
                <span class="t">{{ h.name }}</span>
                <span class="rv-hs">{{ target(h.habit_id) ? $t('{done} von {target}', { done: h.done, target: target(h.habit_id) }) : h.done }}<template v-if="h.streak"> · <svg class="ic" aria-hidden="true"><use href="#i-flame" /></svg>{{ $t('{n} Tage Serie', { n: h.streak }) }}</template></span>
              </span>
              <span v-if="segs(h.habit_id)" class="rv-segs" aria-hidden="true"><i v-for="n in segs(h.habit_id)" :key="n" :class="{ on: n <= h.done }"></i></span>
              <span v-else class="rv-track rv-htrack" aria-hidden="true"><span :style="{ width: `${target(h.habit_id) ? Math.min(100, (h.done / target(h.habit_id)) * 100) : 0}%` }"></span></span>
            </div>
            <div v-if="!review.habits.length" class="rv-empty">{{ $t('Keine aktiven Gewohnheiten.') }}</div>
          </section>
        </div>

        <div class="start-col">
          <section class="rv-group">
            <h2 class="rv-gh">{{ $t('Erledigt') }} <span class="count">{{ completed.length || '' }}</span></h2>
            <div v-for="t in completed" :key="t.id" class="rv-row rv-task">
              <span class="cb ring" role="img" aria-checked="true" :aria-label="$t('Erledigt')"></span>
              <span class="t">{{ t.title }}</span>
              <span class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><template v-if="projectName(t.project_id)"><span class="cdot"></span><span class="rv-ell">{{ projectName(t.project_id) }}</span></template></span>
              <span class="rv-n rv-wd">{{ WD[(new Date(t.completed_at!).getDay() + 6) % 7] }}</span>
            </div>
            <div v-if="!completed.length" class="rv-empty">{{ $t('Nichts erledigt.') }}</div>
          </section>

          <section class="rv-group">
            <h2 class="rv-gh">{{ $t('Überfällig geworden') }} <span class="count">{{ overdue.length || '' }}</span></h2>
            <div v-for="t in overdue" :key="t.id" class="rv-row rv-task rv-od">
              <span class="cb ring" aria-hidden="true"></span>
              <span class="t">{{ t.title }}</span>
              <span class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><template v-if="projectName(t.project_id)"><span class="cdot"></span><span class="rv-ell">{{ projectName(t.project_id) }}</span></template></span>
              <span class="rv-n od">{{ daysAgo(t.planned_date!, today()) }}</span>
              <button type="button" class="btn btn-ghost rv-plan" :aria-label="`Einplanen: ${t.title}`" @click="plan(t)">{{ $t('Heute') }}</button>
            </div>
            <div v-if="!overdue.length" class="rv-empty">{{ $t('Nichts überfällig.') }}</div>
          </section>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.rv-nav { display: flex; align-items: center; gap: 4px; }
.rv-nav :disabled { opacity: .35; cursor: default; }
.rv-ell { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Karten: gleiche Sprache wie .st-card auf der Startseite */
.rv-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 4px 0 16px; }
.rv-card { display: flex; flex-direction: column; gap: 10px; min-width: 0; padding: 14px 16px; border-radius: var(--r-m); background: var(--bg-1); border: 1px solid var(--br-default); transition: border-color var(--dur) ease-out; }
.rv-card:hover { border-color: var(--br-strong); }
.rv-card-h { display: flex; align-items: center; justify-content: space-between; gap: 8px; height: 20px; font: 600 12px/1 var(--font-ui); color: var(--tx-secondary); }
.rv-pill { display: inline-flex; align-items: center; height: 20px; padding: 0 8px; border-radius: 6px; background: var(--bg-2); box-shadow: inset 0 0 0 1px var(--br-subtle); font: 500 11.5px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-secondary); }
.rv-big { font-size: 30px; font-weight: 500; line-height: 1.1; letter-spacing: -0.02em; color: var(--tx-primary); }
.rv-big small { font-size: 15px; letter-spacing: 0; color: var(--tx-muted); }
.rv-big.zero { color: var(--tx-muted); }
.rv-big.od { color: var(--a-red); }
.rv-card-f { display: flex; align-items: center; gap: 10px; min-width: 0; min-height: 18px; margin-top: auto; font: 400 11.5px/1.2 var(--font-ui); color: var(--tx-muted); }
.rv-meter { flex: 1; max-width: 120px; height: 4px; border-radius: 2px; background: var(--br-default); overflow: hidden; }
.rv-meter span { display: block; height: 100%; background: var(--a-blue-hi); border-radius: 2px; }

/* Diagramm in der hervorgehobenen Karte (wie .st-hero) */
.rv-hero { margin-bottom: 40px; padding: 18px 20px 12px; border-radius: 14px; background: linear-gradient(180deg, var(--bg-3), var(--bg-2) 60%); box-shadow: inset 0 0 0 1px var(--br-subtle); }
.rv-h { margin: 0; font: 600 14.5px/1.3 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }
.rv-legend { display: flex; align-items: center; gap: 16px; margin-bottom: 14px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.rv-legend span { display: inline-flex; align-items: center; gap: 6px; }
.rv-legend i { width: 8px; height: 8px; border-radius: 2px; box-sizing: border-box; }
.spacer { flex: 1; }
.k-plan { background: color-mix(in srgb, var(--tx-primary) 13%, transparent); }
.k-trk { background: var(--a-blue-hi); }
.k-soon { background: transparent; border: 1px dashed var(--br-strong); }
.rv-plot { position: relative; padding-left: 30px; }
.rv-grid { position: absolute; left: 0; right: 0; top: 8px; height: 140px; pointer-events: none; }
.rv-grid span { position: absolute; left: 0; right: 0; height: 0; font: 400 10.5px/1 var(--font-mono); color: var(--tx-muted); transform: translateY(-5px); }
.rv-grid span::after { content: ''; position: absolute; left: 30px; right: 0; top: 5px; border-top: 1px dashed var(--br-subtle); }
.rv-grid span:first-child::after { border-top: 1px solid var(--br-default); }
.rv-days { position: relative; display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 6px; }
.rv-day { display: grid; justify-items: center; gap: 4px; padding: 8px 4px 6px; border-radius: var(--r-s); color: inherit; text-decoration: none; transition: background var(--dur) ease-out; }
.rv-day:hover { background: color-mix(in oklab, var(--tx-primary) 4%, transparent); }
.rv-day.now { background: color-mix(in oklab, var(--tx-primary) 6%, transparent); }
.rv-day:focus-visible { outline: 2px solid var(--a-blue-hi); outline-offset: 1px; }
/* Ein Balken je Tag: Plan als Spur, Erfasstes füllt sie von unten. */
.rv-bars { position: relative; height: 140px; width: min(34px, 64%); margin-bottom: 6px; }
.rv-bars i { position: absolute; left: 0; right: 0; bottom: 0; border-radius: 5px 5px 2px 2px; box-sizing: border-box; transition: height var(--dur) ease-out; }
.rv-bars .k-soon { border-bottom: 0; }
.rv-dl { font: 500 11px/1.2 var(--font-ui); letter-spacing: .04em; text-transform: uppercase; color: var(--tx-muted); }
.rv-dl b { color: var(--tx-primary); font-weight: 600; }
.rv-day.now .rv-dl b { display: inline-grid; place-items: center; min-width: 20px; height: 20px; padding: 0 3px; border-radius: 10px; background: var(--a-blue-hi); color: var(--bg-0); }
.rv-n { font: 400 12px/1.2 var(--font-mono); font-variant-numeric: tabular-nums; color: var(--tx-muted); white-space: nowrap; }
.rv-n b { color: var(--tx-primary); font-weight: 500; }
.rv-done { min-height: 11px; font: 500 11px/1 var(--font-ui); color: var(--tx-muted); }

/* Gruppen und Zeilen wie die Aufgabengruppen auf der Startseite */
.rv-group + .rv-group { margin-top: 32px; }
.rv-gh { display: flex; align-items: center; gap: 8px; height: 28px; margin: 0 0 4px; font: 600 14.5px/1.3 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }
.rv-gh .count { font: 400 13px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.rv-row { display: grid; align-items: center; column-gap: 14px; min-height: 42px; padding: 6px 12px; margin: 0 -12px; border-radius: var(--r-s); transition: background var(--dur) ease-out; }
.rv-row:hover { background: color-mix(in oklab, var(--tx-primary) 4%, transparent); }
.rv-row .t { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: 500 14px/1.4 var(--font-ui); color: var(--tx-primary); }
.rv-row .chip { min-width: 0; justify-content: flex-end; }
.rv-proj { grid-template-columns: minmax(110px, 190px) minmax(0, 1fr) 44px 38px; }
.rv-proj .chip { justify-content: flex-start; font-weight: 500; color: var(--tx-primary); }
.rv-track { display: block; height: 6px; border-radius: 3px; background: var(--br-subtle); overflow: hidden; }
.rv-track span { display: block; height: 100%; border-radius: 3px; }
.rv-pct { text-align: right; }
.rv-proj .rv-n.mono { color: var(--tx-secondary); text-align: right; }
.rv-habit { grid-template-columns: 30px minmax(0, 1fr) auto; }
.rv-hicon { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 8px; color: color-mix(in srgb, var(--hc) 85%, var(--tx-primary)); background: color-mix(in srgb, var(--hc) 14%, var(--bg-2)); }
.rv-hicon .ic { width: 52%; height: 52%; stroke-width: 1.9; }
.rv-ht { display: grid; gap: 2px; min-width: 0; }
.rv-hs { display: inline-flex; align-items: center; gap: 3px; font: 400 12px/1.2 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.rv-hs .ic { width: 12px; height: 12px; margin-left: 2px; }
.rv-segs { display: grid; grid-auto-flow: column; grid-auto-columns: 14px; gap: 3px; }
.rv-segs i { height: 6px; border-radius: 3px; background: var(--br-default); }
.rv-segs i.on { background: var(--hc); }
.rv-htrack { width: 116px; }
.rv-htrack span { background: var(--hc); }
.rv-task { grid-template-columns: 18px minmax(0, 1fr) auto 28px; }
.rv-task .cb { pointer-events: none; }
.rv-task:not(.rv-od) .t { color: var(--tx-secondary); }
.rv-wd { text-align: right; font-family: var(--font-ui); }
.rv-od { grid-template-columns: 18px minmax(0, 1fr) auto auto auto; }
.rv-od .od { color: var(--a-red); font-family: var(--font-ui); }
.rv-plan { height: 26px; padding: 0 10px; color: var(--a-blue-tx); opacity: 0; transition: opacity var(--dur) ease-out; }
.rv-od:hover .rv-plan, .rv-plan:focus-visible { opacity: 1; }
@media (hover: none) { .rv-plan { opacity: 1; } }
.rv-empty { padding: 10px 0; font: 400 13px/1.5 var(--font-ui); color: var(--tx-muted); }
.rv-empty b { color: var(--tx-secondary); font-weight: 500; }

@media (max-width: 640px) {
  .rv-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .rv-big { font-size: 24px; }
  .rv-hero { padding: 14px 10px 8px; }
  .rv-plot { padding-left: 22px; }
  .rv-grid span::after { left: 22px; }
  .rv-days { gap: 2px; }
  .rv-bars { width: min(24px, 70%); }
  .rv-dl { font-size: 10px; white-space: nowrap; }
  .rv-n { font-size: 10.5px; }
  .rv-of { display: none; } /* Plan steht im Tooltip und als Balken */
  .rv-legend { flex-wrap: wrap; row-gap: 10px; }
  .rv-legend .spacer { flex-basis: 100%; }
  .rv-proj { grid-template-columns: minmax(0, 1fr) 64px 40px 34px; column-gap: 10px; }
  .rv-task .chip { display: none; }
  .rv-task { grid-template-columns: 18px minmax(0, 1fr) 28px; }
  .rv-od { grid-template-columns: 18px minmax(0, 1fr) auto auto; }
}
</style>
