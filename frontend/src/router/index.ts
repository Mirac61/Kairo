import { createRouter, createWebHistory } from 'vue-router'
import { prefs } from '@/lib/prefs'
import { initAuth, setupRequired, useAuth, setUnauthorizedHandler } from '@/composables/useAuth'
import LoginView from '@/views/LoginView.vue'
import SetupView from '@/views/SetupView.vue'

export const navItems = [
  { path: '/today', name: 'today', label: 'Start', icon: 'today', component: () => import('@/views/TodayView.vue') },
  { path: '/calendar', name: 'calendar', label: 'Kalender', icon: 'cal', component: () => import('@/views/CalendarView.vue') },
  { path: '/tasks', name: 'tasks', label: 'Aufgaben', icon: 'tasks', component: () => import('@/views/TasksView.vue') },
  { path: '/habits', name: 'habits', label: 'Gewohnheiten', icon: 'habits', component: () => import('@/views/HabitsView.vue') },
  { path: '/projects', name: 'projects', label: 'Projekte', icon: 'proj', component: () => import('@/views/ProjectsView.vue') },
  { path: '/notes', name: 'notes', label: 'Notizen', icon: 'note', component: () => import('@/views/NotesView.vue') },
  { path: '/review', name: 'review', label: 'Woche', icon: 'bars', component: () => import('@/views/ReviewView.vue') },
  { path: '/trash', name: 'trash', label: 'Papierkorb', icon: 'trash', component: () => import('@/views/TrashView.vue') },
]

const publicRoutes = ['/login', '/setup']

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView },
    { path: '/setup', name: 'setup', component: SetupView },
    { path: '/', redirect: () => (navItems.some((n) => n.path === prefs.start) ? prefs.start : '/today') },
    ...navItems.map(({ path, name, component }) => ({ path, name, component })),
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/today' },
  ],
})

let authInitialized = false

setUnauthorizedHandler(() => {
  // Auf /setup laufen schon Sidebar-Requests mit 401; nicht von dort wegspringen.
  if (!publicRoutes.includes(router.currentRoute.value.path)) {
    router.push('/login')
  }
})

router.beforeEach(async (to) => {
  if (!authInitialized) {
    authInitialized = true
    await initAuth()
    if (useAuth().authEnabled.value && (await setupRequired())) {
      if (to.path !== '/setup') return '/setup'
      return
    }
  }

  const { isAuthenticated, authEnabled } = useAuth()

  if (!authEnabled.value) return

  if (to.path === '/setup') {
    if (isAuthenticated.value) return '/'
    return
  }

  if (to.path === '/login') {
    if (isAuthenticated.value) return '/'
    return
  }

  if (!isAuthenticated.value && !publicRoutes.includes(to.path)) {
    return '/login'
  }
})

export default router
