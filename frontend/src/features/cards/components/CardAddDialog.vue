<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { createCard, errorMessage, type Deck } from '@/api/client'
import { parseCardList } from '@/lib/cards'
import { vDialog } from '@/lib/dialog'
import { store } from '@/lib/storage'
import { tn } from '@/lib/i18n'
import { keys } from '@/lib/keys'
import CardFormulaInput from './CardFormulaInput.vue'
import CardInlineText from './CardInlineText.vue'

// Ruhiger Fokus-Dialog (Linear-Stil): Fach + Modus oben, randlose Vorderseite & Rückseite mit bedingter LaTeX-Vorschau.
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
const questionEl = ref<InstanceType<typeof CardFormulaInput>>()
const answerEl = ref<InstanceType<typeof CardFormulaInput>>()
const listEl = ref<HTMLTextAreaElement>()

const parsed = computed(() => parseCardList(text.value))

watch(mode, async (m) => {
  await nextTick()
  if (m === 'one') questionEl.value?.focus()
  else listEl.value?.focus()
})

async function save() {
  const d = deck.value.trim()
  const items =
    mode.value === 'one'
      ? [{ question: question.value.trim(), answer: answer.value.trim() }].filter(
          (c) => c.question && c.answer,
        )
      : [...parsed.value.cards]
  if (!d || !items.length || busy.value) return
  if (mode.value === 'one' && both.value)
    items.push({ question: items[0]!.answer, answer: items[0]!.question })
  busy.value = true
  let created = 0
  try {
    // shortcut: eine Anfrage pro Karte; bei Importen über ein paar hundert eine Sammel-Route bauen.
    for (const c of items) {
      // oxlint-disable-next-line eslint/no-await-in-loop -- sequenziell für SQLite & Teilfehler-Tracking
      await createCard({ deck: d, ...c })
      created++
    }
    store.set('kairo-cards-deck', d)
    added.value += created
    error.value = ''
    emit('added')
    if (mode.value === 'list') return emit('close')
    question.value = answer.value = ''
    await nextTick()
    questionEl.value?.focus()
  } catch (e) {
    if (created > 0) {
      added.value += created
      emit('added')
      if (mode.value === 'list') {
        text.value = items
          .slice(created)
          .map((c) => `${c.question} :: ${c.answer}`)
          .join('\n')
      }
    }
    error.value = errorMessage(e)
  }
  busy.value = false
}

// ⌘Enter speichert aus jedem Feld; Enter in der Frage springt zur leeren Antwort oder speichert.
function onKey(e: KeyboardEvent, inQuestion = false) {
  if (e.key !== 'Enter' || e.shiftKey) return
  if (e.metaKey || e.ctrlKey) {
    e.preventDefault()
    void save()
    return
  }
  if (inQuestion) {
    e.preventDefault()
    if (!answer.value.trim()) answerEl.value?.focus()
    else void save()
  }
}
</script>

<template>
  <div v-dialog="() => emit('close')" class="overlay open" @mousedown.self="emit('close')">
    <form class="dialog ca" :aria-label="$t('Karten hinzufügen')" @submit.prevent="save">
      <div class="ca-head">
        <div class="ca-deck-wrap">
          <input
            id="ca-deck"
            v-model="deck"
            class="ca-deck-in"
            list="ca-decks"
            required
            :aria-label="$t('Fach')"
            :placeholder="$t('Neues Fach, z. B. Mathe 2')"
          />
          <datalist id="ca-decks">
            <option v-for="d in decks" :key="d.name" :value="d.name" />
          </datalist>
        </div>

        <div class="ca-head-r">
          <div class="seg ca-tabs" role="tablist" :aria-label="$t('Eingabe')">
            <button type="button" role="tab" :aria-selected="mode === 'one'" @click="mode = 'one'">
              {{ $t('Einzeln') }}
            </button>
            <button
              type="button"
              role="tab"
              :aria-selected="mode === 'list'"
              @click="mode = 'list'"
            >
              {{ $t('Liste einfügen') }}
            </button>
          </div>
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Schließen')"
            @click="emit('close')"
          >
            <svg class="ic"><use href="#i-x" /></svg>
          </button>
        </div>
      </div>

      <div class="ca-body">
        <template v-if="mode === 'one'">
          <CardFormulaInput
            id="ca-q"
            ref="questionEl"
            v-model="question"
            variant="title"
            :rows="1"
            autofocus
            :label="$t('Vorderseite')"
            :placeholder="$t('Vorderseite')"
            @keydown="onKey($event, true)"
          />
          <div class="ca-sep" aria-hidden="true"></div>
          <CardFormulaInput
            id="ca-a"
            ref="answerEl"
            v-model="answer"
            variant="bare"
            :rows="6"
            :label="$t('Rückseite')"
            :placeholder="$t('Mehrere Zeilen gehen, dazu **Markdown** und $Formeln$.')"
            @keydown="onKey($event)"
          />
        </template>

        <template v-else>
          <label for="ca-list" class="ca-list-lbl">
            {{
              $t('Eine Karte pro Zeile: „Frage :: Antwort“ oder mit Tab getrennt (Anki, Quizlet)')
            }}
          </label>
          <textarea
            id="ca-list"
            ref="listEl"
            v-model="text"
            class="ca-list"
            rows="9"
            @keydown="onKey($event)"
          ></textarea>
          <div class="ca-count" aria-live="polite">
            <span class="ok">{{
              tn(parsed.cards.length, '{n} Karte erkannt', '{n} Karten erkannt')
            }}</span>
            <span v-if="parsed.skipped" class="skip">{{
              tn(
                parsed.skipped,
                '{n} Zeile ohne Trenner wird übersprungen',
                '{n} Zeilen ohne Trenner werden übersprungen',
              )
            }}</span>
          </div>
          <div v-if="parsed.cards.length" class="ca-preview">
            <div v-for="(c, i) in parsed.cards.slice(0, 4)" :key="i" class="ca-prow">
              <span><CardInlineText :text="c.question" /></span
              ><span><CardInlineText :text="c.answer" /></span>
            </div>
            <div v-if="parsed.cards.length > 4" class="ca-prow ca-more">
              {{ $t('und {n} weitere', { n: parsed.cards.length - 4 }) }}
            </div>
          </div>
        </template>

        <div v-if="error" class="badge" role="alert">{{ error }}</div>
      </div>

      <div class="ca-foot">
        <label v-if="mode === 'one'" class="ca-rev">
          <button
            type="button"
            role="switch"
            class="ca-switch"
            :aria-checked="both"
            @click="both = !both"
          >
            <i></i>
          </button>
          <span>{{ $t('Auch umgekehrt lernen (Rückseite → Vorderseite)') }}</span>
        </label>

        <div class="ca-foot-r">
          <span v-if="added" class="ca-hint">{{
            tn(added, '{n} Karte hinzugefügt', '{n} Karten hinzugefügt')
          }}</span>
          <button class="btn btn-ghost" type="button" @click="emit('close')">
            {{ added ? $t('Fertig') : $t('Abbrechen') }}
          </button>
          <button
            class="btn btn-primary"
            type="submit"
            :title="
              mode === 'one' ? keys($t('⌘Enter fügt hinzu, das Fenster bleibt offen')) : undefined
            "
            :disabled="
              busy ||
              !deck.trim() ||
              (mode === 'list' ? !parsed.cards.length : !question.trim() || !answer.trim())
            "
          >
            {{
              mode === 'list'
                ? tn(parsed.cards.length, '{n} Karte hinzufügen', '{n} Karten hinzufügen')
                : $t('Hinzufügen')
            }}
          </button>
        </div>
      </div>
    </form>
  </div>
