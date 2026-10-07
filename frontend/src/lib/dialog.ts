import type { Directive } from 'vue'

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

// Für das .overlay eines Dialogs: <div v-if="offen" v-dialog="schließen" class="overlay open">.
// Setzt role="dialog" und aria-modal, holt den Fokus hinein (erstes [autofocus], sonst erstes Eingabefeld),
// hält Tab im Dialog, schließt mit Esc und gibt den Fokus beim Schließen an den Auslöser zurück.
interface State { opener: Element | null; close: () => void; onKey: (e: KeyboardEvent) => void }
const states = new WeakMap<HTMLElement, State>()

export const vDialog: Directive<HTMLElement, () => void> = {
  mounted(el, { value }) {
    const dlg = el.querySelector<HTMLElement>('.dialog') ?? el
    dlg.setAttribute('role', 'dialog')
    dlg.setAttribute('aria-modal', 'true')
    const items = () => [...dlg.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((x) => x.offsetParent !== null)
    const onKey = (e: KeyboardEvent) => {
      const active = document.activeElement
      if (e.key === 'Escape') return states.get(el)?.close()
      if (e.key !== 'Tab' || active?.closest('.p-confirmpopup')) return // die Rückfrage-Blase liegt außerhalb des Dialogs
      const list = items()
      const [first, last] = [list[0], list.at(-1)]
      if (!first || !last) return
      if (!dlg.contains(active) || (e.shiftKey && active === first)) {
        e.preventDefault()
        ;(e.shiftKey ? last : first).focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    }
    states.set(el, { opener: document.activeElement, close: value, onKey })
    // Auf document, damit Esc und Tab auch gelten, wenn ein Klick den Fokus auf <body> gelegt hat.
    document.addEventListener('keydown', onKey)
    ;(dlg.querySelector<HTMLElement>('[autofocus], input:not([disabled]), select:not([disabled]), textarea:not([disabled])') ?? items()[0])?.focus()
  },
  updated(el, { value }) {
    const s = states.get(el)
    if (s) s.close = value
  },
  unmounted(el) {
    const s = states.get(el)
    if (!s) return
    document.removeEventListener('keydown', s.onKey)
    ;(s.opener as HTMLElement | null)?.focus?.()
    states.delete(el)
  },
}
