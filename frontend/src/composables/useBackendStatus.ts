import { onMounted, onUnmounted, ref } from 'vue'
import { getHealth } from '@/api/client'

const INTERVAL_MS = 10_000

export function useBackendStatus() {
  const online = ref(false)
  const version = ref<string | null>(null)
  let timer: ReturnType<typeof setInterval> | undefined

  async function check() {
    try {
      const h = await getHealth()
      online.value = true
      version.value = h.version ?? null
    } catch {
      online.value = false
      version.value = null
    }
  }

  onMounted(() => {
    void check()
    timer = setInterval(() => void check(), INTERVAL_MS)
  })
  onUnmounted(() => clearInterval(timer))

  return { online, version }
}
