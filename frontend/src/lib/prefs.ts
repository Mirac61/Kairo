import { computed, reactive, ref, watch, watchEffect } from 'vue'
import { lang } from './i18n'
import { store } from './storage'

// Einstellungen der Oberfläche (dazu die Sprache). Sie stehen nicht in config.json, sondern unter /api/prefs,
// damit Handy und Rechner gleich eingestellt sind; localStorage ist der Cache für den ersten Paint.
export type Theme = 'dark' | 'light' | 'system'
// Farbstimmung der Flächen; der Akzent bleibt eine eigene Wahl.
export const PALETTES = ['neutral', 'warm', 'slate', 'plum'] as const
export type Palette = (typeof PALETTES)[number]
const paletteOf = (v: string | null): Palette => (PALETTES.includes(v as Palette) ? (v as Palette) : 'warm')
// Statusleiste der installierten App; die Meta-Tags in index.html sind nur der erste Paint.
const THEME_COLORS: Record<Palette, { dark: string; light: string }> = {
  neutral: { dark: '#000000', light: '#fafafa' },
  warm: { dark: '#0d0c0b', light: '#f3efe6' },
  slate: { dark: '#080c12', light: '#f3f6fa' },
  plum: { dark: '#191724', light: '#f7f4fb' },
}
// Akzentfarben als Farbton für --a-hue; Rot und Grün fehlen, weil sie „überfällig“ und „läuft“ bedeuten.
export const ACCENTS = { blue: 255, violet: 300, aqua: 200, pink: 350, orange: 60 } as const
export type Accent = keyof typeof ACCENTS

const KEYS = {
  theme: 'kairo-theme', palette: 'kairo-palette', accent: 'kairo-accent', motion: 'kairo-motion', start: 'kairo-start',
  weekStart: 'kairo-week-start', dayStart: 'kairo-day-start', dayEnd: 'kairo-day-end', calDays: 'kairo-cal-days',
  calHabits: 'kairo-cal-habits', goalFocus: 'kairo-goal-focus', goalDays: 'kairo-goal-days',
} as const

const hour = (key: string, def: number) => {
  const n = Number(store.get(key) ?? def)
  return Number.isInteger(n) && n >= 0 && n <= 24 ? n : def
}

export const prefs = reactive({
  theme: (store.get(KEYS.theme) ?? 'system') as Theme,
  palette: paletteOf(store.get(KEYS.palette)),
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
  // Ziele-Board: Fokus der Woche und Tage, an denen etwas für die Ziele passiert ist (YYYY-MM-DD, durch Leerzeichen getrennt).
  goalFocus: store.get(KEYS.goalFocus) ?? '',
  goalDays: store.get(KEYS.goalDays) ?? '',
})
if (prefs.dayStart >= prefs.dayEnd) Object.assign(prefs, { dayStart: 7, dayEnd: 22 })

const systemLight = ref(matchMedia('(prefers-color-scheme: light)').matches)
matchMedia('(prefers-color-scheme: light)').addEventListener('change', (e) => (systemLight.value = e.matches))
export const dark = computed(() => (prefs.theme === 'system' ? !systemLight.value : prefs.theme === 'dark'))

// Vor dem Mounten aufrufen, damit nichts aufblitzt. Danach folgen <html> und localStorage jeder Änderung.
export function applyPrefs() {
  watchEffect(() => {
    const root = document.documentElement
    const palette = paletteOf(prefs.palette)
    root.dataset.theme = dark.value ? 'dark' : 'light'
    root.dataset.palette = palette
    root.dataset.motion = prefs.motion ? 'reduce' : ''
    root.style.setProperty('--a-hue', String(ACCENTS[prefs.accent] ?? ACCENTS.blue))
    const color = THEME_COLORS[palette][dark.value ? 'dark' : 'light']
    document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((m) => { m.content = color })
  })
  watch(prefs, () => {
    for (const [k, key] of Object.entries(KEYS)) {
      const v = prefs[k as keyof typeof KEYS]
      store.set(key, typeof v === 'boolean' ? (v ? '1' : '0') : String(v))
    }
  })
}

// Abgleich mit dem Server: holen beim Start und wenn die Seite wieder sichtbar wird, schreiben nach jeder Änderung.
// Bevor einmal geholt wurde, wird nichts geschrieben, sonst überschriebe ein neues Gerät den Stand der anderen.
let synced: string | null = null // zuletzt mit dem Server abgeglichener Stand
const snapshot = () => JSON.stringify({ ...prefs, lang: lang.value })

export async function pullPrefs() {
  const res = await fetch('/api/prefs').catch(() => null)
  if (!res?.ok) return // nicht angemeldet oder offline: lokaler Stand gilt weiter
  const data = (await res.json()) as Record<string, unknown>
  const p = prefs as Record<string, unknown>
  for (const k of Object.keys(prefs)) if (typeof data[k] === typeof p[k]) p[k] = data[k]
  prefs.palette = paletteOf(prefs.palette) // alte oder fremde Werte fallen auf Warm zurück
  if (prefs.dayStart >= prefs.dayEnd) Object.assign(prefs, { dayStart: 7, dayEnd: 22 })
  if (data.lang === 'de' || data.lang === 'en') {
    lang.value = data.lang
    store.set('kairo-lang', data.lang)
  }
  synced = Object.keys(data).length ? snapshot() : '' // leer am Server: den lokalen Stand hochladen
  if (!synced) void pushPrefs()
}

async function pushPrefs() {
  const snap = snapshot()
  if (synced === null || snap === synced) return
  const res = await fetch('/api/prefs', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: snap }).catch(() => null)
  if (res?.ok) synced = snap
}

export function syncPrefs() {
  let timer: ReturnType<typeof setTimeout> | undefined
  watch([prefs, lang], () => {
    clearTimeout(timer)
    timer = setTimeout(() => void pushPrefs(), 400)
  })
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') void pullPrefs() })
  void pullPrefs()
}

// Noch nie geholt (z. B. vor dem Login)? Dann beim nächsten Seitenwechsel.
export const prefsPulled = () => synced !== null
