// Prüfung der Notizdateien: node test/noteFiles.test.ts
import assert from 'node:assert/strict'
import lz from 'lz-string'
import { assetCandidates, assetName, drawingsIn, fileKind, fillTemplate, fold, fuzzyScore, headingsOf, isObsidianDrawing, join, noteCandidates, parseObsidianDrawing, parseWikiLink, previewImages, previewWikiLinks, rankCandidates, resolveLink, resolveNote, slugify, under, withExt } from '../src/lib/noteFiles.ts'

assert.equal(fileKind('Uni/a.MD'), 'md')
assert.equal(fileKind('skript.pdf'), 'pdf')
assert.equal(fileKind('x.jpeg'), 'image')
assert.equal(fileKind('film.mp4'), '')
assert.equal(fileKind('bild.svg'), '')
assert.equal(fileKind('baeume/tree.HPP'), 'code')
assert.equal(fileKind('baeume/Makefile.Others'), 'code')
assert.equal(fileKind('Makefile'), 'code')
assert.equal(fileKind('Makefile.md'), 'md')
assert.equal(fileKind('mymakefile'), '')
assert.equal(fileKind('Uni/Skizze.Excalidraw'), 'excalidraw')

const note = 'Uni/Computerarchitektur/VL 1.md'
assert.equal(previewImages('<p><img src="VL%201-1.png" alt=""></p>', note), '<p><img src="/api/files/Uni/Computerarchitektur/VL%201-1.png" alt=""></p>')
assert.equal(previewImages('<img src="../bilder/a.png">', note), '<img src="/api/files/Uni/bilder/a.png">')
assert.equal(previewImages('<img src="a.png">', 'oben.md'), '<img src="/api/files/a.png">')
for (const src of ['https://x.de/a.png', 'data:image/png;base64,AA', '/logo.png']) {
  assert.equal(previewImages(`<img src="${src}">`, note), `<img src="${src}">`)
}

