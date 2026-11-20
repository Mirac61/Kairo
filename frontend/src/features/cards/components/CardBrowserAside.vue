<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { Card, Deck } from '@/api/client'
import { intervalText } from '@/lib/cards'
import { locale } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'
import CardFormulaInput from './CardFormulaInput.vue'
import CardText from './CardText.vue'

// Rechtes Seitenpanel für die gewählte Karte im Kartenbrowser.
const props = defineProps<{
  card: Card
  decks: Deck[]
  dueText: string
  stage: number
  lapsesCount: number
}>()

const emit = defineEmits<{
  close: []
  save: [card: Card, body: { deck: string; question: string; answer: string }]
  remove: [card: Card]
}>()

const form = ref({ deck: '', question: '', answer: '' })
const previewMode = ref(false)
const closeBtn = ref<HTMLButtonElement>()

watch(
  () => props.card,
  (c, prev) => {
    form.value = { deck: c.deck, question: c.question, answer: c.answer }
    if (c.key !== prev?.key) {
      previewMode.value = false
      if (typeof matchMedia === 'function' && matchMedia('(max-width: 760px)').matches) {
        void nextTick(() => closeBtn.value?.focus())
      }
    }
  },
  { immediate: true },
)

const createdFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }),
)

function save() {
  const c = props.card
  const body = {
    deck: form.value.deck.trim(),
    question: form.value.question.trim(),
    answer: form.value.answer.trim(),
  }
  if (!c.id || !body.deck || !body.question || !body.answer) return
  if (body.deck === c.deck && body.question === c.question && body.answer === c.answer) return
  emit('save', c, body)
}
</script>

<template>
  <aside class="kb-aside" :aria-label="$t('Karte bearbeiten')">
    <div class="aside-head">
      <span class="lbl">{{ $t('Karte') }}</span>
      <div class="kb-aside-head-r">
        <button
          v-if="card.id"
          type="button"
          class="btn btn-ghost btn-sm"
          @click="previewMode = !previewMode"
        >
          {{ previewMode ? $t('Bearbeiten') : $t('Vorschau') }}
        </button>
        <button
          ref="closeBtn"
          class="icon-btn"
          type="button"
          :aria-label="$t('Schließen')"
          @click="emit('close')"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
    </div>

    <div class="kb-aside-body">
      <template v-if="card.id">
        <div v-if="!previewMode" class="kb-edit-fields">
          <label for="kb-q" class="kb-sec-lbl">{{ $t('Vorderseite') }}</label>
          <CardFormulaInput
            id="kb-q"
            v-model="form.question"
            variant="title"
            :rows="2"
            :label="$t('Vorderseite')"
            :placeholder="$t('Vorderseite')"
            @change="save"
          />
          <div class="kb-aside-sep" aria-hidden="true"></div>
          <label for="kb-a" class="kb-sec-lbl">{{ $t('Rückseite') }}</label>
          <CardFormulaInput
            id="kb-a"
            v-model="form.answer"
            variant="bare"
            :rows="5"
            :label="$t('Rückseite')"
            :placeholder="$t('Rückseite')"
            @change="save"
          />
        </div>
        <div v-else class="kb-preview-card">
          <span class="kb-sec-lbl">{{ $t('Frage') }}</span>
          <CardText class="kb-preview-front" :text="form.question || '–'" />
          <div class="kb-aside-sep" aria-hidden="true"></div>
          <span class="kb-sec-lbl">{{ $t('Antwort') }}</span>
          <CardText class="kb-preview-back" :text="form.answer || '–'" />
        </div>
      </template>
      <div v-else class="kb-ro">
        <span class="kb-sec-lbl">{{ $t('Frage') }}</span>
        <CardText class="kb-ro-q" :text="card.question" />
        <div class="kb-aside-sep" aria-hidden="true"></div>
        <span class="kb-sec-lbl">{{ $t('Antwort') }}</span>
        <CardText class="kb-ro-a" :text="card.answer" />
        <p class="kb-hint">{{ $t('Karten aus Notizen änderst du in der Notiz.') }}</p>
      </div>

      <div class="kb-props">
        <label v-if="card.id" class="kb-prop-row">
          <span>{{ $t('Fach') }}</span>
          <input id="kb-deck" v-model="form.deck" class="input" list="kb-decks" @change="save" />
          <datalist id="kb-decks">
            <option v-for="d in decks" :key="d.name" :value="d.name" />
          </datalist>
        </label>
        <div v-else class="kb-prop-row">
          <span>{{ $t('Fach') }}</span>
          <div class="kb-val">{{ card.deck }}</div>
        </div>
        <div class="kb-prop-row">
          <span>{{ $t('Fällig') }}</span>
          <div class="kb-val" :class="{ due: card.due && !card.new }">{{ dueText }}</div>
        </div>
        <div class="kb-prop-row">
          <span>{{ $t('Stufe') }}</span>
          <div class="kb-val">
            {{ $t('{n} von 4', { n: stage })
            }}<template v-if="card.interval_days && !card.new">
              · {{ intervalText(card.interval_days) }}</template
            >
          </div>
        </div>
        <div v-if="lapsesCount" class="kb-prop-row">
          <span>{{ $t('Vergessen') }}</span>
          <div class="kb-val">{{ lapsesCount }}×</div>
        </div>
        <div v-if="card.created_at" class="kb-prop-row">
          <span>{{ $t('Erstellt') }}</span>
          <div class="kb-val">{{ createdFmt.format(new Date(card.created_at)) }}</div>
        </div>
      </div>

      <p v-if="card.id" class="kb-hint">
        {{
          $t(
            'Markdown, $Formeln$ und Bilder gehen. Speichert beim Verlassen des Felds; eine neue Frage beginnt von vorn.',
          )
        }}
      </p>
    </div>

    <div class="kb-aside-foot">
      <DeleteButton
        v-if="card.id"
        ghost
        :text="$t('Karte löschen?')"
        @confirm="emit('remove', card)"
      />
      <router-link
        v-else
        class="btn btn-secondary btn-sm"
        :to="{ path: '/notes', query: { open: card.note_path } }"
        >{{ $t('Notiz öffnen') }}</router-link
      >
    </div>
  </aside>
