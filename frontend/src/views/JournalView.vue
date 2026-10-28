<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import {
  errorMessage,
  getJournalSummary,
  getToday,
  isOpen,
  listJournalTaskImports,
  listNotes,
  listTasks,
  readNote,
  type JournalTaskImport,
  type Task,
} from '@/api/client'
import { shiftDate, ymd } from '@/lib/dates'
import { journalPath, parseEntry, splitPoints } from '@/lib/journal'
import { locale, t, tn } from '@/lib/i18n'
import { useUndo } from '@/composables/useUndo'
import { useShortcuts } from '@/composables/useShortcuts'
import { PAPERS, prefs } from '@/lib/prefs'
import { easeTurn, leafFrame, type LeafRole } from '@/lib/pageTurn'
import JournalEntryEditor, { type JournalEntryData } from '@/components/JournalEntryEditor.vue'
import JournalHeatmap from '@/components/JournalHeatmap.vue'
import JournalOpenTasks from '@/components/JournalOpenTasks.vue'
import JournalBackfillDialog from '@/components/JournalBackfillDialog.vue'
import SearchField from '@/components/SearchField.vue'

// So viele Einträge liegen als Seiten im Buch; ältere stehen unter Notizen.
const RECENT_ENTRIES_COUNT = 60
const TURN_ANIMATION_MS = 950

const PAPER_LABELS: Record<'coffee' | 'white' | 'black', string> = {
  coffee: t('Kaffee'),
  white: t('Weiß'),
  black: t('Schwarz'),
}

const route = useRoute()

interface JournalRow {
  date: string
  answers: string[]
  other: string
  mtime: string
}

type DoneTask = Task & { completed_at: string | null }

const todayStr = ref('')
const totalEntriesCount = ref(0)
const currentStreak = ref(0)
const isLoaded = ref(false)
const errorMessageText = ref('')
const allDates = ref<string[]>([])
const pastRows = ref<JournalRow[]>([])
const todayEntry = ref<JournalEntryData | null>(null)
const taskImports = ref<Record<string, { count: number; titles: string[] }>>({})
const allTasks = ref<DoneTask[]>([])
const completedHabits = ref<string[]>([])

// Aufgeschlagene Seite; dir bestimmt, in welche Richtung umgeblättert wird.
const pageDate = ref('')
const flipDirection = ref<'fwd' | 'back'>('fwd')

// Der Umschlag klappt nur beim Öffnen der Ansicht auf; danach erscheinen linke Seiten ohne Verzögerung.
const isCoverOpening = ref(true)

// Editor je Seite: Beim Umblättern sind kurz zwei montiert, gespeichert wird der der offenen Seite.
const editorInstances = new Map<string, InstanceType<typeof JournalEntryEditor>>()
function setEditorInstance(date: string, instance: unknown) {
  if (instance) {
    editorInstances.set(date, instance as InstanceType<typeof JournalEntryEditor>)
  } else {
    editorInstances.delete(date)
  }
}

// Datumsformate
const headerDateFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    })
)
const dayFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
    })
)
const weekdayFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { weekday: 'long' })
)
const monthFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' })
)

function formatWith(formatter: Intl.DateTimeFormat, dateStr: string): string {
  return formatter.format(new Date(`${dateStr}T00:00`))
}

function getRelativeDateLabel(dateStr: string): string {
  if (dateStr === todayStr.value) return t('Heute')
  if (dateStr === shiftDate(todayStr.value, -1)) return t('Gestern')
  return ''
}

function getDayLabel(dateStr: string): string {
  return getRelativeDateLabel(dateStr) || formatWith(dayFmt.value, dateStr)
}

function getMonthLabel(yearMonthStr: string): string {
  return formatWith(monthFmt.value, `${yearMonthStr}-01`)
}

// Suche
const pastSearch = ref('')
const normalizedSearchTerm = computed(() => pastSearch.value.trim().toLowerCase())

function matchesSearch(row: JournalRow): boolean {
  const query = normalizedSearchTerm.value
  if (!query) return true
  const haystack = `${row.date} ${getDayLabel(row.date)} ${row.answers.join(' ')} ${row.other}`.toLowerCase()
  return haystack.includes(query)
}

const visiblePastRows = computed(() => pastRows.value.filter(matchesSearch))

