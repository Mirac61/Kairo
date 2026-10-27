<script setup lang="ts">
import { computed, ref } from 'vue'
import { createResource, deleteResource, errorMessage, listNotes, type NoteNode, type Resource } from '@/api/client'
import { t } from '@/lib/i18n'

const props = defineProps<{ resources: Resource[]; taskId?: string; projectId?: string }>()
const emit = defineEmits<{ changed: [] }>()

const target = ref('')
const label = ref('')
const note = ref('')
const notes = ref<string[] | null>(null) // Dateien im Notizordner, geladen beim ersten Fokus
const error = ref('')
const noteSet = computed(() => new Set(notes.value ?? []))

async function change(fn: () => Promise<unknown>) {
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
    return false
  }
  error.value = ''
  emit('changed')
  return true
}

// Der Typ (URL, Datei, Ordner) wird vom Backend aus dem Ziel abgeleitet.
async function add() {
  const t = target.value.trim()
  if (!t) return
  const ok = await change(() => createResource({ task_id: props.taskId, project_id: props.projectId, target: t, label: label.value.trim() }))
  if (ok) target.value = label.value = ''
}

async function loadNotes() {
  if (notes.value) return
  const out: string[] = []
  const walk = (ns: NoteNode[]) => ns.forEach((n) => (n.dir ? walk(n.children ?? []) : out.push(n.path)))
  try {
    walk(await listNotes())
  } catch (e) {
    error.value = errorMessage(e)
  }
  notes.value = out
}

async function addNote() {
  const p = note.value.trim()
  if (!p) return
  if (notes.value && !noteSet.value.has(p)) return void (error.value = t('Diese Datei gibt es in den Notizen nicht.'))
  const name = p.slice(p.lastIndexOf('/') + 1)
  if (await change(() => createResource({ task_id: props.taskId, project_id: props.projectId, type: 'NOTE', target: p, label: name }))) note.value = ''
}
</script>

<template>
  <div class="res-edit">
    <span class="lbl">{{ $t('Ressourcen') }}</span>
    <div v-for="r in resources" :key="r.id" class="res-line">
      <RouterLink v-if="r.type === 'NOTE'" class="res-t res-note" :to="{ path: '/notes', query: { open: r.target } }" :title="r.target">{{ r.label || r.target }} <span class="muted">· {{ $t('Notiz') }}</span></RouterLink>
      <span v-else class="res-t">{{ r.label || r.target }} <span class="muted">· {{ r.type }}</span></span>
      <button class="icon-btn" type="button" :aria-label="`Ressource entfernen: ${r.label || r.target}`" @click="change(() => deleteResource(r.id))"><svg class="ic"><use href="#i-trash" /></svg></button>
    </div>
    <details class="res-more">
      <summary>{{ $t('Ressource hinzufügen') }}</summary>
    <form class="res-note-add" @submit.prevent="addNote">
      <input v-model="note" class="input" list="res-notes" :placeholder="$t('Notiz oder PDF verknüpfen …')" :aria-label="$t('Notiz oder PDF verknüpfen …')" @focus="loadNotes" />
      <datalist id="res-notes"><option v-for="p in notes ?? []" :key="p" :value="p" /></datalist>
      <button class="btn btn-secondary" type="submit">{{ $t('Verknüpfen') }}</button>
    </form>
    <form class="res-add" @submit.prevent="add">
      <input v-model="target" class="input" :placeholder="$t('URL oder Pfad (~ erlaubt)')" :aria-label="$t('Ziel')" />
      <input v-model="label" class="input" :placeholder="$t('Name (optional)')" :aria-label="$t('Name')" />
      <button class="btn btn-secondary" type="submit">{{ $t('Hinzufügen') }}</button>
    </form>
    </details>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
  </div>
</template>

<style scoped>
.res-edit { display: grid; gap: 8px; }
.res-line { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
.res-t { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.res-note { color: var(--tx-primary); text-decoration: underline; text-decoration-color: var(--br-default); text-underline-offset: 3px; }
.res-more { display: grid; gap: 8px; }
.res-more > summary { list-style: none; cursor: pointer; color: var(--tx-muted); font: 400 13px/1.2 var(--font-ui); }
.res-more > summary::-webkit-details-marker { display: none; }
.res-more > summary:hover { color: var(--tx-primary); }
.res-more[open] > summary { margin-bottom: 8px; }
.res-note-add { display: grid; grid-template-columns: 1fr auto; gap: 8px; }
.res-add { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.res-add .btn { grid-column: 1 / -1; justify-self: start; }
</style>
