<script setup lang="ts">
import { isOpen, taskAction, type Task } from '@/api/client'
import { useRouter } from 'vue-router'
import { useUndo } from '@/composables/useUndo'

// note: verknüpfte Notiz (Pfad im Notizordner); Start öffnet sie gleich mit.
const props = defineProps<{ task: Task; running: boolean; note?: string }>()
const emit = defineEmits<{ run: [fn: () => Promise<unknown>] }>()
const { setDone } = useUndo()
const router = useRouter()
const start = () => emit('run', async () => {
  await taskAction(props.task.id, 'start')
  if (props.note) void router.push({ path: '/notes', query: { open: props.note } })
})

// Die Extension startet die Task und öffnet Workspace und Ressourcen.
const inCode = () => { window.location.href = `vscodium://kairo-local.kairo/start?task=${props.task.id}` }
</script>

<template>
  <div v-if="isOpen(task)" class="row nowrap">
    <button v-if="running" type="button" class="btn btn-primary" :aria-label="`${$t('Pause')}: ${task.title}`" @click="$emit('run', () => taskAction(task.id, 'pause'))">{{ $t('Pause') }}</button>
    <button v-else type="button" class="btn btn-primary" :aria-label="`${$t('Start')}: ${task.title}`" @click="start">{{ $t('Start') }}</button>
    <button type="button" class="btn btn-secondary" :aria-label="`${$t('Fertig')}: ${task.title}`" @click="setDone(task, 'COMPLETED', (fn) => emit('run', fn))">{{ $t('Fertig') }}</button>
    <button type="button" class="btn btn-ghost act-code" :aria-label="`In VSCodium starten: ${task.title}`" @click="inCode">{{ $t('In VSCodium') }}</button>
  </div>
</template>
