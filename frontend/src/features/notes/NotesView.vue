<script setup lang="ts">
import { PANES, useNotesState, type Pane } from './composables/useNotesState'
import NotesConflictDialog from './components/NotesConflictDialog.vue'
import NotesEditorPane from './components/NotesEditorPane.vue'
import NotesSidebar from './components/NotesSidebar.vue'
import './notesView.css'

const {
  tree,
  allFiles,
  obsidianDrawings,
  converting,
  converted,
  convertDrawings,
  paths,
  shown,
  split,
  focus,
  visible,
  bufs,
  docKind,
  setDocEd,
  docDirtyFlag,
  drawingRev,
  isDirty,
  conflict,
  saving,
  error,
  errorPane,
  treeOpen,
  toggleTree,
  ratio,
  startResize,
  nudge,
  toggleSplit,
  ctx,
  onPickUploads,
  headings,
  openFile,
  openDrawing,
  createNoteFromLink,
  uploaded,
  templates,
  useTemplate,
  save,
  reload,
} = useNotesState()

function onActivatePane(pane: Pane) {
  focus.value = pane
}
</script>

<template>
  <div class="notes" :class="{ shut: !treeOpen }">
    <NotesSidebar :tree-open="treeOpen" :tree="tree" :ctx="ctx" @pick="onPickUploads" />

    <div
      class="eds"
      :class="{ split }"
      :style="
        split
          ? { gridTemplateColumns: `minmax(0, ${ratio}fr) minmax(0, ${1 - ratio}fr)` }
          : undefined
      "
    >
      <NotesEditorPane
        v-for="pn in PANES"
        :key="pn"
        :pane="pn"
        :path="paths[pn]"
        :buf="bufs[pn]"
        :is-dirty="isDirty(pn)"
        :saving="saving"
        :split="split"
        :shown="shown"
        :focus="focus"
        :tree-open="treeOpen"
        :visible="visible(pn)"
        :error="error"
        :error-pane="errorPane"
        :obsidian-drawings="obsidianDrawings"
        :converted="converted"
        :converting="converting"
        :templates="templates"
        :all-files="allFiles"
        :heading="headings[pn]"
        :drawing-rev="drawingRev"
        :doc-kind="docKind"
        @activate-pane="onActivatePane"
        @toggle-tree="toggleTree"
        @toggle-split="toggleSplit"
        @save="save"
        @reload="reload"
        @convert-drawings="convertDrawings"
        @close-converted="converted = null"
        @use-template="useTemplate"
        @update-text="bufs[pn].text = $event"
        @uploaded="uploaded"
        @error="error = $event"
        @open-drawing="openDrawing"
        @open-note="openFile"
        @create-note="createNoteFromLink"
        @set-doc-ed="setDocEd"
        @doc-dirty="docDirtyFlag = $event"
      />
      <div
        v-if="split"
        class="ed-split"
        role="separator"
        aria-orientation="vertical"
        :aria-label="$t('Breite der Hälften')"
        tabindex="0"
        :aria-valuenow="Math.round(ratio * 100)"
        aria-valuemin="25"
        aria-valuemax="75"
        :style="{ left: `${ratio * 100}%` }"
        @pointerdown="startResize"
        @keydown.left.prevent="nudge(-0.05)"
        @keydown.right.prevent="nudge(0.05)"
      />
    </div>

    <NotesConflictDialog
      :open="!!conflict"
      :path="paths[conflict ?? 'note'] ?? ''"
      @close="conflict = null"
      @reload="conflict && reload(conflict)"
      @save="conflict && save(true, conflict)"
    />
  </div>
</template>
