<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ApiError, createNote, errorMessage, importJournalTasks, readNote, saveNote, type JournalTaskImport } from '@/api/client'
import { journalPath, normalizeTitle, parseEntry, splitPoints, toMarkdown } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'
import { vDialog } from '@/lib/dialog'

export interface JournalEntryData {
  answers: string[]
  other: string
  mtime: string
}

const props = defineProps<{
  date: string
  entry: JournalEntryData | null
  importedTitles: string[]
}>()

const emit = defineEmits<{
  saved: [entry: JournalEntryData]
  imported: [result: JournalTaskImport]
}>()

const HINTS = computed(() => [t('Etwas, das heute geklappt hat, auch Kleines'), t('Was würdest du beim nächsten Mal anders machen?'), t('Punkt …')])

const q0 = ref(props.entry?.answers[0] ?? '')
const q1 = ref(props.entry?.answers[1] ?? '')
const initPoints = splitPoints(props.entry?.answers[2] ?? '')
const points = ref<string[]>(initPoints.length ? initPoints : [''])
let other = props.entry?.other ?? ''
let mtime = props.entry?.mtime ?? ''

type Status = 'idle' | 'saving' | 'saved' | 'conflict' | 'error'
const status = ref<Status>('idle')
const dirty = ref(false)
const error = ref('')
// Ein Speichervorgang läuft; weitere Änderungen währenddessen werden in der Schleife nachgeholt.
let saving: Promise<void> | null = null
let queued = false
let alive = true
// Was zuletzt geschrieben wurde: flush() schreibt nur, wenn sich seither etwas geändert hat.
let version = 0
let savedVersion = 0
let timer: ReturnType<typeof setTimeout> | undefined
let savedTimer: ReturnType<typeof setTimeout> | undefined
const isDirty = () => version !== savedVersion

// Feld zum Einfügen eines neuen Punkts; ein Punkt bleibt immer übrig.
const pointList = ref<HTMLElement>()
const formEl = ref<HTMLFormElement>()
const canImport = computed(() => points.value.some((p) => p.trim()))

function focusPoint(i: number) {
  void nextTick(() => {
    const inputs = pointList.value?.querySelectorAll('input') as NodeListOf<HTMLInputElement> | undefined
    inputs?.[i]?.focus()
  })
}

function grow(el: HTMLTextAreaElement) {
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}

// Beim Öffnen und beim Drehen des Geräts passen sich die Felder ihrem Inhalt an.
function growAll() {
  formEl.value?.querySelectorAll<HTMLTextAreaElement>('.je-area').forEach(grow)
}

function onChange() {
  version++
  dirty.value = true
  status.value = 'idle'
  error.value = ''
  if (timer) clearTimeout(timer)
  if (savedTimer) {
    clearTimeout(savedTimer)
    savedTimer = undefined
  }
  timer = setTimeout(() => { timer = undefined; void save() }, 800)
}

// Punkte: v-model pro Zeile, Enter teilt am Cursor, Backspace auf leerer Zeile entfernt den Punkt.
function setPoint(i: number, v: string) {
  points.value[i] = v
  onChange()
}

function onPointKey(i: number, e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    const el = e.target as HTMLInputElement
    const caret = el.selectionStart ?? el.value.length
    const before = el.value.slice(0, caret)
    const after = el.value.slice(caret)
    points.value[i] = before
    points.value.splice(i + 1, 0, after)
    onChange()
    focusPoint(i + 1)
  } else if (e.key === 'Backspace' && points.value[i] === '' && points.value.length > 1) {
    e.preventDefault()
    points.value.splice(i, 1)
    onChange()
    focusPoint(Math.max(0, i - 1))
  }
}

function conflict(): false {
  if (alive) {
    status.value = 'conflict'
    error.value = t('Der Eintrag wurde außerhalb von Kairo geändert und nicht überschrieben.')
  }
  return false
}

