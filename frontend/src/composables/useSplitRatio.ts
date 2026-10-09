import { ref } from 'vue'
import { store } from '@/lib/storage'

// Geteilte Ansicht: Breite der linken Hälfte als Anteil (25–75 %), per Trenner oder Pfeiltasten verschiebbar, gemerkt unter key.
export function useSplitRatio(key: string) {
  const clamp = (r: number) => Math.min(0.75, Math.max(0.25, r))
  const ratio = ref(clamp(Number(store.get(key)) || 0.5))
  const keep = () => store.set(key, String(ratio.value))

  function startResize(e: PointerEvent) {
    e.preventDefault()
    const box = (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect()
    const move = (ev: PointerEvent) => (ratio.value = clamp((ev.clientX - box.left) / box.width))
    const up = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      keep()
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }
  function nudge(d: number) {
    ratio.value = clamp(ratio.value + d)
    keep()
  }
  return { ratio, startResize, nudge }
}
