<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createResource, createTask, deleteTask, isOpen, listProjects, listResources, listTasks, restoreTask, taskAction, updateTask,
  type Project, type Resource, type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useTaskSelection } from '@/composables/useTaskSelection'
import { useUndo } from '@/composables/useUndo'
import { useFreeSlot } from '@/composables/useFreeSlot'
import { addDays, dayLabel, daysAgo, dm, hhmm, hm, ruleText, shortDay, weekStart, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { vDialog } from '@/lib/dialog'
import { projectColor } from '@/lib/projectColor'
import { parseQuickAdd, quickTokens, RULE_PRESETS, taskBody, type TokenKind } from '@/lib/quickAdd'
import GitHubIssues from '@/components/GitHubIssues.vue'
import SelectionBar from '@/components/SelectionBar.vue'
import ResourceList from '@/components/ResourceList.vue'
import SearchField from '@/components/SearchField.vue'
import TaskActions from '@/components/TaskActions.vue'
import { locale, t, tn } from '@/lib/i18n'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: t('Backlog'), PLANNED: t('Geplant'), IN_PROGRESS: t('Läuft'),
  PAUSED: t('Pausiert'), COMPLETED: t('Erledigt'), CANCELLED: t('Abgebrochen'),
}
const PRIO_LABEL: Record<string, string> = { URGENT: t('Dringend'), HIGH: t('Hoch'), MEDIUM: t('Mittel'), LOW: t('Niedrig') }
const FILTERS = [
  { id: 'alle', label: t('Offen') }, { id: 'ueber', label: t('Überfällig') }, { id: 'heute', label: t('Heute') },
  { id: 'woche', label: t('Nächste 7 Tage') }, { id: 'spaeter', label: t('Später') }, { id: 'ohne', label: t('Ohne Datum') },
  { id: 'erledigt', label: t('Erledigt') },
  { id: 'prio', label: t('Prio hoch') }, { id: 'journal', label: t('Aus Journal') }, { id: 'abgebrochen', label: t('Abgebrochen') },
]
const EMPTY: Record<string, string> = {
  alle: t('Alles erledigt – oder noch nichts angelegt.'), ueber: t('Nichts überfällig.'), heute: t('Heute ist nichts offen.'),
  woche: t('Die nächsten 7 Tage sind frei.'), spaeter: t('Dafür ist noch nichts eingeplant.'), ohne: t('Alles hat ein Datum.'), erledigt: t('Noch nichts erledigt.'),
}
const EXAMPLES = [t('Sport 30m @morgen'), t('Steuererklärung 2h @fr !hoch'), t('Mails beantworten 15m @heute')]
// Per Auswahl setzbar; Läuft, Pausiert und Erledigt ergeben sich aus Timer und Häkchen.
const SETTABLE: Task['status'][] = ['BACKLOG', 'PLANNED', 'CANCELLED']

const SORTS = [['created', t('Angelegt')], ['day', t('Tag')], ['prio', t('Priorität')], ['title', t('Titel')]] as const
const GROUPS = [['due', t('Fällig')], ['none', t('Keine')], ['project', t('Projekt')], ['day', t('Tag')]] as const
const DUE_TITLE: Record<string, string> = { 0: t('Überfällig'), 1: t('Früher'), 3: t('Diese Woche'), 4: t('Später'), 5: t('Ohne Datum') }
const PRIO_RANK: Record<string, number> = { URGENT: 0, HIGH: 1, MEDIUM: 2, LOW: 3 }
const VIEW_KEY = 'kairo-tasks-view' // Sortierung und Gruppierung merkt sich der Browser

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const github = ref(false)
const viewMenu = ref(false)
const closeGitHub = () => (github.value = false)
// Aus der Issue-Liste: Dialog schließen, neu laden (die Task kann gerade erst angelegt sein) und die Task öffnen.
const openImported = (id: string) => {
  github.value = false
  void load().then(() => focusTask(id))
}
const resources = ref<Resource[]>([])
const filter = ref('alle')
const projectId = ref('alle')
const openId = ref<string | null>(null)
const quick = ref('')
const subTitle = ref('')
const search = ref('')
const sel = ref<string | null>(null) // Auswahl für die Tastenkürzel
const quickEl = ref<HTMLInputElement>()
const view = ref({ sort: 'created', group: 'due' })
try { Object.assign(view.value, JSON.parse(store.get(VIEW_KEY) ?? '{}')) } catch { /* kaputter Wert */ }
watch(view, (v) => store.set(VIEW_KEY, JSON.stringify(v)), { deep: true })
const route = useRoute()
const router = useRouter()

const { offer, setDone } = useUndo()

const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const todayStr = () => ymd(new Date())
const weekEnd = () => ymd(addDays(weekStart(), 7)) // Montag der nächsten Woche
// Der für die Liste maßgebliche Tag: geplanter Tag, sonst Fälligkeit.
const dayOf = (t: Task) => t.planned_date ?? (t.due_at ? ymd(new Date(t.due_at)) : null)

const inScope = (t: Task) => {
  if (projectId.value !== 'alle' && t.project_id !== projectId.value) return false
  const q = search.value.trim().toLowerCase()
  return !q || `${t.title} ${t.description}`.toLowerCase().includes(q)
}
const matches = (t: Task, f: string) => {
  const d = dayOf(t)
  switch (f) {
    case 'erledigt': return t.status === 'COMPLETED'
    case 'abgebrochen': return t.status === 'CANCELLED'
    case 'heute': return isOpen(t) && d === todayStr()
    case 'woche': return isOpen(t) && d !== null && d > todayStr() && d <= ymd(addDays(new Date(), 7))
    case 'spaeter': return isOpen(t) && d !== null && d > ymd(addDays(new Date(), 7))
    case 'ohne': return isOpen(t) && d === null
    case 'ueber': return isOpen(t) && d !== null && d < todayStr()
    case 'prio': return isOpen(t) && (t.priority === 'HIGH' || t.priority === 'URGENT')
    case 'journal': return isOpen(t) && !!t.source_journal_date
    default: return isOpen(t)
  }
}
const visible = computed(() => tasks.value.filter((t) => inScope(t) && matches(t, filter.value)))
const counts = computed(() => Object.fromEntries(FILTERS.map((f) => [f.id, tasks.value.filter((t) => inScope(t) && matches(t, f.id)).length])))
// Sieben Reiter (Tasten 1–7); die seltenen Filter stehen unter „Weitere Filter“ und erscheinen als Reiter, solange sie aktiv sind (auch per Link).
const MAIN = FILTERS.slice(0, 7).map((f) => f.id)
// Sichtbar nur Offen/Heute/Erledigt; die Gruppen in der Liste zeigen Überfällig usw. schon. Rest im Menü, Tasten 1–7 bleiben.
const SHOWN = ['alle', 'heute', 'erledigt']
const tabs = computed(() => FILTERS.filter((f) => SHOWN.includes(f.id) || f.id === filter.value))
const extra = FILTERS.filter((f) => !SHOWN.includes(f.id) && f.id !== filter.value)
// Pfeiltasten, Pos1 und Ende wechseln den Reiter (Tablist-Muster); der Fokus folgt.
function onTabKey(e: KeyboardEvent) {
  const ids = tabs.value.map((f) => f.id)
  const i = ids.indexOf(filter.value)
  const j = ({ ArrowRight: i + 1, ArrowLeft: i - 1 + ids.length, Home: 0, End: ids.length - 1 } as Record<string, number>)[e.key]
  if (j === undefined) return
  e.preventDefault()
  filter.value = ids[j % ids.length]!
  void nextTick(() => document.getElementById(`tk-tab-${filter.value}`)?.focus())
}
const stats = computed(() => {
  const o = tasks.value.filter(isOpen)
  const minutes = o.reduce((n, x) => n + x.estimated_minutes, 0)
  return [t('{n} offen', { n: o.length }), t('{n} überfällig', { n: o.filter(overdue).length }), minutes ? t('{time} geschätzt', { time: hm(minutes) }) : ''].filter(Boolean).join(' · ')
})

