<script setup lang="ts">
import { ref } from 'vue'
import type { NoteNode } from '@/api/client'
import NoteTree, { canDropHere, type TreeCtx } from './NoteTree.vue'

defineProps<{
  treeOpen: boolean
  tree: NoteNode[]
  ctx: TreeCtx
}>()

const emit = defineEmits<{
  (e: 'pick', uploads: { path: string; file: File }[]): void
}>()

const filePick = ref<HTMLInputElement>()
const dirPick = ref<HTMLInputElement>()

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  const uploads = [...(input.files ?? [])].map((file) => ({
    path: file.webkitRelativePath || file.name,
    file,
  }))
  input.value = ''
  emit('pick', uploads)
}
</script>

<template>
  <aside v-show="treeOpen" id="nt-pane" class="nt-pane">
    <div class="nt-head">
      <span class="lbl">{{ $t('Notizen') }}</span>
      <span class="nt-head-act">
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neue Notiz')"
          :title="$t('Neue Notiz')"
          :data-tip="$t('Neue Notiz')"
          @click="ctx.startAdd('', false)"
        >
          <svg class="ic"><use href="#i-plus" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neue Zeichnung')"
          :title="$t('Neue Zeichnung')"
          :data-tip="$t('Neue Zeichnung')"
          @click="ctx.startAdd('', false, true)"
        >
          <svg class="ic"><use href="#i-draw" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neuer Ordner')"
          :title="$t('Neuer Ordner')"
          :data-tip="$t('Neuer Ordner')"
          @click="ctx.startAdd('', true)"
        >
          <svg class="ic"><use href="#i-proj" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Datei hinzufügen')"
          :title="$t('PDF, Bild oder Notiz hinzufügen')"
          :data-tip="$t('PDF, Bild oder Notiz hinzufügen')"
          @click="filePick?.click()"
        >
          <svg class="ic"><use href="#i-upload" /></svg>
        </button>
        <input ref="filePick" type="file" multiple hidden @change="onPick" />
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Ordner hinzufügen')"
          :title="$t('Ordner mit Notizen, PDFs und Bildern hinzufügen')"
          :data-tip="$t('Ordner mit Notizen, PDFs und Bildern hinzufügen')"
          @click="dirPick?.click()"
        >
          <svg class="ic"><use href="#i-folder-up" /></svg>
        </button>
        <input ref="dirPick" type="file" webkitdirectory hidden @change="onPick" />
      </span>
    </div>
    <div
      class="nt-scroll"
      :class="{ drop: ctx.dropTarget === '' }"
      @dragover="
        canDropHere($event, ctx.dragging, '') && ($event.preventDefault(), ctx.setDropTarget(''))
      "
      @dragleave.self="ctx.setDropTarget(null)"
      @drop.prevent="ctx.drop('', $event.dataTransfer)"
    >
      <NoteTree :nodes="tree" :ctx="ctx" />
      <p v-if="!tree.length && !ctx.adding" class="nt-empty">
        {{ $t('Noch keine Notizen. Lege oben eine an oder zieh PDFs und Bilder hierher.') }}
      </p>
    </div>
  </aside>
</template>
