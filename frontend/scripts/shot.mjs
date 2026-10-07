// Screenshot des Kalenders mit Mock-Daten, um UI-Änderungen anzusehen statt nur zu bauen.
//   npm run dev            (in einem zweiten Terminal)
//   npm run shot -- --view day --theme light --size 1280x800
// Ansichten: week (Mo–So), work (Mo–Fr), day, month. Ergebnis: frontend/.shots/<view>-<theme>.png
// Das Backend wird nur für nicht gemockte Pfade gebraucht (z. B. /api/health); die Kalenderdaten stammen aus diesem Skript.
import { mkdirSync } from 'node:fs'
import { parseArgs } from 'node:util'
import { chromium } from 'playwright'

const { values: o } = parseArgs({
  options: {
    view: { type: 'string', default: 'week' },
    theme: { type: 'string', default: 'dark' },
    size: { type: 'string', default: '1555x900' },
    aside: { type: 'string', default: 'open' }, // „Ungeplant“-Spalte: open | closed
    out: { type: 'string' },
  },
})
const VIEWS = { week: 'timeGridWeek', work: 'work', day: 'timeGridDay', month: 'dayGridMonth' }
if (!VIEWS[o.view]) throw new Error(`--view: ${Object.keys(VIEWS).join(' | ')}`)
const [width, height] = o.size.split('x').map(Number)
const base = process.env.KAIRO_URL ?? 'http://127.0.0.1:5173'
const out = o.out ?? `.shots/${o.view}-${o.theme}.png`

// --- Mock-Daten relativ zur aktuellen Woche
const now = new Date()
const mon = new Date(now.getFullYear(), now.getMonth(), now.getDate() - ((now.getDay() + 6) % 7))
const at = (d, hm) => { const [h, m] = hm.split(':').map(Number); return new Date(mon.getFullYear(), mon.getMonth(), mon.getDate() + d, h, m).toISOString() }
const day = (d) => { const x = new Date(mon.getFullYear(), mon.getMonth(), mon.getDate() + d); return `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, '0')}-${String(x.getDate()).padStart(2, '0')}` }
const P = { uni: 'p1', smart: 'p2', kairo: 'p3', privat: 'p4' }
const projects = [['p1', 'Uni', 'blue'], ['p2', 'SmartHome', 'orange'], ['p3', 'Kairo', 'green'], ['p4', 'Privat', 'yellow']]
  .map(([id, name, color]) => ({ id, name, color, status: 'ACTIVE', description: '', local_path: null }))
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
const soon = (() => { const d = new Date(now.getTime() + 53 * 60_000); return `${String(d.getHours()).padStart(2, '0')}:${String(Math.round(d.getMinutes() / 5) * 5 % 60).padStart(2, '0')}` })()
const task = (id, title, p, d, a, min, status = 'PLANNED') => ({
  id, title, description: '', status, priority: 'P3', estimated_minutes: min, due_at: null, planned_date: day(d), planned_start_at: a ? at(d, a) : null, project_id: P[p], parent_task_id: null,
})
const tasks = [
  task('t1', 'Übungsblatt 3', 'uni', 0, '14:00', 90, 'COMPLETED'), task('t2', 'MQTT-Broker einrichten', 'smart', 1, '16:00', 120, 'COMPLETED'),
  task('t3', 'Kalender-Redesign', 'kairo', 2, '13:00', 90, 'COMPLETED'), task('t4', 'Heizungs-Automation testen', 'smart', (now.getDay() + 6) % 7, soon, 30), // heute, in rund 53 Minuten: füllt „Als Nächstes“
  task('t5', 'Kairo: Sync-Bug', 'kairo', 4, '09:00', 120), task('t6', 'Wochenrückblick', 'kairo', 4, '16:30', 30), task('t7', 'Einkaufen', 'privat', 5, '12:00', 60),
  task('t8', 'Fahrrad abholen', 'privat', 1, null, 0, 'COMPLETED'), task('t9', 'MQTT-Broker testen', 'smart', 7, '16:00', 60),
  ...[['t12', 'Vorlesung nacharbeiten', 'uni', 60], ['t13', 'README überarbeiten', 'kairo', 45], ['t14', 'Sensor-Batterien tauschen', 'smart', 15]]
    .map(([id, title, p, min]) => ({ ...task(id, title, p, 0, null, min), planned_date: null })), // ungeplant
  task('t10', 'Zusatz A', 'kairo', 3, null, 0), task('t11', 'Zusatz B', 'kairo', 3, null, 0),
]
const times = [ // der letzte Eintrag läuft seit 23 Minuten
  { id: 'x1', task_id: 't3', project_id: 'p3', started_at: at(2, '13:05'), ended_at: at(2, '14:40') },
  { id: 'x2', task_id: 't1', project_id: 'p1', started_at: at(0, '14:05'), ended_at: at(0, '15:20') },
  { id: 'x3', task_id: 't2', project_id: 'p2', started_at: at(1, '16:10'), ended_at: at(1, '17:40') },
  { id: 'x4', task_id: 't4', project_id: 'p2', started_at: new Date(Date.now() - 23 * 60_000).toISOString(), ended_at: null },
]
const inWindow = (key, list, from, to) => list.filter((e) => e[key[0]] >= from && e[key[1] ?? key[0]] < to)
const mock = {
  '/api/projects': () => projects,
  '/api/tasks': () => tasks,
  '/api/calendar/occurrences': (q) => events.filter((e) => e.occurrence_end > q.get('from') && e.occurrence_start < q.get('to')),
  '/api/time-entries': (q) => inWindow(['started_at'], times, q.get('from'), q.get('to')),
}

// --- Browser
let browser
try {
  browser = await chromium.launch()
} catch (e) {
  throw new Error(`Chromium fehlt: npx playwright install chromium (${e.message.split('\n')[0]})`)
}
const page = await (await browser.newContext({ viewport: { width, height }, colorScheme: o.theme })).newPage()
await page.addInitScript(([view, theme, aside]) => {
  localStorage.setItem('kairo-cal-view', view)
  localStorage.setItem('kairo-cal-weekend', '1')
  localStorage.setItem('kairo-cal-unplanned', aside === 'open' ? '1' : '0')
  localStorage.setItem('kairo-theme', theme)
}, [VIEWS[o.view], o.theme, o.aside])
await page.route('**/api/**', (route) => {
  const u = new URL(route.request().url())
  const f = mock[u.pathname]
  return f && route.request().method() === 'GET' ? route.fulfill({ json: f(u.searchParams) }) : route.continue()
})
try {
  await page.goto(`${base}/calendar`)
} catch {
  await browser.close()
  throw new Error(`${base} nicht erreichbar: erst „npm run dev“ starten (oder KAIRO_URL setzen)`)
}
await page.waitForSelector('.fc-view')
await page.waitForLoadState('networkidle')
await page.waitForTimeout(400)
mkdirSync(out.includes('/') ? out.slice(0, out.lastIndexOf('/')) : '.', { recursive: true })
await page.screenshot({ path: out })
await browser.close()
console.log(out)
