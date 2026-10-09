<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { MdEditor, type ToolbarNames } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { ApiError, createNote, errorMessage, uploadFile } from '@/api/client'
import { assetName, join, previewImages } from '@/lib/noteFiles'
import { parentOf } from '@/components/NoteTree.vue'

// Markdown-Editor der Notizen. Mermaid, KaTeX, Prettier und Highlight lädt md-editor-v3 vom CDN; sie sind abgeschaltet
// (Kairo läuft lokal). Bilder landen über uploadImages neben der Notiz.
const props = defineProps<{ path: string }>()
const text = defineModel<string>({ required: true })
const emit = defineEmits<{ uploaded: []; error: [string] }>()

const TOOLBAR: ToolbarNames[] = [
  'bold', 'italic', 'strikeThrough', 'title', '-', 'quote', 'unorderedList', 'orderedList', 'task', '-',
  'codeRow', 'code', 'link', 'image', 'table', '-', 'revoke', 'next', '=', 'preview', 'previewOnly',
]

// Eingefügte Bilder (⌘V, Toolbar) landen in assets/ neben der Notiz; der Link ist relativ, damit VSCodium sie genauso zeigt.
const ASSETS = 'assets'
async function uploadImages(files: File[], done: (urls: string[]) => void) {
  const p = props.path
  const now = new Date()
  const dir = join(parentOf(p), ASSETS)
  try {
    await createNote(dir, true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e }) // 409: gibt es schon
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
const sanitize = (html: string) => previewImages(html, props.path)

// Bilder in der Vorschau öffnen sich per Klick in voller Auflösung in einem neuen Tab.
function openPreviewImage(e: MouseEvent) {
  const img = (e.target as HTMLElement).closest<HTMLImageElement>('.md-editor-preview img')
  if (img) window.open(img.src, '_blank')
}

// md-editor-v3 bekommt das Theme als Prop; Kairo schaltet es über data-theme an <html>.
const dark = ref(document.documentElement.dataset.theme !== 'light')
const themeObserver = new MutationObserver(() => { dark.value = document.documentElement.dataset.theme !== 'light' })
themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
onBeforeUnmount(() => themeObserver.disconnect())
</script>

<template>
  <div class="nd" @click="openPreviewImage">
    <MdEditor
      v-model="text" :theme="dark ? 'dark' : 'light'" language="en-US" preview-theme="default"
      :toolbars="TOOLBAR" :footers="[]" :sanitize="sanitize" no-highlight no-mermaid no-katex no-prettier no-echarts
      @on-upload-img="uploadImages"
    />
  </div>
</template>

<style scoped>
.nd { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.nd :deep(.md-editor-preview img) { cursor: zoom-in; }

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
  flex: 1; min-height: 0; height: auto; border: 0; border-radius: 0;
  font-family: var(--font-ui);
}
.nd :deep(.md-editor .md-editor-toolbar-wrapper) { padding: 4px 16px; border-bottom-color: var(--br-subtle); }
.nd :deep(.md-editor .md-editor-toolbar-item) { color: var(--tx-secondary); }
.nd :deep(.md-editor .cm-editor) { font-family: var(--font-mono); font-size: 13px; }
.nd :deep(.md-editor .cm-scroller) { padding: 0 8px; }
.nd :deep(.md-editor .md-editor-preview) {
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
  word-break: normal; overflow-wrap: anywhere; /* Standard ist break-all: bricht mitten im Wort */
}
</style>
