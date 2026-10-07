import { createRouter, createWebHistory } from 'vue-router'

export const navItems = [
  { path: '/today', name: 'today', label: 'Start', icon: 'today', component: () => import('@/views/TodayView.vue') },
  { path: '/calendar', name: 'calendar', label: 'Kalender', icon: 'cal', component: () => import('@/views/CalendarView.vue') },
  { path: '/tasks', name: 'tasks', label: 'Aufgaben', icon: 'tasks', component: () => import('@/views/TasksView.vue') },
  { path: '/habits', name: 'habits', label: 'Gewohnheiten', icon: 'habits', component: () => import('@/views/HabitsView.vue') },
  { path: '/projects', name: 'projects', label: 'Projekte', icon: 'proj', component: () => import('@/views/ProjectsView.vue') },
  { path: '/review', name: 'review', label: 'Woche', icon: 'grid', component: () => import('@/views/ReviewView.vue') },
]

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/today' },
    ...navItems.map(({ path, name, component }) => ({ path, name, component })),
    { path: '/:pathMatch(.*)*', redirect: '/today' },
  ],
})
