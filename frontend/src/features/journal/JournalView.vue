<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { shiftDate } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { PAPERS, prefs } from '@/lib/prefs'
import JournalHeatmap from './components/JournalHeatmap.vue'
import JournalOpenTasks from './components/JournalOpenTasks.vue'
import JournalBackfillDialog from './components/JournalBackfillDialog.vue'
import JournalBook from './components/JournalBook.vue'
import SearchField from '@/components/SearchField.vue'
import { useJournalData, type JournalRow } from './composables/useJournalData'

const PAPER_LABELS: Record<'coffee' | 'white' | 'black', string> = {
  coffee: t('Kaffee'),
  white: t('Weiß'),
  black: t('Schwarz'),
}

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
const isPaperMenuOpen = ref(false)

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

const shownEntriesCount = computed(() => visiblePastRows.value.length + (todayEntry.value ? 1 : 0))
const hasOlderEntries = computed(
  () => !normalizedSearchTerm.value && totalEntriesCount.value > shownEntriesCount.value,
)

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
    pastRows.value.some((r) => r.date === routeDate)
  ) {
    pageDate.value = routeDate
  }
})

watch(
  () => route.query.date,
  (newDate) => {
    if (
      typeof newDate === 'string' &&
      (newDate === todayStr.value || pastRows.value.some((r) => r.date === newDate))
    ) {
      pageDate.value = newDate
    }
  },
)

async function openDay(date: string) {
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
function pickPaper(p: 'coffee' | 'white' | 'black') {
  prefs.journalPaper = p
  isPaperMenuOpen.value = false
}
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
                    @click="pickPaper(p)"
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
      <JournalBook
        v-if="pageDate"
        v-model="pageDate"
        :pages="pages"
        :today-str="todayStr"
        :paper="prefs.journalPaper"
        :has-older-entries="hasOlderEntries"
        :get-entry-for-date="getEntryForDate"
        :get-plan-for-date="getPlanForDate"
        :get-done-items-on="getDoneItemsOn"
        :get-imported-titles="getImportedTitles"
        @saved="onEntrySaved"
        @imported="handleTodayImported"
        @leaving-date="removeDraftIfEmpty"
      />

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
  font: 500 12px/1 var(--font-ui);
  color: var(--tx-muted);
}

@media (max-width: 640px) {
  .jr-title {
    font-size: 30px;
  }
}
</style>
