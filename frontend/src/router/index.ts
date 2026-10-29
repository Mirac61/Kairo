import { createRouter, createWebHistory } from 'vue-router'
import { prefs } from '@/lib/prefs'
import { initAuth, setupRequired, useAuth, setUnauthorizedHandler } from '@/composables/useAuth'
import LoginView from '@/views/LoginView.vue'
import SetupView from '@/views/SetupView.vue'

export const navItems = [
  {
    path: '/today',
    name: 'today',
    label: 'Start',
    icon: 'today',
    component: () => import('@/views/TodayView.vue'),
  },
  {
    path: '/calendar',
    name: 'calendar',
    label: 'Kalender',
    icon: 'cal',
    component: () => import('@/views/CalendarView.vue'),
  },
  {
    path: '/tasks',
    name: 'tasks',
    label: 'Aufgaben',
    icon: 'tasks',
    component: () => import('@/views/TasksView.vue'),
  },
  {
    path: '/habits',
    name: 'habits',
    label: 'Gewohnheiten',
    icon: 'habits',
    component: () => import('@/views/HabitsView.vue'),
  },
  {
    path: '/projects',
    name: 'projects',
    label: 'Projekte',
    icon: 'proj',
    component: () => import('@/views/ProjectsView.vue'),
  },
  {
    path: '/notes',
    name: 'notes',
    label: 'Notizen',
    icon: 'note',
    component: () => import('@/views/NotesView.vue'),
  },
  {
    path: '/goals',
    name: 'goals',
    label: 'Ziele',
    icon: 'goal',
    component: () => import('@/views/GoalsView.vue'),
  },
  {
    path: '/journal',
    name: 'journal',
    label: 'Journal',
    icon: 'journal',
    component: () => import('@/views/JournalView.vue'),
  },
  {
    path: '/cards',
    name: 'cards',
    label: 'Lernkarten',
    icon: 'cards',
    component: () => import('@/views/CardsView.vue'),
  },
  {
    path: '/review',
    name: 'review',
    label: 'Rückblick',
    icon: 'bars',
    component: () => import('@/views/ReviewView.vue'),
  },
  {
    path: '/trash',
    name: 'trash',
    label: 'Papierkorb',
    icon: 'trash',
    component: () => import('@/views/TrashView.vue'),
  },
]

const publicRoutes = ['/login', '/setup']

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView },
    { path: '/setup', name: 'setup', component: SetupView },
    {
      path: '/',
      redirect: () => (navItems.some((n) => n.path === prefs.start) ? prefs.start : '/today'),
    },
    ...navItems.map(({ path, name, component }) => ({ path, name, component })),
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/today' },
  ],
})

let authReady: Promise<void> | null = null

setUnauthorizedHandler(() => {
  // Auf /setup laufen schon Sidebar-Requests mit 401; nicht von dort wegspringen.
  if (!publicRoutes.includes(router.currentRoute.value.path)) {
    router.push('/login')
  }
})

router.beforeEach(async (to) => {
  // Gemeinsames Promise: parallele Navigationen (z. B. 401-Umleitung beim Start) warten mit.
  await (authReady ??= initAuth())
  const { isAuthenticated, authEnabled } = useAuth()
  if (!authEnabled.value || isAuthenticated.value) {
    return publicRoutes.includes(to.path) && authEnabled.value ? '/' : undefined
  }
  // Nicht angemeldet: Ohne Konto geht es nur zum Setup, sonst nur zum Login.
  const target = (await setupRequired()) ? '/setup' : '/login'
  if (to.path !== target) return target
})

// Ein Tab von vor dem letzten Build kennt nur alte Chunk-Namen; ohne Neuladen lässt sich die Ansicht nicht öffnen.
// Einmal pro Ziel, damit ein wirklich kaputter Build nicht endlos neu lädt.
router.onError((err, to) => {
  if (!/dynamically imported module|module script|Unable to preload/i.test(String(err?.message)))
    return
  const key = 'kairo-chunk-reload'
  if (sessionStorage.getItem(key) === to.fullPath) return
  sessionStorage.setItem(key, to.fullPath)
  location.assign(to.fullPath)
})

export default router
