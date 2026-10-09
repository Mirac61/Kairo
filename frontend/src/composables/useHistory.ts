import { computed, ref } from 'vue'

// Rückgängig/Wiederholen über JSON-Schnappschüsse von get(); go() setzt einen Stand per set() zurück.
// changed: der aktuelle Stand weicht vom zuletzt gespeicherten (reset) ab.
export function useHistory<T>(get: () => T, set: (v: T) => void) {
  let hist: string[] = []
  const len = ref(0)
  const at = ref(0)
  const savedAt = ref(0)
  const changed = computed(() => at.value !== savedAt.value)

  function reset() {
    hist = [JSON.stringify(get())]
    len.value = 1
    at.value = savedAt.value = 0
  }
  function commit() {
    hist = hist.slice(0, at.value + 1)
    if (savedAt.value > at.value) savedAt.value = -1 // gespeicherter Stand liegt im verworfenen Zweig
    hist.push(JSON.stringify(get()))
    len.value = hist.length
    at.value = hist.length - 1
  }
  function go(to: number): boolean {
    if (to < 0 || to >= hist.length) return false
    at.value = to
    set(JSON.parse(hist[to]!))
    return true
  }

  return { at, len, changed, reset, commit, go }
}
