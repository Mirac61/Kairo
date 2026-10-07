<script setup lang="ts">
import { useConfirm } from 'primevue/useconfirm'

const props = defineProps<{ text: string; label?: string; ghost?: boolean }>()
const emit = defineEmits<{ confirm: [] }>()
const confirm = useConfirm()

// Rot bleibt für „überfällig“ und Fehler: die Bestätigung ist primär (Textfarbe), ihr Wort nennt die Folge,
// der erste Fokus liegt auf „Abbrechen“.
function ask(e: Event) {
  confirm.require({
    target: e.currentTarget as HTMLElement,
    message: props.text,
    acceptLabel: props.label ?? 'Löschen',
    rejectLabel: 'Abbrechen',
    defaultFocus: 'reject',
    acceptProps: { size: 'small' },
    rejectProps: { severity: 'secondary', size: 'small', text: true },
    accept: () => emit('confirm'),
  })
}
</script>

<template>
  <button type="button" class="btn" :class="ghost ? 'btn-ghost' : 'btn-danger'" @click="ask">{{ label ?? 'Löschen' }}</button>
</template>
