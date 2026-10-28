<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { normalizeTitle } from '@/lib/journal'
import { vDialog } from '@/lib/dialog'

interface Candidate {
  title: string
  selected: boolean
  done: boolean
}

const props = defineProps<{
  open: boolean
  dueLabel: string
  points: string[]
  importedTitles: string[]
  importing: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [titles: string[]]
}>()

const candidates = ref<Candidate[]>([])

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      const doneSet = new Set(props.importedTitles.map(normalizeTitle))
      candidates.value = props.points
        .map((p) => p.trim())
        .filter((p) => !!p)
        .map((title) => {
          const done = doneSet.has(normalizeTitle(title))
          return { title, selected: !done, done }
        })
    }
  },
  { immediate: true },
)

const selectedTitles = computed(() =>
  candidates.value
    .filter((c) => c.selected && !c.done && c.title.trim())
    .map((c) => c.title.trim()),
)

function onSubmit() {
  if (selectedTitles.value.length) {
    emit('submit', selectedTitles.value)
  }
}
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <form
      class="dialog je-import-dialog"
      aria-labelledby="je-import-title"
      @submit.prevent="onSubmit"
    >
      <div class="dlg-head">
        <div>
          <h3 id="je-import-title">{{ $t('Als Aufgaben übernehmen') }}</h3>
          <div class="v-sub">{{ $t('Fällig am {date}', { date: dueLabel }) }}</div>
        </div>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <p class="v-sub">{{ $t('Wähle die Aufgaben aus und passe Titel bei Bedarf an.') }}</p>
        <div class="je-task-candidates">
          <label
            v-for="(candidate, i) in candidates"
            :key="i"
            class="je-task-candidate"
            :class="{ done: candidate.done }"
          >
            <input
              v-model="candidate.selected"
              type="checkbox"
              :disabled="candidate.done"
              :aria-label="$t('Aufgabe übernehmen: {title}', { title: candidate.title })"
            />
            <input
              v-model="candidate.title"
              class="input"
              type="text"
              :disabled="candidate.done"
              :aria-label="$t('Aufgabentitel {n}', { n: i + 1 })"
            />
            <span v-if="candidate.done" class="je-done-mark">{{ $t('Übernommen') }}</span>
          </label>
        </div>
      </div>
      <div class="dlg-foot">
        <button class="btn btn-ghost" type="button" :disabled="importing" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          :disabled="importing || !selectedTitles.length"
        >
          {{ importing ? $t('Wird angelegt …') : $t('Aufgaben anlegen') }}
        </button>
      </div>
    </form>
  </div>
</template>