// Zeichnungen: data-drawing mit dem Pfad im Notizordner, src aus dem Cache, sonst ein leeres Platzhalterbild.
const cached = (p: string) => (p === 'Uni/bilder/a b.excalidraw' ? 'blob:x' : undefined)
assert.equal(previewImages('<img src="../bilder/a%20b.excalidraw" alt="">', note, cached), '<img class="not-zoom" data-drawing="Uni/bilder/a b.excalidraw" src="blob:x" alt="">')
assert.equal(previewImages('<img src="a.excalidraw" class="md-zoom">', 'n.md'), `<img data-drawing="a.excalidraw" src="${previewImages('<img src="x.excalidraw">', 'n.md').match(/src="([^"]*)"/)![1]}" class="not-zoom md-zoom">`)
assert.match(previewImages('<img src="neu.excalidraw">', note, cached), /^<img class="not-zoom" data-drawing="Uni\/Computerarchitektur\/neu.excalidraw" src="data:image\/svg\+xml,/)
assert.deepEqual(drawingsIn('<img data-drawing="a.excalidraw" src=""><img data-drawing="b &amp; c.excalidraw" src="">'), ['a.excalidraw', 'b & c.excalidraw'])

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

// Eingefügte Bilder
const at = new Date(2026, 9, 9, 14, 5, 7)
assert.equal(assetName(note, 'image/png', at), 'VL 1-2026-10-09-140507.png')
assert.equal(assetName('a.md', 'image/jpeg', at, 1, 2), 'a-2026-10-09-140507-2.jpg')

// Obsidian-Links: Pfad ab der Wurzel, neben der Notiz, sonst nach Namen (kürzester Pfad)
const files = ['Excalidraw/a.excalidraw', 'x/assets/b.png', 'assets/b.png', 'Uni/c.png']
assert.equal(resolveLink('Excalidraw/a.excalidraw', 'Uni', files), 'Excalidraw/a.excalidraw')
assert.equal(resolveLink('a.excalidraw', 'Uni', files), 'Excalidraw/a.excalidraw')
assert.equal(resolveLink('b.png', 'Uni', files), 'assets/b.png')
assert.equal(resolveLink('c.png', 'Uni', files), 'Uni/c.png')
assert.equal(resolveLink('fehlt.png', 'Uni', files), '')

// ![[…]] in der Vorschau (NoteEditor macht daraus <img data-wiki>)
const wiki = (html: string) => previewImages(html, note, () => undefined, (n) => resolveLink(n, 'Uni', files))
assert.match(wiki('<img src="a.excalidraw" alt="" data-wiki="" width="900">'), /data-drawing="Excalidraw\/a.excalidraw"/)
assert.match(wiki('<img src="a.excalidraw" alt="" data-wiki="" width="900">'), /width="900"/)
assert.equal(wiki('<img src="b.png" alt="" data-wiki="">'), '<img src="/api/files/assets/b.png" alt="" data-wiki="">')
assert.equal(wiki('<img src="fehlt &amp; weg.png" alt="" data-wiki="">'), '<span class="md-wiki-missing">![[fehlt &amp; weg.png]]</span>')

// Obsidian-Zeichnung: gepackte Szene, Bilder und Formeln (auch über mehrere Zeilen)
assert.ok(isObsidianDrawing('Excalidraw/Drawing 1.excalidraw.md') && !isObsidianDrawing('a.excalidraw'))
const scene = { type: 'excalidraw', elements: [{ id: 'i', type: 'image', fileId: 'abcdef0123' }, { text: 'Größe ≤ n² 🙂', x: 1.5 }], files: {} }
const packed = lz.compressToBase64(JSON.stringify(scene))
const md = `---\nexcalidraw-plugin: parsed\n---\n# Excalidraw Data\n## Embedded Files\nabcdef0123: [[Bild 1.png]]\n0123456789: $$\\begin{array}{c}\n0 \\\\ 1\n\\end{array}$$\nfedcba9876: https://example.org\n\n%%\n## Drawing\n\`\`\`compressed-json\n${packed.slice(0, 40)}\n${packed.slice(40)}\n\`\`\`\n%%\n`
const got = parseObsidianDrawing(md, lz.decompressFromBase64)
assert.deepEqual(got.scene, scene)
assert.deepEqual([...got.embedded], [
  ['abcdef0123', { file: 'Bild 1.png' }],
  ['0123456789', { tex: '\\begin{array}{c}\n0 \\\\ 1\n\\end{array}' }],
  ['fedcba9876', { other: 'https://example.org' }],
])
assert.deepEqual(parseObsidianDrawing('```json\n{"elements":[]}\n```', lz.decompressFromBase64).scene, { elements: [] })
assert.throws(() => parseObsidianDrawing('nur Text', lz.decompressFromBase64))

// [[Wikilinks]]
assert.deepEqual(parseWikiLink('SE'), { name: 'SE', heading: '', label: 'SE' })
assert.deepEqual(parseWikiLink(' SE | Software Engineering '), { name: 'SE', heading: '', label: 'Software Engineering' })
assert.deepEqual(parseWikiLink('SE#Ziele'), { name: 'SE', heading: 'Ziele', label: 'SE > Ziele' })
assert.deepEqual(parseWikiLink('SE#Ziele|Hier'), { name: 'SE', heading: 'Ziele', label: 'Hier' })
assert.deepEqual(parseWikiLink('#Ziele'), { name: '', heading: 'Ziele', label: 'Ziele' })
const notes = ['a.md', 'Uni/SE.md', 'Uni/Sub/SE.md', 'Uni/Sub/Deep/Zeug.md', 'bild.png', 'se.png']
assert.equal(resolveNote('se', 'a', notes), 'Uni/SE.md') // Groß/klein egal, kürzester Pfad
assert.equal(resolveNote('SE.md', '', notes), 'Uni/SE.md')
assert.equal(resolveNote('uni/sub/se', '', notes), 'Uni/Sub/SE.md')
assert.equal(resolveNote('SE', 'Uni/Sub', notes), 'Uni/Sub/SE.md') // neben der Notiz zuerst
assert.equal(resolveNote('Sub/Deep/Zeug', '', notes), 'Uni/Sub/Deep/Zeug.md') // Ordner im Link ungenau: Dateiname zählt
assert.equal(resolveNote('bild', '', notes), '') // nur Notizen
assert.equal(resolveNote('fehlt', '', notes), '')
const wl = '<p><a href="#" data-wikilink="se#Ziele|Alias" target="_blank">Alias</a> <a href="#" data-wikilink="fehlt">fehlt</a> <a href="https://x.de">x</a></p>'
assert.equal(
  previewWikiLinks(wl, (n) => resolveNote(n, '', notes)),
  '<p><a href="#" class="md-wikilink" data-note="Uni/SE.md" data-heading="Ziele">Alias</a> <a href="#" class="md-wikilink md-wiki-unresolved" data-name="fehlt">fehlt</a> <a href="https://x.de">x</a></p>',
)
assert.equal(slugify('Offene Punkte'), 'offene-punkte')
assert.equal(slugify('Übung 3'), 'ubung-3')

// Wiki-Link-Autovervollständigung: Kandidaten und Ranking
const cand = ['a.md', 'Uni/Algo.md', 'Uni/AlgoDat.md', 'Uni/Sub/Bäume.md', 'bild.png', 'skript.pdf', 'Uni/Skizze.excalidraw', 'Uni/Makefile']
assert.deepEqual(noteCandidates(cand).map((c) => c.label), ['a', 'Algo', 'AlgoDat', 'Bäume'])
assert.equal(noteCandidates(cand).find((c) => c.label === 'Bäume')!.detail, 'Uni/Sub')
assert.deepEqual(assetCandidates(cand).map((c) => [c.label, c.kind]), [['bild.png', 'image'], ['skript.pdf', 'pdf'], ['Skizze.excalidraw', 'excalidraw']])
assert.equal(fuzzyScore('algo', 'Algo'), 1000) // exakt
assert.equal(fuzzyScore('algo', 'AlgoDat'), 900) // Präfix
assert.equal(fuzzyScore('baum', 'Bäume'), 900) // Akzente egal, Präfix
assert.equal(fuzzyScore('ume', 'Bäume'), 500) // Teilstring
assert.equal(fuzzyScore('agdt', 'AlgoDat'), 300) // Teilfolge
assert.equal(fuzzyScore('xyz', 'Algo'), 0) // kein Treffer
assert.deepEqual(rankCandidates('algo', noteCandidates(cand)).map((c) => c.label), ['Algo', 'AlgoDat']) // exakt vor Präfix
assert.equal(rankCandidates('', noteCandidates(cand)).length, 0) // leere Eingabe: nichts
assert.equal(fold('BäÜme'), 'baume')
assert.deepEqual(headingsOf('# A\n## B\nnicht\n###  C\n'), ['A', 'B', 'C'])
