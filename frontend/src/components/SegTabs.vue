<script setup lang="ts" generic="T extends string">
// Umschalter als Pillen (Klasse .seg). Zahlen sind optional, leere Optionen lassen sich sperren.
export interface SegOption<V extends string = string> {
  value: V
  label: string
  count?: number
  disabled?: boolean
}
defineProps<{ options: SegOption<T>[]; label: string }>()
const model = defineModel<T>({ required: true })
</script>

<template>
  <div class="seg" role="group" :aria-label="label">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      :aria-pressed="model === o.value"
      :disabled="o.disabled"
      @click="model = o.value"
    >
      {{ o.label }}<span v-if="o.count !== undefined" class="count">{{ o.count }}</span>
    </button>
  </div>
</template>
