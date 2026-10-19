<script setup lang="ts">
import { ref, watch } from 'vue'
import { listBacklinks, type Backlink } from '@/api/client'

// Rückverweise: Notizen, die per [[…]] auf diese verweisen. rev (mtime der Notiz) lädt nach dem Speichern neu;
// ein Fehler zeigt einfach nichts, die Notiz selbst soll deshalb nicht gestört werden. Ohne Rückverweise fehlt der Abschnitt.
const props = defineProps<{ path: string; rev: string }>()
const emit = defineEmits<{ open: [string] }>()
const links = ref<Backlink[]>([])
watch(() => [props.path, props.rev], async () => {
  const p = props.path
  const r = await listBacklinks(p).catch(() => [])
  if (p === props.path) links.value = r
}, { immediate: true })
</script>

<template>
  <details v-if="links.length" class="backlinks">
    <summary>{{ $t('Rückverweise') }} ({{ links.length }})</summary>
    <ul>
      <li v-for="l in links" :key="l.path">
        <button type="button" class="btn btn-ghost" :data-tip="l.path" @click="emit('open', l.path)">{{ l.title }}</button>
        <span>{{ l.snippet }}</span>
      </li>
    </ul>
  </details>
</template>

<style scoped>
.backlinks { border-top: 1px solid var(--br-subtle); padding: 6px 16px 6px 24px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); max-height: 30%; overflow-y: auto; }
.backlinks summary { cursor: pointer; }
.backlinks ul { list-style: none; margin: 4px 0 0; padding: 0; }
.backlinks li { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
.backlinks .btn { height: 28px; flex: none; }
.backlinks span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
