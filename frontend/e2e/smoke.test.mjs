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
  await page.getByRole('button', { name: /Neues Ziel/ }).click()
  await page.getByLabel('Ziel', { exact: true }).fill('Smoke-Ziel')
  await page.getByLabel('Warum?').fill('Weil es geht')
  await page.getByLabel('Bild').setInputFiles({ name: 'bild.png', mimeType: 'image/png', buffer: png })
  await page.getByRole('button', { name: 'Anlegen' }).click()
  await page.locator('.gl-pic', { hasText: 'Smoke-Ziel' }).locator('img').waitFor()
  const [g] = (await api('/goals')).filter((x) => x.title === 'Smoke-Ziel')
  assert.equal(g.why, 'Weil es geht')
  assert.match(g.image_path, /^Ziele\/.+\.png$/)
  assert.ok(existsSync(join(env.KAIRO_NOTES_DIR, g.image_path)))
})

test('Journal speichern', async () => {
  await page.goto(`${base}/journal`)
  await page.getByLabel('Was lief gut?').fill('Smoke grün')
  await page.getByLabel('Was ist morgen wichtig?').fill('Weiter so')
  const saved = page.waitForResponse((r) => r.url().includes('/api/notes/Journal/') && r.request().method() === 'PUT')
  await page.locator('.je-saved').waitFor()
  await saved
  const d = new Date()
  const file = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}.md`
  const md = readFileSync(join(env.KAIRO_NOTES_DIR, 'Journal', file), 'utf8')
  assert.match(md, /## Was lief gut\?\n\nSmoke grün\n\n## Was lerne ich daraus\?\n\n## Was ist morgen wichtig\?\n\nWeiter so\n/)
})

// Kein Textverlust: Der Wechsel schreibt den offenen Eintrag, und geöffnete Felder sind sofort aufgeklappt.
test('Journal: Autosave beim Verlassen und beim Eintragswechsel', async () => {
  const journalDir = join(env.KAIRO_NOTES_DIR, 'Journal')
  mkdirSync(journalDir, { recursive: true })
  const dateOf = (offset) => { const d = new Date(); d.setDate(d.getDate() + offset); return d.toLocaleDateString('sv') }
  const two = dateOf(-2)
  const three = dateOf(-3)
  writeFileSync(join(journalDir, `${two}.md`), `# Journal ${two}\n\n## Was lief gut?\n\nZeile eins\nZeile zwei\nZeile drei\n`)
  writeFileSync(join(journalDir, `${three}.md`), `# Journal ${three}\n\n## Was lief gut?\n\nAnderer Tag\n`)

  // Verlassen: Ein Klick auf die Navigation wechselt die Ansicht ohne Neuladen der Seite.
  await page.goto(`${base}/journal`)
  await page.getByLabel('Was lief gut?').fill('Kein Verlust beim Verlassen')
  const leaving = page.waitForResponse((r) => r.url().includes('/api/notes/Journal/') && r.request().method() === 'PUT')
  await page.locator('.sidebar').getByRole('link', { name: /^Aufgaben/ }).click()
  await leaving
  await page.waitForURL(/\/tasks$/)
  assert.match(readFileSync(join(journalDir, `${dateOf(0)}.md`), 'utf8'), /Kein Verlust beim Verlassen/)

  // Öffnen: drei Zeilen stehen sofort vollständig da, nicht abgeschnitten.
  await page.goto(`${base}/journal`)
  await page.locator(`.jr-entry[data-date="${two}"] .jr-row`).click()
  const area = page.locator(`.jr-entry[data-date="${two}"] .je-area`).first()
  await page.waitForFunction((sel) => {
    const el = document.querySelector(sel)
    return el && el.scrollHeight <= el.clientHeight + 1
  }, `.jr-entry[data-date="${two}"] .je-area`)

  // Wechsel: Erst wird der alte Eintrag geschrieben, dann der neue geöffnet.
  await area.fill('Wechsel eins\nWechsel zwei')
  const saving = page.waitForResponse((r) => r.url().includes(`/api/notes/Journal/${two}.md`) && r.request().method() === 'PUT')
  await page.locator(`.jr-entry[data-date="${three}"] .jr-row`).click()
  await saving
  await page.locator(`.jr-entry[data-date="${two}"] .jr-row`).click()
  assert.equal(await page.locator(`.jr-entry[data-date="${two}"] .je-area`).first().inputValue(), 'Wechsel eins\nWechsel zwei')
})

