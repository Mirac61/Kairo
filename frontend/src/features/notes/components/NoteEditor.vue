<script lang="ts">
import { EditorView } from '@codemirror/view'
import { config } from 'md-editor-v3'
import { slugify } from '@/lib/noteFiles'

// Rechtschreibprüfung des Browsers bzw. von macOS (rote Wellenlinie, Vorschläge per Rechtsklick); CodeMirror schaltet sie sonst ab.
// ![[Name]] und ![[Name|900]] wie in Obsidian: ein <img data-wiki>, das sanitize (previewImages) zum Pfad auflöst.
// [[Name]], [[Name|Alias]], [[Name#Überschrift]]: ein <a data-wikilink>, das sanitize (previewWikiLinks) zur Notiz auflöst.
config({
  codeMirrorExtensions: (exts) => [
    ...exts,
    {
      type: 'spellcheck',
      extension: EditorView.contentAttributes.of({ spellcheck: 'true', autocorrect: 'on' }),
    },
  ],
  markdownItConfig: (md) => {
    // Überschriften bekommen einen Anker (slugify), damit [[Name#Abschnitt]] dorthin scrollen kann.
    const headingOpen =
      md.renderer.rules.heading_open ||
      ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))
    md.renderer.rules.heading_open = (tokens, idx, options, env, self) => {
      const text = tokens[idx + 1]?.content ?? ''
      tokens[idx]!.attrSet('id', slugify(text))
      return headingOpen(tokens, idx, options, env, self)
    }
    md.inline.ruler.before('image', 'wikiEmbed', (state, silent) => {
      const m = /^!\[\[([^\]|\n]+)(?:\|([^\]\n]*))?\]\]/.exec(state.src.slice(state.pos))
      if (!m) return false
      if (!silent) {
        const img = state.push('image', 'img', 0)
        img.attrs = [
          ['src', m[1]!.trim()],
          ['alt', ''],
          ['data-wiki', ''],
        ]
        const width = /^\d+/.exec(m[2] ?? '')?.[0]
        if (width) img.attrs.push(['width', width])
        img.children = []
      }
      state.pos += m[0].length
      return true
    })
    md.inline.ruler.before('link', 'wikiLink', (state, silent) => {
      const m = /^\[\[([^[\]\n]+)\]\]/.exec(state.src.slice(state.pos))
      if (!m) return false
      if (!silent) {
        state.push('link_open', 'a', 1).attrs = [
          ['href', '#'],
          ['data-wikilink', m[1]!],
        ]
        state.push('text', '', 0).content = parseWikiLink(m[1]!).label
        state.push('link_close', 'a', -1)
      }
      state.pos += m[0].length
      return true
    })
  },
})
</script>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { MdEditor, type ToolbarNames } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { ApiError, createNote, errorMessage, readNote, uploadFile } from '@/api/client'
import {
  assetName,
  drawingsIn,
  headingsOf,
  join,
  parseWikiLink,
  previewImages,
  previewWikiLinks,
  resolveLink,
  resolveNote,
} from '@/lib/noteFiles'
import { wikiCompletionSource } from '@/lib/noteAutocomplete'
import { drawingSrc, renderDrawing } from '@/lib/drawingPreview'
import { parentOf } from './NoteTree.vue'

// Markdown-Editor der Notizen. Mermaid, KaTeX, Prettier und Highlight lädt md-editor-v3 vom CDN; sie sind abgeschaltet
// (Kairo läuft lokal). Bilder landen über uploadImages neben der Notiz.
// drawingRev: NotesView erhöht ihn nach dem Speichern einer Zeichnung und alle 5 s; eingebettete Zeichnungen prüfen dann ihre mtime.
// files: alle Dateien im Notizordner, für ![[Name]] (Suche nach dem Namen wie in Obsidian). scrollHeading: Ziel-Überschrift.
const props = defineProps<{
  path: string
  drawingRev?: number
  files?: string[]
  scrollHeading?: string
}>()
const text = defineModel<string>({ required: true })
const emit = defineEmits<{
  uploaded: []
  error: [string]
  open: [string]
  openNote: [string, string]
  createNote: [string]
}>()

const TOOLBAR: ToolbarNames[] = [
  'bold',
  'italic',
  'strikeThrough',
  'title',
  '-',
  'quote',
  'unorderedList',
  'orderedList',
  'task',
  '-',
  'codeRow',
  'code',
  'link',
  'image',
  'table',
  '-',
  'revoke',
  'next',
  '=',
  'preview',
  'previewOnly',
]