// Seiten wie im Heft: die älteste vorn, heute ist die letzte beschriebene Seite.
const pages = computed(() => [
  ...visiblePastRows.value.map((r) => r.date).reverse(),
  ...(todayStr.value ? [todayStr.value] : []),
])

const currentPageIndex = computed(() => pages.value.indexOf(pageDate.value))

function getEntryForDate(d: string): JournalRow | JournalEntryData | null {
  if (d === todayStr.value) return todayEntry.value
  return pastRows.value.find((r) => r.date === d) ?? null
}

function getPageKicker(dateStr: string): string {
  return getRelativeDateLabel(dateStr) || getMonthLabel(dateStr.slice(0, 7))
}

const shownEntriesCount = computed(
  () => visiblePastRows.value.length + (todayEntry.value ? 1 : 0)
)
const hasOlderEntries = computed(
  () => !normalizedSearchTerm.value && totalEntriesCount.value > shownEntriesCount.value
)

// Linke Seite: erledigte Aufgaben und Gewohnheiten, plus Vorhaben vom Vortag
function getDoneItemsOn(dateStr: string): string[] {
  const tasksCompleted = allTasks.value
    .filter(
      (t) =>
        t.status === 'COMPLETED' &&
        t.completed_at &&
        ymd(new Date(t.completed_at)) === dateStr
    )
    .sort((a, b) => (a.completed_at ?? '').localeCompare(b.completed_at ?? ''))
    .map((t) => t.title)

  const habitsCompleted =
    dateStr === todayStr.value && completedHabits.value.length
      ? [tn(completedHabits.value.length, '{n} Gewohnheit', '{n} Gewohnheiten')]
      : []

  return [...tasksCompleted, ...habitsCompleted]
}

function getPlanForDate(dateStr: string): string[] {
  const yesterdayStr = shiftDate(dateStr, -1)
  const yesterdayEntry = getEntryForDate(yesterdayStr)
  return splitPoints(yesterdayEntry?.answers[2] ?? '')
}

// Aus dem Journal übernommen und noch offen
const openJournalTasks = computed(() =>
  allTasks.value
    .filter((t) => t.source_journal_date && isOpen(t))
    .sort((a, b) => (a.due_at ?? '').localeCompare(b.due_at ?? ''))
)

function getImportedTitles(dateStr: string): string[] {
  return taskImports.value[dateStr]?.titles ?? []
}

async function readEntries(dates: string[]): Promise<JournalRow[]> {
  return Promise.all(
    dates.map(async (date) => {
      const note = await readNote(journalPath(date))
      const entry = parseEntry(note.content)
      return {
        date,
        answers: entry.answers,
        other: entry.other,
        mtime: note.mtime,
      }
    })
  )
}

async function loadData() {
  try {
    const [notes, imports, summary, todayData, tasks] = await Promise.all([
      listNotes(),
      listJournalTaskImports(),
      getJournalSummary(),
      getToday(),
      listTasks(),
    ])

    const journalDir = notes.find((n) => n.dir && n.path === 'Journal')
    const dates = (journalDir?.children ?? [])
      .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
      .filter((d): d is string => !!d)
      .sort()
      .reverse()

    const rows = await readEntries(dates.slice(0, RECENT_ENTRIES_COUNT))
    todayStr.value = summary.date
    pageDate.value = summary.date
    setTimeout(() => {
      isCoverOpening.value = false
    }, 1100)

    totalEntriesCount.value = summary.total
    currentStreak.value = summary.streak
    allDates.value = dates
    taskImports.value = Object.fromEntries(
      imports.map((i) => [i.journal_date, { count: i.task_count, titles: i.titles }])
    )
    todayEntry.value = rows.find((r) => r.date === summary.date) ?? null
    pastRows.value = rows.filter((r) => r.date !== summary.date)

    allTasks.value = tasks as DoneTask[]
    completedHabits.value = todayData.habits.filter((h) => h.done).map((h) => h.name)

    const routeDate = route.query.date
    if (
      typeof routeDate === 'string' &&
      routeDate !== summary.date &&
      pastRows.value.some((r) => r.date === routeDate)
    ) {
      pageDate.value = routeDate
    }

    errorMessageText.value = ''
  } catch (e) {
    errorMessageText.value = errorMessage(e)
  }
  isLoaded.value = true
}

onMounted(loadData)

watch(
  () => route.query.date,
  (newDate) => {
    if (
      typeof newDate === 'string' &&
      (newDate === todayStr.value || pastRows.value.some((r) => r.date === newDate))
    ) {
      void goToDate(newDate)
    }
  }
)

