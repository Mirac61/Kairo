// Leichtgewichtiger LaTeX-Tokenizer ohne MathJax-Import (damit der MathJax-Chunk lazy bleibt).
// Erkennt $$…$$ (Block) und $…$ (Inline), überspringt \$ sowie optional Markdown-Codeblöcke/-Spans
// und behandelt Währungsangaben wie „$5 oder $10“ nicht als Formel (Pandoc/KaTeX-Regel).
export type MathSegment =
  { kind: 'text'; value: string } | { kind: 'math'; expr: string; display: boolean }

function isEscaped(s: string, pos: number): boolean {
  let backslashes = 0
  for (let i = pos - 1; i >= 0 && s[i] === '\\'; i--) backslashes++
  return backslashes % 2 === 1
}

export function splitMath(src: string, opts?: { skipCode?: boolean }): MathSegment[] {
  if (!src.includes('$')) return [{ kind: 'text', value: src }]
  const skipCode = opts?.skipCode ?? false
  const out: MathSegment[] = []
  let textStart = 0
  let i = 0
  const n = src.length

  const flushText = (end: number) => {
    if (end > textStart) out.push({ kind: 'text', value: src.slice(textStart, end) })
  }

  while (i < n) {
    // Markdown-Codeblöcke (```…```) und Inline-Code (`…`) überspringen
    if (skipCode && src[i] === '`') {
      let ticks = 0
      while (i + ticks < n && src[i + ticks] === '`') ticks++
      const fence = '`'.repeat(ticks)
      const close = src.indexOf(fence, i + ticks)
      if (close !== -1) {
        i = close + ticks
        continue
      }
      i += ticks
      continue
    }

    if (src[i] === '$' && !isEscaped(src, i)) {
      // Block-Formel: $$…$$
      if (src[i + 1] === '$') {
        let j = i + 2
        let found = -1
        while (j < n - 1) {
          if (src[j] === '$' && src[j + 1] === '$' && !isEscaped(src, j)) {
            found = j
            break
          }
          j++
        }
        if (found !== -1) {
          const expr = src.slice(i + 2, found).trim()
          if (expr) {
            flushText(i)
            out.push({ kind: 'math', expr, display: true })
            i = found + 2
            textStart = i
            continue
          }
        }
        i += 2
        continue
      }

      // Inline-Formel: $…$ (kein Leerzeichen direkt nach öffnendem $, kein Zeilenumbruch)
      const first = src[i + 1]
      if (first && !/\s/.test(first) && first !== '$') {
        let j = i + 1
        let found = -1
        while (j < n && src[j] !== '\n') {
          if (src[j] === '$' && !isEscaped(src, j)) {
            const prev = src[j - 1]
            const next = src[j + 1]
            if (prev && !/\s/.test(prev) && !(next && /\d/.test(next))) {
              found = j
              break
            }
          }
          j++
        }
        if (found !== -1) {
          const expr = src.slice(i + 1, found).trim()
          if (expr) {
            flushText(i)
            out.push({ kind: 'math', expr, display: false })
            i = found + 1
            textStart = i
            continue
          }
        }
      }
    }
    i++
  }

  flushText(n)
  return out
}

export function hasMath(src: string, opts?: { skipCode?: boolean }): boolean {
  if (!src.includes('$')) return false
  return splitMath(src, opts).some((s) => s.kind === 'math')
}
