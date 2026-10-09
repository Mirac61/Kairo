<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useConfirm } from 'primevue/useconfirm'
import { MdEditor, type ToolbarNames } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { ApiError, createNote, deleteNote, errorMessage, listNotes, moveNote, readNote, saveNote, uploadFile, type NoteNode } from '@/api/client'
import FileEditor from '@/components/FileEditor.vue'
import NoteTree, { canDrop, canDropHere, parentOf, type TreeCtx } from '@/components/NoteTree.vue'
import { ymd } from '@/lib/dates'
import { vDialog } from '@/lib/dialog'
import { fileKind, fillTemplate, join, previewImages, under, withExt } from '@/lib/noteFiles'
import { store } from '@/lib/storage'
import { t } from '@/lib/i18n'

// Mermaid, KaTeX, Prettier und Highlight lädt md-editor-v3 vom CDN; sie sind abgeschaltet (Kairo läuft lokal).
// Bilder landen über uploadImages neben der Notiz.
const TOOLBAR: ToolbarNames[] = [
  'bold', 'italic', 'strikeThrough', 'title', '-', 'quote', 'unorderedList', 'orderedList', 'task', '-',
  'codeRow', 'code', 'link', 'image', 'table', '-', 'revoke', 'next', '=', 'preview', 'previewOnly',
]
const DISCARD = t('Ungespeicherte Änderungen verwerfen?')
const POLL_MS = 5000

// Zwei Plätze: eine Notiz (Markdown) und ein Dokument (PDF oder Bild). Einzeln ist einer sichtbar, geteilt beide
// (Dokument links, Notiz rechts). Beide bleiben geladen, damit ein Wechsel Scrollposition, Zoom und Ungespeichertes behält.
type Pane = 'doc' | 'note'
const PANES: Pane[] = ['doc', 'note']
const tree = ref<NoteNode[]>([])
const notePath = ref<string | null>(null)
const docPath = ref<string | null>(null)
const shown = ref<Pane>('note') // einzeln: der sichtbare Platz
const split = ref(store.get('kairo-notes-split') === '1')
const focus = ref<Pane>('note') // zuletzt benutzt; dorthin gehen ⌘S und „Datei hinzufügen“
const visible = (p: Pane) => split.value || shown.value === p
const current = computed<Pane>(() => (split.value ? focus.value : shown.value))
const pathOf = (p: Pane) => (p === 'doc' ? docPath.value : notePath.value)

const text = ref('') // Inhalt im Editor
const saved = ref('') // Inhalt laut Platte (beim Laden oder letzten Speichern)
const mtime = ref('') // Version beim Laden; das Backend prüft sie beim Speichern
const external = ref(false) // auf der Platte geändert, während hier Ungespeichertes liegt
const docKind = computed(() => (docPath.value ? fileKind(docPath.value) : ''))
const docEd = ref<InstanceType<typeof FileEditor> | null>(null)
const setDocEd = (el: unknown) => (docEd.value = el as InstanceType<typeof FileEditor> | null) // Funktions-Ref: im v-for wäre ref="…" ein Array
const docDirtyFlag = ref(false)
watch(docPath, () => (docDirtyFlag.value = false))
const isDirty = (p: Pane) => (p === 'doc' ? docDirtyFlag.value : notePath.value !== null && text.value !== saved.value)
const dirty = computed(() => isDirty('doc') || isDirty('note'))
const canSave = (p: Pane) => (p === 'doc' ? !!docPath.value && !/\.gif$/i.test(docPath.value) : notePath.value !== null)
const conflict = ref<Pane | null>(null)
const saving = ref(false)
const error = ref('')
const errorPane = computed<Pane>(() => (visible('note') ? 'note' : 'doc')) // Fehler stehen im rechten bzw. einzigen Platz

// Der Dateibaum lässt sich einklappen, damit die Notiz die volle Breite bekommt.
const treeOpen = ref(store.get('kairo-notes-tree') !== '0')
function toggleTree() {
  treeOpen.value = !treeOpen.value
  store.set('kairo-notes-tree', treeOpen.value ? '1' : '0')
}

