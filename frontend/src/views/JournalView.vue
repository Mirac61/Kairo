<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage, getJournalSummary, getToday, isOpen, listJournalTaskImports, listNotes, listTasks, readNote, type JournalTaskImport, type Task } from '@/api/client'
import { addDays, shiftDate, weekStart, ymd } from '@/lib/dates'
import { journalPath, parseEntry, splitPoints } from '@/lib/journal'
import { locale, t, tn } from '@/lib/i18n'
import { vDialog } from '@/lib/dialog'
import { useUndo } from '@/composables/useUndo'
import JournalEntryEditor, { type JournalEntryData } from '@/components/JournalEntryEditor.vue'
import SearchField from '@/components/SearchField.vue'

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
// In der Liste steht der Ref wegen v-for als Array; es ist immer nur ein Eintrag aufgeklappt.
const pastEditor = ref<InstanceType<typeof JournalEntryEditor>[]>()
const todayEl = ref<HTMLElement>()
const pastEl = ref<HTMLElement>()

const headerDateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }))
const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const entryDateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'long' }))
const monthFmt = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }))
const heatLabel = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))
const relativeLabel = (d: string) => (d === todayStr.value ? t('Heute') : d === shiftDate(todayStr.value, -1) ? t('Gestern') : '')
const label = (d: string) => relativeLabel(d) || fmt(dayFmt.value, d)
const entryDateLabel = (d: string) => (d === shiftDate(todayStr.value, -1) ? t('Gestern, {date}', { date: fmt(entryDateFmt.value, d) }) : fmt(entryDateFmt.value, d))
const monthLabel = (m: string) => fmt(monthFmt.value, `${m}-01`)

// Suche und Reihenfolge der Liste; die Heatmap öffnet denselben Eintrag.
const pastSearch = ref('')
const pastAsc = ref(false)
const searchTerm = computed(() => pastSearch.value.trim().toLowerCase())
const pastMatches = (r: Row) => {
  const q = searchTerm.value
  if (!q) return true
  return `${r.date} ${label(r.date)} ${r.answers.join(' ')} ${r.other}`.toLowerCase().includes(q)
}
const visiblePast = computed(() => {
  const rows = pastRows.value.filter(pastMatches)
  return pastAsc.value ? [...rows].reverse() : rows
})

const groups = computed(() => {
  const out: { month: string; rows: Row[] }[] = []
  for (const r of visiblePast.value) {
    const month = r.date.slice(0, 7)
    const last = out[out.length - 1]
    if (last?.month === month) last.rows.push(r)
    else out.push({ month, rows: [r] })
  }
  return out
})
const shownCount = computed(() => visiblePast.value.length + (todayEntry.value ? 1 : 0))
const hasOlder = computed(() => !searchTerm.value && total.value > shownCount.value)

const completedTasks = computed(() => allTasks.value
  .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) === todayStr.value)
  .sort((a, b) => (a.completed_at ?? '').localeCompare(b.completed_at ?? '')))

const recapText = computed(() => {
  const parts: string[] = []
  if (completedTasks.value.length) {
    parts.push(completedTasks.value.map((t) => t.title).join(', '))
  }
  if (completedHabits.value.length) {
    parts.push(tn(completedHabits.value.length, '{n} Gewohnheit', '{n} Gewohnheiten'))
  }
  return parts.join(' · ')
})

// Aus dem Journal übernommen und noch offen
const openJournalTasks = computed(() => allTasks.value
  .filter((t) => t.source_journal_date && isOpen(t))
  .sort((a, b) => (a.due_at ?? '').localeCompare(b.due_at ?? '')))

