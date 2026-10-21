<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError, createNote, errorMessage, getJournalSummary, importJournalTasks, listJournalTaskImports, listNotes, readNote, saveNote } from '@/api/client'
import { shiftDate } from '@/lib/dates'
import { journalPath, parseEntry, QUESTIONS, splitTaskCandidates, toMarkdown } from '@/lib/journal'
import { locale, t } from '@/lib/i18n'
import { useShortcuts } from '@/composables/useShortcuts'
import { vDialog } from '@/lib/dialog'

// Übersicht der letzten Einträge; Schreiben in einem eigenen Editor.
const RECENT = 30
const HINTS = [t('Etwas, das heute geklappt hat, auch Kleines'), t('Was würdest du beim nächsten Mal anders machen?'), t('Die eine Sache, die morgen zählt')]
const route = useRoute()
const router = useRouter()

interface Row { date: string; answers: string[] }
interface TaskCandidate { title: string; selected: boolean }

const todayStr = ref('')
const rows = ref<Row[]>([])
const total = ref(0)
const streak = ref(0)
const loaded = ref(false)
const error = ref('')
const taskImports = ref<Record<string, number>>({})
const importDate = ref<string | null>(null)
const candidates = ref<TaskCandidate[]>([])
const importingTasks = ref(false)

// Editor
const editing = ref<string | null>(null) // Datum des Eintrags im Editor
const reading = ref<string | null>(null) // Datum des Eintrags in der Leseansicht
const answers = ref<string[]>(QUESTIONS.map(() => ''))
let other = ''
let mtime = '' // leer: Eintrag gibt es noch nicht
const saved = ref('')
const dirty = computed(() => answers.value.join('\u0000') !== saved.value)
const saving = ref(false)
const fields = ref<HTMLTextAreaElement[]>([])

const longFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }))
const dayFmt = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }))
const monthFmt = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }))
const fmt = (f: Intl.DateTimeFormat, d: string) => f.format(new Date(d + 'T00:00'))
const relativeLabel = (d: string) => (todayStr.value && d === todayStr.value ? t('Heute') : todayStr.value && d === shiftDate(todayStr.value, -1) ? t('Gestern') : '')
const label = (d: string) => relativeLabel(d) || fmt(dayFmt.value, d)
const monthLabel = (m: string) => fmt(monthFmt.value, `${m}-01`)
const groups = computed(() => {
  const out: { month: string; rows: Row[] }[] = []
  for (const r of rows.value) {
    const month = r.date.slice(0, 7)
    const last = out[out.length - 1]
    if (last?.month === month) last.rows.push(r)
    else out.push({ month, rows: [r] })
  }
  return out
})
const readRow = computed(() => rows.value.find((r) => r.date === reading.value) ?? null)
const taskImportCount = (date: string) => taskImports.value[date] ?? 0
const selectedTaskTitles = computed(() => candidates.value.filter((c) => c.selected && c.title.trim()).map((c) => c.title.trim()))
const importDueLabel = computed(() => {
  if (!importDate.value) return ''
  const [year = 0, month = 1, day = 1] = importDate.value.split('-').map(Number)
  const date = new Date(Date.UTC(year, month - 1, day + 1))
  return new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }).format(date)
})

const hasToday = computed(() => rows.value.some((r) => r.date === todayStr.value))

