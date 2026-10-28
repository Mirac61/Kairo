<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage, getJournalSummary, getToday, isOpen, listJournalTaskImports, listNotes, listTasks, readNote, type JournalTaskImport, type Task } from '@/api/client'
import { addDays, shiftDate, ymd } from '@/lib/dates'
import { journalPath, parseEntry, splitPoints } from '@/lib/journal'
import { locale, t, tn } from '@/lib/i18n'
import { vDialog } from '@/lib/dialog'
import { useUndo } from '@/composables/useUndo'
import { useShortcuts } from '@/composables/useShortcuts'
import { PAPERS, prefs } from '@/lib/prefs'
import { easeTurn, leafFrame, type LeafRole } from '@/lib/pageTurn'
import JournalEntryEditor, { type JournalEntryData } from '@/components/JournalEntryEditor.vue'
import SearchField from '@/components/SearchField.vue'

// So viele Einträge liegen als Seiten im Buch; ältere stehen unter Notizen.
const RECENT = 60
const HEAT_DAYS = 42
const PAPER_LABELS = { coffee: 'Kaffee', white: 'Weiß', black: 'Schwarz' } as const

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
// Aufgeschlagene Seite; dir bestimmt, in welche Richtung umgeblättert wird.
const pageDate = ref('')
const dir = ref<'fwd' | 'back'>('fwd')
// Der Umschlag klappt nur beim Öffnen der Ansicht auf; danach erscheinen linke Seiten ohne Verzögerung.
const opening = ref(true)
// Editor je Seite: Beim Umblättern sind kurz zwei montiert, gespeichert wird der der offenen Seite.
const editors = new Map<string, InstanceType<typeof JournalEntryEditor>>()
const setEditor = (date: string, c: unknown) => {
  if (c) editors.set(date, c as InstanceType<typeof JournalEntryEditor>)
  else editors.delete(date)
}

const headerDateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }))
const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const weekdayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long' }))
const monthFmt = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }))
const heatLabel = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))
const relativeLabel = (d: string) => (d === todayStr.value ? t('Heute') : d === shiftDate(todayStr.value, -1) ? t('Gestern') : '')
const label = (d: string) => relativeLabel(d) || fmt(dayFmt.value, d)
const monthLabel = (m: string) => fmt(monthFmt.value, `${m}-01`)

// Suche: Das Buch zeigt nur noch die passenden Seiten (und heute).
const pastSearch = ref('')
const searchTerm = computed(() => pastSearch.value.trim().toLowerCase())
const pastMatches = (r: Row) => {
  const q = searchTerm.value
  if (!q) return true
  return `${r.date} ${label(r.date)} ${r.answers.join(' ')} ${r.other}`.toLowerCase().includes(q)
}
const visiblePast = computed(() => pastRows.value.filter(pastMatches))

// Seiten wie im Heft: die älteste vorn, heute ist die letzte beschriebene Seite.
const pages = computed(() => [...visiblePast.value.map((r) => r.date).reverse(), ...(todayStr.value ? [todayStr.value] : [])])
const pageIdx = computed(() => pages.value.indexOf(pageDate.value))
const entryOf = (d: string) => (d === todayStr.value ? todayEntry.value : pastRows.value.find((r) => r.date === d) ?? null)
const pageKicker = (d: string) => relativeLabel(d) || monthLabel(d.slice(0, 7))
const shownCount = computed(() => visiblePast.value.length + (todayEntry.value ? 1 : 0))
const hasOlder = computed(() => !searchTerm.value && total.value > shownCount.value)

// Linke Seite: was an dem Tag erledigt wurde (Gewohnheiten kennt die Ansicht nur für heute)
// und was man sich am Vortag dafür vorgenommen hatte.
const doneOn = (d: string) => [
  ...allTasks.value
    .filter((t) => t.status === 'COMPLETED' && t.completed_at && ymd(new Date(t.completed_at)) === d)
    .sort((a, b) => (a.completed_at ?? '').localeCompare(b.completed_at ?? ''))
    .map((t) => t.title),
  ...(d === todayStr.value && completedHabits.value.length ? [tn(completedHabits.value.length, '{n} Gewohnheit', '{n} Gewohnheiten')] : []),
]
const planFor = (d: string) => splitPoints(entryOf(shiftDate(d, -1))?.answers[2] ?? '')

