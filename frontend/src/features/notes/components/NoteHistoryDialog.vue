<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  errorMessage,
  noteHistory,
  noteSnapshot,
  restoreNote,
  type NoteSnapshot,
} from '@/api/client'
import { vDialog } from '@/lib/dialog'
import { locale } from '@/lib/i18n'

const props = defineProps<{ open: boolean; path: string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'restored'): void }>()

const list = ref<NoteSnapshot[]>([])
const picked = ref('')
const preview = ref('')
const error = ref('')
const loaded = ref(false)

watch(
  () => props.open,
  async (isOpen) => {
    if (!isOpen) return
    list.value = []
    picked.value = preview.value = error.value = ''
    loaded.value = false
    try {
      list.value = await noteHistory(props.path)
      if (list.value[0]) await pick(list.value[0].name)
    } catch (e) {
      error.value = errorMessage(e)
    }
    loaded.value = true
  },
)

async function pick(name: string) {
  picked.value = name
  try {
    preview.value = (await noteSnapshot(props.path, name)).content
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function restore() {
  try {
    await restoreNote(props.path, picked.value)
    emit('restored')
    emit('close')
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const when = (iso: string) =>
  new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(iso),
  )
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <div class="dialog nh" role="dialog" :aria-label="$t('Verlauf')">
      <div class="dlg-head">
        <h3>{{ $t('Verlauf') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <p v-if="error" class="badge" role="alert">{{ error }}</p>
        <p v-else-if="loaded && !list.length" class="nh-empty">
          {{
            $t(
              'Noch keine älteren Fassungen. Sie entstehen beim Speichern, höchstens alle 5 Minuten.',
            )
          }}
        </p>
        <div v-else class="nh-grid">
          <ul class="nh-list">
            <li v-for="s in list" :key="s.name">
              <button type="button" :class="{ on: s.name === picked }" @click="pick(s.name)">
                {{ when(s.time) }}
              </button>
            </li>
          </ul>
          <pre class="nh-pre">{{ preview }}</pre>
        </div>
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button class="btn btn-primary" type="button" :disabled="!picked" @click="restore">
          {{ $t('Diese Fassung wiederherstellen') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.nh {
  width: 720px;
  max-width: calc(100vw - 32px);
}
.nh-grid {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 12px;
  height: 50vh;
}
.nh-list {
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-y: auto;
}
.nh-list button {
  width: 100%;
  padding: 6px 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
  text-align: left;
  cursor: pointer;
}
.nh-list button:hover,
.nh-list button.on {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.nh-pre {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  border: 1px solid var(--br-subtle);
  border-radius: 6px;
  font: 400 12.5px/1.5 var(--font-mono, monospace);
  white-space: pre-wrap;
  word-break: break-word;
}
.nh-empty {
  color: var(--tx-muted);
}
@media (max-width: 640px) {
  .nh-grid {
    grid-template-columns: 1fr;
    grid-template-rows: 120px 1fr;
  }
}
</style>
