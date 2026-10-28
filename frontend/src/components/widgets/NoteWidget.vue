<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { listNotes, readNote, type NoteNode } from '@/api/client'
import { prefs } from '@/lib/prefs'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const text = ref('')
const files = ref<string[]>([])
const missing = ref(false)

// Angepinnte Notiz lesen; ohne Auswahl die Markdown-Dateien zur Wahl anbieten.
watch(
  () => prefs.homeNote,
  async (path) => {
    missing.value = false
    if (path) {
      text.value = await readNote(path).then(
        (n) => n.content,
        () => ((missing.value = true), ''),
      )
      return
    }
    const flat = (ns: NoteNode[]): string[] =>
      ns.flatMap((n) => (n.dir ? flat(n.children ?? []) : n.path.endsWith('.md') ? [n.path] : []))
    files.value = flat(await listNotes().catch(() => []))
      .filter((p) => !p.startsWith('Journal/'))
      .sort()
  },
  { immediate: true },
)

const title = computed(() => prefs.homeNote.split('/').pop()?.replace(/\.md$/, '') ?? '')
// Vorschau ohne Überschriften-, Listen- und Hervorhebungszeichen.
const preview = computed(() =>
  text.value
    .split('\n')
    .map((l) =>
      l
        .replace(/^\s*(#+|[-*+]|\d+\.)\s+/, '')
        .replace(/[*_`]/g, '')
        .trim(),
    )
    .filter((l) => l && l !== title.value)
    .slice(0, 6),
)
</script>

<template>
  <section class="wg wg-note" :aria-label="$t('Angepinnte Notiz')">
    <div class="wg-h">
      <router-link to="/notes"
        ><svg class="ic"><use href="#i-note" /></svg>{{ $t('Angepinnt') }}</router-link
      ><button v-if="prefs.homeNote" type="button" class="wg-link" @click="prefs.homeNote = ''">
        {{ $t('Ändern') }}
      </button>
    </div>
    <template v-if="prefs.homeNote">
      <router-link
        :to="{ path: '/notes', query: { open: prefs.homeNote } }"
        class="wg-title wo-t"
        >{{ title }}</router-link
      >
      <div v-if="missing" class="wg-empty">{{ $t('Notiz nicht gefunden.') }}</div>
      <div v-else class="wo-text">
        <p v-for="(l, i) in preview" :key="i">{{ l }}</p>
      </div>
    </template>
    <select
      v-else
      class="input"
      :aria-label="$t('Notiz wählen')"
      @change="prefs.homeNote = ($event.target as HTMLSelectElement).value"
    >
      <option value="" selected disabled>{{ $t('Notiz wählen') }}</option>
      <option v-for="p in files" :key="p" :value="p">{{ p.replace(/\.md$/, '') }}</option>
    </select>
  </section>
</template>

<style scoped>
.wo-t {
  text-decoration: none;
}
.wo-t:hover {
  text-decoration: underline;
}
/* Spalten-Umbruch: Zeilen, die nicht mehr ganz passen, verschwinden rechts statt halb abgeschnitten */
.wo-text {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-flow: column wrap;
  column-gap: 40px;
  font: 400 12.5px/1.5 var(--font-ui);
  color: var(--tx-secondary);
}
.wo-text p {
  width: 100%;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
