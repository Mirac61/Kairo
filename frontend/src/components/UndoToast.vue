<script setup lang="ts">
import { useUndo } from '@/composables/useUndo'

const { pending, undo } = useUndo()
</script>

<template>
  <!-- Der Container bleibt im DOM, damit Screenreader den neuen Toast ansagen. -->
  <div id="toasts" role="status">
    <div v-if="pending" class="toast-item">
      <span>{{ pending.text }}</span>
      <button type="button" class="btn btn-ghost" @click="undo">Rückgängig</button>
    </div>
  </div>
</template>