// Aus dem Journal übernommen und noch offen
const openJournalTasks = computed(() => allTasks.value
  .filter((t) => t.source_journal_date && isOpen(t))
  .sort((a, b) => (a.due_at ?? '').localeCompare(b.due_at ?? '')))

interface HeatCell { s: string; has: boolean; future: boolean; label: string }
const heatCells = computed<HeatCell[]>(() => {
  if (!todayStr.value) return []
  const set = new Set(allDates.value)
  const today = todayStr.value
  const start = addDays(new Date(`${today}T00:00:00`), -(HEAT_DAYS - 1))
  return Array.from({ length: HEAT_DAYS }, (_, i) => {
    const d = addDays(start, i)
    const s = ymd(d)
    return { s, has: set.has(s), future: s > today, label: heatLabel.value.format(d) }
  })
})
// Ein Tab-Stopp für die Reihe; Pfeiltasten wandern tageweise.
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
    pageDate.value = summary.date
    setTimeout(() => (opening.value = false), 1100)
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
      pageDate.value = route.query.date
    }
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)
watch(() => route.query.date, (date) => {
  if (typeof date === 'string' && (date === todayStr.value || pastRows.value.some((r) => r.date === date))) void go(date)
})

// Ohne Eingabe bleibt ein nachgetragener Tag ein leerer Entwurf; er verschwindet beim Schließen wieder.
const isDraft = (r: Row) => !r.mtime && !r.other.trim() && r.answers.every((a) => !a.trim())

// Erst die offene Seite fertig speichern, dann umblättern: So ist die Zeile in pastRows aktuell,
// bevor der Editor unmountet. Ein leerer Entwurf fällt beim Verlassen wieder heraus.
async function go(date: string) {
  const leaving = pageDate.value
  if (date === leaving) return
  await editors.get(leaving)?.flush()
  dir.value = pages.value.indexOf(date) < pages.value.indexOf(leaving) ? 'back' : 'fwd'
  turnStart = performance.now()
  turning = true
  setTimeout(() => {
    turning = false
    const step = queued
    queued = 0
    if (step) flip(step)
  }, TURN_MS)
  pageDate.value = date
  if (pastRows.value.some((r) => r.date === leaving && isDraft(r))) {
    pastRows.value = pastRows.value.filter((r) => r.date !== leaving)
  }
}

function flip(step: number) {
  if (turning) return void (queued = step) // erst fertig umblättern, dann weiter
  const next = pages.value[pageIdx.value + step]
  if (next) void go(next)
}

// Treffer der Suche aufschlagen: die neueste passende Seite, sonst heute.
watch(searchTerm, (q) => {
  const p = pages.value
  if (!p.includes(pageDate.value) || q) void go((q && p.length > 1 ? p[p.length - 2] : p[p.length - 1]) ?? todayStr.value)
})

useShortcuts((e) => {
  if (e.defaultPrevented) return // die Punktreihe nutzt die Pfeiltasten selbst
  const step = e.key === 'ArrowLeft' || e.key === 'PageUp' ? -1 : e.key === 'ArrowRight' || e.key === 'PageDown' ? 1 : 0
  if (!step) return
  e.preventDefault()
  flip(step)
})

// Am Handy blättert ein waagrechtes Wischen; in Textfeldern bleibt das Ziehen beim Text.
let swipe: { x: number; y: number } | null = null
function onSwipeStart(e: PointerEvent) {
  swipe = e.pointerType !== 'mouse' && !(e.target as HTMLElement).closest('input, textarea, button') ? { x: e.clientX, y: e.clientY } : null
}
function onSwipeEnd(e: PointerEvent) {
  if (!swipe) return
  const dx = e.clientX - swipe.x
  const dy = e.clientY - swipe.y
  swipe = null
  if (Math.abs(dx) > 60 && Math.abs(dx) > 2 * Math.abs(dy)) flip(dx < 0 ? 1 : -1)
}

// ---- Umblättern: alle vier beteiligten Seiten laufen auf derselben Zeitachse (lib/pageTurn.ts) ----
const TURN_MS = 950
let turnStart = 0
let turning = false
let queued = 0 // Klick während des Umblätterns
const reduceMotion = () => document.documentElement.dataset.motion === 'reduce' || matchMedia('(prefers-reduced-motion: reduce)').matches

