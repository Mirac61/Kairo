<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getJournalSummary, listNotes, readNote } from '@/api/client'
import { useHome } from '@/composables/useHome'
import { shiftDate } from '@/lib/dates'
import { locale } from '@/lib/i18n'
import { journalPath, parseEntry, QUESTIONS } from '@/lib/journal'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const { today } = useHome()
const dates = ref(new Set<string>())
const yesterday = ref<{ date: string; intention: string } | null>(null)
const streak = ref(0)

// Lädt, sobald der Tag bekannt ist; Fehler lassen nur Heatmap bzw. Vorhaben weg.
watch(() => today.value?.date, async (day) => {
  if (!day) return
  const prev = shiftDate(day, -1)
  const [notes, intention, summary] = await Promise.all([
    listNotes().catch(() => []),
    readNote(journalPath(prev)).then((n) => parseEntry(n.content).answers[2]?.trim() ?? '').catch(() => ''),
    getJournalSummary().catch(() => null),
  ])
  streak.value = summary?.streak ?? 0
  const dir = notes.find((n) => n.dir && n.path === 'Journal')
  dates.value = new Set((dir?.children ?? []).map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1]).filter((d): d is string => !!d))
  yesterday.value = intention ? { date: prev, intention } : null
}, { immediate: true })

// Die letzten fünf Wochen, Montag bis Sonntag je Zeile; Tage nach heute bleiben leer.
const cells = computed(() => {
  const d = today.value?.date
  if (!d) return []
  const dow = (new Date(`${d}T12:00:00Z`).getUTCDay() + 6) % 7
  return Array.from({ length: 35 }, (_, i) => shiftDate(d, i - 28 - dow))
})
const weekday = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'narrow', timeZone: 'UTC' }))
const written = computed(() => !!today.value && dates.value.has(today.value.date))
</script>

<template>
  <section class="wg wg-journal" :class="{ sm: size === 'S' }" :aria-label="$t('Journal')">
    <div class="wg-h"><router-link to="/journal"><svg class="ic"><use href="#i-journal" /></svg>{{ $t('Journal') }}</router-link><span>{{ written ? $t('Heute geschrieben') : $t('Heute noch offen') }}</span></div>
    <div class="wj-body">
      <div class="wj-text">
        <router-link v-if="yesterday" class="st-journal" :to="{ name: 'journal', query: { date: yesterday.date } }">
          <span class="wj-lbl">{{ $t('Gestern hast du dir vorgenommen:') }}</span>
          <b>{{ yesterday.intention }}</b>
        </router-link>
        <template v-else>
          <div class="wj-t">{{ written ? $t('Heute schon geschrieben.') : $t('3 Dinge heute?') }}</div>
          <ol class="wj-q"><li v-for="q in QUESTIONS" :key="q">{{ $t(q) }}</li></ol>
        </template>
        <router-link to="/journal" class="btn btn-secondary wj-go">{{ written ? $t('Eintrag ansehen') : $t('Heute schreiben') }}</router-link>
      </div>
      <div class="wj-side">
        <div class="wj-heat" aria-hidden="true">
          <b v-for="d in cells.slice(0, 7)" :key="d">{{ weekday.format(new Date(`${d}T12:00:00Z`)) }}</b>
          <i v-for="d in cells" :key="d" :class="{ has: dates.has(d), today: d === today?.date, future: !!today && d > today.date }"></i>
        </div>
        <div class="wj-streak">{{ $tn(streak, '{n} Tag in Folge', '{n} Tage in Folge') }}</div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.wj-body { flex: 1; min-height: 0; display: flex; gap: 28px; }
.wj-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
.wj-t { font: 400 28px/1.15 var(--font-serif); color: var(--tx-primary); }
.wj-q { margin: 0; padding-left: 18px; display: grid; gap: 2px; font: 400 14px/1.5 var(--font-ui); color: var(--tx-secondary); }
.wj-q li::marker { color: var(--tx-muted); font-size: 12px; }
.st-journal { display: grid; gap: 4px; color: inherit; text-decoration: none; }
.st-journal b { font: 400 24px/1.2 var(--font-serif); color: var(--tx-primary); display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; white-space: pre-line; }
.st-journal:hover b { text-decoration: underline; }
.wj-lbl { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }
.wj-go { align-self: flex-start; margin-top: auto; height: 32px; text-decoration: none; }
.wj-side { flex: none; display: flex; flex-direction: column; justify-content: space-between; gap: 10px; }
.wj-heat { display: grid; grid-template-columns: repeat(7, 14px); gap: 4px; }
.wj-heat b { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); text-align: center; margin-bottom: 2px; }
.wj-heat i { height: 14px; border-radius: 3px; background: var(--bg-3); }
.wj-heat i.has { background: var(--a-blue-hi); }
.wj-heat i.today { box-shadow: inset 0 0 0 1.5px var(--a-blue-hi); }
.wj-heat i.future { background: none; box-shadow: inset 0 0 0 1px var(--br-subtle); }
.wj-streak { font: 400 20px/1.2 var(--font-serif); color: var(--tx-primary); }
/* Niedrige Zeilen (kleines Fenster, Handy): Fragen weg, Heatmap auf drei Wochen ohne Wochentage */
.wg-journal { container-type: size; }
@container (max-height: 220px) {
  .wj-q, .wj-heat b, .wj-heat i:nth-of-type(-n+14) { display: none; }
  .st-journal b { font-size: 20px; -webkit-line-clamp: 2; }
  .wj-streak { font-size: 15px; }
}
/* S: nur Vorhaben bzw. Frage und Serie; das ganze Widget führt ohnehin zum Journal */
.sm .wj-body { flex-direction: column; gap: 6px; }
.sm .wj-q, .sm .wj-heat, .sm .wj-go, .sm .wg-h > span { display: none; }
.sm .wj-t { font-size: 20px; }
.sm .st-journal b { font-size: 17px; -webkit-line-clamp: 2; }
.sm .wj-streak { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
</style>
