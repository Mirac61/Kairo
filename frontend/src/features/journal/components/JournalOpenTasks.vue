<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { Task } from '@/api/client'

defineProps<{
  tasks: Task[]
  completingId?: string
}>()

const emit = defineEmits<{
  (e: 'complete', task: Task): void
}>()
</script>

<template>
  <section v-if="tasks.length" class="jr-open-box">
    <div class="jr-open-head">
      <h3 class="jr-open-title">{{ $t('Noch offen aus dem Journal') }}</h3>
      <RouterLink class="jr-open-all" :to="{ path: '/tasks', query: { filter: 'journal' } }">
        {{ $t('Alle Aufgaben') }}<svg class="ic"><use href="#i-right" /></svg>
      </RouterLink>
    </div>
    <ul class="jr-open-list">
      <li v-for="t in tasks.slice(0, 5)" :key="t.id" class="jr-open-item">
        <button
          type="button"
          class="cb jr-open-cb"
          role="checkbox"
          :aria-checked="false"
          :aria-label="$t('{title} erledigt', { title: t.title })"
          :disabled="completingId === t.id"
          @click="emit('complete', t)"
        ></button>
        <RouterLink class="jr-open-link" :to="`/tasks?task=${t.id}`">{{ t.title }}</RouterLink>
      </li>
    </ul>
    <RouterLink
      v-if="tasks.length > 5"
      class="jr-open-more"
      :to="{ path: '/tasks', query: { filter: 'journal' } }"
    >
      {{ $tn(tasks.length - 5, '+{n} weitere Aufgabe', '+{n} weitere Aufgaben') }}
    </RouterLink>
  </section>
</template>

<style scoped>
/* Offene Vorhaben im unteren Bereich der Journal-Timeline */
.jr-open-box {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--br-subtle);
}
.jr-open-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}
.jr-open-title {
  margin: 0;
  font: 500 12px/1.2 var(--font-ui);
  color: var(--tx-secondary);
}
.jr-open-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 7px;
}
.jr-open-item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: start;
  gap: 9px;
}
.jr-open-cb {
  margin-top: 2px;
  cursor: pointer;
}
.jr-open-link {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
}
.jr-open-link:hover {
  color: var(--tx-primary);
  text-decoration: underline;
}
.jr-open-more {
  display: block;
  margin-top: 10px;
  color: var(--tx-muted);
  font: 400 12px/1.3 var(--font-ui);
  text-decoration: none;
}
.jr-open-all {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--tx-muted);
  font: 500 11.5px/1 var(--font-ui);
  text-decoration: none;
  white-space: nowrap;
  transition: color var(--dur) ease-out;
}
.jr-open-all:hover {
  color: var(--tx-primary);
}
.jr-open-all .ic {
  width: 11px;
  height: 11px;
}
</style>
