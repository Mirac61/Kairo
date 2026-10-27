<script setup lang="ts">
import { navItems } from '@/router'
import { vDialog } from '@/lib/dialog'
import { GOTO, SHORTCUTS } from '@/lib/shortcuts'

defineEmits<{ close: [] }>()
const label = (path: string) => navItems.find((n) => n.path === path)?.label ?? 'Einstellungen'
</script>

<template>
  <div v-dialog="() => $emit('close')" class="overlay open" @mousedown.self="$emit('close')">
    <div class="dialog sh" aria-labelledby="sh-title">
      <div class="dlg-head">
        <h3 id="sh-title">{{ $t('Tastenkürzel') }}</h3>
        <button type="button" class="icon-btn" :aria-label="$t('Schließen')" @click="$emit('close')"><svg class="ic"><use href="#i-x" /></svg></button>
      </div>
      <div class="dlg-body sh-body">
        <section v-for="g in SHORTCUTS" :key="g.title">
          <h4>{{ $t(g.title) }}</h4>
          <dl>
            <template v-for="[k, l] in g.keys" :key="k"><dt><kbd class="key">{{ $t(k) }}</kbd></dt><dd>{{ $t(l) }}</dd></template>
          </dl>
        </section>
        <section>
          <h4>{{ $t('Springen mit g') }}</h4>
          <dl>
            <template v-for="(path, k) in GOTO" :key="k"><dt><kbd class="key">g {{ k }}</kbd></dt><dd>{{ $t(label(path)) }}</dd></template>
          </dl>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sh { width: min(720px, calc(100vw - 32px)); }
.sh-body { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 20px 28px; max-height: 70vh; overflow-y: auto; }
h4 { margin-bottom: 8px; font: 500 12px/1.3 var(--font-ui); color: var(--tx-muted); }
dl { display: grid; grid-template-columns: auto 1fr; gap: 6px 10px; align-items: baseline; }
dd { font: 400 13px/1.4 var(--font-ui); color: var(--tx-secondary); }
dt { text-align: right; }
</style>