function isDraftRow(row: JournalRow): boolean {
  return !row.mtime && !row.other.trim() && row.answers.every((a) => !a.trim())
}

let turnStartTime = 0
let isTurningPage = false
let queuedPageStep = 0

async function goToDate(targetDate: string) {
  const leavingDate = pageDate.value
  if (targetDate === leavingDate) return

  await editorInstances.get(leavingDate)?.flush()

  flipDirection.value =
    pages.value.indexOf(targetDate) < pages.value.indexOf(leavingDate) ? 'back' : 'fwd'
  turnStartTime = performance.now()
  isTurningPage = true

  setTimeout(() => {
    isTurningPage = false
    const step = queuedPageStep
    queuedPageStep = 0
    if (step) flipPage(step)
  }, TURN_ANIMATION_MS)

  pageDate.value = targetDate

  if (pastRows.value.some((r) => r.date === leavingDate && isDraftRow(r))) {
    pastRows.value = pastRows.value.filter((r) => r.date !== leavingDate)
  }
}

function flipPage(step: number) {
  if (isTurningPage) {
    queuedPageStep = step
    return
  }
  const nextDate = pages.value[currentPageIndex.value + step]
  if (nextDate) void goToDate(nextDate)
}

// Treffer der Suche aufschlagen
watch(normalizedSearchTerm, (query) => {
  const currentPages = pages.value
  if (!currentPages.includes(pageDate.value) || query) {
    const target =
      (query && currentPages.length > 1
        ? currentPages[currentPages.length - 2]
        : currentPages[currentPages.length - 1]) ?? todayStr.value
    void goToDate(target)
  }
})

useShortcuts((e) => {
  if (e.defaultPrevented) return
  const step =
    e.key === 'ArrowLeft' || e.key === 'PageUp'
      ? -1
      : e.key === 'ArrowRight' || e.key === 'PageDown'
        ? 1
        : 0
  if (!step) return
  e.preventDefault()
  flipPage(step)
})

let touchSwipe: { x: number; y: number } | null = null
function onSwipeStart(e: PointerEvent) {
  touchSwipe =
    e.pointerType !== 'mouse' &&
    !(e.target as HTMLElement).closest('input, textarea, button')
      ? { x: e.clientX, y: e.clientY }
      : null
}

function onSwipeEnd(e: PointerEvent) {
  if (!touchSwipe) return
  const dx = e.clientX - touchSwipe.x
  const dy = e.clientY - touchSwipe.y
  touchSwipe = null

  if (Math.abs(dx) > 60 && Math.abs(dx) > 2 * Math.abs(dy)) {
    flipPage(dx < 0 ? 1 : -1)
  }
}

function isReducedMotion(): boolean {
  return (
    document.documentElement.dataset.motion === 'reduce' ||
    matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

function turnPage(side: 'l' | 'r', entering: boolean, el: Element, done: () => void) {
  const page = el as HTMLElement
  if (isReducedMotion()) return done()

  if (matchMedia('(max-width: 860px)').matches) {
    page.animate(
      entering ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }],
      { duration: 220, easing: 'ease-out' }
    ).onfinish = done
    return
  }

  const isBack = flipDirection.value === 'back'
  const role: LeafRole =
    side === (isBack ? 'l' : 'r')
      ? entering
        ? 'revealed'
        : 'cut'
      : entering
        ? 'flap'
        : 'covered'

  const width = page.offsetWidth
  const height = page.offsetHeight
  const holder = page.parentElement as HTMLElement

  page.classList.add('jr-turning')
  page.style.zIndex = role === 'flap' || role === 'cut' ? '2' : '1'
  if (role === 'flap') holder.style.zIndex = '3'

  const applyFrame = (now: number) => {
    const t = Math.min(1, Math.max(0, (now - turnStartTime) / TURN_ANIMATION_MS))
    const frame = leafFrame(role, easeTurn(t), width, height, isBack)
    if (role === 'cut' || role === 'flap') page.style.clipPath = frame.clip
    if (role === 'flap') page.style.transform = frame.transform
    page.style.setProperty('--leaf', frame.shade)
    return t
  }

  const renderFrame = (now: number) => {
    if (applyFrame(now) < 1) {
      requestAnimationFrame(renderFrame)
      return
    }
    page.classList.remove('jr-turning')
    page.style.clipPath = page.style.transform = page.style.zIndex = holder.style.zIndex = ''
    done()
  }

  applyFrame(performance.now())
  requestAnimationFrame(renderFrame)
}