// Geteilte Ansicht: Breite des Dokuments als Anteil, per Trenner verschiebbar.
const ratio = ref(Math.min(0.75, Math.max(0.25, Number(store.get('kairo-notes-ratio')) || 0.5)))
function toggleSplit() {
  split.value = !split.value
  if (!split.value && !pathOf(shown.value)) shown.value = docPath.value ? 'doc' : 'note'
}
function startResize(e: PointerEvent) {
  e.preventDefault()
  const box = (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect()
  const move = (ev: PointerEvent) => (ratio.value = Math.min(0.75, Math.max(0.25, (ev.clientX - box.left) / box.width)))
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    store.set('kairo-notes-ratio', String(ratio.value))
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}
function nudge(d: number) {
  ratio.value = Math.min(0.75, Math.max(0.25, ratio.value + d))
  store.set('kairo-notes-ratio', String(ratio.value))
}

const crumbsOf = (p: string) => p.replace(/\.[^./]+$/, '').split('/')

function readOpen(): string[] {
  try { return JSON.parse(store.get('kairo-notes-open') ?? '[]') } catch { return [] }
}
const saveOpen = () => store.set('kairo-notes-open', JSON.stringify([...ctx.open]))
const confirmPopup = useConfirm()
let renamingDir = false // Ordner bekommen beim Umbenennen kein .md
const ctx: TreeCtx = reactive({
  isActive: (p: string) => (visible('note') && p === notePath.value) || (visible('doc') && p === docPath.value),
  open: new Set(readOpen()),
  adding: null as TreeCtx['adding'],
  renaming: null as string | null,
  dragging: null as string | null,
  dropTarget: null as string | null,
  select: (p: string) => void openFile(p),
  toggle(p: string) {
    if (!ctx.open.delete(p)) ctx.open.add(p)
    saveOpen()
  },
  startAdd(parent: string, dir: boolean) {
    if (parent) ctx.open.add(parent)
    ctx.renaming = null
    ctx.adding = { parent, dir }
  },
  submitAdd: (name: string) => void add(name),
  cancelAdd() { ctx.adding = null },
  startRename(p: string, dir: boolean) {
    ctx.adding = null
    ctx.renaming = p
    renamingDir = dir
  },
  submitRename: (name: string) => void rename(name),
  cancelRename() { ctx.renaming = null },
  remove: (n: NoteNode, anchor: HTMLElement) => remove(n, anchor),
  drop: (dir: string, files?: FileList) => void drop(dir, files),
})

// Pfade baut der Baum selbst; ein Name ist nur ein Teil (der Browser würde „..“ in der URL sonst auflösen).
function badName(name: string) {
  if (!/[/\\]/.test(name) && !name.startsWith('.')) return false
  error.value = t('Ein Name darf kein „/“ enthalten und nicht mit „.“ beginnen.')
  return true
}

// Nach Umbenennen oder Verschieben: offene Dateien und aufgeklappte Ordner mitnehmen.
function remap(from: string, to: string) {
  const re = (p: string) => (under(p, from) ? to + p.slice(from.length) : p)
  if (notePath.value) notePath.value = re(notePath.value)
  if (docPath.value) docPath.value = re(docPath.value)
  ctx.open = new Set([...ctx.open].map(re))
  saveOpen()
}

async function move(from: string, to: string) {
  if (from === to) return
  try {
    await moveNote(from, to)
    remap(from, to)
    tree.value = await listNotes()
    error.value = ''
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 409 ? t('„{name}“ existiert schon.', { name: to }) : errorMessage(e)
  }
}

async function rename(name: string) {
  const from = ctx.renaming
  ctx.renaming = null
  if (!from || badName(name)) return
  await move(from, join(parentOf(from), renamingDir ? name : withExt(name, from.slice(from.lastIndexOf('.')))))
}

async function drop(dir: string, files?: FileList) {
  const from = ctx.dragging
  ctx.dragging = ctx.dropTarget = null
  if (files?.length) return addFiles(dir, [...files])
  if (!from || !canDrop(from, dir)) return
  if (dir) ctx.open.add(dir)
  await move(from, join(dir, from.slice(from.lastIndexOf('/') + 1)))
}

// Dateien aus dem Finder oder über „Datei hinzufügen“: PDFs und Bilder unverändert, Markdown als neue Notiz.
async function addFiles(dir: string, files: File[]) {
  const bad = files.find((f) => !fileKind(f.name))
  if (bad) {
    error.value = t('„{name}“: Kairo nimmt nur Markdown, PDFs und Bilder (png, jpg, gif, webp).', { name: bad.name })
    return
  }
  let last = ''
  try {
    for (const f of files) {
      const p = join(dir, f.name)
      if (fileKind(p) === 'md') {
        const { mtime } = await createNote(p)
        await saveNote(p, await f.text(), mtime ?? '')
      } else await uploadFile(p, f)
      last = p
    }
    error.value = ''
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 409 ? t('Eine Datei mit diesem Namen gibt es dort schon.') : errorMessage(e)
  }
  if (dir) ctx.open.add(dir)
  saveOpen()
  try {
    tree.value = await listNotes()
  } catch { /* nächste Runde von refresh */ }
  if (last) await openFile(last)
}

// „Datei hinzufügen“ legt in den Ordner der zuletzt benutzten Datei.
const filePick = ref<HTMLInputElement>()
function pick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = [...(input.files ?? [])]
  input.value = ''
  const p = pathOf(current.value) ?? notePath.value ?? docPath.value
  void addFiles(p ? parentOf(p) : '', files)
}