// shortcut: liest die letzten RECENT Einträge einzeln; für mehr bräuchte es Blättern oder einen Endpunkt.
async function load() {
  try {
    const [notes, imports, summary] = await Promise.all([listNotes(), listJournalTaskImports(), getJournalSummary()])
    const dir = notes.find((n) => n.dir && n.path === 'Journal')
    const dates = (dir?.children ?? [])
      .map((n) => /^(\d{4}-\d{2}-\d{2})\.md$/.exec(n.name)?.[1])
      .filter((d): d is string => !!d)
      .sort()
      .reverse()
    todayStr.value = summary.date
    total.value = summary.total
    streak.value = summary.streak
    rows.value = await Promise.all(dates.slice(0, RECENT).map(async (date) => ({
      date, answers: parseEntry((await readNote(journalPath(date))).content).answers,
    })))
    taskImports.value = Object.fromEntries(imports.map((item) => [item.journal_date, item.task_count]))
    if (typeof route.query.date === 'string' && rows.value.some((r) => r.date === route.query.date)) reading.value = route.query.date
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
  loaded.value = true
}
onMounted(load)
watch(() => route.query.date, (date) => {
  if (typeof date === 'string' && rows.value.some((r) => r.date === date)) reading.value = date
  else if (!date) reading.value = null
})

function openTaskImport(row: Row) {
  if (taskImportCount(row.date)) return
  importDate.value = row.date
  candidates.value = splitTaskCandidates(row.answers[2] ?? '').map((title) => ({ title, selected: true }))
}

function closeTaskImport() {
  if (importingTasks.value) return
  importDate.value = null
  candidates.value = []
}

async function submitTaskImport() {
  const date = importDate.value
  if (!date || importingTasks.value || !selectedTaskTitles.value.length) return
  importingTasks.value = true
  try {
    const imported = await importJournalTasks(date, selectedTaskTitles.value)
    taskImports.value = { ...taskImports.value, [date]: imported.task_count }
    error.value = ''
    importDate.value = null
    candidates.value = []
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    importingTasks.value = false
  }
}

const scrollTop = () => (document.querySelector('.content') as HTMLElement | null)?.scrollTo({ top: 0 })

function openRead(date: string) {
  reading.value = date
  scrollTop()
}

function showAll() {
  reading.value = null
  const query = { ...route.query }
  delete query.date
  void router.replace({ query })
}

async function open(date: string, keepReading = false) {
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
  reading.value = keepReading ? date : null
  error.value = ''
  scrollTop()
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
    reading.value = date
    scrollTop()
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
  if (e.key === 'n' && !editing.value && todayStr.value) {
    e.preventDefault()
    void open(todayStr.value)
  }
})
</script>