function onEntrySaved(date: string, entry: JournalEntryData) {
  if (date === todayStr.value) {
    todayEntry.value = entry
    if (!allDates.value.includes(todayStr.value)) {
      allDates.value = [...allDates.value, todayStr.value].sort().reverse()
    }
  } else {
    pastRows.value = pastRows.value.map((r) => (r.date === date ? { ...r, ...entry } : r))
    if (!allDates.value.includes(date)) {
      allDates.value = [...allDates.value, date].sort().reverse()
      totalEntriesCount.value++
    }
  }
}

// Nachtragen
const isBackfillOpen = ref(false)
const backfillDate = ref('')

function openBackfillModal(defaultDate?: string) {
  backfillDate.value = defaultDate || shiftDate(todayStr.value, -1)
  isBackfillOpen.value = true
}

async function openDay(date: string) {
  if (!pastRows.value.some((r) => r.date === date)) {
    if (allDates.value.includes(date)) {
      try {
        const n = await readNote(journalPath(date))
        const e = parseEntry(n.content)
        const row = { date, answers: e.answers, other: e.other, mtime: n.mtime }
        pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
      } catch (e) {
        errorMessageText.value = errorMessage(e)
        return
      }
    } else {
      const row = { date, answers: ['', '', ''], other: '', mtime: '' }
      pastRows.value = [...pastRows.value, row].sort((a, b) => b.date.localeCompare(a.date))
    }
  }
  await goToDate(date)
}

async function handleBackfillSubmit(date: string) {
  isBackfillOpen.value = false
  await openDay(date)
}

function handleTodayImported(result: JournalTaskImport) {
  taskImports.value = {
    ...taskImports.value,
    [result.journal_date]: { count: result.task_count, titles: result.titles },
  }
  void refreshTasks()
}

async function refreshTasks() {
  try {
    allTasks.value = (await listTasks()) as DoneTask[]
  } catch (e) {
    errorMessageText.value = errorMessage(e)
  }
}

const { setDone } = useUndo()
const completingTaskId = ref('')

function handleCompleteTask(task: Task) {
  if (completingTaskId.value) return
  completingTaskId.value = task.id
  setDone(task, 'COMPLETED', async (fn) => {
    try {
      await fn()
      await refreshTasks()
    } catch (e) {
      errorMessageText.value = errorMessage(e)
    } finally {
      completingTaskId.value = ''
    }
  })
}

const isPaperMenuOpen = ref(false)
</script>

