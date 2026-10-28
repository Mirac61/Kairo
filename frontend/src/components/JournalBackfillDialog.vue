<script setup lang="ts">
import { ref, watch } from 'vue'
import { vDialog } from '@/lib/dialog'

const props = defineProps<{
  open: boolean
  initialDate: string
  maxDate: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', date: string): void
}>()

const date = ref(props.initialDate)

watch(
  () => props.initialDate,
  (val) => {
    date.value = val
  }
)

function submit() {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date.value) || date.value > props.maxDate) return
  emit('submit', date.value)
}
</script>

<template>
  <div v-if="open" v-dialog="() => emit('close')" class="overlay open" @mousedown.self="emit('close')">
    <form class="dialog jr-backfill" :aria-label="$t('Eintrag nachtragen')" @submit.prevent="submit">
      <div class="dlg-head">
        <h3>{{ $t('Eintrag nachtragen') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <label class="field">
          <span>{{ $t('Datum') }}</span>
          <input v-model="date" class="input" type="date" :max="maxDate" required autofocus />
        </label>
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" @click="emit('close')">{{ $t('Abbrechen') }}</button>
        <button class="btn btn-primary" type="submit">{{ $t('Öffnen') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.jr-backfill {
  width: 320px;
  max-width: calc(100vw - 32px);
}
</style>
