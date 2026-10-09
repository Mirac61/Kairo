// Prüfung der Notizdateien: node test/noteFiles.test.ts
import assert from 'node:assert/strict'
import { fileKind, fillTemplate, join, previewImages, under, withExt } from '../src/lib/noteFiles.ts'

assert.equal(fileKind('Uni/a.MD'), 'md')
assert.equal(fileKind('skript.pdf'), 'pdf')
assert.equal(fileKind('x.jpeg'), 'image')
assert.equal(fileKind('film.mp4'), '')
assert.equal(fileKind('bild.svg'), '')

const note = 'Uni/Computerarchitektur/VL 1.md'
assert.equal(previewImages('<p><img src="VL%201-1.png" alt=""></p>', note), '<p><img src="/api/files/Uni/Computerarchitektur/VL%201-1.png" alt=""></p>')
assert.equal(previewImages('<img src="../bilder/a.png">', note), '<img src="/api/files/Uni/bilder/a.png">')
assert.equal(previewImages('<img src="a.png">', 'oben.md'), '<img src="/api/files/a.png">')
for (const src of ['https://x.de/a.png', 'data:image/png;base64,AA', '/logo.png']) {
  assert.equal(previewImages(`<img src="${src}">`, note), `<img src="${src}">`)
}

const fri = new Date(2026, 9, 9, 14, 5) // Freitag
assert.equal(
  fillTemplate('# {{titel}}\n{{ordner}} · {{wochentag}}, {{datum}} {{uhrzeit}} {{neu}}', note, fri),
  '# VL 1\nComputerarchitektur · Freitag, 09.10.2026 14:05 {{neu}}',
)
assert.equal(fillTemplate('{{ordner}}', 'oben.md', fri), '')
assert.equal(fillTemplate('{{titel}}', 'Uni/2026-10-09 VL 3.md', fri), 'VL 3') // Datum vorn steckt schon in {{datum}}
assert.equal(fillTemplate('{{titel}}', '2026-10-09.md', fri), '2026-10-09') // nur Datum: bleibt
console.log('noteFiles ok')

// Pfadteile
assert.equal(withExt('Plan'), 'Plan.md')
assert.equal(withExt('Plan.MD'), 'Plan.MD')
assert.equal(withExt('a', '.pdf'), 'a.pdf')
assert.equal(join('', 'a.md'), 'a.md')
assert.equal(join('Uni', 'a.md'), 'Uni/a.md')
assert.ok(under('Uni/a.md', 'Uni') && under('Uni', 'Uni'))
assert.ok(!under('Uni2/a.md', 'Uni'))