// Die 12 Wochen öffnen den passenden Tag; ein Tab-Stopp, Pfeiltasten wandern durchs Raster.
test('Journal: Heatmap öffnet Einträge', async () => {
  const day = (offset) => { const d = new Date(); d.setDate(d.getDate() + offset); return d.toLocaleDateString('sv') }
  const two = day(-2)
  await page.goto(`${base}/journal`)
  const heat = page.locator('.jr-heat')
  await heat.waitFor()
  assert.equal(await heat.getAttribute('role'), 'group')
  assert.match(await heat.getAttribute('aria-label'), /Eintr(ag|äge) in den letzten 12 Wochen$/)
  await page.getByText('Tag mit Eintrag').waitFor()
  assert.equal(await page.locator('.jr-heat-cell').count(), 84)
  assert.equal(await page.locator('.jr-heat-cell[tabindex="0"]').count(), 1)

  // Ein Tag mit Eintrag klappt genau diese Zeile auf.
  const cell = page.locator(`.jr-heat-cell[data-date="${two}"]`)
  assert.match(await cell.getAttribute('class'), /\bon\b/)
  await cell.click()
  await page.locator(`.jr-entry[data-date="${two}"] .je-area`).first().waitFor()
  assert.equal(await page.locator(`.jr-entry[data-date="${two}"] .jr-row`).getAttribute('aria-expanded'), 'true')

  // Pfeiltasten verschieben den Fokus: hoch = ein Tag zurück, links = eine Woche zurück.
  await cell.focus()
  await page.keyboard.press('ArrowUp')
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('data-date')), day(-3))
  await page.keyboard.press('ArrowLeft')
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('data-date')), day(-10))

  // Ein leerer Tag öffnet den Nachtragen-Dialog mit diesem Datum.
  const empty = day(-40)
  await page.locator(`.jr-heat-cell[data-date="${empty}"]`).click()
  const dialog = page.getByRole('dialog', { name: 'Eintrag nachtragen' })
  await dialog.waitFor()
  assert.equal(await dialog.getByLabel('Datum').inputValue(), empty)
  await dialog.getByRole('button', { name: 'Abbrechen' }).click()
})

test('Journal: Einträge durchsuchen und sortieren', async () => {
  const day = (offset) => { const d = new Date(); d.setDate(d.getDate() + offset); return d.toLocaleDateString('sv') }
  const two = day(-2)
  await page.goto(`${base}/journal`)
  await page.locator('.jr-entry').first().waitFor()
  const dates = () => page.locator('.jr-entry').evaluateAll((els) => els.map((e) => e.getAttribute('data-date')))
  const descending = await dates()
  assert.ok(descending.length >= 2)
  assert.deepEqual(descending, [...descending].sort().reverse(), 'neueste zuerst')

  // Suche nach dem Text eines Eintrags; die Liste zeigt nur noch diesen Tag.
  const search = page.getByLabel('Einträge durchsuchen')
  await search.fill('Wechsel eins')
  await page.waitForFunction(() => document.querySelectorAll('.jr-entry').length === 1)
  assert.deepEqual(await dates(), [two])
  await search.fill('gibt es nicht')
  await page.getByText('Keine Treffer für „gibt es nicht“.').waitFor()

  // Sortierung umdrehen: ältester Tag oben.
  await search.fill('')
  await page.getByRole('button', { name: 'Älteste zuerst' }).click()
  const ascending = await dates()
  assert.deepEqual(ascending, [...ascending].sort(), 'älteste zuerst')
  assert.equal(ascending[0], [...descending].sort()[0])
})

