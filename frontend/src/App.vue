<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import ConfirmPopup from 'primevue/confirmpopup'
import { usePrimeVue } from 'primevue/config'
import de from 'primelocale/de.json'
import en from 'primelocale/en.json'
import { lang } from '@/lib/i18n'
import { navItems } from '@/router'
import UndoToast from '@/components/UndoToast.vue'
import SearchPalette from '@/components/SearchPalette.vue'
import { useBackendStatus } from '@/composables/useBackendStatus'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { counts, loadSidebar, pins } from '@/composables/useSidebar'
import { hm } from '@/lib/dates'
import { projectIconOf } from '@/lib/projectIcons'
import { vDialog } from '@/lib/dialog'
import { GOTO } from '@/lib/shortcuts'
import { useShortcuts } from '@/composables/useShortcuts'
import ShortcutHelp from '@/components/ShortcutHelp.vue'
import { store } from '@/lib/storage'
import { dark, prefs } from '@/lib/prefs'

const { online, version } = useBackendStatus()

// Sprache: <html lang> für Screenreader und Silbentrennung, PrimeVue für seine eigenen Texte.
const primevue = usePrimeVue()
watchEffect(() => {
  document.documentElement.lang = lang.value
  primevue.config.locale = lang.value === 'de' ? de.de : en.en
})
const router = useRouter()
const route = useRoute()
const hideSidebar = computed(() => route.path === '/login' || route.path === '/setup')

useLiveEvents(loadSidebar)
const countOf: Record<string, () => string | number> = {
  tasks: () => counts.value.tasks,
  habits: () => counts.value.habits,
  proj: () => counts.value.projects,
  trash: () => counts.value.trash,
}

// „+“ (Handy): Schnelleingabe der Aufgabenliste (die Ansicht fokussiert das Feld).
const newEntry = () => router.push({ path: '/tasks', query: { new: String(Date.now()) } })
// ⌘K und „Suchen …“: Suche über alles; ohne Treffer legt sie eine Aufgabe an.
const searchOpen = ref(false)
function onKey(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey) || hideSidebar.value) return
  if (e.key === 'k') searchOpen.value = !searchOpen.value
  else if (e.key === 'b') toggleSidebar()
  else if (e.key === '[' && history.state?.back) router.back()
  else return
  e.preventDefault()
}

// Pfad in der Kopfleiste: wo man ist, wohin „Zurück“ führt (vue-router merkt sich die vorige Seite in history.state) und Sprung in jeden Bereich.
const labelOf = (path: string) =>
  path.split('?')[0] === '/settings'
    ? 'Einstellungen'
    : (navItems.find((n) => n.path === path.split('?')[0])?.label ?? '')
const here = computed(() => labelOf(route.path) || 'Kairo')
const backLabel = computed(
  () => (
    route.fullPath,
    typeof history.state?.back === 'string' ? labelOf(history.state.back) : ''
  ),
)
const crumbOpen = ref(false)
const keyOf = Object.fromEntries(Object.entries(GOTO).map(([k, path]) => [path, k]))
const jumpItems = [...navItems, { path: '/settings', label: 'Einstellungen', icon: 'gear' }].map(
  (n) => ({ ...n, key: keyOf[n.path] }),
)
onMounted(() => {
  window.addEventListener('keydown', onKey)
  void loadSidebar()
})

// ? zeigt alle Kürzel, g und ein Buchstabe springt in eine Ansicht (lib/shortcuts.ts).
const helpOpen = ref(false)
let gAt = 0
useShortcuts((e) => {
  if (hideSidebar.value) return
  const go = Date.now() - gAt < 1500 && GOTO[e.key]
  gAt = 0
  if (e.key === '?') helpOpen.value = !helpOpen.value
  else if (go) void router.push(go)
  else if (e.key === 'g') gAt = Date.now()
  else return
  e.preventDefault()
  e.stopImmediatePropagation() // sonst reagiert die Ansicht auch auf den Buchstaben
}, true)
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

