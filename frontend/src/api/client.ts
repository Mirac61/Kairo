import { setProjectColors } from '@/lib/projectColor'
import { t } from '@/lib/i18n'
import { handleUnauthorized } from '@/composables/useAuth'

// ============================================================================
// Core HTTP Client & Error Handling
// ============================================================================

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

  const res = await fetch(`/api${path}`, {
    ...init,
    headers,
    credentials: 'include',
  })

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

// Fehlertext aus {"error": "..."} des Backends, sonst Standardtext.
export function errorMessage(err: unknown): string {
  const body = err instanceof ApiError ? (err.body as { error?: string } | undefined) : undefined
  return body?.error ?? t('Backend nicht erreichbar.')
}

// ============================================================================
// System & Health
// ============================================================================

export interface Health {
  status: string
  version: string
}

export function getHealth(): Promise<Health> {
  return api<Health>('/health')
}

// ============================================================================
// Tasks
// ============================================================================

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
export function isOpen(task: Pick<Task, 'status'>): boolean {
  return task.status !== 'COMPLETED' && task.status !== 'CANCELLED'
}

export function listTasks(query = ''): Promise<Task[]> {
  return api<Task[]>(`/tasks${query}`)
}

export function createTask(body: Partial<Task>): Promise<Task> {
  return api<Task>('/tasks', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

// Beim Ändern entfernt "" einen optionalen Wert (due_at, planned_date, …); null lässt ihn unverändert.
export function updateTask(id: string, body: Record<string, unknown>): Promise<Task> {
  return api<Task>(`/tasks/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

// DELETE legt in den Papierkorb, permanent löscht endgültig (Task mit Zeiteinträgen: 409).
export function deleteTask(id: string, permanent = false): Promise<void> {
  const queryParam = permanent ? '?permanent=true' : ''
  return api<void>(`/tasks/${id}${queryParam}`, {
    method: 'DELETE',
  })
}

export function restoreTask(id: string): Promise<unknown> {
  return api<unknown>(`/tasks/${id}/restore`, {
    method: 'POST',
  })
}

export function taskAction(id: string, action: 'start' | 'pause' | 'complete'): Promise<unknown> {
  return api<unknown>(`/tasks/${id}/${action}`, {
    method: 'POST',
  })
}

// Mehrere Aufgaben in einem Aufruf ändern (Sammelaktionen der Mehrfachauswahl).
export function bulkUpdateTasks(
  updates: { id: string; [k: string]: unknown }[],
): Promise<{ updated: number }> {
  return api<{ updated: number }>('/tasks/bulk', {
    method: 'POST',
    body: JSON.stringify({ updates }),
  })
}

// ============================================================================
// Projects
// ============================================================================

export interface Project {
  id: string
  name: string
  description: string
  local_path: string | null
  github_repo?: string | null
  status: string
  color: string
  icon?: string
}

export function listProjects(): Promise<Project[]> {
  return api<Project[]>('/projects').then((projects) => {
    setProjectColors(projects)
    return projects
  })
}

export function createProject(body: Partial<Project>): Promise<Project> {
  return api<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateProject(id: string, body: Partial<Project>): Promise<Project> {
  return api<Project>(`/projects/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteProject(id: string): Promise<void> {
  return api<void>(`/projects/${id}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Resources (Files, Notes, URLs linked to Tasks/Projects)
// ============================================================================

export interface Resource {
  id: string
  task_id: string | null
  project_id: string | null
  type: 'FILE' | 'FOLDER' | 'URL' | 'NOTE' // NOTE: Pfad im Notizordner
  target: string
  label: string
}

export function listResources(): Promise<Resource[]> {
  return api<Resource[]>('/resources')
}

export function createResource(body: Partial<Resource>): Promise<Resource> {
  return api<Resource>('/resources', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteResource(id: string): Promise<void> {
  return api<void>(`/resources/${id}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Habits & Pauses
// ============================================================================

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

export interface HabitDue {
  habit_id: string
  name: string
  date: string
  done: boolean
  preferred_time: string | null
  duration_minutes: number
}

export interface HabitPause {
  id: string
  habit_id: string | null
  start_date: string
  end_date: string
}

export function listHabits(): Promise<Habit[]> {
  return api<Habit[]>('/habits')
}

// Fällige Habits je Tag, from/to inklusive, höchstens 42 Tage.
export function habitsDue(from: string, to: string): Promise<HabitDue[]> {
  return api<HabitDue[]>(
    `/habits/due?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
  )
}

export function createHabit(body: Partial<Habit>): Promise<Habit> {
  return api<Habit>('/habits', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateHabit(id: string, body: Partial<Habit>): Promise<Habit> {
  return api<Habit>(`/habits/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteHabit(id: string, permanent = false): Promise<void> {
  const queryParam = permanent ? '?permanent=true' : ''
  return api<void>(`/habits/${id}${queryParam}`, {
    method: 'DELETE',
  })
}

export function restoreHabit(id: string): Promise<unknown> {
  return api<unknown>(`/habits/${id}/restore`, {
    method: 'POST',
  })
}

// value setzt bei Mengen-Habits die Menge des Tages (ein zweiter Aufruf überschreibt sie); ohne value gilt der Tag als erledigt.
export function completeHabit(id: string, date: string, value?: number): Promise<unknown> {
  return api<unknown>(`/habits/${id}/completions`, {
    method: 'POST',
    body: JSON.stringify({ date, value }),
  })
}

export function uncompleteHabit(id: string, date: string): Promise<unknown> {
  return api<unknown>(`/habits/${id}/completions/${encodeURIComponent(date)}`, {
    method: 'DELETE',
  })
}

export function listHabitPauses(): Promise<HabitPause[]> {
  return api<HabitPause[]>('/habits/pauses')
}

export function createHabitPause(body: {
  habit_id: string | null
  start_date: string
  end_date?: string
}): Promise<HabitPause> {
  return api<HabitPause>('/habits/pauses', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteHabitPause(id: string): Promise<void> {
  return api<void>(`/habits/pauses/${id}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Time Tracking & Entries
// ============================================================================

export interface TimeEntry {
  id: string
  task_id: string | null
  project_id: string | null
  started_at: string
  ended_at: string | null
}

// Zeiteinträge mit Start im Fenster [from, to).
export function listTimeEntries(from: Date, to: Date): Promise<TimeEntry[]> {
  const fromIso = encodeURIComponent(from.toISOString())
  const toIso = encodeURIComponent(to.toISOString())
  return api<TimeEntry[]>(`/time-entries?from=${fromIso}&to=${toIso}`)
}

export function createTimeEntry(body: {
  task_id: string
  started_at: string
  ended_at: string
}): Promise<TimeEntry> {
  return api<TimeEntry>('/time-entries', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateTimeEntry(
  id: string,
  body: { started_at?: string; ended_at?: string },
): Promise<TimeEntry> {
  return api<TimeEntry>(`/time-entries/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteTimeEntry(id: string): Promise<void> {
  return api<void>(`/time-entries/${id}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Calendar & Events
// ============================================================================

export interface TodayEvent {
  id: string
  title: string
  location: string
  occurrence_start: string
  occurrence_end: string
}

export interface CalendarEvent extends TodayEvent {
  start_at: string
  end_at: string
  recurrence_rule: string | null
  project_id: string | null
}

export interface EventBody {
  title?: string
  start_at?: string
  end_at?: string
  location?: string
  recurrence_rule?: string // "" entfernt die Wiederholung
  recurrence_exdates?: string[] // ersetzt die ganze Liste ("YYYY-MM-DD")
  project_id?: string // "" entfernt die Verknüpfung
}

export function createEvent(body: EventBody): Promise<unknown> {
  return api<unknown>('/calendar/events', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateEvent(id: string, body: EventBody): Promise<unknown> {
  return api<unknown>(`/calendar/events/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteEvent(id: string, permanent = false): Promise<void> {
  const queryParam = permanent ? '?permanent=true' : ''
  return api<void>(`/calendar/events/${id}${queryParam}`, {
    method: 'DELETE',
  })
}

export function restoreEvent(id: string): Promise<unknown> {
  return api<unknown>(`/calendar/events/${id}/restore`, {
    method: 'POST',
  })
}

// Konkrete Termine im Fenster [from, to), Serien aufgelöst.
export function getOccurrences(from: Date, to: Date): Promise<CalendarEvent[]> {
  const fromIso = encodeURIComponent(from.toISOString())
  const toIso = encodeURIComponent(to.toISOString())
  return api<CalendarEvent[]>(`/calendar/occurrences?from=${fromIso}&to=${toIso}`)
}

// Lässt eine Instanz der Serie aus; PATCH ersetzt die Liste, daher erst die bisherigen Ausnahmen lesen.
// Liefert die bisherige Liste (für Rückgängig).
export async function skipOccurrence(id: string, day: string): Promise<string[]> {
  const eventDetails = await api<{ recurrence_exdates: string[] }>(`/calendar/events/${id}`)
  await updateEvent(id, { recurrence_exdates: [...eventDetails.recurrence_exdates, day] })
  return eventDetails.recurrence_exdates
}

export interface IcsImportResult {
  created: number
  updated: number
  skipped: number
  unsupported_rules: number
  notes: string[]
}

// Schickt den rohen Text einer .ics-Datei; Termine mit bekannter UID werden aktualisiert.
export function importIcs(text: string): Promise<IcsImportResult> {
  return api<IcsImportResult>('/calendar/import', {
    method: 'POST',
    headers: { 'Content-Type': 'text/calendar' },
    body: text,
  })
}

// ============================================================================
// Today (Dashboard Aggregate)
// ============================================================================

export interface TodayHabit {
  id: string
  name: string
  unit: string
  target_value: number | null
  preferred_time: string | null
  done: boolean
  week_progress: { done: number; target: number } | null
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

export function getToday(date?: string): Promise<Today> {
  const queryParam = date ? `?date=${encodeURIComponent(date)}` : ''
  return api<Today>(`/today${queryParam}`)
}

// ============================================================================
// Goals
// ============================================================================

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
  history: { date: string; percent: number }[] // Verlauf von start_date bis heute; leer ohne Fortschritt
}

export function listGoals(): Promise<Goal[]> {
  return api<Goal[]>('/goals')
}

export function goalToday(): Promise<Goal | null> {
  return api<Goal | null>('/goals/today')
}

export function createGoal(body: Partial<Goal>): Promise<Goal> {
  return api<Goal>('/goals', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateGoal(id: string, body: Partial<Goal>): Promise<Goal> {
  return api<Goal>(`/goals/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteGoal(id: string): Promise<void> {
  return api<void>(`/goals/${id}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Cards (Spaced Repetition / SM-2)
// ============================================================================

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

export interface CardLog {
  key: string
  grade: number
  at: string
}

export interface Deck {
  name: string
  color: string // aus PROJECT_COLORS
  total: number
  due: number
  new_due: number
  learn_due: number
  new_per_day: number // neue Karten pro Tag (Tageslimit)
}

export function listCards(): Promise<Card[]> {
  return api<Card[]>('/cards')
}

export function dueCards(): Promise<Card[]> {
  return api<Card[]>('/cards/due')
}

export function listDecks(): Promise<Deck[]> {
  return api<Deck[]>('/cards/decks')
}

export function createCard(body: {
  deck: string
  question: string
  answer: string
}): Promise<Card> {
  return api<Card>('/cards', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateCard(
  id: string,
  body: { deck: string; question: string; answer: string },
): Promise<Card> {
  return api<Card>(`/cards/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function deleteCard(id: string): Promise<void> {
  return api<void>(`/cards/${id}`, {
    method: 'DELETE',
  })
}

export function setDeck(
  name: string,
  body: { color?: string; new_per_day?: number },
): Promise<void> {
  return api<void>(`/cards/decks/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function setDeckColor(name: string, color: string): Promise<void> {
  return setDeck(name, { color })
}

export function reviewCard(
  key: string,
  grade: number,
): Promise<{ due: string; interval_days: number }> {
  return api<{ due: string; interval_days: number }>(`/cards/${key}/review`, {
    method: 'POST',
    body: JSON.stringify({ grade }),
  })
}

export function undoReview(): Promise<{ key: string }> {
  return api<{ key: string }>('/cards/undo', {
    method: 'POST',
  })
}

export function cardLog(): Promise<CardLog[]> {
  return api<CardLog[]>('/cards/log')
}

export function renameDeck(name: string, to: string): Promise<void> {
  return api<void>(`/cards/decks/${encodeURIComponent(name)}`, {
    method: 'PATCH',
    body: JSON.stringify({ name: to }),
  })
}

export function deleteDeck(name: string): Promise<void> {
  return api<void>(`/cards/decks/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
}

// ============================================================================
// Journal
// ============================================================================

export interface JournalSummary {
  date: string
  total: number
  streak: number
}

export function getJournalSummary(): Promise<JournalSummary> {
  return api<JournalSummary>('/journal/summary')
}

export interface JournalTaskImport {
  journal_date: string
  task_count: number
  imported_at: string
  titles: string[]
}

export function listJournalTaskImports(): Promise<JournalTaskImport[]> {
  return api<JournalTaskImport[]>('/journal/task-imports')
}

export function importJournalTasks(date: string, titles: string[]): Promise<JournalTaskImport> {
  return api<JournalTaskImport>(`/journal/${encodeURIComponent(date)}/tasks`, {
    method: 'POST',
    body: JSON.stringify({ titles }),
  })
}

// ============================================================================
// Notes & Raw Files
// ============================================================================

export interface NoteNode {
  name: string
  path: string
  dir: boolean
  children?: NoteNode[]
}

export interface Note {
  path: string
  content: string
  mtime: string
}

export interface Backlink {
  path: string
  title: string
  snippet: string
}

function encodePathParts(path: string): string {
  return path.split('/').map(encodeURIComponent).join('/')
}

function notePath(path: string): string {
  return `/notes/${encodePathParts(path)}`
}

function filePath(path: string): string {
  return `/files/${encodePathParts(path)}`
}

export function listNotes(): Promise<NoteNode[]> {
  return api<NoteNode[]>('/notes')
}

export function readNote(path: string): Promise<Note> {
  return api<Note>(notePath(path))
}

export function saveNote(
  path: string,
  content: string,
  mtime: string,
  force = false,
): Promise<{ mtime: string }> {
  return api<{ mtime: string }>(notePath(path), {
    method: 'PUT',
    body: JSON.stringify({ content, mtime, force }),
  })
}

export function listBacklinks(path: string): Promise<Backlink[]> {
  return api<Backlink[]>(`/notes/backlinks?path=${encodeURIComponent(path)}`)
}

export function createNote(path: string, dir = false): Promise<{ path: string; mtime?: string }> {
  return api<{ path: string; mtime?: string }>(notePath(path), {
    method: 'POST',
    body: JSON.stringify({ dir }),
  })
}

export function moveNote(path: string, to: string): Promise<{ path: string }> {
  return api<{ path: string }>(notePath(path), {
    method: 'PATCH',
    body: JSON.stringify({ to }),
  })
}

export function deleteNote(path: string): Promise<{ trash: string }> {
  return api<{ trash: string }>(notePath(path), {
    method: 'DELETE',
  })
}

export function uploadFile(path: string, file: Blob): Promise<{ path: string }> {
  return api<{ path: string }>(filePath(path), {
    method: 'POST',
    headers: { 'Content-Type': file.type },
    body: file,
  })
}

export async function fetchFile(path: string): Promise<{ blob: Blob; mtime: string }> {
  const res = await fetch(`/api${filePath(path)}`)
  if (!res.ok) {
    const errorBody = await res.json().catch(() => undefined)
    throw new ApiError(res.status, res.statusText, errorBody)
  }
  return {
    blob: await res.blob(),
    mtime: res.headers.get('X-Kairo-Mtime') ?? '',
  }
}

export function replaceFile(
  path: string,
  file: Blob,
  mtime: string,
  force = false,
): Promise<{ mtime: string }> {
  const forceParam = force ? '&force=1' : ''
  const url = `${filePath(path)}?mtime=${encodeURIComponent(mtime)}${forceParam}`
  return api<{ mtime: string }>(url, {
    method: 'PUT',
    headers: { 'Content-Type': file.type },
    body: file,
  })
}

// ============================================================================
// Review & Trash
// ============================================================================

export interface Review {
  from: string
  to: string
  timezone: string
  tracked_minutes: number
  completed_tasks: number
  days: {
    date: string
    tracked_minutes: number
    calendar_minutes: number
    completed_tasks: number
  }[]
  projects: {
    project_id: string | null
    name: string
    tracked_minutes: number
    completed_tasks: number
    done_tasks: number
    total_tasks: number
  }[]
  habits: {
    habit_id: string
    name: string
    done: number
    streak: number
  }[]
}

// Auswertung für [from, to] ("YYYY-MM-DD", beide inklusive).
export function getReview(from: string, to: string): Promise<Review> {
  return api<Review>(`/review?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}

export interface Trash {
  tasks: (Task & { deleted_at: string })[]
  events: {
    id: string
    title: string
    start_at: string
    recurrence_rule: string | null
    deleted_at: string
  }[]
  habits: (Habit & { deleted_at: string })[]
}

export function getTrash(): Promise<Trash> {
  return api<Trash>('/trash')
}

// ============================================================================
// Settings
// ============================================================================

export interface SettingsFile {
  notes_dir?: string
  work_start?: string
  work_end?: string
  timezone?: string
  ntfy_lead?: string // Go-Dauer, z. B. „10m0s“
}

export interface Settings {
  settings: SettingsFile
  env: Partial<Record<keyof SettingsFile, string>>
  path: string
  restart_needed: boolean
}

export function getSettings(): Promise<Settings> {
  return api<Settings>('/settings')
}

export function saveSettings(body: SettingsFile): Promise<Settings> {
  return api<Settings>('/settings', {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function restartBackend(): Promise<void> {
  return api<void>('/restart', {
    method: 'POST',
  })
}
