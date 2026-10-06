<script setup lang="ts">
import { taskAction, type Task } from '@/api/client'

defineProps<{ task: Task; running: boolean }>()
defineEmits<{ run: [fn: () => Promise<unknown>] }>()
</script>

<template>
  <div v-if="task.status !== 'COMPLETED' && task.status !== 'CANCELLED'" class="row nowrap">
    <button v-if="running" type="button" class="btn btn-secondary" @click="$emit('run', () => taskAction(task.id, 'pause'))">Pause</button>
    <button v-else type="button" class="btn btn-secondary" @click="$emit('run', () => taskAction(task.id, 'start'))">Start</button>
    <button type="button" class="btn btn-ghost" @click="$emit('run', () => taskAction(task.id, 'complete'))">Fertig</button>
  </div>
</template>
