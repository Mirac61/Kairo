<script setup lang="ts">
import { NButton, NPopconfirm } from 'naive-ui'

defineProps<{ text: string }>()
defineEmits<{ confirm: [] }>()
</script>

<template>
  <n-popconfirm positive-text="Löschen" negative-text="Abbrechen" @positive-click="$emit('confirm')">
    <template #trigger><n-button size="small" quaternary type="error">Löschen</n-button></template>
    {{ text }}
  </n-popconfirm>
</template>
