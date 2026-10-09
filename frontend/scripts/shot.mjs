// Screenshot einer Ansicht mit Mock-Daten, um UI-Änderungen anzusehen statt nur zu bauen.
//   pnpm dev               (in einem zweiten Terminal)
//   pnpm shot --view day --theme light --size 1280x800
//   pnpm shot --route tasks|habits|projects|review|trash|notes|settings
//   pnpm shot --view work --time 10:30       (Uhrzeit festsetzen: Jetzt-Linie und Scrollposition des Kalenders)
// Kalender-Ansichten: week (Mo–So), work (Mo–Fr), day, month. Ergebnis: frontend/.shots/<view|route>-<theme>.png
// Das Backend wird nur für nicht gemockte Pfade gebraucht (z. B. /api/health); die Kalenderdaten stammen aus diesem Skript.
import { mkdirSync } from 'node:fs'
import { parseArgs } from 'node:util'
import { chromium } from 'playwright'

const { values: o } = parseArgs({
  options: {
    view: { type: 'string', default: 'week' },
    route: { type: 'string', default: 'calendar' }, // calendar | tasks | habits | projects | review | trash | notes | settings
    theme: { type: 'string', default: 'dark' },
    lang: { type: 'string', default: 'de' }, // de | en
    size: { type: 'string', default: '1555x900' },
    aside: { type: 'string', default: 'open' }, // „Ungeplant“-Spalte: open | closed
    click: { type: 'string' }, // CSS-Selektor, der vor dem Screenshot angeklickt wird (z. B. '.task-row .row-title')
    time: { type: 'string' }, // HH:MM: setzt die Uhrzeit fest (z. B. 10:30), damit Bilder zu jeder Tageszeit gleich aussehen
    out: { type: 'string' },
  },
})
const VIEWS = { week: 'timeGridWeek', work: 'work', day: 'timeGridDay', month: 'dayGridMonth' }
if (!VIEWS[o.view]) throw new Error(`--view: ${Object.keys(VIEWS).join(' | ')}`)
const [width, height] = o.size.split('x').map(Number)
const base = process.env.KAIRO_URL ?? 'http://127.0.0.1:5173'
const out = o.out ?? `.shots/${o.route === 'calendar' ? o.view : o.route}-${o.theme}.png`

// --- Mock-Daten relativ zur aktuellen Woche
const now = new Date()
const mon = new Date(now.getFullYear(), now.getMonth(), now.getDate() - ((now.getDay() + 6) % 7))
const at = (d, hm) => { const [h, m] = hm.split(':').map(Number); return new Date(mon.getFullYear(), mon.getMonth(), mon.getDate() + d, h, m).toISOString() }
const day = (d) => { const x = new Date(mon.getFullYear(), mon.getMonth(), mon.getDate() + d); return `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, '0')}-${String(x.getDate()).padStart(2, '0')}` }
const P = { uni: 'p1', smart: 'p2', kairo: 'p3', privat: 'p4' }
const projects = [['p1', 'Uni', 'blue'], ['p2', 'SmartHome', 'orange'], ['p3', 'Kairo', 'green'], ['p4', 'Privat', 'yellow']]
  .map(([id, name, color]) => ({ id, name, color, status: 'ACTIVE', ...{ p1: { description: 'Vorlesungen, Übungen und Abgaben im dritten Semester.', local_path: '~/code/uni' }, p2: { description: 'Heizungs-Automation und Sensoren über MQTT.', local_path: '~/code/personal/smarthome' }, p3: { description: 'Persönlicher Planer: Kalender, Aufgaben, Gewohnheiten und Zeiterfassung.', local_path: '~/code/personal/kairo' } }[id] ?? { description: '', local_path: null } }))
