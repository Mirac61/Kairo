<script lang="ts">
import type { InjectionKey, Ref } from 'vue'
import type { NoteNode } from '@/api/client'

// Gemeinsamer Zustand aller Ebenen; die Ansicht hält ihn, damit Ereignisse nicht durch jede Ebene gereicht werden müssen.
export interface TreeCtx {
  isActive(path: string): boolean // offen und sichtbar (geteilt: Notiz und Dokument)
  open: Set<string>
  adding: { parent: string; dir: boolean; drawing?: boolean } | null
  renaming: string | null
  dragging: string | null
  dropTarget: string | null // Ordner unter dem Mauszeiger, '' = Wurzelordner
  setDragging(path: string | null): void
  setDropTarget(target: string | null): void
  clearDrag(): void
  select(path: string): void
  toggle(path: string): void
  startAdd(parent: string, dir: boolean, drawing?: boolean): void
  submitAdd(name: string): void
  cancelAdd(): void
  startRename(path: string, dir: boolean): void
  submitRename(name: string): void
  cancelRename(): void
  remove(node: NoteNode): void // sofort, mit Rückgängig im Toast
  duplicate(node: NoteNode): void // nur Dateien
  exportDir(node: NoteNode): void // Ordner samt Inhalt als ZIP
  drop(dir: string, dt?: DataTransfer | null): void // dt.items: Dateien/Ordner aus dem Finder
}

// Das Menü gehört der Seitenleiste (eine Instanz); jede Ebene des Baums ruft es über inject.
// x/y: Fensterkoordinaten; back bekommt den Fokus zurück, wenn das Menü mit Esc schließt.
export const NT_MENU: InjectionKey<(x: number, y: number, n: NoteNode, back: HTMLElement) => void> =
  Symbol('nt-menu')
// Pfad der Zeile, auf die die Kürzel wirken ('' = oberste Ebene); die Zeile bekommt einen Rahmen.
export const NT_CUR: InjectionKey<Ref<string | null>> = Symbol('nt-cur')

export const parentOf = (path: string) => path.slice(0, Math.max(path.lastIndexOf('/'), 0))

// Ziel ist sinnvoll: nicht der eigene Ordner und bei Ordnern nicht in sich selbst.
export const canDrop = (from: string | null, dir: string) =>
  from !== null && dir !== parentOf(from) && dir !== from && !dir.startsWith(`${from}/`)

// Dateien aus dem Finder dürfen in jeden Ordner, Einträge aus dem Baum nur an sinnvolle Ziele.
export const canDropHere = (e: DragEvent, from: string | null, dir: string) =>
  !!e.dataTransfer?.types.includes('Files') || canDrop(from, dir)
</script>

<script setup lang="ts">
import { inject, nextTick, ref } from 'vue'
import { ymd } from '@/lib/dates'
import { fileKind } from '@/lib/noteFiles'
import FoldTransition from '@/components/FoldTransition.vue'

// Rekursiv: ein Ordner rendert seine Kinder wieder mit NoteTree. parent '' ist der Wurzelordner.
const props = withDefaults(
  defineProps<{ nodes: NoteNode[]; ctx: TreeCtx; parent?: string; depth?: number }>(),
  { parent: '', depth: 0 },
)

const name = ref('')
const openMenu = inject(NT_MENU, () => {})
const cur = inject(NT_CUR, ref(null))
// Mit Wert (Umbenennen) holt sich das Feld den alten Namen selbst, egal ob Doppelklick, Menü oder Kürzel es öffnet.
const vFocus = {
  mounted: (el: HTMLInputElement, b: { value?: string }) => {
    if (b.value !== undefined) name.value = b.value
    void nextTick(() => {
      el.focus()
      el.select()
    })
  },
}
// Neue Notiz: das Feld beginnt mit dem heutigen Datum (Vorlesungen sortieren sich so von selbst), der Cursor steht dahinter.
const vStart = {
  mounted: (el: HTMLInputElement, b: { value: string }) => {
    name.value = b.value
    void nextTick(() => {
      el.focus()
      el.setSelectionRange(el.value.length, el.value.length)
    })
  },
}
const indent = () => ({ paddingLeft: `${8 + props.depth * 14}px` })
// Dateien ohne Endung; den Typ zeigt das Icon.
// Code behält die Endung, sonst sähen tree.cpp und tree.hpp gleich aus.
const label = (n: NoteNode) =>
  n.dir || fileKind(n.path) === 'code' ? n.name : n.name.replace(/\.[^.]+$/, '')