function turnPage(side: 'l' | 'r', entering: boolean, el: Element, done: () => void) {
  const page = el as HTMLElement
  if (reduceMotion()) return done()
  // Schmal liegen die Seiten untereinander: überblenden statt falten.
  if (matchMedia('(max-width: 860px)').matches) {
    page.animate(entering ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }], { duration: 220, easing: 'ease-out' }).onfinish = done
    return
  }
  const back = dir.value === 'back'
  const role: LeafRole = side === (back ? 'l' : 'r') ? (entering ? 'revealed' : 'cut') : entering ? 'flap' : 'covered'
  const W = page.offsetWidth
  const H = page.offsetHeight
  const holder = page.parentElement as HTMLElement
  page.classList.add('jr-turning')
  page.style.zIndex = role === 'flap' || role === 'cut' ? '2' : '1'
  if (role === 'flap') holder.style.zIndex = '3' // die Rückseite reicht über den Bund
  const apply = (now: number) => {
    const t = Math.min(1, Math.max(0, (now - turnStart) / TURN_MS))
    const f = leafFrame(role, easeTurn(t), W, H, back)
    if (role === 'cut' || role === 'flap') page.style.clipPath = f.clip
    if (role === 'flap') page.style.transform = f.transform
    page.style.setProperty('--leaf', f.shade)
    return t
  }
  const frame = (now: number) => {
    if (apply(now) < 1) return void requestAnimationFrame(frame)
    page.classList.remove('jr-turning')
    page.style.clipPath = page.style.transform = page.style.zIndex = holder.style.zIndex = ''
    done()
  }
  apply(performance.now()) // sofort, sonst blitzt die neue Seite einen Frame lang ungeschnitten auf
  requestAnimationFrame(frame)
}