test('Start zeigt das gestrige Journal-Vorhaben mit direktem Link', async () => {
  const yesterday = new Date()
  yesterday.setDate(yesterday.getDate() - 1)
  const date = yesterday.toLocaleDateString('sv')
  const dir = join(env.KAIRO_NOTES_DIR, 'Journal')
  mkdirSync(dir, { recursive: true })

  await page.goto(`${base}/today`)
  assert.equal(await page.locator('.st-journal').count(), 0, 'ohne Eintrag von gestern bleibt der Block verborgen')

  writeFileSync(join(dir, `${date}.md`), `# Journal ${date}\n\n## Was ist morgen wichtig?\n\nMorgen Abgabe vorbereiten.\n`)
  await page.reload()
  const reminder = page.locator('.st-journal')
  await reminder.getByText('Morgen Abgabe vorbereiten.').waitFor()
  await reminder.click()
  const point = page.locator('.jr-entry .je-point-input').first()
  await point.waitFor()
  assert.equal(await point.inputValue(), 'Morgen Abgabe vorbereiten.')
})

test('Journal-Vorhaben als Aufgaben übernehmen', async () => {
  const date = new Date().toLocaleDateString('sv')
  mkdirSync(join(env.KAIRO_NOTES_DIR, 'Journal'), { recursive: true })
  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Journal', `${date}.md`), [
    `# Journal ${date}`,
    '',
    '## Was ist morgen wichtig?',
    '',
    'Abgabe einreichen. Kapitel lesen',
    '- Sport machen',
    '',
  ].join('\n'))

  await page.goto(`${base}/journal`)
  await page.locator('.jr-today .je-point-input').first().waitFor()
  await page.getByRole('button', { name: 'Als Aufgaben übernehmen' }).click()
  const dialog = page.getByRole('dialog', { name: 'Als Aufgaben übernehmen' })
  const candidates = dialog.locator('.je-task-candidate')
  await candidates.nth(1).locator('input[type="text"]').fill('Kapitel 7 lesen')
  const created = page.waitForResponse((response) => response.url().endsWith(`/api/journal/${date}/tasks`) && response.request().method() === 'POST')
  await dialog.getByRole('button', { name: 'Aufgaben anlegen' }).click()
  const response = await created
  assert.equal(response.status(), 201, await response.text())

  await page.getByText('2 Aufgaben übernommen').waitFor()
  const imported = (await api('/tasks')).filter((task) => task.source_journal_date === date)
  assert.deepEqual(imported.map((task) => task.title).sort(), ['Abgabe einreichen. Kapitel lesen', 'Kapitel 7 lesen'])
  const expectedDue = new Date(`${date}T12:00:00`)
  expectedDue.setDate(expectedDue.getDate() + 1)
  for (const task of imported) {
    assert.equal(new Date(task.due_at).toLocaleDateString('sv'), expectedDue.toLocaleDateString('sv'))
  }

  // Dedup: ein bereits importierter Titel wird nicht doppelt angelegt.
  const retry = await api(`/journal/${date}/tasks`, { method: 'POST', body: JSON.stringify({ titles: ['Abgabe einreichen. Kapitel lesen'] }) })
  assert.equal(retry.task_count, 2)
  assert.equal((await api('/tasks')).filter((task) => task.source_journal_date === date).length, 2)

  // Seitenleiste: offene Journal-Aufgaben stehen dort mit Checkbox und lassen sich direkt abhaken.
  const sideTask = page.locator('.jr-open-item', { hasText: 'Kapitel 7 lesen' })
  await sideTask.waitFor()
  await sideTask.getByRole('checkbox').click()
  await sideTask.waitFor({ state: 'detached' })
  assert.equal((await api('/tasks')).find((task) => task.title === 'Kapitel 7 lesen').status, 'COMPLETED')
})

