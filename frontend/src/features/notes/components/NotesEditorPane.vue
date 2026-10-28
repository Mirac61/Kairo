<script setup lang="ts">
import { computed } from 'vue'
import type { NoteNode } from '@/api/client'
import CodeViewer from './CodeViewer.vue'
import DrawingEditor from './DrawingEditor.vue'
import FileEditor from './FileEditor.vue'
import NoteBacklinks from './NoteBacklinks.vue'
import NoteEditor from './NoteEditor.vue'
import { fileKind, under } from '@/lib/noteFiles'
import { tn } from '@/lib/i18n'

type Pane = 'doc' | 'note'
type Buf = { text: string; saved: string; mtime: string; external: boolean }

const props = defineProps<{
  pane: Pane
  path: string | null
  buf: Buf
  isDirty: boolean
  saving: boolean
  split: boolean
  shown: Pane
  focus: Pane
  treeOpen: boolean
  visible: boolean
  error: string
  errorPane: Pane
  obsidianDrawings: string[]
  converted: { done: string[]; skipped: string[] } | null
  converting: boolean
  templates: NoteNode[]
  allFiles: string[]
  heading: string
  drawingRev: number
  docKind: string
}>()

const emit = defineEmits<{
  (e: 'activatePane', pane: Pane): void
  (e: 'toggleTree'): void
  (e: 'toggleSplit'): void
  (e: 'save', force?: boolean, pane?: Pane): void
  (e: 'reload', pane: Pane): void
  (e: 'convertDrawings'): void
  (e: 'closeConverted'): void
  (e: 'useTemplate', tpl: NoteNode, pane: Pane): void
  (e: 'updateText', text: string): void
  (e: 'uploaded'): void
  (e: 'error', msg: string): void
  (e: 'openDrawing', path: string): void
  (e: 'openNote', path: string): void
  (e: 'createNote', name: string): void
  (e: 'setDocEd', el: unknown): void
  (e: 'docDirty', dirty: boolean): void
}>()

const isMd = computed(() => !!props.path && fileKind(props.path) === 'md')
const crumbs = computed(() => {
  if (!props.path) return []
  return (fileKind(props.path) === 'code' ? props.path : props.path.replace(/\.[^./]+$/, '')).split(
    '/',
  )
})
const canSave = computed(
  () => !!props.path && !/\.gif$/i.test(props.path) && fileKind(props.path) !== 'code',
)
const offerTemplates = computed(
  () =>
    isMd.value &&
    !props.buf.text.trim() &&
    props.templates.length > 0 &&
    !under(props.path!, 'Vorlagen'),
)
</script>

