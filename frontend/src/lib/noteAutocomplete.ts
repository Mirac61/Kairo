// Wiki-Link-Autovervollständigung für den Notiz-Editor (CodeMirror 6 über md-editor-v3).
// [[…]] vervollständigt Notiznamen, danach # die Überschriften der gewählten Notiz; | ist ein freier Alias.
// ![[…]] vervollständigt Assets (Bild, PDF, Zeichnung). Ohne Treffer steht „Neue Notiz anlegen“ bereit.
import type { Completion, CompletionResult, CompletionSource } from '@codemirror/autocomplete'
import { t } from './i18n'
import {
  assetCandidates,
  fold,
  noteCandidates,
  rankCandidates,
  resolveNote,
  type WikiCandidate,
} from './noteFiles'

export interface WikiAutocomplete {
  path: string
  files: string[]
  readHeadings: (notePath: string) => Promise<string[]>
}

const ICON: Record<WikiCandidate['kind'], string> = {
  note: '▤',
  image: '◧',
  pdf: '▣',
  excalidraw: '✎',
}

const LIMIT = 20

function option(c: WikiCandidate): Completion {
  return {
    label: c.label,
    detail: [ICON[c.kind], c.detail].filter(Boolean).join(' '),
    type: c.kind,
    apply: `${c.label}]]`,
  }
}

export function wikiCompletionSource(opts: WikiAutocomplete): CompletionSource {
  const dir = opts.path.slice(0, Math.max(opts.path.lastIndexOf('/'), 0))
  const headings = new Map<string, Promise<string[]>>()
  const headingCache = (p: string) => {
    let v = headings.get(p)
    if (!v) {
      v = opts.readHeadings(p).catch(() => [])
      headings.set(p, v)
    }
    return v
  }

  return (ctx) => {
    const pos = ctx.pos
    const before = ctx.state.doc.lineAt(pos).text.slice(0, pos - ctx.state.doc.lineAt(pos).from)
    const m = /(\[\[|!\[\[)([^[\]]*)$/.exec(before)
    if (!m) return null
    const embed = m[1] === '![['
    const inner = m[2]!
    const from = pos - inner.length

    if (embed) {
      // ![[Name — der Name vor einem optionalen |900 (Breite) wird vervollständigt.
      const list = rankCandidates(inner.split('|', 1)[0]!, assetCandidates(opts.files)).slice(
        0,
        LIMIT,
      )
      return { from, options: list.map(option) }
    }

    // [[Name|Alias — nach | ist der Alias frei, keine Vorschläge.
    if (inner.includes('|')) return null

    const hash = inner.indexOf('#')
    if (hash >= 0) {
      const target = resolveNote(inner.slice(0, hash), dir, opts.files)
      if (!target) return null
      const q = inner.slice(hash + 1)
      const hFrom = pos - q.length
      return headingCache(target).then((hs) => {
        const list = rankCandidates(
          q,
          hs.map((h) => ({ label: h, path: target, detail: '', kind: 'note' })),
        ).slice(0, LIMIT)
        return {
          from: hFrom,
          options: list.map((c) => ({ label: c.label, type: 'heading', apply: `${c.label}]]` })),
        }
      })
    }

    const list = rankCandidates(inner, noteCandidates(opts.files)).slice(0, LIMIT)
    const options = list.map(option)
    const name = inner.trim()
    if (name && !list.some((c) => fold(c.label) === fold(name))) {
      options.push({
        label: t('Neue Notiz „{name}“ anlegen', { name }),
        type: 'keyword',
        apply: `${name}]]`,
      })
    }
    return { from, options }
  }
}

// Für Tests, die den Trigger nur als Text prüfen wollen (ohne CodeMirror).
export const wikiTrigger = (before: string): { embed: boolean; inner: string } | null => {
  const m = /(\[\[|!\[\[)([^[\]]*)$/.exec(before)
  return m ? { embed: m[1] === '![[', inner: m[2]! } : null
}

export type { CompletionResult }
