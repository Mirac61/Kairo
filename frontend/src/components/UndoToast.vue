<script setup lang="ts">
import { useUndo } from '@/composables/useUndo'

const { pending, undo, show } = useUndo()
</script>

<template>
  <!-- Der Container bleibt im DOM, damit Screenreader den neuen Toast ansagen. -->
  <div id="toasts" role="status">
    <div v-if="pending" class="toast-item">
      <span>{{ pending.text }}</span>
      <button v-if="pending.show" type="button" class="btn btn-ghost" @click="show">
        {{ $t('Anzeigen') }}
      </button>
      <button type="button" class="btn btn-ghost" @click="undo">{{ $t('Rückgängig') }}</button>
    </div>
  </div>
</template>
