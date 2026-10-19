<script setup lang="ts">
import type { Project } from '@/api/client'

defineProps<{ count: number; projects: Project[] }>()
const emit = defineEmits<{ tomorrow: []; project: [id: string]; trash: []; clear: [] }>()
const pickProject = (e: Event) => {
  const el = e.target as HTMLSelectElement
  emit('project', el.value)
  el.selectedIndex = 0
}
</script>

<template>
  <div class="selbar" role="toolbar" :aria-label="$t('Auswahl')">
    <span class="selbar-n">{{ $t('{n} gewählt', { n: count }) }}</span>
    <button type="button" class="btn btn-secondary" :disabled="!count" @click="emit('tomorrow')">{{ $t('Auf morgen') }}</button>
    <select class="input" :disabled="!count" :aria-label="$t('Projekt…')" @change="pickProject">
      <option value="" disabled selected>{{ $t('Projekt…') }}</option>
      <option value="">{{ $t('Kein Projekt') }}</option>
      <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
    </select>
    <button type="button" class="btn btn-secondary" :disabled="!count" @click="emit('trash')">{{ $t('Löschen') }}</button>
    <button type="button" class="icon-btn" :aria-label="$t('Auswahl aufheben')" @click="emit('clear')"><svg class="ic"><use href="#i-x" /></svg></button>
  </div>
</template>