<template>
  <div class="view-inner jr-wrap">
    <div class="jr-page">
      <!-- Kopf -->
      <header class="jr-head">
        <div class="jr-head-row">
          <div>
            <div class="jr-head-kicker">
              {{ $tn(totalEntriesCount, '{n} Eintrag', '{n} Einträge') }}
            </div>
            <h1 class="jr-title">{{ $t('Journal') }}</h1>
          </div>

          <div class="menu-wrap" @keydown.esc="isPaperMenuOpen = false">
            <button
              type="button"
              class="icon-btn jr-more-btn"
              :aria-label="$t('Weitere Optionen')"
              :aria-expanded="isPaperMenuOpen"
              @click="isPaperMenuOpen = !isPaperMenuOpen"
            >
              <svg class="ic"><use href="#i-more" /></svg>
            </button>

            <template v-if="isPaperMenuOpen">
              <div class="menu-scrim" @click="isPaperMenuOpen = false"></div>
              <div class="menu jr-paper-menu" role="menu">
                <span class="menu-l">{{ $t('Papier') }}</span>
                <div class="jr-papers" role="group" :aria-label="$t('Papier')">
                  <button
                    v-for="p in PAPERS"
                    :key="p"
                    type="button"
                    class="jr-paper-pick"
                    :class="`is-${p}`"
                    :aria-pressed="prefs.journalPaper === p"
                    :aria-label="PAPER_LABELS[p]"
                    :title="PAPER_LABELS[p]"
                    @click="prefs.journalPaper = p; isPaperMenuOpen = false"
                  ></button>
                </div>
              </div>
            </template>
          </div>
        </div>

        <!-- Streak & Punktreihe -->
        <JournalHeatmap
          :all-dates="allDates"
          :today-str="todayStr"
          :active-date="pageDate"
          :streak="currentStreak"
          @select="openDay"
          @backfill="openBackfillModal"
        />
      </header>

      <div v-if="errorMessageText" class="badge" role="alert">{{ errorMessageText }}</div>

      <div class="jr-tools">
        <button type="button" class="col-link" @click="openBackfillModal()">
          {{ $t('Nachtragen') }}
        </button>
        <SearchField
          v-if="pastRows.length"
          v-model="pastSearch"
          :label="$t('Einträge durchsuchen')"
        />
      </div>

      <p v-if="normalizedSearchTerm && !visiblePastRows.length" class="v-sub jr-nohit">
        {{ $t('Keine Treffer für „{q}“.', { q: pastSearch.trim() }) }}
      </p>

      <!-- Das Buch: links der Rückblick des Tages, rechts die Schreibseite -->
      <div v-if="pageDate" class="jr-book" :data-paper="prefs.journalPaper">
        <div
          class="jr-spread"
          :class="{ 'jr-opening': isCoverOpening }"
          @pointerdown="onSwipeStart"
          @pointerup="onSwipeEnd"
          @pointercancel="onSwipeEnd"
        >
          <TransitionGroup
            tag="div"
            class="jr-side jr-side-l"
            :css="false"
            @enter="(el: Element, done: () => void) => turnPage('l', true, el, done)"
            @leave="(el: Element, done: () => void) => turnPage('l', false, el, done)"
          >
            <section v-for="d in [pageDate]" :key="d" class="jr-paper jr-left" :data-date="d">
              <span class="jr-page-kicker">{{ getPageKicker(d) }}</span>
              <div class="jr-day">{{ Number(d.slice(8)) }}</div>
              <div class="jr-day-sub">
                {{ formatWith(weekdayFmt, d) }}<br />{{ getMonthLabel(d.slice(0, 7)) }}
              </div>

              <div v-if="getPlanForDate(d).length" class="jr-left-block">
                <h2 class="jr-left-h">{{ $t('Vorgenommen') }}</h2>
                <ul class="jr-left-list">
                  <li v-for="(p, i) in getPlanForDate(d)" :key="i">{{ p }}</li>
                </ul>
              </div>

              <div v-if="getDoneItemsOn(d).length" class="jr-left-block jr-recap">
                <h2 class="jr-left-h">{{ $t('Erledigt') }}</h2>
                <ul class="jr-left-list">
                  <li v-for="(p, i) in getDoneItemsOn(d)" :key="i">{{ p }}</li>
                </ul>
              </div>

              <span class="jr-page-no" aria-hidden="true">{{ 2 * pages.indexOf(d) + 1 }}</span>
            </section>
          </TransitionGroup>

          <TransitionGroup
            tag="div"
            class="jr-side jr-side-r"
            :css="false"
            @enter="(el: Element, done: () => void) => turnPage('r', true, el, done)"
            @leave="(el: Element, done: () => void) => turnPage('r', false, el, done)"
          >
            <article
              v-for="d in [pageDate]"
              :key="d"
              class="jr-entry jr-paper jr-right"
              :class="{ 'jr-today': d === todayStr }"
              :data-date="d"
              :aria-label="formatWith(headerDateFmt, d)"
            >
              <JournalEntryEditor
                :ref="(c) => setEditorInstance(d, c)"
                :date="d"
                :entry="getEntryForDate(d)"
                :imported-titles="getImportedTitles(d)"
                @saved="onEntrySaved(d, $event)"
                @imported="handleTodayImported"
              />
              <span class="jr-page-no" aria-hidden="true">{{ 2 * pages.indexOf(d) + 2 }}</span>
            </article>
          </TransitionGroup>

          <!-- Umschlag: liegt beim Öffnen der Ansicht über der rechten Seite und klappt auf -->
          <div class="jr-cover" aria-hidden="true">
            <span>{{ $t('Journal') }}</span>
          </div>
        </div>

        <nav class="jr-nav" :aria-label="$t('Seiten')">
          <button
            type="button"
            class="icon-btn"
            :disabled="currentPageIndex <= 0"
            :aria-label="$t('Vorherige Seite')"
            @click="flipPage(-1)"
          >
            <svg class="ic"><use href="#i-left" /></svg>
          </button>
          <span class="jr-nav-count">
            {{ $t('Seite {n} von {m}', { n: currentPageIndex + 1, m: pages.length }) }}
          </span>
          <button
            type="button"
            class="icon-btn"
            :disabled="currentPageIndex >= pages.length - 1"
            :aria-label="$t('Nächste Seite')"
            @click="flipPage(1)"
          >
            <svg class="ic"><use href="#i-right" /></svg>
          </button>
        </nav>

        <RouterLink
          v-if="hasOlderEntries && currentPageIndex === 0"
          class="v-sub jr-more"
          to="/notes"
        >
          {{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}
        </RouterLink>
      </div>

      <!-- Offene Journal-Aufgaben -->
      <JournalOpenTasks
        :tasks="openJournalTasks"
        :completing-id="completingTaskId"
        @complete="handleCompleteTask"
      />

      <!-- Nachtragen-Dialog -->
      <JournalBackfillDialog
        :open="isBackfillOpen"
        :initial-date="backfillDate"
        :max-date="todayStr"
        @close="isBackfillOpen = false"
        @submit="handleBackfillSubmit"
      />
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
  transition:
    transform var(--dur) ease-out,
    box-shadow var(--dur) ease-out;
}
.jr-paper-pick.is-coffee {
  background: #efe3c8;
}
.jr-paper-pick.is-white {
  background: #fbfaf7;
}
.jr-paper-pick.is-black {
  background: #1b1a18;
}
.jr-paper-pick:hover {
  transform: scale(1.08);
}
.jr-paper-pick[aria-pressed='true'] {
  box-shadow:
    0 0 0 2px var(--bg-0),
    0 0 0 4px var(--tx-secondary);
}
.jr-paper-pick:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 3px;
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
.jr-tools :deep(input[type='search']) {
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
/* Einbanddeckel hinter den Seiten mit dezentem Schatten */
.jr-spread::before,
.jr-spread::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  width: 50%;
  background: var(--cover);
  box-shadow:
    0 24px 48px -12px rgb(0 0 0 / 0.6),
    inset 0 0 0 1px rgb(255 255 255 / 0.06);
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
  0%,
  62% {
    opacity: 0;
  }
  62.01%,
  100% {
    opacity: 1;
  }
}

