<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  api,
  errorMessage,
  getOccurrences,
  isOpen,
  listNotes,
  listProjects,
  listTasks,
  type CalendarEvent,
  type NoteNode,
  type Project,
  type Task,
} from '@/api/client'
import { addDays, dayLabel, hhmm, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'

// ⌘K: eine Suche über Aufgaben, Projekte, Notizen (Name und Inhalt) und Termine. Enter öffnet den markierten Treffer,
// ohne Treffer legt Enter die Eingabe als Aufgabe an (mit Schnelleingabe-Syntax).
const emit = defineEmits<{ close: [] }>()
const router = useRouter()

interface Hit {
  key: string
  group: string
  title: string
  sub?: string
  color?: string
  go: () => unknown
}
interface NoteHit {
  path: string
  line: number
  snippet: string
}

const q = ref('')
const sel = ref(0)
const error = ref('')
const input = ref<HTMLInputElement>()
const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const files = ref<string[]>([])
const events = ref<CalendarEvent[]>([])
const noteHits = ref<NoteHit[]>([])
const today = ymd(new Date())

onMounted(async () => {
  void nextTick(() => input.value?.focus())
  const walk = (ns: NoteNode[], out: string[]): string[] => (
    ns.forEach((n) => (n.dir ? walk(n.children ?? [], out) : out.push(n.path))),
    out
  )
  // Termine: ein Monat zurück, ein halbes Jahr voraus; Serien kommen dabei als einzelne Termine.
  const from = addDays(new Date(), -30)
  const results = await Promise.allSettled([
    listTasks(),
    listProjects(),
    listNotes(),
    getOccurrences(from, addDays(from, 210)),
  ])
  const [ts, ps, ns, es] = results
  if (ts.status === 'fulfilled') tasks.value = ts.value
  if (ps.status === 'fulfilled') projects.value = ps.value
  if (ns.status === 'fulfilled') files.value = walk(ns.value, [])
  if (es.status === 'fulfilled') events.value = es.value
  const failed = results.find((r) => r.status === 'rejected')
  if (failed) error.value = errorMessage((failed as PromiseRejectedResult).reason)
})

// Volltext der Notizen: kurz warten, damit nicht jeder Tastendruck alle Dateien liest; alte Antworten verwerfen.
let timer = 0
let seq = 0
watch(q, (v) => {
  sel.value = 0
  clearTimeout(timer)
  if (v.trim().length < 2) return void (noteHits.value = [])
  const n = ++seq
  timer = window.setTimeout(async () => {
    try {
      const hits = await api<NoteHit[]>(`/search/notes?q=${encodeURIComponent(v.trim())}`)
      if (n === seq) noteHits.value = hits
    } catch {
      /* Dateinamen und der Rest bleiben, nur der Volltext fehlt */
    }
  }, 150)
})

// Jedes Wort muss vorkommen, Groß/Klein und Akzente egal: „algo baum“ findet „Bäume (AlgoDat)“.
const fold = (s: string) => s.toLowerCase().normalize('NFD').replace(/\p{M}/gu, '')
const words = computed(() => fold(q.value).split(/\s+/).filter(Boolean))
const matches = (...texts: (string | null | undefined)[]) => {
  const hay = fold(texts.filter(Boolean).join(' '))
  return words.value.every((w) => hay.includes(w))
}
const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const fileName = (p: string) => p.slice(p.lastIndexOf('/') + 1).replace(/\.md$/i, '')
const go = (path: string, query: Record<string, string>) => () => router.push({ path, query })

const hits = computed<Hit[]>(() => {
  if (!words.value.length) return []
  const out: Hit[] = []
  const taskHits = tasks.value
    .filter((x) =>
      matches(x.title, x.description, x.project_id && projectName.value.get(x.project_id)),
    )
    .sort((a, b) => Number(isOpen(b)) - Number(isOpen(a))) // offene zuerst
    .slice(0, 6)
  for (const x of taskHits) {
    out.push({
      key: `t${x.id}`,
      group: t('Aufgaben'),
      title: x.title,
      color: projectColor(x.project_id),
      sub: [
        x.project_id && projectName.value.get(x.project_id),
        !isOpen(x) && t(x.status === 'COMPLETED' ? 'Erledigt' : 'Abgebrochen'),
        x.planned_date && dayLabel(x.planned_date, today),
      ]
        .filter(Boolean)
        .join(' · '),
      go: go('/tasks', { task: x.id }),
    })
  }
  for (const p of projects.value.filter((p) => matches(p.name, p.description)).slice(0, 4)) {
    out.push({
      key: `p${p.id}`,
      group: t('Projekte'),
      title: p.name,
      sub: p.description,
      color: projectColor(p.id),
      go: go('/projects', { project: p.id }),
    })
  }
  // Notizen: erst Treffer im Namen, dann im Inhalt (mit Zeile), jede Datei einmal.
  const seen = new Set<string>()
  const byName = files.value.filter((f) => matches(f)).slice(0, 6)
  for (const f of byName) seen.add(f)
  const noteRows = [
    ...byName.map((f) => ({
      path: f,
      snippet: f.includes('/') ? f.slice(0, f.lastIndexOf('/')) : '',
    })),
    ...noteHits.value
      .filter((h) => !seen.has(h.path))
      .map((h) => ({ path: h.path, snippet: h.snippet || h.path })),
  ].slice(0, 8)
  for (const n of noteRows)
    out.push({
      key: `n${n.path}`,
      group: t('Notizen'),
      title: fileName(n.path),
      sub: n.snippet,
      go: go('/notes', { open: n.path }),
    })
  // Termine: kommende zuerst (nächster oben), danach vergangene (jüngster oben); eine Serie nur einmal.
  const now = Date.now()
  const evs = events.value.filter((e) => matches(e.title, e.location))
  const upcoming = evs
    .filter((e) => Date.parse(e.occurrence_end) >= now)
    .sort((a, b) => a.occurrence_start.localeCompare(b.occurrence_start))
  const past = evs
    .filter((e) => Date.parse(e.occurrence_end) < now)
    .sort((a, b) => b.occurrence_start.localeCompare(a.occurrence_start))
  const seenEv = new Set<string>()
  for (const e of [...upcoming, ...past]) {
    if (seenEv.has(e.id) || seenEv.size >= 5) continue
    seenEv.add(e.id)
    const start = new Date(e.occurrence_start)
    const day = ymd(start)
    out.push({
      key: `e${e.id}`,
      group: t('Termine'),
      title: e.title,
      sub: [`${dayLabel(day, today)} ${hhmm(start)}`, e.location].filter(Boolean).join(' · '),
      color: projectColor(e.project_id),
      go: go('/calendar', { date: day }),
    })
  }
  return out
})

// Immer zuletzt: die Eingabe als neue Aufgabe (Schnelleingabe versteht @morgen, #Projekt, …).
const createHit = computed<Hit | null>(() =>
  q.value.trim()
    ? {
        key: 'new',
        group: t('Neu'),
        title: t('Aufgabe „{text}“ anlegen', { text: q.value.trim() }),
        go: go('/tasks', { new: String(Date.now()), text: q.value.trim() }),
      }
    : null,
)
const all = computed(() => [...hits.value, ...(createHit.value ? [createHit.value] : [])])

function pick(h: Hit | undefined) {
  if (!h) return
  void h.go()
  emit('close')
}
function onKey(e: KeyboardEvent) {
  const n = all.value.length
  if (e.key === 'ArrowDown' || (e.key === 'n' && e.ctrlKey)) {
    e.preventDefault()
    sel.value = n ? (sel.value + 1) % n : 0
  } else if (e.key === 'ArrowUp' || (e.key === 'p' && e.ctrlKey)) {
    e.preventDefault()
    sel.value = n ? (sel.value - 1 + n) % n : 0
  } else if (e.key === 'Enter' && !e.isComposing) {
    e.preventDefault()
    pick(all.value[sel.value])
  }
}
watch(
  sel,
  (i) =>
    void nextTick(() =>
      document.querySelector(`.sp-item[data-i="${i}"]`)?.scrollIntoView({ block: 'nearest' }),
    ),
)
</script>

<template>
  <div v-dialog="() => emit('close')" class="overlay open" @mousedown.self="emit('close')">
    <div class="dialog sp" :aria-label="$t('Suche')">
      <div class="sp-head">
        <svg class="ic" aria-hidden="true"><use href="#i-search" /></svg>
        <input
          ref="input"
          v-model="q"
          class="sp-input"
          type="search"
          role="combobox"
          aria-autocomplete="list"
          aria-controls="sp-list"
          :aria-expanded="all.length > 0"
          :aria-activedescendant="all.length ? `sp-${sel}` : undefined"
          :placeholder="$t('Aufgaben, Projekte, Notizen, Termine …')"
          :aria-label="$t('Suche')"
          @keydown="onKey"
        />
        <kbd class="key" aria-hidden="true">esc</kbd>
      </div>
      <ul v-if="all.length" id="sp-list" class="sp-list" role="listbox">
        <template v-for="(h, i) in all" :key="h.key">
          <li v-if="i === 0 || all[i - 1]!.group !== h.group" class="sp-group" role="presentation">
            {{ h.group }}
          </li>
          <li
            :id="`sp-${i}`"
            class="sp-item"
            :data-i="i"
            role="option"
            :aria-selected="i === sel"
            @mousemove="sel = i"
            @click="pick(h)"
          >
            <span
              class="cdot"
              :style="h.color ? { background: h.color } : { visibility: 'hidden' }"
            ></span>
            <span class="sp-t">{{ h.title }}</span>
            <span v-if="h.sub" class="sp-sub">{{ h.sub }}</span>
          </li>
        </template>
      </ul>
      <div v-else class="sp-empty">
        {{ error || $t('Tippe, um zu suchen. Enter ohne Treffer legt eine Aufgabe an.') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.sp {
  width: 640px;
  padding: 0;
  overflow: hidden;
}
.sp-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--br-subtle, var(--br-default));
}
.sp-head .ic {
  color: var(--tx-muted);
  flex: none;
}
.sp-input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--tx-primary);
  font: 400 15px/1.4 var(--font-ui);
}
.sp-input::-webkit-search-cancel-button {
  display: none;
}
.sp-list {
  list-style: none;
  margin: 0;
  padding: 6px;
  max-height: min(60vh, 480px);
  overflow-y: auto;
}
.sp-group {
  padding: 10px 10px 4px;
  font: 500 11px/1 var(--font-ui);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.sp-item {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--r-s, 6px);
  cursor: pointer;
  min-width: 0;
}
.sp-item[aria-selected='true'] {
  background: var(--bg-hover);
}
.sp-item .cdot {
  flex: none;
  align-self: center;
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.sp-t {
  flex: none;
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--tx-primary);
}
.sp-sub {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--tx-muted);
  font-size: 12px;
}
.sp-empty {
  padding: 16px;
  color: var(--tx-muted);
  font: 400 13px/1.4 var(--font-ui);
}
@media (max-width: 640px) {
  .sp-item {
    flex-wrap: wrap;
  }
  .sp-t {
    max-width: 100%;
  }
  .sp-sub {
    flex-basis: 100%;
    padding-left: 18px;
  }
}
</style>
