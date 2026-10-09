import { setProjectColors } from '@/lib/projectColor'

export class ApiError extends Error {
  readonly status: number
  readonly body: unknown

  constructor(status: number, statusText: string, body: unknown) {
    super(`API-Fehler ${status}${statusText ? ` ${statusText}` : ''}`)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(`/api${path}`, { ...init, headers })
  const text = await res.text()
  let body: unknown = undefined
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = text
    }
  }
  if (!res.ok) throw new ApiError(res.status, res.statusText, body)
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

export const completeHabit = (id: string, date: string) =>
  api<unknown>(`/habits/${id}/completions`, { method: 'POST', body: JSON.stringify({ date }) })

export const uncompleteHabit = (id: string, date: string) =>
  api<unknown>(`/habits/${id}/completions/${date}`, { method: 'DELETE' })

export interface Project {
  id: string
  name: string
  description: string
  local_path: string | null
  status: string
  color: string
}

export const listProjects = () => api<Project[]>('/projects').then((ps) => (setProjectColors(ps), ps))

export const listTasks = (query = '') => api<Task[]>(`/tasks${query}`)

export const createTask = (body: Partial<Task>) =>
  api<Task>('/tasks', { method: 'POST', body: JSON.stringify(body) })

// DELETE legt in den Papierkorb, permanent löscht endgültig (Task mit Zeiteinträgen: 409).
export const deleteTask = (id: string, permanent = false) =>
  api<void>(`/tasks/${id}${permanent ? '?permanent=true' : ''}`, { method: 'DELETE' })
export const restoreTask = (id: string) => api<unknown>(`/tasks/${id}/restore`, { method: 'POST' })

// Fehlertext aus {"error": "..."} des Backends, sonst Standardtext.
export function errorMessage(e: unknown): string {
  const body = e instanceof ApiError ? (e.body as { error?: string } | undefined) : undefined
  return body?.error ?? 'Backend nicht erreichbar.'
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
  active: boolean
}

export const listHabits = () => api<Habit[]>('/habits')

export const createHabit = (body: Partial<Habit>) =>
  api<Habit>('/habits', { method: 'POST', body: JSON.stringify(body) })

export const updateHabit = (id: string, body: Partial<Habit>) =>
  api<Habit>(`/habits/${id}`, { method: 'PATCH', body: JSON.stringify(body) })

export const deleteHabit = (id: string, permanent = false) =>
  api<void>(`/habits/${id}${permanent ? '?permanent=true' : ''}`, { method: 'DELETE' })
export const restoreHabit = (id: string) => api<unknown>(`/habits/${id}/restore`, { method: 'POST' })

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

export interface Resource {
  id: string
  task_id: string | null
  project_id: string | null
  type: 'FILE' | 'FOLDER' | 'URL'
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

// Notizen: Markdown-Dateien, PDFs und Bilder im Notizordner (Standard ~/life-os). path ist relativ, mit „/“.
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
