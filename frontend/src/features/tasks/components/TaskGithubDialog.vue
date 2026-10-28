<script setup lang="ts">
import type { Project } from '@/api/client'
import GitHubIssues from '@/components/GitHubIssues.vue'

defineProps<{
  open: boolean
  projects: Project[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'openTask', id: string): void
  (e: 'changed'): void
}>()
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <div class="dialog gh-dlg" role="dialog" :aria-label="$t('GitHub-Issues')">
      <div class="dlg-head">
        <h3>{{ $t('GitHub-Issues') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <GitHubIssues
        class="dlg-body"
        :projects="projects"
        @open="emit('openTask', $event)"
        @changed="emit('changed')"
      />
    </div>
  </div>
</template>

<style scoped>
.gh-dlg {
  width: 680px;
}
</style>
