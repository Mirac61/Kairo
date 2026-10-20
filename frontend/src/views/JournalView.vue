<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { ApiError, createNote, errorMessage, listNotes, readNote, saveNote } from '@/api/client'
import { addDays, ymd } from '@/lib/dates'
import { journalPath, parseEntry, QUESTIONS, toMarkdown } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'
import { useShortcuts } from '@/composables/useShortcuts'

// Übersicht der letzten Einträge; Schreiben in einem eigenen Editor.
const RECENT = 30
const HINTS = [t('Etwas, das heute geklappt hat, auch Kleines'), t('Was würdest du beim nächsten Mal anders machen?'), t('Die eine Sache, die morgen zählt')]

interface Row { date: string; answers: string[] }

const todayStr = ymd(new Date())
const rows = ref<Row[]>([])
const total = ref(0)
const loaded = ref(false)
const error = ref('')

// Editor
const editing = ref<string | null>(null) // Datum des Eintrags im Editor
const answers = ref<string[]>(QUESTIONS.map(() => ''))
let other = ''
let mtime = '' // leer: Eintrag gibt es noch nicht
const saved = ref('')
const dirty = computed(() => answers.value.join('\u0000') !== saved.value)
const saving = ref(false)
const fields = ref<HTMLTextAreaElement[]>([])

const longFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }))
const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))
const label = (d: string) => (d === todayStr ? t('Heute') : d === ymd(addDays(new Date(), -1)) ? t('Gestern') : fmt(dayFmt.value, d))

const hasToday = computed(() => rows.value[0]?.date === todayStr)
// Tage in Folge mit Eintrag, bis heute oder (falls heute noch fehlt) bis gestern.
const streak = computed(() => {
  const have = new Set(rows.value.map((r) => r.date))
  let d = new Date()
  if (!have.has(todayStr)) d = addDays(d, -1)
  let n = 0
  while (have.has(ymd(d))) {
    n++
    d = addDays(d, -1)
  }
  return n
})

// shortcut: liest die letzten RECENT Einträge einzeln; für mehr bräuchte es Blättern oder einen Endpunkt.
async function load() {
  try {
    const dir = (await listNotes()).find((n) => n.dir && n.path === 'Journal')
    const dates = (dir?.children ?? [])
      .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
      .filter((d): d is string => !!d)
      .sort()
      .reverse()
    total.value = dates.length
    rows.value = await Promise.all(dates.slice(0, RECENT).map(async (date) => ({
      date, answers: parseEntry((await readNote(journalPath(date))).content).answers,
    })))
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)

async function open(date: string) {
  try {
    const n = await readNote(journalPath(date))
    const e = parseEntry(n.content)
    answers.value = e.answers
    other = e.other
    mtime = n.mtime
  } catch (e) {
    if (!(e instanceof ApiError && e.status === 404)) {
      error.value = errorMessage(e)
      return
    }
    answers.value = QUESTIONS.map(() => '')
    other = ''
    mtime = ''
  }
  saved.value = answers.value.join('\u0000')
  editing.value = date
  error.value = ''
  await nextTick()
  fields.value.forEach(grow)
  fields.value[answers.value.findIndex((a) => !a.trim())]?.focus()
}

function close() {
  if (dirty.value && !confirm(t('Ungespeicherte Änderungen verwerfen?'))) return
  editing.value = null
}

async function save() {
  const date = editing.value
  if (!date || saving.value) return
  saving.value = true
  const path = journalPath(date)
  try {
    if (!mtime) {
      await createNote('Journal', true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e })
      mtime = (await createNote(path)).mtime ?? ''
    }
    mtime = (await saveNote(path, toMarkdown(date, { answers: answers.value, other }), mtime)).mtime
    saved.value = answers.value.join('\u0000')
    editing.value = null
    await load()
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 409
      ? t('Der Eintrag wurde inzwischen woanders geändert. Lade ihn neu, bevor du speicherst.')
      : errorMessage(e)
  }
  saving.value = false
}

// Textfelder wachsen mit dem Text.
function grow(el: HTMLTextAreaElement) {
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}

function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    void save()
  } else if (e.key === 'Escape') {
    close()
  }
}
// n: heute schreiben (nur in der Übersicht).
useShortcuts((e) => {
  if (e.key === 'n' && !editing.value) {
    e.preventDefault()
    void open(todayStr)
  }
})
</script>