// Bilder in der Vorschau öffnen sich per Klick in voller Auflösung in einem neuen Tab.
function openPreviewImage(e: MouseEvent) {
  const img = (e.target as HTMLElement).closest<HTMLImageElement>('.md-editor-preview img')
  if (img) window.open(img.src, '_blank')
}

function remove(n: NoteNode, anchor: HTMLElement) {
  confirmPopup.require({
    target: anchor,
    message: n.dir
      ? t('Ordner „{name}“ mit allem Inhalt löschen? Er landet in .trash im Notizordner.', { name: n.name })
      : t('„{name}“ löschen? Die Notiz landet in .trash im Notizordner.', { name: n.name.replace(/\.md$/i, '') }),
    acceptLabel: t('Löschen'),
    rejectLabel: t('Abbrechen'),
    defaultFocus: 'reject',
    acceptProps: { size: 'small' },
    rejectProps: { severity: 'secondary', size: 'small', text: true },
    accept: async () => {
      try {
        await deleteNote(n.path)
        if (notePath.value && under(notePath.value, n.path)) {
          notePath.value = null
          text.value = saved.value = ''
        }
        if (docPath.value && under(docPath.value, n.path)) docPath.value = null
        if (!pathOf(shown.value)) shown.value = docPath.value ? 'doc' : 'note'
        tree.value = await listNotes()
        error.value = ''
      } catch (e) {
        error.value = errorMessage(e)
      }
    },
  })
}

function show(n: { content: string; mtime: string }) {
  text.value = saved.value = n.content
  mtime.value = n.mtime
  external.value = false
}

// Notizen gehen in den Notiz-Platz, PDFs und Bilder in den Dokument-Platz; der andere Platz bleibt, wie er ist.
async function openFile(p: string) {
  const pane: Pane = fileKind(p) === 'md' ? 'note' : 'doc'
  if (p !== pathOf(pane)) {
    if (isDirty(pane) && !confirm(DISCARD)) return
    if (pane === 'note') {
      try {
        show(await readNote(p))
      } catch (e) {
        error.value = errorMessage(e)
        return
      }
      notePath.value = p
    } else docPath.value = p
    error.value = ''
  }
  shown.value = focus.value = pane
}

