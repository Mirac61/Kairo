// Screenshot einer Ansicht mit Mock-Daten, um UI-Änderungen anzusehen statt nur zu bauen.
//   pnpm dev               (in einem zweiten Terminal)
//   pnpm shot --view day --theme light --size 1280x800
//   pnpm shot --route tasks|habits|projects|goals|journal|cards|review|trash|notes|settings
//   pnpm shot --view work --time 10:30       (Uhrzeit festsetzen: Jetzt-Linie und Scrollposition des Kalenders)
// Kalender-Ansichten: week (7 Tage), work (5 Tage), day, month; heute jeweils in der Mitte. Ergebnis: frontend/.shots/<view|route>-<theme>.png
// Das Backend wird nur für nicht gemockte Pfade gebraucht (z. B. /api/health); die Kalenderdaten stammen aus diesem Skript.
import { mkdirSync } from 'node:fs'
import { parseArgs } from 'node:util'
import { chromium } from 'playwright'

const { values: o } = parseArgs({
  options: {
    view: { type: 'string', default: 'week' },
    route: { type: 'string', default: 'calendar' }, // calendar | tasks | habits | projects | goals | journal | cards | review | trash | notes | settings
    theme: { type: 'string', default: 'dark' },
    lang: { type: 'string', default: 'de' }, // de | en
    size: { type: 'string', default: '1555x900' },
    aside: { type: 'string', default: 'open' }, // „Ungeplant“-Spalte: open | closed
    click: { type: 'string' }, // CSS-Selektor, der vor dem Screenshot angeklickt wird (z. B. '.task-row .row-title')
    time: { type: 'string' }, // HH:MM: setzt die Uhrzeit fest (z. B. 10:30), damit Bilder zu jeder Tageszeit gleich aussehen
    out: { type: 'string' },
    store: { type: 'string' }, // weitere localStorage-Werte, z. B. kairo-palette=plum,kairo-accent=violet,kairo-week-start=0
  },
})
const VIEWS = { week: 'timeGrid7', work: 'timeGrid5', day: 'timeGridDay', month: 'dayGridMonth' }
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
const projects = [['p1', 'Beispiel: Studium', 'blue'], ['p2', 'Beispiel: Haus', 'orange'], ['p3', 'Beispiel: App', 'green'], ['p4', 'Beispiel: Privat', 'yellow']]
  .map(([id, name, color]) => ({ id, name, color, status: 'ACTIVE', ...{ p1: { description: 'Vorlesungen, Übungen und Abgaben im dritten Semester.', local_path: '~/code/beispiel-studium' }, p2: { description: 'Erfundenes Beispiel: Sensoren über MQTT.', local_path: '~/code/beispiel-haus' }, p3: { description: 'Persönlicher Planer: Kalender, Aufgaben, Gewohnheiten und Zeiterfassung.', local_path: '~/code/beispiel-app' } }[id] ?? { description: '', local_path: null } }))
let n = 0
const ev = (d, a, b, title, p, location = '', endDay = d) => ({
  id: `e${n++}`, title, location, occurrence_start: at(d, a), occurrence_end: at(endDay, b), start_at: at(d, a), end_at: at(endDay, b), recurrence_rule: null, project_id: P[p],
})
const events = []
for (let w = -1; w <= 4; w++) { // Stundenplan: je Woche Mo, Di, Mi, Do
  const o7 = w * 7
  events.push(ev(o7, '10:00', '11:30', 'Beispiel-Vorlesung', 'uni', 'Raum 101'), ev(o7 + 1, '08:15', '09:45', 'Beispiel-Übung', 'uni', 'Raum 102'),
    ev(o7 + 2, '10:00', '11:30', 'Beispiel-Seminar', 'uni', 'Raum 103'), ev(o7 + 3, '10:00', '11:30', 'Beispiel-Vorlesung', 'uni', 'Raum 101'))
}
events.push(ev(0, '12:15', '13:00', 'Mittagessen', 'privat'), ev(3, '11:00', '11:45', 'Sprechstunde', 'uni'), // überschneidet sich mit Beispiel-Vorlesung
  ev(3, '18:00', '19:30', 'Sport', 'privat'), ev(4, '14:00', '16:00', 'Lerngruppe', 'uni', 'Bibliothek'), ev(5, '10:00', '11:30', 'Brunch', 'privat'),
  ev(10, '14:00', '16:00', 'Lerngruppe', 'uni'), ev(2, '00:00', '00:00', 'Abgabe Übungsblatt 3', 'uni', '', 3), ev(15, '00:00', '00:00', 'Kairo 0.4 Release', 'kairo', '', 16))
