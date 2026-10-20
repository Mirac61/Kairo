// Smoke-Tests gegen das echte Backend mit leerer Datenbank in einem Temp-Ordner.
// Voraussetzung: `make build` (das Binary enthält die WebUI). Starten: `pnpm e2e`.
// KAIRO_BIN zeigt auf ein anderes Binary, KAIRO_E2E_PORT wählt den Port (Standard 8813).
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { after, before, test } from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'

const port = process.env.KAIRO_E2E_PORT ?? '8813'
const base = `http://127.0.0.1:${port}`
const dir = mkdtempSync(join(tmpdir(), 'kairo-e2e-'))
const env = {
  ...process.env,
  KAIRO_PORT: port,
  KAIRO_DB_PATH: join(dir, 'kairo.db'),
  KAIRO_TOKEN_PATH: join(dir, 'token'),
  KAIRO_CONFIG_PATH: join(dir, 'config.json'),
  KAIRO_NOTES_DIR: join(dir, 'notes'),
}
let server, browser, page

// Direkter API-Zugriff mit dem Token, wie die Extension ihn nutzt.
async function api(path, init = {}) {
  const token = readFileSync(env.KAIRO_TOKEN_PATH, 'utf8').trim()
  const res = await fetch(`${base}/api${path}`, {
    ...init,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
  })
  assert.ok(res.ok, `${init.method ?? 'GET'} ${path}: ${res.status}`)
  return res.status === 204 ? null : res.json()
}

before(async () => {
  server = spawn(process.env.KAIRO_BIN ?? new URL('../../backend/bin/kairo', import.meta.url).pathname, { env, stdio: 'inherit' })
  for (let i = 0; ; i++) {
    try {
      if ((await fetch(`${base}/api/health`)).ok) break
    } catch { /* startet noch */ }
    if (i > 50) throw new Error('Backend startet nicht')
    await new Promise((r) => setTimeout(r, 100))
  }
  browser = await chromium.launch()
  page = await (await browser.newContext({ viewport: { width: 1400, height: 900 }, locale: 'de-DE' })).newPage()
})

after(async () => {
  await browser?.close()
  server?.kill()
})

test('Aufgabe anlegen', async () => {
  await page.goto(`${base}/tasks`)
  await page.getByLabel('Neue Aufgabe hinzufügen').fill('Smoke-Aufgabe 30m @heute')
  await page.keyboard.press('Enter')
  await page.locator('.task-row', { hasText: 'Smoke-Aufgabe' }).waitFor()
  const tasks = await api('/tasks')
  assert.equal(tasks.find((t) => t.title === 'Smoke-Aufgabe')?.estimated_minutes, 30)
})

test('Timer starten und stoppen', async () => {
  await page.goto(`${base}/today`)
  // Die Knöpfe erscheinen erst, wenn die Maus über der Zeile ist.
  const row = page.locator('.task-row', { hasText: 'Smoke-Aufgabe' })
  await row.hover()
  await page.getByRole('button', { name: 'Start: Smoke-Aufgabe' }).click()
  await row.hover()
  await page.getByRole('button', { name: 'Pause: Smoke-Aufgabe' }).click()
  await page.getByRole('button', { name: 'Start: Smoke-Aufgabe' }).waitFor()
  const entries = await api('/time-entries')
  assert.equal(entries.length, 1)
  assert.ok(entries[0].ended_at, 'Zeiteintrag ist abgeschlossen')
})

test('Termin verschieben', async () => {
  const day = new Date().toLocaleDateString('sv')
  const at = (h) => new Date(`${day}T${h}:00`).toISOString()
  const ev = await api('/calendar/events', { method: 'POST', body: JSON.stringify({ title: 'Smoke-Termin', start_at: at('10:00'), end_at: at('11:00') }) })
  await page.addInitScript(() => localStorage.setItem('kairo-cal-view', 'timeGridDay'))
  await page.goto(`${base}/calendar`)
  // Steht der WebSocket, lädt der Kalender neu und ersetzt die Termin-Elemente; erst danach ziehen.
  await page.waitForLoadState('networkidle')
  const event = page.locator('.fc-event', { hasText: 'Smoke-Termin' })
  await event.scrollIntoViewIfNeeded()
  const box = await event.boundingBox()
  const slot = await page.locator('.fc-timegrid-slot').first().boundingBox()
  // Zwei Slots tiefer ziehen; FullCalendar braucht mehrere Mausbewegungen.
  await page.mouse.move(box.x + box.width / 2, box.y + 10)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2, box.y + 10 + 2 * slot.height, { steps: 10 })
  await page.mouse.up()
  await page.waitForResponse((r) => r.url().includes(`/calendar/events/${ev.id}`) && r.request().method() !== 'GET')
  const moved = (await api(`/calendar/events/${ev.id}`)).start_at
  assert.ok(new Date(moved) > new Date(ev.start_at), `verschoben: ${ev.start_at} → ${moved}`)
})

