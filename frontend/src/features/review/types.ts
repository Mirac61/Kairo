import type { Task } from '@/api/client'

// Die API liefert completed_at, Task kennt es noch nicht.
export type DoneTask = Task & { completed_at: string | null }

export type ReviewDay = {
  date: string
  calendar_minutes: number
  tracked_minutes: number
  completed_tasks: number
  i: number
  plan: number
  future: boolean
}