interface HeatCell { s: string; has: boolean; future: boolean; label: string }
const heatCells = computed<HeatCell[]>(() => {
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
// Ein Tab-Stopp fürs Raster; Pfeiltasten wandern tag- und wochenweise.
const heatFocus = ref('')
const heatCount = computed(() => heatCells.value.filter((c) => c.has).length)

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
    heatFocus.value = summary.date
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
  await Promise.all([todayEditor.value?.flush(), pastEditor.value?.[0]?.flush()])
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

// ---- Verpasste Tage nachtragen ----
const backfillOpen = ref(false)
const backfillDate = ref('')

function openBackfill() {
  backfillDate.value = shiftDate(todayStr.value, -1)
  backfillOpen.value = true
}
function closeBackfill() {
  backfillOpen.value = false
}

function scrollToEl(el: HTMLElement | null | undefined, block: ScrollLogicalPosition = 'center') {
  if (!el) return
  const reduce = document.documentElement.dataset.motion === 'reduce' || matchMedia('(prefers-reduced-motion: reduce)').matches
  el.scrollIntoView({ block, behavior: reduce ? 'auto' : 'smooth' })
}

async function reveal(date: string) {
  await switchEntry(date)
  await nextTick()
  scrollToEl(pastEl.value?.querySelector<HTMLElement>(`[data-date="${date}"]`))
}

// Öffnet einen Tag: vorhandene Zeile aufklappen, ältere aus dem Notizordner nachladen
// oder einen leeren Entwurf anlegen. Eine Datei schreibt erst der Editor beim Tippen.
async function openDay(date: string) {
  if (!pastRows.value.some((r) => r.date === date)) {
    if (allDates.value.includes(date)) {
      try {
        const n = await readNote(journalPath(date))
        const e = parseEntry(n.content)
        const row = { date, answers: e.answers, other: e.other, mtime: n.mtime }
        pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
      } catch (e) {
        error.value = errorMessage(e)
        return
      }
    } else {
      const row = { date, answers: ['', '', ''], other: '', mtime: '' }
      pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
    }
  }
  await reveal(date)
}

async function startBackfill() {
  const date = backfillDate.value
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || date > todayStr.value) return
  backfillOpen.value = false
  await openDay(date)
}

// ---- Heatmap: Tag antippen ----
function heatAction(c: HeatCell) {
  return c.has ? t('Eintrag öffnen') : t('Eintrag nachtragen')
}
function heatTitle(c: HeatCell) {
  return c.future ? c.label : `${c.label} · ${heatAction(c)}`
}
function heatAria(c: HeatCell) {
  return c.future ? c.label : `${c.label}: ${heatAction(c)}`
}

async function openHeat(c: HeatCell) {
  if (c.future) return
  if (c.s === todayStr.value) {
    scrollToEl(todayEl.value, 'start')
    return
  }
  if (!c.has) {
    // Leerer Tag: erst bestätigen, dann als Entwurf öffnen.
    backfillDate.value = c.s
    backfillOpen.value = true
    return
  }
  await openDay(c.s)
}

function onHeatKey(e: KeyboardEvent, c: HeatCell) {
  // Das Raster läuft spaltenweise: eine Spalte ist eine Woche, eine Zeile ein Wochentag.
  const step = e.key === 'ArrowLeft' ? -7 : e.key === 'ArrowRight' ? 7 : e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0
  if (!step) return
  e.preventDefault()
  const cells = heatCells.value
  const from = cells.findIndex((x) => x.s === c.s)
  let max = cells.length - 1
  while (max > 0 && cells[max]!.future) max--
  const to = Math.min(max, Math.max(0, from + step))
  heatFocus.value = cells[to]!.s
  const buttons = (e.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>('.jr-heat-cell')
  buttons?.[to]?.focus()
}

function onTodayImported(result: JournalTaskImport) {
  taskImports.value = { ...taskImports.value, [result.journal_date]: { count: result.task_count, titles: result.titles } }
  void refreshTasks()
}

function onPastSaved(date: string, entry: JournalEntryData) {
  pastRows.value = pastRows.value.map((r) => (r.date === date ? { ...r, ...entry } : r))
  // Ein nachgetragener Tag erscheint sofort in Zähler und Heatmap.
  if (!allDates.value.includes(date)) {
    allDates.value = [...allDates.value, date].sort().reverse()
    total.value++
  }
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

// Aufgabe direkt erledigen; danach wandert sie in den Tagesrückblick.
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
      <!-- Kopf -->
      <header class="jr-head">
        <div class="jr-head-kicker">
          <span class="jr-kicker-badge">{{ $t('Heute') }}</span>
          <span class="jr-kicker-sep">·</span>
          <span class="jr-kicker-count">{{ $tn(total, '{n} Eintrag', '{n} Einträge') }}</span>
        </div>
        <h1 class="jr-title">{{ todayStr ? fmt(headerDateFmt, todayStr) : $t('Journal') }}</h1>

        <!-- Streak & Heatmap -->
        <div class="jr-meta">
          <span v-if="streak" class="jr-streak">{{ $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge') }}</span>
          <div class="jr-heat" role="group" :aria-label="$tn(heatCount, '{n} Eintrag in den letzten 12 Wochen', '{n} Einträge in den letzten 12 Wochen')">
            <button
              v-for="c in heatCells" :key="c.s" type="button" class="jr-heat-cell" :data-date="c.s"
              :class="{ on: c.has, future: c.future }" :disabled="c.future" :tabindex="c.s === heatFocus ? 0 : -1"
              :title="heatTitle(c)" :aria-label="heatAria(c)"
              @focus="heatFocus = c.s" @click="openHeat(c)" @keydown="onHeatKey($event, c)"
            ></button>
          </div>
          <span class="sr-only">{{ $t('Tag mit Eintrag') }}</span>
        </div>
      </header>

      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <!-- Heute -->
      <section v-if="todayStr" ref="todayEl" class="jr-today">
        <JournalEntryEditor
          ref="todayEditor"
          :date="todayStr"
          :entry="todayEntry"
          :imported-titles="importedTitles(todayStr)"
          @saved="onTodaySaved"
          @imported="onTodayImported"
        />

        <!-- Tagesrückblick -->
        <div v-if="recapText" class="jr-recap">
          <span class="jr-recap-lead">{{ $t('Heute erledigt:') }}</span> {{ recapText }}
        </div>
      </section>

      <!-- Offene Journal-Aufgaben -->
      <section v-if="openJournalTasks.length" class="jr-open-box">
        <div class="jr-open-head">
          <h3 class="jr-open-title">{{ $t('Noch offen aus dem Journal') }}</h3>
          <RouterLink class="jr-open-all" :to="{ path: '/tasks', query: { filter: 'journal' } }">{{ $t('Alle Aufgaben') }}<svg class="ic"><use href="#i-right" /></svg></RouterLink>
        </div>
        <ul class="jr-open-list">
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
        <RouterLink v-if="openJournalTasks.length > 5" class="jr-open-more" :to="{ path: '/tasks', query: { filter: 'journal' } }">
          {{ $tn(openJournalTasks.length - 5, '+{n} weitere Aufgabe', '+{n} weitere Aufgaben') }}
        </RouterLink>
      </section>

      <!-- Vergangene Einträge -->
      <section ref="pastEl" class="jr-past">
        <div class="jr-past-bar">
          <h2 class="jr-past-heading">{{ $t('Vergangene Einträge') }}</h2>
          <button type="button" class="col-link jr-backfill-btn" @click="openBackfill">{{ $t('Nachtragen') }}</button>
          <div v-if="pastRows.length" class="seg jr-seg" role="group" :aria-label="$t('Sortierung')">
            <button type="button" :aria-pressed="!pastAsc" @click="pastAsc = false">{{ $t('Neueste zuerst') }}</button>
            <button type="button" :aria-pressed="pastAsc" @click="pastAsc = true">{{ $t('Älteste zuerst') }}</button>
          </div>
          <SearchField v-if="pastRows.length" v-model="pastSearch" :label="$t('Einträge durchsuchen')" />
        </div>

        <div v-if="loaded && !pastRows.length" class="jr-empty">
          <p class="jr-empty-t">{{ $t('Drei kurze Fragen am Abend.') }}</p>
          <p class="v-sub">{{ $t('Was lief gut, was lernst du daraus, was ist morgen wichtig. Zwei Minuten reichen.') }}</p>
        </div>
        <p v-else-if="pastRows.length && !visiblePast.length" class="v-sub jr-nohit">{{ $t('Keine Treffer für „{q}“.', { q: pastSearch.trim() }) }}</p>

        <div v-else-if="pastRows.length" class="jr-months">
          <section v-for="g in groups" :key="g.month" class="jr-month-group">
            <h3 class="jr-month">{{ monthLabel(g.month) }}</h3>
            <div class="jr-feed">
              <article v-for="r in g.rows" :key="r.date" class="jr-entry" :data-date="r.date">
                <header class="jr-entry-head">
                  <button
                    type="button"
                    class="jr-row"
                    :aria-expanded="expandedDate === r.date"
                    :aria-label="expandedDate === r.date ? $t('Eintrag vom {date} schließen', { date: label(r.date) }) : $t('Eintrag vom {date} bearbeiten', { date: label(r.date) })"
                    @click="toggle(r.date)"
                  >
                    <span class="jr-date">{{ entryDateLabel(r.date) }}</span>
                    <span class="jr-edit-action">{{ expandedDate === r.date ? $t('Fertig') : $t('Bearbeiten') }}</span>
                  </button>
                </header>

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

                <div v-else class="jr-entry-body" role="button" tabindex="0" :aria-label="$t('Eintrag bearbeiten')" @click="toggle(r.date)" @keydown.enter="toggle(r.date)">
                  <div v-if="r.answers[0]?.trim()" class="jr-feed-section">
                    <div class="jr-feed-q">{{ $t('Was lief gut?') }}</div>
                    <div class="jr-feed-a">{{ r.answers[0] }}</div>
                  </div>
                  <div v-if="r.answers[1]?.trim()" class="jr-feed-section">
                    <div class="jr-feed-q">{{ $t('Was lerne ich daraus?') }}</div>
                    <div class="jr-feed-a">{{ r.answers[1] }}</div>
                  </div>
                  <div v-if="r.answers[2]?.trim()" class="jr-feed-section">
                    <div class="jr-feed-q">{{ $t('Was ist morgen wichtig?') }}</div>
                    <ul v-if="splitPoints(r.answers[2]).length" class="jr-feed-points">
                      <li v-for="(p, i) in splitPoints(r.answers[2])" :key="i">{{ p }}</li>
                    </ul>
                    <div v-else class="jr-feed-a">{{ r.answers[2] }}</div>
                  </div>
                  <div v-if="r.other?.trim()" class="jr-feed-section">
                    <div class="jr-feed-other">{{ r.other }}</div>
                  </div>
                </div>
              </article>
            </div>
          </section>
        </div>

        <RouterLink v-if="hasOlder" class="v-sub jr-more" to="/notes">{{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}</RouterLink>
      </section>

      <!-- Nachtragen-Dialog -->
      <div v-if="backfillOpen" v-dialog="closeBackfill" class="overlay open" @mousedown.self="closeBackfill">
        <form class="dialog jr-backfill" :aria-label="$t('Eintrag nachtragen')" @submit.prevent="startBackfill">
          <div class="dlg-head">
            <h3>{{ $t('Eintrag nachtragen') }}</h3>
            <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="closeBackfill"><svg class="ic"><use href="#i-x" /></svg></button>
          </div>
          <div class="dlg-body">
            <label class="field"><span>{{ $t('Datum') }}</span><input v-model="backfillDate" class="input" type="date" :max="todayStr" required /></label>
          </div>
          <div class="dlg-foot">
            <button class="btn btn-ghost" type="button" @click="closeBackfill">{{ $t('Abbrechen') }}</button>
            <button class="btn btn-primary" type="submit">{{ $t('Öffnen') }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.jr-page {
  max-width: 640px;
  margin: 0 auto;
  width: 100%;
}

/* Kopf */
.jr-head {
  margin-bottom: 32px;
}
.jr-head-kicker {
  display: flex;
  align-items: center;
  gap: 8px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
.jr-kicker-sep {
  opacity: 0.5;
}
.jr-title {
  margin: 6px 0 0;
  font: 400 38px/1.15 var(--font-serif);
  color: var(--tx-primary);
  letter-spacing: -0.01em;
}

/* Streak / Heatmap */
.jr-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 14px;
}
.jr-streak {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-heat {
  display: grid;
  grid-template-rows: repeat(7, 5px);
  grid-template-columns: repeat(12, 5px);
  grid-auto-flow: column;
  gap: 3px;
  align-items: center;
}
.jr-heat-cell {
  width: 5px;
  height: 5px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: var(--br-default);
  opacity: 0.35;
  cursor: pointer;
  position: relative;
  transition: opacity var(--dur) ease-out, transform var(--dur) ease-out;
}
.jr-heat-cell::before {
  content: '';
  position: absolute;
  inset: -3px;
}
.jr-heat-cell.on {
  background: var(--p-orange);
  opacity: 0.9;
}
.jr-heat-cell.future {
  background: var(--br-subtle);
  opacity: 0.15;
  cursor: default;
}
.jr-heat-cell:not(.future):hover {
  opacity: 1;
  transform: scale(1.3);
}
.jr-heat-cell:focus-visible {
  outline: 2px solid var(--tx-secondary);
  outline-offset: 2px;
}

/* Screenreader */
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

/* Heute */
.jr-today {
  margin-bottom: 48px;
}

/* Tagesrückblick */
.jr-recap {
  margin-top: 24px;
  font: 400 14px/1.6 var(--font-ui);
  color: var(--tx-muted);
}
.jr-recap-lead {
  color: var(--tx-secondary);
  font-weight: 500;
}

/* Offene Aufgaben */
.jr-open-box {
  margin-bottom: 48px;
  padding: 16px 20px;
  border-radius: var(--r-m);
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
}
.jr-open-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.jr-open-title {
  margin: 0;
  font: 500 13px/1.3 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-open-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}
.jr-open-item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: start;
  gap: 10px;
}
.jr-open-cb {
  margin-top: 3px;
  cursor: pointer;
}
.jr-open-link {
  font: 400 14px/1.4 var(--font-ui);
  color: var(--tx-secondary);
  text-decoration: none;
}
.jr-open-link:hover {
  color: var(--tx-primary);
  text-decoration: underline;
}
.jr-open-more {
  display: block;
  margin-top: 10px;
  color: var(--tx-muted);
  font: 400 12px/1.3 var(--font-ui);
  text-decoration: none;
}
.jr-open-all {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--tx-muted);
  font: 500 12px/1 var(--font-ui);
  text-decoration: none;
  transition: color var(--dur) ease-out;
}
.jr-open-all:hover {
  color: var(--tx-secondary);
}
.jr-open-all .ic {
  width: 12px;
  height: 12px;
}

/* Vergangene Einträge */
.jr-past {
  margin-top: 16px;
}
.jr-past-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--br-subtle);
  margin-bottom: 24px;
}
.jr-past-bar :deep(.search-btn),
.jr-past-bar :deep(input[type="search"]) {
  margin-left: auto;
}
.jr-past-heading {
  margin: 0;
  font: 400 18px/1.2 var(--font-serif);
  color: var(--tx-secondary);
}
button.col-link {
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
  transition: color var(--dur) ease-out;
}
button.col-link:hover {
  color: var(--tx-primary);
  text-decoration: underline;
}
.jr-seg button {
  font-size: 12px;
  padding: 4px 10px;
}
.jr-past-bar :deep(input[type="search"]) {
  max-width: 200px;
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
  border-radius: 999px;
}
.jr-empty {
  display: grid;
  justify-items: start;
  gap: 8px;
  padding: 32px 0;
}
.jr-empty p {
  margin: 0;
}
.jr-empty-t {
  font: 400 20px/1.3 var(--font-serif);
  color: var(--tx-primary);
}
.jr-nohit {
  margin: 16px 0;
}

.jr-months {
  display: grid;
  gap: 40px;
}
.jr-month-group {
  display: grid;
  gap: 16px;
}
.jr-month {
  margin: 0;
  font: italic 400 15px/1 var(--font-serif);
  color: var(--tx-muted);
}
.jr-feed {
  display: grid;
  gap: 28px;
}
.jr-entry {
  padding-bottom: 24px;
  border-bottom: 1px solid color-mix(in srgb, var(--br-subtle) 70%, transparent);
}
.jr-entry:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}
.jr-entry-head {
  display: flex;
  align-items: baseline;
}
.jr-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  text-align: left;
  color: inherit;
}
.jr-date {
  font: 400 20px/1.3 var(--font-serif);
  color: var(--tx-primary);
  letter-spacing: -0.01em;
}
.jr-edit-action {
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  opacity: 0;
  transition: opacity var(--dur) ease-out, color var(--dur) ease-out;
}
.jr-entry:hover .jr-edit-action,
.jr-row:focus-visible .jr-edit-action,
.jr-entry:focus-within .jr-edit-action {
  opacity: 1;
}
.jr-row:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 4px;
  border-radius: var(--r-s);
}

.jr-entry-body {
  margin-top: 14px;
  display: grid;
  gap: 14px;
  cursor: pointer;
}
.jr-entry-body:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 4px;
  border-radius: var(--r-s);
}
.jr-feed-section {
  display: grid;
  gap: 4px;
}
.jr-feed-q {
  font: italic 400 13px/1.3 var(--font-serif);
  color: var(--tx-muted);
}
.jr-feed-a {
  font: 400 15px/1.65 var(--font-ui);
  color: var(--tx-primary);
  white-space: pre-wrap;
}
.jr-feed-points {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 4px;
}
.jr-feed-points li {
  position: relative;
  padding-left: 14px;
  font: 400 15px/1.65 var(--font-ui);
  color: var(--tx-primary);
}
.jr-feed-points li::before {
  content: '';
  position: absolute;
  left: 2px;
  top: 0.65em;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--p-orange);
  opacity: 0.8;
}
.jr-feed-other {
  font: 400 14px/1.6 var(--font-ui);
  color: var(--tx-secondary);
  white-space: pre-wrap;
}

.jr-more {
  display: inline-block;
  margin-top: 24px;
  text-decoration: none;
}
.jr-more:hover {
  color: var(--tx-secondary);
  text-decoration: underline;
}

.jr-backfill {
  width: 320px;
  max-width: calc(100vw - 32px);
}

@media (max-width: 640px) {
  .jr-title {
    font-size: 28px;
  }
  .jr-meta {
    gap: 12px;
  }
  .jr-past-bar {
    gap: 8px;
  }
  .jr-past-tools {
    width: 100%;
    justify-content: space-between;
  }
  .jr-seg {
    transform: none;
  }
}
</style>
