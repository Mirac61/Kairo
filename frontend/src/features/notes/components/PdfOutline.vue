<script setup lang="ts">
import type { OutlineItem } from '@/composables/usePdfRender'

// Verschachteltes Inhaltsverzeichnis einer PDF; ein Klick springt zur Seite des Eintrags.
defineProps<{ items: OutlineItem[] }>()
defineEmits<{ go: [number] }>()
</script>

<template>
  <ul class="po">
    <li v-for="(it, i) in items" :key="i">
      <button type="button" :disabled="it.page < 0" :title="it.title" @click="$emit('go', it.page)">
        {{ it.title || '–' }}
      </button>
      <PdfOutline v-if="it.items.length" :items="it.items" @go="$emit('go', $event)" />
    </li>
  </ul>
</template>

<style scoped>
.po {
  margin: 0;
  padding: 0;
  list-style: none;
}
.po .po {
  padding-left: 12px;
}
.po button {
  display: block;
  width: 100%;
  padding: 4px 8px;
  border-radius: var(--r-s);
  font: 400 12px/1.35 var(--font-ui);
  color: var(--tx-secondary);
  text-align: left;
  overflow-wrap: anywhere;
}
.po button:hover:not(:disabled) {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.po button:disabled {
  color: var(--tx-disabled);
  cursor: default;
}
</style>
