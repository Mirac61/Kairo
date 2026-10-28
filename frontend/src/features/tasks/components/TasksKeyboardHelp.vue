<script setup lang="ts"></script>

<template>
  <div class="keys" :aria-label="$t('Tastenkürzel')">
    <span><kbd class="key">n</kbd>{{ $t('Neu') }}</span>
    <span><kbd class="key">j</kbd><kbd class="key">k</kbd>{{ $t('Wählen') }}</span>
    <span><kbd class="key">x</kbd>{{ $t('Erledigt') }}</span>
    <span><kbd class="key">Enter</kbd>{{ $t('Öffnen') }}</span>
    <span><kbd class="key">s</kbd>{{ $t('Start/Pause') }}</span>
    <span><kbd class="key">t</kbd>{{ $t('Heute') }}</span>
    <span><kbd class="key">1</kbd>–<kbd class="key">7</kbd>{{ $t('Reiter') }}</span>
  </div>
</template>

<style scoped>
.keys {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 20px;
  justify-content: center;
  margin-top: 24px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.keys span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.keys .key {
  min-width: 20px;
  text-align: center;
  color: var(--tx-muted);
}
</style>
