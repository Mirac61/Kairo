<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { dueCards, type Card } from '@/api/client'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const cards = ref<Card[] | null>(null)
onMounted(() => dueCards().then((cs) => (cards.value = cs), () => (cards.value = [])))

// Fällige Karten je Stapel, größte zuerst; der Balken zeigt den Anteil am Ganzen.
const decks = computed(() => {
  const m = new Map<string, number>()
  for (const c of cards.value ?? []) m.set(c.deck, (m.get(c.deck) ?? 0) + 1)
  return [...m].sort((a, b) => b[1] - a[1]).slice(0, 3)
})
const total = computed(() => cards.value?.length ?? 0)
const fresh = computed(() => (cards.value ?? []).filter((c) => c.new).length)
</script>

<template>
  <section class="wg wg-cards" :aria-label="$t('Lernkarten')">
    <div class="wg-h"><router-link to="/cards"><svg class="ic"><use href="#i-cards" /></svg>{{ $t('Lernkarten') }}</router-link><span v-if="fresh">{{ $t('{n} neu', { n: fresh }) }}</span></div>
    <div class="wk-body">
      <div class="wk-sum">
        <span class="wg-big wk-n">{{ cards ? total : '–' }}</span>
        <span class="wg-meta">{{ total ? $t('fällig heute') : $t('Nichts fällig') }}</span>
      </div>
      <ul v-if="decks.length" class="wk-decks">
        <li v-for="[deck, n] in decks" :key="deck">
          <span class="wk-d">{{ deck || $t('Ohne Stapel') }}</span><span class="wk-c mono">{{ n }}</span>
          <span class="wk-bar" aria-hidden="true"><i :style="{ width: (n / total) * 100 + '%' }"></i></span>
        </li>
      </ul>
    </div>
    <router-link v-if="total" to="/cards" class="btn btn-secondary wk-go">{{ $t('Lernen') }}</router-link>
  </section>
</template>

<style scoped>
.wk-body { flex: 1; min-height: 0; overflow: hidden; display: flex; align-items: flex-start; gap: 20px; }
.wk-sum { display: grid; gap: 4px; flex: none; }
.wk-n { font-size: 56px; }
/* Spalten-Umbruch schiebt Stapel, die nicht mehr ganz passen, rechts aus dem Bild statt sie halb zu zeigen */
.wk-decks { flex: 1; align-self: stretch; min-width: 0; margin: 4px 0 0; padding: 0; list-style: none; display: flex; flex-flow: column wrap; gap: 10px 40px; overflow: hidden; }
.wk-decks li { width: 100%; display: grid; grid-template-columns: minmax(0, 1fr) auto; row-gap: 4px; align-items: baseline; }
.wk-d { font: 500 14px/1.3 var(--font-ui); color: var(--tx-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wk-c { font-size: 12px; color: var(--tx-muted); }
.wk-bar { grid-column: 1 / -1; height: 3px; border-radius: 2px; background: var(--br-subtle); }
.wk-bar i { display: block; height: 3px; border-radius: 2px; background: var(--a-blue-hi); }
.wg-cards { container-type: size; }
@container (max-height: 220px) {
  .wk-n { font-size: 34px; }
}
.wk-go { align-self: flex-start; height: 32px; text-decoration: none; }
</style>
