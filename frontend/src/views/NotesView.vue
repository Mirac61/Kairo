<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute } from 'vue-router'
import { useConfirm } from 'primevue/useconfirm'
import { ApiError, createNote, deleteNote, errorMessage, listNotes, moveNote, readNote, saveNote, uploadFile, type NoteNode } from '@/api/client'
import CodeViewer from '@/components/CodeViewer.vue'
import DrawingEditor from '@/components/DrawingEditor.vue'
import FileEditor from '@/components/FileEditor.vue'
import NoteBacklinks from '@/components/NoteBacklinks.vue'
import NoteEditor from '@/components/NoteEditor.vue'
import NoteTree, { canDrop, canDropHere, parentOf, type TreeCtx } from '@/components/NoteTree.vue'
import { vDialog } from '@/lib/dialog'
import { fileKind, fillTemplate, isObsidianDrawing, join, under, withExt } from '@/lib/noteFiles'
import { store } from '@/lib/storage'
import { t, tn } from '@/lib/i18n'
import { useSplitRatio } from '@/composables/useSplitRatio'

const DISCARD = t('Ungespeicherte Änderungen verwerfen?')
const POLL_MS = 5000

// Zwei Plätze: links („doc“) eine Notiz, ein PDF, ein Bild oder eine Zeichnung, rechts („note“) eine Notiz. Einzeln ist einer sichtbar,
// geteilt beide. Beide bleiben geladen, damit ein Wechsel Scrollposition, Zoom und Ungespeichertes behält.
type Pane = 'doc' | 'note'
const PANES: Pane[] = ['doc', 'note']
const tree = ref<NoteNode[]>([])
const allFiles = computed(() => {
  const out: string[] = []
  const walk = (ns: NoteNode[]) => ns.forEach((n) => (n.dir ? walk(n.children ?? []) : out.push(n.path)))
  walk(tree.value)
  return out
})
// Zeichnungen aus Obsidian (*.excalidraw.md) einmal in Kairo-Zeichnungen umwandeln; das Ergebnis bleibt bis zum Schließen stehen.
const obsidianDrawings = computed(() => allFiles.value.filter(isObsidianDrawing))
const converting = ref(false)
const converted = ref<{ done: string[]; skipped: string[] } | null>(null)
async function convertDrawings() {
  converting.value = true
  try {
    const { convertObsidianDrawings } = await import('@/lib/obsidian')
    converted.value = await convertObsidianDrawings(allFiles.value)
    tree.value = await listNotes()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    converting.value = false
  }
}
const paths = reactive<Record<Pane, string | null>>({ doc: null, note: null })
const shown = ref<Pane>('note') // einzeln: der sichtbare Platz
const split = ref(store.get('kairo-notes-split') === '1')
const focus = ref<Pane>('note') // zuletzt benutzt; dorthin gehen ⌘S, „Datei hinzufügen“ und (geteilt) Notizen aus dem Baum
const visible = (p: Pane) => split.value || shown.value === p
const current = computed<Pane>(() => (split.value ? focus.value : shown.value))
const isMd = (p: Pane) => !!paths[p] && fileKind(paths[p]!) === 'md'

// Notiz je Platz: text im Editor, saved laut Platte, mtime als Version beim Laden (das Backend prüft sie beim Speichern),
// external = auf der Platte geändert, während hier Ungespeichertes liegt.
type Buf = { text: string; saved: string; mtime: string; external: boolean }
const blank = (): Buf => ({ text: '', saved: '', mtime: '', external: false })
const bufs = reactive<Record<Pane, Buf>>({ doc: blank(), note: blank() })
const docKind = computed(() => (paths.doc ? fileKind(paths.doc) : ''))
const docEd = ref<InstanceType<typeof FileEditor> | null>(null)
const setDocEd = (el: unknown) => (docEd.value = el as InstanceType<typeof FileEditor> | null) // Funktions-Ref: im v-for wäre ref="…" ein Array
const docDirtyFlag = ref(false)
const drawingRev = ref(0) // nach jedem Speichern einer Zeichnung und alle 5 s: eingebettete Zeichnungen in Notizen prüfen
watch(() => paths.doc, () => (docDirtyFlag.value = false))
const isDirty = (p: Pane) => (isMd(p) ? bufs[p].text !== bufs[p].saved : p === 'doc' && docDirtyFlag.value)
const dirty = computed(() => isDirty('doc') || isDirty('note'))
const canSave = (p: Pane) => !!paths[p] && !/\.gif$/i.test(paths[p]!) && fileKind(paths[p]!) !== 'code'
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

