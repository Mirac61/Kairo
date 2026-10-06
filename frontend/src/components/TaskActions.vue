<script setup lang="ts">
import { NButton, NSpace } from 'naive-ui'
import { taskAction, type Task } from '@/api/client'

defineProps<{ task: Task; running: boolean }>()
defineEmits<{ run: [fn: () => Promise<unknown>] }>()
</script>

<template>
  <n-space v-if="task.status !== 'COMPLETED' && task.status !== 'CANCELLED'" :size="6" :wrap="false">
    <n-button v-if="running" size="small" @click="$emit('run', () => taskAction(task.id, 'pause'))">Pause</n-button>
    <n-button v-else size="small" type="primary" @click="$emit('run', () => taskAction(task.id, 'start'))">Start</n-button>
    <n-button size="small" @click="$emit('run', () => taskAction(task.id, 'complete'))">Fertig</n-button>
  </n-space>
</template>
