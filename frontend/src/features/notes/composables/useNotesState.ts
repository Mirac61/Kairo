import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute } from 'vue-router'
import {
  ApiError,
  createNote,
  errorMessage,
  listNotes,
  readNote,
  saveNote,
  type NoteNode,
} from '@/api/client'
import { parentOf } from '../components/NoteTree.vue'
import { fileKind, fillTemplate, isObsidianDrawing, join, withExt } from '@/lib/noteFiles'
import { store } from '@/lib/storage'
import { t } from '@/lib/i18n'
import { useSplitRatio } from '@/composables/useSplitRatio'
import { useNoteTreeOps } from './useNoteTreeOps'

export type Pane = 'doc' | 'note'
export const PANES: Pane[] = ['doc', 'note']

export type Buf = { text: string; saved: string; mtime: string; external: boolean }
export type Upload = { path: string; file: File }

interface EditorInstance {
  save: (force?: boolean) => Promise<void>
  reload: () => void | Promise<void>
}

const DISCARD = t('Ungespeicherte Änderungen verwerfen?')
const POLL_MS = 5000

export function useNotesState() {
  const tree = ref<NoteNode[]>([])
  const allFiles = computed(() => {
    const out: string[] = []
    const walk = (ns: NoteNode[]) =>
      ns.forEach((n) => (n.dir ? walk(n.children ?? []) : out.push(n.path)))
    walk(tree.value)
    return out
  })

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
  const shown = ref<Pane>('note')
  const split = ref(store.get('kairo-notes-split') === '1')
  const focus = ref<Pane>('note')
  const visible = (p: Pane) => split.value || shown.value === p
  const current = computed<Pane>(() => (split.value ? focus.value : shown.value))
  const isMd = (p: Pane) => !!paths[p] && fileKind(paths[p]!) === 'md'

  const blank = (): Buf => ({ text: '', saved: '', mtime: '', external: false })
  const bufs = reactive<Record<Pane, Buf>>({ doc: blank(), note: blank() })
  const docKind = computed(() => (paths.doc ? fileKind(paths.doc) : ''))
  const docEd = ref<EditorInstance | null>(null)
  const setDocEd = (el: unknown) => (docEd.value = el as EditorInstance | null)
  const docDirtyFlag = ref(false)
  const drawingRev = ref(0)

  watch(
    () => paths.doc,
    () => (docDirtyFlag.value = false),
  )

  const isDirty = (p: Pane) =>
    isMd(p) ? bufs[p].text !== bufs[p].saved : p === 'doc' && docDirtyFlag.value
  const dirty = computed(() => isDirty('doc') || isDirty('note'))

  const conflict = ref<Pane | null>(null)
  const saving = ref(false)
  const error = ref('')
  const errorPane = computed<Pane>(() => (visible('note') ? 'note' : 'doc'))

  const treeOpen = ref(store.get('kairo-notes-tree') !== '0')
  function toggleTree() {
    treeOpen.value = !treeOpen.value
    store.set('kairo-notes-tree', treeOpen.value ? '1' : '0')
  }

  const { ratio, startResize, nudge } = useSplitRatio('kairo-notes-ratio')
  function toggleSplit() {
    split.value = !split.value
    if (!split.value) shown.value = paths[focus.value] ? focus.value : paths.doc ? 'doc' : 'note'
  }

  function show(pane: Pane, n: { content: string; mtime: string }) {
    Object.assign(bufs[pane], {
      text: n.content,
      saved: n.content,
      mtime: n.mtime,
      external: false,
    })
  }

  const headings = reactive<Record<Pane, string>>({ doc: '', note: '' })

  async function openFile(p: string, heading = '') {
    const md = fileKind(p) === 'md'
    const pane: Pane =
      PANES.find((pn) => paths[pn] === p) ??
      (!md || (split.value && focus.value === 'doc') ? 'doc' : 'note')
    if (p !== paths[pane]) {
      headings[pane] = ''
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
    headings[pane] = heading
    if (matchMedia('(max-width:760px)').matches) treeOpen.value = false
  }

  const { ctx, addFiles } = useNoteTreeOps({
    tree,
    paths,
    bufs,
    shown,
    error,
    openFile,
    blank,
    visible,
    panes: PANES,
  })

  function onPickUploads(uploads: Upload[]) {
    const p = paths[current.value] ?? paths.note ?? paths.doc
    void addFiles(p ? parentOf(p) : '', uploads)
  }

  function openDrawing(p: string) {
    split.value = true
    void openFile(p)
  }

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

  async function uploaded() {
    try {
      tree.value = await listNotes()
      error.value = ''
    } catch (e) {
      error.value = errorMessage(e)
    }
  }

  const TEMPLATES = 'Vorlagen'
  const templates = computed(
    () =>
      tree.value
        .find((n) => n.dir && n.name === TEMPLATES)
        ?.children?.filter((n) => fileKind(n.path) === 'md') ?? [],
  )

  async function useTemplate(t: NoteNode, pn: Pane) {
    const p = paths[pn]
    try {
      const { content } = await readNote(t.path)
      if (p && paths[pn] === p && !bufs[pn].text.trim()) bufs[pn].text = fillTemplate(content, p)
    } catch (e) {
      error.value = errorMessage(e)
    }
  }

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
        if (paths[pane] === p)
          Object.assign(bufs[pane], { saved: content, mtime: r.mtime, external: false })
      }
      conflict.value = null
      error.value = ''
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) conflict.value = pane
      else
        error.value =
          e instanceof Error && !(e instanceof ApiError) && !(e instanceof TypeError)
            ? e.message
            : errorMessage(e)
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

  async function refresh() {
    if (document.visibilityState !== 'visible' || saving.value) return
    drawingRev.value++
    try {
      tree.value = await listNotes()
      for (const pn of PANES) {
        const [p, m] = [paths[pn], bufs[pn].mtime]
        if (!p || !isMd(pn)) continue
        const n = await readNote(p)
        if (n.mtime === m || paths[pn] !== p || bufs[pn].mtime !== m || saving.value) continue
        if (isDirty(pn)) bufs[pn].external = true
        else show(pn, n)
      }
    } catch {
      /* Backend kurz weg oder Datei gelöscht */
    }
  }

  function onKey(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 's') return
    e.preventDefault()
    void save()
  }

  function onUnload(e: BeforeUnloadEvent) {
    if (dirty.value) e.preventDefault()
  }

  onBeforeRouteLeave(() => !dirty.value || confirm(DISCARD))

  watch([() => paths.note, () => paths.doc, shown, split], () => {
    store.set('kairo-notes-file', paths.note ?? '')
    store.set('kairo-notes-doc', paths.doc ?? '')
    store.set('kairo-notes-shown', shown.value)
    store.set('kairo-notes-split', split.value ? '1' : '0')
  })

  const route = useRoute()
  let timer = 0

  function openFromRoute() {
    const want = route.query.open
    if (typeof want === 'string' && allFiles.value.includes(want)) void openFile(want)
  }

  watch(() => route.query.open, openFromRoute)

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
    const files = new Set(allFiles.value)
    const [lastNote, lastDoc] = [
      store.get('kairo-notes-file') ?? '',
      store.get('kairo-notes-doc') ?? '',
    ]
    const load = (pn: Pane, p: string) =>
      void readNote(p).then(
        (n) => {
          if (!paths[pn]) {
            paths[pn] = p
            show(pn, n)
          }
        },
        () => {},
      )
    const doc = [lastDoc, lastNote].find(
      (p) => p && files.has(p) && (p === lastDoc || fileKind(p) !== 'md'),
    )
    if (doc && fileKind(doc) === 'md') load('doc', doc)
    else if (doc) paths.doc = doc
    shown.value = store.get('kairo-notes-shown') === 'doc' && doc ? 'doc' : 'note'
    focus.value = shown.value
    if (lastNote && lastNote !== doc && fileKind(lastNote) === 'md' && files.has(lastNote))
      load('note', lastNote)
    openFromRoute()
  })

  onBeforeUnmount(() => {
    window.removeEventListener('keydown', onKey, true)
    window.removeEventListener('beforeunload', onUnload)
    window.removeEventListener('focus', refresh)
    clearInterval(timer)
  })

  return {
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
    current,
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
  }
}
