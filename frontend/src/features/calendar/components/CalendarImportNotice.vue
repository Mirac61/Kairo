<script setup lang="ts">
import Message from 'primevue/message'

defineProps<{
  notice: { text: string; notes: string[] }
}>()

defineEmits<{
  (e: 'close'): void
}>()
</script>

<template>
  <Message severity="success" closable @close="$emit('close')">
    {{ notice.text }}
    <ul v-if="notice.notes.length" class="notes">
      <li v-for="n in notice.notes" :key="n">{{ n }}</li>
    </ul>
  </Message>
</template>

<style scoped>
.notes {
  margin: 6px 0 0;
  padding-left: 18px;
  max-height: 120px;
  overflow: auto;
  font-size: 12px;
}
</style>
