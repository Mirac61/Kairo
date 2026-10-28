<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Root } from 'react-dom/client'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import { ApiError, errorMessage, fetchFile, replaceFile } from '@/api/client'
import { lang, t } from '@/lib/i18n'
import { loadExcalidraw } from '@/lib/drawingPreview'

// Zeichnung (.excalidraw, JSON wie bei excalidraw.com) im linken Platz. Gleiche Schnittstelle wie FileEditor:
// NotesView speichert über save() und lädt über reload() neu. Ändert sich die Szene, meldet autosave nach 1 s;
// NotesView speichert dann, damit ein Konflikt (409) im gewohnten Dialog landet.
const props = defineProps<{ path: string }>()
const emit = defineEmits<{ dirty: [boolean]; autosave: [] }>()

const AUTOSAVE_MS = 1000
const host = ref<HTMLDivElement>()
const failed = ref('')
let root: Root | null = null
let api: ExcalidrawImperativeAPI | null = null
let lib: Awaited<ReturnType<typeof loadExcalidraw>> | null = null
let scene: unknown = null // geladene Datei, nur für den ersten Aufbau
let version = '' // mtime beim Laden, das Backend prüft sie beim Speichern
let savedHash = 0
let mount = 0 // neuer key = Excalidraw neu aufbauen (reload)
let timer = 0
let dirty = false

const dark = ref(document.documentElement.dataset.theme !== 'light')
const themeObserver = new MutationObserver(() => {
  dark.value = document.documentElement.dataset.theme !== 'light'
})
themeObserver.observe(document.documentElement, {
  attributes: true,
  attributeFilter: ['data-theme'],
})

function setDirty(d: boolean) {
  if (d !== dirty) emit('dirty', (dirty = d))
}

function onChange(elements: readonly { version: number }[]) {
  if (!lib) return
  // onChange kommt auch bei Auswahl und Scrollen; nur geänderte Elemente zählen.
  setDirty(lib.hashElementsVersion(elements as never) !== savedHash)
  clearTimeout(timer)
  if (dirty) timer = window.setTimeout(() => emit('autosave'), AUTOSAVE_MS)
}

async function paint() {
  if (!host.value) return
  const [{ createElement }, { createRoot }] = await Promise.all([
    import('react'),
    import('react-dom/client'),
  ])
  root ??= createRoot(host.value)
  root.render(
    createElement(lib!.Excalidraw, {
      key: mount,
      initialData: scene as never,
      excalidrawAPI: (a: ExcalidrawImperativeAPI) => (api = a),
      onChange: onChange as never,
      theme: dark.value ? 'dark' : 'light',
      langCode: lang.value === 'de' ? 'de-DE' : 'en',
      // Speichern und Öffnen übernimmt Kairo (⌘S, Baum); Bild-Export bleibt.
      UIOptions: { canvasActions: { loadScene: false, saveToActiveFile: false } },
    }),
  )
}

async function load() {
  clearTimeout(timer)
  try {
    const [l, f] = await Promise.all([loadExcalidraw(), fetchFile(props.path)])
    lib = l
    const text = await f.blob.text()
    scene = text.trim() ? JSON.parse(text) : {}
    version = f.mtime
    savedHash = lib.hashElementsVersion(((scene as { elements?: never[] }).elements ?? []) as never)
    failed.value = ''
    setDirty(false)
    mount++
    await paint()
  } catch (e) {
    failed.value =
      e instanceof ApiError
        ? errorMessage(e)
        : t('Die Datei lässt sich nicht anzeigen ({reason}).', {
            reason: e instanceof Error ? e.message : String(e),
          })
  }
}

async function save(force = false) {
  if (!api || !lib) return
  clearTimeout(timer)
  const elements = api.getSceneElementsIncludingDeleted()
  const json = lib.serializeAsJSON(elements, api.getAppState(), api.getFiles(), 'local')
  version = (
    await replaceFile(props.path, new Blob([json], { type: 'application/json' }), version, force)
  ).mtime
  savedHash = lib.hashElementsVersion(elements)
  // Während des Speicherns weitergezeichnet: bleibt ungespeichert, der nächste Autosave holt es nach.
  onChange(api.getSceneElementsIncludingDeleted())
}

watch([dark, lang], () => void paint())
onMounted(load)
onBeforeUnmount(() => {
  clearTimeout(timer)
  themeObserver.disconnect()
  root?.unmount()
})

defineExpose({ save, reload: load })
</script>

<template>
  <div v-if="failed" class="ed-empty">{{ failed }}</div>
  <div v-show="!failed" ref="host" class="dw" />
</template>

<style scoped>
.dw {
  flex: 1;
  min-height: 0;
  position: relative;
}
.ed-empty {
  flex: 1;
  display: grid;
  place-items: center;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
</style>
