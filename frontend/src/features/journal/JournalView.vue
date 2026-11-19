<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { shiftDate } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import JournalHeatmap from './components/JournalHeatmap.vue'
import JournalOpenTasks from './components/JournalOpenTasks.vue'
import JournalBackfillDialog from './components/JournalBackfillDialog.vue'
import JournalBook from './components/JournalBook.vue'
import SearchField from '@/components/SearchField.vue'
import PageHeader from '@/components/PageHeader.vue'
import IconButton from '@/components/IconButton.vue'
import { useJournalData, type JournalRow } from './composables/useJournalData'

const route = useRoute()
const {
  todayStr,
  totalEntriesCount,
  currentStreak,
  errorMessageText,
  allDates,
  pastRows,
  todayEntry,
  completingTaskId,
  openJournalTasks,
  getEntryForDate,
  getDoneItemsOn,
  getPlanForDate,
  getImportedTitles,
  loadData,
  onEntrySaved,
  removeDraftIfEmpty,
  ensureRowLoaded,
  handleTodayImported,
  handleCompleteTask,
} = useJournalData()

const pageDate = ref('')
const mode = ref<'write' | 'markdown' | 'read'>('write')
const selectedMonth = ref('')

// Suche
const pastSearch = ref('')
const normalizedSearchTerm = computed(() => pastSearch.value.trim().toLowerCase())

function matchesSearch(row: JournalRow): boolean {
  const query = normalizedSearchTerm.value
  if (!query) return true
  const haystack = `${row.date} ${row.answers.join(' ')} ${row.other}`.toLowerCase()
  return haystack.includes(query)
}

const visiblePastRows = computed(() => pastRows.value.filter(matchesSearch))

// Seiten wie im Heft: die älteste vorn, heute ist die letzte beschriebene Seite.
const pages = computed(() => [
  ...visiblePastRows.value.map((r) => r.date).reverse(),
  ...(todayStr.value ? [todayStr.value] : []),
])

// Blättern im Kopf; JournalBook speichert die alte Seite, bevor es die neue aufschlägt.
const pageIndex = computed(() => pages.value.indexOf(pageDate.value))
function flip(step: number) {
  const next = pages.value[pageIndex.value + step]
  if (next) pageDate.value = next
}

const shownEntriesCount = computed(() => visiblePastRows.value.length + (todayEntry.value ? 1 : 0))
const hasOlderEntries = computed(
  () => !normalizedSearchTerm.value && totalEntriesCount.value > shownEntriesCount.value,
)

// Formatierung für Kopfzeilen-Breadcrumb & skalierbares Monats-Archiv
const crumbFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    }),
)
const dayLabelFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
    }),
)
const monthTitleFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      month: 'long',
      year: 'numeric',
    }),
)
const wdShortFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short' }))

const headerDate = computed(() =>
  pageDate.value ? crumbFmt.value.format(new Date(`${pageDate.value}T00:00`)) : '',
)

function formatMonthHeading(ym: string): string {
  const [y = 2000, m = 1] = ym.split('-').map(Number)
  return monthTitleFmt.value.format(new Date(y, m - 1, 1))
}

