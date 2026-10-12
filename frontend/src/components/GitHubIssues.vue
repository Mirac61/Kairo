<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage, type Project } from '@/api/client'

// Offene GitHub-Issues; „Als Aufgabe“ legt eine Task mit der Issue-URL als Ressource an.
// Ohne projectId: alle Issues, an denen der Nutzer beteiligt ist (nach Repo gruppiert, Projekt wählbar).
// Mit projectId: alle Issues des Repos hinter dem Projektordner; ohne GitHub-Repo bleibt die Komponente leer.
interface Issue { title: string; url: string; repo: string; number: number; labels: string[]; updated_at: string; task_id: string | null }

const props = defineProps<{ projects?: Project[]; projectId?: string }>()
const emit = defineEmits<{ open: [taskId: string]; changed: [] }>()

const issues = ref<Issue[] | null>(null)
const repo = ref('')
const error = ref('')
const target = ref(props.projectId ?? '')
const busy = ref('')

const byRepo = computed(() => {
  const m = new Map<string, Issue[]>()
  for (const i of issues.value ?? []) m.set(i.repo, [...(m.get(i.repo) ?? []), i])
  return [...m]
})

async function load() {
  try {
    if (props.projectId) {
      const r = await api<{ repo: string; issues: Issue[] }>(`/projects/${props.projectId}/github/issues`)
      repo.value = r.repo
      issues.value = r.issues
    } else {
      issues.value = await api<Issue[]>('/github/issues')
    }
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function add(i: Issue) {
  busy.value = i.url
  try {
    const task = await api<{ id: string }>('/github/issues/import', {
      method: 'POST',
      body: JSON.stringify({ url: i.url, title: i.title, repo: i.repo, number: i.number, ...(target.value ? { project_id: target.value } : {}) }),
    })
    i.task_id = task.id
    error.value = ''
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = ''
  }
}

onMounted(load)
</script>

<template>
  <div v-if="!projectId || repo || error" class="gh">
    <div v-if="!projectId" class="gh-bar">
      <span class="v-sub">{{ $t('Offen, aus allen Repos, an denen du beteiligt bist.') }}</span>
      <select v-model="target" class="input" :aria-label="$t('Projekt für neue Aufgaben')">
        <option value="">{{ $t('Ohne Projekt') }}</option>
        <option v-for="p in (projects ?? []).filter((p) => p.status !== 'ARCHIVED')" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
    </div>
    <span v-else class="lbl">{{ $t('GitHub-Issues') }} · {{ repo }}<span v-if="issues?.length" class="count"> · {{ issues.length }}</span></span>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div v-if="!issues && !error" class="v-sub">{{ $t('Lädt …') }}</div>
    <div v-else-if="issues && !issues.length" class="v-sub">{{ $t('Keine offenen Issues.') }}</div>
    <div class="gh-list">
      <section v-for="[r, list] in byRepo" :key="r">
        <h4 v-if="!projectId" class="lbl">{{ r }}</h4>
        <div v-for="i in list" :key="i.url" class="gh-row">
          <a class="gh-title" :href="i.url" target="_blank" rel="noopener"><span class="mono">#{{ i.number }}</span> {{ i.title }}</a>
          <span v-for="l in i.labels" :key="l" class="chip">{{ l }}</span>
          <button v-if="i.task_id" class="btn btn-ghost" type="button" @click="emit('open', i.task_id)">{{ $t('Zur Aufgabe') }}</button>
          <button v-else class="btn" type="button" :disabled="busy === i.url" @click="add(i)"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Als Aufgabe') }}</button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.gh { display: flex; flex-direction: column; gap: 12px; }
.gh-bar { display: flex; align-items: center; gap: 12px; }
.gh-bar .v-sub { flex: 1; }
.gh-bar select { width: auto; }
.gh-list { display: flex; flex-direction: column; gap: 16px; max-height: 60vh; overflow-y: auto; }
.gh-list h4 { margin: 0 0 6px; }
.gh-row { display: flex; align-items: center; gap: 8px; min-height: 36px; border-top: 1px solid var(--br-subtle); }
.gh-title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--tx-primary); text-decoration: none; }
.gh-title:hover { text-decoration: underline; }
.gh-title .mono { color: var(--tx-muted); }
.gh-row .btn { flex: none; }
</style>