// Geteilte Ansicht: Breite des linken Platzes als Anteil, per Trenner verschiebbar.
const { ratio, startResize, nudge } = useSplitRatio('kairo-notes-ratio')
function toggleSplit() {
  split.value = !split.value
  if (!split.value) shown.value = paths[focus.value] ? focus.value : paths.doc ? 'doc' : 'note'
}

const crumbsOf = (p: string) => (fileKind(p) === 'code' ? p : p.replace(/\.[^./]+$/, '')).split('/')

function readOpen(): string[] {
  try { return JSON.parse(store.get('kairo-notes-open') ?? '[]') } catch { return [] }
}
const saveOpen = () => store.set('kairo-notes-open', JSON.stringify([...ctx.open]))
const confirmPopup = useConfirm()
let renamingDir = false // Ordner bekommen beim Umbenennen kein .md
const ctx: TreeCtx = reactive({
  isActive: (p: string) => PANES.some((pn) => visible(pn) && paths[pn] === p),
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
  startAdd(parent: string, dir: boolean, drawing = false) {
    if (parent) ctx.open.add(parent)
    ctx.renaming = null
    ctx.adding = { parent, dir, drawing }
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
  drop: (dir: string, dt?: DataTransfer | null) => void drop(dir, dt),
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
  for (const pn of PANES) if (paths[pn]) paths[pn] = re(paths[pn]!)
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
  const keep = renamingDir || fileKind(from) === 'code' // Code zeigt den vollen Namen (auch „Makefile“ ohne Endung)
  await move(from, join(parentOf(from), keep ? name : withExt(name, from.slice(from.lastIndexOf('.')))))
}

async function drop(dir: string, dt?: DataTransfer | null) {
  const from = ctx.dragging
  ctx.dragging = ctx.dropTarget = null
  // Einträge sofort holen: nach dem ersten await ist dataTransfer leer.
  const entries = [...(dt?.items ?? [])].filter((i) => i.kind === 'file').map((i) => i.webkitGetAsEntry()).filter((e) => e !== null)
  if (entries.length) return addFiles(dir, (await Promise.all(entries.map(readEntry))).flat())
  if (!from || !canDrop(from, dir)) return
  if (dir) ctx.open.add(dir)
  await move(from, join(dir, from.slice(from.lastIndexOf('/') + 1)))
}

type Upload = { path: string; file: File } // path relativ zum Zielordner, bei Ordnern mit Unterordnern

// Aus dem Finder gezogene Ordner rekursiv auslesen; readEntries liefert in Häppchen, bis es leer ist.
async function readEntry(e: FileSystemEntry): Promise<Upload[]> {
  if (e.isFile) {
    const file = await new Promise<File>((ok, fail) => (e as FileSystemFileEntry).file(ok, fail))
    return [{ path: e.fullPath.slice(1), file }]
  }
  const reader = (e as FileSystemDirectoryEntry).createReader()
  const out: Upload[] = []
  for (;;) {
    const batch = await new Promise<FileSystemEntry[]>((ok, fail) => reader.readEntries(ok, fail))
    if (!batch.length) return out
    for (const c of batch) out.push(...(await readEntry(c)))
  }
}

const parents = (p: string) => p.split('/').slice(0, -1).map((_, i, a) => a.slice(0, i + 1).join('/'))

// Dateien und Ordner aus dem Finder oder über „Datei/Ordner hinzufügen“: PDFs und Bilder unverändert,
// Markdown als neue Notiz, Unterordner werden angelegt. Versteckte Dateien (.DS_Store) bleiben weg,
// andere Dateitypen werden übersprungen und gemeldet.
async function addFiles(dir: string, uploads: Upload[]) {
  uploads = uploads.filter((u) => !u.path.split('/').some((s) => s.startsWith('.')))
  const bad = uploads.find((u) => !fileKind(u.path))
  if (bad) error.value = t('„{name}“: Kairo nimmt nur Markdown, PDFs, Bilder (png, jpg, gif, webp), C/C++, Makefiles und Zeichnungen.', { name: bad.path })
  uploads = uploads.filter((u) => fileKind(u.path))
  if (!uploads.length) return
  let last = ''
  try {
    for (const d of new Set(uploads.flatMap((u) => parents(u.path).map((d) => join(dir, d))))) {
      await createNote(d, true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e })
      ctx.open.add(d)
    }
    for (const { path, file } of uploads) {
      const p = join(dir, path)
      if (fileKind(p) === 'md') {
        const { mtime } = await createNote(p)
        await saveNote(p, await file.text(), mtime ?? '')
      } else await uploadFile(p, file)
      last = p
    }
    if (!bad) error.value = ''
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

// „Datei/Ordner hinzufügen“ legt in den Ordner der zuletzt benutzten Datei.
const filePick = ref<HTMLInputElement>()
const dirPick = ref<HTMLInputElement>()
function pick(e: Event) {
  const input = e.target as HTMLInputElement
  const uploads = [...(input.files ?? [])].map((file) => ({ path: file.webkitRelativePath || file.name, file }))
  input.value = ''
  const p = paths[current.value] ?? paths.note ?? paths.doc
  void addFiles(p ? parentOf(p) : '', uploads)
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
        for (const pn of PANES) {
          if (!paths[pn] || !under(paths[pn]!, n.path)) continue
          paths[pn] = null
          Object.assign(bufs[pn], blank())
        }
        if (!paths[shown.value]) shown.value = paths.doc ? 'doc' : 'note'
        tree.value = await listNotes()
        error.value = ''
      } catch (e) {
        error.value = errorMessage(e)
      }
    },
  })
}