// Schreibt einmal; false bei Konflikt oder Fehler, damit die Schleife nicht weiterläuft.
async function saveOnce(): Promise<boolean> {
  if (alive) status.value = 'saving'
  const v = version
  const answers = [q0.value, q1.value, points.value.join('\n')]
  try {
    // Vor dem Schreiben prüfen, ob die Datei seit dem Laden außerhalb geändert wurde.
    if (mtime) {
      try {
        const current = await readNote(journalPath(props.date))
        if (current.mtime !== mtime) return conflict()
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 404)) throw e
        return conflict()
      }
    } else {
      try {
        await readNote(journalPath(props.date))
        return conflict()
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 404)) throw e
      }
    }

    let nextMtime = mtime
    if (!nextMtime) {
      await createNote('Journal', true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e })
      nextMtime = (await createNote(journalPath(props.date))).mtime ?? ''
    }
    nextMtime = (await saveNote(journalPath(props.date), toMarkdown(props.date, { answers, other }), nextMtime)).mtime
    mtime = nextMtime
    savedVersion = v
    if (v === version) dirty.value = false
    // Nach dem Unmount wird noch geschrieben, aber nichts mehr angezeigt oder gemeldet.
    if (!alive) return true
    status.value = 'saved'
    error.value = ''
    emit('saved', { answers, other, mtime })
    if (savedTimer) clearTimeout(savedTimer)
    savedTimer = setTimeout(() => {
      savedTimer = undefined
      if (alive && status.value === 'saved') status.value = 'idle'
    }, 2000)
    return true
  } catch (e) {
    if (!alive) return false
    if (e instanceof ApiError && e.status === 409) {
      conflict()
    } else {
      status.value = 'error'
      error.value = errorMessage(e)
    }
    return false
  }
}

async function drain() {
  do {
    queued = false
    if (!(await saveOnce())) return
  } while (queued)
}

// Ein Vorgang zur Zeit; kommt währenddessen eine Änderung, läuft er danach noch einmal.
function save(): Promise<void> {
  if (saving) {
    queued = true
    return saving
  }
  saving = drain().finally(() => { saving = null })
  return saving
}

// Vor dem Wechsel eines Eintrags oder beim Verlassen der Seite: Timer stoppen und fertig schreiben.
async function flush() {
  if (timer) {
    clearTimeout(timer)
    timer = undefined
  }
  if (!isDirty()) {
    if (saving) await saving
    return
  }
  await save()
}
defineExpose({ flush })

async function reload() {
  try {
    const n = await readNote(journalPath(props.date))
    const e = parseEntry(n.content)
    q0.value = e.answers[0] ?? ''
    q1.value = e.answers[1] ?? ''
    const ps = splitPoints(e.answers[2] ?? '')
    points.value = ps.length ? ps : ['']
    other = e.other
    mtime = n.mtime
    status.value = 'idle'
    error.value = ''
    void nextTick(growAll)
  } catch (e) {
    if (!(e instanceof ApiError && e.status === 404)) {
      status.value = 'error'
      error.value = errorMessage(e)
      return
    }
    q0.value = ''
    q1.value = ''
    points.value = ['']
    other = ''
    mtime = ''
    status.value = 'idle'
    error.value = ''
    void nextTick(growAll)
  }
}

onMounted(() => {
  window.addEventListener('resize', growAll)
  void nextTick(growAll)
})
onBeforeUnmount(() => {
  alive = false
  if (savedTimer) clearTimeout(savedTimer)
  window.removeEventListener('resize', growAll)
  // Die Seite wird verlassen; die Anfrage läuft als SPA-Navigation weiter.
  void flush()
})

// ---- Aufgaben übernehmen ----
const importOpen = ref(false)
const candidates = ref<{ title: string; selected: boolean; done: boolean }[]>([])
const importing = ref(false)
const selectedTitles = computed(() => candidates.value.filter((c) => c.selected && !c.done && c.title.trim()).map((c) => c.title.trim()))
const importedCount = computed(() => props.importedTitles.length)
const importDueLabel = computed(() => {
  const [year = 0, month = 1, day = 1] = props.date.split('-').map(Number)
  const date = new Date(Date.UTC(year, month - 1, day + 1))
  return new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }).format(date)
})

