import type { CalendarEvent } from '@/api/client'
import { addDays, hhmm, ymd } from './dates.ts'

// Wiederholung: wöchentlich an Wochentagen, optional mit Enddatum (RRULE-Teilmenge des Backends).
export const WEEKDAYS = [['MO', 'Mo'], ['TU', 'Di'], ['WE', 'Mi'], ['TH', 'Do'], ['FR', 'Fr'], ['SA', 'Sa'], ['SU', 'So']] as const
const CODE_OF_DAY = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'] // Index = Date.getDay()
const WEEKLY_RULE = /^FREQ=WEEKLY;BYDAY=([A-Z,]+)(?:;UNTIL=(\d{4})(\d{2})(\d{2}))?$/

// Ganztägig = von Mitternacht bis Mitternacht (ab 23 h wegen der Zeitumstellung); das Backend kennt kein eigenes Flag.
const atMidnight = (d: Date) => d.getHours() === 0 && d.getMinutes() === 0
export const isAllDay = (start: Date, end: Date) => atMidnight(start) && atMidnight(end) && end.getTime() - start.getTime() >= 23 * 3_600_000

// Letzter Tag eines Zeitraums mit exklusivem Ende.
const lastDay = (end: Date) => ymd(addDays(end, -1))

// Zustand des Dialogs zum Anlegen (Termin oder Task) und Bearbeiten von Terminen.
export interface Form {
  id: string | null // gesetzt beim Bearbeiten eines Termins
  kind: string
  title: string
  location: string
  date: string
  from: string
  to: string
  allDay: boolean // Task: ohne Uhrzeit; Termin: ganztägig
  endDate: string // letzter Tag eines ganztägigen Termins
  keepTimes: boolean // mehrtägiger Termin mit Uhrzeit: Zeiten bleiben unverändert (der Dialog kennt nur einen Tag)
  projectId: string // '' = kein Projekt
  repeat: boolean
  days: string[]
  until: string
  customRule: string | null // Regel, die der Dialog nicht abbildet; bleibt unverändert
  day: string // Tag der angeklickten Instanz, nur bei Serien gesetzt
  ask: boolean // Löschen wartet auf Bestätigung
}

export function newForm(start: Date, end: Date, allDay: boolean): Form {
  return {
    id: null, kind: 'event', title: '', location: '', date: ymd(start),
    from: allDay ? '09:00' : hhmm(start), to: allDay ? '10:00' : hhmm(end), allDay,
    endDate: allDay ? lastDay(end) : ymd(start), keepTimes: false, projectId: '',
    repeat: false, days: [CODE_OF_DAY[start.getDay()]!], until: '', customRule: null, day: '', ask: false,
  }
}

export function editForm(ev: CalendarEvent, day: string): Form {
  const [start, end] = [new Date(ev.start_at), new Date(ev.end_at)]
  const allDay = isAllDay(start, end)
  const f: Form = {
    ...newForm(start, end, allDay), id: ev.id, title: ev.title, location: ev.location,
    keepTimes: !allDay && ymd(end) !== ymd(start), projectId: ev.project_id ?? '', day: ev.recurrence_rule ? day : '',
  }
  const rule = ev.recurrence_rule
  const m = rule?.match(WEEKLY_RULE)
  if (m) Object.assign(f, { repeat: true, days: m[1]!.split(','), until: m[2] ? `${m[2]}-${m[3]}-${m[4]}` : '' })
  else if (rule) f.customRule = rule
  return f
}

// RRULE des Dialogs; '' = keine Wiederholung.
export function ruleOf(f: Form): string {
  if (!f.repeat) return ''
  const days = f.days.length ? f.days : [CODE_OF_DAY[new Date(`${f.date}T00:00:00`).getDay()]!]
  return `FREQ=WEEKLY;BYDAY=${days.join(',')}${f.until ? `;UNTIL=${f.until.replaceAll('-', '')}` : ''}`
}
