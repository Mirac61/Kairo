import type { EventApi } from '@fullcalendar/core'
import { hhmm, hm } from '@/lib/dates'
import { t } from '@/lib/i18n'

export function createDomElement(tag: string, className: string, textContent = ''): HTMLElement {
  const element = document.createElement(tag)
  element.className = className
  element.textContent = textContent
  return element
}

// ISO-Kalenderwoche (Donnerstag der Woche entscheidet über das Jahr).
export function isoWeek(date: Date): number {
  const target = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()))
  target.setUTCDate(target.getUTCDate() + 4 - (target.getUTCDay() || 7))
  const yearStart = Date.UTC(target.getUTCFullYear(), 0, 1)
  return Math.ceil(((target.getTime() - yearStart) / 86_400_000 + 1) / 7)
}

export function paintDay(th: HTMLElement, stats: Map<string, [number, number]>): void {
  const capEl = th.querySelector<HTMLElement>('.kt-cap')
  if (!capEl) return // Monatskopf hat keine Statistik

  const [p, ist] = stats.get(th.dataset.date ?? '') ?? [0, 0]
  capEl.textContent = `${hm(ist)} / ${hm(p)}h`

  const tBar = th.querySelector<HTMLElement>('.kt-bar .t')
  if (tBar) {
    const pct =
      p > 0
        ? Math.min(100, Math.round((ist / p) * 100))
        : ist > 0
          ? Math.min(100, Math.round((ist / 480) * 100))
          : 0
    tBar.style.width = `${pct}%`
  }

  th.title = `${t('Ist')}: ${hm(ist)} · ${t('Plan')}: ${hm(p)}`
}

export function renderDayHeader(
  a: { date: Date; isToday: boolean; view: { type: string } },
  localeVal: string,
): string | { domNodes: HTMLElement[] } {
  const month = a.view.type === 'dayGridMonth'
  const wd = a.date
    .toLocaleDateString(localeVal, { weekday: month ? 'short' : 'long' })
    .replace('.', '')
  if (month) return wd
  if (a.view.type === 'timeGridDay') {
    const head = createDomElement('div', 'kt-dayhead')
    head.append(
      createDomElement('span', '', t('Plan')),
      createDomElement('span', 'kt-trk-h', t('Erfasst')),
    )
    return { domNodes: [head] }
  }
  const row = createDomElement('div', 'kt-row')
  row.append(createDomElement('span', 'kt-cap'))
  const bar = createDomElement('div', 'kt-bar')
  bar.append(createDomElement('i', 't'))
  const root = createDomElement('div', `kt-dh${a.isToday ? ' today' : ''}`)
  root.append(
    createDomElement('small', 'kt-wd-l', wd),
    createDomElement(
      'small',
      'kt-wd-s',
      a.date.toLocaleDateString(localeVal, { weekday: 'short' }).replace('.', ''),
    ),
    createDomElement('b', '', String(a.date.getDate())),
    row,
    bar,
  )
  return { domNodes: [root] }
}

export function renderEventContent(a: { event: EventApi; view: { type: string } }): {
  domNodes: HTMLElement[]
} {
  const { event } = a
  const p = event.extendedProps as { kind: string; proj?: string; place?: string }
  if (p.kind === 'track') {
    if (a.view.type !== 'timeGridDay') return { domNodes: [] }
    const x = event.extendedProps as { title: string; label: string }
    const card = createDomElement('div', 'kt-trk')
    card.append(createDomElement('b', '', x.title), createDomElement('span', '', x.label))
    return { domNodes: [card] }
  }
  const month = a.view.type === 'dayGridMonth'
  const { start, end } = event
  const timed = !event.allDay && start
  const time = createDomElement('span', 'kt-time', timed ? hhmm(start) : '')
  if (timed && end && !month) {
    time.append(createDomElement('span', 'kt-end', `–${hhmm(end)}`))
  }
  const titleText = createDomElement('span', 'kt-title', event.title)
  const meta = [p.proj, p.place].filter(Boolean).join(' · ')
  const root = createDomElement('div', month ? 'kt-in kt-month' : 'kt-in')
  if (month) {
    root.append(...(timed ? [time] : []), titleText)
  } else {
    const head = createDomElement('div', 'kt-head')
    head.append(titleText)
    root.append(
      head,
      ...(timed ? [time] : []),
      ...(meta ? [createDomElement('span', 'kt-meta', meta)] : []),
    )
  }
  return { domNodes: [root] }
}

export function mountEvent(a: { event: EventApi; el: HTMLElement; timeText: string }): void {
  const color = a.event.extendedProps.color as string | undefined
  if (color) {
    a.el.style.setProperty('--acc', color)
    const m = color.match(/--p-([a-z]+)/) || color.match(/^([a-z]+)$/)
    if (m?.[1]) {
      a.el.setAttribute('data-color', m[1])
    }
  } else {
    a.el.setAttribute('data-color', 'blue')
  }
  const x = a.event.extendedProps as { kind: string; title?: string; label?: string }
  a.el.title =
    x.kind === 'track'
      ? `${t('Erfasst')}: ${x.title} · ${x.label}`
      : `${a.timeText ? `${a.timeText} ` : ''}${a.event.title}`
  if (x.kind === 'habit') {
    a.el.title += ` · ${t('Klick hakt ab, Verschieben ändert die Uhrzeit an allen Tagen')}`
  }
}

export function scrollStart(slotMinHour: number, evs: EventApi[] = []): string {
  const h = new Date().getHours() - 1
  let m = h <= slotMinHour ? slotMinHour * 60 : (h - 1) * 60 + 30
  for (const e of evs) {
    if (e.allDay || !e.start || !e.end || e.extendedProps.kind === 'track') continue
    const s = e.start.getHours() * 60 + e.start.getMinutes()
    if (s < m && m < e.end.getHours() * 60 + e.end.getMinutes()) {
      m = Math.floor(s / 60) * 60 - 30
    }
  }
  m = Math.max(m, slotMinHour * 60)
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}:00`
}
