import { createRouter, createWebHistory } from 'vue-router'

export const navItems = [
  { path: '/today', name: 'today', label: 'Today', component: () => import('@/views/TodayView.vue') },
  { path: '/calendar', name: 'calendar', label: 'Kalender', component: () => import('@/views/CalendarView.vue') },
  { path: '/tasks', name: 'tasks', label: 'Tasks', component: () => import('@/views/TasksView.vue') },
  { path: '/habits', name: 'habits', label: 'Habits', component: () => import('@/views/HabitsView.vue') },
  { path: '/projects', name: 'projects', label: 'Projekte', component: () => import('@/views/ProjectsView.vue') },
]

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/today' },
    ...navItems.map(({ path, name, component }) => ({ path, name, component })),
    { path: '/:pathMatch(.*)*', redirect: '/today' },
  ],
})