// Teilaufgaben stehen im Detail ihrer Elterntask; eigene Zeile nur, wenn die Eltern nicht (als oberste Ebene) in der Liste stehen.
const rows = computed(() => {
  const byId = new Map(visible.value.map((t) => [t.id, t]))
  return visible.value.filter((t) => {
    const p = t.parent_task_id ? byId.get(t.parent_task_id) : undefined
    return !p || p.parent_task_id
  })
})
const SORT: Record<string, (a: Task, b: Task) => number> = {
  created: () => 0, // Reihenfolge der API
  day: (a, b) => (dayOf(a) ?? '9').localeCompare(dayOf(b) ?? '9') || (a.planned_start_at ?? '').localeCompare(b.planned_start_at ?? ''),
  prio: (a, b) => (PRIO_RANK[a.priority] ?? 2) - (PRIO_RANK[b.priority] ?? 2),
  title: (a, b) => a.title.localeCompare(b.title, locale.value),
}

// Gruppen in Anzeigereihenfolge: Fälligkeit nach Dringlichkeit, Tag aufsteigend, Projekt nach Name; „ohne“ steht zuletzt.
const todayLong = new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short' }).format(new Date()).replace('.,', '')
function dueBucket(t: Task) {
  const d = dayOf(t)
  if (!d) return '5'
  if (d < todayStr()) return isOpen(t) ? '0' : '1'
  return d === todayStr() ? '2' : d < weekEnd() ? '3' : '4'
}
const groups = computed(() => {
  const sorted = [...rows.value].sort(SORT[view.value.sort] ?? SORT.created!)
  const by = view.value.group
  if (!sorted.length) return []
  if (by !== 'project' && by !== 'day' && by !== 'due') return [{ key: '', title: '', rows: sorted, minutes: 0 }]
  const keyOf = (t: Task) => (by === 'project' ? t.project_id : by === 'due' ? dueBucket(t) : dayOf(t)) ?? ''
  const titleOf = (k: string) => by === 'due' ? (k === '2' ? `${t('Heute')} · ${todayLong}` : DUE_TITLE[k]!)
    : by === 'project' ? (k ? projectName.value.get(k) ?? t('Projekt') : t('Ohne Projekt')) : k ? dayLabel(k, todayStr()) : t('Ohne Tag')
  const m = new Map<string, Task[]>()
  for (const t of sorted) m.set(keyOf(t), [...(m.get(keyOf(t)) ?? []), t])
  return [...m]
    .sort(([a], [b]) => Number(a === '') - Number(b === '') || (by === 'project' ? titleOf(a).localeCompare(titleOf(b), locale.value) : a.localeCompare(b)))
    .map(([key, rs]) => ({ key, title: titleOf(key), rows: rs, minutes: rs.reduce((n, t) => n + t.estimated_minutes, 0) }))
})
const flat = computed(() => groups.value.flatMap((g) => g.rows))
// Verschwindet die ausgewählte Task (erledigt, gefiltert), rückt die Auswahl auf die Nachbarin.
watch(flat, (now, before) => {
  if (!sel.value || now.some((t) => t.id === sel.value)) return
  sel.value = now[Math.min(before.findIndex((t) => t.id === sel.value), now.length - 1)]?.id ?? null
})

const open = computed(() => tasks.value.find((t) => t.id === openId.value))
watch(openId, () => (subTitle.value = ''))
const prioLevel = (t: Task) => ({ LOW: 1, MEDIUM: 2, HIGH: 3, URGENT: 3 })[t.priority as 'LOW'] ?? 2
// Rot nur, wenn hohe Priorität auch heute fällig oder überfällig ist; sonst bleibt der Balken grau.
const urgent = (t: Task) => prioLevel(t) === 3 && (matches(t, 'heute') || matches(t, 'ueber'))
const kidsOf = (id: string) => tasks.value.filter((t) => t.parent_task_id === id)
const progress = (id: string) => {
  const k = kidsOf(id).filter((c) => c.status !== 'CANCELLED')
  return k.length ? `${k.filter((c) => c.status === 'COMPLETED').length}/${k.length}` : ''
}
const resOf = (id: string) => resources.value.filter((r) => r.task_id === id)

const { error, load, run } = useLoader(async () => {
  ;[tasks.value, projects.value, resources.value] = await Promise.all([listTasks(), listProjects(), listResources()])
})

const selection = useTaskSelection(() => flat.value, run, () => ymd(addDays(new Date(), 1)))
const { picked, onClick: pickRow, onPointerDown, onPointerUp } = selection

// Archivierte Projekte sind für #name nicht wählbar.
const activeProjects = computed(() => projects.value.filter((p) => p.status !== 'ARCHIVED'))
const quickParsed = computed(() => parseQuickAdd(quick.value, activeProjects.value, todayStr()))
const tokens = computed(() => quickTokens(quick.value, activeProjects.value, todayStr()))
// Spiegel hinter dem (durchsichtigen) Eingabefeld: dieselben Zeichen samt Leerraum, erkannte Wörter eingefärbt.
const segs = computed(() => {
  let k = 0
  return quick.value.split(/(\s+)/).filter(Boolean).map((w) => ({ w, kind: /^\s/.test(w) ? '' : tokens.value[k++]?.kind ?? '' }))
})
const mirror = ref<HTMLElement>()
const syncScroll = () => { if (mirror.value && quickEl.value) mirror.value.scrollLeft = quickEl.value.scrollLeft }
watch(quick, () => nextTick(syncScroll))

