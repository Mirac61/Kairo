import { ref } from 'vue'
import { errorMessage } from '@/api/client'

// Laden und Ändern einer Ansicht mit gemeinsamer Fehleranzeige.
// fetch füllt den Zustand der Ansicht; run führt eine Änderung aus und lädt danach neu.
// Schlägt die Änderung fehl, bleibt die Meldung stehen (ein Neuladen würde sie sofort löschen).
export function useLoader(fetch: () => Promise<unknown>) {
  const error = ref('')

  async function load() {
    try {
      await fetch()
      error.value = ''
    } catch (e) {
      error.value = errorMessage(e)
    }
  }

  async function run(fn: () => Promise<unknown>): Promise<boolean> {
    try {
      await fn()
    } catch (e) {
      error.value = errorMessage(e)
      return false
    }
    await load()
    return true
  }

  return { error, load, run }
}
