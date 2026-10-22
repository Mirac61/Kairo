<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage, getJournalSummary, getToday, isOpen, listJournalTaskImports, listNotes, listTasks, readNote, type JournalTaskImport, type Task } from '@/api/client'
import { addDays, shiftDate, weekStart, ymd } from '@/lib/dates'
import { journalPath, parseEntry, splitPoints } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'
import { useUndo } from '@/composables/useUndo'
import JournalEntryEditor, { type JournalEntryData } from '@/components/JournalEntryEditor.vue'

// Kompakte Liste der letzten Einträge; der Heute-Eintrag steht separat oben.
const RECENT = 60
const HEAT_WEEKS = 12

const route = useRoute()

interface Row { date: string; answers: string[]; other: string; mtime: string }
type DoneTask = Task & { completed_at: string | null }

const todayStr = ref('')
const total = ref(0)
const streak = ref(0)
const loaded = ref(false)
const error = ref('')
const allDates = ref<string[]>([])
const pastRows = ref<Row[]>([])
const todayEntry = ref<JournalEntryData | null>(null)
const taskImports = ref<Record<string, { count: number; titles: string[] }>>({})
const allTasks = ref<DoneTask[]>([])
const completedHabits = ref<string[]>([])
const expandedDate = ref<string | null>(null)
const todayEditor = ref<InstanceType<typeof JournalEntryEditor>>()
const pastEditor = ref<InstanceType<typeof JournalEntryEditor>>()

// Rechte Spalte an der linken ausrichten: Der Rückblick endet an der Heute-Karte,
// „Noch offen“ beginnt auf Höhe des ersten vergangenen Eintrags.
const todayEl = ref<HTMLElement>()
const topEl = ref<HTMLElement>()
const pastEl = ref<HTMLElement>()
const todayH = ref(0)
const openGap = ref(20)
let todayObserver: ResizeObserver | undefined

async function alignSidebar() {
  await nextTick()
  const today = todayEl.value
  const top = topEl.value
  const past = pastEl.value
  if (!today || !top || !past) return
  if (!todayObserver) {
    todayObserver = new ResizeObserver(() => { void alignSidebar() })
    todayObserver.observe(today)
  }
  todayH.value = today.getBoundingClientRect().height
  await nextTick()
  const first = past.querySelector<HTMLElement>('.jr-entry')
  openGap.value = first ? Math.max(20, first.getBoundingClientRect().top - top.getBoundingClientRect().bottom) : 20
}

const onResize = () => { void alignSidebar() }
onMounted(() => {
  window.addEventListener('resize', onResize)
  void alignSidebar()
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  todayObserver?.disconnect()
})
watch(locale, () => { void alignSidebar() })

const longFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }))
const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const monthFmt = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }))
const heatLabel = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))
const relativeLabel = (d: string) => (d === todayStr.value ? t('Heute') : d === shiftDate(todayStr.value, -1) ? t('Gestern') : '')
const label = (d: string) => relativeLabel(d) || fmt(dayFmt.value, d)
const monthLabel = (m: string) => fmt(monthFmt.value, `${m}-01`)
// Vorschau: je beantworteter Frage der erste Punkt, damit jeder Tag als kurze Stichpunkte lesbar bleibt.
const previewPoints = (r: Row) => r.answers.map((a) => splitPoints(a)[0] ?? '').filter(Boolean)

const groups = computed(() => {
  const out: { month: string; rows: Row[] }[] = []
  for (const r of pastRows.value) {
    const month = r.date.slice(0, 7)
    const last = out[out.length - 1]
    if (last?.month === month) last.rows.push(r)
    else out.push({ month, rows: [r] })
  }
  return out
})
const shownCount = computed(() => pastRows.value.length + (todayEntry.value ? 1 : 0))

const completedTasks = computed(() => allTasks.value
  .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) === todayStr.value)
  .sort((a, b) => (a.completed_at ?? '').localeCompare(b.completed_at ?? '')))
// Aus dem Journal übernommen und noch offen: der einzige Ort, an dem der Import sichtbar weiterlebt.
const openJournalTasks = computed(() => allTasks.value
  .filter((t) => t.source_journal_date && isOpen(t))
  .sort((a, b) => (a.due_at ?? '').localeCompare(b.due_at ?? '')))