// Vorschläge für das #/@/!-Wort am Ende; ein schon vollständig getipptes Wort braucht keine Liste mehr.
const focused = ref(false)
const dismissed = ref(false)
const suggIdx = ref(0)
watch(quick, () => { dismissed.value = false; suggIdx.value = 0 })
const DAY_WORDS = ['heute', 'morgen', 'übermorgen', 'montag', 'dienstag', 'mittwoch', 'donnerstag', 'freitag', 'samstag', 'sonntag']
const RULE_WORDS = ['täglich', 'werktags', 'wöchentlich', 'monatlich']
const PRIO_WORDS = [['dringend', 'URGENT'], ['hoch', 'HIGH'], ['mittel', 'MEDIUM'], ['niedrig', 'LOW']]
const SUGG_TITLE: Record<string, string> = { '#': t('Projekt'), '@': t('Fällig'), '!': t('Priorität') }
const sugg = computed(() => {
  const m = focused.value && !dismissed.value ? /(?:^|\s)([#@!])(\S*)$/.exec(quick.value) : null
  if (!m) return { title: '', items: [] }
  const ch = m[1]!
  const low = m[2]!.toLowerCase()
  const items = (ch === '#'
    ? activeProjects.value.map((p) => ({ v: `#${p.name.replace(/\s+/g, '')}`, label: p.name, sub: t('{n} offen', { n: tasks.value.filter((x) => x.project_id === p.id && isOpen(x)).length }), color: projectColor(p.id) }))
    : ch === '@'
      ? [...DAY_WORDS.map((w) => ({ v: `@${w}`, label: `@${w}`, sub: dayLabel(parseQuickAdd(`@${w}`, [], todayStr()).date!, todayStr()), color: '' })),
         ...RULE_WORDS.map((w) => ({ v: `@${w}`, label: `@${w}`, sub: t('Wiederholung'), color: '' }))]
      : PRIO_WORDS.map(([w, k]) => ({ v: `!${w}`, label: `!${w}`, sub: PRIO_LABEL[k!]!, color: '' }))
  ).filter((x) => x.v.slice(1).toLowerCase().startsWith(low)).slice(0, 8)
  // Ein einziger Treffer, der schon passt oder erkannt ist (@fr), braucht keine Liste mehr.
  const last = tokens.value.at(-1)?.kind
  const done = items.length === 1 && (items[0]!.v.toLowerCase() === `${ch}${low}` || (!!last && last !== 'bad'))
  return { title: SUGG_TITLE[ch]!, items: done ? [] : items }
})
const suggSel = computed(() => Math.min(suggIdx.value, sugg.value.items.length - 1))
function accept(v: string) {
  quick.value = quick.value.replace(/[#@!]\S*$/, `${v} `)
  quickEl.value?.focus()
}

// Chips unter dem Feld: zeigen das Erkannte; ein Klick ersetzt das Wort dieser Art (#/@/! öffnen die Vorschläge, Dauer und Priorität schalten weiter).
const DURS = [15, 30, 60, 120]
function setTok(kind: TokenKind, val: string) {
  const rest = tokens.value.filter((x) => x.kind !== kind).map((x) => x.w).join(' ')
  quick.value = [rest, val].filter(Boolean).join(' ') + (val.length > 1 ? ' ' : '')
  quickEl.value?.focus()
}
const chips = computed(() => {
  const q = quickParsed.value
  const when = [q.date && `${dayLabel(q.date, todayStr())}${q.time ? ` ${q.time}` : ''}`, q.rule && `↻ ${ruleText(q.rule)}`].filter(Boolean).join(' · ')
  const dur = q.minutes ? DURS[DURS.indexOf(q.minutes) + 1] : 30 // nach 2h (oder einer krummen Dauer) fällt sie weg
  const prio = ({ '': '!hoch', HIGH: '!mittel', MEDIUM: '!niedrig' } as Record<string, string>)[q.priority ?? ''] ?? ''
  return [
    { kind: 'project' as const, lead: '#', label: q.project?.name ?? t('Projekt'), on: !!q.project, color: q.project ? projectColor(q.project.id) : '', val: '#' },
    { kind: 'date' as const, lead: '@', label: when || t('Fällig'), on: !!when, color: '', val: '@' },
    { kind: 'dur' as const, lead: '~', label: q.minutes ? hm(q.minutes) : t('Dauer'), on: !!q.minutes, color: '', val: dur ? (dur >= 60 ? `${dur / 60}h` : `${dur}m`) : '' },
    { kind: 'prio' as const, lead: '!', label: q.priority ? PRIO_LABEL[q.priority]! : t('Priorität'), on: !!q.priority, color: '', val: prio },
  ]
})
const details = ref(false)
const noteText = ref('')
const subsText = ref('')
const issueRef = ref('')
// „owner/repo#12“ oder ein Link; undefined = kein Issue, null = nicht lesbar.
function issueLink(raw: string) {
  const s = raw.trim()
  if (!s) return undefined
  const m = /^([\w.-]+\/[\w.-]+)#(\d+)$/.exec(s)
  if (m) return { url: `https://github.com/${m[1]}/issues/${m[2]}`, label: s }
  return /^https?:\/\/\S+$/.test(s) ? { url: s, label: '' } : null
}
// Zeitvorschlag wie auf der Startseite; Tasks ohne Schätzung zählen 30 Minuten.
const slotDay = computed(() => {
  const q = quickParsed.value
  return q.title && !q.time ? (q.date ?? todayStr()) : null
})
const nowMin = () => { const d = new Date(); return d.getHours() * 60 + d.getMinutes() }
const { hint: slotHint, slot, blocks, win } = useFreeSlot(() => slotDay.value, () => quickParsed.value.minutes, todayStr, nowMin)
// Lage eines Blocks auf der Mini-Tagesleiste (Arbeitszeit = volle Breite).
const span = (a: number, b: number) => {
  const { start, end } = win.value
  const l = Math.max(a, start)
  return { left: `${((l - start) / (end - start)) * 100}%`, width: `${(Math.max(Math.min(b, end) - l, 0) / (end - start)) * 100}%` }
}
// Zeitvorschlag im Bearbeiten-Dialog: freier Platz am Datum der geöffneten Aufgabe (solange sie keine Uhrzeit hat).
const editSlotDay = computed(() => {
  const t = open.value
  return t && t.planned_date && !t.planned_start_at ? t.planned_date : null
})
const { hint: editSlotHint } = useFreeSlot(() => editSlotDay.value, () => open.value?.estimated_minutes ?? 0, todayStr, nowMin)
const takeEditSlot = (time: string) => { if (open.value) setTime(open.value, time) }
const defaultProject = () => (projectId.value === 'alle' ? null : projectId.value)
// Enter legt an; plan (⌘↵) setzt die Uhrzeit der nächsten freien Lücke. Notiz, Teilaufgaben und Issue kommen aus den Details.
function add(plan = false) {
  const q = quickParsed.value
  if (!q.title) return
  const issue = issueLink(issueRef.value)
  if (issue === null) { error.value = t('GitHub-Issue als owner/repo#12 oder als Link angeben.'); return }
  const time = plan && slotHint.value?.time ? slotHint.value.time.padStart(5, '0') : null
  const note = noteText.value.trim()
  const subs = subsText.value.split('\n').map((x) => x.trim()).filter(Boolean)
  void run(async () => {
    const task = await createTask({ project_id: defaultProject(), ...taskBody(time ? { ...q, date: slotDay.value, time } : q), title: q.title, ...(note ? { description: note } : {}) })
    for (const title of subs) await createTask({ title, parent_task_id: task.id, project_id: task.project_id })
    if (issue) await createResource({ task_id: task.id, type: 'URL', target: issue.url, label: issue.label })
    quick.value = noteText.value = subsText.value = issueRef.value = ''
    details.value = false
    created([task], t('„{title}“ angelegt', { title: task.title }))
  })
}
// Mehrere eingefügte Zeilen werden mehrere Aufgaben, jede mit eigener Erkennung.
function onPaste(e: ClipboardEvent) {
  const lines = (e.clipboardData?.getData('text') ?? '').split(/\r?\n/).map((x) => x.trim()).filter(Boolean)
  if (lines.length < 2) return
  e.preventDefault()
  const qs = lines.map((l) => parseQuickAdd(l, activeProjects.value, todayStr())).filter((q) => q.title)
  void run(async () => {
    const made: Task[] = []
    for (const q of qs) made.push(await createTask({ project_id: defaultProject(), ...taskBody(q), title: q.title }))
    if (made.length) created(made, tn(made.length, '{n} Aufgabe angelegt', '{n} Aufgaben angelegt'))
  })
}
// Neu angelegte Zeilen leuchten kurz auf. Rückgängig löscht endgültig (Teilaufgaben und Links hängen daran);
// „Anzeigen“ gibt es, wenn der aktuelle Reiter die erste neue Aufgabe nicht zeigt.
const fresh = ref<string[]>([])
function created(list: Task[], text: string) {
  const first = list[0]!
  fresh.value = list.map((x) => x.id)
  setTimeout(() => (fresh.value = []), 1800)
  offer(text, () => run(async () => { for (const x of list) await deleteTask(x.id, true) }),
    inScope(first) && matches(first, filter.value) ? undefined : () => focusTask(first.id))
}
function onQuickKey(e: KeyboardEvent) {
  if (e.isComposing) return
  const { items } = sugg.value
  if (items.length && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
    e.preventDefault()
    suggIdx.value = (suggSel.value + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length
  } else if (items.length && ((e.key === 'Tab' && !e.shiftKey) || (e.key === 'Enter' && !e.metaKey && !e.ctrlKey))) {
    e.preventDefault()
    accept(items[suggSel.value]!.v)
  } else if (items.length && e.key === 'Escape') dismissed.value = true
  else if (e.key === 'Enter') { e.preventDefault(); add(e.metaKey || e.ctrlKey) }
  else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'd') { e.preventDefault(); details.value = !details.value }
  else if (e.key === 'Escape') { if (quick.value) quick.value = ''; else quickEl.value?.blur() }
}
function addSub(t: Task) {
  const title = subTitle.value.trim()
  if (!title) return
  void run(async () => {
    await createTask({ title, parent_task_id: t.id, project_id: t.project_id })
    subTitle.value = ''
  })
}

const toggle = (t: Task) =>
  t.status === 'COMPLETED' ? run(() => updateTask(t.id, { status: t.planned_date ? 'PLANNED' : 'BACKLOG' })) : setDone(t, 'COMPLETED', run)

// Löschen legt in den Papierkorb; Rückgängig holt die Task samt Teilaufgaben zurück.
const trash = ({ id, title }: Task) =>
  void run(async () => {
    await deleteTask(id)
    offer(t('„{title}“ gelöscht', { title }), () => run(() => restoreTask(id)))
  })

const startTime = (t: Task) => (t.planned_start_at ? hhmm(new Date(t.planned_start_at)) : '')
// Eine Spalte, ein Format: Datum (+ Uhrzeit), „fällig 12.10.“ ohne Plan, überfällig als „vor 2 Tagen“ (Datum im Tooltip).
const when = (task: Task) => {
  const d = dayOf(task)
  if (!d) return { text: '', tip: '' }
  if (overdue(task)) return { text: daysAgo(d, todayStr()), tip: `${t('Fällig')} ${shortDay(d)}` }
  if (!task.planned_date) return { text: `${t('fällig')} ${dm(d)}`, tip: '' }
  return { text: `${dayLabel(d, todayStr())} ${startTime(task)}`.trim(), tip: '' }
}
const overdue = (t: Task) => isOpen(t) && dayOf(t) !== null && dayOf(t)! < todayStr()

// Datum und Uhrzeit hängen zusammen: Ohne Datum gibt es keine Uhrzeit, ein neues Datum behält sie.
function setDate(t: Task, date: string) {
  const time = startTime(t)
  void run(() => updateTask(t.id, { planned_date: date, planned_start_at: date && time ? new Date(`${date}T${time}`).toISOString() : '' }))
}
function setTime(t: Task, time: string) {
  if (!t.planned_date) return
  void run(() => updateTask(t.id, { planned_start_at: time ? new Date(`${t.planned_date}T${time}`).toISOString() : '' }))
}
const setEstimate = (t: Task, value: string) => void run(() => updateTask(t.id, { estimated_minutes: Number(value) || 0 }))
const setField = (t: Task, field: 'title' | 'description' | 'priority' | 'project_id' | 'status' | 'recurrence_rule', value: string) => {
  if (field === 'title' && !value.trim()) return
  if (field === 'status' && value === 'CANCELLED') return setDone(t, 'CANCELLED', run)
  void run(() => updateTask(t.id, { [field]: value }))
}
// Fälligkeit als Tag; gespeichert wird das Ende dieses Tages in Ortszeit.
const dueDay = (t: Task) => (t.due_at ? ymd(new Date(t.due_at)) : '')
const setDue = (t: Task, day: string) => void run(() => updateTask(t.id, { due_at: day ? new Date(`${day}T23:59:00`).toISOString() : '' }))
const valueOf = (e: Event) => (e.target as HTMLInputElement).value
// Wiederholung: Vorlagen plus „jeden <Wochentag des geplanten Tags>“; eine andere gespeicherte Regel bleibt auswählbar.
const weekdayCode = (day: string) => ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'][new Date(`${day}T12:00:00`).getDay()]
const ruleOptions = (t: Task) => [...new Set([
  ...RULE_PRESETS.map(([r]) => r),
  ...(t.planned_date ? [`FREQ=WEEKLY;BYDAY=${weekdayCode(t.planned_date)}`] : []),
  ...(t.recurrence_rule ? [t.recurrence_rule] : []),
])]

// Tastenkürzel: n neu · j/k wählen · x erledigen · Enter öffnen · s Start/Pause · t heute. Nicht beim Tippen, nicht in Dialogen.
function move(d: number) {
  const l = flat.value
  if (!l.length) return
  const i = l.findIndex((t) => t.id === sel.value)
  sel.value = l[i < 0 ? (d > 0 ? 0 : l.length - 1) : Math.min(Math.max(i + d, 0), l.length - 1)]!.id
  if (openId.value) openId.value = sel.value // das Detail folgt der Auswahl
  void nextTick(() => document.querySelector(`[data-task="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' }))
}
useShortcuts((e) => {
  const el = e.target as HTMLElement
  const t = flat.value.find((x) => x.id === sel.value)
  if (e.key === 'n') { e.preventDefault(); quickEl.value?.focus() }
  else if (e.key === 'j') move(1)
  else if (e.key === 'k') move(-1)
  else if (e.key === 'Enter' && t && !el.closest('button, a')) openId.value = openId.value === t.id ? null : t.id
  else if (e.key === 'x' && t) void toggle(t)
  else if (e.key === 's' && t && isOpen(t)) void run(() => taskAction(t.id, t.status === 'IN_PROGRESS' ? 'pause' : 'start'))
  else if (e.key === 't' && t && isOpen(t)) setDate(t, todayStr())
  else if (/^[1-7]$/.test(e.key)) filter.value = MAIN[Number(e.key) - 1]!
})

// Aus dem Kalender: /tasks?task=ID öffnet die Task, auch wenn sie erledigt oder abgebrochen ist.
const focusFromRoute = () => focusTask(route.query.task)
function focusTask(id: unknown) {
  const t = tasks.value.find((x) => x.id === id)
  if (!t) return
  filter.value = t.status === 'COMPLETED' ? 'erledigt' : t.status === 'CANCELLED' ? 'abgebrochen' : 'alle'
  projectId.value = 'alle'
  search.value = ''
  openId.value = sel.value = t.id
  void nextTick(() => document.querySelector(`[data-task="${t.id}"]`)?.scrollIntoView({ block: 'center' }))
}

// „Neu …“ / ⌘K aus der Sidebar: /tasks?new=… fokussiert die Schnelleingabe.
function focusNew() {
  if (!route.query.new) return
  if (typeof route.query.text === 'string') quick.value = route.query.text // aus der Suche: Eingabe übernehmen
  void nextTick(() => quickEl.value?.focus())
  void router.replace({ query: {} })
}
watch(() => route.query.new, focusNew)

// Aus dem Journal: /tasks?filter=journal wählt den Filter; unbekannte IDs bleiben wirkungslos.
function filterFromRoute() {
  const f = route.query.filter
  if (typeof f === 'string' && FILTERS.some((x) => x.id === f)) filter.value = f
}
watch(() => route.query.filter, filterFromRoute, { immediate: true })

onMounted(() => {
  focusNew()
  void load().then(focusFromRoute)
})
useLiveEvents(load)
</script>

<template>
  <div class="view-inner" :class="{ 'tk-split': open }">
    <div class="tk-left">
    <div class="v-head v-head-row tk-head">
      <h1 class="v-title">{{ $t('Aufgaben') }}</h1>
      <span class="spacer"></span>
      <p class="tk-stats">{{ stats }}</p>
      <button class="btn tk-pick" type="button" :aria-pressed="selection.mode.value" @click="selection.mode.value = !selection.mode.value">{{ $t('Auswählen') }}</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <form class="tk-comp" :class="{ focus: focused }" @submit.prevent="add()">
      <div class="tk-line">
        <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
        <div class="tk-field">
          <div ref="mirror" class="tk-mirror" aria-hidden="true"><span v-for="(g, i) in segs" :key="i" :class="g.kind && `k-${g.kind}`" :style="g.kind === 'project' && quickParsed.project ? { '--tc': projectColor(quickParsed.project.id) } : undefined">{{ g.w }}</span></div>
          <input
            ref="quickEl" v-model="quick" type="text" role="combobox" aria-autocomplete="list" :aria-expanded="sugg.items.length > 0" aria-controls="tk-sugg"
            :aria-activedescendant="sugg.items.length ? `tk-sugg-${suggSel}` : undefined" :placeholder="$t('Neue Aufgabe … z. B. Übungsblatt 5 #Uni 1h @fr !hoch')"
            :aria-label="$t('Neue Aufgabe hinzufügen')" aria-keyshortcuts="n" spellcheck="false" autocomplete="off"
            @keydown="onQuickKey" @paste="onPaste" @focus="focused = true" @blur="focused = false" @scroll="syncScroll" @keyup="syncScroll" @click="syncScroll"
          />
        </div>
      </div>
      <ul v-if="sugg.items.length" id="tk-sugg" class="tk-sugg" role="listbox" :aria-label="sugg.title">
        <li v-for="(o, i) in sugg.items" :id="`tk-sugg-${i}`" :key="o.v" role="option" :aria-selected="i === suggSel" @mousedown.prevent="accept(o.v)">
          <span class="cdot" :style="{ background: o.color || 'var(--tx-muted)' }"></span>{{ o.label }}<span class="sub">{{ o.sub }}</span>
        </li>
        <li class="tk-sugg-help" aria-hidden="true">{{ $t('↑↓ wählen · Tab übernehmen') }}</li>
      </ul>
      <div class="tk-chips">
        <button v-for="c in chips" :key="c.kind" type="button" class="tk-chip" :class="{ on: c.on }" :style="c.color ? { '--cc': c.color } : undefined" @mousedown.prevent @click="setTok(c.kind, c.val)"><b aria-hidden="true">{{ c.lead }}</b>{{ c.label }}</button>
        <button type="button" class="tk-chip" :aria-expanded="details" aria-keyshortcuts="Meta+D" @mousedown.prevent @click="details = !details">{{ $t('Details') }}<kbd class="key" aria-hidden="true">⌘D</kbd></button>
      </div>
      <div v-if="details" class="tk-details">
        <label class="field"><span>{{ $t('Notiz') }}</span><textarea v-model="noteText" class="input" rows="2" :placeholder="$t('Kontext, Link, Gedanke …')"></textarea></label>
        <label class="field"><span>{{ $t('Teilaufgaben, eine pro Zeile') }}</span><textarea v-model="subsText" class="input" rows="2"></textarea></label>
        <label class="field"><span>{{ $t('GitHub-Issue verknüpfen') }}</span><input v-model="issueRef" class="input" placeholder="owner/repo#12" /></label>
      </div>
      <div class="tk-foot">
        <template v-if="quickParsed.title">
          <span><kbd class="key">↵</kbd>{{ $t('Anlegen') }}</span>
          <button v-if="slotHint?.time" type="button" class="tk-plan" @mousedown.prevent @click="add(true)"><kbd class="key">⌘↵</kbd>{{ slotHint.text }}</button>
          <span v-else-if="slotHint">{{ slotHint.text }}</span>
          <span v-if="slotDay" class="tk-day" aria-hidden="true">
            {{ Math.floor(win.start / 60) }}
            <span class="tk-day-bar"><i v-for="b in blocks" :key="b.key" :style="span(b.start, b.end)"></i><i v-if="slot" class="slot" :style="span(slot.start, slot.start + (quickParsed.minutes || 30))"></i></span>
            {{ Math.ceil(win.end / 60) }}
          </span>
        </template>
        <span v-else class="tk-tip">{{ $t('Kürzel: # Projekt · @ Fällig · ! Priorität · 30m / 1h Dauer · mehrere Zeilen einfügen legt mehrere Aufgaben an') }}</span>
      </div>
    </form>

    <div class="tk-toolbar">
    <div class="filterbar tk-bar">
      <div class="seg tk-tabs" role="tablist" :aria-label="$t('Filter')" @keydown="onTabKey">
        <button
          v-for="f in tabs" :id="`tk-tab-${f.id}`" :key="f.id" type="button" role="tab" :aria-selected="filter === f.id" :tabindex="filter === f.id ? 0 : -1"
          @click="filter = f.id"
        >{{ f.label }} <span v-if="counts[f.id]" class="count" :class="{ od: f.id === 'ueber' }">{{ counts[f.id] }}</span></button>
      </div>
      <span class="spacer"></span>
      <SearchField v-model="search" :label="$t('Aufgaben durchsuchen')" />
    </div>
    <div class="tk-opts">
      <span class="spacer"></span>
      <select class="input" :value="extra.some((f) => f.id === filter) ? filter : ''" :aria-label="$t('Weitere Filter')" @change="filter = valueOf($event) || 'alle'">
        <option value="">{{ $t('Weitere Filter') }}</option>
        <option v-for="f in extra" :key="f.id" :value="f.id">{{ f.label }} ({{ counts[f.id] }})</option>
      </select>
      <div class="menu-wrap" @keydown.esc="viewMenu = false">
        <button class="btn" type="button" :aria-expanded="viewMenu" @click="viewMenu = !viewMenu">{{ $t('Ansicht') }}<template v-if="projectId !== 'alle'"> · {{ projects.find((p) => p.id === projectId)?.name }}</template><svg class="ic tk-chev" aria-hidden="true"><use href="#i-chev" /></svg></button>
        <template v-if="viewMenu">
          <div class="menu-scrim" @click="viewMenu = false"></div>
          <div class="menu">
            <label>{{ $t('Projekt') }}
              <select v-model="projectId" class="input">
                <option value="alle">{{ $t('Alle') }}</option>
                <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </label>
            <label>{{ $t('Gruppe') }}
              <select v-model="view.group" class="input"><option v-for="[v, l] in GROUPS" :key="v" :value="v">{{ l }}</option></select>
            </label>
            <label>{{ $t('Sortierung') }}
              <select v-model="view.sort" class="input"><option v-for="[v, l] in SORTS" :key="v" :value="v">{{ l }}</option></select>
            </label>
          </div>
        </template>
      </div>
      <button class="btn" type="button" @click="github = true">{{ $t('GitHub-Issues') }}</button>
    </div>
    </div>
    <div class="tasks-layout" :class="{ 'with-aside': open }">
    <div class="tasks-main" role="tabpanel" :aria-labelledby="`tk-tab-${filter}`">
    <div v-if="!rows.length" class="tk-empty">
      <span class="tk-empty-ic" aria-hidden="true"><svg class="ic"><use href="#i-check" /></svg></span>
      <p class="tk-empty-h">{{ search || projectId !== 'alle' ? $t('Keine Aufgaben.') : EMPTY[filter] ?? $t('Keine Aufgaben.') }}</p>
      <template v-if="(filter === 'alle' || filter === 'ohne') && !search">
        <p class="tk-ex-h">{{ $t('Probier eine Zeile aus – ein Klick füllt sie ins Feld.') }}</p>
        <div class="tk-ex"><button v-for="x in EXAMPLES" :key="x" type="button" @click="quick = x; quickEl?.focus()"><span v-for="(w, i) in x.split(' ')" :key="i" :class="{ tok: /^[#@!]|^\d+[mh]$/.test(w) }">{{ w }}</span></button></div>
      </template>
    </div>
    <div v-if="rows.length" class="tk-cols" aria-hidden="true"><span class="c-t">{{ $t('Aufgabe') }}</span><span class="c-p">{{ $t('Projekt') }}</span><span class="c-d">{{ $t('Dauer') }}</span><span class="c-i">{{ $t('Prio') }}</span><span class="c-f">{{ $t('Fällig') }}</span></div>
    <div v-for="g in groups" :key="g.key" class="tgroup">
      <span v-if="g.title" class="lbl" :class="{ od: view.group === 'due' && g.key === '0' }">{{ g.title }}<span class="count">{{ g.rows.length }}</span><span v-if="g.minutes" class="g-sum">{{ $t('{time} geschätzt', { time: hm(g.minutes) }) }}</span></span>
      <div class="tasklist">
      <template v-for="t in g.rows" :key="t.id">
        <div class="task-row" :class="{ done: !isOpen(t), selected: sel === t.id, picked: picked.includes(t.id), fresh: fresh.includes(t.id) }" :data-task="t.id" @click.capture="pickRow($event, t.id)" @mousedown.shift.prevent @pointerdown="onPointerDown($event, t.id)" @pointerup="onPointerUp" @pointercancel="onPointerUp">
          <button type="button" class="cb sq" role="checkbox" :aria-checked="t.status === 'COMPLETED'" :aria-label="$t('{title} erledigt', { title: t.title })" :disabled="t.status === 'CANCELLED'" @click="toggle(t)"></button>
          <button type="button" class="t row-title" :aria-expanded="openId === t.id" @click="openId = openId === t.id ? null : t.id; sel = t.id; subTitle = ''">{{ t.title }}</button>
          <span v-if="t.recurrence_rule" class="rec-i" role="img" :aria-label="$t('Wiederholt sich: {rule}', { rule: ruleText(t.recurrence_rule) })" :data-tip="ruleText(t.recurrence_rule)">↻</span>
          <span v-if="progress(t.id)" class="badge mono" role="img" :aria-label="`${$t('Teilaufgaben erledigt')}: ${progress(t.id)}`">{{ progress(t.id) }}</span>
          <span v-if="t.status === 'IN_PROGRESS' || t.status === 'PAUSED'" class="status-l" :class="{ run: t.status === 'IN_PROGRESS' }"><span class="cdot"></span>{{ STATUS_LABEL[t.status] }}</span>
          <span class="pcol"><span class="chip" :style="{ '--chip-c': projectColor(t.project_id) }"><template v-if="t.project_id"><span class="cdot"></span>{{ projectName.get(t.project_id) }}</template></span></span>
          <span class="dur">{{ t.estimated_minutes ? hm(t.estimated_minutes) : '' }}</span>
          <span class="prio-i" :class="{ high: urgent(t) }" role="img" :aria-label="`${$t('Priorität')}: ${PRIO_LABEL[t.priority] ?? t.priority}`"><i v-for="n in 3" :key="n" :class="{ on: n <= prioLevel(t) }"></i></span>
          <span class="due" :class="{ od: overdue(t) }" :data-tip="when(t).tip || undefined">{{ when(t).text }}</span>
        </div>
      </template>
      </div>
    </div>
    </div>
    </div>

    <SelectionBar v-if="picked.length || selection.mode.value" :count="picked.length" :projects="projects.filter((p) => p.status !== 'ARCHIVED')"
      @tomorrow="selection.toTomorrow()" @project="selection.toProject" @trash="selection.trash" @clear="selection.clear" />
    <div v-if="!rows.length" class="keys" :aria-label="$t('Tastenkürzel')">
      <span><kbd class="key">n</kbd>{{ $t('Neu') }}</span>
      <span><kbd class="key">j</kbd><kbd class="key">k</kbd>{{ $t('Wählen') }}</span>
      <span><kbd class="key">x</kbd>{{ $t('Erledigt') }}</span>
      <span><kbd class="key">Enter</kbd>{{ $t('Öffnen') }}</span>
      <span><kbd class="key">s</kbd>{{ $t('Start/Pause') }}</span>
      <span><kbd class="key">t</kbd>{{ $t('Heute') }}</span>
      <span><kbd class="key">1</kbd>–<kbd class="key">7</kbd>{{ $t('Reiter') }}</span>
    </div>
    </div>
    <aside v-if="open" class="task-aside" :aria-label="$t('Aufgabendetails')">
      <template v-for="t in [open]" :key="t.id"><!-- t: die geöffnete Task, wie in der Zeile -->
        <div class="aside-head"><span class="lbl">{{ $t('Aufgabe') }}</span><button class="icon-btn" type="button" :aria-label="$t('Details schließen')" @click="openId = null"><svg class="ic"><use href="#i-x" /></svg></button></div>
        <div class="task-detail">
          <input class="td-title" :value="t.title" :aria-label="$t('Titel')" @change="(e) => setField(t, 'title', valueOf(e))" />
          <div class="td-meta">
            <select class="input td-pill" :value="t.project_id ?? ''" :aria-label="$t('Projekt')" :style="{ '--chip-c': projectColor(t.project_id) }" @change="(e) => setField(t, 'project_id', valueOf(e))">
              <option value="">{{ $t('Kein Projekt') }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
            <select class="input td-pill plain" :value="t.status" :aria-label="$t('Status')" @change="(e) => setField(t, 'status', valueOf(e))">
              <option v-for="st in SETTABLE" :key="st" :value="st">{{ STATUS_LABEL[st] }}</option>
              <option v-if="!SETTABLE.includes(t.status)" :value="t.status" disabled>{{ STATUS_LABEL[t.status] }}</option>
            </select>
          </div>
          <TaskActions :task="t" :running="t.status === 'IN_PROGRESS'" :note="resOf(t.id).find((r) => r.type === 'NOTE')?.target" @run="run" />
          <div class="td-props">
            <label class="td-row"><span>{{ $t('Fällig') }}</span><input class="input" type="date" :value="dueDay(t)" @change="(e) => setDue(t, valueOf(e))" /></label>
            <template v-if="isOpen(t)">
              <div class="td-row"><span>{{ $t('Geplant') }}</span>
                <input class="input" type="date" :value="t.planned_date ?? ''" :aria-label="$t('Datum')" @change="(e) => setDate(t, valueOf(e))" />
                <input class="input" type="time" :value="startTime(t)" :disabled="!t.planned_date" :aria-label="$t('Uhrzeit')" @change="(e) => setTime(t, valueOf(e))" />
              </div>
              <label class="td-row"><span>{{ $t('Dauer') }}</span><input class="input" type="number" min="0" :value="t.estimated_minutes" :aria-label="$t('Minuten')" @change="(e) => setEstimate(t, valueOf(e))" /><em>min</em></label>
            </template>
            <label class="td-row"><span>{{ $t('Priorität') }}</span>
              <select class="input" :value="t.priority" @change="(e) => setField(t, 'priority', valueOf(e))">
                <option v-for="(label, key) in PRIO_LABEL" :key="key" :value="key">{{ label }}</option>
              </select>
            </label>
            <label v-if="isOpen(t)" class="td-row"><span>{{ $t('Wiederholung') }}</span>
              <select class="input" :value="t.recurrence_rule ?? ''" @change="(e) => setField(t, 'recurrence_rule', valueOf(e))">
                <option value="">{{ $t('Nicht wiederholen') }}</option>
                <option v-for="r in ruleOptions(t)" :key="r" :value="r">{{ ruleText(r) }}</option>
              </select>
            </label>
          </div>
          <div v-if="isOpen(t)" class="td-quick">
            <button v-if="editSlotHint?.time && !t.planned_start_at" type="button" class="btn btn-secondary" @click="takeEditSlot(editSlotHint.time)">{{ editSlotHint.text }}</button>
            <span v-else-if="editSlotHint && !t.planned_start_at" class="qa-hint">{{ editSlotHint.text }}</span>
            <button v-if="t.planned_date !== todayStr()" type="button" class="btn btn-secondary" @click="setDate(t, todayStr())">{{ $t('Heute') }}</button>
          </div>
          <label class="td-sec"><span class="lbl">{{ $t('Beschreibung') }}</span><textarea class="td-desc" rows="2" :placeholder="$t('Beschreibung hinzufügen')" :value="t.description" @change="(e) => setField(t, 'description', valueOf(e))"></textarea></label>
          <div class="td-sec sub-sec">
            <span class="lbl">{{ $t('Teilaufgaben') }}<template v-if="progress(t.id)"> · {{ progress(t.id) }}</template></span>
            <div v-for="c in kidsOf(t.id)" :key="c.id" class="sub-row" :class="{ done: !isOpen(c) }">
              <button type="button" class="cb" role="checkbox" :aria-checked="c.status === 'COMPLETED'" :aria-label="$t('{title} erledigt', { title: c.title })" :disabled="c.status === 'CANCELLED'" @click="toggle(c)"></button>
              <span>{{ c.title }}</span>
            </div>
            <form class="addrow" @submit.prevent="addSub(t)">
              <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
              <input v-model="subTitle" type="text" :placeholder="$t('Teilaufgabe hinzufügen')" :aria-label="$t('Teilaufgabe hinzufügen')" />
            </form>
          </div>
          <ResourceList class="td-sec" :resources="resOf(t.id)" :task-id="t.id" @changed="load" />
          <div class="td-foot"><span></span><button type="button" class="btn btn-ghost" @click="trash(t)">{{ $t('Löschen') }}</button></div>
        </div>
      </template>
    </aside>
    <div v-if="github" v-dialog="closeGitHub" class="overlay open" @mousedown.self="closeGitHub">
      <div class="dialog gh-dlg" role="dialog" :aria-label="$t('GitHub-Issues')">
        <div class="dlg-head">
          <h3>{{ $t('GitHub-Issues') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="closeGitHub"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <GitHubIssues class="dlg-body" :projects="projects" @open="openImported" @changed="load" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.gh-dlg { width: 680px; }
.keys { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 20px; justify-content: center; margin-top: 24px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.keys span { display: inline-flex; align-items: center; gap: 6px; }
.keys .key { min-width: 20px; text-align: center; color: var(--tx-muted); }
.rec-i { color: var(--tx-muted); font-size: 13px; line-height: 1; }
.task-row.picked { background: var(--bg-selected); }
.task-row.selected { background: var(--bg-2); box-shadow: inset 0 0 0 1px var(--br-default); }
.task-row:hover:not(.selected) { background: var(--bg-hover); }
.row-title { text-align: left; background: none; border: 0; color: inherit; cursor: pointer; }
/* Detail als angedockte Spalte über die volle Höhe; nur die Liste links scrollt. Unter 1000px (und am Handy) als Sheet am unteren Rand. */
.tasks-layout { margin-top: 16px; }
@media (min-width: 1001px) {
  /* Detailspalte bleibt rechts stehen (volle Höhe unter der Kopfzeile), links scrollt die Seite wie sonst. */
  .view-inner.tk-split { display: grid; grid-template-columns: minmax(0, 1fr) 380px; max-width: none; padding: 0; }
  .tk-split .tk-left { min-width: 0; padding: 32px 40px 64px 32px; }
  .tk-split .task-aside { position: sticky; top: 56px; align-self: start; height: calc(100dvh - 56px); }
  .tk-cols { position: sticky; top: 56px; z-index: 2; }
}
.task-aside { display: flex; flex-direction: column; background: var(--bg-1); border-left: 1px solid var(--br-subtle); overflow-y: auto; scrollbar-width: none; }
.task-aside::-webkit-scrollbar { display: none; }
.aside-head { display: flex; align-items: center; justify-content: space-between; padding: 18px 20px 0 24px; }
.task-detail { flex: 1; display: flex; flex-direction: column; gap: 0; padding: 12px 24px 18px; }
.td-title { width: 100%; padding: 4px 0; border: 0; background: none; color: var(--tx-primary); font: 600 20px/1.3 var(--font-ui); letter-spacing: -.015em; }
.td-title:focus-visible { outline: none; box-shadow: 0 1px 0 var(--a-focus); }
.td-meta { display: flex; align-items: center; gap: 8px; margin: 8px 0 18px; }
.td-pill { height: 24px; width: auto; max-width: 180px; padding: 0 22px 0 9px; border-radius: 7px; font: 500 12px/1 var(--font-ui); cursor: pointer; background-color: color-mix(in srgb, var(--chip-c, var(--tx-muted)) 10%, transparent); border: 1px solid color-mix(in srgb, var(--chip-c, var(--tx-muted)) 24%, transparent); color: color-mix(in srgb, var(--chip-c, var(--tx-muted)) 70%, var(--tx-primary)); }
.td-pill.plain { padding-left: 0; border-color: transparent; background-color: transparent; color: var(--tx-secondary); font-weight: 400; }
.task-detail > .row { margin-bottom: 4px; }
.td-props { margin-top: 18px; border-top: 1px solid var(--br-subtle); }
.td-row { display: flex; align-items: center; gap: 8px; height: 40px; border-bottom: 1px solid var(--br-subtle); font: 400 13px/1 var(--font-ui); }
.td-row > span { flex: none; width: 104px; color: var(--tx-muted); }
.td-row > em { font-style: normal; color: var(--tx-muted); font-size: 12px; }
.td-row .input { flex: 1; min-width: 0; width: auto; height: 28px; padding: 0 6px; border: 0; background-color: transparent; color: var(--tx-primary); font: 400 13px/1 var(--font-ui); border-radius: 6px; }
.td-row input[type="number"] { flex: none; width: 56px; font-variant-numeric: tabular-nums; }
.td-row .input:hover { background-color: var(--bg-2); }
.td-quick { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.td-quick:not(:has(*)) { display: none; }
.td-sec { display: block; margin-top: 18px; }
.td-sec > .lbl, .sub-sec > .lbl { display: block; margin-bottom: 8px; font: 600 11px/1 var(--font-ui); letter-spacing: .08em; text-transform: uppercase; color: var(--tx-muted); }
.td-desc { width: 100%; padding: 6px 8px; margin: 0 -8px; border: 0; border-radius: 8px; background: none; color: var(--tx-secondary); font: 400 13px/1.55 var(--font-ui); resize: vertical; }
.td-desc:hover, .td-desc:focus-visible { background: var(--bg-2); }
.td-foot { display: flex; align-items: center; justify-content: space-between; margin-top: auto; padding-top: 14px; border-top: 1px solid var(--br-subtle); font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
@media (max-width: 1000px) {
  .task-aside { position: fixed; inset: auto 0 0 0; z-index: 20; max-height: 65vh; border: 0; border-radius: var(--r-m) var(--r-m) 0 0; box-shadow: var(--shadow-pop); }
}
/* Handy: Sheet über der Tab-Leiste und über dem „Neu“-Knopf (beide z-index 50). */
@media (max-width: 760px) {
  .task-aside { bottom: calc(56px + env(safe-area-inset-bottom)); z-index: 55; max-height: calc(100dvh - 56px - env(safe-area-inset-bottom) - 48px); }
}
.tk-tabs { flex-wrap: wrap; }
.tk-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 8px; padding-bottom: 14px; border-bottom: 1px solid var(--br-subtle); }
.tk-toolbar .tk-bar { flex: 1 1 auto; margin-bottom: 0; }
.tk-opts { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; margin-left: auto; }
@media (max-width: 900px) { .tk-toolbar { display: block; } .tk-opts { margin-top: 8px; } }
.tk-stats { margin: 0 4px 3px 0; font: 400 13px/1 var(--font-ui); font-variant-numeric: tabular-nums; color: var(--tx-muted); }
.tk-pick { height: 28px; padding: 0 12px; border: 0; border-radius: 999px; background: none; box-shadow: none; color: var(--tx-secondary); font-size: 12px; }
.tk-pick:hover, .tk-pick[aria-pressed="true"] { background: var(--bg-2); color: var(--tx-primary); }
.tk-tip { display: none; } /* Kürzel-Hinweis erst beim Tippen, die Chips darüber zeigen sie schon */
.tk-opts select { height: 28px; width: auto; padding: 0 26px 0 10px; font-size: 12px; border-radius: 999px; }
.tk-by { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.spacer { flex: 1; }
.tk-tabs .count.od { color: var(--a-red); }
.tk-opts .menu select { width: 150px; }
.tk-chev { width: 12px; height: 12px; margin-left: 4px; transform: rotate(90deg); }
.tk-opts .btn, .tk-opts select { height: 28px; padding: 0 12px; font-size: 12px; border-radius: 999px; border: 0; background-color: transparent; box-shadow: none; color: var(--tx-secondary); }
.tk-opts select { padding-right: 26px; cursor: pointer; }
.tk-opts .btn:hover, .tk-opts select:hover { background-color: var(--bg-2); color: var(--tx-primary); }
/* Composer: Karte mit eingefärbtem Spiegel hinter dem durchsichtigen Feld; beide Ebenen brauchen dieselbe Schrift. */
.tk-comp { position: relative; margin-bottom: 28px; padding: 16px 18px 14px; border-radius: 14px; background: var(--bg-1); box-shadow: 0 0 0 1px var(--br-default); transition: box-shadow var(--dur); }
.tk-comp.focus .tk-tip { display: block; }
.tk-comp.focus .tk-foot:has(> .tk-tip) { margin-top: 10px; }
.tk-comp:not(.focus) .tk-foot:has(> .tk-tip) { display: none; }
.tk-comp.focus { box-shadow: 0 0 0 1px var(--a-focus), 0 0 0 4px var(--ring); }
.tk-line { display: flex; align-items: center; gap: 10px; }
.tk-line .ic { color: var(--tx-muted); flex: none; }
.tk-field { position: relative; flex: 1; min-width: 0; }
.tk-field input, .tk-mirror { font: 400 15px/24px var(--font-ui); letter-spacing: 0; padding: 0; border: 0; margin: 0; }
.tk-field input { position: relative; display: block; width: 100%; height: 24px; background: transparent; color: transparent; caret-color: var(--tx-primary); outline: none; }
.tk-field input::placeholder { color: var(--tx-muted); }
.tk-field input::selection { background: var(--bg-selected); color: transparent; }
.tk-mirror { position: absolute; inset: 0; white-space: pre; overflow: hidden; color: var(--tx-primary); pointer-events: none; }
.tk-mirror span { --tc: var(--tx-primary); color: var(--tc); border-radius: 4px; }
.tk-mirror [class^="k-"] { background: color-mix(in srgb, var(--tc) 16%, transparent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--tc) 16%, transparent); }
.tk-mirror .k-date { --tc: var(--a-blue); }
.tk-mirror .k-dur { --tc: var(--p-green); }
.tk-mirror .k-prio { --tc: var(--p-orange); }
.tk-mirror .k-bad { --tc: var(--a-red); background: none; box-shadow: none; text-decoration: underline wavy; text-underline-offset: 4px; }
.tk-comp.focus .tk-mirror .k-bad:last-child { --tc: var(--tx-secondary); text-decoration: none; } /* wird gerade getippt */
.tk-sugg { position: absolute; z-index: 30; top: 48px; left: 42px; min-width: 260px; margin: 0; padding: 4px; list-style: none; border-radius: 10px; background: var(--bg-2); box-shadow: 0 0 0 1px var(--br-default), var(--shadow-pop); }
.tk-sugg li[role="option"] { display: flex; align-items: center; gap: 8px; padding: 7px 10px; border-radius: 6px; font: 400 13px/1 var(--font-ui); cursor: pointer; }
.tk-sugg li[aria-selected="true"] { background: var(--bg-hover); }
.tk-sugg .cdot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
.tk-sugg .sub { margin-left: auto; padding-left: 16px; color: var(--tx-muted); }
.tk-sugg-help { padding: 6px 10px 4px; font: 400 11px/1 var(--font-ui); color: var(--tx-muted); }
.tk-chips { display: flex; flex-wrap: wrap; gap: 6px; margin: 10px 0 0 26px; }
.tk-chip { --cc: var(--tx-secondary); display: inline-flex; align-items: center; gap: 4px; height: 24px; padding: 0 9px; border: 1px dashed var(--br-default); border-radius: 999px; background: none; color: var(--tx-muted); font: 400 12px/1 var(--font-ui); cursor: pointer; }
.tk-chip b { font-weight: 500; opacity: .7; }
.tk-chip:hover { color: var(--tx-primary); }
.tk-chip.on { border-style: solid; border-color: color-mix(in srgb, var(--cc) 35%, transparent); background: color-mix(in srgb, var(--cc) 12%, transparent); color: var(--cc); }
.tk-chip .key { padding: 1px 4px; font-size: 10px; }
.tk-details { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 10px; margin: 12px 0 0 26px; }
.tk-details .field { display: grid; gap: 4px; font: 400 12px/1.3 var(--font-ui); color: var(--tx-secondary); }
.tk-foot { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 16px; margin: 12px 0 0 26px; font: 400 12px/1.3 var(--font-ui); color: var(--tx-muted); }
.tk-foot > span, .tk-plan { display: inline-flex; align-items: center; gap: 6px; }
.tk-plan { padding: 0; border: 0; background: none; color: var(--tx-secondary); font: inherit; cursor: pointer; }
.tk-plan:hover { color: var(--tx-primary); }
.tk-foot .key { padding: 2px 5px; font-size: 11px; }
.tk-day { margin-left: auto; font-variant-numeric: tabular-nums; }
.tk-day-bar { position: relative; display: block; width: 140px; height: 6px; border-radius: 3px; background: var(--bg-2); overflow: hidden; }
.tk-day-bar i { position: absolute; top: 0; bottom: 0; background: var(--br-strong); }
.tk-day-bar i.slot { background: var(--a-blue); }
.tk-sum { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); font-variant-numeric: tabular-nums; }
.tk-empty { display: grid; gap: 8px; justify-items: center; text-align: center; padding: 64px 0 48px; }
.tk-empty-ic { display: grid; place-items: center; width: 44px; height: 44px; margin-bottom: 8px; border-radius: 50%; background: var(--bg-1); box-shadow: 0 0 0 1px var(--br-default); color: var(--tx-muted); }
.tk-empty-h { margin: 0; font: 500 15px/1.4 var(--font-ui); letter-spacing: -0.005em; color: var(--tx-primary); }
.tk-ex-h { margin: 0 0 8px; font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.tk-ex { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; max-width: 560px; }
.tk-ex button { display: inline-flex; gap: 5px; height: 32px; align-items: center; padding: 0 14px; border: 0; border-radius: 999px; background: var(--bg-1); box-shadow: 0 0 0 1px var(--br-default); color: var(--tx-secondary); font: 400 13px/1 var(--font-ui); cursor: pointer; transition: background var(--dur), box-shadow var(--dur), color var(--dur); }
.tk-ex button:hover { background: var(--bg-2); box-shadow: 0 0 0 1px var(--br-strong); color: var(--tx-primary); }
.tk-ex .tok { color: var(--tx-primary); font-weight: 500; }
.task-row.fresh { background: var(--bg-selected); transition: background 1s; }
@media (max-width: 640px) {
  .tk-sum, .tk-chip .key, .tk-foot .key, .tk-day, .tk-sugg-help { display: none; }
  .tk-details { grid-template-columns: 1fr; }
  .tk-sugg { left: 16px; right: 16px; min-width: 0; }
  /* Kopf kompakt: Anlegen über Schnelleingabe und „Neu“-Knopf, Filter als eine wischbare Zeile. */
  .tk-head { flex-wrap: wrap; }
  .tk-stats { flex-basis: 100%; order: 3; margin: 6px 0 0; font-size: 14px; color: var(--tx-secondary); }
  .v-head-row .btn-primary, .tk-by, .tk-pick { display: none; } /* Auswählen am Handy per langem Druck auf eine Zeile */
  /* Wischbare Zeilen blenden rechts aus, als Hinweis, dass es weitergeht. .tk-opts per Verlauf statt Maske: die Maske träfe auch das fixierte Ansicht-Menü. */
  .tk-tabs { mask-image: linear-gradient(to right, #000 calc(100% - 32px), transparent); }
  .tk-opts::after { content: ""; position: sticky; right: 0; flex: none; align-self: stretch; width: 32px; margin-left: -32px; background: linear-gradient(to right, transparent, var(--bg-0)); pointer-events: none; }
  .tk-tabs { flex: 1 1 0; min-width: 0; flex-wrap: nowrap; max-width: 100%; overflow-x: auto; scrollbar-width: none; }
  .tk-tabs button { flex: none; }
  .tk-opts { margin-top: 12px; flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; }
  .tk-opts > * { flex: none; }
  .tk-bar .spacer { display: none; }
  .tk-bar > .input { flex-basis: 100%; max-width: none; }
}
.btn-primary .key { margin-left: 6px; background: transparent; color: inherit; border-color: currentColor; opacity: .85; }
.tgroup > .lbl.od { color: var(--a-red); }
.g-sum { margin-left: auto; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.task-detail .field.wide, .task-detail .full { width: 100%; }
.sub-sec { display: grid; gap: 8px; }
.sub-sec .addrow { margin-top: 0; }
.sub-row { display: flex; align-items: center; gap: 10px; font: 400 14px/1.4 var(--font-ui); }
.sub-row.done span { text-decoration: line-through; color: var(--tx-muted); }
</style>
