<script setup lang="ts">
import { computed, ref, useId, watchEffect } from 'vue'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import { dark } from '@/lib/prefs'

// Frage oder Antwort einer Karte als Markdown. Formeln in $…$ und $$…$$ werden zu SVG (MathJax, eigener Chunk, nur geladen,
// wenn ein $ vorkommt). md-editor filtert eingebettetes HTML (XSS-Plugin), daher stehen im Markdown nur Platzhalter, die sanitize
// im fertigen HTML ersetzt. shortcut: ![[Bild]] aus Notizkarten löst die Vorschau nicht auf; wie im Notiz-Editor nachziehen, wenn gebraucht.
const props = defineProps<{ text: string }>()
const id = useId()
const md = ref(props.text)
const svgs = ref<string[]>([])
const MATH = /\$\$([\s\S]+?)\$\$|\$([^$\n]+?)\$/g

watchEffect(async () => {
  const src = props.text
  if (!src.includes('$')) {
    svgs.value = []
    md.value = src
    return
  }
  const { texToSvg } = await import('@/lib/tex')
  if (src !== props.text) return // inzwischen eine andere Karte
  const out: string[] = []
  md.value = src.replace(MATH, (_, block: string | undefined, inline: string) => {
    out.push(
      `<span class="ct-math${block ? ' is-block' : ''}">${texToSvg((block ?? inline).trim(), !!block)}</span>`,
    )
    return `KAIROMATH${out.length - 1}X`
  })
  svgs.value = out
})
const sanitize = computed(() => {
  const list = svgs.value
  return (html: string) => html.replace(/KAIROMATH(\d+)X/g, (m, i: string) => list[Number(i)] ?? m)
})
</script>

<template>
  <MdPreview
    :id="id"
    class="ct"
    :model-value="md"
    :theme="dark ? 'dark' : 'light'"
    language="en-US"
    preview-theme="default"
    no-mermaid
    no-katex
    no-highlight
    no-echarts
    no-img-zoom-in
    :code-foldable="false"
    :sanitize="sanitize"
  />
</template>

<style scoped>
/* md-editor-v3 an Kairo anpassen: keine eigene Fläche, Schrift und Farben aus den Tokens. */
.ct {
  --md-bk-color: transparent;
  background: none;
  font: inherit;
  color: inherit;
}
.ct :deep(.md-editor-preview-wrapper) {
  padding: 0;
}
.ct :deep(.md-editor-preview) {
  font: inherit;
  color: inherit;
  word-break: break-word;
}
.ct :deep(.md-editor-preview > :first-child) {
  margin-top: 0;
}
.ct :deep(.md-editor-preview > :last-child) {
  margin-bottom: 0;
}
.ct :deep(.md-editor-preview p) {
  margin: 0.5em 0;
  line-height: inherit;
}
.ct :deep(.md-editor-preview pre),
.ct :deep(.md-editor-preview code) {
  font-family: var(--font-mono);
  font-size: 0.85em;
}
.ct :deep(.md-editor-preview pre) {
  text-align: left;
  border-radius: var(--r-s);
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
}
.ct :deep(.md-editor-preview img) {
  max-width: 100%;
  border-radius: var(--r-s);
}
.ct :deep(.ct-math svg) {
  vertical-align: middle;
  max-width: 100%;
}
.ct :deep(.ct-math.is-block) {
  display: block;
  margin: 0.6em 0;
  text-align: center;
}
</style>
