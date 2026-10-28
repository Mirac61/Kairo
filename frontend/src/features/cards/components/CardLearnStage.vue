<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { Card } from '@/api/client'
import { intervalText } from '@/lib/cards'
import CardText from './CardText.vue'

defineProps<{
  card: Card
  shown: boolean
  reverse: boolean
  practice: boolean
  done: number
  total: number
  busy: boolean
  front: string
  back: string
  colorOf: (deck: string) => string
  grades: string[]
}>()

defineEmits<{
  reveal: []
  grade: [grade: number]
}>()
</script>

<template>
  <div class="cl-stage">
    <article
      class="cl-card"
      :class="{ 'is-hidden': !shown, 'is-shown': shown }"
      :style="{ '--dc': colorOf(card.deck) }"
      :aria-label="$t('Karte')"
      @click="!shown && $emit('reveal')"
    >
      <div class="cl-meta">
        <div class="cl-meta-left">
          <span class="cl-chip"><i></i>{{ card.deck }}</span>
          <span v-if="card.new" class="cl-tag c-new">{{ $t('Neu') }}</span>
          <span v-else-if="card.learning" class="cl-tag c-learn">{{ $t('Lernen') }}</span>
          <span v-if="reverse" class="cl-tag c-rev">{{ $t('Antwort → Frage') }}</span>
        </div>
        <span class="cl-pos mono">{{ done + 1 }} / {{ total }}</span>
      </div>

      <div class="cl-body">
        <p v-if="practice" class="cl-practice">
          {{ $t('Übungsrunde · Bewertungen ändern den Lernplan nicht') }}
        </p>

        <!-- Vor dem Aufdecken: Nur Frage im Zentrum -->
        <div v-if="!shown" class="cl-front-wrap">
          <CardText class="cl-front" :text="front" />
          <div class="cl-hint">
            <span>{{ $t('Klicken oder Leertaste drücken') }}</span>
          </div>
        </div>

        <!-- Nach dem Aufdecken: Frage oben kompakt, Antwort dominant im Zentrum -->
        <div v-else class="cl-revealed-wrap">
          <div class="cl-q-context">
            <span class="cl-sec-label">{{ $t('Frage') }}</span>
            <CardText class="cl-front-compact" :text="front" />
          </div>
          <div class="cl-divider" aria-hidden="true"></div>
          <div class="cl-a-context">
            <span class="cl-sec-label cl-a-label">{{ $t('Antwort') }}</span>
            <CardText class="cl-back-text" :text="back" />
            <RouterLink
              v-if="card.note_path"
              class="col-link cl-src"
              :to="{ path: '/notes', query: { open: card.note_path } }"
            >
              {{ $t('aus {path}', { path: card.note_path }) }}
            </RouterLink>
          </div>
        </div>
      </div>
    </article>

    <div class="cl-actions">
      <button v-if="!shown" type="button" class="btn btn-primary cl-show" @click="$emit('reveal')">
        <span>{{ $t('Antwort zeigen') }}</span>
        <kbd>{{ $t('Leertaste') }}</kbd>
      </button>

      <template v-else>
        <button
          v-for="(g, i) in grades"
          :key="i"
          type="button"
          class="cl-grade"
          :class="[`g${i}`, { 'is-default': i === 2 }]"
          :disabled="busy"
          @click="$emit('grade', i)"
        >
          <div class="cl-grade-top">
            <span class="cl-grade-name">{{ g }}</span>
            <kbd class="cl-grade-kbd">{{ i + 1 }}<template v-if="i === 2"> · ␣</template></kbd>
          </div>
          <div class="cl-grade-interval">
            <b class="mono">{{ practice ? '–' : intervalText(card.next_intervals[i]!) }}</b>
          </div>
        </button>
      </template>
    </div>
  </div>
</template>
