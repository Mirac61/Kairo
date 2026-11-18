<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { shiftDate } from '@/lib/dates'
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
</script>

<template>
  <div class="view-inner jr-wrap">
    <PageHeader :title="$t('Journal')">
      <JournalHeatmap
        :all-dates="allDates"
        :today-str="todayStr"
        :active-date="pageDate"
        :streak="currentStreak"
        @select="openDay"
        @backfill="openBackfillModal"
      />
      <template #actions>
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
        :get-entry-for-date="getEntryForDate"
        :get-plan-for-date="getPlanForDate"
        :get-done-items-on="getDoneItemsOn"
        :get-imported-titles="getImportedTitles"
        @saved="onEntrySaved"
        @imported="handleTodayImported"
        @leaving-date="removeDraftIfEmpty"
      />

      <JournalOpenTasks
        :tasks="openJournalTasks"
        :completing-id="completingTaskId"
        @complete="handleCompleteTask"
      />

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
/* Eine Schreibspalte wie im Notiz-Editor, ohne Blatt und Rahmen */
.jr-page {
  max-width: 720px;
  padding: 24px 0 0;
}
.jr-nav {
  display: flex;
  gap: 2px;
}
.jr-nohit {
  margin: 0 0 12px;
}
</style>