// Seitenleiste einklappen: nur Symbole, damit die Ansicht mehr Platz bekommt. Ohne gespeicherte Wahl startet sie im schmalen Fenster eingeklappt.
const mini = ref(
  (store.get('kairo-sidebar') ?? (matchMedia('(max-width:960px)').matches ? 'mini' : 'full')) ===
    'mini',
)
function toggleSidebar() {
  mini.value = !mini.value
  store.set('kairo-sidebar', mini.value ? 'mini' : 'full')
  // Der Kalender misst sich nur bei Fenster-Resize neu.
  void nextTick(() => window.dispatchEvent(new Event('resize')))
}

// Handy: sechs Tabs für den Alltag, der Rest (und Einstellungen) im „Mehr“-Blatt. Die Seitenleiste blendet design.css unter 760px aus.
const tabPaths = ['/today', '/calendar', '/tasks', '/habits', '/projects', '/notes']
const tabs = tabPaths.map((p) => navItems.find((n) => n.path === p)!)
const more = navItems.filter((n) => !tabPaths.includes(n.path))
const moreOpen = ref(false)
const showFab = computed(() => route.path === '/today' || route.path === '/tasks') // sonst verdeckt er Zeilenaktionen
const inMore = computed(() => !tabPaths.includes(route.path))
watch(
  () => route.path,
  () => {
    moreOpen.value = false
    crumbOpen.value = false
  },
)

// Theme: der Knopf wählt das Gegenteil des aktuellen; „System“ gibt es in den Einstellungen.
const toggleTheme = () => (prefs.theme = dark.value ? 'light' : 'dark')

function goBack() {
  crumbOpen.value = false
  router.back()
}

function openSearchFromMore() {
  moreOpen.value = false
  searchOpen.value = true
}
</script>

