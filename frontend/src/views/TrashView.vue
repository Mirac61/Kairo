<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  deleteEvent, deleteHabit, deleteTask, getTrash, listProjects, restoreEvent, restoreHabit, restoreTask,
  type Project, type Trash,
} from '@/api/client'
import { projectColor } from '@/lib/projectColor'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import DeleteButton from '@/components/DeleteButton.vue'
import SearchField from '@/components/SearchField.vue'

const trash = ref<Trash>({ tasks: [], events: [], habits: [] })
const projects = ref<Project[]>([])

const { error, load, run } = useLoader(async () => {
  ;[trash.value, projects.value] = await Promise.all([getTrash(), listProjects()])
})

const when = (iso: string) =>
  new Intl.DateTimeFormat('de-DE', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(iso))

// Eine Liste, neueste zuerst; der Typ steht als Icon und Wort in der Zeile.
const rows = computed(() => [
  ...trash.value.tasks.map((t) => ({ id: t.id, kind: 'Aufgabe', icon: 'tasks', title: t.title, project: t.project_id, at: t.deleted_at, restore: () => restoreTask(t.id), purge: () => deleteTask(t.id, true) })),
  ...trash.value.events.map((e) => ({ id: e.id, kind: 'Termin', icon: 'cal', title: e.title, project: null, at: e.deleted_at, restore: () => restoreEvent(e.id), purge: () => deleteEvent(e.id, true) })),
  ...trash.value.habits.map((h) => ({ id: h.id, kind: 'Gewohnheit', icon: 'habits', title: h.name, project: null, at: h.deleted_at, restore: () => restoreHabit(h.id), purge: () => deleteHabit(h.id, true) })),
].sort((x, y) => y.at.localeCompare(x.at)))
const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const KINDS = ['Aufgabe', 'Termin', 'Gewohnheit'] as const
const PLURAL = { Aufgabe: 'Aufgaben', Termin: 'Termine', Gewohnheit: 'Gewohnheiten' } as const
const kind = ref<'' | (typeof KINDS)[number]>('')
const count = (k: string) => rows.value.filter((r) => !k || r.kind === k).length
const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter((r) => (!kind.value || r.kind === kind.value) && (!q || `${r.title} ${r.kind}`.toLowerCase().includes(q)))
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
    <div class="filterbar">
      <button type="button" class="fchip" :aria-pressed="kind === ''" @click="kind = ''">Alle <span class="count">{{ count('') }}</span></button>
      <button v-for="k in KINDS" :key="k" type="button" class="fchip" :aria-pressed="kind === k" :disabled="!count(k)" @click="kind = k">{{ PLURAL[k] }} <span class="count">{{ count(k) }}</span></button>
      <SearchField v-model="search" label="Papierkorb durchsuchen" />
    </div>
    <div v-if="!shown.length" class="v-sub">Keine Treffer für „{{ search }}“.</div>
    <div v-else class="tasklist">
      <div v-for="r in shown" :key="r.kind + r.id" class="task-row">
        <svg class="ic tr-ic" aria-hidden="true"><use :href="`#i-${r.icon}`" /></svg>
        <span class="t tr-title">{{ r.title }}
          <span class="tr-from">{{ r.kind }}<template v-if="r.project && projectName.get(r.project)"> · <i class="pdot" :style="{ background: projectColor(r.project) }"></i>aus {{ projectName.get(r.project) }}</template></span>
        </span>
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
.tr-title { display: grid; gap: 2px; white-space: normal; }
.tr-from { display: flex; align-items: center; gap: 6px; font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
.tr-from .pdot { display: inline-block; width: 6px; height: 6px; border-radius: 50%; }
</style>