const todayIdx = (now.getDay() + 6) % 7
const soon = (() => { const d = new Date(now.getTime() + 53 * 60_000); return `${String(d.getHours()).padStart(2, '0')}:${String(Math.round(d.getMinutes() / 5) * 5 % 60).padStart(2, '0')}` })()
const task = (id, title, p, d, a, min, status = 'PLANNED') => ({
  id, title, description: '', status, priority: 'MEDIUM', estimated_minutes: min, due_at: null, planned_date: day(d), planned_start_at: a ? at(d, a) : null, project_id: P[p], parent_task_id: null,
})
const tasks = [
  task('t1', 'Übungsblatt 3', 'uni', 0, '14:00', 90, 'COMPLETED'), task('t2', 'MQTT-Broker einrichten', 'smart', 1, '16:00', 120, 'COMPLETED'),
  task('t3', 'Kalender-Redesign', 'kairo', 2, '13:00', 90, 'COMPLETED'), task('t4', 'Beispielaufgabe: Heizung testen', 'smart', todayIdx, soon, 30), // heute, in rund 53 Minuten: füllt „Als Nächstes“
  task('t5', 'Kairo: Sync-Bug', 'kairo', 4, '09:00', 120), task('t6', 'Wochenrückblick', 'kairo', 4, '16:30', 30), task('t7', 'Einkaufen', 'privat', 5, '12:00', 60),
  task('t8', 'Fahrrad abholen', 'privat', 1, null, 0, 'COMPLETED'), task('t9', 'MQTT-Broker testen', 'smart', 7, '16:00', 60),
  ...[['t12', 'Vorlesung nacharbeiten', 'uni', 60], ['t13', 'README überarbeiten', 'kairo', 45], ['t14', 'Sensor-Batterien tauschen', 'smart', 15]]
    .map(([id, title, p, min]) => ({ ...task(id, title, p, 0, null, min), planned_date: null })), // ungeplant
  task('t10', 'Zusatz A', 'kairo', 3, null, 0), task('t11', 'Zusatz B', 'kairo', 3, null, 0),
  task('t15', 'Fahrrad abholen', 'privat', 0, null, 30), // Montag, noch offen: ab Dienstag überfällig
  task('t16', 'Laborübung (Beispiel)', 'uni', todayIdx, '17:00', 60, 'IN_PROGRESS'), // läuft (siehe times)
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
  .map(([id, name, frequency_type, frequency_config]) => ({ id, name, description: '', frequency_type, frequency_config, active: true, start_date: day(-40), end_date: null, target_value: null, unit: '', duration_minutes: 30, preferred_time: null }))
Object.assign(habits[0], { preferred_time: '19:00', duration_minutes: 60, start_date: day(-400) })
Object.assign(habits[1], { start_date: day(-200) })
Object.assign(habits[2], { preferred_time: '17:00', duration_minutes: 45, target_value: 5, unit: 'km' })
const WD = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
// Fällige Gewohnheiten je Tag für den Kalender (vereinfachte Regel ohne X-mal pro Woche).
const habitsDue = (q) => {
  const out = []
  for (let d = new Date(`${q.get('from')}T12:00:00`); d <= new Date(`${q.get('to')}T12:00:00`); d.setDate(d.getDate() + 1)) {
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    for (const h of habits) {
      const c = h.frequency_config
      if (h.frequency_type === 'SPECIFIC_WEEKDAYS' ? c.weekdays.includes(WD[d.getDay()]) : h.frequency_type === 'WEEKLY' ? c.weekday === WD[d.getDay()] : true) {
        out.push({ habit_id: h.id, name: h.name, date, done: date < day(todayIdx) && date.charCodeAt(9) % 3 !== 0, preferred_time: h.preferred_time, duration_minutes: h.duration_minutes })
      }
    }
  }
  return out
}
const completions = (id) => Array.from({ length: 370 }, (_, i) => i).filter((i) => (Number(id.slice(1)) * 7 + i) % 5 !== 0).map((i) => ({ date: day(-i) }))
const review = (q) => ({
  from: q.get('from'), to: q.get('to'), timezone: 'Europe/Berlin', tracked_minutes: 440, completed_tasks: 4,
  days: [[225, 165], [210, 90], [210, 185], [225, 0], [270, 0], [150, 0], [0, 0]].map(([c, t], i) => ({ date: day(i), calendar_minutes: c, tracked_minutes: t, completed_tasks: i < 3 ? 1 : 0 })),
  projects: [['p3', 'Beispiel: App', 185, 2, 4, 6], ['p1', 'Beispiel: Studium', 180, 1, 3, 5], ['p2', 'Beispiel: Haus', 75, 1, 2, 4], ['p4', 'Beispiel: Privat', 0, 0, 1, 3]]
    .map(([project_id, name, tracked_minutes, completed_tasks, done_tasks, total_tasks]) => ({ project_id, name, tracked_minutes, completed_tasks, done_tasks, total_tasks })),
  habits: habits.map((h, i) => ({ habit_id: h.id, name: h.name, done: 3 - (i % 2), streak: [14, 0, 6, 3][i] })),
})
const trash = () => ({
  tasks: [{ ...task('x1', 'Laborübung vorbereiten 1h', 'uni', 0, null, 0), deleted_at: iso(0, '14:43') }, { ...task('x2', 'test', 'kairo', 0, null, 0), deleted_at: iso(0, '15:29') }],
  events: [{ id: 'x3', title: 'Mittagessen', start_at: at(0, '12:15'), recurrence_rule: null, deleted_at: iso(-5, '11:10') }],
  habits: [{ ...habits[3], id: 'x4', name: 'alte Gewohnheit', deleted_at: iso(-20, '20:02') }],
})
// --- Mock-Daten für Notizen (uni/ ist aufgeklappt, uni/Beispiel-Vorlesung.md geöffnet)
const noteTree = [
  { name: 'Journal', path: 'Journal', dir: true, children: [-1, -2, -4].map((d) => ({ name: `${day(todayIdx + d)}.md`, path: `Journal/${day(todayIdx + d)}.md`, dir: false })) },
  { name: 'projekte', path: 'projekte', dir: true, children: [{ name: 'kairo.md', path: 'projekte/kairo.md', dir: false }] },
  { name: 'uni', path: 'uni', dir: true, children: [
    { name: 'Übung.md', path: 'uni/Übung.md', dir: false },
    { name: 'Beispiel-Vorlesung.md', path: 'uni/Beispiel-Vorlesung.md', dir: false },
  ] },
  { name: 'inbox.md', path: 'inbox.md', dir: false },
]
const noteText = `# Beispiel-Vorlesung

Vorlesung **Mo + Do 10:00**, Raum 101. Abgabe Übungsblatt 3 am Mittwoch.

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
const journalText = '# Journal\n\n## Was lief gut?\n\nDen Kalender fürs Handy fertig gemacht, abends noch gelaufen.\n\n## Was lerne ich daraus?\n\nKleine Schritte mit Screenshot prüfen spart Zeit.\n\n## Was ist morgen wichtig?\n\nÜbungsblatt 3 abgeben.\n'
const card = (deck, question, answer, due, extra = {}) => ({ key: question, id: extra.note ? null : question, deck, question, answer, note_path: extra.note ?? null, new: !!extra.new, due, due_date: extra.new ? null : day(todayIdx + (due ? 0 : 3)), interval_days: 3 })
const cards = [
  card('Computerarchitektur', 'Was ist ein CMOS?', 'Ein PMOS- und ein NMOS-Transistor im Paar; im Ruhezustand fließt kaum Strom.', true, { note: 'Computerarchitektur/2026-10-09_Fundamente.md' }),
  card('Computerarchitektur', 'Was macht ein Multiplexer?', 'Wählt einen von mehreren Eingängen über Steuerleitungen aus.', true, { new: true }),
  card('Computerarchitektur', 'Big-Endian', 'Höchstwertiges Byte an der niedrigsten Adresse.', false),
  card('Mathe 2', 'Ableitung von sin(x)', 'cos(x)', true),
  card('Mathe 2', 'Eigenwert', 'λ mit Av = λv für ein v ≠ 0', false),
  card('Japanisch', '勉強', 'べんきょう – lernen', true, { new: true }),
  card('Japanisch', '仕事', 'しごと – Arbeit', true),
  card('Japanisch', '会社', 'かいしゃ – Firma', false),
]
const decks = ['Computerarchitektur', 'Japanisch', 'Mathe 2'].map((name, i) => {
  const cs = cards.filter((c) => c.deck === name)
  return { name, color: ['violet', 'pink', 'aqua'][i], total: cs.length, due: cs.filter((c) => c.due).length, new_due: cs.filter((c) => c.new).length }
})
const goal = (id, title, why, extra = {}) => ({ id, title, why, image_path: null, kind: 'NONE', project_id: null, habit_id: null, target: 0, start_date: day(0), due_date: null, archived: false, parent_id: null, progress: null, ...extra })
const goals = [
  goal('g1', 'Ein Werkstudentenplatz bekommen', 'Ich will selber Geld verdienen und Arbeitserfahrung sammeln.', { image_path: 'Ziele/g1.svg', due_date: day(90), progress: { current: 1, target: 3, percent: 55 } }),
  goal('g1a', 'Lebenslauf fertig', '', { parent_id: 'g1', archived: true }),
  goal('g1b', '20 Bewerbungen schicken', 'Jede Absage ist eine Übung.', { parent_id: 'g1', kind: 'PROJECT_TASKS', project_id: 'p3', progress: { current: 7, target: 20, percent: 35 } }),
  goal('g1c', 'Portfolio mit Kairo', '', { parent_id: 'g1', image_path: 'Ziele/g1c.svg', kind: 'PROJECT_HOURS', project_id: 'p3', target: 40, progress: { current: 22, target: 40, percent: 55 } }),
  goal('g1d', 'Interview üben', 'Ruhig bleiben, Beispiele parat haben.', { parent_id: 'g1', due_date: day(30) }),
  goal('g2', 'Japanisch auf N4', 'Einmal in Tokio ohne Übersetzer durch den Tag.', { progress: { current: 0, target: 2, percent: 20 } }),
  goal('g2a', '300 Kanji', '', { parent_id: 'g2', kind: 'HABIT_COUNT', habit_id: 'h1', target: 300, progress: { current: 120, target: 300, percent: 40 } }),
  goal('g2b', 'Probeprüfung bestehen', '', { parent_id: 'g2' }),
  goal('g3', 'Halbmarathon', 'Geschafft.', { archived: true }),
]
// Platzhalterbild für /api/files: Farbverlauf je nach Name.
const fileOf = /^\/api\/files\/(.+)$/
const svgImage = (name) => `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><defs><linearGradient id="a" x2="1" y2="1"><stop stop-color="${name.includes('g1') ? '#7c5cff' : '#2bb673'}"/><stop offset="1" stop-color="#1b1d24"/></linearGradient></defs><rect width="640" height="360" fill="url(#a)"/></svg>`
const inWindow = (key, list, from, to) => list.filter((e) => e[key[0]] >= from && e[key[1] ?? key[0]] < to)
const mock = {
  '/api/projects': () => projects,
  '/api/tasks': () => tasks,
  '/api/calendar/occurrences': (q) => events.filter((e) => e.occurrence_end > q.get('from') && e.occurrence_start < q.get('to')),
  '/api/projects/stats': () => projects.map(({ id }) => {
    const es = times.filter((e) => e.project_id === id)
    const end = (e) => (e.ended_at ? Date.parse(e.ended_at) : Date.now())
    const sum = (xs) => xs.reduce((a, e) => a + Math.floor((end(e) - Date.parse(e.started_at)) / 60_000), 0)
    return { project_id: id, week_minutes: sum(es.filter((e) => e.started_at >= mon.toISOString())), total_minutes: sum(es), last_active_at: new Date(Math.max(0, ...es.map(end))).toISOString() }
  }).filter((s) => s.total_minutes),
  '/api/time-entries': (q) => (q.get('from') ? inWindow(['started_at'], times, q.get('from'), q.get('to')) : times),
  '/api/resources': () => [],
  '/api/journal/summary': () => ({ date: day(todayIdx), total: 3, streak: 2 }),
  '/api/journal/task-imports': () => [],
  '/api/habits': () => habits,
  '/api/habits/due': habitsDue,
  '/api/github/issues': () => [
    { title: 'Kalender: Habits per Drag verschieben', url: 'https://github.com/demo/kairo/issues/41', repo: 'demo/kairo', number: 41, labels: ['enhancement'], updated_at: iso(0), task_id: null },
    { title: 'Absturz beim ICS-Import ohne DTEND', url: 'https://github.com/demo/kairo/issues/38', repo: 'demo/kairo', number: 38, labels: ['bug'], updated_at: iso(-1), task_id: 't4' },
    { title: 'MQTT-Reconnect nach Standby', url: 'https://github.com/demo/haus/issues/7', repo: 'demo/haus', number: 7, labels: [], updated_at: iso(-3), task_id: null },
  ],
  '/api/review': review,
  '/api/goals': () => goals,
  '/api/cards': () => cards,
  '/api/cards/due': () => cards.filter((c) => c.due),
  '/api/cards/decks': () => decks,
  '/api/goals/today': () => goals[0],
  '/api/trash': trash,
  '/api/notes': () => noteTree,
  '/api/prefs': () => ({}), // leer: es gelten die --store-Werte
  '/api/settings': () => ({ settings: { notes_dir: '/Users/demo/Kairo', work_start: '09:00', work_end: '17:00' }, env: { timezone: 'KAIRO_TIMEZONE' }, path: '/Users/demo/.config/kairo/config.json', restart_needed: true }),
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
const projectIssuesOf = /^\/api\/projects\/(p\d)\/github\/issues$/

// --- Browser
let browser
try {
  browser = await chromium.launch({ args: ['--lang=de-DE'] })
} catch (e) {
  throw new Error(`Chromium fehlt: pnpm exec playwright install chromium (${e.message.split('\n')[0]})`)
}
const page = await (await browser.newContext({ viewport: { width, height }, colorScheme: o.theme, locale: 'de-DE' })).newPage()
if (o.time) await page.clock.setFixedTime(new Date(`${day(todayIdx)}T${o.time}:00`))
await page.addInitScript(([view, theme, aside, lang, extra]) => {
  localStorage.setItem('kairo-cal-view', view)
  localStorage.setItem('kairo-cal-days', view === 'timeGrid5' ? '5' : '7')
  localStorage.setItem('kairo-cal-unplanned', aside === 'open' ? '1' : '0')
  localStorage.setItem('kairo-theme', theme)
  localStorage.setItem('kairo-lang', lang)
  localStorage.setItem('kairo-notes-open', '["uni"]')
  localStorage.setItem('kairo-notes-file', 'uni/Beispiel-Vorlesung.md')
  for (const kv of extra ? extra.split(',') : []) localStorage.setItem(...kv.split('='))
}, [VIEWS[o.view], o.theme, o.aside, o.lang, o.store])
await page.route('**/api/**', (route) => {
  const u = new URL(route.request().url())
  const c = completionsOf.exec(u.pathname)
  const n = noteOf.exec(u.pathname)
  const gi = projectIssuesOf.exec(u.pathname)
  const file = fileOf.exec(u.pathname)
  if (file) return route.fulfill({ contentType: 'image/svg+xml', body: svgImage(file[1]) })
  const f = gi ? () => (gi[1] === 'p3' ? { repo: 'demo/kairo', issues: mock['/api/github/issues']().filter((i) => i.repo === 'demo/kairo') } : { repo: '', issues: [] }) : c ? () => completions(c[1]) : n ? () => ({ path: decodeURIComponent(n[1]), content: n[1].startsWith('Journal/') ? journalText : noteText, mtime: '1' }) : mock[u.pathname]
  return f && route.request().method() === 'GET' ? route.fulfill({ json: f(u.searchParams) }) : route.continue()
})
try {
  await page.goto(`${base}/${o.route}`)
} catch {
  await browser.close()
  throw new Error(`${base} nicht erreichbar: erst „pnpm dev“ starten (oder KAIRO_URL setzen)`)
}
await page.waitForSelector({ calendar: '.fc-view', notes: '.notes', settings: '.st', login: '.auth-page', setup: '.auth-page' }[o.route] ?? '.view-inner')
await page.waitForLoadState('networkidle')
// Mehrere Klicks nacheinander mit „ && “ getrennt, z. B. '.cd-tile-learn && .cd-show'.
if (o.click) for (const sel of o.click.split(' && ')) await page.locator(sel).first().click()
await page.waitForTimeout(400)
mkdirSync(out.includes('/') ? out.slice(0, out.lastIndexOf('/')) : '.', { recursive: true })
// Querformat-Überlauf (Handy): Seitenbreite und Elemente, die rechts über den Rand ragen.
console.log(await page.evaluate(() => {
  const w = innerWidth
  const bad = [...document.querySelectorAll('.content *')].filter((e) => e.getBoundingClientRect().right > w + 1 && e.getBoundingClientRect().width > 0)
  return `scrollWidth=${document.documentElement.scrollWidth} innerWidth=${w} über den Rand: ${bad.length} ${bad.slice(0, 6).map((e) => e.tagName.toLowerCase() + '.' + [...e.classList].join('.')).join(' | ')}`
}))
// Kennzeichnung auf jedem Bild: alle Daten oben sind erfunden (docs/screenshots).
await page.evaluate((label) => {
  const b = Object.assign(document.createElement('div'), { textContent: label })
  b.style.cssText = 'font:600 11px/1 system-ui;letter-spacing:.06em;text-transform:uppercase;color:var(--text-2,#888);border:1px dashed currentColor;border-radius:6px;padding:5px 8px;margin:0 0 12px;text-align:center'
  const side = document.querySelector('.sidebar:not(.mini)')
  if (side) side.prepend(b)
  else document.body.append(Object.assign(b, { style: b.style.cssText + ';position:fixed;top:6px;right:6px;z-index:99;margin:0;background:var(--bg,#fff)' }))
}, o.lang === 'en' ? 'Demo data' : 'Beispieldaten')
await page.screenshot({ path: out })
await browser.close()
console.log(out)