<template>
  <svg width="0" height="0" style="position: absolute" aria-hidden="true">
    <defs>
      <symbol id="i-cal" viewBox="0 0 24 24">
        <rect x="3.5" y="4.5" width="17" height="16" rx="2" />
        <path d="M3.5 9.5h17M8 2.5v4M16 2.5v4" />
      </symbol>
      <symbol id="i-clock" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="8.5" />
        <path d="M12 7.5V12l3 2" />
      </symbol>
      <symbol id="i-today" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="8.5" />
        <circle cx="12" cy="12" r="2.4" fill="currentColor" stroke="none" />
      </symbol>
      <symbol id="i-tasks" viewBox="0 0 24 24">
        <rect x="4" y="4" width="16" height="16" rx="3" />
        <path d="m8.5 12.3 2.4 2.4 4.8-5.4" />
      </symbol>
      <symbol id="i-habits" viewBox="0 0 24 24">
        <path d="M21 3v5h-5M3 21v-5h5" />
        <path d="M4.6 9.5a8 8 0 0 1 13.4-3.3L21 8M3 16l3 1.8a8 8 0 0 0 13.4-3.3" />
      </symbol>
      <symbol id="i-proj" viewBox="0 0 24 24">
        <path
          d="M3.5 7a2 2 0 0 1 2-2H9l2 2.5h7.5a2 2 0 0 1 2 2V17a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z"
        />
      </symbol>
      <symbol id="i-goal" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="8.5" />
        <circle cx="12" cy="12" r="4.5" />
        <circle cx="12" cy="12" r="0.8" />
      </symbol>
      <symbol id="i-journal" viewBox="0 0 24 24">
        <path d="M15.5 4.5l4 4L9 19H5v-4z" />
        <path d="M13 7l4 4" />
      </symbol>
      <symbol id="i-cards" viewBox="0 0 24 24">
        <rect x="3.5" y="7.5" width="13" height="12" rx="1.5" />
        <path d="M7.5 4.5h11a2 2 0 0 1 2 2v9.5" />
      </symbol>
      <symbol id="i-bars" viewBox="0 0 24 24">
        <path d="M4.5 20.5V13M9.75 20.5V5M15 20.5v-7M20.25 20.5V9" />
      </symbol>
      <symbol id="i-grid" viewBox="0 0 24 24">
        <rect x="4" y="4" width="7" height="7" rx="1.5" />
        <rect x="13" y="4" width="7" height="7" rx="1.5" />
        <rect x="4" y="13" width="7" height="7" rx="1.5" />
        <rect x="13" y="13" width="7" height="7" rx="1.5" />
      </symbol>
      <symbol id="i-list" viewBox="0 0 24 24"><path d="M5 7h14M5 12h14M5 17h14" /></symbol>
      <symbol id="i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></symbol>
      <symbol id="i-search" viewBox="0 0 24 24">
        <circle cx="11" cy="11" r="6.5" />
        <path d="m16 16 4.5 4.5" />
      </symbol>
      <symbol id="i-left" viewBox="0 0 24 24"><path d="M14.5 5.5 8 12l6.5 6.5" /></symbol>
      <symbol id="i-right" viewBox="0 0 24 24"><path d="M9.5 5.5 16 12l-6.5 6.5" /></symbol>
      <symbol id="i-sun" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="4" />
        <path
          d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5 5l1.4 1.4M17.6 17.6 19 19M19 5l-1.4 1.4M6.4 17.6 5 19"
        />
      </symbol>
      <symbol id="i-moon" viewBox="0 0 24 24">
        <path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" />
      </symbol>
      <symbol id="i-x" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18" /></symbol>
      <symbol id="i-link" viewBox="0 0 24 24">
        <path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.2 1.2" />
        <path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.2-1.2" />
      </symbol>
      <symbol id="i-file" viewBox="0 0 24 24">
        <path
          d="M13.5 2.5H7A1.5 1.5 0 0 0 5.5 4v16A1.5 1.5 0 0 0 7 21.5h10a1.5 1.5 0 0 0 1.5-1.5V7.5z"
        />
        <path d="M13.5 2.5V7.5h5" />
      </symbol>
      <symbol id="i-note" viewBox="0 0 24 24">
        <path
          d="M6.5 3.5h11a1.5 1.5 0 0 1 1.5 1.5v14a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 19V5a1.5 1.5 0 0 1 1.5-1.5z"
        />
        <path d="M8.5 8h7M8.5 12h7M8.5 16h4" />
      </symbol>
      <symbol id="i-md" viewBox="0 0 24 24">
        <rect x="2.5" y="5.5" width="19" height="13" rx="2" />
        <path d="M6 15V9l2.5 3L11 9v6M16.5 9v5.5M14.3 12.8l2.2 2.2 2.2-2.2" />
      </symbol>
      <symbol id="i-pdf" viewBox="0 0 24 24">
        <path
          d="M13.5 2.5H7A1.5 1.5 0 0 0 5.5 4v16A1.5 1.5 0 0 0 7 21.5h10a1.5 1.5 0 0 0 1.5-1.5V7.5z"
        />
        <path d="M13.5 2.5V7.5h5" />
        <rect x="3" y="12.5" width="12" height="6" rx="1.2" fill="currentColor" stroke="none" />
      </symbol>
      <symbol id="i-image" viewBox="0 0 24 24">
        <rect x="3.5" y="4.5" width="17" height="15" rx="2" />
        <circle cx="9" cy="9.5" r="1.6" />
        <path d="m4 17.5 5-5 4 4 2.5-2.5 4.5 4.5" />
      </symbol>
      <symbol id="i-folder-up" viewBox="0 0 24 24">
        <path
          d="M3.5 7a2 2 0 0 1 2-2H9l2 2.5h7.5a2 2 0 0 1 2 2V17a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2zM12 16.5v-6M9.5 13 12 10.5 14.5 13"
        />
      </symbol>
      <symbol id="i-draw" viewBox="0 0 24 24">
        <path d="m15 4.5 4.5 4.5-9 9H6v-4.5z" />
        <path d="M3.5 20.5c2-.8 3.5-.8 5 0s3 .8 4.5 0 3-.8 4.5 0" />
      </symbol>
      <symbol id="i-code" viewBox="0 0 24 24">
        <path d="M8.5 7.5 4 12l4.5 4.5M15.5 7.5 20 12l-4.5 4.5" />
      </symbol>
      <symbol id="i-upload" viewBox="0 0 24 24">
        <path d="M12 15V4.5M7.5 9 12 4.5 16.5 9M4.5 15v3.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15" />
      </symbol>
      <symbol id="i-cursor" viewBox="0 0 24 24">
        <path d="M6 3.5v15l4.2-4 2.8 6.3 2.4-1-2.8-6.2h5.9z" />
      </symbol>
      <symbol id="i-marker" viewBox="0 0 24 24">
        <path d="m14.5 3.5 6 6-8.5 8.5H9l-1.5-1.5v-3z" />
        <path d="m11.5 7.5 5 5M7.5 16.5 4 20h5" />
      </symbol>
      <symbol id="i-line" viewBox="0 0 24 24"><path d="M5 19 19 5" /></symbol>
      <symbol id="i-arrow" viewBox="0 0 24 24"><path d="M5 19 19 5M10 5h9v9" /></symbol>
      <symbol id="i-rect" viewBox="0 0 24 24">
        <rect x="4" y="5.5" width="16" height="13" rx="1.5" />
      </symbol>
      <symbol id="i-ellipse" viewBox="0 0 24 24">
        <ellipse cx="12" cy="12" rx="8.5" ry="7" />
      </symbol>
      <symbol id="i-text" viewBox="0 0 24 24"><path d="M5.5 7V5h13v2M12 5v14M9 19h6" /></symbol>
      <symbol id="i-undo" viewBox="0 0 24 24">
        <path d="M9 14 4.5 9.5 9 5" />
        <path d="M4.5 9.5H15a4.5 4.5 0 0 1 0 9h-3" />
      </symbol>
      <symbol id="i-redo" viewBox="0 0 24 24">
        <path d="m15 14 4.5-4.5L15 5" />
        <path d="M19.5 9.5H9a4.5 4.5 0 0 0 0 9h3" />
      </symbol>
      <symbol id="i-minus" viewBox="0 0 24 24"><path d="M5 12h14" /></symbol>
      <symbol id="i-check" viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5" /></symbol>
      <symbol id="i-more" viewBox="0 0 24 24" fill="currentColor">
        <circle cx="5.5" cy="12" r="1.5" />
        <circle cx="12" cy="12" r="1.5" />
        <circle cx="18.5" cy="12" r="1.5" />
      </symbol>
      <!-- Gewohnheiten (lib/habitIcons.ts) -->
      <symbol id="hi-book" viewBox="0 0 24 24">
        <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20" />
      </symbol>
      <symbol id="hi-dumbbell" viewBox="0 0 24 24">
        <path d="M6.5 6.5v11M17.5 6.5v11M3.5 9v6M20.5 9v6M6.5 12h11" />
      </symbol>
      <symbol id="hi-run" viewBox="0 0 24 24">
        <circle cx="15" cy="4.5" r="1.8" />
        <path d="M8 20l3-5 3 2v5M6 11.5l3-3.5 4 1 2.5 3.5 3 .5M11 15l-1.5-5" />
      </symbol>
      <symbol id="hi-bike" viewBox="0 0 24 24">
        <circle cx="5.5" cy="17.5" r="3.5" />
        <circle cx="18.5" cy="17.5" r="3.5" />
        <circle cx="15" cy="5" r="1" />
        <path d="M12 17.5V14l-3-3 4-3 2 3h2" />
      </symbol>
      <symbol id="hi-droplet" viewBox="0 0 24 24">
        <path
          d="M12 22a7 7 0 0 0 7-7c0-2-1-3.9-3-5.5s-3.5-4-4-6.5c-.5 2.5-2 4.9-4 6.5C6 11.1 5 13 5 15a7 7 0 0 0 7 7z"
        />
      </symbol>
      <symbol id="hi-leaf" viewBox="0 0 24 24">
        <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.5 19 2c1 2 2 4.2 2 8 0 5.5-4.8 10-10 10z" />
        <path d="M2 21c0-3 1.9-5.4 5.1-6C9.5 14.5 12 13 13 12" />
      </symbol>
      <symbol id="hi-bed" viewBox="0 0 24 24">
        <path d="M2 4v16M2 8h18a2 2 0 0 1 2 2v10M2 17h20M6 8v9" />
      </symbol>
      <symbol id="hi-food" viewBox="0 0 24 24">
        <path
          d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2M7 2v20M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3zm0 0v7"
        />
      </symbol>
      <symbol id="hi-coffee" viewBox="0 0 24 24">
        <path
          d="M10 2v2M14 2v2M6 2v2M17 8a1 1 0 0 1 1 1v8a4 4 0 0 1-4 4H8a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1h12zM18 9h1a3 3 0 0 1 0 6h-1"
        />
      </symbol>
      <symbol id="hi-pill" viewBox="0 0 24 24">
        <path d="m10.5 20.5 10-10a4.95 4.95 0 1 0-7-7l-10 10a4.95 4.95 0 1 0 7 7zM8.5 8.5l7 7" />
      </symbol>
      <symbol id="hi-heart" viewBox="0 0 24 24">
        <path
          d="M19 14c1.5-1.5 3-3.2 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.8 0-3 .5-4.5 2-1.5-1.5-2.7-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4 3 5.5l7 7z"
        />
      </symbol>
      <symbol id="hi-globe" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="10" />
        <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20M2 12h20" />
      </symbol>
      <symbol id="hi-music" viewBox="0 0 24 24">
        <path d="M9 18V5l12-2v13" />
        <circle cx="6" cy="18" r="3" />
        <circle cx="18" cy="16" r="3" />
      </symbol>
      <symbol id="hi-phone" viewBox="0 0 24 24">
        <rect x="5" y="2" width="14" height="20" rx="2" />
        <path d="M12 18h.01" />
      </symbol>
      <symbol id="hi-sparkles" viewBox="0 0 24 24">
        <path
          d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9zM19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z"
        />
      </symbol>
      <symbol id="hi-wallet" viewBox="0 0 24 24">
        <path
          d="M19 7V4a1 1 0 0 0-1-1H5a2 2 0 0 0 0 4h15a1 1 0 0 1 1 1v4h-3a2 2 0 0 0 0 4h3a1 1 0 0 0 1-1v-2a1 1 0 0 0-1-1M3 5v14a2 2 0 0 0 2 2h15a1 1 0 0 0 1-1v-4"
        />
      </symbol>
      <symbol id="i-flame" viewBox="0 0 24 24">
        <path
          d="M12 3c.5 3.5 5 5.6 5 10.5a5 5 0 0 1-10 0c0-2.2 1.1-3.6 2.3-4.6.2 1.6 1 2.6 2.2 2.9C11 9 10.6 6 12 3z"
        />
      </symbol>
      <symbol id="i-external" viewBox="0 0 24 24">
        <path d="M14 4.5h5.5V10M19.5 4.5 11 13" />
        <path
          d="M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10"
        />
      </symbol>
      <symbol id="i-sidebar" viewBox="0 0 24 24">
        <rect x="3.5" y="4.5" width="17" height="15" rx="3.5" />
        <path d="M9.5 4.5v15" />
      </symbol>
      <symbol id="i-split" viewBox="0 0 24 24">
        <rect x="3.5" y="4.5" width="17" height="15" rx="2" />
        <path d="M12 4.5v15" />
      </symbol>
      <symbol id="i-chev" viewBox="0 0 24 24"><path d="M9.5 6 15.5 12l-6 6" /></symbol>
      <symbol id="i-pen" viewBox="0 0 24 24">
        <path d="M15.5 4.5 19.5 8.5 8.5 19.5H4.5V15.5z" />
        <path d="M13 7l4 4" />
      </symbol>
      <symbol id="i-gear" viewBox="0 0 24 24">
        <path d="M4 7h9M17 7h3M4 17h3M11 17h9" />
        <circle cx="15" cy="7" r="2" />
        <circle cx="9" cy="17" r="2" />
      </symbol>
      <symbol id="i-trash" viewBox="0 0 24 24">
        <path
          d="M4.5 6.5h15M9.5 6.5V4.5h5v2M6.5 6.5l.8 13a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4l.8-13"
        />
      </symbol>
      <symbol id="i-alert" viewBox="0 0 24 24">
        <path
          d="M10.3 3.9 2.4 17.5a2 2 0 0 0 1.7 3h15.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"
        />
        <path d="M12 9.5v4M12 17h.01" />
      </symbol>
      <symbol id="i-inbox" viewBox="0 0 24 24">
        <path
          d="M3.5 13.5 6 5a1.5 1.5 0 0 1 1.4-1h9.2A1.5 1.5 0 0 1 18 5l2.5 8.5V18a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 18z"
        />
        <path d="M3.5 13.5H9a3 3 0 0 0 6 0h5.5" />
      </symbol>
      <symbol id="i-terminal" viewBox="0 0 24 24">
        <polyline points="4 17 10 11 4 5" />
        <line x1="12" y1="19" x2="20" y2="19" />
      </symbol>
      <symbol id="i-briefcase" viewBox="0 0 24 24">
        <rect x="2" y="7" width="20" height="14" rx="2" />
        <path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16" />
      </symbol>
      <symbol id="i-rocket" viewBox="0 0 24 24">
        <path
          d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"
        />
        <path
          d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"
        />
      </symbol>
      <symbol id="i-cpu" viewBox="0 0 24 24">
        <rect x="4" y="4" width="16" height="16" rx="2" />
        <rect x="9" y="9" width="6" height="6" />
        <path d="M9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 14h3M1 9h3M1 14h3" />
      </symbol>
      <symbol id="i-palette" viewBox="0 0 24 24">
        <circle cx="13.5" cy="6.5" r=".5" fill="currentColor" />
        <circle cx="17.5" cy="10.5" r=".5" fill="currentColor" />
        <circle cx="8.5" cy="7.5" r=".5" fill="currentColor" />
        <circle cx="6.5" cy="12.5" r=".5" fill="currentColor" />
        <path
          d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.93 0 1.5-.67 1.5-1.5 0-.39-.15-.74-.39-1-.23-.27-.38-.62-.38-1 0-.83.67-1.5 1.5-1.5H16c3.31 0 6-2.69 6-6 0-5.5-4.5-10-10-10z"
        />
      </symbol>
      <symbol id="i-database" viewBox="0 0 24 24">
        <ellipse cx="12" cy="5" rx="9" ry="3" />
        <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
        <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3" />
      </symbol>
      <symbol id="i-bug" viewBox="0 0 24 24">
        <rect width="8" height="14" x="8" y="6" rx="4" />
        <path d="m19 7-3 2M5 7l3 2M19 19l-3-2M5 19l3-2M20 13h-4M4 13h4M10 4l1 2M14 4l-1 2" />
      </symbol>
      <symbol id="i-cloud" viewBox="0 0 24 24">
        <path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z" />
      </symbol>
      <symbol id="i-shield" viewBox="0 0 24 24">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      </symbol>
      <symbol id="i-zap" viewBox="0 0 24 24">
        <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
      </symbol>
    </defs>
  </svg>
  <div class="app">
    <aside v-if="!hideSidebar" class="sidebar" :class="{ mini }">
      <div class="brand">
        <span class="logo-mark" aria-hidden="true"><i></i></span
        ><span class="brand-t"
          ><b>Kairo</b><small>{{ $t('Persönliches OS') }}</small></span
        >
      </div>
      <nav :aria-label="$t('Hauptnavigation')">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :aria-label="mini ? $t(item.label) : undefined"
          :data-tip="mini ? $t(item.label) : undefined"
        >
          <svg class="ic"><use :href="`#i-${item.icon}`" /></svg
          ><span class="nav-l">{{ $t(item.label) }}</span>
          <span v-if="countOf[item.icon]?.()" class="nav-n">{{ countOf[item.icon]!() }}</span>
        </RouterLink>
      </nav>
      <div v-if="pins.length" class="pins">
        <div class="pins-head">
          <span>{{ $t('Aktiv') }}</span
          ><span>{{ $t('diese Woche') }}</span>
        </div>
        <RouterLink
          v-for="p in pins"
          :key="p.id"
          :to="{ path: '/projects', query: { project: p.id } }"
          class="nav-item pin"
        >
          <svg class="ic pin-ic" aria-hidden="true"><use :href="`#${projectIconOf(p)}`" /></svg
          ><span class="nav-l">{{ p.name }}</span>
          <span v-if="p.minutes" class="nav-n">{{ hm(p.minutes) }}</span>
        </RouterLink>
      </div>
      <div class="sidebar-footer">
        <div class="status" :class="{ off: !online }">
          <span class="status-dot" />
          <span :data-tip="online && version ? version : undefined">{{
            online ? $t('Verbunden') : $t('Getrennt')
          }}</span>
        </div>
        <div class="sb-btns">
          <button
            class="icon-btn"
            :aria-label="$t('Tastenkürzel')"
            :data-tip="`${$t('Tastenkürzel')} (?)`"
            aria-keyshortcuts="?"
            @click="helpOpen = true"
          >
            <kbd class="key sb-help">?</kbd>
          </button>
          <RouterLink
            to="/settings"
            class="icon-btn"
            :aria-label="$t('Einstellungen')"
            :data-tip="$t('Einstellungen')"
            ><svg class="ic"><use href="#i-gear" /></svg
          ></RouterLink>
        </div>
      </div>
    </aside>
    <main class="content" :class="{ 'content-full': hideSidebar }">
      <header v-if="!hideSidebar" class="topbar">
        <button
          type="button"
          class="tb-btn"
          :aria-expanded="!mini"
          aria-keyshortcuts="Meta+B Control+B"
          :aria-label="mini ? $t('Seitenleiste ausklappen') : $t('Seitenleiste einklappen')"
          :data-tip="`${mini ? $t('Ausklappen') : $t('Einklappen')} (⌘B)`"
          @click="toggleSidebar"
        >
          <svg class="ic"><use href="#i-sidebar" /></svg>
        </button>
        <span class="tb-sep" aria-hidden="true"></span>
        <nav class="crumbs" :aria-label="$t('Pfad')">
          <RouterLink to="/today" class="crumb">Kairo</RouterLink
          ><span class="crumb-sl" aria-hidden="true">/</span>
          <div class="menu-wrap" @keydown.esc="crumbOpen = false">
            <button
              type="button"
              class="crumb cur"
              :aria-expanded="crumbOpen"
              aria-haspopup="menu"
              @click="crumbOpen = !crumbOpen"
            >
              {{ $t(here)
              }}<svg class="ic crumb-chev" aria-hidden="true"><use href="#i-chev" /></svg>
            </button>
            <template v-if="crumbOpen">
              <div class="menu-scrim" @click="crumbOpen = false"></div>
              <div class="menu crumb-menu" role="menu">
                <button
                  v-if="backLabel"
                  type="button"
                  class="menu-i crumb-back"
                  role="menuitem"
                  aria-keyshortcuts="Meta+BracketLeft"
                  @click="goBack"
                >
                  <svg class="ic"><use href="#i-left" /></svg
                  >{{ $t('Zurück zu {name}', { name: $t(backLabel) }) }}<kbd class="key">⌘[</kbd>
                </button>
                <div v-if="backLabel" class="crumb-hr"></div>
                <RouterLink
                  v-for="item in jumpItems"
                  :key="item.path"
                  :to="item.path"
                  class="menu-i crumb-i"
                  role="menuitem"
                  :aria-current="item.path === route.path ? 'page' : undefined"
                  @click="crumbOpen = false"
                >
                  <svg class="ic"><use :href="`#i-${item.icon}`" /></svg>{{ $t(item.label)
                  }}<kbd v-if="item.key" class="key">G {{ item.key.toUpperCase() }}</kbd>
                </RouterLink>
              </div>
            </template>
          </div>
        </nav>
        <button
          type="button"
          class="tb-search"
          aria-keyshortcuts="Meta+K Control+K"
          @click="searchOpen = true"
        >
          <svg class="ic"><use href="#i-search" /></svg><span>{{ $t('Suchen oder neu …') }}</span
          ><kbd class="key" aria-hidden="true">⌘K</kbd>
        </button>
        <button
          type="button"
          class="tb-btn"
          :aria-label="$t('Design wechseln')"
          :data-tip="$t('Design wechseln')"
          @click="toggleTheme"
        >
          <svg class="ic"><use :href="dark ? '#i-sun' : '#i-moon'" /></svg>
        </button>
      </header>
      <RouterView />
    </main>
    <template v-if="!hideSidebar">
      <button
        v-if="moreOpen"
        type="button"
        class="sheet-scrim"
        :aria-label="$t('Schließen')"
        @click="moreOpen = false"
      ></button>
      <div
        v-if="moreOpen"
        v-dialog="() => (moreOpen = false)"
        class="more-sheet"
        role="dialog"
        :aria-label="$t('Mehr')"
      >
        <button type="button" class="nav-item" @click="openSearchFromMore">
          <svg class="ic"><use href="#i-search" /></svg><span class="nav-l">{{ $t('Suche') }}</span>
        </button>
        <RouterLink v-for="item in more" :key="item.path" :to="item.path" class="nav-item">
          <svg class="ic"><use :href="`#i-${item.icon}`" /></svg
          ><span class="nav-l">{{ $t(item.label) }}</span>
          <span v-if="countOf[item.icon]?.()" class="nav-n">{{ countOf[item.icon]!() }}</span>
        </RouterLink>
        <RouterLink to="/settings" class="nav-item"
          ><svg class="ic"><use href="#i-gear" /></svg
          ><span class="nav-l">{{ $t('Einstellungen') }}</span></RouterLink
        >
        <button type="button" class="nav-item" @click="toggleTheme">
          <svg class="ic"><use :href="dark ? '#i-sun' : '#i-moon'" /></svg
          ><span class="nav-l">{{ $t('Design wechseln') }}</span>
        </button>
        <div class="status" :class="{ off: !online }">
          <span class="status-dot" />
          <span>{{ online ? $t('Verbunden') : $t('Getrennt') }}</span>
        </div>
      </div>
      <nav class="tabbar" :aria-label="$t('Hauptnavigation')">
        <RouterLink v-for="item in tabs" :key="item.path" :to="item.path" class="tab">
          <svg class="ic"><use :href="`#i-${item.icon}`" /></svg><span>{{ $t(item.label) }}</span>
        </RouterLink>
        <button
          type="button"
          class="tab"
          :class="{ on: inMore || moreOpen }"
          :aria-expanded="moreOpen"
          @click="moreOpen = !moreOpen"
        >
          <svg class="ic"><use href="#i-list" /></svg><span>{{ $t('Mehr') }}</span>
        </button>
      </nav>
      <button v-if="showFab" type="button" class="fab" :aria-label="$t('Neu …')" @click="newEntry">
        <svg class="ic"><use href="#i-plus" /></svg>
      </button>
    </template>
    <SearchPalette v-if="searchOpen" @close="searchOpen = false" />
    <ShortcutHelp v-if="helpOpen" @close="helpOpen = false" />
    <ConfirmPopup />
    <UndoToast />
  </div>
</template>