function show(pane: Pane, n: { content: string; mtime: string }) {
  Object.assign(bufs[pane], { text: n.content, saved: n.content, mtime: n.mtime, external: false })
}

// PDFs und Bilder gehen in den linken Platz, Notizen in den rechten, geteilt in den zuletzt benutzten. Eine schon offene
// Datei wird nur gezeigt; zwei Kopien derselben Notiz würden sich beim Speichern gegenseitig überschreiben.
async function openFile(p: string) {
  const md = fileKind(p) === 'md'
  const pane: Pane = PANES.find((pn) => paths[pn] === p) ?? (!md || (split.value && focus.value === 'doc') ? 'doc' : 'note')
  if (p !== paths[pane]) {
    if (isDirty(pane) && !confirm(DISCARD)) return
    if (md) {
      try {
        show(pane, await readNote(p))
      } catch (e) {
        error.value = errorMessage(e)
        return
      }
    }
    paths[pane] = p
    error.value = ''
  }
  shown.value = focus.value = pane
  if (matchMedia('(max-width:760px)').matches) treeOpen.value = false // Handy: Baum und Notiz nacheinander, nicht nebeneinander
}

// Klick auf eine eingebettete Zeichnung: geteilt öffnen, links zeichnen, rechts die Notiz live.
function openDrawing(p: string) {
  split.value = true
  void openFile(p)
}

// Klick auf einen unaufgelösten [[Link]]: die Notiz neben der aktuellen anlegen (oder öffnen, falls sie inzwischen existiert).
async function createNoteFromLink(name: string) {
  const p = join(parentOf(paths[current.value] ?? ''), withExt(name))
  try {
    await createNote(p)
    tree.value = await listNotes()
  } catch (e) {
    if (!(e instanceof ApiError && e.status === 409)) {
      error.value = errorMessage(e)
      return
    }
  }
  await openFile(p)
}

