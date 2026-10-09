<script lang="ts">
import type { NoteNode } from '@/api/client'

// Gemeinsamer Zustand aller Ebenen; die Ansicht hält ihn, damit Ereignisse nicht durch jede Ebene gereicht werden müssen.
export interface TreeCtx {
  active: string | null
  open: Set<string>
  adding: { parent: string; dir: boolean } | null
  renaming: string | null
  dragging: string | null
  dropTarget: string | null // Ordner unter dem Mauszeiger, '' = Wurzelordner
  select(path: string): void
  toggle(path: string): void
  startAdd(parent: string, dir: boolean): void
  submitAdd(name: string): void
  cancelAdd(): void
  startRename(path: string, dir: boolean): void
  submitRename(name: string): void
  cancelRename(): void
  remove(node: NoteNode, anchor: HTMLElement): void
  drop(dir: string, files?: FileList): void // files: aus dem Finder gezogen
}

export const parentOf = (path: string) => path.slice(0, Math.max(path.lastIndexOf('/'), 0))

// Ziel ist sinnvoll: nicht der eigene Ordner und bei Ordnern nicht in sich selbst.
export const canDrop = (from: string | null, dir: string) =>
  from !== null && dir !== parentOf(from) && dir !== from && !dir.startsWith(`${from}/`)

// Dateien aus dem Finder dürfen in jeden Ordner, Einträge aus dem Baum nur an sinnvolle Ziele.
export const canDropHere = (e: DragEvent, from: string | null, dir: string) =>
  !!e.dataTransfer?.types.includes('Files') || canDrop(from, dir)
</script>

<script setup lang="ts">
import { ref } from 'vue'
import { fileKind } from '@/lib/noteFiles'

// Rekursiv: ein Ordner rendert seine Kinder wieder mit NoteTree. parent '' ist der Wurzelordner.
const props = withDefaults(defineProps<{ nodes: NoteNode[]; ctx: TreeCtx; parent?: string; depth?: number }>(), { parent: '', depth: 0 })

const name = ref('')
const vFocus = { mounted: (el: HTMLInputElement) => { el.focus(); el.select() } }
const indent = () => ({ paddingLeft: `${8 + props.depth * 14}px` })
// Dateien ohne Endung; den Typ zeigt das Icon.
const label = (n: NoteNode) => (n.dir ? n.name : n.name.replace(/\.[^.]+$/, ''))
const ICONS = { md: '#i-md', pdf: '#i-pdf', image: '#i-image', '': '#i-file' }
const icon = (n: NoteNode) => (n.dir ? '#i-proj' : ICONS[fileKind(n.path)])
// Auf einer Datei abgelegt heißt: in ihren Ordner.
const dirOf = (n: NoteNode) => (n.dir ? n.path : props.parent)

function submit(done: (n: string) => void, cancel: () => void) {
  const n = name.value.trim()
  name.value = ''
  if (n) done(n)
  else cancel()
}
function startRename(n: NoteNode) {
  name.value = label(n)
  props.ctx.startRename(n.path, n.dir)
}
function dragStart(e: DragEvent, n: NoteNode) {
  e.dataTransfer?.setData('text/plain', n.path)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  props.ctx.dragging = n.path
}
function dragOver(e: DragEvent, n: NoteNode) {
  if (!canDropHere(e, props.ctx.dragging, dirOf(n))) return // ohne preventDefault zeigt der Browser „nicht erlaubt“
  e.preventDefault()
  e.stopPropagation()
  props.ctx.dropTarget = dirOf(n)
}
</script>

