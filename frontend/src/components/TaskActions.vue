<script setup lang="ts">
import { taskAction, type Task } from '@/api/client'

const props = defineProps<{ task: Task; running: boolean }>()
defineEmits<{ run: [fn: () => Promise<unknown>] }>()

// Die Extension startet die Task und öffnet Workspace und Ressourcen.
const inCode = () => { window.location.href = `vscodium://kairo-local.kairo/start?task=${props.task.id}` }
</script>

<template>
  <div v-if="task.status !== 'COMPLETED' && task.status !== 'CANCELLED'" class="row nowrap">
    <button v-if="running" type="button" class="btn btn-secondary" :aria-label="`Pause: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'pause'))">Pause</button>
    <button v-else type="button" class="btn btn-secondary" :aria-label="`Start: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'start'))">Start</button>
    <button type="button" class="btn btn-ghost" :aria-label="`Fertig: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'complete'))">Fertig</button>
    <button type="button" class="btn btn-ghost" :aria-label="`In VSCodium starten: ${task.title}`" @click="inCode">In VSCodium</button>
  </div>
</template>