test('Gewohnheit mit Uhrzeit im Kalender abhaken', async () => {
  const day = new Date().toLocaleDateString('sv')
  const idx = (new Date().getDay() + 6) % 7
  await page.goto(`${base}/habits`)
  await page.getByRole('button', { name: 'Neue Gewohnheit' }).click()
  await page.getByLabel('Name').fill('Smoke-Gewohnheit')
  await page.getByRole('button', { name: 'Bestimmte Tage' }).click()
  // Montag ist vorausgewählt; heute dazunehmen, damit sie im Tageskalender erscheint.
  if (idx > 0) await page.getByRole('group', { name: 'Wochentage' }).getByRole('button').nth(idx).click()
  await page.getByLabel('Uhrzeit').fill('10:00')
  await page.getByRole('button', { name: 'Anlegen' }).click()
  await page.locator('.hk-name', { hasText: 'Smoke-Gewohnheit' }).waitFor()
  const h = (await api('/habits')).find((x) => x.name === 'Smoke-Gewohnheit')
  assert.equal(h.frequency_type, 'SPECIFIC_WEEKDAYS')
  assert.equal(h.preferred_time, '10:00')

  // Die Tagesansicht ist aus „Termin verschieben“ noch gespeichert.
  await page.goto(`${base}/calendar`)
  await page.waitForLoadState('networkidle')
  const block = page.locator('.fc-event.kt-habit', { hasText: 'Smoke-Gewohnheit' })
  await block.scrollIntoViewIfNeeded()
  await block.click()
  await page.waitForResponse((r) => r.url().includes(`/habits/${h.id}/completions`) && r.request().method() === 'POST')
  const done = await api(`/habits/${h.id}/completions?from=${day}&to=${day}`)
  assert.equal(done.length, 1)
})

test('Notiz speichern', async () => {
  await page.goto(`${base}/notes`)
  await page.getByRole('button', { name: 'Neue Notiz', exact: true }).click()
  await page.getByLabel('Name der neuen Notiz').fill('Smoke')
  await page.keyboard.press('Enter')
  const editor = page.locator('.cm-content')
  await editor.click()
  await page.keyboard.type('Hallo aus dem Smoke-Test')
  await page.keyboard.press('ControlOrMeta+s')
  await page.waitForResponse((r) => r.url().includes('/api/notes/Smoke.md') && r.request().method() === 'PUT')
  assert.match(readFileSync(join(env.KAIRO_NOTES_DIR, 'Smoke.md'), 'utf8'), /Hallo aus dem Smoke-Test/)
})

// Ziel mit Bild anlegen: das Bild landet im Notizordner unter Ziele/, die Karte zeigt es.
test('Ziel mit Bild anlegen', async () => {
  const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64')
  await page.goto(`${base}/goals`)
  await page.getByRole('button', { name: 'Neues Ziel' }).click()
  await page.getByLabel('Ziel', { exact: true }).fill('Smoke-Ziel')
  await page.getByLabel('Warum?').fill('Weil es geht')
  await page.getByLabel('Bild').setInputFiles({ name: 'bild.png', mimeType: 'image/png', buffer: png })
  await page.getByRole('button', { name: 'Anlegen' }).click()
  await page.locator('.gl-card', { hasText: 'Smoke-Ziel' }).locator('img').waitFor()
  const [g] = (await api('/goals')).filter((x) => x.title === 'Smoke-Ziel')
  assert.equal(g.why, 'Weil es geht')
  assert.match(g.image_path, /^Ziele\/.+\.png$/)
  assert.ok(existsSync(join(env.KAIRO_NOTES_DIR, g.image_path)))
})

