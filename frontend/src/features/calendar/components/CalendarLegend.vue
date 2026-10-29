<script setup lang="ts">
</script>

<template>
  <div class="cal-legend" aria-hidden="true">
    <span><i class="lg-termin" />{{ $t('Termin') }}</span>
    <span><i class="lg-task" />{{ $t('Aufgabe') }}</span>
    <span><i class="lg-track" />{{ $t('Erfasst') }}</span>
  </div>
</template>

<style scoped>
.cal-legend {
  display: flex;
  align-items: center;
  gap: 14px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.cal-legend span {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cal-legend i {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  box-sizing: border-box;
  border: 1px solid var(--tx-muted);
}
.lg-termin {
  background: color-mix(in oklab, var(--tx-secondary) 30%, transparent);
}
.lg-task {
  border-style: dashed !important;
}
.lg-track {
  width: 3px !important;
  height: 12px !important;
  border: 0 !important;
  border-radius: 2px !important;
  background: var(--tx-secondary);
}

@container (max-width: 1180px) {
  .cal-legend {
    display: none;
  }
}
</style>
