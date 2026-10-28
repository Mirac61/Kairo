import { reactive, type Ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import {
  ApiError,
  createNote,
  deleteNote,
  errorMessage,
  listNotes,
  moveNote,
  saveNote,
  uploadFile,
  type NoteNode,
} from '@/api/client'
import { canDrop, parentOf, type TreeCtx } from '../components/NoteTree.vue'
import { fileKind, join, under, withExt } from '@/lib/noteFiles'
import { store } from '@/lib/storage'
import { t } from '@/lib/i18n'
import type { Buf, Pane, Upload } from './useNotesState'

const EMPTY_DRAWING = JSON.stringify({
  type: 'excalidraw',
  version: 2,
  source: 'kairo',
  elements: [],
  appState: {},
  files: {},
})

interface TreeOpsOptions {
  tree: Ref<NoteNode[]>
  paths: Record<Pane, string | null>
  bufs: Record<Pane, Buf>
  shown: Ref<Pane>
  error: Ref<string>
  openFile: (p: string) => Promise<void>
  blank: () => Buf
  visible: (p: Pane) => boolean
  panes: Pane[]
}

export function useNoteTreeOps(opts: TreeOpsOptions) {
  const { tree, paths, bufs, shown, error, openFile, blank, visible, panes } = opts

  function readOpen(): string[] {
    try {
      return JSON.parse(store.get('kairo-notes-open') ?? '[]')
    } catch {
      return []
    }
  }
  const saveOpen = () => store.set('kairo-notes-open', JSON.stringify([...ctx.open]))
  const confirmPopup = useConfirm()
  let renamingDir = false

  const ctx: TreeCtx = reactive({
    isActive: (p: string) => panes.some((pn) => visible(pn) && paths[pn] === p),
    open: new Set(readOpen()),
    adding: null as TreeCtx['adding'],
    renaming: null as string | null,
    dragging: null as string | null,
    dropTarget: null as string | null,
    setDragging(p: string | null) {
      ctx.dragging = p
    },
    setDropTarget(p: string | null) {
      ctx.dropTarget = p
    },
    clearDrag() {
      ctx.dragging = ctx.dropTarget = null
    },
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
    cancelAdd() {
      ctx.adding = null
    },
    startRename(p: string, dir: boolean) {
      ctx.adding = null
      ctx.renaming = p
      renamingDir = dir
    },
    submitRename: (name: string) => void rename(name),
    cancelRename() {
      ctx.renaming = null
    },
    remove: (n: NoteNode, anchor: HTMLElement) => remove(n, anchor),
    drop: (dir: string, dt?: DataTransfer | null) => void drop(dir, dt),
  })

  function badName(name: string) {
    if (!/[/\\]/.test(name) && !name.startsWith('.')) return false
    error.value = t('Ein Name darf kein „/“ enthalten und nicht mit „.“ beginnen.')
    return true
  }

  function remap(from: string, to: string) {
    const re = (p: string) => (under(p, from) ? to + p.slice(from.length) : p)
    for (const pn of panes) if (paths[pn]) paths[pn] = re(paths[pn]!)
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
      error.value =
        e instanceof ApiError && e.status === 409
          ? t('„{name}“ existiert schon.', { name: to })
          : errorMessage(e)
    }
  }

  async function rename(name: string) {
    const from = ctx.renaming
    ctx.renaming = null
    if (!from || badName(name)) return
    const keep = renamingDir || fileKind(from) === 'code'
    await move(
      from,
      join(parentOf(from), keep ? name : withExt(name, from.slice(from.lastIndexOf('.')))),
    )
  }

  async function drop(dir: string, dt?: DataTransfer | null) {
    const from = ctx.dragging
    ctx.dragging = ctx.dropTarget = null
    const entries = [...(dt?.items ?? [])]
      .filter((i) => i.kind === 'file')
      .map((i) => i.webkitGetAsEntry())
      .filter((e) => e !== null)
    if (entries.length) return addFiles(dir, (await Promise.all(entries.map(readEntry))).flat())
    if (!from || !canDrop(from, dir)) return
    if (dir) ctx.open.add(dir)
    await move(from, join(dir, from.slice(from.lastIndexOf('/') + 1)))
  }

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

  const parents = (p: string) =>
    p
      .split('/')
      .slice(0, -1)
      .map((_, i, a) => a.slice(0, i + 1).join('/'))

  async function addFiles(dir: string, uploads: Upload[]) {
    uploads = uploads.filter((u) => !u.path.split('/').some((s) => s.startsWith('.')))
    const bad = uploads.find((u) => !fileKind(u.path))
    if (bad)
      error.value = t(
        '„{name}“: Kairo nimmt nur Markdown, PDFs, Bilder (png, jpg, gif, webp), C/C++, Makefiles und Zeichnungen.',
        { name: bad.path },
      )
    uploads = uploads.filter((u) => fileKind(u.path))
    if (!uploads.length) return
    let last = ''
    try {
      for (const d of new Set(uploads.flatMap((u) => parents(u.path).map((d) => join(dir, d))))) {
        await createNote(d, true).catch((e) => {
          if (!(e instanceof ApiError && e.status === 409)) throw e
        })
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
      error.value =
        e instanceof ApiError && e.status === 409
          ? t('Eine Datei mit diesem Namen gibt es dort schon.')
          : errorMessage(e)
    }
    if (dir) ctx.open.add(dir)
    saveOpen()
    try {
      tree.value = await listNotes()
    } catch {
      /* nächste Runde von refresh */
    }
    if (last) await openFile(last)
  }

  function remove(n: NoteNode, anchor: HTMLElement) {
    confirmPopup.require({
      target: anchor,
      message: n.dir
        ? t('Ordner „{name}“ mit allem Inhalt löschen? Er landet in .trash im Notizordner.', {
            name: n.name,
          })
        : t('„{name}“ löschen? Die Notiz landet in .trash im Notizordner.', {
            name: n.name.replace(/\.md$/i, ''),
          }),
      acceptLabel: t('Löschen'),
      rejectLabel: t('Abbrechen'),
      defaultFocus: 'reject',
      acceptProps: { size: 'small' },
      rejectProps: { severity: 'secondary', size: 'small', text: true },
      accept: async () => {
        try {
          await deleteNote(n.path)
          for (const pn of panes) {
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

  async function add(name: string) {
    const target = ctx.adding
    ctx.adding = null
    if (!target || badName(name)) return
    const p = join(
      target.parent,
      target.dir ? name : withExt(name, target.drawing ? '.excalidraw' : '.md'),
    )
    try {
      if (target.drawing)
        await uploadFile(p, new Blob([EMPTY_DRAWING], { type: 'application/json' }))
      else await createNote(p, target.dir)
      if (target.dir) ctx.toggle(p)
      tree.value = await listNotes()
      if (!target.dir) await openFile(p)
      error.value = ''
    } catch (e) {
      error.value = errorMessage(e)
    }
  }

  return {
    ctx,
    addFiles,
  }
}
