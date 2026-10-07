import { onBeforeUnmount, onMounted } from 'vue'

// Tastenkürzel einer Ansicht. handler läuft nicht mit Modifier, nicht beim Tippen und nicht in Dialogen.
export function useShortcuts(handler: (e: KeyboardEvent) => void) {
  const onKey = (e: KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey || (e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')) return
    handler(e)
  }
  onMounted(() => window.addEventListener('keydown', onKey))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
}
