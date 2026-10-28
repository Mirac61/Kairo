<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  cardLog,
  errorMessage,
  listCards,
  listDecks,
  type Card,
  type CardLog,
  type Deck,
} from '@/api/client'
import { shuffled } from '@/lib/cards'
import CardAddDialog from './components/CardAddDialog.vue'
import CardBrowser from './components/CardBrowser.vue'
import CardDeck from './components/CardDeck.vue'
import CardLearn from './components/CardLearn.vue'

// Lernkarten wie Anki: Startansicht ist der Kartenbrowser (Fächer, Liste, gewählte Karte); dazu Fach-Optionen und Lernrunde.
const cards = ref<Card[]>([])
const decks = ref<Deck[]>([])
const log = ref<CardLog[]>([])
const error = ref('')
const loaded = ref(false)
const mode = ref<'home' | 'deck' | 'learn'>('home')
const deckName = ref<string | null>(null)
const adding = ref(false)

async function load() {
  try {
    ;[cards.value, decks.value, log.value] = await Promise.all([
      listCards(),
      listDecks(),
      cardLog(),
    ])
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)

const colorOf = (name: string) =>
  `var(--p-${decks.value.find((d) => d.name === name)?.color ?? 'neutral'})`
const current = computed(() => decks.value.find((d) => d.name === deckName.value))
const cardsOf = (name: string) => cards.value.filter((c) => c.deck === name)

function openDeck(name: string) {
  deckName.value = name
  mode.value = 'deck'
}
function openAdd(name: string | null) {
  deckName.value = name
  adding.value = true
}

// --- Lernrunde: erst Karten in Arbeit, dann fällige Wiederholungen, dann neue. Übungsrunden sind gemischt.
const round = ref<{ id: number; cards: Card[]; practice: boolean; title: string | null }>({
  id: 0,
  cards: [],
  practice: false,
  title: null,
})
const order = (c: Card) => (c.learning ? 0 : c.new ? 2 : 1)
function learn(name: string | null, practice = false, keys?: string[]) {
  const pool = name === null ? cards.value : cardsOf(name)
  const list = keys
    ? shuffled(cards.value.filter((c) => keys.includes(c.key)))
    : practice
      ? shuffled(pool)
      : pool.filter((c) => c.due).sort((a, b) => order(a) - order(b))
  if (!list.length) return
  if (name !== null) deckName.value = name
  round.value = { id: round.value.id + 1, cards: list, practice: practice || !!keys, title: name }
  mode.value = 'learn'
}
function stopLearning() {
  mode.value = 'home'
  void load()
}

function onDeckChanged(name: string | null) {
  deckName.value = name
  if (!name) mode.value = 'home'
  void load()
}
</script>

<template>
  <div class="view-inner cv-page">
    <CardLearn
      v-if="mode === 'learn'"
      :key="round.id"
      :round="round.cards"
      :title="round.title"
      :practice="round.practice"
      :cards="cards"
      :decks="decks"
      :log="log"
      :color-of="colorOf"
      @exit="stopLearning"
      @finished="load"
      @again="(keys) => learn(round.title, true, keys)"
    />

    <CardDeck
      v-else-if="mode === 'deck' && current"
      :deck="current"
      :cards="cardsOf(current.name)"
      :log="log"
      :color-of="colorOf"
      @back="mode = 'home'"
      @learn="(p) => learn(current!.name, p)"
      @browse="mode = 'home'"
      @add="adding = true"
      @changed="onDeckChanged"
    />

    <template v-else>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>
      <CardBrowser
        v-if="loaded"
        :key="deckName ?? ''"
        :cards="cards"
        :decks="decks"
        :log="log"
        :deck="deckName"
        :color-of="colorOf"
        @changed="load"
        @add="openAdd"
        @learn="(d) => learn(d)"
        @options="openDeck"
      />
    </template>

    <CardAddDialog
      v-if="adding"
      :decks="decks"
      :deck="deckName"
      @close="adding = false"
      @added="load"
    />
  </div>
</template>

<style scoped>
.cv-page {
  max-width: none;
  min-height: 100dvh;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}
@media (max-width: 760px) {
  .cv-page {
    min-height: 0;
  }
}
</style>
