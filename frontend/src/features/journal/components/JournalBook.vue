<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import type { JournalTaskImport } from '@/api/client'
import { shiftDate } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { useShortcuts } from '@/composables/useShortcuts'
import JournalEntryEditor, { type JournalEntryData } from './JournalEntryEditor.vue'
import type { JournalRow } from '../composables/useJournalData'

const props = defineProps<{
  modelValue: string
  pages: string[]
  todayStr: string
  hasOlderEntries: boolean
  getEntryForDate: (d: string) => JournalRow | JournalEntryData | null
  getPlanForDate: (d: string) => string[]
  getDoneItemsOn: (d: string) => string[]
  getImportedTitles: (d: string) => string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [date: string]
  saved: [date: string, entry: JournalEntryData]
  imported: [result: JournalTaskImport]
  leavingDate: [date: string]
}>()

const pageDate = ref(props.modelValue)

watch(
  () => props.modelValue,
  (newDate) => {
    if (newDate && newDate !== pageDate.value) {
      void goToDate(newDate)
    }
  },
)

const editorInstances = new Map<string, InstanceType<typeof JournalEntryEditor>>()
function setEditorInstance(date: string, instance: unknown) {
  if (instance) {
    editorInstances.set(date, instance as InstanceType<typeof JournalEntryEditor>)
  } else {
    editorInstances.delete(date)
  }
}

// Das Jahr steht nur bei älteren Seiten dabei.
const dateFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    }),
)
const dateYearFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    }),
)

function formatDate(dateStr: string): string {
  const sameYear = dateStr.slice(0, 4) === props.todayStr.slice(0, 4)
  return (sameYear ? dateFmt.value : dateYearFmt.value).format(new Date(`${dateStr}T00:00`))
}

function getRelativeDateLabel(dateStr: string): string {
  if (dateStr === props.todayStr) return t('Heute')
  if (dateStr === shiftDate(props.todayStr, -1)) return t('Gestern')
  return ''
}

const currentPageIndex = computed(() => props.pages.indexOf(pageDate.value))

async function goToDate(targetDate: string) {
  const leavingDate = pageDate.value
  if (targetDate === leavingDate) return

  await editorInstances.get(leavingDate)?.flush()

  pageDate.value = targetDate
  emit('update:modelValue', targetDate)
  emit('leavingDate', leavingDate)
}

function flipPage(step: number) {
  const nextDate = props.pages[currentPageIndex.value + step]
  if (nextDate) void goToDate(nextDate)
}

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
    e.pointerType !== 'mouse' && !(e.target as HTMLElement).closest('input, textarea, button')
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
</script>

<template>
  <div v-if="pageDate" class="jr-book">
    <div
      class="jr-sheet"
      @pointerdown="onSwipeStart"
      @pointerup="onSwipeEnd"
      @pointercancel="onSwipeEnd"
    >
      <!-- Das Datum ist die Überschrift der Seite, geblättert wird im Kopf der Ansicht -->
      <div class="jr-bar">
        <h2 class="jr-date">
          {{ formatDate(pageDate) }}
          <span v-if="getRelativeDateLabel(pageDate)" class="jr-rel">{{
            getRelativeDateLabel(pageDate)
          }}</span>
        </h2>
      </div>

      <article
        :key="pageDate"
        class="jr-entry"
        :class="{ 'jr-today': pageDate === todayStr }"
        :data-date="pageDate"
        :aria-label="formatDate(pageDate)"
      >
        <div v-if="getPlanForDate(pageDate).length" class="jr-block">
          <h3 class="jr-block-h">{{ $t('Vorgenommen') }}</h3>
          <ul class="jr-block-list">
            <li v-for="(p, i) in getPlanForDate(pageDate)" :key="i">{{ p }}</li>
          </ul>
        </div>

        <div v-if="getDoneItemsOn(pageDate).length" class="jr-block jr-recap">
          <h3 class="jr-block-h">{{ $t('Erledigt') }}</h3>
          <ul class="jr-block-list">
            <li v-for="(p, i) in getDoneItemsOn(pageDate)" :key="i">{{ p }}</li>
          </ul>
        </div>

        <JournalEntryEditor
          :ref="(c) => setEditorInstance(pageDate, c)"
          :date="pageDate"
          :entry="getEntryForDate(pageDate)"
          :imported-titles="getImportedTitles(pageDate)"
          @saved="emit('saved', pageDate, $event)"
          @imported="emit('imported', $event)"
        />
      </article>
    </div>

    <RouterLink v-if="hasOlderEntries && currentPageIndex === 0" class="v-sub jr-more" to="/notes">
      {{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}
    </RouterLink>
  </div>
</template>

<style scoped src="./journalBook.css"></style>
