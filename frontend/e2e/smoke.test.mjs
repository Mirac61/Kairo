// Smoke-Tests gegen das echte Backend mit leerer Datenbank in einem Temp-Ordner.
// Voraussetzung: `make build` (das Binary enthält die WebUI). Starten: `pnpm e2e`.
// KAIRO_BIN zeigt auf ein anderes Binary, KAIRO_E2E_PORT wählt den Port (Standard 8813).
import { spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
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

test('Sprache umschalten', async () => {
  await page.goto(`${base}/settings`)
  await page.getByLabel('Sprache').selectOption('en')
  await page.getByRole('link', { name: 'Tasks' }).waitFor()
  assert.equal(await page.locator('html').getAttribute('lang'), 'en')
  await page.getByLabel('Language').selectOption('de')
  await page.getByRole('link', { name: 'Aufgaben' }).waitFor()
})
