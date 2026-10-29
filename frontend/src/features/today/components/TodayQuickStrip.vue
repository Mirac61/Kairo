<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import type { Card, Goal, TodayHabit } from '@/api/client'
import type { Block } from '@/composables/useHome'
import { hm } from '@/lib/dates'
import { locale } from '@/lib/i18n'

const props = defineProps<{
  habits: TodayHabit[]
  cards: Card[] | null
  goal: Goal | null
  events?: Block[]
  allDay?: Block[]
  nowMin?: number
}>()

const emit = defineEmits<{
  (e: 'toggleHabit', id: string, done: boolean): void
  (e: 'gradeCard', grade: number): void
}>()

const flipCard = ref(false)
const RING_CIRCUMFERENCE = 2 * Math.PI * 12

const habitsShown = computed(() => {
  return [...props.habits].sort((a, b) => Number(a.done) - Number(b.done)).slice(0, 4)
})

const currentCard = computed(() => props.cards?.[0])

const sortedEvents = computed(() => {
  return [...(props.events ?? [])].sort((a, b) => a.start - b.start)
})

function isCurrentEvent(b: Block): boolean {
  const now = props.nowMin ?? -1
  return b.start <= now && now < b.end
}

function isPastEvent(b: Block): boolean {
  const now = props.nowMin ?? -1
  return b.end <= now
}

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
  <aside class="st-side-stack" :aria-label="$t('Tagesübersicht')">
    <!-- 1. Kompakte Termine heute (Mini-Agenda) -->
    <section
      v-if="sortedEvents.length || allDay?.length"
      class="st-sec"
      :aria-label="$t('Termine')"
    >
      <div class="st-sec-h">
        <RouterLink to="/calendar" class="st-sec-title">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-cal" /></svg>
          <span>{{ $t('Termine') }}</span>
        </RouterLink>
        <span class="st-count">{{ sortedEvents.length + (allDay?.length ?? 0) }}</span>
      </div>

      <div class="st-agenda">
        <div v-for="b in allDay ?? []" :key="b.key" class="st-ev all-day">
          <span class="st-ev-bar"></span>
          <span class="st-ev-title">{{ b.title }}</span>
        </div>

        <div
          v-for="b in sortedEvents"
          :key="b.key"
          class="st-ev"
          :class="{ cur: isCurrentEvent(b), past: isPastEvent(b) }"
        >
          <span class="st-ev-bar"></span>
          <span class="st-ev-time">{{ hm(b.start) }}–{{ hm(b.end) }}</span>
          <span class="st-ev-title">{{ b.title }}</span>
          <span v-if="isCurrentEvent(b)" class="st-ev-now">{{ $t('Jetzt') }}</span>
        </div>
      </div>
    </section>

    <!-- 2. Gewohnheiten heute -->
    <section class="st-sec" :aria-label="$t('Gewohnheiten')">
      <div class="st-sec-h">
        <RouterLink to="/habits" class="st-sec-title">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-habits" /></svg>
          <span>{{ $t('Gewohnheiten') }}</span>
        </RouterLink>
        <span class="st-count">
          {{ habits.filter((h) => h.done).length }}/{{ habits.length }}
        </span>
      </div>

      <div class="st-sec-b">
        <button
          v-for="h in habitsShown"
          :key="h.id"
          type="button"
          class="st-habit"
          :class="{ done: h.done }"
          :aria-pressed="h.done"
          @click="emit('toggleHabit', h.id, h.done)"
        >
          <svg width="18" height="18" viewBox="0 0 30 30" aria-hidden="true">
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
                ? `${h.week_progress.done}/${h.week_progress.target}`
                : (h.preferred_time ?? '')
            }}
          </span>
        </button>

        <RouterLink v-if="habits.length > 4" to="/habits" class="st-more">
          {{ $t('{n} weitere', { n: habits.length - 4 }) }} →
        </RouterLink>
        <span v-if="!habits.length" class="st-empty">
          {{ $t('Heute keine Gewohnheiten.') }}
        </span>
      </div>
    </section>

    <!-- 3. Fokus-Ziel -->
    <section v-if="goal" class="st-sec" :aria-label="$t('Ziel')">
      <div class="st-sec-h">
        <RouterLink to="/goals" class="st-sec-title">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-goal" /></svg>
          <span>{{ $t('Fokus-Ziel') }}</span>
        </RouterLink>
        <span v-if="goal.progress && (goal.progress.percent ?? 0) > 0" class="st-count">
          {{ goal.progress.percent }}%
        </span>
        <span v-else-if="goal.due_date" class="st-count">{{ formatShortDate(goal.due_date) }}</span>
      </div>

      <div class="st-sec-b">
        <span class="st-goal-t">{{ goal.title }}</span>
        <p v-if="goal.why" class="st-goal-why">„{{ goal.why }}“</p>
        <div
          v-if="goal.progress && (goal.progress.percent ?? 0) > 0"
          class="st-segs"
          aria-hidden="true"
        >
          <i
            v-for="n in 8"
            :key="n"
            :class="{ on: n <= Math.round((goal.progress.percent ?? 0) / 12.5) }"
          ></i>
        </div>
      </div>
    </section>

    <!-- 4. Fällige Lernkarte (als eine Zeile statt Kasten im Kasten) -->
    <section v-if="currentCard" class="st-sec" :aria-label="$t('Lernkarte')">
      <div class="st-sec-h">
        <RouterLink to="/cards" class="st-sec-title">
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><use href="#i-cards" /></svg>
          <span>{{ $t('Lernkarte') }}</span>
        </RouterLink>
        <span class="st-count">{{ $t('{n} fällig', { n: cards?.length ?? 0 }) }}</span>
      </div>

      <div class="st-sec-b">
        <button v-if="!flipCard" type="button" class="st-card-line" @click="flipCard = true">
          <span class="st-q">{{ currentCard.question }}</span>
          <span class="st-flip-hint">· {{ $t('Umdrehen') }} →</span>
        </button>

        <div v-else class="st-card-open">
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
    </section>
  </aside>
