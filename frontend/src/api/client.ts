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
  started_at: string
  ended_at: string | null
}

export interface Today {
  date: string
  timezone: string
  events: TodayEvent[]
  tasks: Task[]
  active_tasks: Task[]
  habits: TodayHabit[]
  running_time_entry: TimeEntry | null
  planned_minutes: number
  calendar_minutes: number
  tracked_minutes: number
}

export const getToday = (date?: string) => api<Today>(`/today${date ? `?date=${date}` : ''}`)

export const taskAction = (id: string, action: 'start' | 'pause' | 'complete') =>
  api<unknown>(`/tasks/${id}/${action}`, { method: 'POST' })

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
}

export const listProjects = () => api<Project[]>('/projects')

export const listTasks = (query = '') => api<Task[]>(`/tasks${query}`)

export const createTask = (body: Partial<Task>) =>
  api<Task>('/tasks', { method: 'POST', body: JSON.stringify(body) })

export const deleteTask = (id: string) => api<void>(`/tasks/${id}`, { method: 'DELETE' })

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