/* Beide Seiten eines Blatts liegen beim Umblättern in derselben Zelle übereinander. */
.jr-side {
  position: relative;
  z-index: 1;
  display: grid;
}
.jr-side-l {
  grid-area: 1 / 1;
}
.jr-side-r {
  grid-area: 1 / 2;
}

.jr-paper {
  grid-area: 1 / 1;
  position: relative;
  min-height: clamp(520px, calc(100dvh - 320px), 760px);
  padding: 38px 42px 46px;
  background: radial-gradient(
    ellipse 130% 95% at 50% 40%,
    var(--paper) 60%,
    var(--paper-2) 135%
  );
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

/* Der Editor schreibt mit Tinte */
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
.jr-left .jr-page-no {
  left: 28px;
}
.jr-right .jr-page-no {
  right: 28px;
}

/* Umschlag */
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
  0%,
  25% {
    opacity: 1;
    transform: perspective(2400px) rotateY(0deg);
  }
  62% {
    opacity: 1;
    transform: perspective(2400px) rotateY(-90deg);
    filter: brightness(0.6);
  }
  62.01%,
  100% {
    opacity: 0;
    transform: perspective(2400px) rotateY(-90deg);
  }
}

@keyframes jr-shift {
  0%,
  25% {
    transform: translateX(-25%);
  }
  100% {
    transform: none;
  }
}
.jr-opening .jr-left {
  animation: jr-appear 160ms 600ms ease-out both;
}
@keyframes jr-appear {
  from {
    opacity: 0;
  }
}

/* Umblättern */
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
.jr-more-btn {
  width: 32px;
  height: 32px;
}
.jr-paper-menu {
  padding: 8px 10px;
  min-width: 140px;
}
.jr-paper-menu .menu-l {
  display: block;
  padding: 2px 4px 6px;
  font: 600 11px/1 var(--font-ui);
  color: var(--tx-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.jr-nav-count {
  min-width: 96px;
  text-align: center;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
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

@media (max-width: 860px) {
  .jr-spread {
    grid-template-columns: 1fr;
    padding: 6px;
    animation: none;
  }
  .jr-cover,
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
}
</style>
