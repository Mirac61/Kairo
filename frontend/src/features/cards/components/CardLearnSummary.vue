<script setup lang="ts">
import { computed } from 'vue'
import type { Card, CardLog, Deck } from '@/api/client'
import { forecast, intervalText, perDay } from '@/lib/cards'
import { addDays, ymd } from '@/lib/dates'
import { tn } from '@/lib/i18n'

const props = defineProps<{
  results: Map<string, { card: Card; grades: number[]; next: number | null }>
  cards: Card[]
  decks: Deck[]
  log: CardLog[]
  colorOf: (deck: string) => string
  minutes: number
  grades: string[]
}>()

const emit = defineEmits<{
  exit: []
  again: [keys: string[]]
}>()

const gradeCounts = computed(() => {
  const n = [0, 0, 0, 0]
  for (const r of props.results.values()) for (const g of r.grades) n[g]!++
  return n
})

const roundList = computed(() =>
  [...props.results.values()].sort(
    (a, b) => Math.min(...a.grades) - Math.min(...b.grades) || b.grades.length - a.grades.length,
  ),
)

const hard = computed(() =>
  roundList.value.filter((r) => r.grades.some((g) => g <= 1)).map((r) => r.card.key),
)

const answerText = (gs: number[]) => {
  const again = gs.filter((g) => g === 0).length
  return again ? tn(again, '{n}× nochmal', '{n}× nochmal') : props.grades[gs.at(-1)!]!
}

const open = computed(() => props.decks.filter((d) => d.due > 0))
const tomorrow = computed(() => forecast(props.cards, new Date(), 2)[1] ?? 0)

const week = computed(() => {
  const days = perDay(props.log)
  const today = new Date()
  const monday = addDays(today, -((today.getDay() + 6) % 7))
  return Array.from({ length: 7 }, (_, i) => {
    const d = addDays(monday, i)
    return {
      n: days.get(ymd(d)) ?? 0,
      today: ymd(d) === ymd(today),
      label: d.toLocaleDateString(undefined, { weekday: 'narrow' }),
    }
  })
})

const weekTotal = computed(() => week.value.reduce((a, d) => a + d.n, 0))
const weekMax = computed(() => Math.max(1, ...week.value.map((d) => d.n)))
</script>

<template>
  <div class="cl-done">
    <div class="cl-done-main">
      <div class="v-head-row cl-done-head">
        <div>
          <div class="v-sub cl-done-sub">
            {{ $tn(results.size, '{n} Karte', '{n} Karten') }} ·
            {{ $t('{n} min', { n: minutes }) }} · {{ $t('morgen {n} fällig', { n: tomorrow }) }}
          </div>
          <h1 class="v-title cl-celebrate-title">
            <span>🎉</span>
            {{ $t('Runde geschafft') }}
          </h1>
        </div>
        <div class="cl-done-act">
          <button type="button" class="btn btn-secondary" @click="emit('exit')">
            {{ $t('Zu den Fächern') }}
          </button>
          <button
            v-if="hard.length"
            type="button"
            class="btn btn-primary"
            @click="emit('again', hard)"
          >
            {{ $t('Schwere nochmal üben') }}<span class="cl-n">{{ hard.length }}</span>
          </button>
        </div>
      </div>

      <section class="card cl-grades-sum" :aria-label="$t('Bewertungen')">
        <div class="cl-kpis">
          <div v-for="(g, i) in grades" :key="i" class="cl-kpi">
            <b class="mono" :class="`c-g${i}`">{{ gradeCounts[i] }}</b>
            <span>{{ g }}</span>
          </div>
        </div>
        <div
          class="cl-dist"
          role="img"
          :aria-label="grades.map((g, i) => `${gradeCounts[i]} ${g}`).join(', ')"
        >
          <i v-for="(n, i) in gradeCounts" :key="i" :class="`g${i}`" :style="{ flex: n }"></i>
        </div>
      </section>

      <section class="card cl-list" :aria-label="$t('Karten dieser Runde')">
        <div class="cl-row cl-row-h">
          <span>{{ $t('Karte') }}</span>
          <span>{{ $t('Antwort') }}</span>
          <span>{{ $t('Wieder dran') }}</span>
        </div>
        <div v-for="r in roundList" :key="r.card.key" class="cl-row">
          <span class="cl-cell">{{ r.card.question }}</span>
          <span class="cl-pill" :class="{ 'is-again': r.grades.includes(0) }">{{
            answerText(r.grades)
          }}</span>
          <span class="mono cl-when">{{
            r.next === null ? $t('Übung') : r.next === 0 ? $t('heute') : intervalText(r.next)
          }}</span>
        </div>
        <p v-if="!roundList.length" class="v-sub cl-row">{{ $t('Nichts bewertet.') }}</p>
      </section>
    </div>

    <aside class="cl-done-side">
      <section class="card cl-side" :aria-label="$t('Heute noch offen')">
        <div class="cl-side-h">
          <span>{{ $t('Heute noch offen') }}</span
          ><span class="mono">{{ open.reduce((n, d) => n + d.due, 0) }}</span>
        </div>
        <div v-for="d in open" :key="d.name" class="cl-side-r">
          <i class="cl-dot" :style="{ background: colorOf(d.name) }"></i>
          <span>{{ d.name }}</span>
          <span class="mono c-due">{{ $t('{n} fällig', { n: d.due }) }}</span>
        </div>
        <p v-if="!open.length" class="cl-side-empty">{{ $t('Alles erledigt für heute.') }}</p>
      </section>
      <section class="card cl-side cl-week" :aria-label="$t('Diese Woche gelernt')">
        <div class="cl-side-h">
          <span>{{ $t('Diese Woche gelernt') }}</span
          ><span class="mono">{{ $tn(weekTotal, '{n} Karte', '{n} Karten') }}</span>
        </div>
        <div
          class="cl-bars"
          role="img"
          :aria-label="week.map((d) => `${d.label} ${d.n}`).join(', ')"
        >
          <i
            v-for="(d, i) in week"
            :key="i"
            :class="{ on: d.today }"
            :style="{ height: Math.max(4, (d.n / weekMax) * 100) + '%' }"
          ></i>
        </div>
        <div class="cl-bars-x mono">
          <span v-for="(d, i) in week" :key="i" :class="{ on: d.today }">{{ d.label }}</span>
        </div>
      </section>
    </aside>
  </div>
</template>