function onSaved(date: string, entry: JournalEntryData) {
  if (date === todayStr.value) onTodaySaved(entry)
  else onPastSaved(date, entry)
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

// Schlägt einen Tag auf: vorhandene Seite, ältere aus dem Notizordner nachladen
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
  await go(date)
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
    await go(c.s)
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
  const step = e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0
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

const menuOpen = ref(false)
</script>

<template>
  <div class="view-inner jr-wrap">
    <div class="jr-page">
      <!-- Kopf -->
      <header class="jr-head">
        <div class="jr-head-row">
          <div>
            <div class="jr-head-kicker">{{ $tn(total, '{n} Eintrag', '{n} Einträge') }}</div>
            <h1 class="jr-title">{{ $t('Journal') }}</h1>
          </div>
          <div class="menu-wrap" @keydown.esc="menuOpen = false">
            <button
              type="button" class="icon-btn jr-more-btn" :aria-label="$t('Weitere Optionen')"
              :aria-expanded="menuOpen" @click="menuOpen = !menuOpen"
            ><svg class="ic"><use href="#i-more" /></svg></button>
            <template v-if="menuOpen">
              <div class="menu-scrim" @click="menuOpen = false"></div>
              <div class="menu jr-paper-menu" role="menu">
                <span class="menu-l">{{ $t('Papier') }}</span>
                <div class="jr-papers" role="group" :aria-label="$t('Papier')">
                  <button
                    v-for="p in PAPERS" :key="p" type="button" class="jr-paper-pick" :class="`is-${p}`"
                    :aria-pressed="prefs.journalPaper === p" :aria-label="$t(PAPER_LABELS[p])" :title="$t(PAPER_LABELS[p])"
                    @click="prefs.journalPaper = p; menuOpen = false"
                  ></button>
                </div>
              </div>
            </template>
          </div>
        </div>

        <!-- Streak & Punktreihe -->
        <div class="jr-meta">
          <span v-if="streak" class="jr-streak">{{ $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge') }}</span>
          <div class="jr-heat" role="group" :aria-label="$tn(heatCount, '{n} Eintrag in den letzten 6 Wochen', '{n} Einträge in den letzten 6 Wochen')">
            <button
              v-for="c in heatCells" :key="c.s" type="button" class="jr-heat-cell" :data-date="c.s"
              :class="{ on: c.has, future: c.future, open: c.s === pageDate }" :disabled="c.future" :tabindex="c.s === heatFocus ? 0 : -1"
              :title="heatTitle(c)" :aria-label="heatAria(c)"
              @focus="heatFocus = c.s" @click="openHeat(c)" @keydown="onHeatKey($event, c)"
            ></button>
          </div>
          <span class="sr-only">{{ $t('Tag mit Eintrag') }}</span>
        </div>
      </header>

      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div class="jr-tools">
        <button type="button" class="col-link" @click="openBackfill">{{ $t('Nachtragen') }}</button>
        <SearchField v-if="pastRows.length" v-model="pastSearch" :label="$t('Einträge durchsuchen')" />
      </div>
      <p v-if="searchTerm && !visiblePast.length" class="v-sub jr-nohit">{{ $t('Keine Treffer für „{q}“.', { q: pastSearch.trim() }) }}</p>

      <!-- Das Buch: links der Rückblick des Tages, rechts die Schreibseite -->
      <div v-if="pageDate" class="jr-book" :data-paper="prefs.journalPaper">
        <div class="jr-spread" :class="{ 'jr-opening': opening }" @pointerdown="onSwipeStart" @pointerup="onSwipeEnd" @pointercancel="onSwipeEnd">
          <TransitionGroup tag="div" class="jr-side jr-side-l" :css="false" @enter="(el: Element, done: () => void) => turnPage('l', true, el, done)" @leave="(el: Element, done: () => void) => turnPage('l', false, el, done)">
            <section v-for="d in [pageDate]" :key="d" class="jr-paper jr-left" :data-date="d">
              <span class="jr-page-kicker">{{ pageKicker(d) }}</span>
              <div class="jr-day">{{ Number(d.slice(8)) }}</div>
              <div class="jr-day-sub">{{ fmt(weekdayFmt, d) }}<br>{{ monthLabel(d.slice(0, 7)) }}</div>
              <div v-if="planFor(d).length" class="jr-left-block">
                <h2 class="jr-left-h">{{ $t('Vorgenommen') }}</h2>
                <ul class="jr-left-list"><li v-for="(p, i) in planFor(d)" :key="i">{{ p }}</li></ul>
              </div>
              <div v-if="doneOn(d).length" class="jr-left-block jr-recap">
                <h2 class="jr-left-h">{{ $t('Erledigt') }}</h2>
                <ul class="jr-left-list"><li v-for="(p, i) in doneOn(d)" :key="i">{{ p }}</li></ul>
              </div>
              <span class="jr-page-no" aria-hidden="true">{{ 2 * pages.indexOf(d) + 1 }}</span>
            </section>
          </TransitionGroup>
          <TransitionGroup tag="div" class="jr-side jr-side-r" :css="false" @enter="(el: Element, done: () => void) => turnPage('r', true, el, done)" @leave="(el: Element, done: () => void) => turnPage('r', false, el, done)">
            <article
              v-for="d in [pageDate]" :key="d" class="jr-entry jr-paper jr-right" :class="{ 'jr-today': d === todayStr }" :data-date="d"
              :aria-label="fmt(headerDateFmt, d)"
            >
              <JournalEntryEditor
                :ref="(c) => setEditor(d, c)"
                :date="d"
                :entry="entryOf(d)"
                :imported-titles="importedTitles(d)"
                @saved="onSaved(d, $event)"
                @imported="onTodayImported"
              />
              <span class="jr-page-no" aria-hidden="true">{{ 2 * pages.indexOf(d) + 2 }}</span>
            </article>
          </TransitionGroup>
          <!-- Umschlag: liegt beim Öffnen der Ansicht über der rechten Seite und klappt auf -->
          <div class="jr-cover" aria-hidden="true"><span>{{ $t('Journal') }}</span></div>
        </div>

        <nav class="jr-nav" :aria-label="$t('Seiten')">
          <button type="button" class="icon-btn" :disabled="pageIdx <= 0" :aria-label="$t('Vorherige Seite')" @click="flip(-1)"><svg class="ic"><use href="#i-left" /></svg></button>
          <span class="jr-nav-count">{{ $t('Seite {n} von {m}', { n: pageIdx + 1, m: pages.length }) }}</span>
          <button type="button" class="icon-btn" :disabled="pageIdx >= pages.length - 1" :aria-label="$t('Nächste Seite')" @click="flip(1)"><svg class="ic"><use href="#i-right" /></svg></button>
        </nav>
        <RouterLink v-if="hasOlder && pageIdx === 0" class="v-sub jr-more" to="/notes">{{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}</RouterLink>
      </div>

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
.jr-wrap {
  padding-top: 18px;
  padding-bottom: 28px;
}
.jr-page {
  max-width: 1080px;
  margin: 0 auto;
  width: 100%;
}

/* Kopf */
.jr-head {
  margin-bottom: 16px;
}
.jr-head-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}
.jr-head-kicker {
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
.jr-title {
  margin: 6px 0 0;
  font: 400 38px/1.15 var(--font-serif);
  color: var(--tx-primary);
  letter-spacing: -0.01em;
}

/* Papierwahl: drei Farbfelder */
.jr-papers {
  display: flex;
  gap: 8px;
  padding-bottom: 6px;
}
.jr-paper-pick {
  width: 22px;
  height: 22px;
  padding: 0;
  border-radius: 50%;
  border: 1px solid var(--br-default);
  cursor: pointer;
  transition: transform var(--dur) ease-out, box-shadow var(--dur) ease-out;
}
.jr-paper-pick.is-coffee { background: #efe3c8; }
.jr-paper-pick.is-white { background: #fbfaf7; }
.jr-paper-pick.is-black { background: #1b1a18; }
.jr-paper-pick:hover { transform: scale(1.08); }
.jr-paper-pick[aria-pressed='true'] { box-shadow: 0 0 0 2px var(--bg-0), 0 0 0 4px var(--tx-secondary); }
.jr-paper-pick:focus-visible { outline: 2px solid var(--ring); outline-offset: 3px; }

/* Streak / Punktreihe */
.jr-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: 16px;
}
.jr-streak {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-heat {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.jr-heat-cell {
  width: 6px;
  height: 6px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: var(--tx-muted);
  opacity: 0.3;
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
.jr-heat-cell.open {
  outline: 1px solid var(--tx-secondary);
  outline-offset: 2px;
  opacity: 1;
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

.jr-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
  margin-bottom: 12px;
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
  color: var(--tx-secondary);
}
.jr-tools :deep(input[type="search"]) {
  max-width: 200px;
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
  border-radius: 999px;
}
.jr-nohit {
  margin: 0 0 12px;
}

/* Das Buch. Papier ist unabhängig vom App-Theme, wie ein echtes Heft. */
.jr-book {
  --paper: #efe3c8;
  --paper-2: #e2d1ad;
  --ink: #3a2a1b;
  --ink-2: #6a5440;
  --ink-3: #73634e;
  --rule: rgb(58 42 27 / 0.16);
  --dot: #b0602a;
  --cover: #6b4529;
  --cover-ink: #e9d3a8;
  --half: 400ms; /* eine Hälfte des Umblätterns: bis die Seite senkrecht steht */
  margin-bottom: 14px;
}
.jr-book[data-paper='white'] {
  --paper: #fbfaf7;
  --paper-2: #ebe8e1;
  --ink: #1d1c1a;
  --ink-2: #55524c;
  --ink-3: #75726c;
  --rule: rgb(29 28 26 / 0.12);
  --dot: #d0702e;
  --cover: #34404d;
  --cover-ink: #dfe4ea;
}
.jr-book[data-paper='black'] {
  --paper: #1d1c1a;
  --paper-2: #121110;
  --ink: #ece4d4;
  --ink-2: #b4ab9b;
  --ink-3: #8c8478;
  --rule: rgb(236 228 212 / 0.14);
  --dot: #e08a4a;
  --cover: #0b0b0a;
  --cover-ink: #b9a77f;
}

/* Der Einband schaut als schmaler, sauberer Rand um die Doppelseite. */
.jr-spread {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  padding: 8px 10px;
  animation: jr-shift 980ms cubic-bezier(0.5, 0, 0.2, 1) both;
}
/* Einbanddeckel hinter den Seiten mit dezentem, edlem Schatten */
.jr-spread::before,
.jr-spread::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  width: 50%;
  background: var(--cover);
  box-shadow: 0 24px 48px -12px rgb(0 0 0 / 0.6), inset 0 0 0 1px rgb(255 255 255 / 0.06);
}
.jr-spread::before {
  left: 0;
  border-radius: 7px 0 0 7px;
  animation: jr-board 980ms both;
}
.jr-spread::after {
  right: 0;
  border-radius: 0 7px 7px 0;
}
@keyframes jr-board {
  0%, 62% { opacity: 0; }
  62.01%, 100% { opacity: 1; }
}

/* Beide Seiten eines Blatts liegen beim Umblättern in derselben Zelle übereinander. */
.jr-side {
  position: relative;
  z-index: 1;
  display: grid;
}
.jr-side-l { grid-area: 1 / 1; }
.jr-side-r { grid-area: 1 / 2; }

.jr-paper {
  grid-area: 1 / 1;
  position: relative;
  min-height: clamp(520px, calc(100dvh - 320px), 760px);
  padding: 38px 42px 46px;
  background: radial-gradient(ellipse 130% 95% at 50% 40%, var(--paper) 60%, var(--paper-2) 135%);
}
/* Bund in der Mitte mit sanftem Papierschatten, Buchblock mit dezentem Schnitt */
.jr-left {
  border-radius: 4px 0 0 4px;
  box-shadow:
    inset -20px 0 26px -12px rgb(0 0 0 / 0.22),
    -1px 1px 0 var(--paper-2),
    -2px 2px 0 var(--paper),
    -3px 3px 0 var(--paper-2),
    -4px 4px 0 var(--paper),
    -5px 5px 0 var(--paper-2),
    -8px 12px 20px -4px rgb(0 0 0 / 0.38);
}
.jr-right {
  border-radius: 0 4px 4px 0;
  box-shadow:
    inset 20px 0 26px -12px rgb(0 0 0 / 0.22),
    1px 1px 0 var(--paper-2),
    2px 2px 0 var(--paper),
    3px 3px 0 var(--paper-2),
    4px 4px 0 var(--paper),
    5px 5px 0 var(--paper-2),
    8px 12px 20px -4px rgb(0 0 0 / 0.38);
}
/* Der Editor schreibt mit Tinte: seine Farb-Tokens zeigen auf das Papier (der Import-Dialog bleibt unberührt). */
.jr-paper :deep(.je-form) {
  --tx-primary: var(--ink);
  --tx-secondary: var(--ink-2);
  --tx-muted: var(--ink-3);
  --tx-disabled: var(--ink-3);
  --br-default: var(--rule);
  --br-subtle: var(--rule);
  --ring: var(--ink-2);
  --p-orange: var(--dot);
}
.jr-paper :deep(.je-label) {
  font: 400 25px/1.2 var(--font-serif);
  color: var(--ink);
  letter-spacing: -0.005em;
}
.jr-paper :deep(.je-q) {
  gap: 4px;
}
.jr-paper :deep(.je-form) {
  gap: clamp(26px, 3.8vh, 38px);
}
.jr-paper :deep(.je-area) {
  min-height: 52px;
}
.jr-paper :deep(.je-area),
.jr-paper :deep(.je-point-input) {
  color: var(--ink-2);
}
.jr-paper :deep(.je-import:hover) {
  background: var(--rule);
}
.jr-page-kicker {
  font: italic 400 16px/1 var(--font-serif);
  color: var(--ink-3);
}
.jr-day {
  margin-top: 6px;
  font: 400 88px/0.9 var(--font-serif);
  color: var(--ink);
  font-variant-numeric: lining-nums;
}
.jr-day-sub {
  margin-top: 6px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--rule);
  font: italic 400 22px/1.25 var(--font-serif);
  color: var(--ink-2);
}
.jr-left-block {
  margin-top: 24px;
}
.jr-left-h {
  margin: 0 0 8px;
  font: 400 23px/1.2 var(--font-serif);
  color: var(--ink);
}
.jr-left-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 4px;
}
.jr-left-list li {
  position: relative;
  padding-left: 16px;
  font: 400 15px/1.6 var(--font-ui);
  color: var(--ink-2);
}
.jr-left-list li::before {
  content: '';
  position: absolute;
  left: 2px;
  top: 0.7em;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--dot);
  opacity: 0.75;
}
.jr-page-no {
  position: absolute;
  bottom: 20px;
  font: italic 400 14px/1 var(--font-serif);
  color: var(--ink-3);
}
.jr-left .jr-page-no { left: 28px; }
.jr-right .jr-page-no { right: 28px; }

/* Umschlag: klappt beim Öffnen am Bund nach links auf */
.jr-cover {
  grid-area: 1 / 2;
  z-index: 5;
  margin: -8px -10px -8px 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0 7px 7px 0;
  background: linear-gradient(90deg, rgb(0 0 0 / 0.28), transparent 7%), var(--cover);
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.06);
  transform-origin: left center;
  opacity: 0;
  pointer-events: none;
  animation: jr-cover 980ms cubic-bezier(0.5, 0, 0.2, 1) both;
}
.jr-cover span {
  font: italic 400 44px/1 var(--font-serif);
  color: var(--cover-ink);
  letter-spacing: 0.02em;
}
@keyframes jr-cover {
  0%, 25% { opacity: 1; transform: perspective(2400px) rotateY(0deg); }
  62% { opacity: 1; transform: perspective(2400px) rotateY(-90deg); filter: brightness(0.6); }
  62.01%, 100% { opacity: 0; transform: perspective(2400px) rotateY(-90deg); }
}
/* Geschlossen liegt das Buch mittig; beim Aufklappen rückt es nach links. */
@keyframes jr-shift {
  0%, 25% { transform: translateX(-25%); }
  100% { transform: none; }
}
.jr-opening .jr-left {
  animation: jr-appear 160ms 600ms ease-out both;
}
@keyframes jr-appear {
  from { opacity: 0; }
}

/* Umblättern: Clip, Lage und Schattierung jeder Seite setzt turnPage() pro Frame (lib/pageTurn.ts). */
.jr-paper::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  opacity: 0;
}
.jr-turning {
  pointer-events: none;
  transform-origin: 0 0;
}
.jr-turning::after {
  opacity: 1;
  background: var(--leaf);
}

.jr-nav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 10px;
}
.jr-more-btn { width: 32px; height: 32px; }
.jr-paper-menu { padding: 8px 10px; min-width: 140px; }
.jr-paper-menu .menu-l { display: block; padding: 2px 4px 6px; font: 600 11px/1 var(--font-ui); color: var(--tx-muted); text-transform: uppercase; letter-spacing: .06em; }
.jr-nav-count {
  min-width: 96px;
  text-align: center;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
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

.jr-more {
  display: block;
  margin-top: 12px;
  text-align: center;
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

@media (max-width: 860px) {
  /* Schmal liegen die Seiten untereinander; statt zu drehen blenden sie über. */
  .jr-spread {
    grid-template-columns: 1fr;
    padding: 6px;
    animation: none;
  }
  .jr-cover,
  .jr-ribbon,
  .jr-spread::before {
    display: none;
  }
  .jr-spread::after {
    width: 100%;
    border-radius: 8px;
  }
  .jr-side-r {
    grid-area: 2 / 1;
  }
  .jr-paper {
    min-height: 0;
  }
  .jr-left {
    padding: 24px 22px;
    border-radius: 4px 4px 0 0;
    box-shadow: inset 0 -20px 24px -22px rgb(0 0 0 / 0.3);
  }
  .jr-right {
    padding: 28px 22px 44px;
    border-radius: 0 0 4px 4px;
    box-shadow: inset 0 20px 24px -22px rgb(0 0 0 / 0.3);
  }
  .jr-left .jr-page-no,
  .jr-paper::after {
    display: none;
  }
  .jr-day {
    font-size: 64px;
  }
  .jr-day-sub {
    font-size: 18px;
    padding-bottom: 0;
    border-bottom: 0;
  }
  .jr-opening .jr-left {
    animation: none;
  }
}
@media (max-width: 640px) {
  .jr-title {
    font-size: 30px;
  }
  .jr-meta {
    gap: 12px;
  }
  /* Schmal passen nur vier Wochen in eine Zeile. */
  .jr-heat-cell:nth-child(-n + 14) {
    display: none;
  }
}
</style>
