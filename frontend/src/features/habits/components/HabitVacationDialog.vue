<script setup lang="ts">
import { ref, watch } from 'vue'
import type { HabitPause } from '@/api/client'
import { formatPauseSpan } from '@/lib/habits'
import { addDays, ymd } from '@/lib/dates'

const props = defineProps<{
  open: boolean
  vacations: HabitPause[]
  dateShortFmt: Intl.DateTimeFormat
  todayStr: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', payload: { startDate: string; endDate: string }): void
  (e: 'deletePause', pauseId: string): void
}>()

const pauseFrom = ref(props.todayStr)
const pauseUntil = ref(ymd(addDays(new Date(), 7)))

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      pauseFrom.value = props.todayStr
      pauseUntil.value = ymd(addDays(new Date(), 7))
    }
  },
)

function submit() {
  if (pauseUntil.value < pauseFrom.value) return
  emit('save', {
    startDate: pauseFrom.value,
    endDate: pauseUntil.value,
  })
}
</script>

<template>
  <form v-if="open" class="hd-pause card" @submit.prevent="submit">
    <span class="hd-pause-t">{{ $t('Alle Gewohnheiten pausieren') }}</span>
    <label>
      {{ $t('von') }}
      <input v-model="pauseFrom" class="input" type="date" required />
    </label>
    <label>
      {{ $t('bis') }}
      <input v-model="pauseUntil" class="input" type="date" :min="pauseFrom" required />
    </label>
    <button class="btn btn-primary" type="submit">{{ $t('Pausieren') }}</button>
    <button class="btn btn-ghost" type="button" @click="emit('close')">
      {{ $t('Abbrechen') }}
    </button>
    <ul v-if="vacations.length" class="hd-chips">
      <li v-for="p in vacations" :key="p.id">
        {{ $t('Urlaub') }} · {{ formatPauseSpan(p, dateShortFmt) }}
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Pause entfernen')"
          @click="emit('deletePause', p.id)"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </li>
    </ul>
  </form>
</template>

<style scoped>
.hd-pause {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.hd-pause label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font: 400 12.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-pause .input {
  width: auto;
  height: 30px;
}
.hd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: -8px 0 0;
  padding: 0;
  list-style: none;
}
.hd-chips li {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 4px 2px 10px;
  border-radius: 99px;
  font: 500 12px/1.6 var(--font-ui);
  color: var(--tx-secondary);
  background: var(--bg-2);
}
.hd-chips .icon-btn {
  width: 20px;
  height: 20px;
}
.hd-chips .icon-btn .ic {
  width: 12px;
  height: 12px;
}
.hd-pause.card {
  margin: -6px 0 14px;
  padding: 12px 14px;
}
.hd-pause-t {
  font: 500 13px/1.3 var(--font-ui);
  color: var(--tx-primary);
  margin-right: 4px;
}
</style>