const ICONS = {
  md: '#i-md',
  pdf: '#i-pdf',
  image: '#i-image',
  code: '#i-code',
  excalidraw: '#i-draw',
  '': '#i-file',
}
const icon = (n: NoteNode) => (n.dir ? '#i-proj' : ICONS[fileKind(n.path)])
const kind = (n: NoteNode) => (n.dir ? undefined : fileKind(n.path) || undefined)
// Auf einer Datei abgelegt heißt: in ihren Ordner.
const dirOf = (n: NoteNode) => (n.dir ? n.path : props.parent)

function submit(done: (n: string) => void, cancel: () => void) {
  const n = name.value.trim()
  name.value = ''
  if (n) done(n)
  else cancel()
}
// Rechtsklick öffnet am Mauszeiger, „⋯“ unter dem Knopf.
function menu(e: MouseEvent, n: NoteNode) {
  const el = e.currentTarget as HTMLElement
  const main = el.closest('.nt-row')!.querySelector<HTMLElement>('.nt-main')!
  const r = el.getBoundingClientRect()
  if (e.type === 'contextmenu') openMenu(e.clientX, e.clientY, n, main)
  else openMenu(r.left, r.bottom + 4, n, main)
}
// Die Kürzel selbst verarbeitet NotesSidebar (aus den Menüeinträgen); hier nur für Screenreader.
const ARIA_KEYS = 'N Z O R F2 Backspace Delete'
function dragStart(e: DragEvent, n: NoteNode) {
  e.dataTransfer?.setData('text/plain', n.path)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  props.ctx.setDragging(n.path)
}
function dragOver(e: DragEvent, n: NoteNode) {
  if (!canDropHere(e, props.ctx.dragging, dirOf(n))) return // ohne preventDefault zeigt der Browser „nicht erlaubt“
  e.preventDefault()
  e.stopPropagation()
  props.ctx.setDropTarget(dirOf(n))
}
</script>

<template>
  <ul class="nt" :role="depth ? 'group' : 'tree'" :aria-label="depth ? undefined : 'Notizen'">
    <li v-for="n in nodes" :key="n.path" role="none">
      <div v-if="ctx.renaming === n.path" class="nt-new" :style="indent()">
        <svg class="ic nt-ic" :data-kind="kind(n)" aria-hidden="true"><use :href="icon(n)" /></svg>
        <input
          v-model="name"
          v-focus="label(n)"
          class="input nt-input"
          :aria-label="`${n.name} umbenennen`"
          @keydown.enter.prevent="submit(ctx.submitRename, ctx.cancelRename)"
          @keydown.esc.prevent="ctx.cancelRename()"
          @blur="ctx.cancelRename()"
        />
      </div>
      <div
        v-else
        class="nt-row"
        :data-path="n.path"
        draggable="true"
        :class="{
          active: !n.dir && ctx.isActive(n.path),
          cur: cur === n.path,
          drop: n.dir && ctx.dropTarget === n.path,
          dragging: ctx.dragging === n.path,
        }"
        @contextmenu.prevent="menu($event, n)"
        @dragstart="dragStart($event, n)"
        @dragend="ctx.clearDrag()"
        @dragover="dragOver($event, n)"
        @drop.prevent.stop="ctx.drop(dirOf(n), $event.dataTransfer)"
      >
        <button
          type="button"
          class="nt-main"
          role="treeitem"
          :style="indent()"
          :aria-expanded="n.dir ? ctx.open.has(n.path) : undefined"
          :aria-current="!n.dir && ctx.isActive(n.path) ? 'page' : undefined"
          :aria-keyshortcuts="`${ARIA_KEYS} ${n.dir ? 'E' : 'D'}`"
          @click="n.dir ? ctx.toggle(n.path) : ctx.select(n.path)"
          @dblclick="ctx.startRename(n.path, n.dir)"
        >
          <svg
            class="ic nt-chev"
            :class="{ open: ctx.open.has(n.path), hidden: !n.dir }"
            aria-hidden="true"
          >
            <use href="#i-chev" />
          </svg>
          <svg class="ic nt-ic" :data-kind="kind(n)" aria-hidden="true">
            <use :href="icon(n)" />
          </svg>
          <span class="nt-name">{{ label(n) }}</span>
        </button>
        <button
          type="button"
          class="icon-btn nt-more"
          :aria-label="$t('Weitere Aktionen für {name}', { name: label(n) })"
          aria-haspopup="menu"
          @click="menu($event, n)"
        >
          <svg class="ic"><use href="#i-more" /></svg>
        </button>
      </div>
      <FoldTransition>
        <NoteTree
          v-if="n.dir && ctx.open.has(n.path)"
          :nodes="n.children ?? []"
          :ctx="ctx"
          :parent="n.path"
          :depth="depth + 1"
        />
      </FoldTransition>
    </li>
    <li v-if="ctx.adding?.parent === parent" role="none" class="nt-new" :style="indent()">
      <svg class="ic nt-ic" aria-hidden="true">
        <use :href="ctx.adding.dir ? '#i-proj' : ctx.adding.drawing ? '#i-draw' : '#i-md'" />
      </svg>
      <input
        v-model="name"
        v-start="ctx.adding.dir || ctx.adding.drawing ? '' : `${ymd(new Date())} `"
        class="input nt-input"
        :placeholder="
          ctx.adding.dir
            ? $t('Ordnername')
            : ctx.adding.drawing
              ? $t('Name der Zeichnung')
              : $t('Name der Notiz')
        "
        :aria-label="
          ctx.adding.dir
            ? $t('Name des neuen Ordners')
            : ctx.adding.drawing
              ? $t('Name der neuen Zeichnung')
              : $t('Name der neuen Notiz')
        "
        @keydown.enter.prevent="submit(ctx.submitAdd, ctx.cancelAdd)"
        @keydown.esc.prevent="ctx.cancelAdd()"
        @blur="ctx.cancelAdd()"
      />
    </li>
  </ul>
