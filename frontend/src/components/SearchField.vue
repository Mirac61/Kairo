<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

defineProps<{ label: string }>()
const model = defineModel<string>({ default: '' })
const el = ref<HTMLInputElement>()

// „/“ springt ins Suchfeld. Nicht beim Tippen, nicht in Dialogen.
function onKey(e: KeyboardEvent) {
  if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey || (e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')) return
  e.preventDefault()
  el.value?.focus()
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <!-- Esc leert die Suche, ein zweites Esc verlässt das Feld. -->
  <input
    ref="el" v-model="model" type="search" class="input" placeholder="Suchen …" title="Suchen (/)" :aria-label="label" aria-keyshortcuts="/"
    @keydown.esc="model ? (model = '') : el?.blur()"
  />
</template>
