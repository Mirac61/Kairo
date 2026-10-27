import { setProjectColors } from '@/lib/projectColor'
import { t } from '@/lib/i18n'
import { handleUnauthorized } from '@/composables/useAuth'

export class ApiError extends Error {
  readonly status: number
  readonly body: unknown

  constructor(status: number, statusText: string, body: unknown) {
    super(`${t('API-Fehler')} ${status}${statusText ? ` ${statusText}` : ''}`)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

export interface ApiOptions extends RequestInit {
  skipUnauthorizedHandler?: boolean
}

export async function api<T>(path: string, init?: ApiOptions): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(`/api${path}`, { ...init, headers, credentials: 'include' })
  const text = await res.text()
  let body: unknown = undefined
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = text
    }
  }
  if (!res.ok) {
    if (res.status === 401 && !init?.skipUnauthorizedHandler) {
      handleUnauthorized()
    }
    throw new ApiError(res.status, res.statusText, body)
  }
  return body as T
}

export interface Health {
  status: string
  version: string
}

export const getHealth = () => api<Health>('/health')

export interface Task {
  id: string
  title: string
  description: string
  status: 'BACKLOG' | 'PLANNED' | 'IN_PROGRESS' | 'PAUSED' | 'COMPLETED' | 'CANCELLED'
  priority: string
  estimated_minutes: number
  due_at: string | null
  planned_date: string | null
  planned_start_at: string | null
  project_id: string | null
  parent_task_id: string | null
  recurrence_rule?: string | null
  source_journal_date?: string | null
}

// Offen = weder erledigt noch abgebrochen.
export const isOpen = (t: Pick<Task, 'status'>) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'

export interface TodayEvent {
  id: string
  title: string
  location: string
  occurrence_start: string
  occurrence_end: string
}

export interface TodayHabit {
  id: string
  name: string
  unit: string
  target_value: number | null
  preferred_time: string | null
  done: boolean
  week_progress: { done: number; target: number } | null
}

export interface TimeEntry {
  id: string
  task_id: string | null
  project_id: string | null
  started_at: string
  ended_at: string | null
}

export interface Today {
  date: string
  timezone: string
  events: TodayEvent[]
  tasks: Task[]
  overdue: Task[]
  active_tasks: Task[]
  habits: TodayHabit[]
  running_time_entry: TimeEntry | null
  planned_minutes: number
  calendar_minutes: number
  tracked_minutes: number
  work_minutes: number
  work_start: number // Arbeitsfenster in Minuten seit Mitternacht, 0/0 ohne Fenster
  work_end: number
  free_minutes: number
  overplanned_minutes: number
  unestimated_tasks: number
}

export const getToday = (date?: string) => api<Today>(`/today${date ? `?date=${date}` : ''}`)

export const taskAction = (id: string, action: 'start' | 'pause' | 'complete') =>
  api<unknown>(`/tasks/${id}/${action}`, { method: 'POST' })

// Zeiteinträge mit Start im Fenster [from, to).
export const listTimeEntries = (from: Date, to: Date) =>
  api<TimeEntry[]>(`/time-entries?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`)

export const createTimeEntry = (body: { task_id: string; started_at: string; ended_at: string }) =>
  api<TimeEntry>('/time-entries', { method: 'POST', body: JSON.stringify(body) })

