<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import type { Card, Goal, TodayHabit } from '@/api/client'
import { locale, t, tn } from '@/lib/i18n'

const props = defineProps<{
  habits: TodayHabit[]
  cards: Card[] | null
  goal: Goal | null
}>()

const emit = defineEmits<{
  (e: 'toggleHabit', id: string, done: boolean): void
  (e: 'gradeCard', grade: number): void
}>()

const openStrip = ref<'habits' | 'cards' | 'goal' | null>(null)
const flipCard = ref(false)
const RING_CIRCUMFERENCE = 2 * Math.PI * 12

const habitsShown = computed(() => {
  return [...props.habits]
    .sort((a, b) => Number(a.done) - Number(b.done))
    .slice(0, 4)
})

const currentCard = computed(() => props.cards?.[0])

const habitSummary = computed(() => {
  const openCount = props.habits.filter((h) => !h.done).length
  if (openCount > 0) return tn(openCount, '{n} Gewohnheit offen', '{n} Gewohnheiten offen')
  if (props.habits.length) return t('Alle Gewohnheiten erledigt')
  return t('Keine Gewohnheiten')
})

const cardSummary = computed(() => {
  const count = props.cards?.length ?? 0
  if (count > 0) return tn(count, '{n} Karte wartet', '{n} Karten warten')
  return t('Keine Karten fällig')
})

const goalSummary = computed(() => {
  if (!props.goal) return t('Kein Ziel')
  if (props.goal.progress) return t('Ziel {p}%', { p: props.goal.progress.percent })
  return props.goal.title
})

function formatShortDate(dateString: string): string {
  return new Intl.DateTimeFormat(locale.value, {
    day: 'numeric',
    month: 'short',
  }).format(new Date(`${dateString}T12:00:00`))
}

function handleGrade(gradeValue: number) {
  flipCard.value = false
  emit('gradeCard', gradeValue)
}
</script>