// Eingefügte Bilder (⌘V, Toolbar) landen in assets/ neben der Notiz; der Link ist relativ, damit VSCodium sie genauso zeigt.
const ASSETS = 'assets'
async function uploadImages(files: File[], done: (urls: string[]) => void) {
  const p = props.path
  const now = new Date()
  const dir = join(parentOf(p), ASSETS)
  try {
    await createNote(dir, true).catch((e) => {
      if (!(e instanceof ApiError && e.status === 409)) throw e
    }) // 409: gibt es schon
    const names: string[] = []
    for (const [i, f] of files.entries()) {
      const name = assetName(p, f.type, now, i, files.length)
      await uploadFile(join(dir, name), f)
      names.push(`${ASSETS}/${encodeURI(name)}`)
    }
    done(names)
    emit('uploaded')
  } catch (e) {
    emit('error', errorMessage(e))
  }
}
// Eingebettete Zeichnungen (![](x.excalidraw)): sanitize ist synchron und nimmt fertige Bilder aus dem Cache. Fehlt eins,
// rendert drawingPreview es nach; rendered++ gibt sanitize eine neue Identität, und md-editor rendert die Vorschau neu.
const rendered = ref(0)
let drawings: string[] = []
const redraw = (ps: string[]) =>
  void Promise.all(ps.map((p) => renderDrawing(p).catch(() => false))).then((r) => {
    if (r.includes(true)) rendered.value++
  })
const sanitize = computed(() => {
  void rendered.value
  const files = props.files ?? []
  return (html: string) => {
    const dir = parentOf(props.path)
    const out = previewWikiLinks(
      previewImages(html, props.path, drawingSrc, (name) => resolveLink(name, dir, files)),
      (name) => (name ? resolveNote(name, dir, files) : props.path),
    )
    drawings = drawingsIn(out)
    redraw(drawings.filter((p) => !drawingSrc(p)))
    return out
  }
})
watch(
  () => props.drawingRev,
  () => redraw(drawings),
)

// Autovervollständigung für [[…]] und ![[…]]: md-editor-v3 reicht CompletionSources an CodeMirror weiter.
const completions = computed(() => [
  wikiCompletionSource({
    path: props.path,
    files: props.files ?? [],
    readHeadings: (p) => readNote(p).then((n) => headingsOf(n.content)),
  }),
])

// [[Name#Abschnitt]]: nach dem Öffnen zur Überschrift in der Vorschau scrollen.
const root = ref<HTMLElement>()
watch(
  () => props.scrollHeading,
  (h) => {
    if (!h) return
    void nextTick(() => {
      root.value
        ?.querySelector(`.md-editor-preview [id="${CSS.escape(slugify(h))}"]`)
        ?.scrollIntoView({ block: 'start', behavior: 'smooth' })
    })
  },
)

// Bilder in der Vorschau öffnen sich per Klick in voller Auflösung in einem neuen Tab, Zeichnungen links in Kairo, [[Links]] die Notiz
// (mit Überschrift für [[Name#Abschnitt]]); ein unaufgelöster Link (md-wiki-unresolved) meldet den Namen, damit die Ansicht die Notiz anlegen kann.
function openPreviewImage(e: MouseEvent) {
  const link = (e.target as HTMLElement).closest<HTMLAnchorElement>(
    '.md-editor-preview a.md-wikilink',
  )
  if (link) {
    e.preventDefault()
    if (link.dataset.note) emit('openNote', link.dataset.note, link.dataset.heading || '')
    else if (link.dataset.name) emit('createNote', link.dataset.name)
    return
  }
  const img = (e.target as HTMLElement).closest<HTMLImageElement>('.md-editor-preview img')
  if (img?.dataset.drawing) emit('open', img.dataset.drawing)
  else if (img) window.open(img.src, '_blank')
}

// md-editor-v3 bekommt das Theme als Prop; Kairo schaltet es über data-theme an <html>.
const dark = ref(document.documentElement.dataset.theme !== 'light')
const themeObserver = new MutationObserver(() => {
  dark.value = document.documentElement.dataset.theme !== 'light'
})
themeObserver.observe(document.documentElement, {
  attributes: true,
  attributeFilter: ['data-theme'],
})
onBeforeUnmount(() => themeObserver.disconnect())
</script>

