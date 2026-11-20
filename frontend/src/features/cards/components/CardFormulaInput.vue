<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { hasMath as detectMath } from '@/lib/math'
import CardText from './CardText.vue'

// Eingabefeld für Karten-Texte: reiner Text bleibt ein ruhiges Textarea; erst sobald $…$ oder $$…$$
// vorkommt, erscheint darunter eine kompakte Live-Vorschau (debounced für flüssiges Tippen).
const props = withDefaults(
  defineProps<{
    modelValue: string
    id?: string
    label?: string
    rows?: number
    placeholder?: string
    autofocus?: boolean
    variant?: 'default' | 'title' | 'bare'
  }>(),
  {
    id: undefined,
    label: undefined,
    rows: 3,
    placeholder: undefined,
    autofocus: false,
    variant: 'default',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  change: [e: Event]
  keydown: [e: KeyboardEvent]
}>()

const hasMath = computed(() => detectMath(props.modelValue, { skipCode: true }))
const previewText = ref(props.modelValue)
let timer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.modelValue,
  (val) => {
    if (timer) clearTimeout(timer)
    if (!hasMath.value) {
      previewText.value = val
      return
    }
    timer = setTimeout(() => {
      previewText.value = val
    }, 150)
  },
)
onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})

const el = ref<HTMLTextAreaElement>()
defineExpose({ focus: () => el.value?.focus() })
</script>

<template>
  <div class="cfi">
    <textarea
      :id="id"
      ref="el"
      class="input cfi-in"
      :class="[`is-${variant}`, { 'has-math': hasMath }]"
      :value="modelValue"
      :rows="rows"
      :aria-label="label"
      :autofocus="autofocus || undefined"
      :placeholder="placeholder"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
      @change="emit('change', $event)"
      @keydown="emit('keydown', $event)"
    ></textarea>
    <div v-if="hasMath" class="cfi-prev" aria-live="polite">
      <span class="cfi-lbl">{{ $t('Vorschau') }}</span>
      <CardText class="cfi-body" :text="previewText" />
    </div>
  </div>
</template>

<style scoped>
.cfi {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.cfi-in {
  height: auto;
  resize: vertical;
  font: 400 14px/1.55 var(--font-ui);
  padding: 10px 12px;
}
.cfi-in.has-math {
  font: 400 13.5px/1.6 var(--font-mono);
}
.cfi-in.is-title {
  padding: 6px 0;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: transparent;
  resize: none;
  font: 600 19px/1.35 var(--font-ui);
  letter-spacing: -0.015em;
  transition: border-color var(--dur) var(--ease);
}
.cfi-in.is-title.has-math {
  font: 500 16px/1.45 var(--font-mono);
}
.cfi-in.is-title:hover {
  border-bottom-color: var(--br-default);
}
.cfi-in.is-title:focus-visible {
  outline: none;
  box-shadow: none;
  border-radius: 0;
  border-bottom-color: var(--a-focus);
}
.cfi-in.is-bare {
  padding: 6px 0;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: transparent;
  font: 400 15px/1.65 var(--font-ui);
  transition: border-color var(--dur) var(--ease);
}
.cfi-in.is-bare.has-math {
  font: 400 14px/1.65 var(--font-mono);
}
.cfi-in.is-bare:focus-visible {
  outline: none;
  box-shadow: none;
  border-radius: 0;
  border-bottom-color: var(--a-focus);
}
.cfi-prev {
  padding: 10px 14px;
  border-radius: var(--r-s);
  border: 1px solid var(--br-subtle);
  background: var(--bg-card);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cfi-lbl {
  font: 500 10.5px/1 var(--font-ui);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.cfi-body {
  font: 400 14.5px/1.6 var(--font-ui);
  color: var(--tx-primary);
}
</style>