<template>
  <div class="today-strip-container">
    <!-- Kompakte Zeile: Gewohnheiten, Lernkarte, Fokus-Ziel -->
    <div class="st-strip" role="region" :aria-label="$t('Tagesübersicht')">
      <button
        type="button"
        class="st-strip-btn"
        :class="{ open: openStrip === 'habits' }"
        :aria-expanded="openStrip === 'habits'"
        @click="openStrip = openStrip === 'habits' ? null : 'habits'"
      >
        <svg class="ic" aria-hidden="true"><use href="#i-habits" /></svg>
        <span>{{ habitSummary }}</span>
      </button>

      <span class="st-strip-sep" aria-hidden="true">·</span>

      <button
        type="button"
        class="st-strip-btn"
        :class="{ open: openStrip === 'cards' }"
        :aria-expanded="openStrip === 'cards'"
        @click="openStrip = openStrip === 'cards' ? null : 'cards'"
      >
        <svg class="ic" aria-hidden="true"><use href="#i-cards" /></svg>
        <span>{{ cardSummary }}</span>
      </button>

      <span class="st-strip-sep" aria-hidden="true">·</span>

      <button
        type="button"
        class="st-strip-btn"
        :class="{ open: openStrip === 'goal' }"
        :aria-expanded="openStrip === 'goal'"
        @click="openStrip = openStrip === 'goal' ? null : 'goal'"
      >
        <svg class="ic" aria-hidden="true"><use href="#i-goal" /></svg>
        <span>{{ goalSummary }}</span>
      </button>
    </div>

    <!-- Ausklappbarer Detailbereich -->
    <div v-if="openStrip" class="st-strip-detail">
      <!-- 1. Gewohnheiten -->
      <section v-if="openStrip === 'habits'" class="st-card" :aria-label="$t('Gewohnheiten')">
        <div class="st-card-h">
          <RouterLink to="/habits">{{ $t('Gewohnheiten') }}</RouterLink>
          <span class="st-pill">
            {{ habits.filter((h) => h.done).length }}/{{ habits.length }} {{ $t('heute') }}
          </span>
        </div>

        <div class="st-card-b">
          <button
            v-for="h in habitsShown"
            :key="h.id"
            type="button"
            class="st-habit"
            :aria-pressed="h.done"
            @click="emit('toggleHabit', h.id, h.done)"
          >
            <svg width="24" height="24" viewBox="0 0 30 30" aria-hidden="true">
              <circle cx="15" cy="15" r="12" class="st-ring-bg" />
              <circle
                cx="15"
                cy="15"
                r="12"
                class="st-ring"
                :stroke-dasharray="RING_CIRCUMFERENCE"
                :stroke-dashoffset="h.done ? 0 : RING_CIRCUMFERENCE"
                transform="rotate(-90 15 15)"
              />
              <path
                d="M10.5 15.3l3 3 6-6.4"
                class="st-ring-chk"
                :style="{ opacity: h.done ? 1 : 0 }"
              />
            </svg>
            <span class="st-habit-n">{{ h.name }}</span>
            <span class="st-habit-m">
              {{
                h.week_progress
                  ? $t('{n} von {m}', { n: h.week_progress.done, m: h.week_progress.target })
                  : h.preferred_time ?? ''
              }}
            </span>
          </button>
          <RouterLink v-if="habits.length > 3" to="/habits" class="st-more">
            {{ $t('{n} weitere', { n: habits.length - 3 }) }}
          </RouterLink>
          <span v-if="!habits.length" class="st-empty">
            {{ $t('Heute keine Gewohnheiten.') }}
          </span>
        </div>

        <div class="st-card-f">
          <span v-if="habits.length && habits.every((h) => h.done)" class="st-all-done">
            ✓ {{ $t('Alles für heute erledigt') }}
          </span>
          <span
            v-else-if="habits.length === 1 && habits[0]?.preferred_time"
            class="st-meta-note"
          >
            {{ $t('Geplant um {time} Uhr', { time: habits[0].preferred_time }) }}
          </span>
          <span v-else-if="habits.length" class="st-meta-note">
            {{ $tn(habits.filter((h) => !h.done).length, '{n} noch offen', '{n} noch offen') }}
          </span>
        </div>
      </section>

      <!-- 2. Lernkarte -->
      <section v-else-if="openStrip === 'cards'" class="st-card" :aria-label="$t('Lernkarte')">
        <div class="st-card-h">
          <RouterLink to="/cards">{{ $t('Lernkarte') }}</RouterLink>
          <span v-if="currentCard?.deck" class="st-pill">{{ currentCard.deck }}</span>
          <span v-else-if="cards?.length" class="st-pill">{{ $t('{n} fällig', { n: cards.length }) }}</span>
        </div>

        <div v-if="currentCard" class="st-card-b">
          <div class="st-flip" :class="{ on: flipCard }" @click="flipCard = !flipCard">
            <div class="st-face st-front" :aria-hidden="flipCard">
              <span class="st-q">{{ currentCard.question }}</span>
            </div>
            <div class="st-face st-back" :aria-hidden="!flipCard">
              <span class="st-a">{{ currentCard.answer }}</span>
              <div class="st-grade">
                <button type="button" class="btn btn-secondary" @click.stop="handleGrade(0)">
                  {{ $t('Nochmal') }}
                </button>
                <button type="button" class="btn btn-primary" @click.stop="handleGrade(2)">
                  {{ $t('Gewusst') }}
                </button>
              </div>
            </div>
          </div>
        </div>
        <div v-else-if="cards" class="st-card-b">
          <div class="st-empty">
            <b>{{ $t('Für heute gelernt.') }}</b>
            {{ $t('Die nächste Karte kommt, wenn sie fällig ist.') }}
          </div>
        </div>

        <div v-if="currentCard" class="st-card-f">
          <span class="st-meta-note">
            {{ $tn(cards?.length ?? 0, '{n} Karte wartet', '{n} Karten warten') }}
          </span>
          <span class="st-hint">
            <kbd>{{ $t('Leertaste') }}</kbd> {{ $t('Umdrehen') }}
          </span>
        </div>
      </section>

      <!-- 3. Fokus-Ziel -->
      <section v-else-if="openStrip === 'goal'" class="st-card" :aria-label="$t('Ziel')">
        <div class="st-card-h">
          <RouterLink to="/goals">{{ $t('Fokus-Ziel') }}</RouterLink>
          <span v-if="goal?.due_date" class="st-pill">{{ formatShortDate(goal.due_date) }}</span>
          <span v-else-if="goal?.progress" class="st-pill">{{ goal.progress.percent }}%</span>
        </div>

        <div v-if="goal" class="st-card-b">
          <span class="st-goal-t">{{ goal.title }}</span>
          <p v-if="goal.why" class="st-goal-why">„{{ goal.why }}“</p>
          <div v-if="goal.progress" class="st-segs" aria-hidden="true">
            <i v-for="n in 8" :key="n" :class="{ on: n <= Math.round((goal.progress.percent ?? 0) / 12.5) }"></i>
          </div>
        </div>
        <div v-else class="st-card-b">
          <div class="st-empty">
            {{ $t('Kein Ziel') }}
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.today-strip-container {
  margin-top: 24px;
}
.st-strip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 14px;
  border-radius: var(--r-m);
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  font: 400 13px/1 var(--font-ui);
}
.st-strip-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--tx-secondary);
  font: inherit;
  cursor: pointer;
  transition: background var(--dur), color var(--dur);
}
.st-strip-btn:hover,
.st-strip-btn.open {
  background: var(--bg-hover);
  color: var(--tx-primary);
}
.st-strip-btn .ic {
  color: var(--tx-muted);
}
.st-strip-sep {
  color: var(--br-strong);
}
.st-strip-detail {
  margin-top: 14px;
}

