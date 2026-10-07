<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  deleteEvent, deleteHabit, deleteTask, errorMessage, getTrash, restoreEvent, restoreHabit, restoreTask,
  type Trash,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'
import SearchField from '@/components/SearchField.vue'

const trash = ref<Trash>({ tasks: [], events: [], habits: [] })
const error = ref('')

async function load() {
  try {
    trash.value = await getTrash()
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Eine Meldung wie „Task hat Zeiteinträge, stattdessen abbrechen“ (409) bleibt stehen.
async function run(fn: () => Promise<unknown>) {
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
    return
  }
  await load()
}

const when = (iso: string) =>
  new Intl.DateTimeFormat('de-DE', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(iso))

// Eine Liste, neueste zuerst; der Typ steht als Icon und Wort in der Zeile.
const rows = computed(() => [
  ...trash.value.tasks.map((t) => ({ id: t.id, kind: 'Aufgabe', icon: 'tasks', title: t.title, at: t.deleted_at, restore: () => restoreTask(t.id), purge: () => deleteTask(t.id, true) })),
  ...trash.value.events.map((e) => ({ id: e.id, kind: 'Termin', icon: 'cal', title: e.title, at: e.deleted_at, restore: () => restoreEvent(e.id), purge: () => deleteEvent(e.id, true) })),
  ...trash.value.habits.map((h) => ({ id: h.id, kind: 'Gewohnheit', icon: 'habits', title: h.name, at: h.deleted_at, restore: () => restoreHabit(h.id), purge: () => deleteHabit(h.id, true) })),
].sort((x, y) => y.at.localeCompare(x.at)))
const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return q ? rows.value.filter((r) => `${r.title} ${r.kind}`.toLowerCase().includes(q)) : rows.value
})

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">Papierkorb</h1>
      <div class="v-sub">{{ rows.length }} {{ rows.length === 1 ? 'Eintrag' : 'Einträge' }}</div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div v-if="!rows.length" class="v-sub">Der Papierkorb ist leer.</div>
    <template v-else>
    <div class="filterbar"><SearchField v-model="search" label="Papierkorb durchsuchen" /></div>
    <div v-if="!shown.length" class="v-sub">Keine Treffer für „{{ search }}“.</div>
    <div v-else class="tasklist">
      <div v-for="r in shown" :key="r.kind + r.id" class="task-row">
        <svg class="ic tr-ic" aria-hidden="true"><use :href="`#i-${r.icon}`" /></svg>
        <span class="t">{{ r.title }}</span>
        <span class="tr-kind">{{ r.kind }}</span>
        <span class="due">{{ when(r.at) }}</span>
        <span class="row-act"><button type="button" class="btn btn-secondary" :aria-label="`Wiederherstellen: ${r.title}`" @click="run(r.restore)">Wiederherstellen</button>
        <DeleteButton ghost label="Endgültig löschen" :text="`„${r.title}“ endgültig löschen? Das lässt sich nicht rückgängig machen.`" @confirm="run(r.purge)" /></span>
      </div>
    </div>
    </template>
  </div>
</template>

<style scoped>
.tr-ic { flex: none; color: var(--tx-muted); }
.tr-kind { flex: none; min-width: 84px; font: 400 13px/1 var(--font-ui); color: var(--tx-secondary); }
</style>
