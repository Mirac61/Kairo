import { onMounted, onUnmounted } from 'vue'

const DEBOUNCE_MS = 100
const RECONNECT_MS = 2_000

// Ruft onChange bei jedem /ws-Ereignis und bei jedem (Wieder-)Verbinden auf.
// Eine Aktion sendet oft mehrere Ereignisse; sie werden zu einem Aufruf gebündelt.
export function useLiveEvents(onChange: () => void) {
  let ws: WebSocket | undefined
  let debounce: ReturnType<typeof setTimeout> | undefined
  let retry: ReturnType<typeof setTimeout> | undefined
  let closed = false

  const notify = () => {
    clearTimeout(debounce)
    debounce = setTimeout(onChange, DEBOUNCE_MS)
  }

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${location.host}/ws`)
    ws.onopen = notify
    ws.onmessage = notify
    ws.onclose = () => {
      if (!closed) retry = setTimeout(connect, RECONNECT_MS)
    }
  }

  onMounted(connect)
  onUnmounted(() => {
    closed = true
    clearTimeout(debounce)
    clearTimeout(retry)
    ws?.close()
  })
}