.st-card {
  padding: 18px 20px;
  border-radius: var(--r-m);
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
}
.st-card-h {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
  font: 600 14px/1 var(--font-ui);
}
.st-card-h a {
  color: var(--tx-primary);
  text-decoration: none;
}
.st-card-h a:hover {
  text-decoration: underline;
}
.st-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 20px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-subtle);
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.st-card-b {
  display: grid;
  gap: 8px;
}
.st-habit {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  cursor: pointer;
  text-align: left;
}
.st-habit:hover {
  background: var(--bg-hover);
}
.st-ring-bg {
  fill: none;
  stroke: var(--br-default);
  stroke-width: 2.5;
}
.st-ring {
  fill: none;
  stroke: var(--a-green);
  stroke-width: 2.5;
  transition: stroke-dashoffset 0.25s ease;
}
.st-ring-chk {
  fill: none;
  stroke: var(--a-green);
  stroke-width: 2.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: opacity 0.2s ease;
}
.st-habit-n {
  flex: 1;
}
.st-habit-m {
  font-size: 12px;
  color: var(--tx-muted);
}
.st-more {
  display: inline-block;
  margin-top: 4px;
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
  text-decoration: none;
}
.st-more:hover {
  color: var(--tx-primary);
}
.st-card-f {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px solid var(--br-subtle);
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.st-all-done {
  color: var(--a-green);
  font-weight: 500;
}
.st-meta-note {
  font-size: 12px;
}
.st-hint {
  font-size: 11px;
}
.st-hint kbd {
  padding: 2px 4px;
  border-radius: 4px;
  background: var(--bg-2);
  border: 1px solid var(--br-default);
}

.st-flip {
  perspective: 1000px;
  min-height: 120px;
  cursor: pointer;
}
.st-face {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 120px;
  padding: 16px;
  border-radius: 8px;
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-default);
}
.st-front .st-q {
  font: 500 15px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.st-back {
  display: none;
}
.st-flip.on .st-front {
  display: none;
}
.st-flip.on .st-back {
  display: flex;
}
.st-back .st-a {
  font: 400 14px/1.5 var(--font-ui);
  color: var(--tx-secondary);
  margin-bottom: 14px;
}
.st-grade {
  display: flex;
  gap: 8px;
}

.st-goal-t {
  font: 500 14px/1.35 var(--font-ui);
  color: var(--tx-primary);
}
.st-goal-why {
  margin: 4px 0 6px;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.st-segs {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 3px;
  margin-top: 4px;
}
.st-segs i {
  height: 4px;
  border-radius: 2px;
  background: var(--br-default);
  transition: background 300ms;
}
.st-segs i.on {
  background: var(--tx-secondary);
}
.st-empty {
  padding: 14px 0;
  text-align: center;
  color: var(--tx-muted);
  font: 400 13px/1.4 var(--font-ui);
}
</style>