test('Journal: Rückblick bleibt bei vielen Einträgen bündig', async () => {
  const today = new Date().toLocaleDateString('sv')
  for (let i = 1; i <= 8; i++) {
    const task = await api('/tasks', { method: 'POST', body: JSON.stringify({ title: `Rückblick ${i}`, planned_date: today }) })
    await api(`/tasks/${task.id}`, { method: 'PATCH', body: JSON.stringify({ status: 'COMPLETED' }) })
  }
  await page.goto(`${base}/journal`)
  await page.waitForFunction(() => {
    const recap = document.querySelector('.jr-recap-box')
    const today = document.querySelector('.jr-today')
    return recap && today && Math.abs(recap.getBoundingClientRect().bottom - today.getBoundingClientRect().bottom) <= 1
  })
  assert.ok(await page.locator('.jr-recap-body').evaluate((el) => el.scrollHeight > el.clientHeight), 'Rückblick scrollt innen')
})

test('Journal: „Alle Aufgaben“ filtert nach Journal', async () => {
  await page.goto(`${base}/journal`)
  const open = await api('/tasks')
  const expected = open.filter((task) => task.source_journal_date && !['COMPLETED', 'CANCELLED'].includes(task.status))
  assert.ok(expected.length, 'mindestens eine offene Journal-Aufgabe')
  await page.locator('.jr-open-all').click()
  await page.waitForURL(/\/tasks\?filter=journal$/)
  const tab = page.locator('.tk-tabs button', { hasText: 'Aus Journal' })
  await tab.waitFor()
  assert.equal(await tab.getAttribute('aria-pressed'), 'true')
  assert.match(await tab.innerText(), new RegExp(`\\b${expected.length}\\b`))
  assert.equal(await page.locator('.task-row').count(), expected.length)
  await page.locator('.task-row', { hasText: 'Abgabe einreichen. Kapitel lesen' }).waitFor()
})

test('Journal am Handy: Rückblick und offene Aufgaben vor den Einträgen', async () => {
  await page.setViewportSize({ width: 390, height: 844 })
  try {
    await page.goto(`${base}/journal`)
    await page.locator('.jr-open-box').waitFor()
    const box = await page.evaluate(() => {
      const rect = (sel) => document.querySelector(sel)?.getBoundingClientRect()
      return {
        tops: ['.jr-today', '.jr-side-top', '.jr-open-box', '.jr-past'].map((sel) => rect(sel)?.top ?? -1),
        overflow: document.documentElement.scrollWidth - window.innerWidth,
      }
    })
    assert.ok(box.tops.every((top, i) => i === 0 || top > box.tops[i - 1]), `Reihenfolge: ${box.tops}`)
    assert.ok(box.overflow <= 1, `kein Überlauf: ${box.overflow}px`)
  } finally {
    await page.setViewportSize({ width: 1400, height: 900 })
  }
})