<template>
  <section
    v-show="visible"
    class="ed"
    :class="[`ed-${pane}`, { on: split && focus === pane }]"
    @pointerdown.capture="emit('activatePane', pane)"
    @focusin="emit('activatePane', pane)"
  >
    <header class="ed-head">
      <button
        v-if="pane === (split ? 'doc' : shown)"
        type="button"
        class="icon-btn"
        aria-controls="nt-pane"
        :aria-expanded="treeOpen"
        :aria-label="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'"
        :data-tip="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'"
        @click="emit('toggleTree')"
      >
        <svg class="ic"><use :href="treeOpen ? '#i-left' : '#i-right'" /></svg>
      </button>
      <div class="ed-path">
        <template v-if="path">
          <span v-for="(c, i) in crumbs" :key="i" :class="{ last: i === crumbs.length - 1 }">{{
            c
          }}</span>
        </template>
        <span v-else class="last">{{
          pane === 'doc' ? $t('Keine Datei geöffnet') : $t('Keine Notiz geöffnet')
        }}</span>
      </div>
      <span v-if="isDirty" class="save-status dirty" role="status"
        ><i></i>{{ $t('Ungespeicherte Änderungen') }}</span
      >
      <span v-else-if="path && !saving" class="save-status saved" role="status"
        ><svg class="ic" aria-hidden="true"><use href="#i-check" /></svg
        >{{ $t('Gespeichert') }}</span
      >
      <button
        v-if="canSave"
        type="button"
        class="btn btn-secondary ed-save"
        :disabled="!isDirty || saving"
        aria-keyshortcuts="Meta+S Control+S"
        @click="emit('save', false, pane)"
      >
        {{ $t('Speichern') }}<kbd class="key" aria-hidden="true">⌘S</kbd>
      </button>
      <button
        v-if="pane === (split ? 'note' : shown)"
        type="button"
        class="icon-btn"
        :class="{ on: split }"
        :aria-pressed="split"
        :aria-label="$t('Geteilte Ansicht')"
        :data-tip="split ? $t('Nur eine Datei zeigen') : $t('Geteilt: zwei Dateien nebeneinander')"
        @click="emit('toggleSplit')"
      >
        <svg class="ic"><use href="#i-split" /></svg>
      </button>
    </header>
    <div v-if="error && pane === errorPane" class="ed-bar" role="alert">{{ error }}</div>
    <div
      v-if="pane === errorPane && (obsidianDrawings.length || converted)"
      class="ed-bar"
      role="status"
    >
      <template v-if="converted">
        {{
          tn(
            converted.done.length,
            '{n} Zeichnung umgewandelt, das Original liegt im Papierkorb.',
            '{n} Zeichnungen umgewandelt, die Originale liegen im Papierkorb.',
          )
        }}
        <template v-if="converted.skipped.length">
          {{ $t('Nicht umgewandelt:') }} {{ converted.skipped.join(' · ') }}</template
        >
        <button type="button" class="btn btn-ghost" @click="emit('closeConverted')">
          {{ $t('Schließen') }}
        </button>
      </template>
      <template v-else>
        {{
          tn(
            obsidianDrawings.length,
            '{n} Zeichnung aus Obsidian gefunden.',
            '{n} Zeichnungen aus Obsidian gefunden.',
          )
        }}
        <button
          type="button"
          class="btn btn-ghost"
          :disabled="converting"
          @click="emit('convertDrawings')"
        >
          {{ converting ? $t('Wandle um …') : $t('In Kairo-Zeichnungen umwandeln') }}
        </button>
      </template>
    </div>

    <template v-if="isMd">
      <div v-if="buf.external" class="ed-bar" role="alert">
        {{ $t('Die Datei wurde außerhalb von Kairo geändert. Beim Speichern fragt Kairo nach.') }}
        <button type="button" class="btn btn-ghost" @click="emit('reload', pane)">
          {{ $t('Verwerfen und neu laden') }}
        </button>
      </div>
      <div v-if="offerTemplates" class="ed-tpl">
        <span>{{ $t('Vorlage') }}</span>
        <button
          v-for="tpl in templates"
          :key="tpl.path"
          type="button"
          class="btn btn-ghost"
          @click="emit('useTemplate', tpl, pane)"
        >
          {{ tpl.name.replace(/\.md$/i, '') }}
        </button>
      </div>
      <NoteEditor
        :model-value="buf.text"
        :path="path!"
        :drawing-rev="drawingRev"
        :files="allFiles"
        :scroll-heading="heading"
        @update:model-value="emit('updateText', $event)"
        @uploaded="emit('uploaded')"
        @error="emit('error', $event)"
        @open="emit('openDrawing', $event)"
        @open-note="emit('openNote', $event)"
        @create-note="emit('createNote', $event)"
      />
      <NoteBacklinks :path="path!" :rev="buf.mtime" @open="emit('openNote', $event)" />
    </template>
    <FileEditor
      v-else-if="pane === 'doc' && path && (docKind === 'pdf' || docKind === 'image')"
      :ref="(el) => emit('setDocEd', el)"
      :key="path"
      :path="path"
      :kind="docKind"
      :active="visible && (!split || focus === 'doc')"
      @dirty="emit('docDirty', $event)"
    />
    <DrawingEditor
      v-else-if="pane === 'doc' && path && docKind === 'excalidraw'"
      :ref="(el) => emit('setDocEd', el)"
      :key="path"
      :path="path"
      @dirty="emit('docDirty', $event)"
      @autosave="emit('save', false, 'doc')"
    />
    <CodeViewer v-else-if="pane === 'doc' && path && docKind === 'code'" :key="path" :path="path" />
    <div v-else class="ed-empty">
      {{
        pane === 'doc'
          ? $t('Klick hier hinein und wähle im Baum eine Notiz, ein PDF oder ein Bild.')
          : $t('Wähle links eine Notiz oder lege eine neue an.')
      }}
    </div>
  </section>
</template>
