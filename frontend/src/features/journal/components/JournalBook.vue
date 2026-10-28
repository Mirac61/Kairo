<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import type { JournalTaskImport } from '@/api/client'
import { shiftDate } from '@/lib/dates'
import { locale, t } from '@/lib/i18n'
import { useShortcuts } from '@/composables/useShortcuts'
import { easeTurn, leafFrame, type LeafRole } from '@/lib/pageTurn'
import JournalEntryEditor, { type JournalEntryData } from './JournalEntryEditor.vue'
import type { JournalRow } from '../composables/useJournalData'

const TURN_ANIMATION_MS = 950

const props = defineProps<{
  modelValue: string
  pages: string[]
  todayStr: string
  paper: 'coffee' | 'white' | 'black'
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
const flipDirection = ref<'fwd' | 'back'>('fwd')
const isCoverOpening = ref(true)

onMounted(() => {
  setTimeout(() => {
    isCoverOpening.value = false
  }, 1100)
})

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

// Datumsformate
const headerDateFmt = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    }),
)
const weekdayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long' }))
const monthFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }),
)

function formatWith(formatter: Intl.DateTimeFormat, dateStr: string): string {
  return formatter.format(new Date(`${dateStr}T00:00`))
}

function getRelativeDateLabel(dateStr: string): string {
  if (dateStr === props.todayStr) return t('Heute')
  if (dateStr === shiftDate(props.todayStr, -1)) return t('Gestern')
  return ''
}

function getMonthLabel(yearMonthStr: string): string {
  return formatWith(monthFmt.value, `${yearMonthStr}-01`)
}

function getPageKicker(dateStr: string): string {
  return getRelativeDateLabel(dateStr) || getMonthLabel(dateStr.slice(0, 7))
}

const currentPageIndex = computed(() => props.pages.indexOf(pageDate.value))

let turnStartTime = 0
let isTurningPage = false
let queuedPageStep = 0

async function goToDate(targetDate: string) {
  const leavingDate = pageDate.value
  if (targetDate === leavingDate) return

  await editorInstances.get(leavingDate)?.flush()

  flipDirection.value =
    props.pages.indexOf(targetDate) < props.pages.indexOf(leavingDate) ? 'back' : 'fwd'
  turnStartTime = performance.now()
  isTurningPage = true

  setTimeout(() => {
    isTurningPage = false
    const step = queuedPageStep
    queuedPageStep = 0
    if (step) flipPage(step)
  }, TURN_ANIMATION_MS)

  pageDate.value = targetDate
  emit('update:modelValue', targetDate)
  emit('leavingDate', leavingDate)
}

function flipPage(step: number) {
  if (isTurningPage) {
    queuedPageStep = step
    return
  }
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
    page.animate(entering ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }], {
      duration: 220,
      easing: 'ease-out',
    }).onfinish = done
    return
  }

  const isBack = flipDirection.value === 'back'
  const role: LeafRole =
    side === (isBack ? 'l' : 'r') ? (entering ? 'revealed' : 'cut') : entering ? 'flap' : 'covered'

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
</script>

<template>
  <div v-if="pageDate" class="jr-book" :data-paper="paper">
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
            @saved="emit('saved', d, $event)"
            @imported="emit('imported', $event)"
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

    <RouterLink v-if="hasOlderEntries && currentPageIndex === 0" class="v-sub jr-more" to="/notes">
      {{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}
    </RouterLink>
  </div>
</template>

<style scoped src="./journalBook.css"></style>