<template>
  <ul class="nt" :role="depth ? 'group' : 'tree'" :aria-label="depth ? undefined : 'Notizen'">
    <li v-for="n in nodes" :key="n.path" role="none">
      <div v-if="ctx.renaming === n.path" class="nt-new" :style="indent()">
        <svg class="ic nt-ic" aria-hidden="true"><use :href="icon(n)" /></svg>
        <input
          v-model="name" v-focus class="input nt-input" :aria-label="`${n.name} umbenennen`"
          @keydown.enter.prevent="submit(ctx.submitRename, ctx.cancelRename)" @keydown.esc.prevent="ctx.cancelRename()" @blur="ctx.cancelRename()"
        />
      </div>
      <div
        v-else class="nt-row" draggable="true"
        :class="{ active: !n.dir && n.path === ctx.active, drop: n.dir && ctx.dropTarget === n.path, dragging: ctx.dragging === n.path }"
        @dragstart="dragStart($event, n)" @dragend="ctx.dragging = ctx.dropTarget = null"
        @dragover="dragOver($event, n)" @drop.prevent.stop="ctx.drop(dirOf(n), $event.dataTransfer?.files)"
      >
        <button
          type="button" class="nt-main" role="treeitem" :style="indent()"
          :aria-expanded="n.dir ? ctx.open.has(n.path) : undefined"
          :aria-current="!n.dir && n.path === ctx.active ? 'page' : undefined"
          aria-keyshortcuts="F2"
          @click="n.dir ? ctx.toggle(n.path) : ctx.select(n.path)" @dblclick="startRename(n)" @keydown.f2.prevent="startRename(n)"
        >
          <svg class="ic nt-chev" :class="{ open: ctx.open.has(n.path), hidden: !n.dir }" aria-hidden="true"><use href="#i-chev" /></svg>
          <svg class="ic nt-ic" aria-hidden="true"><use :href="icon(n)" /></svg>
          <span class="nt-name">{{ label(n) }}</span>
        </button>
        <span class="nt-act">
          <template v-if="n.dir">
            <button type="button" class="icon-btn" :aria-label="`Neue Notiz in ${n.name}`" data-tip="Neue Notiz" @click="ctx.startAdd(n.path, false)"><svg class="ic"><use href="#i-plus" /></svg></button>
            <button type="button" class="icon-btn" :aria-label="`Neuer Ordner in ${n.name}`" data-tip="Neuer Ordner" @click="ctx.startAdd(n.path, true)"><svg class="ic"><use href="#i-proj" /></svg></button>
          </template>
          <button type="button" class="icon-btn" :aria-label="`${label(n)} umbenennen`" data-tip="Umbenennen (F2)" @click="startRename(n)"><svg class="ic"><use href="#i-pen" /></svg></button>
          <button type="button" class="icon-btn" :aria-label="`${label(n)} löschen`" data-tip="Löschen" @click="ctx.remove(n, $event.currentTarget as HTMLElement)"><svg class="ic"><use href="#i-trash" /></svg></button>
        </span>
      </div>
      <NoteTree v-if="n.dir && ctx.open.has(n.path)" :nodes="n.children ?? []" :ctx="ctx" :parent="n.path" :depth="depth + 1" />
    </li>
    <li v-if="ctx.adding?.parent === parent" role="none" class="nt-new" :style="indent()">
      <svg class="ic nt-ic" aria-hidden="true"><use :href="ctx.adding.dir ? '#i-proj' : '#i-md'" /></svg>
      <input
        v-model="name" v-focus class="input nt-input" :placeholder="ctx.adding.dir ? 'Ordnername' : 'Name der Notiz'"
        :aria-label="ctx.adding.dir ? 'Name des neuen Ordners' : 'Name der neuen Notiz'"
        @keydown.enter.prevent="submit(ctx.submitAdd, ctx.cancelAdd)" @keydown.esc.prevent="ctx.cancelAdd()" @blur="ctx.cancelAdd()"
      />
    </li>
  </ul>
</template>

<style scoped>
.nt { list-style: none; margin: 0; padding: 0; }
.nt-row { position: relative; display: flex; align-items: center; border-radius: 7px; transition: background var(--dur) ease-out; }
.nt-row:hover { background: var(--bg-hover); }
.nt-row.active { background: var(--bg-selected); }
.nt-row.drop { background: var(--bg-selected); box-shadow: inset 0 0 0 1px var(--a-blue-hi); }
.nt-row.dragging { opacity: .5; }
.nt-main {
  flex: 1; min-width: 0; display: flex; align-items: center; gap: 6px; height: 30px; padding-right: 8px;
  font: 400 13px/1 var(--font-ui); color: var(--tx-secondary); text-align: left;
}
.nt-row:hover .nt-main, .nt-row.active .nt-main { color: var(--tx-primary); }
.nt-row.active .nt-ic { color: var(--a-blue-hi); }
.nt-chev { width: 12px; height: 12px; color: var(--tx-muted); transition: transform var(--dur) ease-out; }
.nt-chev.open { transform: rotate(90deg); }
.nt-chev.hidden { visibility: hidden; }
.nt-ic { width: 15px; height: 15px; color: var(--tx-muted); }
.nt-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.nt-act { display: none; gap: 2px; padding-right: 4px; }
.nt-act .icon-btn { width: 24px; height: 24px; }
.nt-act .ic { width: 13px; height: 13px; }
.nt-row:hover .nt-act, .nt-row:focus-within .nt-act { display: flex; }
.nt-new { display: flex; align-items: center; gap: 6px; height: 34px; padding-right: 4px; }
.nt-new .nt-ic { margin-left: 18px; }
.nt-input { height: 28px; padding: 0 8px; font-size: 13px; }
</style>