<template>
  <div class="view-inner">
    <!-- Editor -->
    <template v-if="editing">
      <button type="button" class="col-link jr-back" @click="close"><svg class="ic"><use href="#i-left" /></svg>{{ $t('Alle Einträge') }}</button>
      <div class="v-head">
        <h1 class="v-title">{{ label(editing) }}</h1>
        <div class="v-sub">{{ fmt(longFmt, editing) }}</div>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <form class="card jr-editor" @submit.prevent="save" @keydown="onKey">
        <section v-for="(q, i) in QUESTIONS" :key="q" class="jr-q">
          <label :for="`jr-${i}`" class="jr-q-head"><span class="jr-num">{{ i + 1 }}</span>{{ $t(q) }}</label>
          <textarea :id="`jr-${i}`" :ref="(el) => el && (fields[i] = el as HTMLTextAreaElement)" v-model="answers[i]" class="jr-area" rows="2"
            :placeholder="HINTS[i]" @input="grow($event.target as HTMLTextAreaElement)"></textarea>
        </section>
        <div class="jr-foot">
          <span class="jr-keys"><kbd>⌘S</kbd> {{ $t('speichern') }} · <kbd>Esc</kbd> {{ $t('zurück') }}</span>
          <button class="btn btn-ghost" type="button" @click="close">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit" :disabled="!dirty || saving">{{ $t('Speichern') }}</button>
        </div>
      </form>
    </template>

    <!-- Übersicht -->
    <template v-else>
      <div class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ $t('Journal') }}</h1>
          <div class="v-sub">
            {{ $tn(total, '{n} Eintrag', '{n} Einträge') }}<template v-if="streak > 1"> · {{ $t('{n} Tage in Folge', { n: streak }) }}</template>
          </div>
        </div>
        <button class="btn btn-primary" type="button" aria-keyshortcuts="n" @click="open(todayStr)">
          <svg class="ic"><use :href="hasToday ? '#i-journal' : '#i-plus'" /></svg>{{ hasToday ? $t('Heute weiterschreiben') : $t('Heute schreiben') }}
        </button>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div v-if="loaded && !rows.length" class="card jr-empty">
        <p class="jr-empty-t">{{ $t('Drei kurze Fragen am Abend.') }}</p>
        <p class="v-sub">{{ $t('Was lief gut, was lernst du daraus, was ist morgen wichtig. Zwei Minuten reichen.') }}</p>
        <button class="btn btn-primary" type="button" @click="open(todayStr)">{{ $t('Ersten Eintrag schreiben') }}</button>
      </div>

      <div v-else-if="rows.length" class="card jr-table" role="table" :aria-label="$t('Letzte Einträge')">
        <div class="jr-row jr-th" role="row">
          <span role="columnheader">{{ $t('Tag') }}</span>
          <span v-for="q in QUESTIONS" :key="q" role="columnheader">{{ $t(q) }}</span>
        </div>
        <button v-for="r in rows" :key="r.date" type="button" class="jr-row" role="row" @click="open(r.date)">
          <span class="jr-date" role="cell"><b>{{ label(r.date) }}</b></span>
          <span v-for="(a, i) in r.answers" :key="i" class="jr-cell" role="cell" :data-q="$t(QUESTIONS[i]!)">
            <template v-if="a">{{ a }}</template><i v-else>–</i>
          </span>
        </button>
      </div>
      <p v-if="total > rows.length" class="v-sub jr-more">{{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}</p>
    </template>
  </div>
</template>

<style scoped>
/* Übersicht */
.jr-table { padding: 0; overflow: hidden; }
.jr-row {
  display: grid; grid-template-columns: 120px repeat(3, minmax(0, 1fr)); gap: 20px; align-items: start;
  width: 100%; padding: 14px 18px; border: 0; border-top: 1px solid var(--br-default); background: none;
  color: inherit; font: inherit; text-align: left; cursor: pointer;
}
.jr-row:first-child { border-top: 0; }
button.jr-row:hover { background: var(--bg-hover); }
.jr-th { cursor: default; padding-top: 12px; padding-bottom: 12px; font: 500 12px/1.3 var(--font-ui); color: var(--tx-muted); background: var(--bg-2); }
.jr-date b { font: 600 14px/1.4 var(--font-ui); color: var(--tx-primary); }
.jr-cell {
  font: 400 13px/1.5 var(--font-ui); color: var(--tx-secondary); white-space: pre-line;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.jr-cell i { font-style: normal; color: var(--tx-disabled); }
.jr-more { margin-top: 12px; }
.jr-empty { display: grid; justify-items: start; gap: 8px; max-width: 560px; padding: 28px; }
.jr-empty p { margin: 0; }
.jr-empty-t { font: 600 17px/1.3 var(--font-ui); color: var(--tx-primary); }
.jr-empty .btn { margin-top: 8px; }

/* Editor */
.jr-back { display: inline-flex; align-items: center; gap: 4px; margin-bottom: 12px; padding: 0; border: 0; background: none; cursor: pointer; }
.jr-editor { max-width: 720px; padding: 8px 0 0; }
.jr-editor, .jr-empty { margin-top: 4px; }
.jr-q { display: grid; gap: 8px; padding: 20px 28px; border-bottom: 1px solid var(--br-default); }
.jr-q-head { display: flex; align-items: center; gap: 12px; font: 600 16px/1.3 var(--font-ui); color: var(--tx-primary); }
.jr-num {
  display: grid; place-items: center; flex: none; width: 24px; height: 24px; border-radius: 50%;
  font: 600 12px/1 var(--font-ui); color: var(--a-blue); background: color-mix(in srgb, var(--a-blue) 16%, transparent);
}
.jr-area {
  width: 100%; min-height: 52px; padding: 4px 0 0 36px; border: 0; outline: none; resize: none; overflow: hidden;
  background: none; color: var(--tx-primary); font: 400 15px/1.6 var(--font-ui);
}
.jr-area::placeholder { color: var(--tx-disabled); }
.jr-q:focus-within .jr-num { color: var(--bg-0); background: var(--a-blue); }
.jr-foot { display: flex; align-items: center; gap: 8px; padding: 14px 20px; }
.jr-keys { flex: 1; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.jr-keys kbd { font: 400 11px/1 var(--font-mono); }

@media (max-width: 760px) {
  .jr-th { display: none; }
  .jr-row { grid-template-columns: 1fr; gap: 6px; padding: 14px 16px; }
  .jr-cell { -webkit-line-clamp: 2; }
  .jr-cell::before { content: attr(data-q) ' '; color: var(--tx-muted); font-weight: 500; }
  .jr-cell:has(i) { display: none; }
  .jr-q { padding: 16px; }
  .jr-area { padding-left: 0; }
  .jr-keys { display: none; }
}
</style>
