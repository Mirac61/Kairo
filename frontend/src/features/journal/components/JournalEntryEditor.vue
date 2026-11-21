<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ApiError,
  createNote,
  errorMessage,
  importJournalTasks,
  readNote,
  saveNote,
  type JournalTaskImport,
} from '@/api/client'
import { journalPath, parseEntry, splitPoints, toMarkdown } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'
import JournalImportDialog from './JournalImportDialog.vue'

export interface JournalEntryData {
  answers: string[]
  other: string
  mtime: string
  markdown?: string
}

const props = defineProps<{
  date: string
  entry: JournalEntryData | null
  importedTitles: string[]
  mode?: 'write' | 'markdown' | 'read'
}>()

const emit = defineEmits<{
  saved: [entry: JournalEntryData]
  imported: [result: JournalTaskImport]
}>()

const HINTS = computed(() => [
  t('Etwas, das heute geklappt hat, auch Kleines'),
  t('Was würdest du beim nächsten Mal anders machen?'),
  t('Punkt …'),
  t('Gedanken, Markdown, [[Notizen]] oder eigene ## Abschnitte …'),
])

function unwrapOther(rawOther: string): string {
  return rawOther.replace(/^##\s+Freie Notizen\s*\n+/i, '').trim()
}

function wrapOther(text: string): string {
  const trimmed = text.trim()
  if (!trimmed) return ''
  if (/^##\s/.test(trimmed)) return trimmed
  return `## Freie Notizen\n\n${trimmed}`
}

const q0 = ref(props.entry?.answers[0] ?? '')
const q1 = ref(props.entry?.answers[1] ?? '')
// Punkte als eine Markdown-Liste: eine Zeile je Punkt, Aufzählungszeichen tippt man selbst.
const pointsText = ref(props.entry?.answers[2] ?? '')
const points = computed(() => splitPoints(pointsText.value))
const freeNotes = ref(unwrapOther(props.entry?.other ?? ''))
const rawMd = ref('')
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
// Wurde zuletzt in der geführten Ansicht getippt? Dann ist rawMd veraltet und wird beim
// Wechsel nach Markdown aus den Feldern neu gebaut.
let guidedDirty = false
let timer: ReturnType<typeof setTimeout> | undefined
let savedTimer: ReturnType<typeof setTimeout> | undefined
const isDirty = () => version !== savedVersion

// Feld für Punkte; die Höhe wächst mit dem Inhalt, wie bei den Antworten.
const formEl = ref<HTMLFormElement>()
const canImport = computed(() => points.value.length > 0)

function grow(el: HTMLTextAreaElement) {
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}

function onAreaInput(e: Event) {
  grow(e.target as HTMLTextAreaElement)
  guidedDirty = true
  onChange()
}

function onRawMdInput(e: Event) {
  const el = e.target as HTMLTextAreaElement
  grow(el)
  const parsed = parseEntry(rawMd.value)
  q0.value = parsed.answers[0] ?? ''
  q1.value = parsed.answers[1] ?? ''
  pointsText.value = parsed.answers[2] ?? ''
  freeNotes.value = unwrapOther(parsed.other)
  guidedDirty = false
  onChange()
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
  timer = setTimeout(() => {
    timer = undefined
    void save()
  }, 800)
}

// Punkte: ein Feld, eine Zeile je Punkt; die Liste bleibt Markdown.
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
  const answers = [q0.value, q1.value, pointsText.value]
  const other = wrapOther(freeNotes.value)
  const entry = props.mode === 'markdown' ? parseEntry(rawMd.value) : { answers, other }
  const content = props.mode === 'markdown' ? rawMd.value : toMarkdown(props.date, entry)
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
      await createNote('Journal', true).catch((e) => {
        if (!(e instanceof ApiError && e.status === 409)) throw e
      })
      nextMtime = (await createNote(journalPath(props.date))).mtime ?? ''
    }
    nextMtime = (await saveNote(journalPath(props.date), content, nextMtime)).mtime
    mtime = nextMtime
    savedVersion = v
    if (v === version) dirty.value = false
    guidedDirty = false // Datei und Felder stimmen jetzt überein
    // Nach dem Unmount wird noch geschrieben, aber nichts mehr angezeigt oder gemeldet.
    if (!alive) return true
    status.value = 'saved'
    error.value = ''
    emit('saved', { ...entry, markdown: content, mtime })
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
  saving = drain().finally(() => {
    saving = null
  })
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
    pointsText.value = e.answers[2] ?? ''
    freeNotes.value = unwrapOther(e.other)
    rawMd.value = n.content
    guidedDirty = false
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
    pointsText.value = ''
    freeNotes.value = ''
    rawMd.value = toMarkdown(props.date, { answers: ['', '', ''], other: '' })
    guidedDirty = false
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
watch(
  () => props.entry,
  (e) => {
    if (isDirty() || saving) return
    q0.value = e?.answers[0] ?? ''
    q1.value = e?.answers[1] ?? ''
    pointsText.value = e?.answers[2] ?? ''
    freeNotes.value = unwrapOther(e?.other ?? '')
    rawMd.value =
      e?.markdown ??
      toMarkdown(props.date, {
        answers: [q0.value, q1.value, pointsText.value],
        other: wrapOther(freeNotes.value),
      })
    guidedDirty = false
    mtime = e?.mtime ?? ''
    void nextTick(growAll)
  },
)
watch(
  () => props.mode,
  (m, previous) => {
    // Beim Wechsel nach Markdown den aktuellen Stand zeigen: nach Tippen in der geführten
    // Ansicht (auch über „Lesen“) neu bauen, sonst nur ein leeres Feld füllen.
    if (m === 'markdown' && (previous === 'write' || guidedDirty || !rawMd.value.trim())) {
      const fields = {
        answers: [q0.value, q1.value, pointsText.value],
        other: wrapOther(freeNotes.value),
      }
      rawMd.value =
        previous === 'write' || guidedDirty
          ? toMarkdown(props.date, fields)
          : (props.entry?.markdown ?? toMarkdown(props.date, fields))
      guidedDirty = false
    }
    if (m !== 'read') void nextTick(growAll)
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  if (savedTimer) clearTimeout(savedTimer)
  window.removeEventListener('resize', growAll)
  // Die Seite wird verlassen; die Anfrage läuft als SPA-Navigation weiter.
  void flush()
})

type InlineToken = { kind: 'text' | 'bold' | 'code' | 'wiki'; text: string }
type BlockLine = { kind: 'quote' | 'para' | 'bullet'; tokens: InlineToken[] }

function parseInline(src: string): InlineToken[] {
  const out: InlineToken[] = []
  const re = /\[\[([^\]]+)\]\]|\*\*([^*]+)\*\*|`([^`]+)`/g
  let last = 0
  for (const m of src.matchAll(re)) {
    const idx = m.index ?? 0
    if (idx > last) out.push({ kind: 'text', text: src.slice(last, idx) })
    if (m[1] !== undefined) out.push({ kind: 'wiki', text: m[1] })
    else if (m[2] !== undefined) out.push({ kind: 'bold', text: m[2] })
    else if (m[3] !== undefined) out.push({ kind: 'code', text: m[3] })
    last = idx + m[0].length
  }
  if (last < src.length) out.push({ kind: 'text', text: src.slice(last) })
  return out
}

function parseBlocks(src: string): BlockLine[] {
  return src
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      if (line.startsWith('> ')) {
        return { kind: 'quote', tokens: parseInline(line.slice(2)) }
      }
      if (/^[-*]\s+/.test(line)) {
        return { kind: 'bullet', tokens: parseInline(line.replace(/^[-*]\s+/, '')) }
      }
      return { kind: 'para', tokens: parseInline(line) }
    })
}

const cleanPoints = computed(() => points.value.map((p) => p.trim()).filter(Boolean))
const hasAnyContent = computed(() =>
  Boolean(q0.value.trim() || q1.value.trim() || cleanPoints.value.length || freeNotes.value.trim()),
)

const freeReadSections = computed(() => {
  const raw = wrapOther(freeNotes.value)
  if (!raw) return []
  return raw
    .split(/^(?=#{2,6}\s)/m)
    .map((b) => b.trim())
    .filter(Boolean)
    .map((block) => {
      const m = block.match(/^(#{2,6})\s+([^\n]+)\n*([\s\S]*)$/)
      const title = m ? (m[2] ?? '').trim() : t('Freie Notizen')
      const body = m ? (m[3] ?? '').trim() : block
      return { title, blocks: parseBlocks(body) }
    })
    .filter((s) => s.blocks.length > 0)
})

// ---- Aufgaben übernehmen ----
const importOpen = ref(false)
const importing = ref(false)
const importedCount = computed(() => props.importedTitles.length)
const importDueLabel = computed(() => {
  const [year = 0, month = 1, day = 1] = props.date.split('-').map(Number)
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(year, month - 1, day + 1)))
})

function openImport() {
  importOpen.value = true
}

async function submitImport(titles: string[]) {
  if (!titles.length || importing.value) return
  importing.value = true
  try {
    const result = await importJournalTasks(props.date, titles)
    emit('imported', result)
    importOpen.value = false
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

    <div v-if="mode === 'read'" class="je-read">
      <p v-if="!hasAnyContent" class="je-empty">{{ $t('Noch kein Eintrag') }}</p>
      <section class="je-read-sec">
        <h3 class="je-label">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was lief gut?') }}
        </h3>
        <template v-for="(b, idx) in parseBlocks(q0)" :key="idx">
          <blockquote v-if="b.kind === 'quote'" class="je-read-quote">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </blockquote>
          <div v-else-if="b.kind === 'bullet'" class="je-read-li">
            <span class="je-bullet" aria-hidden="true">-</span>
            <span>
              <template v-for="(tok, ti) in b.tokens" :key="ti">
                <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
                <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
                <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
                <template v-else>{{ tok.text }}</template>
              </template>
            </span>
          </div>
          <p v-else class="je-read-p">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </p>
        </template>
      </section>

      <section class="je-read-sec">
        <h3 class="je-label">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was lerne ich daraus?') }}
        </h3>
        <template v-for="(b, idx) in parseBlocks(q1)" :key="idx">
          <blockquote v-if="b.kind === 'quote'" class="je-read-quote">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </blockquote>
          <div v-else-if="b.kind === 'bullet'" class="je-read-li">
            <span class="je-bullet" aria-hidden="true">-</span>
            <span>
              <template v-for="(tok, ti) in b.tokens" :key="ti">
                <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
                <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
                <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
                <template v-else>{{ tok.text }}</template>
              </template>
            </span>
          </div>
          <p v-else class="je-read-p">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </p>
        </template>
      </section>

      <section class="je-read-sec">
        <h3 class="je-label">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was ist morgen wichtig?') }}
        </h3>
        <ul class="je-read-list">
          <li v-for="(p, idx) in cleanPoints" :key="idx">
            <span class="je-bullet" aria-hidden="true">-</span>
            <span>{{ p }}</span>
          </li>
        </ul>
      </section>

      <section v-for="(sec, sIdx) in freeReadSections" :key="sIdx" class="je-read-sec">
        <h3 class="je-label"><span class="je-mk" aria-hidden="true">##</span>{{ sec.title }}</h3>
        <template v-for="(b, idx) in sec.blocks" :key="idx">
          <blockquote v-if="b.kind === 'quote'" class="je-read-quote">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </blockquote>
          <div v-else-if="b.kind === 'bullet'" class="je-read-li">
            <span class="je-bullet" aria-hidden="true">-</span>
            <span>
              <template v-for="(tok, ti) in b.tokens" :key="ti">
                <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
                <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
                <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
                <template v-else>{{ tok.text }}</template>
              </template>
            </span>
          </div>
          <p v-else class="je-read-p">
            <template v-for="(tok, ti) in b.tokens" :key="ti">
              <strong v-if="tok.kind === 'bold'">{{ tok.text }}</strong>
              <code v-else-if="tok.kind === 'code'" class="je-read-code">{{ tok.text }}</code>
              <span v-else-if="tok.kind === 'wiki'" class="je-read-wiki">{{ tok.text }}</span>
              <template v-else>{{ tok.text }}</template>
            </template>
          </p>
        </template>
      </section>
    </div>

    <form v-else-if="mode === 'markdown'" ref="formEl" class="je-form" @submit.prevent>
      <section class="je-q">
        <textarea
          v-model="rawMd"
          class="je-area je-raw-md"
          rows="12"
          :aria-label="$t('Freies Markdown für den gesamten Eintrag')"
          @input="onRawMdInput"
        ></textarea>
        <div class="je-foot">
          <span class="je-status" aria-live="polite">
            <span v-if="dirty" class="save-status dirty"
              ><i></i>{{ $t('Ungespeicherte Änderungen') }}</span
            >
            <span v-else-if="status === 'saving'" class="save-status">{{
              $t('Wird gespeichert …')
            }}</span>
            <span v-else class="save-status saved je-saved"
              ><svg class="ic"><use href="#i-check" /></svg>{{ $t('Gespeichert') }}</span
            >
          </span>
        </div>
      </section>
    </form>

    <form v-else ref="formEl" class="je-form" @submit.prevent>
      <section class="je-q">
        <label class="je-label" for="je-a">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was lief gut?') }}
        </label>
        <textarea
          id="je-a"
          v-model="q0"
          class="je-area"
          rows="1"
          :placeholder="HINTS[0]"
          @input="onAreaInput"
        ></textarea>
      </section>

      <section class="je-q">
        <label class="je-label" for="je-b">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was lerne ich daraus?') }}
        </label>
        <textarea
          id="je-b"
          v-model="q1"
          class="je-area"
          rows="1"
          :placeholder="HINTS[1]"
          @input="onAreaInput"
        ></textarea>
      </section>

      <section class="je-q">
        <label class="je-label" for="je-c">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Was ist morgen wichtig?') }}
        </label>
        <textarea
          id="je-c"
          v-model="pointsText"
          class="je-area je-points-text"
          rows="1"
          :placeholder="HINTS[2]"
          @input="onAreaInput"
        ></textarea>
      </section>

      <section class="je-q">
        <label class="je-label" for="je-free">
          <span class="je-mk" aria-hidden="true">##</span>{{ $t('Freie Notizen') }}
        </label>
        <textarea
          id="je-free"
          v-model="freeNotes"
          class="je-area"
          rows="2"
          :placeholder="HINTS[3]"
          @input="onAreaInput"
        ></textarea>
        <div class="je-foot">
          <button
            v-if="canImport"
            type="button"
            class="btn btn-ghost je-import"
            @click="openImport"
          >
            <svg class="ic"><use href="#i-tasks" /></svg>{{ $t('Als Aufgaben übernehmen') }}
          </button>
          <span v-if="importedCount" class="je-imported"
            >✓ {{ $tn(importedCount, '{n} Aufgabe übernommen', '{n} Aufgaben übernommen') }}</span
          >
          <span class="je-status" aria-live="polite">
            <span v-if="dirty" class="save-status dirty"
              ><i></i>{{ $t('Ungespeicherte Änderungen') }}</span
            >
            <span v-else-if="status === 'saving'" class="save-status">{{
              $t('Wird gespeichert …')
            }}</span>
            <span v-else class="save-status saved je-saved"
              ><svg class="ic"><use href="#i-check" /></svg>{{ $t('Gespeichert') }}</span
            >
          </span>
        </div>
      </section>
    </form>

    <JournalImportDialog
      :open="importOpen"
      :due-label="importDueLabel"
      :points="points"
      :imported-titles="importedTitles"
      :importing="importing"
      @close="importOpen = false"
      @submit="submitImport"
    />
  </div>
</template>

<style scoped src="./journalEntryEditor.css"></style>
