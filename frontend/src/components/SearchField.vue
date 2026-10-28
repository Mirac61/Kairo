<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

defineProps<{ label: string }>()
const model = defineModel<string>({ default: '' })
const el = ref<HTMLInputElement>()
const btn = ref<HTMLButtonElement>()
// Eingeklappt als Lupe, damit die Suche nicht neben der Schnelleingabe wie ein zweites Eingabefeld wirkt.
const open = ref(false)

async function show() {
  open.value = true
  await nextTick()
  el.value?.focus()
}
async function hide() {
  open.value = false
  await nextTick()
  btn.value?.focus()
}

// „/“ springt ins Suchfeld. Nicht beim Tippen, nicht in Dialogen.
function onKey(e: KeyboardEvent) {
  if (
    e.key !== '/' ||
    e.ctrlKey ||
    e.metaKey ||
    e.altKey ||
    (e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')
  )
    return
  e.preventDefault()
  void show()
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <button
    v-if="!open && !model"
    ref="btn"
    type="button"
    class="icon-btn search-btn"
    :aria-label="label"
    :title="$t('Suchen (/)')"
    aria-keyshortcuts="/"
    @click="show"
  >
    <svg class="ic"><use href="#i-search" /></svg>
  </button>
  <!-- Esc leert die Suche, ein zweites Esc klappt sie wieder ein. -->
  <input
    v-else
    ref="el"
    v-model="model"
    type="search"
    class="input"
    :placeholder="$t('Suchen …')"
    :title="$t('Suchen (/)')"
    :aria-label="label"
    aria-keyshortcuts="/"
    @blur="open = false"
    @keydown.esc="model ? (model = '') : hide()"
  />
</template>
