<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, provide, ref } from 'vue'
import type { NoteNode } from '@/api/client'
import { t } from '@/lib/i18n'
import { ariaKeys, keys } from '@/lib/keys'
import { assetName } from '@/lib/noteFiles'
import NoteTree, { canDropHere, NT_CUR, NT_MENU, parentOf, type TreeCtx } from './NoteTree.vue'

const props = defineProps<{
  treeOpen: boolean
  tree: NoteNode[]
  ctx: TreeCtx
}>()

const emit = defineEmits<{
  (e: 'pick', uploads: { path: string; file: File }[], dir?: string): void
}>()

// Ein Menü (Haus-Muster .menu) für Rechtsklick, „⋯“ und Hinzufügen; '-' ist ein Trenner.
type Item = { label: string; key?: string; danger?: boolean; run: () => void }
const menu = ref<{ x: number; y: number; items: (Item | '-')[]; back: HTMLElement } | null>(null)
const menuEl = ref<HTMLElement>()

async function openAt(x: number, y: number, items: (Item | '-')[], back: HTMLElement) {
  menu.value = { x, y, items, back }
  await nextTick()
  const el = menuEl.value
  if (!el || !menu.value) return
  // Am Fensterrand nach links bzw. oben klappen statt abschneiden.
  const r = el.getBoundingClientRect()
  menu.value.x = Math.max(8, Math.min(x, innerWidth - r.width - 8))
  if (y + r.height > innerHeight - 8) menu.value.y = Math.max(8, y - r.height)
  el.querySelector<HTMLElement>('.menu-i')?.focus()
}
function close(refocus = false) {
  const back = menu.value?.back
  menu.value = null
  if (refocus) back?.focus()
}
function run(i: Item) {
  close()
  i.run()
}
// Gedrückte Taste in der Schreibweise der Menü-Kürzel („N“, „⌫“); mit ⌘/Strg/⌥ ist es nie ein Notiz-Kürzel.
// F2 ist ein Zweitname für Umbenennen (Windows Explorer, VS Code), Entf einer für Löschen.
function combo(e: KeyboardEvent) {
  if (e.metaKey || e.ctrlKey || e.altKey) return ''
  if (e.key === 'F2') return 'R'
  const k = e.key === 'Backspace' || e.key === 'Delete' ? '⌫' : e.key.toUpperCase()
  return `${e.shiftKey ? '⇧' : ''}${k}`
}
// Die Menüeinträge sind die einzige Tabelle der Kürzel: was dort steht, funktioniert auch.
function runKey(items: (Item | '-')[], e: KeyboardEvent) {
  const hit = items.find((i): i is Item => i !== '-' && i.key === combo(e))
  if (!hit) return false
  e.preventDefault()
  run(hit)
  return true
}
function onMenuKey(e: KeyboardEvent) {
  if (e.key === 'Escape' || e.key === 'Tab') return close(true)
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return runKey(menu.value!.items, e)
  e.preventDefault()
  const all = [...menuEl.value!.querySelectorAll<HTMLElement>('.menu-i')]
  const i = all.indexOf(document.activeElement as HTMLElement)
  all[(i + (e.key === 'ArrowDown' ? 1 : -1) + all.length) % all.length]?.focus()
}

// Einzeltasten wie in der Aufgaben-Ansicht: gelten, solange man nicht schreibt (Esc verlässt den Editor).
// ⌘ + Buchstabe gehört im Browser fast immer dem Browser, ⌥ tippt Sonderzeichen.
// Neu anlegen in dir ('' = oberste Ebene).
const newItems = (dir: string): Item[] => [
  { label: t('Neue Notiz'), key: 'N', run: () => props.ctx.startAdd(dir, false) },
  { label: t('Neue Zeichnung'), key: 'Z', run: () => props.ctx.startAdd(dir, false, true) },
  { label: t('Neuer Ordner'), key: 'O', run: () => props.ctx.startAdd(dir, true) },
]
// Der Browser kennt keinen Dialog für Dateien und Ordner zugleich, daher zwei Einträge.
const uploadItems = (): Item[] => [
  { label: t('Dateien …'), run: () => filePick.value?.click() },
  { label: t('Ordner …'), run: () => dirPick.value?.click() },
]
// Oberste Ebene: Rechtsklick auf freie Fläche im Baum, und die Kürzel, solange keine Zeile gewählt ist.
const rootItems = (): (Item | '-')[] => [...newItems(''), '-', ...uploadItems()]