</template>

<style scoped>
.st-side-stack {
  display: flex;
  flex-direction: column;
  font-variant-numeric: tabular-nums;
}
.st-sec {
  padding: 20px 0;
  border-top: 1px solid var(--br-subtle);
}
.st-sec:first-child {
  padding-top: 0;
  border-top: none;
}
.st-sec-h {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.st-sec-title {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
  text-decoration: none;
  transition: color var(--dur) ease;
}
.st-sec-title:hover {
  color: var(--tx-primary);
}
.st-sec-title .ic {
  width: 14px;
  height: 14px;
  color: var(--tx-muted);
}
.st-count {
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* 1. Mini-Agenda (Termine) */
.st-agenda {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.st-ev {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.st-ev.past {
  opacity: 0.45;
}
.st-ev-bar {
  width: 2px;
  height: 13px;
  border-radius: 999px;
  background: var(--br-strong);
  flex: none;
}
.st-ev.cur .st-ev-bar {
  background: var(--a-blue);
}
.st-ev-time {
  font-size: 12.5px;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.st-ev.cur .st-ev-time {
  color: var(--tx-secondary);
}
.st-ev-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 400;
}
.st-ev.cur .st-ev-title {
  font-weight: 500;
}
.st-ev-now {
  font: 500 12px/1 var(--font-ui);
  color: var(--a-blue-tx, var(--a-blue));
  white-space: nowrap;
}

/* 2. Gewohnheiten */
.st-sec-b {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.st-habit {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 5px 6px;
  margin: 0 -6px;
  border: 0;
  border-radius: var(--r-s);
  background: transparent;
  color: var(--tx-primary);
  font: 400 13.5px/1.35 var(--font-ui);
  cursor: pointer;
  text-align: left;
  transition: background var(--dur) ease;
}
.st-habit:hover {
  background: var(--bg-hover);
}
.st-habit.done .st-habit-n {
  color: var(--tx-muted);
  text-decoration: line-through;
}
.st-ring-bg {
  fill: none;
  stroke: var(--br-strong);
  stroke-width: 2.5;
}
.st-ring {
  fill: none;
  stroke: var(--tx-secondary);
  stroke-width: 2.5;
  transition: stroke-dashoffset 0.25s ease;
}
.st-ring-chk {
  fill: none;
  stroke: var(--tx-secondary);
  stroke-width: 2.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: opacity 0.2s ease;
}
.st-habit-n {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.st-habit-m {
  font-size: 12px;
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.st-more {
  display: inline-block;
  margin-top: 6px;
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
  text-decoration: none;
  transition: color var(--dur) ease;
}
.st-more:hover {
  color: var(--tx-primary);
}

/* 3. Fokus-Ziel */
.st-goal-t {
  font: 500 13.5px/1.35 var(--font-ui);
  color: var(--tx-primary);
}
.st-goal-why {
  margin: 3px 0 6px;
  font: 400 12.5px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.st-segs {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 3px;
  margin-top: 4px;
}
.st-segs i {
  height: 3px;
  border-radius: 2px;
  background: var(--br-default);
  transition: background 300ms;
}
.st-segs i.on {
  background: var(--tx-secondary);
}

/* 4. Lernkarte (als eine Zeile) */
.st-card-line {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 4px 6px;
  margin: 0 -6px;
  border: 0;
  border-radius: var(--r-s);
  background: transparent;
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  cursor: pointer;
  text-align: left;
  transition: background var(--dur) ease;
}
.st-card-line:hover {
  background: var(--bg-hover);
}
.st-q {
  color: var(--tx-primary);
}
.st-flip-hint {
  color: var(--tx-muted);
  font-size: 12.5px;
  white-space: nowrap;
}
.st-card-open {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 4px 0;
}
.st-a {
  font: 400 13px/1.45 var(--font-ui);
  color: var(--tx-secondary);
}
.st-grade {
  display: flex;
  gap: 8px;
}
.st-grade .btn {
  height: 26px;
  padding: 0 10px;
  font-size: 12px;
}
.st-empty {
  padding: 4px 0;
  color: var(--tx-muted);
  font: 400 12.5px/1.4 var(--font-ui);
}
</style>
