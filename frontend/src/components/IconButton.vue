<script setup lang="ts">
// Icon-Knopf mit Pflicht-Label: dasselbe Wort als aria-label und Tooltip, Kürzel nur im Tooltip.
const props = defineProps<{ icon: string; label: string; shortcut?: string }>()
</script>

<template>
  <button
    type="button"
    class="icon-btn"
    :aria-label="props.label"
    :title="props.shortcut ? `${props.label} (${props.shortcut})` : props.label"
  >
    <svg class="ic" aria-hidden="true"><use :href="`#i-${props.icon}`" /></svg>
  </button>
</template>
