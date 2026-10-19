import { computed, reactive, ref, watch, watchEffect } from 'vue'
import { store } from './storage'

// Einstellungen, die nur in diesem Browser gelten (wie die Sprache). Sie stehen nicht in config.json.
export type Theme = 'dark' | 'light' | 'system'
// Akzentfarben als Farbton für --a-hue; Rot und Grün fehlen, weil sie „überfällig“ und „läuft“ bedeuten.
export const ACCENTS = { blue: 255, violet: 300, aqua: 200, pink: 350, orange: 60 } as const
export type Accent = keyof typeof ACCENTS

const KEYS = {
  theme: 'kairo-theme', accent: 'kairo-accent', motion: 'kairo-motion', start: 'kairo-start',
  weekStart: 'kairo-week-start', dayStart: 'kairo-day-start', dayEnd: 'kairo-day-end', calDays: 'kairo-cal-days',
  calHabits: 'kairo-cal-habits',
} as const

const hour = (key: string, def: number) => {
  const n = Number(store.get(key) ?? def)
  return Number.isInteger(n) && n >= 0 && n <= 24 ? n : def
}

export const prefs = reactive({
  theme: (store.get(KEYS.theme) ?? 'system') as Theme,
  accent: (store.get(KEYS.accent) ?? 'blue') as Accent,
  motion: store.get(KEYS.motion) === '1', // true = Bewegung reduzieren
  start: store.get(KEYS.start) ?? '/today',
  weekStart: store.get(KEYS.weekStart) === '0' ? 0 : 1,
  dayStart: hour(KEYS.dayStart, 7), // sichtbarer Tagesbereich in Kalender und Tagesplan
  dayEnd: hour(KEYS.dayEnd, 22),
  // Tage der Wochenansicht, heute in der Mitte. Früher ein Schalter „Wochenende“ (kairo-cal-weekend).
  calDays: (store.get(KEYS.calDays) ?? (store.get('kairo-cal-weekend') === '1' ? '7' : '5')) === '7' ? 7 : 5,
  // Gewohnheiten auch in der Wochenansicht (in der Tagesansicht immer); sonst füllen sie den Ganztags-Streifen.
  calHabits: store.get(KEYS.calHabits) === '1',
})
if (prefs.dayStart >= prefs.dayEnd) Object.assign(prefs, { dayStart: 7, dayEnd: 22 })

const systemLight = ref(matchMedia('(prefers-color-scheme: light)').matches)
matchMedia('(prefers-color-scheme: light)').addEventListener('change', (e) => (systemLight.value = e.matches))
export const dark = computed(() => (prefs.theme === 'system' ? !systemLight.value : prefs.theme === 'dark'))

// Vor dem Mounten aufrufen, damit nichts aufblitzt. Danach folgen <html> und localStorage jeder Änderung.
export function applyPrefs() {
  watchEffect(() => {
    const root = document.documentElement
    root.dataset.theme = dark.value ? 'dark' : 'light'
    root.dataset.motion = prefs.motion ? 'reduce' : ''
    root.style.setProperty('--a-hue', String(ACCENTS[prefs.accent] ?? ACCENTS.blue))
  })
  watch(prefs, () => {
    for (const [k, key] of Object.entries(KEYS)) {
      const v = prefs[k as keyof typeof KEYS]
      store.set(key, typeof v === 'boolean' ? (v ? '1' : '0') : String(v))
    }
  })
}
