<script setup lang="ts">
import { vDialog } from '@/lib/dialog'

defineProps<{
  open: boolean
  path: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'reload'): void
  (e: 'save'): void
}>()
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <div class="dialog" :aria-label="$t('Datei extern geändert')">
      <div class="dlg-head">
        <h3>{{ $t('Datei wurde extern geändert') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <p class="ed-msg">
          {{
            $t(
              '„{name}“ wurde seit dem Laden außerhalb von Kairo geändert, zum Beispiel in VSCodium. Überschreiben ersetzt diese Änderungen durch deinen Stand aus Kairo.',
              { name: path },
            )
          }}
        </p>
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button class="btn btn-secondary" type="button" @click="emit('reload')">
          {{ $t('Verwerfen und neu laden') }}
        </button>
        <button class="btn btn-primary" type="button" @click="emit('save')">
          {{ $t('Überschreiben') }}
        </button>
      </div>
    </div>
  </div>
</template>
