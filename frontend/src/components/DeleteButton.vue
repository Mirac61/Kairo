<script setup lang="ts">
import { useConfirm } from 'primevue/useconfirm'

const props = defineProps<{ text: string }>()
const emit = defineEmits<{ confirm: [] }>()
const confirm = useConfirm()

function ask(e: Event) {
  confirm.require({
    target: e.currentTarget as HTMLElement,
    message: props.text,
    acceptLabel: 'Löschen',
    rejectLabel: 'Abbrechen',
    acceptProps: { severity: 'danger', size: 'small' },
    rejectProps: { severity: 'secondary', size: 'small', text: true },
    accept: () => emit('confirm'),
  })
}
</script>

<template>
  <button type="button" class="btn btn-danger" @click="ask">Löschen</button>
</template>
