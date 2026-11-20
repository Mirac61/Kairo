<script setup lang="ts">
export interface RhythmRow {
  id: string
  name: string
  days: boolean[]
  count: number
}

defineProps<{
  rows: RhythmRow[]
  rangeLabel: string
}>()
</script>

<template>
  <div v-if="rows.length" class="pr-rhythm">
    <div class="pr-sec-head">
      <span class="pr-sec-title">{{ $t('Wochen-Rhythmus') }}</span>
      <span class="pr-muted pr-mono">{{ rangeLabel }}</span>
    </div>
    <div class="pr-rhythm-list">
      <div v-for="r in rows" :key="r.id" class="pr-rhythm-row">
        <span class="pr-rhythm-name">{{ r.name }}</span>
        <div class="pr-rhythm-right">
          <span class="pr-rhythm-dashes" aria-hidden="true">
            <i v-for="(ok, idx) in r.days" :key="idx" :class="{ on: ok }"></i>
          </span>
          <span class="pr-rhythm-stat">{{ r.count }}/7</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pr-rhythm {
  padding-top: 20px;
  border-top: 1px solid var(--br-subtle);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.pr-sec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.pr-sec-title {
  font: 500 11.5px/1 var(--font-ui);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.pr-muted {
  color: var(--tx-muted);
}
.pr-mono {
  font-family: var(--font-mono);
  font-size: 11px;
}
.pr-rhythm-list {
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.pr-rhythm-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font: 400 13px/1.2 var(--font-ui);
}
.pr-rhythm-name {
  color: var(--tx-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pr-rhythm-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.pr-rhythm-dashes {
  display: flex;
  align-items: center;
  gap: 4px;
}
.pr-rhythm-dashes i {
  display: block;
  width: 10px;
  height: 3px;
  border-radius: 2px;
  background: var(--br-default);
}
.pr-rhythm-dashes i.on {
  background: var(--a-blue-hi);
}
.pr-rhythm-stat {
  font-family: var(--font-mono);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
  min-width: 24px;
  text-align: right;
}
</style>