async function add(name: string) {
  const target = ctx.adding
  ctx.adding = null
  if (!target) return
  if (badName(name)) return
  const p = join(target.parent, target.dir ? name : withExt(name))
  try {
    await createNote(p, target.dir)
    if (target.dir) ctx.toggle(p)
    tree.value = await listNotes()
    if (!target.dir) await openFile(p)
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Eingefügte Bilder (⌘V, Toolbar) landen in assets/ neben der Notiz; der Link ist relativ, damit VSCodium sie genauso zeigt.
const ASSETS = 'assets'
async function uploadImages(files: File[], done: (urls: string[]) => void) {
  const p = notePath.value
  if (!p) return
  const d = new Date()
  const stamp = `${ymd(d)}-${[d.getHours(), d.getMinutes(), d.getSeconds()].map((n) => String(n).padStart(2, '0')).join('')}`
  const base = p.slice(p.lastIndexOf('/') + 1).replace(/\.md$/i, '')
  const dir = join(parentOf(p), ASSETS)
  try {
    await createNote(dir, true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e }) // 409: gibt es schon
    const names: string[] = []
    for (const [i, f] of files.entries()) {
      const name = `${base}-${stamp}${files.length > 1 ? `-${i + 1}` : ''}.${f.type === 'image/jpeg' ? 'jpg' : f.type.split('/')[1]}`
      await uploadFile(join(dir, name), f)
      names.push(`${ASSETS}/${encodeURI(name)}`)
    }
    done(names)
    tree.value = await listNotes()
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const sanitize = (html: string) => (notePath.value ? previewImages(html, notePath.value) : html)

// Vorlagen sind Notizen im Ordner „Vorlagen“; eine leere Notiz bietet sie an.
const TEMPLATES = 'Vorlagen'
const templates = computed(() => tree.value.find((n) => n.dir && n.name === TEMPLATES)?.children?.filter((n) => fileKind(n.path) === 'md') ?? [])
const offerTemplates = computed(() => !!notePath.value && !text.value.trim() && templates.value.length > 0 && !under(notePath.value, TEMPLATES))

async function useTemplate(t: NoteNode) {
  const p = notePath.value
  try {
    const { content } = await readNote(t.path)
    if (p && notePath.value === p && !text.value.trim()) text.value = fillTemplate(content, p)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// force überschreibt auch eine extern geänderte Datei (nach Rückfrage im Dialog).
async function save(force = false, pane: Pane = current.value) {
  if (!pathOf(pane) || saving.value || (!isDirty(pane) && !force)) return
  saving.value = true
  try {
    if (pane === 'doc') await docEd.value?.save(force)
    else {
      const [p, content] = [notePath.value!, text.value]
      const r = await saveNote(p, content, mtime.value, force)
      if (notePath.value === p) {
        saved.value = content
        mtime.value = r.mtime
        external.value = false
      }
    }
    conflict.value = null
    error.value = ''
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) conflict.value = pane
    // PDFs und Bilder: eigene Fehlermeldungen des Editors (z. B. verschlüsselte PDF) durchreichen
    else error.value = e instanceof Error && !(e instanceof ApiError) && !(e instanceof TypeError) ? e.message : errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function reload(pane: Pane) {
  conflict.value = null
  if (pane === 'doc') return docEd.value?.reload()
  if (!notePath.value) return
  try {
    show(await readNote(notePath.value))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Änderungen aus VSCodium holen: ohne eigene Änderungen still neu laden, sonst nur Hinweis (Speichern fragt dann nach).
async function refresh() {
  if (document.visibilityState !== 'visible' || saving.value) return
  try {
    tree.value = await listNotes()
    const [p, m] = [notePath.value, mtime.value]
    if (!p) return
    const n = await readNote(p)
    if (n.mtime === m || notePath.value !== p || mtime.value !== m || saving.value) return // inzwischen gespeichert oder gewechselt
    if (isDirty('note')) external.value = true
    else show(n)
  } catch { /* Backend kurz weg oder Datei gelöscht: nächste Runde */ }
}

function onKey(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 's') return
  e.preventDefault() // sonst öffnet der Browser „Seite speichern“
  void save()
}
function onUnload(e: BeforeUnloadEvent) {
  if (dirty.value) e.preventDefault()
}
onBeforeRouteLeave(() => !dirty.value || confirm(DISCARD))

// Stand für den nächsten Besuch: offene Dateien, sichtbarer Platz, geteilt oder nicht.
watch([notePath, docPath, shown, split], () => {
  store.set('kairo-notes-file', notePath.value ?? '')
  store.set('kairo-notes-doc', docPath.value ?? '')
  store.set('kairo-notes-shown', shown.value)
  store.set('kairo-notes-split', split.value ? '1' : '0')
})

// md-editor-v3 bekommt das Theme als Prop; Kairo schaltet es über data-theme an <html>.
const dark = ref(document.documentElement.dataset.theme !== 'light')
const themeObserver = new MutationObserver(() => { dark.value = document.documentElement.dataset.theme !== 'light' })

let timer = 0
onMounted(async () => {
  window.addEventListener('keydown', onKey, true)
  window.addEventListener('beforeunload', onUnload)
  window.addEventListener('focus', refresh)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  timer = window.setInterval(refresh, POLL_MS)
  try {
    tree.value = await listNotes()
  } catch (e) {
    error.value = errorMessage(e)
  }
  // Früher stand in kairo-notes-file auch eine PDF; die wandert in den Dokument-Platz.
  const files = new Set<string>()
  const walk = (ns: NoteNode[]) => ns.forEach((n) => (n.dir ? walk(n.children ?? []) : files.add(n.path)))
  walk(tree.value)
  const [lastNote, lastDoc] = [store.get('kairo-notes-file') ?? '', store.get('kairo-notes-doc') ?? '']
  const doc = [lastDoc, lastNote].find((p) => p && fileKind(p) !== 'md' && files.has(p))
  if (doc) docPath.value = doc
  shown.value = store.get('kairo-notes-shown') === 'doc' && doc ? 'doc' : 'note'
  focus.value = shown.value
  if (lastNote && fileKind(lastNote) === 'md' && files.has(lastNote)) {
    void readNote(lastNote).then((n) => { if (!notePath.value) { notePath.value = lastNote; show(n) } }, () => {})
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey, true)
  window.removeEventListener('beforeunload', onUnload)
  window.removeEventListener('focus', refresh)
  themeObserver.disconnect()
  clearInterval(timer)
})
</script>

<template>
  <div class="notes" :class="{ shut: !treeOpen }">
    <aside v-show="treeOpen" id="nt-pane" class="nt-pane">
      <div class="nt-head">
        <span class="lbl">{{ $t('Notizen') }}</span>
        <span class="nt-head-act">
          <button type="button" class="icon-btn" :aria-label="$t('Neue Notiz')" :data-tip="$t('Neue Notiz')" @click="ctx.startAdd('', false)"><svg class="ic"><use href="#i-plus" /></svg></button>
          <button type="button" class="icon-btn" :aria-label="$t('Neuer Ordner')" :data-tip="$t('Neuer Ordner')" @click="ctx.startAdd('', true)"><svg class="ic"><use href="#i-proj" /></svg></button>
          <button type="button" class="icon-btn" :aria-label="$t('Datei hinzufügen')" :data-tip="$t('PDF, Bild oder Notiz hinzufügen')" @click="filePick?.click()"><svg class="ic"><use href="#i-upload" /></svg></button>
          <input ref="filePick" type="file" multiple hidden accept=".md,.pdf,.png,.jpg,.jpeg,.gif,.webp" @change="pick" />
        </span>
      </div>
      <div
        class="nt-scroll" :class="{ drop: ctx.dropTarget === '' }"
        @dragover="canDropHere($event, ctx.dragging, '') && ($event.preventDefault(), ctx.dropTarget = '')"
        @dragleave.self="ctx.dropTarget = null" @drop.prevent="ctx.drop('', $event.dataTransfer?.files)"
      >
        <NoteTree :nodes="tree" :ctx="ctx" />
        <p v-if="!tree.length && !ctx.adding" class="nt-empty">{{ $t('Noch keine Notizen. Lege oben eine an oder zieh PDFs und Bilder hierher.') }}</p>
      </div>
    </aside>

    <div class="eds" :class="{ split }" :style="split ? { gridTemplateColumns: `minmax(0, ${ratio}fr) minmax(0, ${1 - ratio}fr)` } : undefined">
      <section
        v-for="pn in PANES" v-show="visible(pn)" :key="pn" class="ed" :class="`ed-${pn}`"
        @pointerdown.capture="focus = pn" @focusin="focus = pn" @click="openPreviewImage"
      >
        <header class="ed-head">
          <button
            v-if="pn === (split ? 'doc' : shown)" type="button" class="icon-btn" aria-controls="nt-pane" :aria-expanded="treeOpen"
            :aria-label="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'" :data-tip="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'"
            @click="toggleTree"
          ><svg class="ic"><use :href="treeOpen ? '#i-left' : '#i-right'" /></svg></button>
          <div class="ed-path">
            <template v-if="pathOf(pn)">
              <span v-for="(c, i) in crumbsOf(pathOf(pn)!)" :key="i" :class="{ last: i === crumbsOf(pathOf(pn)!).length - 1 }">{{ c }}</span>
            </template>
            <span v-else class="last">{{ pn === 'doc' ? $t('Kein PDF geöffnet') : $t('Keine Notiz geöffnet') }}</span>
          </div>
          <span v-if="isDirty(pn)" class="ed-dirty" role="status"><i></i>{{ $t('Ungespeichert') }}</span>
          <button v-if="canSave(pn)" type="button" class="btn btn-secondary ed-save" :disabled="!isDirty(pn) || saving" aria-keyshortcuts="Meta+S Control+S" @click="save(false, pn)">
            {{ $t('Speichern') }}<kbd class="key" aria-hidden="true">⌘S</kbd>
          </button>
          <button
            v-if="pn === (split ? 'note' : shown)" type="button" class="icon-btn" :class="{ on: split }" :aria-pressed="split"
            :aria-label="$t('Geteilte Ansicht')" :data-tip="split ? $t('Nur eine Datei zeigen') : $t('Geteilt: PDF links, Notiz rechts')" @click="toggleSplit"
          ><svg class="ic"><use href="#i-split" /></svg></button>
        </header>
        <div v-if="error && pn === errorPane" class="ed-bar" role="alert">{{ error }}</div>

        <template v-if="pn === 'note'">
          <div v-if="external" class="ed-bar" role="alert">
            {{ $t('Die Datei wurde außerhalb von Kairo geändert. Beim Speichern fragt Kairo nach.') }}
            <button type="button" class="btn btn-ghost" @click="reload('note')">{{ $t('Verwerfen und neu laden') }}</button>
          </div>
          <div v-if="offerTemplates" class="ed-tpl">
            <span>{{ $t('Vorlage') }}</span>
            <button v-for="t in templates" :key="t.path" type="button" class="btn btn-ghost" @click="useTemplate(t)">{{ t.name.replace(/\.md$/i, '') }}</button>
          </div>
          <MdEditor
            v-if="notePath" v-model="text" class="ed-md" :theme="dark ? 'dark' : 'light'" language="en-US" preview-theme="default"
            :toolbars="TOOLBAR" :footers="[]" :sanitize="sanitize" no-highlight no-mermaid no-katex no-prettier no-echarts
            @on-upload-img="uploadImages"
          />
          <div v-else class="ed-empty">{{ $t('Wähle links eine Notiz oder lege eine neue an.') }}</div>
        </template>
        <template v-else>
          <FileEditor
            v-if="docPath && (docKind === 'pdf' || docKind === 'image')" :ref="setDocEd" :key="docPath" :path="docPath" :kind="docKind"
            :active="visible('doc') && (!split || focus === 'doc')" @dirty="docDirtyFlag = $event"
          />
          <div v-else class="ed-empty">{{ $t('Wähle im Baum ein PDF oder ein Bild.') }}</div>
        </template>
      </section>
      <div
        v-if="split" class="ed-split" role="separator" aria-orientation="vertical" :aria-label="$t('Breite der Hälften')" tabindex="0"
        :aria-valuenow="Math.round(ratio * 100)" aria-valuemin="25" aria-valuemax="75" :style="{ left: `${ratio * 100}%` }"
        @pointerdown="startResize" @keydown.left.prevent="nudge(-0.05)" @keydown.right.prevent="nudge(0.05)"
      />
    </div>

    <div v-if="conflict" v-dialog="() => (conflict = null)" class="overlay open" @mousedown.self="conflict = null">
      <div class="dialog" :aria-label="$t('Datei extern geändert')">
        <div class="dlg-head">
          <h3>{{ $t('Datei wurde extern geändert') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="conflict = null"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <p class="ed-msg">{{ $t('„{name}“ wurde seit dem Laden außerhalb von Kairo geändert, zum Beispiel in VSCodium. Überschreiben ersetzt diese Änderungen durch deinen Stand aus Kairo.', { name: pathOf(conflict) ?? '' }) }}</p>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" @click="conflict = null">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-secondary" type="button" @click="reload(conflict)">{{ $t('Verwerfen und neu laden') }}</button>
          <button class="btn btn-primary" type="button" @click="save(true, conflict)">{{ $t('Überschreiben') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.notes { display: grid; grid-template-columns: 260px minmax(0, 1fr); height: 100%; }
.notes.shut { grid-template-columns: minmax(0, 1fr); }
.nt-pane { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--br-subtle); background: var(--bg-1); }
.nt-head { display: flex; align-items: center; justify-content: space-between; height: 48px; padding: 0 8px 0 16px; border-bottom: 1px solid var(--br-subtle); }
.nt-head-act { display: flex; gap: 2px; }
.nt-scroll { flex: 1; overflow-y: auto; padding: 8px; }
.nt-scroll.drop { box-shadow: inset 0 0 0 1px var(--a-blue-hi); }
.nt-empty { padding: 8px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }

.eds { position: relative; display: grid; grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); min-width: 0; min-height: 0; }
.ed { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.eds.split .ed-doc { border-right: 1px solid var(--br-subtle); }
/* Trenner: schmale Griffzone über der Grenze, beim Hover/Ziehen als Linie sichtbar */
.ed-split { position: absolute; top: 0; bottom: 0; z-index: 2; width: 9px; margin-left: -4px; cursor: col-resize; }
.ed-split::after { content: ''; position: absolute; top: 0; bottom: 0; left: 4px; width: 1px; transition: background var(--dur) ease-out; }
.ed-split:hover::after, .ed-split:active::after, .ed-split:focus-visible::after { background: var(--a-blue-hi); }
.ed-split:focus-visible { outline: none; }
.ed-head .icon-btn.on { background: var(--bg-selected); color: var(--a-blue-hi); }
.ed-head { display: flex; align-items: center; gap: 12px; height: 48px; padding: 0 16px 0 12px; border-bottom: 1px solid var(--br-subtle); }
.ed-path { flex: 1; min-width: 0; display: flex; align-items: center; overflow: hidden; white-space: nowrap; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.ed-path span + span::before { content: '/'; margin: 0 6px; color: var(--tx-disabled); }
.ed-path .last { color: var(--tx-primary); font-weight: 500; overflow: hidden; text-overflow: ellipsis; }
.ed-dirty { display: inline-flex; align-items: center; gap: 6px; font: 400 12px/1 var(--font-ui); color: var(--tx-secondary); }
.ed-dirty i { width: 6px; height: 6px; border-radius: 50%; background: var(--a-blue-hi); }
.ed-save { height: 30px; padding: 0 8px 0 12px; }
.ed-save .key { padding: 2px 5px; font-size: 11px; }
.ed-bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 16px 6px 24px; border-bottom: 1px solid var(--br-subtle); font: 400 13px/1.4 var(--font-ui); color: var(--a-red); }
.ed-bar .btn { height: 28px; }
.ed-tpl { display: flex; align-items: center; gap: 4px; padding: 6px 16px 6px 24px; border-bottom: 1px solid var(--br-subtle); font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.ed-tpl span { margin-right: 4px; }
.ed-tpl .btn { height: 28px; }
.ed :deep(.md-editor-preview img) { cursor: zoom-in; }
.ed-empty { flex: 1; display: grid; place-items: center; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.ed-msg { font: 400 13px/1.5 var(--font-ui); color: var(--tx-secondary); }

/* md-editor-v3 an die Kairo-Tokens anpassen (gilt für Dark und Light, weil die Tokens mitwechseln). */
.ed :deep(.md-editor) {
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
.ed :deep(.md-editor .md-editor-toolbar-wrapper) { padding: 4px 16px; border-bottom-color: var(--br-subtle); }
.ed :deep(.md-editor .md-editor-toolbar-item) { color: var(--tx-secondary); }
.ed :deep(.md-editor .cm-editor) { font-family: var(--font-mono); font-size: 13px; }
.ed :deep(.md-editor .cm-scroller) { padding: 0 8px; }
.ed :deep(.md-editor .md-editor-preview) {
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
