<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  errorMessage,
  reviewCard,
  undoReview,
  type Card,
  type CardLog,
  type Deck,
} from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { t } from '@/lib/i18n'
import CardLearnStage from './CardLearnStage.vue'
import CardLearnSummary from './CardLearnSummary.vue'
import './cardLearn.css'

// Eine Lernrunde wie bei Anki: Vorderseite, „Antwort zeigen“, vier Bewertungen. „Nochmal“ stellt die Karte ans Ende.
// practice: Übungsrunde, nichts wird gespeichert. Am Ende die Auswertung der Runde.
const props = defineProps<{
  round: Card[]
  title: string | null
  practice: boolean
  cards: Card[] // alle Karten, für „morgen fällig“ (nach dem Neuladen am Rundenende)
  decks: Deck[]
  log: CardLog[]
  colorOf: (deck: string) => string
}>()
const emit = defineEmits<{ exit: []; finished: []; again: [keys: string[]] }>()

const GRADES = [t('Nochmal'), t('Schwer'), t('Gut'), t('Leicht')]

const queue = ref<Card[]>([...props.round])
const practice = ref(props.practice)
const reverse = ref(false)
const shown = ref(false)
const busy = ref(false)
const error = ref('')
const started = Date.now()
const minutes = ref(0)
// Pro Karte: alle Bewertungen dieser Runde und der nächste Abstand (null in der Übungsrunde).
const results = reactive(new Map<string, { card: Card; grades: number[]; next: number | null }>())
const history = ref<{ queue: Card[]; key: string; saved: boolean }[]>([])

const card = computed(() => queue.value[0])
const done = computed(() => props.round.length - new Set(queue.value.map((c) => c.key)).size)
const counts = computed(() => {
  const n = { new: 0, learn: 0, due: 0 }
  for (const c of queue.value) n[c.new ? 'new' : c.learning ? 'learn' : 'due']++
  return n
})
const kindOf = (c?: Card) => (!c ? '' : c.new ? 'new' : c.learning ? 'learn' : 'due')
const front = computed(() =>
  card.value ? (reverse.value ? card.value.answer : card.value.question) : '',
)
const back = computed(() =>
  card.value ? (reverse.value ? card.value.question : card.value.answer) : '',
)

function toggleReverse() {
  reverse.value = !reverse.value
  shown.value = false
}

async function grade(g: number) {
  const c = card.value
  if (!c || !shown.value || busy.value) return
  busy.value = true
  try {
    const saved = !practice.value
    const res = saved ? await reviewCard(c.key, g) : null
    history.value.push({ queue: [...queue.value], key: c.key, saved })
    const r = results.get(c.key) ?? { card: c, grades: [], next: null }
    r.grades.push(g)
    r.next = res?.interval_days ?? null
    results.set(c.key, r)
    queue.value.shift()
    // Nach „Nochmal“ ist die Karte wieder in Arbeit; die Abstände sind dann die einer neuen Wiederholung (SM-2 im Backend).
    if (g === 0)
      queue.value.push({ ...c, new: false, learning: true, next_intervals: [0, 1, 1, 4] })
    shown.value = false
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

async function undo() {
  const h = history.value.at(-1)
  if (!h || busy.value) return
  busy.value = true
  try {
    if (h.saved) await undoReview()
    history.value.pop()
    queue.value = h.queue
    const r = results.get(h.key)!
    r.grades.pop()
    if (!r.grades.length) results.delete(h.key)
    shown.value = true
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

watch(card, (c) => {
  if (c) return
  minutes.value = Math.max(1, Math.round((Date.now() - started) / 60000))
  emit('finished')
})

useShortcuts((e) => {
  if (e.key === 'Escape') return emit('exit')
  if (e.key === 'z') {
    e.preventDefault()
    return void undo()
  }
  if (!card.value) return
  if (e.key === ' ' || e.key === 'Enter') {
    // Wie bei Anki: erst aufdecken, dann „Gut“.
    e.preventDefault()
    if (shown.value) void grade(2)
    else shown.value = true
  } else if (shown.value && ['1', '2', '3', '4'].includes(e.key)) {
    e.preventDefault()
    void grade(Number(e.key) - 1)
  }
})
</script>

<template>
  <div class="cl">
    <header class="cl-top">
      <button type="button" class="col-link cl-back" @click="emit('exit')">
        <svg class="ic"><use href="#i-left" /></svg>
        <i v-if="title" class="cl-dot" :style="{ background: colorOf(title) }"></i>
        <span>{{ title ?? $t('Alle Fächer') }}</span>
        <kbd>Esc</kbd>
      </button>

      <div
        class="cl-counts"
        :aria-label="
          $t('{a} neu, {b} lernen, {c} fällig', { a: counts.new, b: counts.learn, c: counts.due })
        "
      >
        <span class="cl-count-pill c-new" :class="{ on: kindOf(card) === 'new' }">
          <small>{{ $t('Neu') }}</small>
          <b class="mono">{{ counts.new }}</b>
        </span>
        <span class="cl-count-pill c-learn" :class="{ on: kindOf(card) === 'learn' }">
          <small>{{ $t('Lernen') }}</small>
          <b class="mono">{{ counts.learn }}</b>
        </span>
        <span class="cl-count-pill c-due" :class="{ on: kindOf(card) === 'due' }">
          <small>{{ $t('Fällig') }}</small>
          <b class="mono">{{ counts.due }}</b>
        </span>
      </div>

      <div class="cl-tools">
        <button
          type="button"
          class="cl-tool"
          :aria-pressed="practice"
          :title="$t('Üben ohne Wertung')"
          @click="practice = !practice"
        >
          <span>{{ $t('Üben ohne Wertung') }}</span>
        </button>
        <button
          type="button"
          class="cl-tool"
          :aria-pressed="reverse"
          :title="$t('Umgedreht')"
          @click="toggleReverse"
        >
          <svg class="ic"><use href="#i-redo" /></svg>
          <span>{{ $t('Umgedreht') }}</span>
        </button>
        <button
          type="button"
          class="cl-tool"
          :disabled="!history.length || busy"
          :title="$t('Rückgängig')"
          @click="undo"
        >
          <svg class="ic"><use href="#i-undo" /></svg>
          <span>{{ $t('Rückgängig') }}</span>
          <kbd>Z</kbd>
        </button>
      </div>
    </header>

    <div class="cl-progress" aria-hidden="true">
      <i :style="{ width: (round.length ? done / round.length : 0) * 100 + '%' }"></i>
    </div>
    <div v-if="error" class="badge cl-err" role="alert">{{ error }}</div>

    <CardLearnStage
      v-if="card"
      :card="card"
      :shown="shown"
      :reverse="reverse"
      :practice="practice"
      :done="done"
      :total="round.length"
      :busy="busy"
      :front="front"
      :back="back"
      :color-of="colorOf"
      :grades="GRADES"
      @reveal="shown = true"
      @grade="grade"
    />

    <CardLearnSummary
      v-else
      :results="results"
      :cards="cards"
      :decks="decks"
      :log="log"
      :color-of="colorOf"
      :minutes="minutes"
      :grades="GRADES"
      @exit="emit('exit')"
      @again="emit('again', $event)"
    />
  </div>
</template>