test('Journal speichern', async () => {
  const loaded = page.waitForResponse((r) => r.url().includes('/api/notes/Journal/'))
  await page.goto(`${base}/journal`)
  await loaded
  await page.getByLabel('Was lief gut?').fill('Smoke grün')
  await page.getByLabel('Was ist morgen wichtig?').fill('Weiter so')
  const saved = page.waitForResponse((r) => r.url().includes('/api/notes/Journal/') && r.request().method() === 'PUT')
  await page.getByRole('button', { name: 'Speichern' }).click()
  await saved
  const d = new Date()
  const file = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}.md`
  const md = readFileSync(join(env.KAIRO_NOTES_DIR, 'Journal', file), 'utf8')
  assert.match(md, /## Was lief gut\?\n\nSmoke grün\n\n## Was lerne ich daraus\?\n\n## Was ist morgen wichtig\?\n\nWeiter so\n/)
})

test('Lernkarte bewerten', async () => {
  mkdirSync(env.KAIRO_NOTES_DIR, { recursive: true })
  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Karten.md'), '- Smoke-Frage :: Smoke-Antwort\n')
  await page.goto(`${base}/cards`)
  await page.getByText('Smoke-Frage').waitFor()
  await page.keyboard.press('Space')
  await page.getByText('Smoke-Antwort').waitFor()
  const reviewed = page.waitForResponse((r) => r.url().includes('/api/cards/') && r.request().method() === 'POST')
  await page.keyboard.press('3')
  await reviewed
  await page.getByText('Fertig für heute.').waitFor()
  assert.equal((await api('/cards/due')).filter((c) => c.question === 'Smoke-Frage').length, 0)
})

// Rechteck auf PDF und Bild zeichnen und speichern; die Datei auf der Platte muss sich ändern.
test('PDF und Bild bezeichnen', async () => {
  const { PDFDocument } = await import('pdf-lib')
  const doc = await PDFDocument.create()
  doc.addPage([400, 300])
  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Blatt.pdf'), await doc.save())
  const png = await page.evaluate(() => {
    const c = Object.assign(document.createElement('canvas'), { width: 400, height: 300 })
    c.getContext('2d').fillRect(0, 0, 10, 10)
    return c.toDataURL('image/png').split(',')[1]
  })
  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Bild.png'), Buffer.from(png, 'base64'))

  for (const name of ['Blatt.pdf', 'Bild.png']) {
    const before = readFileSync(join(env.KAIRO_NOTES_DIR, name))
    await page.goto(`${base}/notes`)
    await page.getByRole('treeitem', { name: name.replace(/\.\w+$/, ''), exact: true }).click()
    await page.getByRole('button', { name: 'Rechteck' }).click()
    const box = await page.locator('.fe-page').first().boundingBox()
    await page.mouse.move(box.x + 40, box.y + 40)
    await page.mouse.down()
    await page.mouse.move(box.x + 140, box.y + 120, { steps: 5 })
    await page.mouse.up()
    await page.keyboard.press('ControlOrMeta+s')
    await page.waitForResponse((r) => r.url().includes(`/api/files/${name}`) && r.request().method() === 'PUT')
    assert.notDeepEqual(readFileSync(join(env.KAIRO_NOTES_DIR, name)), before, name)
  }
})

// Ordner samt Unterordner hochladen; versteckte und fremde Dateien bleiben draußen, Code lässt sich ansehen.
test('Ordner hinzufügen', async () => {
  const src = join(mkdtempSync(join(tmpdir(), 'kairo-e2e-up-')), 'Mappe')
  mkdirSync(join(src, 'Unter'), { recursive: true })
  writeFileSync(join(src, 'Liste.md'), '# Liste')
  writeFileSync(join(src, 'Unter', 'Tief.md'), '# Tief')
  writeFileSync(join(src, '.DS_Store'), '')
  writeFileSync(join(src, 'Text.txt'), '')
  writeFileSync(join(src, 'Unter', 'Makefile.Others'), 'all: baum\n\tg++ -o baum baum.cpp\n')
  await page.goto(`${base}/notes`)
  await page.locator('input[webkitdirectory]').setInputFiles(src)
  await page.getByRole('treeitem', { name: 'Tief', exact: true }).waitFor()
  assert.equal(readFileSync(join(env.KAIRO_NOTES_DIR, 'Mappe', 'Unter', 'Tief.md'), 'utf8'), '# Tief')
  assert.equal(readFileSync(join(env.KAIRO_NOTES_DIR, 'Mappe', 'Liste.md'), 'utf8'), '# Liste')
  assert.ok(!existsSync(join(env.KAIRO_NOTES_DIR, 'Mappe', 'Text.txt')))
  await page.getByRole('treeitem', { name: 'Makefile.Others', exact: true }).click()
  await page.locator('.cv .cm-content', { hasText: 'g++ -o baum baum.cpp' }).waitFor()
})

// Zeichnung anlegen, Autosave abwarten, in einer Notiz einbetten und per Klick öffnen.
test('Zeichnung', async () => {
  await page.goto(`${base}/notes`)
  await page.getByRole('button', { name: 'Neue Zeichnung', exact: true }).click()
  await page.getByLabel('Name der neuen Zeichnung').fill('Skizze')
  await page.keyboard.press('Enter')
  const canvas = page.locator('.dw canvas.interactive')
  await canvas.waitFor()
  const box = await canvas.boundingBox()
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2)
  await page.keyboard.press('r')
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2 + 160, box.y + box.height / 2 + 100, { steps: 5 })
  await page.mouse.up()
  await page.waitForResponse((r) => r.url().includes('/api/files/Skizze.excalidraw') && r.request().method() === 'PUT')
  assert.match(readFileSync(join(env.KAIRO_NOTES_DIR, 'Skizze.excalidraw'), 'utf8'), /"type": "rectangle"/)

  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Mit Zeichnung.md'), '# Plan\n\n![](Skizze.excalidraw)\n')
  await page.goto(`${base}/notes`)
  await page.getByRole('treeitem', { name: 'Mit Zeichnung', exact: true }).click()
  const img = page.locator('img[data-drawing="Skizze.excalidraw"][src^="blob:"]')
  await img.click()
  await page.locator('.ed-doc .dw canvas.interactive').waitFor()
  await page.locator('.ed-note .md-editor-preview').waitFor() // geteilt: rechts bleibt die Notiz sichtbar
  assert.equal(await page.locator('.medium-zoom-overlay').count(), 0, 'kein Bild-Zoom über der Zeichnung')
})

test('Sprache umschalten', async () => {
  await page.goto(`${base}/settings`)
  await page.getByRole('button', { name: 'English' }).click()
  await page.getByRole('link', { name: 'Tasks' }).waitFor()
  assert.equal(await page.locator('html').getAttribute('lang'), 'en')
  await page.getByRole('button', { name: 'Deutsch' }).click()
  await page.getByRole('link', { name: 'Aufgaben' }).waitFor()
})

// Einstellungen gelten auf allen Geräten: ein zweiter Browser (eigener localStorage) übernimmt das Theme.
test('Einstellungen auf allen Geräten', async () => {
  await page.goto(`${base}/settings`)
  await page.getByRole('button', { name: 'Darstellung' }).click()
  await page.getByRole('radio', { name: /Hell/ }).click()
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light')
  const other = await browser.newContext({ locale: 'de-DE' })
  try {
    const p2 = await other.newPage()
    await p2.goto(`${base}/today`)
    await p2.waitForFunction(() => document.documentElement.dataset.theme === 'light', null, { timeout: 5000 })
  } finally {
    await other.close()
  }
  await page.getByRole('radio', { name: /Dunkel/ }).click()
  await page.waitForFunction(async () => (await (await fetch('/api/prefs')).json()).theme === 'dark')
})

// Handy (390×844): Tab-Leiste und „Neu“-Knopf liegen fest über dem Inhalt und dürfen nichts verdecken.
test('Handy: Fälligkeit sichtbar, Aufgabendetails über der Tab-Leiste', async () => {
  const due = new Date(Date.now() + 2 * 86400e3)
  await api('/tasks', { method: 'POST', body: JSON.stringify({ title: 'Handy-Aufgabe', due_at: due.toISOString() }) })
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: 'de-DE' })
  const m = await ctx.newPage()
  try {
    await m.goto(`${base}/tasks`)
    const row = m.locator('.task-row', { hasText: 'Handy-Aufgabe' })
    await row.waitFor()
    assert.ok(await row.locator('.due').isVisible(), 'Fälligkeit in der Zeile')
    await row.locator('.row-title').click()
    const del = m.locator('.task-aside').getByRole('button', { name: 'Löschen' })
    await del.scrollIntoViewIfNeeded()
    const box = await del.boundingBox()
    const tabTop = (await m.locator('.tabbar').boundingBox()).y
    assert.ok(box.y + box.height <= tabTop, `Löschen endet bei ${box.y + box.height}, Tab-Leiste beginnt bei ${tabTop}`)
    // Kein anderes Element (z. B. der „Neu“-Knopf) liegt über dem Knopf.
    const onTop = await del.evaluate((el) => {
      const r = el.getBoundingClientRect()
      return el.contains(document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2))
    })
    assert.ok(onTop, 'Löschen ist klickbar')
  } finally {
    await ctx.close()
  }
})