function treeItems(n: NoteNode): (Item | '-')[] {
  const c = props.ctx
  return [
    ...newItems(n.dir ? n.path : parentOf(n.path)), // auf einer Datei: im selben Ordner anlegen
    '-',
    { label: t('Umbenennen'), key: 'R', run: () => c.startRename(n.path, n.dir) },
    n.dir
      ? { label: t('Als ZIP exportieren'), key: 'E', run: () => c.exportDir(n) }
      : { label: t('Duplizieren'), key: 'D', run: () => c.duplicate(n) },
    '-',
    { label: t('Löschen'), key: '⌫', danger: true, run: () => c.remove(n) },
  ]
}
provide(NT_MENU, (x, y, n, back) => void openAt(x, y, treeItems(n), back))
function rootMenu(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('.nt-row, input')) return
  e.preventDefault()
  void openAt(e.clientX, e.clientY, rootItems(), e.currentTarget as HTMLElement)
}

// Worauf die Kürzel wirken: die angeklickte (oder per Tab erreichte) Zeile; '' = oberste Ebene nach Klick auf
// freie Fläche im Baum; null = die offene Datei (Neues landet neben ihr), ohne offene Datei die oberste Ebene.
// Ein Klick außerhalb des Baums setzt auf null zurück. Nicht über den Fokus der Zeile: Safari fokussiert Knöpfe
// beim Klick nicht.
const current = ref<string | null>(null)
provide(NT_CUR, current)
function track(e: Event) {
  const row = (e.target as HTMLElement).closest<HTMLElement>('.nt-row')
  if (row || e.type === 'pointerdown') current.value = row?.dataset.path ?? ''
}
function forget(e: PointerEvent) {
  if (!(e.target as HTMLElement).closest('#nt-pane, .nt-menu')) current.value = null
}
function find(nodes: NoteNode[], hit: (n: NoteNode) => boolean): NoteNode | undefined {
  for (const n of nodes) {
    const found = hit(n) ? n : n.children && find(n.children, hit)
    if (found) return found
  }
}
function target(): NoteNode | undefined {
  const p = current.value
  if (p !== null) return p ? find(props.tree, (n) => n.path === p) : undefined
  return find(props.tree, (n) => !n.dir && props.ctx.isActive(n.path))
}
const TYPING = 'input, textarea, select, [contenteditable], .overlay, dialog, .excalidraw'
// Überall in der Notizen-Ansicht, nur nicht beim Schreiben und in Dialogen. Esc verlässt den Editor (außer
// eine Vorschlagsliste ist offen: die schließt Esc zuerst), danach gelten die Tasten.
// Capture-Phase: läuft vor FileEditor (window, Bubble), damit R im Baum nicht zugleich das Rechteck wählt.
function onKey(e: KeyboardEvent) {
  const el = e.target as HTMLElement
  if (
    e.key === 'Escape' &&
    el.closest('.cm-content') &&
    !document.querySelector('.cm-tooltip-autocomplete')
  )
    return el.blur()
  if (!props.treeOpen || menu.value || el.closest(TYPING)) return
  // Ein offenes PDF/Bild hat eigene Werkzeugtasten (R, O, ⌫ …); die gelten, bis man in den Baum klickt.
  if (current.value === null && document.querySelector('.fe-keys')) return
  const n = target()
  if (runKey(n ? treeItems(n) : rootItems(), e)) e.stopImmediatePropagation()
}
// ⌘V mit Dateien (im Finder kopiert, Screenshot) lädt sie dorthin, wo N anlegen würde. Beim Schreiben fügt der Editor ein.
function onPaste(e: ClipboardEvent) {
  const el = e.target as HTMLElement
  if (!props.treeOpen || el.closest?.(TYPING)) return
  const files = [...(e.clipboardData?.files ?? [])]
  if (!files.length) return
  e.preventDefault()
  const now = new Date()
  // Bilder aus der Zwischenablage heißen im Browser „image.png“; mit Zeitstempel kollidiert das zweite nicht.
  const uploads = files.map((file, i) => ({
    path: /^image\.\w+$/.test(file.name)
      ? assetName(t('Bild'), file.type, now, i, files.length)
      : file.name,
    file,
  }))
  const n = target()
  emit('pick', uploads, n ? (n.dir ? n.path : parentOf(n.path)) : '')
}
onMounted(() => {
  addEventListener('keydown', onKey, true)
  addEventListener('paste', onPaste)
  addEventListener('pointerdown', forget, true)
})
onBeforeUnmount(() => {
  removeEventListener('keydown', onKey, true)
  removeEventListener('paste', onPaste)
  removeEventListener('pointerdown', forget, true)
})

