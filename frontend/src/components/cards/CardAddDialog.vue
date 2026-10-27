<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { createCard, errorMessage, type Deck } from '@/api/client'
import { parseCardList } from '@/lib/cards'
import { vDialog } from '@/lib/dialog'
import { store } from '@/lib/storage'
import { tn } from '@/lib/i18n'

// Karten anlegen: einzeln (bleibt offen für die nächste) oder als eingefügte Liste „Frage :: Antwort“ / Tab-getrennt.
const props = defineProps<{ decks: Deck[]; deck: string | null }>()
const emit = defineEmits<{ close: []; added: [] }>()

const mode = ref<'one' | 'list'>('one')
const deck = ref(props.deck ?? store.get('kairo-cards-deck') ?? props.decks[0]?.name ?? '')
const question = ref('')
const answer = ref('')
const both = ref(false)
const text = ref('')
const busy = ref(false)
const error = ref('')
const added = ref(0)
const questionEl = ref<HTMLTextAreaElement>()

const parsed = computed(() => parseCardList(text.value))

async function save() {
  const d = deck.value.trim()
  const items = mode.value === 'one'
    ? [{ question: question.value.trim(), answer: answer.value.trim() }].filter((c) => c.question && c.answer)
    : parsed.value.cards
  if (!d || !items.length || busy.value) return
  if (mode.value === 'one' && both.value) items.push({ question: items[0]!.answer, answer: items[0]!.question })
  busy.value = true
  try {
    // shortcut: eine Anfrage pro Karte; bei Importen über ein paar hundert eine Sammel-Route bauen.
    for (const c of items) await createCard({ deck: d, ...c })
    store.set('kairo-cards-deck', d)
    added.value += items.length
    error.value = ''
    emit('added')
    if (mode.value === 'list') return emit('close')
    question.value = answer.value = ''
    await nextTick()
    questionEl.value?.focus()
  } catch (e) {
    error.value = errorMessage(e)
  }
  busy.value = false
}

// ⌘Enter speichert aus jedem Feld; in der Frage reicht Enter.
function onKey(e: KeyboardEvent, inQuestion = false) {
  if (e.key !== 'Enter' || e.shiftKey) return
  if (e.metaKey || e.ctrlKey || inQuestion) {
    e.preventDefault()
    void save()
  }
}
</script>

