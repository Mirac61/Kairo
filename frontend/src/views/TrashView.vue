<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  deleteEvent, deleteHabit, deleteTask, errorMessage, getTrash, restoreEvent, restoreHabit, restoreTask,
  type Trash,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import DeleteButton from '@/components/DeleteButton.vue'

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

const sections = computed(() => [
  { title: 'Aufgaben', rows: trash.value.tasks.map((t) => ({ id: t.id, title: t.title, at: t.deleted_at, restore: () => restoreTask(t.id), purge: () => deleteTask(t.id, true) })) },
  { title: 'Termine', rows: trash.value.events.map((e) => ({ id: e.id, title: e.title, at: e.deleted_at, restore: () => restoreEvent(e.id), purge: () => deleteEvent(e.id, true) })) },
  { title: 'Gewohnheiten', rows: trash.value.habits.map((h) => ({ id: h.id, title: h.name, at: h.deleted_at, restore: () => restoreHabit(h.id), purge: () => deleteHabit(h.id, true) })) },
].filter((s) => s.rows.length))

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head"><h1 class="v-title">Papierkorb</h1></div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div v-if="!sections.length" class="v-sub">Der Papierkorb ist leer.</div>
    <div v-for="s in sections" :key="s.title" class="tgroup">
      <span class="lbl">{{ s.title }}<span class="count">{{ s.rows.length }}</span></span>
      <div class="card tasklist">
        <div v-for="r in s.rows" :key="r.id" class="task-row">
          <span class="t">{{ r.title }}</span>
          <span class="due">gelöscht {{ when(r.at) }}</span>
          <button type="button" class="btn btn-secondary" :aria-label="`Wiederherstellen: ${r.title}`" @click="run(r.restore)">Wiederherstellen</button>
          <DeleteButton label="Endgültig löschen" :text="`„${r.title}“ endgültig löschen? Das lässt sich nicht rückgängig machen.`" @confirm="run(r.purge)" />
        </div>
      </div>
    </div>
  </div>
</template>