let n = 0
const ev = (d, a, b, title, p, location = '', endDay = d) => ({
  id: `e${n++}`, title, location, occurrence_start: at(d, a), occurrence_end: at(endDay, b), start_at: at(d, a), end_at: at(endDay, b), recurrence_rule: null, project_id: P[p],
})
const events = []
for (let w = -1; w <= 4; w++) { // Stundenplan: je Woche Mo, Di, Mi, Do
  const o7 = w * 7
  events.push(ev(o7, '10:00', '11:30', 'Software Engineering', 'uni', 'H 1.02'), ev(o7 + 1, '08:15', '09:45', 'Datenbanken – Übung', 'uni', 'R 2.14'),
    ev(o7 + 2, '10:00', '11:30', 'Quality Goals & Models', 'uni', 'H 0.11'), ev(o7 + 3, '10:00', '11:30', 'Software Engineering', 'uni', 'H 1.02'))
}
events.push(ev(0, '12:15', '13:00', 'Mensa mit Jonas', 'privat'), ev(3, '11:00', '11:45', 'Sprechstunde Weber', 'uni'), // überschneidet sich mit Software Engineering
  ev(3, '18:00', '19:30', 'Sport', 'privat'), ev(4, '14:00', '16:00', 'Lerngruppe', 'uni', 'Bibliothek'), ev(5, '10:00', '11:30', 'Brunch', 'privat'),
  ev(10, '14:00', '16:00', 'Lerngruppe', 'uni'), ev(2, '00:00', '00:00', 'Abgabe Übungsblatt 3', 'uni', '', 3), ev(15, '00:00', '00:00', 'Kairo 0.4 Release', 'kairo', '', 16))
const todayIdx = (now.getDay() + 6) % 7
const soon = (() => { const d = new Date(now.getTime() + 53 * 60_000); return `${String(d.getHours()).padStart(2, '0')}:${String(Math.round(d.getMinutes() / 5) * 5 % 60).padStart(2, '0')}` })()
const task = (id, title, p, d, a, min, status = 'PLANNED') => ({
  id, title, description: '', status, priority: 'MEDIUM', estimated_minutes: min, due_at: null, planned_date: day(d), planned_start_at: a ? at(d, a) : null, project_id: P[p], parent_task_id: null,
})
const tasks = [
  task('t1', 'Übungsblatt 3', 'uni', 0, '14:00', 90, 'COMPLETED'), task('t2', 'MQTT-Broker einrichten', 'smart', 1, '16:00', 120, 'COMPLETED'),
  task('t3', 'Kalender-Redesign', 'kairo', 2, '13:00', 90, 'COMPLETED'), task('t4', 'Heizungs-Automation testen', 'smart', todayIdx, soon, 30), // heute, in rund 53 Minuten: füllt „Als Nächstes“
  task('t5', 'Kairo: Sync-Bug', 'kairo', 4, '09:00', 120), task('t6', 'Wochenrückblick', 'kairo', 4, '16:30', 30), task('t7', 'Einkaufen', 'privat', 5, '12:00', 60),
  task('t8', 'Fahrrad abholen', 'privat', 1, null, 0, 'COMPLETED'), task('t9', 'MQTT-Broker testen', 'smart', 7, '16:00', 60),
  ...[['t12', 'Vorlesung nacharbeiten', 'uni', 60], ['t13', 'README überarbeiten', 'kairo', 45], ['t14', 'Sensor-Batterien tauschen', 'smart', 15]]
    .map(([id, title, p, min]) => ({ ...task(id, title, p, 0, null, min), planned_date: null })), // ungeplant
  task('t10', 'Zusatz A', 'kairo', 3, null, 0), task('t11', 'Zusatz B', 'kairo', 3, null, 0),
  task('t15', 'Fahrrad abholen', 'privat', 0, null, 30), // Montag, noch offen: ab Dienstag überfällig
  task('t16', 'ST Labor 2: Aufgaben verteilen', 'uni', todayIdx, '17:00', 60, 'IN_PROGRESS'), // läuft (siehe times)
]
for (const [id, priority] of [['t5', 'HIGH'], ['t15', 'URGENT'], ['t7', 'LOW'], ['t4', 'HIGH']]) tasks.find((t) => t.id === id).priority = priority
for (const t of tasks) if (t.status === 'COMPLETED') t.completed_at = t.planned_start_at ?? at(0, '12:00') // die API liefert es für Rückblick und Woche
const times = [ // der letzte Eintrag läuft seit 23 Minuten
  { id: 'x1', task_id: 't3', project_id: 'p3', started_at: at(2, '13:05'), ended_at: at(2, '14:40') },
  { id: 'x2', task_id: 't1', project_id: 'p1', started_at: at(0, '14:05'), ended_at: at(0, '15:20') },
  { id: 'x3', task_id: 't2', project_id: 'p2', started_at: at(1, '16:10'), ended_at: at(1, '17:40') },
  { id: 'x4', task_id: 't16', project_id: 'p1', started_at: new Date(Date.now() - 12.6 * 60_000).toISOString(), ended_at: null },
]
// --- Mock-Daten für Gewohnheiten, Rückblick und Papierkorb
const iso = (d, hm = '12:00') => at(d, hm)
const habits = [['h1', 'lernen', 'DAILY', {}], ['h2', 'Lesen · 20 min', 'DAILY', {}], ['h3', 'sport', 'SPECIFIC_WEEKDAYS', { weekdays: ['MO', 'TH'] }], ['h4', 'Wochenrückblick', 'WEEKLY', { weekday: 'SU' }]]
  .map(([id, name, frequency_type, frequency_config]) => ({ id, name, description: '', frequency_type, frequency_config, active: true, start_date: day(-40) }))