export const updateTimeEntry = (id: string, body: { started_at?: string; ended_at?: string }) =>
  api<TimeEntry>(`/time-entries/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

export const deleteTimeEntry = (id: string) => api<void>(`/time-entries/${id}`, { method: 'DELETE' })

// value setzt bei Mengen-Habits die Menge des Tages (ein zweiter Aufruf überschreibt sie); ohne value gilt der Tag als erledigt.
export const completeHabit = (id: string, date: string, value?: number) =>
  api<unknown>(`/habits/${id}/completions`, { method: 'POST', body: JSON.stringify({ date, value }) })

// Pausen: an diesen Tagen ist nichts fällig, Serien brechen nicht. habit_id null = alle Gewohnheiten (Urlaub).
export interface HabitPause {
  id: string
  habit_id: string | null
  start_date: string
  end_date: string
}
export const listHabitPauses = () => api<HabitPause[]>('/habits/pauses')
export const createHabitPause = (body: { habit_id: string | null; start_date: string; end_date?: string }) =>
  api<HabitPause>('/habits/pauses', { method: 'POST', body: JSON.stringify(body) })
export const deleteHabitPause = (id: string) => api<void>(`/habits/pauses/${id}`, { method: 'DELETE' })

export const uncompleteHabit = (id: string, date: string) =>
  api<unknown>(`/habits/${id}/completions/${date}`, { method: 'DELETE' })

export interface Project {
  id: string
  name: string
  description: string
  local_path: string | null
  github_repo?: string | null
  status: string
  color: string
}

export const listProjects = () => api<Project[]>('/projects').then((ps) => (setProjectColors(ps), ps))

export const listTasks = (query = '') => api<Task[]>(`/tasks${query}`)

export const createTask = (body: Partial<Task>) =>
  api<Task>('/tasks', { method: 'POST', body: JSON.stringify(body) })

export interface JournalSummary {
  date: string
  total: number
  streak: number
}

export const getJournalSummary = () => api<JournalSummary>('/journal/summary')

export interface JournalTaskImport {
  journal_date: string
  task_count: number
  imported_at: string
  titles: string[]
}

export const listJournalTaskImports = () => api<JournalTaskImport[]>('/journal/task-imports')

export const importJournalTasks = (date: string, titles: string[]) =>
  api<JournalTaskImport>(`/journal/${encodeURIComponent(date)}/tasks`, { method: 'POST', body: JSON.stringify({ titles }) })

// DELETE legt in den Papierkorb, permanent löscht endgültig (Task mit Zeiteinträgen: 409).
export const deleteTask = (id: string, permanent = false) =>
  api<void>(`/tasks/${id}${permanent ? '?permanent=true' : ''}`, { method: 'DELETE' })
export const restoreTask = (id: string) => api<unknown>(`/tasks/${id}/restore`, { method: 'POST' })

// Fehlertext aus {"error": "..."} des Backends, sonst Standardtext.
export function errorMessage(e: unknown): string {
  const body = e instanceof ApiError ? (e.body as { error?: string } | undefined) : undefined
  return body?.error ?? t('Backend nicht erreichbar.')
}

export const createProject = (body: Partial<Project>) =>
  api<Project>('/projects', { method: 'POST', body: JSON.stringify(body) })

export const updateProject = (id: string, body: Partial<Project>) =>
  api<Project>(`/projects/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

export const deleteProject = (id: string) => api<void>(`/projects/${id}`, { method: 'DELETE' })

export interface FrequencyConfig {
  weekday?: string
  weekdays?: string[]
  times?: number
}

export interface Habit {
  id: string
  name: string
  description: string
  frequency_type: 'DAILY' | 'WEEKLY' | 'SPECIFIC_WEEKDAYS' | 'TIMES_PER_WEEK'
  frequency_config: FrequencyConfig
  target_value: number | null
  unit: string
  icon: string // Symbolname aus lib/habitIcons.ts, leer = aus dem Namen geraten
  remind: boolean // ntfy zur Uhrzeit, solange offen
  preferred_time: string | null // HH:MM
  duration_minutes: number
  project_id: string | null // gesetzt: erledigt, sobald am Tag duration_minutes im Projekt erfasst sind
  start_date: string
  end_date: string | null
  active: boolean
}

export const listHabits = () => api<Habit[]>('/habits')

export interface HabitDue {
  habit_id: string
  name: string
  date: string
  done: boolean
  preferred_time: string | null
  duration_minutes: number
}

// Fällige Habits je Tag, from/to inklusive, höchstens 42 Tage.
export const habitsDue = (from: string, to: string) => api<HabitDue[]>(`/habits/due?from=${from}&to=${to}`)

export const createHabit = (body: Partial<Habit>) =>
  api<Habit>('/habits', { method: 'POST', body: JSON.stringify(body) })

export const updateHabit = (id: string, body: Partial<Habit>) =>
  api<Habit>(`/habits/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

export const deleteHabit = (id: string, permanent = false) =>
  api<void>(`/habits/${id}${permanent ? '?permanent=true' : ''}`, { method: 'DELETE' })
export const restoreHabit = (id: string) => api<unknown>(`/habits/${id}/restore`, { method: 'POST' })

// Ziele fürs Motivationsboard. progress kommt vom Backend (aus Projekt, Zeiterfassung oder Gewohnheit), bei NONE null.
export type GoalKind = 'NONE' | 'PROJECT_TASKS' | 'PROJECT_HOURS' | 'HABIT_COUNT'
export interface Goal {
  id: string
  title: string
  why: string
  image_path: string | null // im Notizordner, über filesUrl
  kind: GoalKind
  project_id: string | null
  habit_id: string | null
  target: number // Stunden bei PROJECT_HOURS, Anzahl bei HABIT_COUNT
  start_date: string
  due_date: string | null
  archived: boolean
  parent_id: string | null // gesetzt: Teilziel eines Langzeitziels
  progress: { current: number; target: number; percent: number } | null
}

export const listGoals = () => api<Goal[]>('/goals')
export const goalToday = () => api<Goal | null>('/goals/today')
export const createGoal = (body: Partial<Goal>) => api<Goal>('/goals', { method: 'POST', body: JSON.stringify(body) })
export const updateGoal = (id: string, body: Partial<Goal>) =>
  api<Goal>(`/goals/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
export const deleteGoal = (id: string) => api<void>(`/goals/${id}`, { method: 'DELETE' })

// Karteikarten: Zeilen „Frage :: Antwort“ in den Notizen (Fach = erster Ordner) oder direkt angelegt (id gesetzt).
// Fällig nach SM-2. grade: 0 nochmal, 1 schwer, 2 gut, 3 leicht.
export interface Card {
  key: string
  id: string | null
  deck: string
  question: string
  answer: string
  note_path: string | null
  new: boolean
  learning: boolean // zuletzt „Nochmal“, noch nicht wieder gewusst
  due: boolean
  due_date: string | null
  interval_days: number
  next_intervals: [number, number, number, number] // Tage bis zur nächsten Wiederholung je Bewertung
  created_at: string | null
}
export type DueCard = Card
export interface CardLog { key: string; grade: number; at: string }
export interface Deck {
  name: string
  color: string // aus PROJECT_COLORS
  total: number
  due: number
  new_due: number
  learn_due: number
  new_per_day: number // neue Karten pro Tag (Tageslimit)
}
export const listCards = () => api<Card[]>('/cards')
export const dueCards = () => api<Card[]>('/cards/due')
export const listDecks = () => api<Deck[]>('/cards/decks')
export const createCard = (body: { deck: string; question: string; answer: string }) =>
  api<Card>('/cards', { method: 'POST', body: JSON.stringify(body) })
export const updateCard = (id: string, body: { deck: string; question: string; answer: string }) =>
  api<Card>(`/cards/${id}`, { method: 'PUT', body: JSON.stringify(body) })
export const deleteCard = (id: string) => api<void>(`/cards/${id}`, { method: 'DELETE' })
export const setDeck = (name: string, body: { color?: string; new_per_day?: number }) =>
  api<void>(`/cards/decks/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(body) })
export const setDeckColor = (name: string, color: string) => setDeck(name, { color })
export const reviewCard = (key: string, grade: number) =>
  api<{ due: string; interval_days: number }>(`/cards/${key}/review`, { method: 'POST', body: JSON.stringify({ grade }) })
export const undoReview = () => api<{ key: string }>('/cards/undo', { method: 'POST' })
export const cardLog = () => api<CardLog[]>('/cards/log')
export const renameDeck = (name: string, to: string) =>
  api<void>(`/cards/decks/${encodeURIComponent(name)}`, { method: 'PATCH', body: JSON.stringify({ name: to }) })
export const deleteDeck = (name: string) => api<void>(`/cards/decks/${encodeURIComponent(name)}`, { method: 'DELETE' })

export interface EventBody {
  title?: string
  start_at?: string
  end_at?: string
  location?: string
  recurrence_rule?: string // "" entfernt die Wiederholung
  recurrence_exdates?: string[] // ersetzt die ganze Liste ("YYYY-MM-DD")
  project_id?: string // "" entfernt die Verknüpfung
}

export const createEvent = (body: EventBody) =>
  api<unknown>('/calendar/events', { method: 'POST', body: JSON.stringify(body) })

export const deleteEvent = (id: string, permanent = false) =>
  api<void>(`/calendar/events/${id}${permanent ? '?permanent=true' : ''}`, { method: 'DELETE' })
export const restoreEvent = (id: string) => api<unknown>(`/calendar/events/${id}/restore`, { method: 'POST' })

// Beim Ändern entfernt "" einen optionalen Wert (due_at, planned_date, …); null lässt ihn unverändert.
export const updateTask = (id: string, body: Record<string, unknown>) =>
  api<Task>(`/tasks/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

// Mehrere Aufgaben in einem Aufruf ändern (Sammelaktionen der Mehrfachauswahl).
export const bulkUpdateTasks = (updates: { id: string; [k: string]: unknown }[]) =>
  api<{ updated: number }>('/tasks/bulk', { method: 'POST', body: JSON.stringify({ updates }) })

export interface Resource {
  id: string
  task_id: string | null
  project_id: string | null
  type: 'FILE' | 'FOLDER' | 'URL' | 'NOTE' // NOTE: Pfad im Notizordner
  target: string
  label: string
}

export const listResources = () => api<Resource[]>('/resources')

export const createResource = (body: Partial<Resource>) =>
  api<Resource>('/resources', { method: 'POST', body: JSON.stringify(body) })

export const deleteResource = (id: string) => api<void>(`/resources/${id}`, { method: 'DELETE' })

export interface CalendarEvent extends TodayEvent {
  start_at: string
  end_at: string
  recurrence_rule: string | null
  project_id: string | null
}

// Konkrete Termine im Fenster [from, to), Serien aufgelöst.
export const getOccurrences = (from: Date, to: Date) =>
  api<CalendarEvent[]>(`/calendar/occurrences?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`)

export const updateEvent = (id: string, body: EventBody) =>
  api<unknown>(`/calendar/events/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

// Lässt eine Instanz der Serie aus; PATCH ersetzt die Liste, daher erst die bisherigen Ausnahmen lesen.
// Liefert die bisherige Liste (für Rückgängig).
export async function skipOccurrence(id: string, day: string) {
  const e = await api<{ recurrence_exdates: string[] }>(`/calendar/events/${id}`)
  await updateEvent(id, { recurrence_exdates: [...e.recurrence_exdates, day] })
  return e.recurrence_exdates
}

export interface Review {
  from: string
  to: string
  timezone: string
  tracked_minutes: number
  completed_tasks: number
  days: { date: string; tracked_minutes: number; calendar_minutes: number; completed_tasks: number }[]
  projects: { project_id: string | null; name: string; tracked_minutes: number; completed_tasks: number; done_tasks: number; total_tasks: number }[]
  habits: { habit_id: string; name: string; done: number; streak: number }[]
}

// Auswertung für [from, to] ("YYYY-MM-DD", beide inklusive).
export const getReview = (from: string, to: string) => api<Review>(`/review?from=${from}&to=${to}`)

// Papierkorb: jeweils zuletzt gelöschte zuerst.
export interface Trash {
  tasks: (Task & { deleted_at: string })[]
  events: { id: string; title: string; start_at: string; recurrence_rule: string | null; deleted_at: string }[]
  habits: (Habit & { deleted_at: string })[]
}

export const getTrash = () => api<Trash>('/trash')

// ICS-Import.
export interface IcsImportResult {
  created: number
  updated: number
  skipped: number
  unsupported_rules: number
  notes: string[]
}

// Schickt den rohen Text einer .ics-Datei; Termine mit bekannter UID werden aktualisiert.
export const importIcs = (text: string) =>
  api<IcsImportResult>('/calendar/import', { method: 'POST', headers: { 'Content-Type': 'text/calendar' }, body: text })

// Notizen: Markdown-Dateien, PDFs und Bilder im Notizordner (Standard ~/Kairo, siehe Einstellungen). path ist relativ, mit „/“.
export interface NoteNode {
  name: string
  path: string
  dir: boolean
  children?: NoteNode[]
}

// mtime ist die Version beim Laden; das Backend lehnt Speichern mit 409 ab, wenn die Datei inzwischen anders aussieht.
export interface Note {
  path: string
  content: string
  mtime: string
}

const notePath = (path: string) => `/notes/${path.split('/').map(encodeURIComponent).join('/')}`

export const listNotes = () => api<NoteNode[]>('/notes')

export const readNote = (path: string) => api<Note>(notePath(path))

export const saveNote = (path: string, content: string, mtime: string, force = false) =>
  api<{ mtime: string }>(notePath(path), { method: 'PUT', body: JSON.stringify({ content, mtime, force }) })

// Notizen, die per [[…]] auf path verweisen (snippet: die Zeile mit dem Verweis).
export interface Backlink {
  path: string
  title: string
  snippet: string
}

export const listBacklinks = (path: string) => api<Backlink[]>(`/notes/backlinks?path=${encodeURIComponent(path)}`)

export const createNote = (path: string, dir = false) =>
  api<{ path: string; mtime?: string }>(notePath(path), { method: 'POST', body: JSON.stringify({ dir }) })

// Umbenennen oder Verschieben; 409, wenn das Ziel schon existiert.
export const moveNote = (path: string, to: string) =>
  api<{ path: string }>(notePath(path), { method: 'PATCH', body: JSON.stringify({ to }) })

// Löschen verschiebt nach .trash/ im Notizordner (dort von Hand wiederherstellbar).
export const deleteNote = (path: string) => api<{ trash: string }>(notePath(path), { method: 'DELETE' })

// PDFs und Bilder (Rohbytes). Ausgeliefert werden sie unter filesUrl (lib/noteFiles).
const filePath = (path: string) => `/files/${path.split('/').map(encodeURIComponent).join('/')}`

// Legt eine Datei an; 409, wenn der Name schon existiert.
export const uploadFile = (path: string, file: Blob) =>
  api<{ path: string }>(filePath(path), { method: 'POST', headers: { 'Content-Type': file.type }, body: file })

// Lädt eine Datei samt Version (mtime) zum Bearbeiten.
export async function fetchFile(path: string) {
  const res = await fetch(`/api${filePath(path)}`)
  if (!res.ok) throw new ApiError(res.status, res.statusText, await res.json().catch(() => undefined))
  return { blob: await res.blob(), mtime: res.headers.get('X-Kairo-Mtime') ?? '' }
}

// Speichert eine bearbeitete Datei; 409, wenn sie sich seit dem Laden geändert hat (außer force). Die alte Fassung landet in .trash.
export const replaceFile = (path: string, file: Blob, mtime: string, force = false) =>
  api<{ mtime: string }>(`${filePath(path)}?mtime=${encodeURIComponent(mtime)}${force ? '&force=1' : ''}`, {
    method: 'PUT', headers: { 'Content-Type': file.type }, body: file,
  })

export interface SettingsFile {
  notes_dir?: string
  work_start?: string
  work_end?: string
  timezone?: string
  ntfy_lead?: string // Go-Dauer, z. B. „10m0s“
}

export interface Settings {
  settings: SettingsFile
  // Feld → Umgebungsvariable, die es festlegt; diese Felder sind in der WebUI gesperrt.
  env: Partial<Record<keyof SettingsFile, string>>
  path: string
  restart_needed: boolean
}

export const getSettings = () => api<Settings>('/settings')

export const saveSettings = (body: SettingsFile) =>
  api<Settings>('/settings', { method: 'PUT', body: JSON.stringify(body) })

export const restartBackend = () => api<void>('/restart', { method: 'POST' })
