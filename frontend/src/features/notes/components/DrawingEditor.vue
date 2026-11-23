<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import type { Root } from 'react-dom/client'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import type { ExcalidrawImageElement, FileId } from '@excalidraw/excalidraw/element/types'
import { ApiError, errorMessage, fetchFile, replaceFile } from '@/api/client'
import { useShortcuts } from '@/composables/useShortcuts'
import { vDialog } from '@/lib/dialog'
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

// Formel (LaTeX) als SVG-Bild wie beim Obsidian-Plugin; customData.latex hält den Quelltext. Ist genau eine Formel markiert,
// ersetzt der Dialog sie. Die Vorschau im Dialog erspart den Umweg über eine Notiz.
const EX_PX = 10 // MathJax misst in ex; 10 px passen zu Excalidraws 20-px-Text
const texOpen = ref(false)
const texSrc = ref('')
const texOld = shallowRef<ExcalidrawImageElement | null>(null) // shallow: Excalidraw-Elemente nicht in Proxys hüllen
let texToSvg: ((tex: string) => string) | null = null
// MathJax-SVG ohne html/action-Packages (lib/tex.ts), Fehler kommen escaped zurück.
const texPreview = computed(() =>
  texOpen.value && texSrc.value.trim() ? texToSvg!(texSrc.value) : '',
)

async function openTex() {
  if (!api) return
  const selected = api.getAppState().selectedElementIds
  const sel = api.getSceneElements().filter((e) => selected[e.id])
  texOld.value =
    sel.length === 1 && sel[0]!.type === 'image' && sel[0]!.customData?.latex ? sel[0]! : null
  texToSvg ??= (await import('@/lib/tex')).texToSvg
  texSrc.value = texOld.value?.customData?.latex ?? ''
  texOpen.value = true
}

async function insertTex() {
  const src = texSrc.value.trim()
  const old = texOld.value
  texOpen.value = false
  if (!api || !lib || !texToSvg || !src) return
  const svg = texToSvg(src)
  const [width, height] = ['width', 'height'].map(
    (a) => parseFloat(new RegExp(`${a}="([\\d.]+)ex"`).exec(svg)?.[1] ?? '2') * EX_PX,
  ) as [number, number]
  const fileId = `tex-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}` as FileId
  api.addFiles([
    {
      id: fileId,
      mimeType: 'image/svg+xml',
      dataURL: await lib.getDataURL(new Blob([svg], { type: 'image/svg+xml' })),
      created: Date.now(),
    },
  ])
  const all = api.getSceneElementsIncludingDeleted()
  const customData = { ...old?.customData, latex: src }
  let el
  if (old) {
    el = lib.newElementWith(old, { fileId, width, height, customData })
  } else {
    const { scrollX, scrollY, zoom, width: vw, height: vh } = api.getAppState()
    el = lib.convertToExcalidrawElements([
      {
        type: 'image',
        fileId,
        x: vw / 2 / zoom.value - scrollX - width / 2,
        y: vh / 2 / zoom.value - scrollY - height / 2,
        width,
        height,
        customData,
      },
    ])[0]!
  }
  api.updateScene({
    elements: old ? all.map((e) => (e.id === old.id ? el : e)) : [...all, el],
    appState: { selectedElementIds: { [el.id]: true } },
    captureUpdate: lib.CaptureUpdateAction.IMMEDIATELY,
  })
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
      renderTopRightUI: () =>
        createElement(
          'button',
          {
            type: 'button',
            className: 'sidebar-trigger dw-tex',
            title: t('Formel einfügen oder markierte bearbeiten (LaTeX, Taste M)'),
            'aria-label': t('Formel einfügen oder markierte bearbeiten (LaTeX, Taste M)'),
            onClick: () => void openTex(),
          },
          '∑',
        ),
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

// M öffnet den Formel-Dialog. Excalidraws Befehlspalette (⌘/) steckt nicht im npm-Paket, nur auf excalidraw.com;
// die Hilfe nennt sie trotzdem. M belegt Excalidraw nicht.
useShortcuts((e) => {
  if (e.code !== 'KeyM' || !host.value?.offsetParent) return // offsetParent: Fenster gerade versteckt
  if (!(e.target === document.body || host.value.contains(e.target as Node))) return
  e.preventDefault()
  void openTex()
}, true)

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
  <div
    v-if="texOpen"
    v-dialog="() => (texOpen = false)"
    class="overlay open"
    @mousedown.self="texOpen = false"
  >
    <div class="dialog" :aria-label="$t('Formel (LaTeX)')">
      <div class="dlg-head">
        <h3>{{ $t('Formel (LaTeX)') }}</h3>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Schließen')"
          @click="texOpen = false"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <textarea
          v-model="texSrc"
          class="input mono dw-texsrc"
          rows="4"
          spellcheck="false"
          :aria-label="$t('Formel (LaTeX)')"
          placeholder="\frac{a}{b} = \sqrt{x^2 + 1}"
          @keydown.enter.meta.prevent="insertTex"
          @keydown.enter.ctrl.prevent="insertTex"
        />
        <!-- eslint-disable-next-line vue/no-v-html -- MathJax-SVG ohne html/action-Packages, siehe lib/tex.ts -->
        <div class="dw-texprev" :aria-label="$t('Vorschau')" v-html="texPreview" />
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" @click="texOpen = false">
          {{ $t('Abbrechen') }}
        </button>
        <button class="btn btn-primary" type="button" :disabled="!texSrc.trim()" @click="insertTex">
          {{ texOld ? $t('Formel ändern') : $t('Formel einfügen') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dw {
  flex: 1;
  min-height: 0;
  position: relative;
}
/* Aussehen von Excalidraws Bibliotheks-Knopf (sidebar-trigger), nur quadratisch und mit größerem Zeichen. */
.dw :deep(.dw-tex) {
  width: var(--lg-button-size);
  font-size: 16px;
  line-height: 1;
}
.dw-texsrc {
  height: auto;
  padding: 8px 10px;
  resize: vertical;
}
.dw-texprev {
  min-height: 48px;
  display: grid;
  place-items: center;
  overflow-x: auto;
  color: var(--tx-primary);
}
.ed-empty {
  flex: 1;
  display: grid;
  place-items: center;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
</style>