</template>

<style scoped>
.kb-aside {
  width: 420px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-1);
  border-left: 1px solid var(--br-subtle);
  position: sticky;
  top: 0;
  height: 100dvh;
}
.aside-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px 0 24px;
}
.kb-aside-head-r {
  display: flex;
  align-items: center;
  gap: 6px;
}
.kb-aside-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px 20px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.kb-edit-fields,
.kb-preview-card,
.kb-ro {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.kb-sec-lbl {
  display: block;
  font: 500 10.5px/1 var(--font-ui);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.kb-aside-sep {
  height: 1px;
  background: var(--br-subtle);
  margin: 10px 0;
}
.kb-preview-front,
.kb-ro-q {
  font: 500 17px/1.45 var(--font-ui);
  color: var(--tx-primary);
}
.kb-preview-back,
.kb-ro-a {
  font: 400 15px/1.6 var(--font-ui);
  color: var(--tx-secondary);
}
.kb-hint {
  margin: 0;
  font: 400 12px/1.5 var(--font-ui);
  color: var(--tx-muted);
}
.kb-props {
  border-top: 1px solid var(--br-subtle);
}
.kb-prop-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  border-bottom: 1px solid var(--br-subtle);
  font: 400 13px/1 var(--font-ui);
}
.kb-prop-row > span {
  flex: none;
  width: 92px;
  color: var(--tx-muted);
}
.kb-val {
  flex: 1;
  min-width: 0;
  padding: 0 6px;
  color: var(--tx-primary);
}
.kb-val.due {
  color: var(--a-blue-tx);
  font-weight: 500;
}
.kb-prop-row .input {
  flex: 1;
  min-width: 0;
  width: auto;
  height: 28px;
  padding: 0 6px;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: transparent;
  color: var(--tx-primary);
  font: 400 13px/1 var(--font-ui);
}
.kb-prop-row .input:hover {
  background: var(--bg-2);
  border-radius: 6px;
}
.kb-prop-row .input:focus-visible {
  outline: none;
  box-shadow: none;
  border-radius: 0;
  background: transparent;
  border-bottom-color: var(--a-focus);
}
.kb-aside-foot {
  padding: 12px 24px max(16px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--br-subtle);
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 8px;
}
.btn-sm {
  height: 28px;
  padding: 0 10px;
  font-size: 12.5px;
}
@media (max-width: 760px) {
  .kb-aside {
    position: fixed;
    inset: 0;
    width: 100%;
    height: auto;
    z-index: 95;
    border-left: 0;
  }
}
</style>
