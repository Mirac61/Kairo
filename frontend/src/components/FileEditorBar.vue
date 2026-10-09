<script setup lang="ts">
import { COLORS, SCALES, SIZES, TOOLS, type Page, type Size, type Tool } from '@/lib/annotate'
import { filesUrl } from '@/lib/noteFiles'

// Werkzeugleiste des FileEditor: Werkzeuge, Farbe, Stärke, Verlauf, Bildgröße beim Speichern und Zoom.
defineProps<{
  kind: 'pdf' | 'image'; path: string; editable: boolean
  tool: Tool; color: string; size: Size; canUndo: boolean; canRedo: boolean; canRemove: boolean
  page?: Page; pct: number
}>()
const outScale = defineModel<number>('outScale', { required: true })
defineEmits<{ tool: [Tool]; color: [string]; size: [Size]; undo: []; redo: []; remove: []; zoom: [1 | -1]; fit: [] }>()
</script>

<template>
  <div class="fe-bar" role="toolbar" :aria-label="$t('Bearbeiten')">
    <template v-if="editable">
      <template v-for="t in TOOLS" :key="t.id">
        <button
          type="button" class="icon-btn" :class="{ on: tool === t.id }" :aria-pressed="tool === t.id"
          :aria-label="$t(t.label)" :data-tip="`${$t(t.label)} (${t.key})`" @click="$emit('tool', t.id)"
        ><svg class="ic"><use :href="t.icon" /></svg></button>
        <i v-if="t.sep" class="sep" />
      </template>
      <i class="sep" />
      <button
        v-for="[c, name] in COLORS" :key="c" type="button" class="fe-sw" :class="{ on: color === c }"
        :aria-pressed="color === c" :aria-label="$t(name)" :data-tip="$t(name)" @click="$emit('color', c)"
      ><i :style="{ background: c }" /></button>
      <i class="sep" />
      <button
        v-for="(z, k) in SIZES" :key="z.id" type="button" class="icon-btn fe-dot" :class="{ on: size === z.id }"
        :aria-pressed="size === z.id" :aria-label="$t(z.label)" :data-tip="$t(z.label)" @click="$emit('size', z.id)"
      ><i :style="{ width: `${4 + k * 3}px`, height: `${4 + k * 3}px` }" /></button>
      <i class="sep" />
      <button type="button" class="icon-btn" :aria-label="$t('Rückgängig')" :data-tip="$t('Rückgängig (⌘Z)')" :disabled="!canUndo" @click="$emit('undo')"><svg class="ic"><use href="#i-undo" /></svg></button>
      <button type="button" class="icon-btn" :aria-label="$t('Wiederholen')" :data-tip="$t('Wiederholen (⇧⌘Z)')" :disabled="!canRedo" @click="$emit('redo')"><svg class="ic"><use href="#i-redo" /></svg></button>
      <button type="button" class="icon-btn" :aria-label="$t('Auswahl löschen')" :data-tip="$t('Löschen (⌫)')" :disabled="!canRemove" @click="$emit('remove')"><svg class="ic"><use href="#i-trash" /></svg></button>
      <template v-if="kind === 'image' && page">
        <i class="sep" />
        <label class="fe-scale" :data-tip="$t('Beim Speichern: {w} × {h} px', { w: Math.round(page.w * outScale), h: Math.round(page.h * outScale) })">
          <span>{{ $t('Größe') }}</span>
          <select v-model.number="outScale" :aria-label="$t('Größe beim Speichern')">
            <option v-for="k in SCALES" :key="k" :value="k">{{ k * 100 }} %</option>
          </select>
        </label>
      </template>
    </template>
    <span class="fe-zoom">
      <button type="button" class="icon-btn" :aria-label="$t('Verkleinern')" :data-tip="$t('Verkleinern (⌘−)')" @click="$emit('zoom', -1)"><svg class="ic"><use href="#i-minus" /></svg></button>
      <button type="button" class="fe-pct" :aria-label="$t('Einpassen')" :data-tip="$t('Einpassen (⌘0)')" @click="$emit('fit')">{{ pct }} %</button>
      <button type="button" class="icon-btn" :aria-label="$t('Vergrößern')" :data-tip="$t('Vergrößern (⌘+)')" @click="$emit('zoom', 1)"><svg class="ic"><use href="#i-plus" /></svg></button>
      <template v-if="kind === 'pdf'">
        <i class="sep" />
        <a class="icon-btn" :href="filesUrl(path)" target="_blank" rel="noopener" :aria-label="$t('Im PDF-Viewer des Browsers öffnen')" :data-tip="$t('Im PDF-Viewer öffnen (Suche, Drucken)')"><svg class="ic"><use href="#i-external" /></svg></a>
      </template>
    </span>
  </div>
</template>

<style scoped>
/* wie die Werkzeugleiste des Markdown-Editors */
.fe-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 2px; min-height: 40px; padding: 4px 16px; border-bottom: 1px solid var(--br-subtle); background: var(--bg-0); }
.fe-bar [data-tip]:hover::after { bottom: auto; top: calc(100% + 8px); } /* Tooltip unter der Leiste, nicht über dem Pfad */
.fe-bar .icon-btn.on { background: var(--bg-selected); color: var(--a-blue-hi); }
.fe-bar .icon-btn:disabled { color: var(--tx-disabled); background: none; cursor: default; }
.sep { flex: none; width: 1px; height: 16px; margin: 0 6px; background: var(--br-default); }
.fe-sw { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 50%; transition: box-shadow var(--dur) ease-out; }
.fe-sw i { width: 12px; height: 12px; border-radius: 50%; box-shadow: inset 0 0 0 1px rgb(127 127 127 / .45); }
.fe-sw:hover { box-shadow: 0 0 0 1px var(--br-strong); }
.fe-sw.on { box-shadow: 0 0 0 1.5px var(--tx-primary); }
.fe-dot i { border-radius: 50%; background: currentColor; }
.fe-scale { display: inline-flex; align-items: center; gap: 6px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.fe-scale select { height: 26px; padding: 0 6px; border: 1px solid var(--br-default); border-radius: var(--r-s); background: var(--bg-1); color: var(--tx-primary); font: 400 12px/1 var(--font-ui); font-variant-numeric: tabular-nums; }
.fe-zoom { display: flex; align-items: center; gap: 2px; margin-left: auto; } /* bricht die Leiste um, bleibt der Zoom rechts */
.fe-pct { min-width: 52px; height: 28px; border-radius: var(--r-s); font: 400 12px/1 var(--font-mono); font-variant-numeric: tabular-nums; color: var(--tx-secondary); transition: background var(--dur) ease-out, color var(--dur) ease-out; }
.fe-pct:hover { background: var(--bg-2); color: var(--tx-primary); }
</style>
