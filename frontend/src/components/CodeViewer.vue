<script setup lang="ts">
// Nur-Lese-Ansicht für C/C++ und Makefiles aus Vorlesungen. Bearbeiten bleibt bei VSCodium.
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView, lineNumbers } from '@codemirror/view'
import { HighlightStyle, StreamLanguage, syntaxHighlighting } from '@codemirror/language'
import { cpp } from '@codemirror/lang-cpp'
import { tags } from '@lezer/highlight'
import { errorMessage, fetchFile } from '@/api/client'

const props = defineProps<{ path: string }>()
const host = ref<HTMLElement>()
const error = ref('')
let view: EditorView | undefined

// Makefiles: Kommentare, Direktiven, Targets, Zuweisungen und $(VAR) reichen zum Lesen.
const make = StreamLanguage.define<null>({
  token(s) {
    if (s.match(/#.*/)) return 'comment'
    if (s.match(/\$(\([^)]*\)|\{[^}]*\}|.)/)) return 'variable-2'
    if (s.sol() && s.match(/-?include\b|ifn?eq\b|ifn?def\b|else\b|endif\b|define\b|endef\b|export\b|override\b/)) return 'keyword'
    if (s.sol() && s.match(/[^\s:#=]+(?=\s*:(?!=))/)) return 'def'
    if (s.sol() && s.match(/[\w.]+(?=\s*[:+?]?=)/)) return 'def'
    s.next()
    return null
  },
})

const style = HighlightStyle.define([
  { tag: tags.keyword, color: 'var(--a-blue-tx)' },
  { tag: tags.comment, color: 'var(--tx-muted)', fontStyle: 'italic' },
  { tag: [tags.string, tags.character], color: 'var(--a-run)' },
  { tag: [tags.number, tags.bool, tags.null, tags.processingInstruction, tags.special(tags.variableName)], color: 'var(--a-red)' },
  { tag: tags.definition(tags.variableName), fontWeight: '600' },
])

// Vorlesungsmaterial ist oft noch Latin-1 (Umlaute in Kommentaren).
function decode(buf: ArrayBuffer) {
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(buf)
  } catch {
    return new TextDecoder('windows-1252').decode(buf)
  }
}

onMounted(async () => {
  try {
    const text = decode(await (await fetchFile(props.path)).blob.arrayBuffer())
    const isMake = /(^|\/)makefile(\.[^/]*)?$/i.test(props.path)
    view = new EditorView({
      parent: host.value!,
      state: EditorState.create({
        doc: text,
        extensions: [lineNumbers(), EditorState.readOnly.of(true), isMake ? make : cpp(), syntaxHighlighting(style), EditorState.tabSize.of(4)],
      }),
    })
  } catch (e) {
    error.value = errorMessage(e)
  }
})
onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div v-if="error" class="ed-bar" role="alert">{{ error }}</div>
  <div ref="host" class="cv" />
</template>

<style scoped>
.cv { flex: 1; min-height: 0; display: flex; }
.cv :deep(.cm-editor) { flex: 1; min-width: 0; font: 400 13px/1.55 var(--font-mono); color: var(--tx-primary); }
.cv :deep(.cm-editor.cm-focused) { outline: none; }
.cv :deep(.cm-scroller) { overflow: auto; font: inherit; }
.cv :deep(.cm-content) { padding: 12px 0; caret-color: var(--tx-primary); }
.cv :deep(.cm-gutters) { background: transparent; border: 0; color: var(--tx-disabled); padding-left: 8px; }
.cv :deep(.cm-line) { padding: 0 16px; }
.cv :deep(.cm-selectionBackground), .cv :deep(.cm-focused .cm-selectionBackground) { background: var(--bg-selected) !important; }
</style>
