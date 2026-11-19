import { computed, reactive, ref, watch, watchEffect } from 'vue'
import { lang } from './i18n.ts'
import { store } from './storage.ts'

// Einstellungen der Oberfläche (dazu die Sprache). Sie stehen nicht in config.json, sondern unter /api/prefs,
// damit Handy und Rechner gleich eingestellt sind; localStorage ist der Cache für den ersten Paint.
export type Theme = 'dark' | 'light' | 'system'
// Farbstimmung; Graphit, Schiefer und Neutral nehmen die Akzentwahl, Carbonfox und Dragon bringen eigene Farben mit.
export const PALETTES = ['graphit', 'slate2', 'neutral', 'carbonfox', 'dragon'] as const
export type Palette = (typeof PALETTES)[number]
const paletteOf = (v: string | null): Palette =>
  PALETTES.includes(v as Palette) ? (v as Palette) : 'graphit'
// Statusleiste der installierten App; die Meta-Tags in index.html sind nur der erste Paint.
const THEME_COLORS: Record<Palette, { dark: string; light: string }> = {
  graphit: { dark: '#141517', light: '#f6f6f4' },
  slate2: { dark: '#090c0d', light: '#f3f5f6' },
  neutral: { dark: '#000000', light: '#fafafa' },
  carbonfox: { dark: '#0c0c0c', light: '#f4f4f4' },
  dragon: { dark: '#181616', light: '#f2ecbc' },
}
// Akzentfarben als Farbton für --a-hue; Rot und Grün fehlen, weil sie „überfällig“ und „läuft“ bedeuten.
export const ACCENTS = { orange: 38, blue: 255, violet: 300, aqua: 200, pink: 350 } as const
export type Accent = keyof typeof ACCENTS

// App-Icon-Varianten: Standard ist der Kairo-Gipfel („peak“), dazu Kairos Fokus („focus“) und K + Funke („monogram“).
export const APP_ICONS = ['peak', 'focus', 'monogram'] as const
export type AppIcon = (typeof APP_ICONS)[number]
const appIconOf = (v: string | null): AppIcon =>
  APP_ICONS.includes(v as AppIcon) ? (v as AppIcon) : 'peak'

const KEYS = {
  theme: 'kairo-theme',
  palette: 'kairo-palette',
  accent: 'kairo-accent',
  appIcon: 'kairo-app-icon',
  motion: 'kairo-motion',
  start: 'kairo-start',
  weekStart: 'kairo-week-start',
  dayStart: 'kairo-day-start',
  dayEnd: 'kairo-day-end',
  calDays: 'kairo-cal-days',
  calHabits: 'kairo-cal-habits',
  name: 'kairo-name',
} as const

const hour = (key: string, def: number) => {
  const n = Number(store.get(key) ?? def)
  return Number.isInteger(n) && n >= 0 && n <= 24 ? n : def
}

export const prefs = reactive({
  theme: (store.get(KEYS.theme) ?? 'system') as Theme,
  palette: paletteOf(store.get(KEYS.palette)),
  accent: (store.get(KEYS.accent) ?? 'orange') as Accent,
  appIcon: appIconOf(store.get(KEYS.appIcon)),
  motion: store.get(KEYS.motion) === '1', // true = Bewegung reduzieren
  start: store.get(KEYS.start) ?? '/today',
  weekStart: store.get(KEYS.weekStart) === '0' ? 0 : 1,
  dayStart: hour(KEYS.dayStart, 7), // sichtbarer Tagesbereich in Kalender und Tagesplan
  dayEnd: hour(KEYS.dayEnd, 22),
  // Tage der Wochenansicht, heute in der Mitte. Früher ein Schalter „Wochenende“ (kairo-cal-weekend).
  calDays:
    (store.get(KEYS.calDays) ?? (store.get('kairo-cal-weekend') === '1' ? '7' : '5')) === '7'
      ? 7
      : 5,
  // Gewohnheiten auch in der Wochenansicht (in der Tagesansicht immer); sonst füllen sie den Ganztags-Streifen.
  calHabits: store.get(KEYS.calHabits) === '1',
  name: store.get(KEYS.name) ?? '', // für die Begrüßung auf der Startseite
})
if (prefs.dayStart >= prefs.dayEnd) Object.assign(prefs, { dayStart: 7, dayEnd: 22 })