</template>

<style scoped>
.ca {
  width: 720px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
}
.ca-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px 12px 22px;
  border-bottom: 1px solid var(--br-subtle);
}
.ca-deck-wrap {
  flex: 1;
  min-width: 0;
}
.ca-deck-in {
  width: 100%;
  max-width: 280px;
  height: 30px;
  padding: 0 8px;
  margin-left: -8px;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--tx-primary);
  font: 500 13.5px/1 var(--font-ui);
  transition:
    background var(--dur) var(--ease),
    border-color var(--dur) var(--ease);
}
.ca-deck-in::placeholder {
  color: var(--tx-muted);
  font-weight: 400;
}
.ca-deck-in:hover {
  background: var(--bg-2);
}
.ca-deck-in:focus-visible {
  outline: none;
  box-shadow: none;
  border-radius: 0;
  background: transparent;
  border-bottom-color: var(--a-focus);
}
.ca-head-r {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.ca-tabs button[aria-selected='true'] {
  background: var(--bg-3);
  color: var(--tx-primary);
}
.ca-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 22px 20px;
  max-height: 68vh;
  overflow-y: auto;
}
.ca-sep {
  height: 1px;
  background: var(--br-subtle);
  margin: 2px 0;
}
.ca-list-lbl {
  margin: 0;
  font: 400 12.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.ca-list {
  width: 100%;
  padding: 8px 0;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: transparent;
  color: var(--tx-primary);
  font: 400 13px/1.6 var(--font-mono);
  resize: vertical;
  transition: border-color var(--dur) var(--ease);
}
.ca-list:focus-visible {
  outline: none;
  box-shadow: none;
  border-bottom-color: var(--a-focus);
}
.ca-count {
  display: flex;
  gap: 12px;
  font: 400 12.5px/1 var(--font-ui);
}
.ca-count .ok {
  color: var(--tx-secondary);
}
.ca-count .skip {
  color: var(--a-red);
}
.ca-preview {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-s);
  overflow: hidden;
  background: var(--bg-0);
}
.ca-prow {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  padding: 7px 10px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12.5px/1.3 var(--font-ui);
  color: var(--tx-secondary);
}
.ca-prow:first-child {
  border-top: 0;
}
.ca-prow span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ca-prow span:first-child {
  color: var(--tx-primary);
}
.ca-more {
  grid-template-columns: 1fr;
  color: var(--tx-muted);
}
.ca-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px 14px 22px;
  border-top: 1px solid var(--br-subtle);
  background: var(--bg-0);
}
.ca-rev {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font: 400 12.5px/1.3 var(--font-ui);
  color: var(--tx-secondary);
  cursor: pointer;
}
.ca-switch {
  position: relative;
  width: 30px;
  height: 17px;
  padding: 0;
  border: 1px solid var(--br-default);
  border-radius: 999px;
  background: var(--bg-2);
  cursor: pointer;
  flex: none;
  transition:
    background var(--dur) var(--ease),
    border-color var(--dur) var(--ease);
}
.ca-switch i {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: var(--tx-muted);
  transition:
    transform var(--dur) var(--ease),
    background var(--dur) var(--ease);
}
.ca-switch[aria-checked='true'] {
  background: var(--tx-primary);
  border-color: var(--tx-primary);
}
.ca-switch[aria-checked='true'] i {
  transform: translateX(13px);
  background: var(--bg-0);
}
.ca-foot-r {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-left: auto;
}
.ca-hint {
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  margin-right: 4px;
}

@media (max-width: 640px) {
  .ca-head {
    flex-wrap: wrap;
    padding: 12px 14px;
  }
  .ca-body {
    padding: 14px;
  }
  .ca-foot {
    padding: 12px 14px;
  }
}
</style>