const filePick = ref<HTMLInputElement>()
const dirPick = ref<HTMLInputElement>()

function openAdd(e: MouseEvent) {
  const btn = e.currentTarget as HTMLElement
  const r = btn.getBoundingClientRect()
  void openAt(r.left, r.bottom + 6, uploadItems(), btn)
}

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  const uploads = [...(input.files ?? [])].map((file) => ({
    path: file.webkitRelativePath || file.name,
    file,
  }))
  input.value = ''
  emit('pick', uploads)
}
</script>

<template>
  <aside v-show="treeOpen" id="nt-pane" class="nt-pane" @pointerdown="track" @focusin="track">
    <div class="nt-head">
      <span class="lbl">{{ $t('Notizen') }}</span>
      <span class="nt-head-act">
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neue Notiz')"
          :title="$t('Neue Notiz')"
          :data-tip="$t('Neue Notiz')"
          @click="ctx.startAdd('', false)"
        >
          <svg class="ic"><use href="#i-plus" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neue Zeichnung')"
          :title="$t('Neue Zeichnung')"
          :data-tip="$t('Neue Zeichnung')"
          @click="ctx.startAdd('', false, true)"
        >
          <svg class="ic"><use href="#i-draw" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Neuer Ordner')"
          :title="$t('Neuer Ordner')"
          :data-tip="$t('Neuer Ordner')"
          @click="ctx.startAdd('', true)"
        >
          <svg class="ic"><use href="#i-proj" /></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :aria-label="$t('Dateien oder Ordner hinzufügen')"
          :title="$t('PDFs, Bilder, Notizen oder ganze Ordner hinzufügen')"
          :data-tip="$t('PDFs, Bilder, Notizen oder ganze Ordner hinzufügen')"
          aria-haspopup="menu"
          @click="openAdd"
        >
          <svg class="ic"><use href="#i-upload" /></svg>
        </button>
        <input ref="filePick" type="file" multiple hidden @change="onPick" />
        <input ref="dirPick" type="file" webkitdirectory hidden @change="onPick" />
      </span>
    </div>
    <div
      class="nt-scroll"
      :class="{ drop: ctx.dropTarget === '' }"
      @contextmenu="rootMenu"
      @dragover="
        canDropHere($event, ctx.dragging, '') && ($event.preventDefault(), ctx.setDropTarget(''))
      "
      @dragleave.self="ctx.setDropTarget(null)"
      @drop.prevent="ctx.drop('', $event.dataTransfer)"
    >
      <NoteTree :nodes="tree" :ctx="ctx" />
      <p v-if="!tree.length && !ctx.adding" class="nt-empty">
        {{ $t('Noch keine Notizen. Lege oben eine an oder zieh PDFs und Bilder hierher.') }}
      </p>
    </div>
    <Teleport v-if="menu" to="body">
      <div class="menu-scrim" @click="close()" @contextmenu.prevent="close()"></div>
      <div
        ref="menuEl"
        class="menu nt-menu"
        role="menu"
        :style="{ '--x': `${menu.x}px`, '--y': `${menu.y}px` }"
        @keydown="onMenuKey"
      >
        <template v-for="(i, k) in menu.items" :key="k">
          <hr v-if="i === '-'" class="nt-menu-sep" />
          <button
            v-else
            type="button"
            role="menuitem"
            class="menu-i"
            :class="{ danger: i.danger }"
            :aria-keyshortcuts="i.key && ariaKeys(i.key)"
            @click="run(i)"
          >
            <span>{{ i.label }}</span
            ><kbd v-if="i.key" class="key" aria-hidden="true">{{ keys(i.key) }}</kbd>
          </button>
        </template>
      </div>
    </Teleport>
  </aside>
</template>

<style scoped>
.nt-menu .menu-i {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.nt-menu-sep {
  height: 1px;
  margin: 4px 6px;
  border: 0;
  background: var(--br-default);
}
/* Am Mauszeiger statt unter einem Knopf; auf dem Handy bleibt es das Blatt aus design.css. */
@media (min-width: 641px) {
  .nt-menu {
    position: fixed;
    left: var(--x);
    top: var(--y);
    right: auto;
    min-width: 200px;
  }
}
</style>
