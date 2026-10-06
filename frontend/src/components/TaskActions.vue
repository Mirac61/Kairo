<script setup lang="ts">
import Button from 'primevue/button'
import { taskAction, type Task } from '@/api/client'

defineProps<{ task: Task; running: boolean }>()
defineEmits<{ run: [fn: () => Promise<unknown>] }>()
</script>

<template>
  <div v-if="task.status !== 'COMPLETED' && task.status !== 'CANCELLED'" class="row nowrap">
    <Button v-if="running" label="Pause" size="small" severity="secondary" @click="$emit('run', () => taskAction(task.id, 'pause'))" />
    <Button v-else label="Start" size="small" @click="$emit('run', () => taskAction(task.id, 'start'))" />
    <Button label="Fertig" size="small" severity="secondary" @click="$emit('run', () => taskAction(task.id, 'complete'))" />
  </div>
</template>