const completions = (id) => Array.from({ length: 28 }, (_, i) => i).filter((i) => (Number(id.slice(1)) * 7 + i) % 5 !== 0).map((i) => ({ date: day(-i) }))
const review = (q) => ({
  from: q.get('from'), to: q.get('to'), timezone: 'Europe/Berlin', tracked_minutes: 440, completed_tasks: 4,
  days: [[225, 165], [210, 90], [210, 185], [225, 0], [270, 0], [150, 0], [0, 0]].map(([c, t], i) => ({ date: day(i), calendar_minutes: c, tracked_minutes: t, completed_tasks: i < 3 ? 1 : 0 })),
  projects: [['p3', 'Kairo', 185, 2, 4, 6], ['p1', 'Uni', 180, 1, 3, 5], ['p2', 'SmartHome', 75, 1, 2, 4], ['p4', 'Privat', 0, 0, 1, 3]]
    .map(([project_id, name, tracked_minutes, completed_tasks, done_tasks, total_tasks]) => ({ project_id, name, tracked_minutes, completed_tasks, done_tasks, total_tasks })),
  habits: habits.map((h, i) => ({ habit_id: h.id, name: h.name, done: 3 - (i % 2), streak: [14, 0, 6, 3][i] })),
})
const trash = () => ({
  tasks: [{ ...task('x1', 'ST labor 2 Aufgaben verteilen und machen 1h', 'uni', 0, null, 0), deleted_at: iso(0, '14:43') }, { ...task('x2', 'test', 'kairo', 0, null, 0), deleted_at: iso(0, '15:29') }],
  events: [{ id: 'x3', title: 'Mensa mit Jonas', start_at: at(0, '12:15'), recurrence_rule: null, deleted_at: iso(-5, '11:10') }],
  habits: [{ ...habits[3], id: 'x4', name: 'alte Gewohnheit', deleted_at: iso(-20, '20:02') }],
})
// --- Mock-Daten für Notizen (uni/ ist aufgeklappt, uni/Software Engineering.md geöffnet)
const noteTree = [
  { name: 'projekte', path: 'projekte', dir: true, children: [{ name: 'kairo.md', path: 'projekte/kairo.md', dir: false }] },
  { name: 'uni', path: 'uni', dir: true, children: [
    { name: 'Datenbanken.md', path: 'uni/Datenbanken.md', dir: false },
    { name: 'Software Engineering.md', path: 'uni/Software Engineering.md', dir: false },
  ] },
  { name: 'inbox.md', path: 'inbox.md', dir: false },
]
const noteText = `# Software Engineering

Vorlesung **Mo + Do 10:00**, H 1.02. Abgabe Übungsblatt 3 am Mittwoch.

## Offene Punkte

- [x] Kapitel 4 nacharbeiten
- [ ] UML-Klassendiagramm für die Übung
- [ ] Fragen für die Sprechstunde sammeln

> Anforderungen zuerst klären, dann modellieren.

| Woche | Thema |
| --- | --- |
| 5 | Entwurfsmuster |
| 6 | Testen |

Siehe auch [Skript](https://example.org) und \`git log --oneline\`.
`
const noteOf = /^\/api\/notes\/(.+)$/
const inWindow = (key, list, from, to) => list.filter((e) => e[key[0]] >= from && e[key[1] ?? key[0]] < to)
const mock = {
  '/api/projects': () => projects,
  '/api/tasks': () => tasks,
  '/api/calendar/occurrences': (q) => events.filter((e) => e.occurrence_end > q.get('from') && e.occurrence_start < q.get('to')),
  '/api/time-entries': (q) => (q.get('from') ? inWindow(['started_at'], times, q.get('from'), q.get('to')) : times),
  '/api/resources': () => [],
  '/api/habits': () => habits,
  '/api/review': review,
  '/api/trash': trash,
  '/api/notes': () => noteTree,
  '/api/settings': () => ({ settings: { notesDir: '/Users/demo/Kairo', workStart: '09:00', workEnd: '17:00' }, env: { timezone: 'KAIRO_TIMEZONE' }, path: '/Users/demo/.config/kairo/config.json', restart_needed: true }),
  '/api/today': (q) => {
    const date = q.get('date') ?? day(todayIdx)
    const open = (t) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'
    const todays = tasks.filter((t) => t.planned_date === date && open(t)).sort((a, b) => (a.planned_start_at ?? '9').localeCompare(b.planned_start_at ?? '9'))
    return {
      date, timezone: 'Europe/Berlin', unestimated_tasks: 0, overplanned_minutes: 0, planned_minutes: 270, calendar_minutes: 90, tracked_minutes: 197, work_minutes: 0, free_minutes: 203,
      events: events.filter((e) => e.occurrence_start.slice(0, 10) <= date && e.occurrence_end > at(todayIdx, '00:00')).filter((e) => e.occurrence_start >= at(todayIdx, '00:00') && e.occurrence_start < at(todayIdx + 1, '00:00')).map(({ id, title, location, occurrence_start, occurrence_end }) => ({ id, title, location, occurrence_start, occurrence_end })),
      tasks: todays, overdue: tasks.filter((t) => t.planned_date && t.planned_date < date && open(t)), active_tasks: [],
      running_time_entry: times.find((e) => !e.ended_at) ?? null,
      habits: habits.filter((h) => h.id !== 'h3' && h.id !== 'h4').map((h, i) => ({ id: h.id, name: h.name, unit: '', target_value: null, preferred_time: null, done: i === 0, week_progress: null })),
    }
  },
}
const completionsOf = /^\/api\/habits\/(h\d)\/completions$/

