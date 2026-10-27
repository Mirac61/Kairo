<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '@/api/client'
import type { Size } from '@/lib/homeLayout'

// Offene Issues, an denen der Nutzer beteiligt ist (wie GitHubIssues.vue ohne Projekt); Übernehmen geht in Aufgaben.
interface Issue { title: string; url: string; repo: string; number: number }

const props = defineProps<{ size: Size }>()
const issues = ref<Issue[] | null>(null)
const error = ref('')
onMounted(() => api<Issue[]>('/github/issues').then((x) => (issues.value = x), (e) => (error.value = errorMessage(e))))
const shown = computed(() => (issues.value ?? []).slice(0, { S: 1, M: 1, L: 6 }[props.size]))
</script>

<template>
  <section class="wg wg-github" :aria-label="$t('GitHub')">
    <div class="wg-h"><router-link to="/tasks"><svg class="ic"><use href="#i-code" /></svg>{{ $t('GitHub') }}</router-link></div>
    <div v-if="issues?.length" class="wg-big">{{ issues.length }}<small>{{ $t('offen') }}</small></div>
    <div v-if="error" class="wg-empty">{{ error }}</div>
    <a v-for="i in shown" :key="i.url" :href="i.url" target="_blank" rel="noopener" class="wi-row">
      <span class="wi-t">{{ i.title }}</span>
      <span class="wi-s">{{ i.repo }} · #{{ i.number }}</span>
    </a>
    <div v-if="issues && !issues.length" class="wg-empty">{{ $t('Keine offenen Issues.') }}</div>
  </section>
</template>

<style scoped>
.wi-row:first-of-type { margin-top: auto; }
.wi-row { display: grid; gap: 2px; min-width: 0; color: inherit; text-decoration: none; }
.wi-row:hover .wi-t { text-decoration: underline; }
.wi-t { font: 400 13.5px/1.3 var(--font-ui); color: var(--tx-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wi-s { font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
</style>
