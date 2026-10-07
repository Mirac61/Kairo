<script setup lang="ts">
import { taskAction, type Task } from '@/api/client'
import { useUndo } from '@/composables/useUndo'

const props = defineProps<{ task: Task; running: boolean }>()
const emit = defineEmits<{ run: [fn: () => Promise<unknown>] }>()
const { setDone } = useUndo()

// Die Extension startet die Task und öffnet Workspace und Ressourcen.
const inCode = () => { window.location.href = `vscodium://kairo-local.kairo/start?task=${props.task.id}` }
</script>

<template>
  <div v-if="task.status !== 'COMPLETED' && task.status !== 'CANCELLED'" class="row nowrap">
    <button v-if="running" type="button" class="btn btn-primary" :aria-label="`Pause: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'pause'))">Pause</button>
    <button v-else type="button" class="btn btn-primary" :aria-label="`Start: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'start'))">Start</button>
    <button type="button" class="btn btn-secondary" :aria-label="`Fertig: ${task.title}`" @click="setDone(task, 'COMPLETED', (fn) => emit('run', fn))">Fertig</button>
    <button type="button" class="btn btn-ghost" :aria-label="`In VSCodium starten: ${task.title}`" @click="inCode">In VSCodium</button>
  </div>
</template>
