import { api } from '@/api/client'
import { computed, ref } from 'vue'

export interface User {
  id: string
  email: string
  createdAt: string
}

const user = ref<User | null>(null)
const loading = ref(true)
const authEnabled = ref(false)

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null) {
  onUnauthorized = handler
}

export function useAuth() {
  return {
    user: computed(() => user.value),
    loading: computed(() => loading.value),
    isAuthenticated: computed(() => user.value !== null),
    authEnabled: computed(() => authEnabled.value),
  }
}

export async function refreshMe() {
  try {
    const me = await api<User>('/auth/me', { method: 'GET', skipUnauthorizedHandler: true })
    user.value = me
    authEnabled.value = true
    return me
  } catch (err: any) {
    if (err?.status === 401) {
      user.value = null
      authEnabled.value = true
      return null
    }
    if (err?.status === 404) {
      user.value = null
      authEnabled.value = false
      return null
    }
    throw err
  }
}

export async function login(email: string, password: string) {
  await api<void>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  await refreshMe()
}

export async function register(email: string, password: string) {
  await api<User>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  await login(email, password)
}

export async function logout() {
  await api<void>('/auth/logout', { method: 'POST' })
  user.value = null
}

export async function setupRequired() {
  try {
    const res = await api<{ setupRequired: boolean }>('/auth/setup-required', { method: 'GET', skipUnauthorizedHandler: true })
    return res.setupRequired
  } catch (err: any) {
    // Wenn der Server keine Auth-Routen anbietet, gibt es kein Setup.
    if (err?.status === 404) return false
    throw err
  }
}

export function handleUnauthorized() {
  user.value = null
  onUnauthorized?.()
}

export function setAuthUser(me: User | null) {
  user.value = me
}

export async function initAuth() {
  loading.value = true
  try {
    await refreshMe()
  } catch {
    user.value = null
  } finally {
    loading.value = false
  }
}