const hasMatchMedia = typeof matchMedia === 'function'
const systemLight = ref(hasMatchMedia ? matchMedia('(prefers-color-scheme: light)').matches : false)
if (hasMatchMedia) {
  matchMedia('(prefers-color-scheme: light)').addEventListener(
    'change',
    (e) => (systemLight.value = e.matches),
  )
}
export const dark = computed(() =>
  prefs.theme === 'system' ? !systemLight.value : prefs.theme === 'dark',
)

// Vor dem Mounten aufrufen, damit nichts aufblitzt. Danach folgen <html> und localStorage jeder Änderung.
export function applyPrefs() {
  watchEffect(() => {
    const root = document.documentElement
    const palette = paletteOf(prefs.palette)
    const appIcon = appIconOf(prefs.appIcon)
    root.dataset.theme = dark.value ? 'dark' : 'light'
    root.dataset.palette = palette
    root.dataset.appIcon = appIcon
    root.dataset.motion = prefs.motion ? 'reduce' : ''
    root.style.setProperty('--a-hue', String(ACCENTS[prefs.accent] ?? ACCENTS.orange))
    const color = THEME_COLORS[palette][dark.value ? 'dark' : 'light']
    document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((m) => {
      m.content = color
    })
    const cs = getComputedStyle(root)
    const accentHi = cs.getPropertyValue('--a-blue-hi').trim() || '#d97343'
    const favSvg =
      appIcon === 'focus'
        ? `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#0b0b0c"/><path d="M44.5 36.5 A 15.5 15.5 0 1 1 27.5 17" fill="none" stroke="#f0eeeb" stroke-width="5.2" stroke-linecap="round"/><circle cx="41.8" cy="22.2" r="6.8" fill="${accentHi}"/></svg>`
        : appIcon === 'monogram'
          ? `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#0b0b0c"/><rect x="17" y="16" width="6" height="32" rx="3" fill="#f0eeeb"/><path d="M26 32.8 L42 48" stroke="#f0eeeb" stroke-width="6" stroke-linecap="round"/><path d="M26 31.8 L34.5 23.8" stroke="#52525b" stroke-width="5.4" stroke-linecap="round"/><circle cx="42" cy="19.5" r="5.8" fill="${accentHi}"/></svg>`
          : `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#0b0b0c"/><circle cx="44" cy="20" r="10.5" fill="${accentHi}"/><polygon points="27,22 10,50 31,50" fill="#f0eeeb"/><polygon points="27,22 31,50 45,50" fill="#52525b"/></svg>`
    const fav = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (fav) fav.href = `data:image/svg+xml,${encodeURIComponent(favSvg)}`
    const suffix = appIcon === 'peak' ? '' : `-${appIcon}`
    const touch = document.querySelector<HTMLLinkElement>('link[rel="apple-touch-icon"]')
    if (touch) touch.href = `/icon${suffix}-180.png`
    const mf = document.querySelector<HTMLLinkElement>('link[rel="manifest"]')
    if (mf) mf.href = `/manifest${suffix}.webmanifest`
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
  prefs.palette = paletteOf(prefs.palette) // alte oder fremde Werte fallen auf Graphit zurück
  prefs.appIcon = appIconOf(prefs.appIcon)
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
  const res = await fetch('/api/prefs', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: snap,
  }).catch(() => null)
  if (res?.ok) synced = snap
}

export function syncPrefs() {
  let timer: ReturnType<typeof setTimeout> | undefined
  watch([prefs, lang], () => {
    clearTimeout(timer)
    timer = setTimeout(() => void pushPrefs(), 400)
  })
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') void pullPrefs()
  })
  void pullPrefs()
}

// Noch nie geholt (z. B. vor dem Login)? Dann beim nächsten Seitenwechsel.
export const prefsPulled = () => synced !== null