<template>
  <div class="view-inner">
    <!-- Editor -->
    <template v-if="editing">
      <button type="button" class="col-link jr-back" @click="close"><svg class="ic"><use href="#i-left" /></svg>{{ reading ? $t('Zurück zum Eintrag') : $t('Alle Einträge') }}</button>
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

    <!-- Leseansicht eines Eintrags -->
    <template v-else-if="readRow">
      <button type="button" class="col-link jr-back" @click="showAll"><svg class="ic"><use href="#i-left" /></svg>{{ $t('Alle Einträge') }}</button>
      <div class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ fmt(longFmt, readRow.date) }}</h1>
          <div v-if="relativeLabel(readRow.date)" class="v-sub jr-read-sub"><span class="badge">{{ relativeLabel(readRow.date) }}</span></div>
        </div>
        <button class="btn btn-primary" type="button" @click="open(readRow.date, true)">
          <svg class="ic"><use href="#i-journal" /></svg>{{ $t('Bearbeiten') }}
        </button>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <article class="card jr-read">
        <section v-for="(q, i) in QUESTIONS" :key="q" class="jr-read-q">
          <h2 class="jr-read-head">{{ $t(q) }}</h2>
          <p v-if="readRow.answers[i]" class="jr-read-a">{{ readRow.answers[i] }}</p>
          <p v-else class="jr-read-a empty">–</p>
        </section>
      </article>
    </template>

    <!-- Übersicht -->
    <template v-else>
      <div class="v-head v-head-row">
        <div>
          <h1 class="v-title">{{ $t('Journal') }}</h1>
          <div class="v-sub">
            <template v-if="streak">{{ $t('{n} Tage in Folge', { n: streak }) }} · </template>{{ $tn(total, '{n} Eintrag', '{n} Einträge') }}
          </div>
        </div>
        <button v-if="loaded && todayStr && !hasToday" class="btn btn-primary" type="button" aria-keyshortcuts="n" @click="open(todayStr)">
          <svg class="ic"><use href="#i-plus" /></svg>{{ $t('Heute schreiben') }}
        </button>
      </div>
      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <div v-if="loaded && !rows.length" class="card jr-empty">
        <p class="jr-empty-t">{{ $t('Drei kurze Fragen am Abend.') }}</p>
        <p class="v-sub">{{ $t('Was lief gut, was lernst du daraus, was ist morgen wichtig. Zwei Minuten reichen.') }}</p>
      </div>

      <div v-else-if="rows.length" class="jr-months">
        <section v-for="g in groups" :key="g.month" class="jr-month-group">
          <h2 class="jr-month">{{ monthLabel(g.month) }}</h2>
          <div class="jr-feed">
            <article v-for="r in g.rows" :key="r.date" class="jr-entry" :class="{ today: r.date === todayStr }">
              <button type="button" class="jr-entry-open" @click="r.date === todayStr ? open(r.date) : openRead(r.date)">
                <span class="jr-entry-head">
                  <span class="jr-entry-date">
                    <b>{{ label(r.date) }}</b>
                    <span>{{ fmt(longFmt, r.date) }}</span>
                  </span>
                  <svg class="ic jr-chev"><use href="#i-right" /></svg>
                </span>
                <span class="jr-entry-answers">
                  <span v-for="(a, i) in r.answers" :key="i" class="jr-preview" :class="{ empty: !a }">
                    <span class="jr-q-lbl">{{ $t(QUESTIONS[i]!) }}</span>
                    <span class="jr-a">{{ a || '–' }}</span>
                  </span>
                </span>
              </button>
              <button v-if="r.answers[2]?.trim() || taskImportCount(r.date)" type="button" class="btn btn-ghost jr-import-btn"
                :disabled="!!taskImportCount(r.date)" @click="openTaskImport(r)">
                <template v-if="taskImportCount(r.date)"><span aria-hidden="true">✓ </span>{{ $tn(taskImportCount(r.date), '{n} Aufgabe übernommen', '{n} Aufgaben übernommen') }}</template>
                <template v-else>{{ $t('Als Aufgaben übernehmen') }}</template>
              </button>
            </article>
          </div>
        </section>
      </div>
      <p v-if="total > rows.length" class="v-sub jr-more">{{ $t('Ältere Einträge findest du unter Notizen im Ordner Journal.') }}</p>
    </template>

    <div v-if="importDate" v-dialog="closeTaskImport" class="overlay open" @mousedown.self="closeTaskImport">
      <form class="dialog jr-import-dialog" aria-labelledby="jr-import-title" @submit.prevent="submitTaskImport">
        <div class="dlg-head">
          <div>
            <h3 id="jr-import-title">{{ $t('Als Aufgaben übernehmen') }}</h3>
            <div class="v-sub">{{ $t('Fällig am {date}', { date: importDueLabel }) }}</div>
          </div>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="closeTaskImport"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <p class="v-sub">{{ $t('Wähle die Aufgaben aus und passe Titel bei Bedarf an.') }}</p>
          <div class="jr-task-candidates">
            <label v-for="(candidate, i) in candidates" :key="i" class="jr-task-candidate">
              <input v-model="candidate.selected" type="checkbox" :aria-label="$t('Aufgabe übernehmen: {title}', { title: candidate.title })" />
              <input v-model="candidate.title" class="input" type="text" :aria-label="$t('Aufgabentitel {n}', { n: i + 1 })" />
            </label>
          </div>
        </div>
        <div class="dlg-foot">
          <button class="btn btn-ghost" type="button" :disabled="importingTasks" @click="closeTaskImport">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit" :disabled="importingTasks || !selectedTaskTitles.length">
            {{ importingTasks ? $t('Wird angelegt …') : $t('Aufgaben anlegen') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
/* Übersicht: eine Zeitleiste aus Tageskarten statt einer Tabelle. */
.jr-months { display: grid; gap: 28px; margin-top: 8px; }
.jr-month-group { display: grid; gap: 10px; }
.jr-month { font: 600 12px/1 var(--font-ui); letter-spacing: .06em; text-transform: uppercase; color: var(--tx-muted); padding-left: 2px; }
.jr-feed { position: relative; display: grid; gap: 10px; }
.jr-feed::before { content: ''; position: absolute; left: 13px; top: 18px; bottom: 18px; width: 1px; background: var(--br-default); }
.jr-entry {
  position: relative; display: grid; gap: 12px; margin-left: 30px; padding: 18px 20px;
  border: 1px solid var(--br-default); border-radius: var(--r-m); background: var(--bg-1);
  color: inherit;
  transition: border-color var(--dur) ease-out, background var(--dur) ease-out;
}
.jr-entry-open { display: grid; gap: 14px; width: 100%; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.jr-entry::before {
  content: ''; position: absolute; left: -21px; top: 24px; width: 9px; height: 9px; border-radius: 50%;
  background: var(--br-strong); box-shadow: 0 0 0 4px var(--bg-0);
}
.jr-entry.today { border-color: var(--br-default); background: color-mix(in srgb, var(--a-blue) 5%, var(--bg-1)); }
.jr-entry.today::before { background: var(--a-blue-hi); }
.jr-entry:hover, .jr-entry:focus-within { border-color: var(--a-blue-hi); background: var(--bg-hover); }
.jr-entry-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.jr-entry-date { display: grid; gap: 3px; min-width: 0; }
.jr-entry-date b { font: 600 16px/1.25 var(--font-ui); color: var(--tx-primary); }
.jr-entry-date span { font: 400 12px/1.4 var(--font-ui); color: var(--tx-muted); }
.jr-chev { color: var(--tx-muted); margin-top: 3px; transition: transform var(--dur) ease-out, color var(--dur) ease-out; }
.jr-entry:hover .jr-chev { color: var(--tx-secondary); transform: translateX(2px); }
.jr-entry-answers { display: grid; gap: 16px; }
.jr-preview { display: grid; gap: 2px; min-width: 0; }
.jr-q-lbl { font: 500 11px/1.3 var(--font-ui); letter-spacing: .04em; text-transform: uppercase; color: var(--tx-muted); }
.jr-a {
  max-width: 70ch; font: 400 13px/1.55 var(--font-ui); color: var(--tx-secondary); white-space: pre-line;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.jr-preview.empty .jr-a { color: var(--tx-disabled); }
.jr-import-btn { justify-self: start; }
.jr-more { margin-top: 12px; }

.jr-import-dialog { max-width: 520px; }
.jr-import-dialog .dlg-head .v-sub { margin-top: 6px; }
.jr-import-dialog .dlg-body > .v-sub { margin: 0; }
.jr-task-candidates { display: grid; gap: 8px; max-height: 42vh; overflow-y: auto; }
.jr-task-candidate { display: grid; grid-template-columns: 18px minmax(0, 1fr); align-items: center; gap: 10px; }
.jr-task-candidate input[type='checkbox'] { margin: 0; }

/* Leseansicht */
.jr-read { max-width: 720px; padding: 4px 32px; margin-top: 4px; }
.jr-read-sub { display: flex; align-items: center; gap: 8px; }
.jr-read-q { display: grid; gap: 8px; padding: 22px 0; border-top: 1px solid var(--br-subtle); }
.jr-read-q:first-child { border-top: 0; }
.jr-read-head { font: 500 11px/1.3 var(--font-ui); letter-spacing: .06em; text-transform: uppercase; color: var(--tx-muted); }
.jr-read-a { max-width: 70ch; margin: 0; font: 400 15px/1.7 var(--font-ui); color: var(--tx-primary); white-space: pre-line; }
.jr-read-a.empty { color: var(--tx-disabled); }

.jr-empty { display: grid; justify-items: start; gap: 8px; max-width: 560px; padding: 28px; }
.jr-empty p { margin: 0; }
.jr-empty-t { font: 600 17px/1.3 var(--font-ui); color: var(--tx-primary); }

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
  .jr-feed::before, .jr-entry::before { display: none; }
  .jr-entry { margin-left: 0; padding: 16px; }
  .jr-read { padding: 2px 18px; }
  .jr-read-q { padding: 18px 0; }
  .jr-q { padding: 16px; }
  .jr-area { padding-left: 0; }
  .jr-keys { display: none; }
}
</style>
