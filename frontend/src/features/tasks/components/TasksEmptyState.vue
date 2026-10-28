<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@/lib/i18n'

const props = defineProps<{
  filter: string
  search: string
  projectId: string
  emptyMessages?: Record<string, string>
  examples?: readonly string[]
}>()

const emit = defineEmits<{
  (e: 'selectExample', example: string): void
}>()

const DEFAULT_EMPTY_MESSAGES = computed<Record<string, string>>(() => ({
  alle: t('Alles erledigt – oder noch nichts angelegt.'),
  ueber: t('Nichts überfällig.'),
  heute: t('Heute ist nichts offen.'),
  woche: t('Die nächsten 7 Tage sind frei.'),
  spaeter: t('Dafür ist noch nichts eingeplant.'),
  ohne: t('Alles hat ein Datum.'),
  erledigt: t('Noch nichts erledigt.'),
}))

const DEFAULT_EXAMPLES = computed<readonly string[]>(() => [
  t('Sport 30m @morgen'),
  t('Steuererklärung 2h @fr !hoch'),
  t('Mails beantworten 15m @heute'),
])

const activeMessages = computed(() => props.emptyMessages ?? DEFAULT_EMPTY_MESSAGES.value)
const activeExamples = computed(() => props.examples ?? DEFAULT_EXAMPLES.value)
</script>

<template>
  <div class="tk-empty">
    <span class="tk-empty-ic" aria-hidden="true">
      <svg class="ic"><use href="#i-check" /></svg>
    </span>
    <p class="tk-empty-h">
      {{
        search || projectId !== 'alle'
          ? $t('Keine Aufgaben.')
          : (activeMessages[filter] ?? $t('Keine Aufgaben.'))
      }}
    </p>
    <template v-if="(filter === 'alle' || filter === 'ohne') && !search">
      <p class="tk-ex-h">{{ $t('Probier eine Zeile aus – ein Klick füllt sie ins Feld.') }}</p>
      <div class="tk-ex">
        <button
          v-for="example in activeExamples"
          :key="example"
          type="button"
          @click="emit('selectExample', example)"
        >
          <span
            v-for="(word, i) in example.split(' ')"
            :key="i"
            :class="{ tok: /^[#@!]|^\d+[mh]$/.test(word) }"
            >{{ word }}</span
          >
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.tk-empty {
  display: grid;
  gap: 8px;
  justify-items: center;
  text-align: center;
  padding: 64px 0 48px;
}
.tk-empty-ic {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin-bottom: 8px;
  border-radius: 50%;
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  color: var(--tx-muted);
}
.tk-empty-ic .ic {
  width: 20px;
  height: 20px;
}
.tk-empty-h {
  margin: 0;
  font: 500 15px/1.4 var(--font-ui);
  letter-spacing: -0.005em;
  color: var(--tx-primary);
}
.tk-ex-h {
  margin: 0 0 8px;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.tk-ex {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  max-width: 560px;
}
.tk-ex button {
  display: inline-flex;
  gap: 5px;
  height: 32px;
  align-items: center;
  padding: 0 14px;
  border: 0;
  border-radius: 999px;
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  color: var(--tx-secondary);
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
  transition:
    background var(--dur),
    box-shadow var(--dur),
    color var(--dur);
}
.tk-ex button:hover {
  background: var(--bg-2);
  box-shadow: 0 0 0 1px var(--br-strong);
  color: var(--tx-primary);
}
.tk-ex .tok {
  color: var(--tx-primary);
  font-weight: 500;
}
</style>