function snippetOf(answers?: string[], other?: string): string {
  const first =
    answers?.find((a) => a.trim())?.trim() ||
    (other ?? '').replace(/^##\s+Freie Notizen\s*/i, '').trim()
  return first.replace(/\s+/g, ' ').replace(/^[-*#>\s]+/, '')
}

function dayTag(d: string): string {
  if (d === todayStr.value) return t('Heute')
  if (d === shiftDate(todayStr.value, -1)) return t('Gestern')
  return wdShortFmt.value.format(new Date(`${d}T00:00`)).replace(/\.$/, '')
}

// Alle Monate über sämtliche Einträge (auch bei hunderten/tausenden Einträgen über Jahre hinweg)
const allArchiveDates = computed(() => {
  const set = new Set<string>(allDates.value)
  if (todayStr.value) set.add(todayStr.value)
  return [...set].sort().reverse()
})

interface MonthOption {
  key: string
  label: string
  count: number
}

const monthOptions = computed<MonthOption[]>(() => {
  const counts = new Map<string, number>()
  for (const d of allArchiveDates.value) {
    const ym = d.slice(0, 7)
    counts.set(ym, (counts.get(ym) ?? 0) + 1)
  }
  return [...counts.entries()].map(([key, count]) => ({
    key,
    label: formatMonthHeading(key),
    count,
  }))
})

interface ArchiveDay {
  date: string
  title: string
  tag: string
  snippet: string
  isToday: boolean
  isLoaded: boolean
}

interface ArchiveGroup {
  ym: string
  label: string
  items: ArchiveDay[]
}

const archiveGroups = computed<ArchiveGroup[]>(() => {
  const dates = normalizedSearchTerm.value
    ? visiblePastRows.value.map((r) => r.date)
    : allArchiveDates.value.filter((d) => !selectedMonth.value || d.startsWith(selectedMonth.value))

  const groups: ArchiveGroup[] = []
  let current: ArchiveGroup | null = null

  for (const d of dates) {
    const ym = d.slice(0, 7)
    if (!current || current.ym !== ym) {
      current = { ym, label: formatMonthHeading(ym), items: [] }
      groups.push(current)
    }
    const entry = getEntryForDate(d)
    const isToday = d === todayStr.value
    const snippet = entry
      ? snippetOf(entry.answers, entry.other) || t('Noch kein Eintrag')
      : isToday
        ? t('Noch kein Eintrag')
        : ''
    current.items.push({
      date: d,
      title: dayLabelFmt.value.format(new Date(`${d}T00:00`)),
      tag: dayTag(d),
      snippet,
      isToday,
      isLoaded: Boolean(entry),
    })
  }
  return groups
})

// Treffer der Suche aufschlagen
watch(normalizedSearchTerm, (query) => {
  const currentPages = pages.value
  if (!currentPages.includes(pageDate.value) || query) {
    const target =
      (query && currentPages.length > 1
        ? currentPages[currentPages.length - 2]
        : currentPages[currentPages.length - 1]) ?? todayStr.value
    pageDate.value = target
  }
})

onMounted(async () => {
  await loadData()
  pageDate.value = todayStr.value

  const routeDate = route.query.date
  if (
    typeof routeDate === 'string' &&
    routeDate !== todayStr.value &&
    (pastRows.value.some((r) => r.date === routeDate) || allDates.value.includes(routeDate))
  ) {
    await openDay(routeDate)
  }
})

watch(
  () => route.query.date,
  (newDate) => {
    if (
      typeof newDate === 'string' &&
      (newDate === todayStr.value ||
        pastRows.value.some((r) => r.date === newDate) ||
        allDates.value.includes(newDate))
    ) {
      void openDay(newDate)
    }
  },
)

async function openDay(date: string) {
  if (date === todayStr.value) {
    pageDate.value = date
    return
  }
  const ok = await ensureRowLoaded(date)
  if (ok) {
    pageDate.value = date
  }
}

// Nachtragen
const isBackfillOpen = ref(false)
const backfillDate = ref('')

function openBackfillModal(defaultDate?: string) {
  backfillDate.value = defaultDate || shiftDate(todayStr.value, -1)
  isBackfillOpen.value = true
}

async function handleBackfillSubmit(date: string) {
  isBackfillOpen.value = false
  await openDay(date)
}
</script>

<template>
  <div class="view-inner jr-shell">
    <div class="jr-main">
      <div class="jr-head-wrap">
        <PageHeader :title="$t('Journal')">
          <div class="jr-crumb">
            <span>{{ $t('Journal') }}</span>
            <template v-if="headerDate">
              <span class="jr-crumb-sep" aria-hidden="true">/</span>
              <strong>{{ headerDate }}</strong>
            </template>
          </div>
          <template #actions>
            <button
              v-if="todayStr && pageDate && pageDate !== todayStr"
              type="button"
              class="btn btn-ghost jr-today-btn"
              @click="openDay(todayStr)"
            >
              {{ $t('Heute') }}
            </button>
            <div class="jr-mode" role="group" :aria-label="$t('Modus')">
              <button
                type="button"
                class="jr-mode-btn"
                :class="{ on: mode === 'write' }"
                :aria-pressed="mode === 'write'"
                @click="mode = 'write'"
              >
                {{ $t('Schreiben') }}
              </button>
              <button
                type="button"
                class="jr-mode-btn"
                :class="{ on: mode === 'markdown' }"
                :aria-pressed="mode === 'markdown'"
                @click="mode = 'markdown'"
              >
                {{ $t('Markdown') }}
              </button>
              <button
                type="button"
                class="jr-mode-btn"
                :class="{ on: mode === 'read' }"
                :aria-pressed="mode === 'read'"
                @click="mode = 'read'"
              >
                {{ $t('Lesen') }}
              </button>
            </div>
            <SearchField
              v-if="pastRows.length"
              v-model="pastSearch"
              :label="$t('Einträge durchsuchen')"
            />
            <IconButton icon="plus" :label="$t('Nachtragen')" @click="openBackfillModal()" />
            <nav class="jr-nav" :aria-label="$t('Seiten')">
              <IconButton
                icon="left"
                :label="$t('Vorherige Seite')"
                shortcut="←"
                :disabled="pageIndex <= 0"
                @click="flip(-1)"
              />
              <IconButton
                icon="right"
                :label="$t('Nächste Seite')"
                shortcut="→"
                :disabled="pageIndex >= pages.length - 1"
                @click="flip(1)"
              />
            </nav>
          </template>
        </PageHeader>
      </div>

      <div class="jr-page">
        <div v-if="errorMessageText" class="badge" role="alert">{{ errorMessageText }}</div>

        <p v-if="normalizedSearchTerm && !visiblePastRows.length" class="v-sub jr-nohit">
          {{ $t('Keine Treffer für „{q}“.', { q: pastSearch.trim() }) }}
        </p>

        <JournalBook
          v-if="pageDate"
          v-model="pageDate"
          :pages="pages"
          :today-str="todayStr"
          :has-older-entries="hasOlderEntries"
          :mode="mode"
          :get-entry-for-date="getEntryForDate"
          :get-plan-for-date="getPlanForDate"
          :get-done-items-on="getDoneItemsOn"
          :get-imported-titles="getImportedTitles"
          @saved="onEntrySaved"
          @imported="handleTodayImported"
          @leaving-date="removeDraftIfEmpty"
        />
      </div>
    </div>

    <aside class="jr-timeline" :aria-label="$t('Archiv')">
      <JournalHeatmap
        :all-dates="allDates"
        :today-str="todayStr"
        :active-date="pageDate"
        :streak="currentStreak"
        @select="openDay"
        @backfill="openBackfillModal"
      />

      <div class="jr-archive-wrap">
        <div class="jr-archive-bar">
          <h3 class="jr-tl-title">
            {{ $t('Archiv') }}
            <span v-if="totalEntriesCount" class="jr-tl-cnt">{{ totalEntriesCount }}</span>
          </h3>
          <select
            v-if="monthOptions.length && !normalizedSearchTerm"
            v-model="selectedMonth"
            class="jr-month-select"
            :aria-label="$t('Monat wählen')"
          >
            <option value="">{{ $t('Alle Monate') }}</option>
            <option v-for="m in monthOptions" :key="m.key" :value="m.key">
              {{ m.label }} ({{ m.count }})
            </option>
          </select>
        </div>

        <div class="jr-archive">
          <div v-for="g in archiveGroups" :key="g.ym" class="jr-month-group">
            <div class="jr-month-head">
              <span>{{ g.label }}</span>
              <small>{{ g.items.length }}</small>
            </div>
            <div class="jr-day-list">
              <button
                v-for="d in g.items"
                :key="d.date"
                type="button"
                class="jr-day-item"
                :class="{ active: d.date === pageDate }"
                :aria-current="d.date === pageDate ? 'date' : undefined"
                @click="openDay(d.date)"
              >
                <span class="jr-day-top">
                  <span class="jr-day-name">{{ d.title }}</span>
                  <small :class="{ today: d.isToday }">{{ d.tag }}</small>
                </span>
                <span v-if="d.snippet" class="jr-day-snip">{{ d.snippet }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <JournalOpenTasks
        :tasks="openJournalTasks"
        :completing-id="completingTaskId"
        @complete="handleCompleteTask"
      />
    </aside>

    <JournalBackfillDialog
      :open="isBackfillOpen"
      :initial-date="backfillDate"
      :max-date="todayStr"
      @close="isBackfillOpen = false"
      @submit="handleBackfillSubmit"
    />
  </div>
</template>

<style scoped>
/* Zwei-Spalten-Journal: Schreibfläche + skalierbare Archiv-Seitenleiste */
.view-inner.jr-shell {
  max-width: none;
  padding: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 304px;
  min-height: 100%;
  width: 100%;
  box-sizing: border-box;
}
.jr-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.jr-head-wrap {
  padding: 8px 28px 0;
  border-bottom: 1px solid color-mix(in oklab, var(--br-subtle) 85%, transparent);
}
.jr-head-wrap :deep(.page-head) {
  margin-bottom: 0;
}
.jr-crumb {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font: 400 13px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.jr-crumb strong {
  font-weight: 500;
  color: var(--tx-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.jr-crumb-sep {
  opacity: 0.45;
}
.jr-today-btn {
  height: 26px;
  padding: 0 9px;
  font-size: 12px;
  color: var(--a-blue-hi);
}
.jr-mode {
  display: inline-flex;
  align-items: center;
  padding: 2px;
  gap: 2px;
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
  border-radius: 999px;
  margin-right: 4px;
}
.jr-mode-btn {
  padding: 4px 10px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--tx-muted);
  font: 400 12px/1 var(--font-ui);
  cursor: pointer;
  transition:
    background-color var(--dur) ease-out,
    color var(--dur) ease-out;
}
.jr-mode-btn:hover {
  color: var(--tx-secondary);
}
.jr-mode-btn.on {
  background: var(--bg-2);
  color: var(--tx-primary);
  font-weight: 500;
}
.jr-nav {
  display: flex;
  gap: 2px;
}

.jr-page {
  width: 100%;
  max-width: 736px;
  margin: 0 auto;
  padding: 32px 28px 80px;
  box-sizing: border-box;
}
.jr-nohit {
  margin: 0 0 12px;
}

/* Rechte Seitenleiste: Heatmap oben, scrollbares Monatsarchiv in der Mitte, offene Aufgaben unten */
.jr-timeline {
  position: sticky;
  top: 0;
  height: 100dvh;
  max-height: 100dvh;
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-width: 0;
  padding: 18px 16px 20px;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  box-sizing: border-box;
  overflow: hidden;
}
.jr-archive-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--br-subtle);
}
.jr-archive-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.jr-tl-title {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  margin: 0;
  font: 500 12px/1.2 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-tl-cnt {
  font: 400 11.5px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.jr-month-select {
  max-width: 154px;
  height: 26px;
  padding: 0 8px;
  border: 1px solid var(--br-subtle);
  border-radius: 6px;
  background: var(--bg-card);
  color: var(--tx-secondary);
  font: 400 11.5px/1 var(--font-ui);
  cursor: pointer;
}
.jr-month-select:hover {
  border-color: var(--br-default);
  color: var(--tx-primary);
}
.jr-month-select:focus-visible {
  outline: 2px solid var(--a-focus);
  outline-offset: 1px;
}
.jr-archive {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-right: 2px;
}
.jr-month-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.jr-month-head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  padding: 3px 4px;
  background: var(--bg-1);
  font: 500 11px/1.2 var(--font-ui);
  color: var(--tx-muted);
}
.jr-month-head small {
  font-variant-numeric: tabular-nums;
  color: var(--tx-disabled);
}
.jr-day-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.jr-day-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition:
    background-color var(--dur) ease-out,
    border-color var(--dur) ease-out;
}
.jr-day-item:hover {
  background: var(--bg-hover);
}
.jr-day-item.active {
  background: var(--bg-card);
  border-color: var(--br-default);
}
.jr-day-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font: 400 13px/1.3 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-day-item.active .jr-day-top {
  color: var(--tx-primary);
  font-weight: 500;
}
.jr-day-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.jr-day-top small {
  flex: none;
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.jr-day-top small.today {
  color: var(--a-blue-hi);
  font-weight: 500;
}
.jr-day-snip {
  font: 400 12px/1.35 var(--font-ui);
  color: var(--tx-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 960px) {
  .view-inner.jr-shell {
    grid-template-columns: minmax(0, 1fr);
    padding: 0;
  }
  .jr-head-wrap {
    padding: 8px 16px 0;
  }
  .jr-page {
    padding: 20px 16px 32px;
  }
  .jr-timeline {
    position: static;
    height: auto;
    max-height: none;
    overflow: visible;
    border-left: 0;
    border-top: 1px solid var(--br-subtle);
    padding: 20px 16px calc(120px + env(safe-area-inset-bottom));
  }
  .jr-archive {
    max-height: 280px;
  }
}

@media (max-width: 640px) {
  .jr-crumb strong,
  .jr-crumb-sep {
    display: none;
  }
  .jr-mode {
    margin-right: 0;
  }
  .jr-mode-btn {
    padding: 4px 8px;
  }
}
</style>
