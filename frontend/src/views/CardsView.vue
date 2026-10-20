<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { dueCards, errorMessage, reviewCard, type DueCard } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { t } from '@/lib/i18n'

const GRADES = [t('Nochmal'), t('Schwer'), t('Gut'), t('Leicht')]

const queue = ref<DueCard[]>([])
const total = ref(0)
const done = ref(0)
const shown = ref(false)
const loaded = ref(false)
const busy = ref(false)
const error = ref('')
const card = computed(() => queue.value[0])

onMounted(async () => {
  try {
    queue.value = await dueCards()
    total.value = queue.value.length
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
})

// „Nochmal“ stellt die Karte ans Ende der Runde; sonst ist sie für heute erledigt.
async function grade(g: number) {
  const c = card.value
  if (!c || !shown.value || busy.value) return
  busy.value = true
  try {
    await reviewCard(c.key, g)
    queue.value.shift()
    if (g === 0) queue.value.push(c)
    else done.value++
    shown.value = false
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

useShortcuts((e) => {
  if (!card.value) return
  if (!shown.value && (e.key === ' ' || e.key === 'Enter')) {
    e.preventDefault()
    shown.value = true
  } else if (shown.value && ['1', '2', '3', '4'].includes(e.key)) {
    e.preventDefault()
    void grade(Number(e.key) - 1)
  }
})
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">{{ $t('Lernkarten') }}</h1>
      <div class="v-sub">{{ total ? $t('{n} von {m} gelernt', { n: done, m: total }) : $t('Wiederholung nach Abständen aus deinen Notizen') }}</div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div v-if="total" class="pbar cd-progress" aria-hidden="true"><div class="pbar-fill" :style="{ width: (done / total) * 100 + '%' }"></div></div>

    <section v-if="card" class="card cd-card" :aria-label="$t('Karte')">
      <div class="cd-meta">
        <span v-if="card.new" class="chip">{{ $t('Neu') }}</span>
        <router-link class="col-link" :to="{ path: '/notes', query: { open: card.note_path } }">{{ card.note_path }}</router-link>
      </div>
      <p class="cd-q">{{ card.question }}</p>
      <template v-if="shown">
        <hr class="cd-sep" />
        <p class="cd-a">{{ card.answer }}</p>
        <div class="cd-grades">
          <button v-for="(l, g) in GRADES" :key="g" type="button" class="btn" :class="g === 2 ? 'btn-primary' : 'btn-secondary'" :disabled="busy" @click="grade(g)">
            {{ l }}<kbd>{{ g + 1 }}</kbd>
          </button>
        </div>
      </template>
      <button v-else type="button" class="btn btn-primary cd-show" @click="shown = true">{{ $t('Antwort zeigen') }}<kbd>{{ $t('Leertaste') }}</kbd></button>
    </section>

    <p v-else-if="loaded && total" class="cd-empty">{{ $t('Fertig für heute. Morgen geht es weiter.') }}</p>
    <div v-else-if="loaded && !error" class="cd-empty">
      <p>{{ $t('Keine Karten fällig. Schreib in eine Notiz eine Zeile wie') }}</p>
      <code>Mitochondrium :: Kraftwerk der Zelle</code>
      <p>{{ $t('Jede solche Zeile wird eine Karte. Kairo fragt sie in wachsenden Abständen ab.') }}</p>
    </div>
  </div>
</template>

<style scoped>
.cd-progress { max-width: 640px; margin-bottom: 20px; }
.cd-card { display: grid; gap: 16px; max-width: 640px; padding: 24px; }
.cd-meta { display: flex; align-items: center; gap: 10px; min-width: 0; }
.cd-meta .col-link { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cd-q { margin: 0; font: 600 20px/1.4 var(--font-ui); color: var(--tx-primary); white-space: pre-wrap; }
.cd-a { margin: 0; font: 400 16px/1.5 var(--font-ui); color: var(--tx-secondary); white-space: pre-wrap; }
.cd-sep { width: 100%; margin: 0; border: 0; border-top: 1px solid var(--br-default); }
.cd-grades { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.cd-show { justify-self: start; }
.cd-card kbd { margin-left: 8px; font: 400 11px/1 var(--font-mono); opacity: 0.6; }
.cd-empty { display: grid; gap: 8px; max-width: 640px; color: var(--tx-muted); }
.cd-empty p { margin: 0; }
.cd-empty code { justify-self: start; padding: 4px 8px; border-radius: var(--r-s); background: var(--bg-1); color: var(--tx-primary); }
@media (max-width: 520px) {
  .cd-grades { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .cd-card kbd { display: none; }
}
</style>
