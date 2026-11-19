<script setup lang="ts">
// Kopfzeile jeder Ansicht nach Notizen-Vorbild: 48 px, links Tabs oder Pfad, rechts Icons.
// Die H1 bleibt für Screenreader, sichtbar markiert die Seitenleiste die Ansicht.
defineProps<{ title: string }>()
</script>

<template>
  <header class="filterbar page-head">
    <h1 class="v-title sr-only">{{ title }}</h1>
    <slot />
    <div v-if="$slots.actions" class="ph-actions"><slot name="actions" /></div>
  </header>
</template>