function openImport() {
  candidates.value = points.value
    .map((p) => {
      const title = p.trim()
      if (!title) return null
      const done = props.importedTitles.some((t) => normalizeTitle(t) === normalizeTitle(title))
      return { title, selected: !done, done }
    })
    .filter((c): c is { title: string; selected: boolean; done: boolean } => c !== null)
  importOpen.value = true
}

function closeImport() {
  if (importing.value) return
  importOpen.value = false
  candidates.value = []
}

async function submitImport() {
  const titles = selectedTitles.value
  if (!titles.length || importing.value) return
  importing.value = true
  try {
    const result = await importJournalTasks(props.date, titles)
    emit('imported', result)
    importOpen.value = false
    candidates.value = []
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="je">
    <div v-if="status === 'conflict'" class="je-conflict" role="alert">
      <div>
        <b>{{ $t('Außerhalb geändert') }}</b>
        <span>{{ error }}</span>
      </div>
      <button class="btn btn-ghost" type="button" @click="reload">{{ $t('Neu laden') }}</button>
    </div>
    <div v-else-if="status === 'error'" class="badge" role="alert">{{ error }}</div>

    <form ref="formEl" class="je-form" @submit.prevent>
      <section class="je-q">
        <label class="je-label" for="je-a">{{ $t('Was lief gut?') }}</label>
        <textarea id="je-a" v-model="q0" class="je-area" rows="1" :placeholder="HINTS[0]" @input="grow($event.target as HTMLTextAreaElement); onChange()"></textarea>
      </section>

      <section class="je-q">
        <label class="je-label" for="je-b">{{ $t('Was lerne ich daraus?') }}</label>
        <textarea id="je-b" v-model="q1" class="je-area" rows="1" :placeholder="HINTS[1]" @input="grow($event.target as HTMLTextAreaElement); onChange()"></textarea>
      </section>

      <section class="je-q">
        <label class="je-label" for="je-c">{{ $t('Was ist morgen wichtig?') }}</label>
        <div ref="pointList" class="je-points">
          <div v-for="(p, i) in points" :key="i" class="je-point">
            <span class="je-bullet" aria-hidden="true"></span>
            <input
              :id="i === 0 ? 'je-c' : undefined"
              class="je-point-input"
              :value="p"
              :placeholder="HINTS[2]"
              :aria-label="i === 0 ? $t('Was ist morgen wichtig?') : $t('Punkt {n}', { n: i + 1 })"
              @input="setPoint(i, ($event.target as HTMLInputElement).value)"
              @keydown="onPointKey(i, $event)"
            />
          </div>
        </div>
        <div class="je-foot">
          <button v-if="canImport" type="button" class="btn btn-ghost je-import" @click="openImport">
            <svg class="ic"><use href="#i-tasks" /></svg>{{ $t('Als Aufgaben übernehmen') }}
          </button>
          <span v-if="importedCount" class="je-imported">✓ {{ $tn(importedCount, '{n} Aufgabe übernommen', '{n} Aufgaben übernommen') }}</span>
          <span class="je-status" aria-live="polite">
            <span v-if="dirty" class="save-status dirty"><i></i>{{ $t('Ungespeicherte Änderungen') }}</span>
            <span v-else-if="status === 'saving'" class="save-status">{{ $t('Wird gespeichert …') }}</span>
            <span v-else class="save-status saved je-saved"><svg class="ic"><use href="#i-check" /></svg>{{ $t('Gespeichert') }}</span>
          </span>
        </div>
      </section>
    </form>

    <div v-if="importOpen" v-dialog="closeImport" class="overlay open" @mousedown.self="closeImport">
      <form class="dialog je-import-dialog" aria-labelledby="je-import-title" @submit.prevent="submitImport">
        <div class="dlg-head">
          <div>
            <h3 id="je-import-title">{{ $t('Als Aufgaben übernehmen') }}</h3>
            <div class="v-sub">{{ $t('Fällig am {date}', { date: importDueLabel }) }}</div>
          </div>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="closeImport"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <p class="v-sub">{{ $t('Wähle die Aufgaben aus und passe Titel bei Bedarf an.') }}</p>
          <div class="je-task-candidates">
            <label v-for="(candidate, i) in candidates" :key="i" class="je-task-candidate" :class="{ done: candidate.done }">
              <input v-model="candidate.selected" type="checkbox" :disabled="candidate.done" :aria-label="$t('Aufgabe übernehmen: {title}', { title: candidate.title })" />
              <input v-model="candidate.title" class="input" type="text" :disabled="candidate.done" :aria-label="$t('Aufgabentitel {n}', { n: i + 1 })" />
              <span v-if="candidate.done" class="je-done-mark">{{ $t('Übernommen') }}</span>
            </label>
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" :disabled="importing" @click="closeImport">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit" :disabled="importing || !selectedTitles.length">
            {{ importing ? $t('Wird angelegt …') : $t('Aufgaben anlegen') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.je-form { display: grid; gap: 24px; }
.je-q { display: grid; gap: 8px; padding: 0; border-top: 0; }
.je-label {
  font: italic 400 20px/1.3 var(--font-serif);
  color: var(--tx-secondary);
  letter-spacing: normal;
  text-transform: none;
}
.je-area {
  width: 100%; min-height: 40px; padding: 4px 0 6px; border: 0; border-bottom: 1px solid transparent;
  outline: none; resize: none; overflow: hidden; background: none; color: var(--tx-primary);
  font: 400 16px/1.7 var(--font-ui); transition: border-color var(--dur) ease-out;
}
.je-area:focus { border-bottom-color: var(--br-default); }
.je-area:focus-visible { outline: 2px solid var(--ring); outline-offset: 4px; border-radius: var(--r-s); }
.je-area::placeholder { color: var(--tx-muted); opacity: 1; }

.je-points { display: grid; gap: 4px; }
.je-point { display: flex; align-items: center; gap: 12px; }
.je-bullet {
  flex: none; width: 5px; height: 5px; border-radius: 50%;
  background: var(--p-orange); opacity: 0.75;
}
.je-point-input {
  flex: 1; min-width: 0; height: auto; padding: 4px 0 6px; border: 0; border-bottom: 1px solid transparent;
  outline: none; background: none; color: var(--tx-primary); font: 400 16px/1.7 var(--font-ui);
  transition: border-color var(--dur) ease-out;
}
.je-point-input:focus { border-bottom-color: var(--br-default); }
.je-point-input:focus-visible { outline: 2px solid var(--ring); outline-offset: 4px; border-radius: var(--r-s); }
.je-point-input::placeholder { color: var(--tx-muted); opacity: 1; }

.je-foot { display: flex; align-items: center; gap: 16px; margin-top: 8px; }
.je-import { height: 28px; padding: 0 10px; font-size: 13px; color: var(--tx-secondary); }
.je-import .ic { width: 14px; height: 14px; }
.je-imported { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.je-status { margin-left: auto; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.je-saved { display: inline-flex; align-items: center; gap: 5px; color: var(--tx-muted); }
.je-saved .ic { width: 13px; height: 13px; }

.je-conflict {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  margin-bottom: 16px; padding: 12px 14px; border: 1px solid var(--br-default); border-radius: var(--r-m);
  background: var(--bg-1);
}
.je-conflict div { display: grid; gap: 2px; }
.je-conflict b { font: 600 13px/1.3 var(--font-ui); color: var(--tx-primary); }
.je-conflict span { font: 400 12px/1.4 var(--font-ui); color: var(--tx-secondary); }

.je-import-dialog { max-width: 520px; }
.je-import-dialog .dlg-head .v-sub { margin-top: 6px; }
.je-import-dialog .dlg-body > .v-sub { margin: 0; }
.je-task-candidates { display: grid; gap: 8px; max-height: 42vh; overflow-y: auto; }
.je-task-candidate { display: grid; grid-template-columns: 18px minmax(0, 1fr) auto; align-items: center; gap: 10px; }
.je-task-candidate input[type='checkbox'] { margin: 0; }
.je-task-candidate.done .input { color: var(--tx-muted); text-decoration: line-through; }
.je-done-mark { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); white-space: nowrap; }
</style>