// --- Browser
let browser
try {
  browser = await chromium.launch({ args: ['--lang=de-DE'] })
} catch (e) {
  throw new Error(`Chromium fehlt: pnpm exec playwright install chromium (${e.message.split('\n')[0]})`)
}
const page = await (await browser.newContext({ viewport: { width, height }, colorScheme: o.theme, locale: 'de-DE' })).newPage()
if (o.time) await page.clock.setFixedTime(new Date(`${day(todayIdx)}T${o.time}:00`))
await page.addInitScript(([view, theme, aside, lang]) => {
  localStorage.setItem('kairo-cal-view', view)
  localStorage.setItem('kairo-cal-weekend', '1')
  localStorage.setItem('kairo-cal-unplanned', aside === 'open' ? '1' : '0')
  localStorage.setItem('kairo-theme', theme)
  localStorage.setItem('kairo-lang', lang)
  localStorage.setItem('kairo-notes-open', '["uni"]')
  localStorage.setItem('kairo-notes-file', 'uni/Software Engineering.md')
}, [VIEWS[o.view], o.theme, o.aside, o.lang])
await page.route('**/api/**', (route) => {
  const u = new URL(route.request().url())
  const c = completionsOf.exec(u.pathname)
  const n = noteOf.exec(u.pathname)
  const f = c ? () => completions(c[1]) : n ? () => ({ path: decodeURIComponent(n[1]), content: noteText, mtime: '1' }) : mock[u.pathname]
  return f && route.request().method() === 'GET' ? route.fulfill({ json: f(u.searchParams) }) : route.continue()
})
try {
  await page.goto(`${base}/${o.route}`)
} catch {
  await browser.close()
  throw new Error(`${base} nicht erreichbar: erst „pnpm dev“ starten (oder KAIRO_URL setzen)`)
}
await page.waitForSelector({ calendar: '.fc-view', notes: '.notes' }[o.route] ?? '.view-inner')
await page.waitForLoadState('networkidle')
if (o.click) await page.locator(o.click).first().click()
await page.waitForTimeout(400)
mkdirSync(out.includes('/') ? out.slice(0, out.lastIndexOf('/')) : '.', { recursive: true })
await page.screenshot({ path: out })
await browser.close()
console.log(out)
