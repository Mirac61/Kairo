import { onBeforeUnmount, onMounted } from 'vue'

// Tastenkürzel einer Ansicht. handler läuft nicht mit Modifier, nicht beim Tippen und nicht in Dialogen.
// capture: vor den Ansichten drankommen (die globalen Kürzel in App.vue stoppen dann das Ereignis).
export function useShortcuts(handler: (e: KeyboardEvent) => void, capture = false) {
  const onKey = (e: KeyboardEvent) => {
    if (
      e.ctrlKey ||
      e.metaKey ||
      e.altKey ||
      (e.target as HTMLElement).closest('input, textarea, select, [contenteditable], .overlay')
    )
      return
    handler(e)
  }
  onMounted(() => window.addEventListener('keydown', onKey, capture))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKey, capture))
}