<template>
  <div v-dialog="() => emit('close')" class="overlay open" @mousedown.self="emit('close')">
    <form class="dialog ca" :aria-label="$t('Karten hinzufügen')" @submit.prevent="save">
      <div class="dlg-head">
        <h3>{{ $t('Karten hinzufügen') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')"><svg class="ic"><use href="#i-x" /></svg></button>
      </div>
      <div class="dlg-body">
        <div class="ca-top">
          <div class="field ca-deck">
            <label for="ca-deck">{{ $t('Fach') }}</label>
            <input id="ca-deck" v-model="deck" class="input" list="ca-decks" required :placeholder="$t('Neues Fach, z. B. Mathe 2')" />
            <datalist id="ca-decks"><option v-for="d in decks" :key="d.name" :value="d.name" /></datalist>
          </div>
          <div class="ca-tabs" role="tablist" :aria-label="$t('Eingabe')">
            <button type="button" role="tab" :aria-selected="mode === 'one'" @click="mode = 'one'">{{ $t('Einzeln') }}</button>
            <button type="button" role="tab" :aria-selected="mode === 'list'" @click="mode = 'list'">{{ $t('Liste einfügen') }}</button>
          </div>
        </div>

        <template v-if="mode === 'one'">
          <div class="field">
            <label for="ca-q">{{ $t('Vorderseite') }}</label>
            <textarea id="ca-q" ref="questionEl" v-model="question" class="input" rows="2" autofocus :placeholder="$t('Was macht ein Multiplexer?')" @keydown="onKey($event, true)"></textarea>
          </div>
          <div class="field">
            <label for="ca-a">{{ $t('Rückseite') }}</label>
            <textarea id="ca-a" v-model="answer" class="input" rows="5" :placeholder="$t('Mehrere Zeilen gehen, dazu **Markdown** und $Formeln$.')" @keydown="onKey($event)"></textarea>
          </div>
          <label class="ca-check"><input v-model="both" type="checkbox" />{{ $t('Auch umgekehrt lernen (Rückseite → Vorderseite)') }}</label>
        </template>
        <template v-else>
          <div class="field">
            <label for="ca-list">{{ $t('Eine Karte pro Zeile: „Frage :: Antwort“ oder mit Tab getrennt (Anki, Quizlet)') }}</label>
            <textarea id="ca-list" v-model="text" class="input ca-list" rows="9" autofocus @keydown="onKey($event)"></textarea>
          </div>
          <div class="ca-count" aria-live="polite">
            <span class="ok">{{ tn(parsed.cards.length, '{n} Karte erkannt', '{n} Karten erkannt') }}</span>
            <span v-if="parsed.skipped" class="skip">{{ tn(parsed.skipped, '{n} Zeile ohne Trenner wird übersprungen', '{n} Zeilen ohne Trenner werden übersprungen') }}</span>
          </div>
          <div v-if="parsed.cards.length" class="ca-preview">
            <div v-for="(c, i) in parsed.cards.slice(0, 4)" :key="i" class="ca-prow"><span>{{ c.question }}</span><span>{{ c.answer }}</span></div>
            <div v-if="parsed.cards.length > 4" class="ca-prow ca-more">{{ $t('und {n} weitere', { n: parsed.cards.length - 4 }) }}</div>
          </div>
        </template>
        <div v-if="error" class="badge" role="alert">{{ error }}</div>
      </div>
      <div class="dlg-foot ca-foot">
        <span class="ca-hint">{{ mode === 'one' ? (added ? tn(added, '{n} Karte hinzugefügt', '{n} Karten hinzugefügt') : $t('⌘Enter fügt hinzu, das Fenster bleibt offen')) : '' }}</span>
        <button class="btn btn-ghost" type="button" @click="emit('close')">{{ added ? $t('Fertig') : $t('Abbrechen') }}</button>
        <button class="btn btn-primary" type="submit" :disabled="busy || !deck.trim() || (mode === 'list' ? !parsed.cards.length : !question.trim() || !answer.trim())">
          {{ mode === 'list' ? tn(parsed.cards.length, '{n} Karte hinzufügen', '{n} Karten hinzufügen') : $t('Hinzufügen') }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.ca { width: 640px; }
.ca-top { display: flex; align-items: flex-end; gap: 12px; }
.ca-deck { flex: 1; }
.ca-tabs { display: flex; padding: 3px; border-radius: var(--r-s); border: 1px solid var(--br-default); background: var(--bg-0); }
.ca-tabs button { height: 32px; padding: 0 12px; border: 0; border-radius: 6px; background: none; color: var(--tx-muted); font: 400 13px/1 var(--font-ui); cursor: pointer; white-space: nowrap; }
.ca-tabs button[aria-selected='true'] { background: var(--bg-3); color: var(--tx-primary); }
textarea.input { height: auto; resize: vertical; font: 400 14px/1.5 var(--font-ui); padding: 10px 12px; }
.ca-list { font: 400 13px/1.6 var(--font-mono); }
.ca-check { display: flex; align-items: center; gap: 10px; font: 400 13.5px/1.3 var(--font-ui); color: var(--tx-secondary); cursor: pointer; }
.ca-count { display: flex; flex-wrap: wrap; gap: 14px; font: 400 13px/1.3 var(--font-ui); }
.ca-count .ok { color: var(--p-green); }
.ca-count .skip { color: var(--p-yellow); }
.ca-preview { border: 1px solid var(--br-subtle); border-radius: var(--r-s); overflow: hidden; }
.ca-prow { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 14px; padding: 9px 14px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-secondary); }
.ca-prow + .ca-prow { border-top: 1px solid var(--br-subtle); }
.ca-prow span:first-child { color: var(--tx-primary); font-weight: 500; }
.ca-more { display: block; font-size: 12px; color: var(--tx-muted); }
.ca-foot { align-items: center; }
.ca-hint { margin-right: auto; font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
@media (max-width: 760px) {
  .ca-top { flex-direction: column; align-items: stretch; }
  .ca-hint { display: none; }
}
</style>