async function add(name: string) {
  const target = ctx.adding
  ctx.adding = null
  if (!target) return
  if (badName(name)) return
  const p = join(target.parent, target.dir ? name : withExt(name, target.drawing ? '.excalidraw' : '.md'))
  try {
    if (target.drawing) await uploadFile(p, new Blob([EMPTY_DRAWING], { type: 'application/json' }))
    else await createNote(p, target.dir)
    if (target.dir) ctx.toggle(p)
    tree.value = await listNotes()
    if (!target.dir) await openFile(p)
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const EMPTY_DRAWING = JSON.stringify({ type: 'excalidraw', version: 2, source: 'kairo', elements: [], appState: {}, files: {} })

async function uploaded() {
  try {
    tree.value = await listNotes()
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Vorlagen sind Notizen im Ordner „Vorlagen“; eine leere Notiz bietet sie an.
const TEMPLATES = 'Vorlagen'
const templates = computed(() => tree.value.find((n) => n.dir && n.name === TEMPLATES)?.children?.filter((n) => fileKind(n.path) === 'md') ?? [])
const offerTemplates = (pn: Pane) => isMd(pn) && !bufs[pn].text.trim() && templates.value.length > 0 && !under(paths[pn]!, TEMPLATES)

async function useTemplate(t: NoteNode, pn: Pane) {
  const p = paths[pn]
  try {
    const { content } = await readNote(t.path)
    if (p && paths[pn] === p && !bufs[pn].text.trim()) bufs[pn].text = fillTemplate(content, p)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// force überschreibt auch eine extern geänderte Datei (nach Rückfrage im Dialog).
async function save(force = false, pane: Pane = current.value) {
  if (!paths[pane] || saving.value || (!isDirty(pane) && !force)) return
  saving.value = true
  try {
    if (!isMd(pane)) {
      await docEd.value?.save(force)
      if (docKind.value === 'excalidraw') drawingRev.value++
    } else {
      const [p, content] = [paths[pane]!, bufs[pane].text]
      const r = await saveNote(p, content, bufs[pane].mtime, force)
      if (paths[pane] === p) Object.assign(bufs[pane], { saved: content, mtime: r.mtime, external: false })
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
  if (!paths[pane]) return
  if (!isMd(pane)) return docEd.value?.reload()
  try {
    show(pane, await readNote(paths[pane]!))
  } catch (e) {
    error.value = errorMessage(e)
  }
}

// Änderungen aus VSCodium holen: ohne eigene Änderungen still neu laden, sonst nur Hinweis (Speichern fragt dann nach).
async function refresh() {
  if (document.visibilityState !== 'visible' || saving.value) return
  drawingRev.value++ // auch Änderungen aus VSCodium; neu gerendert wird nur bei neuer mtime
  try {
    tree.value = await listNotes()
    for (const pn of PANES) {
      const [p, m] = [paths[pn], bufs[pn].mtime]
      if (!p || !isMd(pn)) continue
      const n = await readNote(p)
      if (n.mtime === m || paths[pn] !== p || bufs[pn].mtime !== m || saving.value) continue // inzwischen gespeichert oder gewechselt
      if (isDirty(pn)) bufs[pn].external = true
      else show(pn, n)
    }
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
watch([() => paths.note, () => paths.doc, shown, split], () => {
  store.set('kairo-notes-file', paths.note ?? '')
  store.set('kairo-notes-doc', paths.doc ?? '')
  store.set('kairo-notes-shown', shown.value)
  store.set('kairo-notes-split', split.value ? '1' : '0')
})

const route = useRoute()
let timer = 0
onMounted(async () => {
  window.addEventListener('keydown', onKey, true)
  window.addEventListener('beforeunload', onUnload)
  window.addEventListener('focus', refresh)
  timer = window.setInterval(refresh, POLL_MS)
  try {
    tree.value = await listNotes()
  } catch (e) {
    error.value = errorMessage(e)
  }
  // Früher stand in kairo-notes-file auch eine PDF; die wandert in den linken Platz.
  const files = new Set(allFiles.value)
  const [lastNote, lastDoc] = [store.get('kairo-notes-file') ?? '', store.get('kairo-notes-doc') ?? '']
  const load = (pn: Pane, p: string) => void readNote(p).then((n) => { if (!paths[pn]) { paths[pn] = p; show(pn, n) } }, () => {})
  const doc = [lastDoc, lastNote].find((p) => p && files.has(p) && (p === lastDoc || fileKind(p) !== 'md'))
  if (doc && fileKind(doc) === 'md') load('doc', doc)
  else if (doc) paths.doc = doc
  shown.value = store.get('kairo-notes-shown') === 'doc' && doc ? 'doc' : 'note'
  focus.value = shown.value
  if (lastNote && lastNote !== doc && fileKind(lastNote) === 'md' && files.has(lastNote)) load('note', lastNote)
  // ?open=Pfad: aus einer Aufgabe verknüpfte Notiz oder ein Treffer der Suche
  openFromRoute()
})
function openFromRoute() {
  const want = route.query.open
  if (typeof want === 'string' && allFiles.value.includes(want)) void openFile(want)
}
watch(() => route.query.open, openFromRoute) // Suche, während die Notizen schon offen sind
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey, true)
  window.removeEventListener('beforeunload', onUnload)
  window.removeEventListener('focus', refresh)
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
          <button type="button" class="icon-btn" :aria-label="$t('Neue Zeichnung')" :data-tip="$t('Neue Zeichnung')" @click="ctx.startAdd('', false, true)"><svg class="ic"><use href="#i-draw" /></svg></button>
          <button type="button" class="icon-btn" :aria-label="$t('Neuer Ordner')" :data-tip="$t('Neuer Ordner')" @click="ctx.startAdd('', true)"><svg class="ic"><use href="#i-proj" /></svg></button>
          <button type="button" class="icon-btn" :aria-label="$t('Datei hinzufügen')" :data-tip="$t('PDF, Bild oder Notiz hinzufügen')" @click="filePick?.click()"><svg class="ic"><use href="#i-upload" /></svg></button>
          <input ref="filePick" type="file" multiple hidden @change="pick" />
          <button type="button" class="icon-btn" :aria-label="$t('Ordner hinzufügen')" :data-tip="$t('Ordner mit Notizen, PDFs und Bildern hinzufügen')" @click="dirPick?.click()"><svg class="ic"><use href="#i-folder-up" /></svg></button>
          <input ref="dirPick" type="file" webkitdirectory hidden @change="pick" />
        </span>
      </div>
      <div
        class="nt-scroll" :class="{ drop: ctx.dropTarget === '' }"
        @dragover="canDropHere($event, ctx.dragging, '') && ($event.preventDefault(), ctx.dropTarget = '')"
        @dragleave.self="ctx.dropTarget = null" @drop.prevent="ctx.drop('', $event.dataTransfer)"
      >
        <NoteTree :nodes="tree" :ctx="ctx" />
        <p v-if="!tree.length && !ctx.adding" class="nt-empty">{{ $t('Noch keine Notizen. Lege oben eine an oder zieh PDFs und Bilder hierher.') }}</p>
      </div>
    </aside>

    <div class="eds" :class="{ split }" :style="split ? { gridTemplateColumns: `minmax(0, ${ratio}fr) minmax(0, ${1 - ratio}fr)` } : undefined">
      <section
        v-for="pn in PANES" v-show="visible(pn)" :key="pn" class="ed" :class="[`ed-${pn}`, { on: split && focus === pn }]"
        @pointerdown.capture="focus = pn" @focusin="focus = pn"
      >
        <header class="ed-head">
          <button
            v-if="pn === (split ? 'doc' : shown)" type="button" class="icon-btn" aria-controls="nt-pane" :aria-expanded="treeOpen"
            :aria-label="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'" :data-tip="treeOpen ? 'Dateibaum einklappen' : 'Dateibaum ausklappen'"
            @click="toggleTree"
          ><svg class="ic"><use :href="treeOpen ? '#i-left' : '#i-right'" /></svg></button>
          <div class="ed-path">
            <template v-if="paths[pn]">
              <span v-for="(c, i) in crumbsOf(paths[pn]!)" :key="i" :class="{ last: i === crumbsOf(paths[pn]!).length - 1 }">{{ c }}</span>
            </template>
            <span v-else class="last">{{ pn === 'doc' ? $t('Keine Datei geöffnet') : $t('Keine Notiz geöffnet') }}</span>
          </div>
          <span v-if="isDirty(pn)" class="ed-dirty" role="status"><i></i>{{ $t('Ungespeichert') }}</span>
          <button v-if="canSave(pn)" type="button" class="btn btn-secondary ed-save" :disabled="!isDirty(pn) || saving" aria-keyshortcuts="Meta+S Control+S" @click="save(false, pn)">
            {{ $t('Speichern') }}<kbd class="key" aria-hidden="true">⌘S</kbd>
          </button>
          <button
            v-if="pn === (split ? 'note' : shown)" type="button" class="icon-btn" :class="{ on: split }" :aria-pressed="split"
            :aria-label="$t('Geteilte Ansicht')" :data-tip="split ? $t('Nur eine Datei zeigen') : $t('Geteilt: zwei Dateien nebeneinander')" @click="toggleSplit"
          ><svg class="ic"><use href="#i-split" /></svg></button>
        </header>
        <div v-if="error && pn === errorPane" class="ed-bar" role="alert">{{ error }}</div>
        <div v-if="pn === errorPane && (obsidianDrawings.length || converted)" class="ed-bar" role="status">
          <template v-if="converted">
            {{ tn(converted.done.length, '{n} Zeichnung umgewandelt, das Original liegt im Papierkorb.', '{n} Zeichnungen umgewandelt, die Originale liegen im Papierkorb.') }}
            <template v-if="converted.skipped.length"> {{ $t('Nicht umgewandelt:') }} {{ converted.skipped.join(' · ') }}</template>
            <button type="button" class="btn btn-ghost" @click="converted = null">{{ $t('Schließen') }}</button>
          </template>
          <template v-else>
            {{ tn(obsidianDrawings.length, '{n} Zeichnung aus Obsidian gefunden.', '{n} Zeichnungen aus Obsidian gefunden.') }}
            <button type="button" class="btn btn-ghost" :disabled="converting" @click="convertDrawings">{{ converting ? $t('Wandle um …') : $t('In Kairo-Zeichnungen umwandeln') }}</button>
          </template>
        </div>

        <template v-if="isMd(pn)">
          <div v-if="bufs[pn].external" class="ed-bar" role="alert">
            {{ $t('Die Datei wurde außerhalb von Kairo geändert. Beim Speichern fragt Kairo nach.') }}
            <button type="button" class="btn btn-ghost" @click="reload(pn)">{{ $t('Verwerfen und neu laden') }}</button>
          </div>
          <div v-if="offerTemplates(pn)" class="ed-tpl">
            <span>{{ $t('Vorlage') }}</span>
            <button v-for="t in templates" :key="t.path" type="button" class="btn btn-ghost" @click="useTemplate(t, pn)">{{ t.name.replace(/\.md$/i, '') }}</button>
          </div>
          <NoteEditor v-model="bufs[pn].text" :path="paths[pn]!" :drawing-rev="drawingRev" :files="allFiles" @uploaded="uploaded" @error="error = $event" @open="openDrawing" @open-note="openFile" @create-note="createNoteFromLink" />
          <NoteBacklinks :path="paths[pn]!" :rev="bufs[pn].mtime" @open="openFile" />
        </template>
        <FileEditor
          v-else-if="pn === 'doc' && paths.doc && (docKind === 'pdf' || docKind === 'image')" :ref="setDocEd" :key="paths.doc" :path="paths.doc" :kind="docKind"
          :active="visible('doc') && (!split || focus === 'doc')" @dirty="docDirtyFlag = $event"
        />
        <DrawingEditor
          v-else-if="pn === 'doc' && paths.doc && docKind === 'excalidraw'" :ref="setDocEd" :key="paths.doc" :path="paths.doc"
          @dirty="docDirtyFlag = $event" @autosave="save(false, 'doc')"
        />
        <CodeViewer v-else-if="pn === 'doc' && paths.doc && docKind === 'code'" :key="paths.doc" :path="paths.doc" />
        <div v-else class="ed-empty">{{ pn === 'doc' ? $t('Klick hier hinein und wähle im Baum eine Notiz, ein PDF oder ein Bild.') : $t('Wähle links eine Notiz oder lege eine neue an.') }}</div>
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
          <p class="ed-msg">{{ $t('„{name}“ wurde seit dem Laden außerhalb von Kairo geändert, zum Beispiel in VSCodium. Überschreiben ersetzt diese Änderungen durch deinen Stand aus Kairo.', { name: paths[conflict] ?? '' }) }}</p>
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
@media (max-width: 760px) {
  .notes { grid-template-columns: minmax(0, 1fr); }
  .notes:not(.shut) .eds { display: none; }
  .nt-pane { border-right: 0; }
  .ed-head { padding-right: 8px; }
}
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
.eds.split .ed.on .ed-head { box-shadow: inset 0 -1px 0 var(--a-blue-hi); } /* geteilt: hierhin öffnet der Baum Notizen */
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
.ed-empty { flex: 1; display: grid; place-items: center; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.ed-msg { font: 400 13px/1.5 var(--font-ui); color: var(--tx-secondary); }

</style>
