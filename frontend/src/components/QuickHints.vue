<script setup lang="ts">
import { computed } from 'vue'
import { dayLabel, hm } from '@/lib/dates'
import type { Quick } from '@/lib/quickAdd'

const props = defineProps<{ q: Quick; today: string }>()
const PRIO = { URGENT: 'Dringend', HIGH: 'Hoch', MEDIUM: 'Mittel', LOW: 'Niedrig' } as const

// Zeigt, was die Schnelleingabe erkannt hat, bevor Enter gedrückt wird.
const hints = computed(() => {
  const { q } = props
  return [
    q.date && `${dayLabel(q.date, props.today)}${q.time ? ` ${q.time}` : ''}`,
    q.minutes && hm(q.minutes),
    q.project && `#${q.project.name}`,
    q.priority && PRIO[q.priority],
  ].filter(Boolean) as string[]
})
</script>

<template>
  <div v-if="hints.length" class="qa-hints" aria-live="polite">
    <span v-for="h in hints" :key="h" class="qa-hint">{{ h }}</span>
  </div>
</template>