</template>

<style scoped>
.nt {
  list-style: none;
  margin: 0;
  padding: 0;
}
.nt-row {
  position: relative;
  display: flex;
  align-items: center;
  border-radius: 7px;
  transition: background var(--dur) ease-out;
}
.nt-row:hover {
  background: var(--bg-hover);
}
.nt-row.active {
  background: var(--bg-selected);
}
.nt-row.cur {
  box-shadow: inset 0 0 0 1px var(--ring);
}
.nt-row.drop {
  background: var(--bg-selected);
  box-shadow: inset 0 0 0 1px var(--a-blue-hi);
}
.nt-row.dragging {
  opacity: 0.5;
}
.nt-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding-right: 8px;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-secondary);
  text-align: left;
}
.nt-row:hover .nt-main,
.nt-row.active .nt-main {
  color: var(--tx-primary);
}
.nt-row.active .nt-ic {
  color: var(--a-blue-hi);
}
.nt-chev {
  width: 12px;
  height: 12px;
  color: var(--tx-muted);
  transition: transform var(--dur-fold) var(--ease-fold);
}
.nt-chev.open {
  transform: rotate(90deg);
}
.nt-chev.hidden {
  visibility: hidden;
}
.nt-ic {
  width: 15px;
  height: 15px;
  color: var(--tx-muted);
}
/* Nur was keine normale Notiz ist, bekommt eine gedämpfte Farbe; Ordner und Markdown bleiben grau. */
.nt-ic[data-kind='pdf'] {
  color: color-mix(in srgb, var(--p-red) 80%, var(--tx-muted));
}
.nt-ic[data-kind='excalidraw'] {
  color: color-mix(in srgb, var(--p-violet) 80%, var(--tx-muted));
}
.nt-ic[data-kind='image'] {
  color: color-mix(in srgb, var(--p-green) 80%, var(--tx-muted));
}
.nt-ic[data-kind='code'] {
  color: color-mix(in srgb, var(--p-yellow) 80%, var(--tx-muted));
}
.nt-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.nt-more {
  display: none;
  width: 24px;
  height: 24px;
  margin-right: 4px;
}
.nt-more .ic {
  width: 13px;
  height: 13px;
}
.nt-row:hover .nt-more,
.nt-row:focus-within .nt-more {
  display: inline-flex;
}
/* Touch kennt kein Hover: „⋯“ bleibt sichtbar, Rechtsklick gibt es dort nicht. */
@media (hover: none) {
  .nt-more {
    display: inline-flex;
  }
}
.nt-new {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding-right: 4px;
}
.nt-new .nt-ic {
  margin-left: 18px;
}
.nt-input {
  height: 28px;
  padding: 0 8px;
  font-size: 13px;
}
</style>
