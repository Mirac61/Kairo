<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ApiError, createNote, errorMessage, listNotes, readNote, saveNote } from '@/api/client'
import { addDays, ymd } from '@/lib/dates'
import { journalPath, parseEntry, QUESTIONS, toMarkdown } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'

const todayStr = ymd(new Date())
const date = ref(todayStr)
const answers = ref<string[]>(QUESTIONS.map(() => ''))
let other = ''
const mtime = ref('') // leer: Eintrag gibt es noch nicht
const saved = ref('') // Stand beim Laden/Speichern, um ungespeicherte Änderungen zu erkennen
const dirty = computed(() => answers.value.join('\u0000') !== saved.value)
const error = ref('')
const status = ref('')
const days = ref<string[]>([]) // Tage mit Eintrag, neueste zuerst

const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }))
const shortFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))

async function loadList() {
  const dir = (await listNotes()).find((n) => n.dir && n.path === 'Journal')
  days.value = (dir?.children ?? [])
    .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
    .filter((d): d is string => !!d)
    .sort()
    .reverse()
}

async function loadEntry() {
  status.value = ''
  try {
    const n = await readNote(journalPath(date.value))
    const e = parseEntry(n.content)
    answers.value = e.answers
    other = e.other
    mtime.value = n.mtime
    error.value = ''
  } catch (e) {
    answers.value = QUESTIONS.map(() => '')
    other = ''
    mtime.value = ''
    error.value = e instanceof ApiError && e.status === 404 ? '' : errorMessage(e)
  }
  saved.value = answers.value.join('\u0000')
}

async function save() {
  const path = journalPath(date.value)
  try {
    if (!mtime.value) {
      await createNote('Journal', true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e })
      mtime.value = (await createNote(path)).mtime ?? ''
    }
    mtime.value = (await saveNote(path, toMarkdown(date.value, { answers: answers.value, other }), mtime.value)).mtime
    saved.value = answers.value.join('\u0000')
    status.value = t('Gespeichert')
    error.value = ''
    await loadList()
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 409
      ? t('Der Eintrag wurde inzwischen woanders geändert. Lade ihn neu, bevor du speicherst.')
      : errorMessage(e)
  }
}

// Ungespeichertes geht beim Tageswechsel nicht verloren: vorher speichern.
async function go(d: string) {
  if (d > todayStr || d === date.value) return
  if (dirty.value) await save()
  date.value = d
}
const shift = (n: number) => go(ymd(addDays(new Date(date.value + 'T00:00'), n)))

watch(date, loadEntry)
onMounted(() => {
  void loadEntry()
  void loadList().catch((e) => (error.value = errorMessage(e)))
})

function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    void save()
  }
}
</script>

<template>
  <div class="view-inner" @keydown="onKey">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">{{ $t('Journal') }}</h1>
        <div class="v-sub">{{ fmt(dayFmt, date) }}</div>
      </div>
      <div class="jr-nav">
        <button class="icon-btn" type="button" :aria-label="$t('Vorheriger Tag')" @click="shift(-1)"><svg class="ic"><use href="#i-left" /></svg></button>
        <button class="btn btn-ghost" type="button" :disabled="date === todayStr" @click="go(todayStr)">{{ $t('Heute') }}</button>
        <button class="icon-btn" type="button" :aria-label="$t('Nächster Tag')" :disabled="date === todayStr" @click="shift(1)"><svg class="ic"><use href="#i-right" /></svg></button>
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <div class="jr-layout">
      <form class="jr-form" @submit.prevent="save">
        <div v-for="(q, i) in QUESTIONS" :key="q" class="field">
          <label :for="`jr-${i}`">{{ $t(q) }}</label>
          <textarea :id="`jr-${i}`" v-model="answers[i]" class="input jr-text" rows="4"></textarea>
        </div>
        <div class="jr-foot">
          <span class="v-sub" role="status">{{ dirty ? $t('Nicht gespeichert') : status }}</span>
          <button class="btn btn-primary" type="submit" :disabled="!dirty">{{ $t('Speichern') }}</button>
        </div>
      </form>

      <aside v-if="days.length" class="jr-days" :aria-label="$t('Frühere Einträge')">
        <h2 class="col-title">{{ $t('Frühere Einträge') }}</h2>
        <button v-for="d in days" :key="d" type="button" class="jr-day" :aria-current="d === date ? 'date' : undefined" @click="go(d)">
          {{ fmt(shortFmt, d) }}
        </button>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.jr-nav { display: flex; align-items: center; gap: 4px; }
.jr-layout { display: grid; grid-template-columns: minmax(0, 1fr) 200px; gap: 32px; align-items: start; }
.jr-form { display: grid; gap: 16px; max-width: 720px; }
.jr-text { resize: vertical; min-height: 96px; font: 400 14px/1.5 var(--font-ui); }
.jr-foot { display: flex; align-items: center; justify-content: flex-end; gap: 12px; }
.jr-days { display: grid; gap: 2px; }
.jr-days .col-title { margin-bottom: 8px; }
.jr-day { padding: 6px 8px; border: 0; border-radius: var(--r-s); background: none; color: var(--tx-secondary); font: 400 13px/1.3 var(--font-ui); text-align: left; cursor: pointer; }
.jr-day:hover { color: var(--tx-primary); background: var(--bg-1); }
.jr-day[aria-current] { color: var(--tx-primary); font-weight: 600; }
@media (max-width: 760px) {
  .jr-layout { grid-template-columns: 1fr; }
}
</style>