<template>
  <div ref="root" class="nd" @click="openPreviewImage">
    <MdEditor
      v-model="text"
      :theme="dark ? 'dark' : 'light'"
      language="en-US"
      preview-theme="default"
      :toolbars="TOOLBAR"
      :footers="[]"
      :sanitize="sanitize"
      :completions="completions"
      no-highlight
      no-mermaid
      no-katex
      no-prettier
      no-echarts
      @on-upload-img="uploadImages"
    />
  </div>
</template>

<style scoped>
.nd {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.nd :deep(.md-editor-preview img) {
  cursor: zoom-in;
}
.nd :deep(.md-editor-preview a.md-wiki-unresolved) {
  color: var(--tx-muted);
  text-decoration: underline dotted;
  cursor: not-allowed;
}
.nd :deep(.md-editor-preview img[data-drawing]) {
  cursor: pointer;
}

/* md-editor-v3 an die Kairo-Tokens anpassen (gilt für Dark und Light, weil die Tokens mitwechseln). */
.nd :deep(.md-editor) {
  --md-color: var(--tx-primary);
  --md-hover-color: var(--tx-primary);
  --md-bk-color: var(--bg-0);
  --md-bk-color-outstand: var(--bg-2);
  --md-bk-hover-color: var(--bg-hover);
  --md-border-color: var(--br-subtle);
  --md-border-hover-color: var(--br-strong);
  --md-border-active-color: var(--br-strong);
  --md-modal-mask: var(--scrim);
  --md-modal-shadow: var(--shadow-pop);
  --md-scrollbar-bg-color: transparent;
  --md-scrollbar-thumb-color: var(--br-default);
  --md-scrollbar-thumb-hover-color: var(--br-strong);
  --md-scrollbar-thumb-active-color: var(--br-strong);
  flex: 1;
  min-height: 0;
  height: auto;
  border: 0;
  border-radius: 0;
  font-family: var(--font-ui);
}
.nd :deep(.md-editor .md-editor-toolbar-wrapper) {
  padding: 4px 16px;
  border-bottom-color: var(--br-subtle);
}
.nd :deep(.md-editor .md-editor-toolbar-item) {
  color: var(--tx-secondary);
}
@media (max-width: 760px) {
  .nd :deep(.md-editor .md-editor-toolbar-wrapper) {
    padding: 4px 8px;
    overflow-x: auto;
  }
}
.nd :deep(.md-editor .cm-editor) {
  font-family: var(--font-mono);
  font-size: 13px;
}
.nd :deep(.md-editor .cm-scroller) {
  padding: 0 8px;
}
.nd :deep(.md-editor .md-editor-preview) {
  max-width: 72ch; /* lange Zeilen lesen sich schlecht, auch wenn nur die Vorschau offen ist */
  --md-theme-color: var(--tx-secondary);
  --md-theme-heading-color: var(--tx-primary);
  --md-theme-strong-color: var(--tx-primary);
  --md-theme-link-color: var(--a-blue-tx);
  --md-theme-link-hover-color: var(--a-blue-hi);
  --md-theme-border-color: var(--br-subtle);
  --md-theme-bg-color: var(--bg-0);
  --md-theme-bg-color-inset: var(--bg-2);
  --md-theme-quote-color: var(--tx-muted);
  --md-theme-quote-border: 3px solid var(--br-strong);
  --md-theme-quote-bg-color: transparent;
  --md-theme-code-inline-color: var(--tx-primary);
  --md-theme-code-inline-bg-color: var(--bg-2);
  --md-theme-code-block-color: var(--tx-primary);
  --md-theme-code-block-bg-color: var(--bg-1);
  --md-theme-table-stripe-color: var(--bg-1);
  --md-theme-table-border-color: var(--br-subtle);
  --md-theme-table-td-border-color: var(--br-subtle);
  font: 400 15px/1.6 var(--font-ui);
  word-break: normal;
  overflow-wrap: anywhere; /* Standard ist break-all: bricht mitten im Wort */
}
</style>

<style>
/* Zeichnungen sind hell gezeichnet; im Dunkeln invertieren wie Excalidraw selbst. Ungescoped, weil :global() in
   scoped CSS den Rest des Selektors verwirft und dann die ganze Seite träfe. */
[data-theme='dark'] .nd .md-editor-preview img[data-drawing] {
  filter: invert(93%) hue-rotate(180deg);
}
</style>
