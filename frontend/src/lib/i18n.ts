import { computed, ref } from 'vue'
import en from './en.ts'

// Deutsch ist die Quelle: der deutsche Text ist der Schlüssel, en.ts übersetzt ihn.
// Fehlt ein Eintrag, bleibt der deutsche Text stehen. {name} wird durch vars.name ersetzt.
export type Lang = 'de' | 'en'
export const lang = ref<Lang>('de')
export const locale = computed(() => (lang.value === 'de' ? 'de-DE' : 'en-GB'))

export function t(de: string, vars?: Record<string, string | number>): string {
  const s = lang.value === 'en' ? (en[de] ?? de) : de
  return vars ? s.replace(/\{(\w+)\}/g, (m, k: string) => (k in vars ? String(vars[k]) : m)) : s
}

// Mehrzahl: n = 1 nimmt one, sonst many; {n} steht für die Zahl.
export const tn = (n: number, one: string, many: string) => t(n === 1 ? one : many, { n })