const heatCells = computed(() => {
  if (!todayStr.value) return []
  const set = new Set(allDates.value)
  const today = todayStr.value
  const start = addDays(weekStart(new Date(`${today}T00:00:00`)), -7 * (HEAT_WEEKS - 1))
  return Array.from({ length: HEAT_WEEKS * 7 }, (_, i) => {
    const d = addDays(start, i)
    const s = ymd(d)
    return { s, has: set.has(s), future: s > today, label: heatLabel.value.format(d) }
  })
})

const importedTitles = (date: string) => taskImports.value[date]?.titles ?? []

async function readEntries(dates: string[]): Promise<Row[]> {
  return Promise.all(dates.map(async (date) => {
    const n = await readNote(journalPath(date))
    const e = parseEntry(n.content)
    return { date, answers: e.answers, other: e.other, mtime: n.mtime }
  }))
}

async function load() {
  try {
    const [notes, imports, summary, today, tasks] = await Promise.all([
      listNotes(), listJournalTaskImports(), getJournalSummary(), getToday(), listTasks(),
    ])
    const dir = notes.find((n) => n.dir && n.path === 'Journal')
    const dates = (dir?.children ?? [])
      .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
      .filter((d): d is string => !!d)
      .sort()
      .reverse()

    // Erst die Einträge lesen, dann alle reaktiven Werte synchron setzen,
    // damit der Heute-Editor direkt mit seinem Eintrag montiert.
    const rows = await readEntries(dates.slice(0, RECENT))
    todayStr.value = summary.date
    total.value = summary.total
    streak.value = summary.streak
    allDates.value = dates
    taskImports.value = Object.fromEntries(imports.map((i) => [i.journal_date, { count: i.task_count, titles: i.titles }]))
    todayEntry.value = rows.find((r) => r.date === summary.date) ?? null
    pastRows.value = rows.filter((r) => r.date !== summary.date)

    allTasks.value = tasks as DoneTask[]
    completedHabits.value = today.habits.filter((h) => h.done).map((h) => h.name)

    if (typeof route.query.date === 'string' && route.query.date !== summary.date && pastRows.value.some((r) => r.date === route.query.date)) {
      expandedDate.value = route.query.date
    }
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
  void alignSidebar()
}
onMounted(load)
watch(() => route.query.date, (date) => {
  if (typeof date === 'string' && date !== todayStr.value && pastRows.value.some((r) => r.date === date)) void switchEntry(date)
})

function toggle(date: string) {
  void switchEntry(expandedDate.value === date ? null : date)
}

// Ohne Eingabe bleibt ein nachgetragener Tag ein leerer Entwurf; er verschwindet beim Schließen wieder.
const isDraft = (r: Row) => !r.mtime && !r.other.trim() && r.answers.every((a) => !a.trim())

// Erst den offenen Eintrag fertig speichern, dann wechseln: So ist die Zeile in pastRows aktuell,
// bevor die Komponente unmountet.
async function switchEntry(date: string | null) {
  await Promise.all([todayEditor.value?.flush(), pastEditor.value?.flush()])
  if (date === null && expandedDate.value) {
    const closing = expandedDate.value
    expandedDate.value = null
    if (pastRows.value.some((r) => r.date === closing && isDraft(r))) {
      pastRows.value = pastRows.value.filter((r) => r.date !== closing)
    }
    return
  }
  expandedDate.value = date
}

function onTodaySaved(entry: JournalEntryData) {
  todayEntry.value = entry
  if (!allDates.value.includes(todayStr.value)) allDates.value = [...allDates.value, todayStr.value].sort().reverse()
}

function onTodayImported(result: JournalTaskImport) {
  taskImports.value = { ...taskImports.value, [result.journal_date]: { count: result.task_count, titles: result.titles } }
  void refreshTasks()
}

function onPastSaved(date: string, entry: JournalEntryData) {
  pastRows.value = pastRows.value.map((r) => (r.date === date ? { ...r, ...entry } : r))
}

function onPastImported(result: JournalTaskImport) {
  onTodayImported(result)
}

// Nach Import oder Erledigen die Aufgabenliste neu laden (und Fehler sichtbar machen).
async function refreshTasks() {
  try {
    allTasks.value = (await listTasks()) as DoneTask[]
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Aufgabe direkt aus der Seitenleiste erledigen; danach wandert sie in den Tagesrückblick.
const { setDone } = useUndo()
const completing = ref('')
function completeTask(task: Task) {
  if (completing.value) return
  completing.value = task.id
  setDone(task, 'COMPLETED', async (fn) => {
    try {
      await fn()
      await refreshTasks()
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      completing.value = ''
    }
  })
}
</script>

<template>
  <div class="view-inner">
    <div class="jr-page">
      <div class="v-head">
        <h1 class="v-title">{{ $t('Journal') }}</h1>
        <div class="v-sub">{{ $tn(total, '{n} Eintrag', '{n} Einträge') }}</div>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div class="jr-layout">
        <div class="jr-main">
        <!-- Heute -->
        <section v-if="todayStr" ref="todayEl" class="jr-today">
          <header class="jr-today-head">
            <h2 class="jr-today-title">{{ $t('Heute') }}</h2>
            <span class="jr-today-date">{{ fmt(longFmt, todayStr) }}</span>
          </header>
          <JournalEntryEditor
            ref="todayEditor"
            :date="todayStr"
            :entry="todayEntry"
            :imported-titles="importedTitles(todayStr)"
            @saved="onTodaySaved"
            @imported="onTodayImported"
          />
        </section>

        <!-- Vergangene Einträge -->
        <section ref="pastEl" class="jr-past">
          <h2 class="jr-past-title">{{ $t('Letzte Einträge') }}</h2>
          <div v-if="loaded && !pastRows.length" class="jr-empty">
            <p class="jr-empty-t">{{ $t('Drei kurze Fragen am Abend.') }}</p>
            <p class="v-sub">{{ $t('Was lief gut, was lernst du daraus, was ist morgen wichtig. Zwei Minuten reichen.') }}</p>
          </div>
          <div v-else-if="pastRows.length" class="jr-months" :class="{ 'has-timeline': pastRows.length >= 3 }">
            <section v-for="g in groups" :key="g.month" class="jr-month-group">
              <h3 v-if="g.rows.length > 1" class="jr-month">{{ monthLabel(g.month) }}</h3>
              <div class="jr-feed">
                <article v-for="r in g.rows" :key="r.date" class="jr-entry" :data-date="r.date">
                  <button type="button" class="jr-row" :aria-expanded="expandedDate === r.date" @click="toggle(r.date)">
                    <span class="jr-date">{{ label(r.date) }}</span>
                    <span class="jr-points" :class="{ empty: !previewPoints(r).length }">
                      <span v-for="(p, i) in previewPoints(r)" :key="i" class="jr-point">{{ p }}</span>
                      <span v-if="!previewPoints(r).length" class="jr-point">–</span>
                    </span>
                    <svg class="ic jr-chev"><use href="#i-right" /></svg>
                  </button>
                  <JournalEntryEditor
                    v-if="expandedDate === r.date"
                    ref="pastEditor"
                    :key="r.date"
                    :date="r.date"
                    :entry="r"
                    :imported-titles="importedTitles(r.date)"
                    @saved="onPastSaved(r.date, $event)"
                    @imported="onPastImported"
                  />
                </article>
              </div>
            </section>
          </div>
          <p v-if="total > shownCount" class="v-sub jr-more">{{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}</p>
        </section>
      </div>

      <!-- Seitenleiste -->
      <aside class="jr-side">
        <div ref="topEl" class="jr-side-top" :style="{ minHeight: todayH ? `${todayH}px` : undefined }">
          <section class="jr-side-box">
          <div class="jr-side-head">
            <h2 class="jr-side-title">{{ $t('Letzte 12 Wochen') }}</h2>
            <span v-if="streak" class="jr-streak">{{ $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge') }}</span>
          </div>
          <div class="jr-heat" role="group" :aria-label="$t('Letzte 12 Wochen')">
            <i v-for="c in heatCells" :key="c.s" :class="{ on: c.has, future: c.future }" :title="c.label"></i>
          </div>
        </section>
        <section class="jr-side-box">
          <h2 class="jr-side-title">{{ $t('Tagesrückblick') }}</h2>
          <template v-if="completedTasks.length || completedHabits.length">
            <div v-if="completedTasks.length" class="jr-recap-group">
              <span class="jr-recap-label">{{ $t('Erledigte Aufgaben') }}</span>
              <ul class="jr-recap-list"><li v-for="t in completedTasks" :key="t.id">{{ t.title }}</li></ul>
            </div>
            <div v-if="completedHabits.length" class="jr-recap-group">
              <span class="jr-recap-label">{{ $t('Erledigte Gewohnheiten') }}</span>
              <ul class="jr-recap-list"><li v-for="h in completedHabits" :key="h">{{ h }}</li></ul>
            </div>
          </template>
          <p v-else class="jr-recap-empty">{{ $t('Nichts erledigt.') }}</p>
        </section>
        </div>
        <section v-if="openJournalTasks.length" class="jr-side-box" :style="{ marginTop: `${openGap}px` }">
          <h2 class="jr-side-title">{{ $t('Noch offen aus dem Journal') }}</h2>
          <ul class="jr-recap-list jr-open-list">
            <li v-for="t in openJournalTasks.slice(0, 5)" :key="t.id" class="jr-open-item">
              <button
                type="button"
                class="cb jr-open-cb"
                role="checkbox"
                :aria-checked="false"
                :aria-label="$t('{title} erledigt', { title: t.title })"
                :disabled="completing === t.id"
                @click="completeTask(t)"
              ></button>
              <RouterLink class="jr-open-link" :to="`/tasks?task=${t.id}`">{{ t.title }}</RouterLink>
            </li>
          </ul>
          <RouterLink class="jr-open-all" to="/tasks">{{ $t('Alle Aufgaben') }}<svg class="ic"><use href="#i-right" /></svg></RouterLink>
        </section>
      </aside>
      </div>
    </div>
  </div>
</template>

<style scoped>
.jr-page { max-width: 1040px; margin: 0 auto; }
.jr-layout { display: grid; grid-template-columns: minmax(0, 700px) minmax(0, 260px); gap: 40px; align-items: start; justify-content: space-between; }
.jr-main { min-width: 0; }

/* Heute */
.jr-today { padding: 18px 20px 20px; border: 1px solid var(--br-default); border-radius: var(--r-m); background: var(--bg-1); margin-bottom: 28px; }
.jr-today-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.jr-today-title { font: 600 16px/1.3 var(--font-ui); letter-spacing: -0.01em; color: var(--tx-primary); }
.jr-today-date { font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }

/* Vergangene Einträge */
.jr-past { margin-top: 4px; }
.jr-past-title { font: 600 13px/1.3 var(--font-ui); color: var(--tx-secondary); margin-bottom: 12px; }
.jr-months { display: grid; gap: 22px; }
.jr-month-group { display: grid; gap: 8px; }
.jr-month { font: 600 11px/1 var(--font-ui); letter-spacing: .06em; text-transform: uppercase; color: var(--tx-muted); padding-left: 2px; }
.jr-feed { position: relative; display: grid; gap: 8px; }
.jr-feed::before { content: none; }
.has-timeline .jr-feed { padding-left: 18px; }
.has-timeline .jr-feed::before {
  content: ''; position: absolute; left: 5px; top: 14px; bottom: 14px; width: 1px; background: var(--br-default);
}
.has-timeline .jr-entry { position: relative; }
.has-timeline .jr-entry::before {
  content: ''; position: absolute; left: -17px; top: 16px; width: 7px; height: 7px; border-radius: 50%;
  background: var(--br-strong); box-shadow: 0 0 0 3px var(--bg-0);
}
.jr-entry { border: 1px solid var(--br-default); border-radius: var(--r-m); background: var(--bg-1); transition: border-color var(--dur) ease-out, background var(--dur) ease-out; }
.jr-entry:hover { border-color: var(--br-strong); }
.jr-row { display: flex; align-items: flex-start; gap: 12px; width: 100%; padding: 12px 14px; color: inherit; text-align: left; }
.jr-date { flex: none; min-width: 86px; font: 500 13px/1.5 var(--font-ui); color: var(--tx-secondary); }
.jr-points { flex: 1; min-width: 0; display: grid; gap: 1px; }
.jr-point {
  position: relative; padding-left: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font: 400 13px/1.5 var(--font-ui); color: var(--tx-secondary);
}
.jr-point::before { content: ''; position: absolute; left: 1px; top: .62em; width: 4px; height: 4px; border-radius: 50%; background: var(--br-strong); }
.jr-points.empty .jr-point { color: var(--tx-disabled); }
.jr-points.empty .jr-point::before { background: var(--br-default); }
.jr-chev { flex: none; margin-top: 3px; color: var(--tx-muted); transition: transform var(--dur) ease-out, color var(--dur) ease-out; }
.jr-entry:hover .jr-chev { color: var(--tx-secondary); transform: translateX(2px); }
.jr-entry .je { padding: 0 14px 14px; border-top: 1px solid var(--br-subtle); }
.jr-more { margin-top: 12px; }

.jr-empty { display: grid; justify-items: start; gap: 8px; padding: 20px; border: 1px solid var(--br-default); border-radius: var(--r-m); background: var(--bg-1); }
.jr-empty p { margin: 0; }
.jr-empty-t { font: 600 16px/1.3 var(--font-ui); color: var(--tx-primary); }

/* Seitenleiste */
.jr-side { position: sticky; top: 0; display: flex; flex-direction: column; min-width: 0; }
.jr-side-top { display: flex; flex-direction: column; gap: 20px; }
.jr-side-top .jr-side-box:last-child { flex: 1; }
.jr-side-box { padding: 16px; border: 1px solid var(--br-subtle); border-radius: var(--r-m); background: var(--bg-1); }
.jr-side-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; margin-bottom: 12px; }
.jr-side-title { font: 600 13px/1.3 var(--font-ui); color: var(--tx-secondary); margin-bottom: 12px; }
.jr-side-head .jr-side-title { margin-bottom: 0; }
.jr-streak { flex: none; font: 400 11px/1.3 var(--font-ui); color: var(--tx-muted); white-space: nowrap; }

.jr-heat { display: grid; grid-template-rows: repeat(7, 1fr); grid-auto-flow: column; grid-template-columns: repeat(12, 1fr); gap: 3px; justify-content: center; }
.jr-heat i { aspect-ratio: 1; border-radius: 3px; background: var(--bg-2); }
.jr-heat i.on { background: var(--a-blue-hi); }
/* Künftige Tage bleiben als leise Kästchen sichtbar, damit das Raster ein geschlossenes Rechteck bleibt. */
.jr-heat i.future { background: var(--bg-2); opacity: .3; }

.jr-recap-group { display: grid; gap: 6px; }
.jr-recap-group + .jr-recap-group { margin-top: 12px; }
.jr-recap-label { font: 500 10px/1.2 var(--font-ui); letter-spacing: .08em; text-transform: uppercase; color: var(--tx-muted); }
.jr-recap-list { list-style: none; display: grid; gap: 4px; }
.jr-recap-list li { font: 400 14px/1.5 var(--font-ui); color: var(--tx-primary); overflow-wrap: anywhere; }
.jr-recap-empty { font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.jr-open-list { gap: 7px; }
.jr-open-item { display: grid; grid-template-columns: 16px minmax(0, 1fr); align-items: start; gap: 8px; }
.jr-open-cb { margin-top: 3px; cursor: pointer; }
.jr-open-cb:hover:not(:disabled) { border-color: var(--a-blue-hi); }
.jr-open-link { color: var(--tx-secondary); text-decoration: none; }
.jr-open-link:hover { color: var(--tx-primary); text-decoration: underline; }
.jr-open-all {
  display: inline-flex; align-items: center; gap: 4px; margin-top: 12px;
  color: var(--a-blue-tx); font: 500 12px/1 var(--font-ui); text-decoration: none;
  transition: color var(--dur) ease-out, gap var(--dur) ease-out;
}
.jr-open-all:hover { color: var(--a-blue-hi); gap: 7px; }
.jr-open-all .ic { width: 13px; height: 13px; }

@media (max-width: 1000px) {
  .jr-layout { grid-template-columns: minmax(0, 1fr); }
  .jr-side { position: static; }
}
</style>
