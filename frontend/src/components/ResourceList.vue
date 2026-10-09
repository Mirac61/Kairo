<script setup lang="ts">
import { ref } from 'vue'
import { createResource, deleteResource, errorMessage, type Resource } from '@/api/client'

const props = defineProps<{ resources: Resource[]; taskId?: string; projectId?: string }>()
const emit = defineEmits<{ changed: [] }>()

const target = ref('')
const label = ref('')
const error = ref('')

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
</script>

<template>
  <div class="res-edit">
    <span class="lbl">{{ $t('Ressourcen') }}</span>
    <div v-for="r in resources" :key="r.id" class="res-line">
      <span class="res-t">{{ r.label || r.target }} <span class="muted">· {{ r.type }}</span></span>
      <button class="icon-btn" type="button" :aria-label="`Ressource entfernen: ${r.label || r.target}`" @click="change(() => deleteResource(r.id))"><svg class="ic"><use href="#i-trash" /></svg></button>
    </div>
    <form class="res-add" @submit.prevent="add">
      <input v-model="target" class="input" :placeholder="$t('URL oder Pfad (~ erlaubt)')" :aria-label="$t('Ziel')" />
      <input v-model="label" class="input" :placeholder="$t('Name (optional)')" :aria-label="$t('Name')" />
      <button class="btn btn-secondary" type="submit">{{ $t('Hinzufügen') }}</button>
    </form>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
  </div>
</template>

<style scoped>
.res-edit { display: grid; gap: 8px; }
.res-line { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
.res-t { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.res-add { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.res-add .btn { grid-column: 1 / -1; justify-self: start; }
</style>