test('Journal: verpassten Tag nachtragen', async () => {
  const dir = join(env.KAIRO_NOTES_DIR, 'Journal')
  const day = (offset) => { const d = new Date(); d.setDate(d.getDate() + offset); return d.toLocaleDateString('sv') }
  const date = day(-4)
  const empty = day(-5)
  assert.ok(!existsSync(join(dir, `${date}.md`)))

  await page.goto(`${base}/journal`)
  await page.getByRole('button', { name: 'Nachtragen', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Eintrag nachtragen' })
  await dialog.getByLabel('Datum').fill(date)
  await dialog.getByRole('button', { name: 'Öffnen' }).click()

  const entry = page.locator(`.jr-entry[data-date="${date}"]`)
  const area = entry.locator('.je-area').first()
  await area.waitFor()
  const saved = page.waitForResponse((r) => r.url().includes(`/api/notes/Journal/${date}.md`) && r.request().method() === 'PUT')
  await area.fill('Nachgetragen')
  await entry.locator('.je-saved').waitFor()
  await saved
  assert.match(readFileSync(join(dir, `${date}.md`), 'utf8'), /Nachgetragen/)
  // Mit Datei bleibt die Zeile beim Schließen stehen.
  await entry.locator('.jr-row').click()
  await page.locator(`.jr-entry[data-date="${date}"]`).waitFor()

  // Leerer Entwurf: keine Datei, und beim Schließen verschwindet die Zeile wieder.
  await page.getByRole('button', { name: 'Nachtragen', exact: true }).click()
  await dialog.getByLabel('Datum').fill(empty)
  await dialog.getByRole('button', { name: 'Öffnen' }).click()
  const draft = page.locator(`.jr-entry[data-date="${empty}"]`)
  await draft.locator('.je-area').first().waitFor()
  assert.ok(!existsSync(join(dir, `${empty}.md`)))
  await draft.locator('.jr-row').click()
  await draft.waitFor({ state: 'detached' })
  assert.ok(!existsSync(join(dir, `${empty}.md`)))
})

test('Lernkarte bewerten', async () => {
  mkdirSync(env.KAIRO_NOTES_DIR, { recursive: true })
  writeFileSync(join(env.KAIRO_NOTES_DIR, 'Karten.md'), '- Smoke-Frage :: Smoke-Antwort\n')
  await page.goto(`${base}/cards`)
  // Eine direkt angelegte Karte in neuem Fach; gelernt wird nur das Fach der Notizkarte.
  await page.getByLabel('Fach', { exact: true }).fill('Smoke-Fach')
  await page.getByLabel('Frage', { exact: true }).fill('Eigene Frage')
  await page.getByLabel('Antwort', { exact: true }).fill('Eigene Antwort')
  await page.keyboard.press('Enter')
  await page.locator('.cd-tile', { hasText: 'Smoke-Fach' }).waitFor()
  await page.locator('.cd-tile', { hasText: 'Allgemein' }).getByRole('button', { name: 'Lernen' }).click()
  await page.getByText('Smoke-Frage').waitFor()
  await page.keyboard.press('Space')
  await page.getByText('Smoke-Antwort').waitFor()
  const reviewed = page.waitForResponse((r) => r.url().includes('/api/cards/') && r.request().method() === 'POST')
  await page.keyboard.press('3')
  await reviewed
  await page.getByText('Fertig für heute.').waitFor()
  const due = await api('/cards/due')
  assert.deepEqual(due.map((c) => [c.deck, c.question]), [['Smoke-Fach', 'Eigene Frage']])
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

// Einstellungen gelten auf allen Geräten: ein zweiter Browser (eigener localStorage) übernimmt Theme und Farbstimmung.
test('Einstellungen auf allen Geräten', async () => {
  // Der PUT ist entprellt; erst wenn der Server den Stand hat, darf das zweite Gerät ihn holen.
  async function waitPrefs(want) {
    for (let i = 0; i < 50; i++) {
      const p = await page.evaluate(async () => await (await fetch('/api/prefs')).json())
      if (Object.entries(want).every(([k, v]) => p[k] === v)) return
      await page.waitForTimeout(100)
    }
    throw new Error(`Einstellungen nicht übernommen: ${JSON.stringify(want)}`)
  }
  await page.goto(`${base}/settings`)
  await page.getByRole('button', { name: 'Darstellung' }).click()
  await page.getByRole('radio', { name: /Hell/ }).click()
  await page.getByRole('radio', { name: 'Pflaume' }).click()
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light' && document.documentElement.dataset.palette === 'plum')
  await waitPrefs({ theme: 'light', palette: 'plum' })
  const themeColors = () => page.locator('meta[name="theme-color"]').evaluateAll((els) => els.map((e) => e.getAttribute('content')))
  assert.deepEqual(await themeColors(), ['#f7f4fb', '#f7f4fb']) // Statusleiste folgt der Farbstimmung
  const other = await browser.newContext({ locale: 'de-DE' })
  try {
    const p2 = await other.newPage()
    await p2.goto(`${base}/today`)
    await p2.waitForFunction(() => document.documentElement.dataset.theme === 'light' && document.documentElement.dataset.palette === 'plum', null, { timeout: 5000 })
  } finally {
    await other.close()
  }
  await page.getByRole('radio', { name: /Dunkel/ }).click()
  await page.getByRole('radio', { name: 'Warm' }).click()
  await waitPrefs({ theme: 'dark', palette: 'warm' })
  assert.deepEqual(await themeColors(), ['#0d0c0b', '#0d0c0b'])
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
