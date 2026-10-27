// Zeitvorschlag für eine Aufgabe: erster freier Platz am Zieltag, über die Daten von
// GET /api/today?date=… gerechnet. day: Zieltag oder null, minutes: Dauer (0 = 30 Minuten),
// nowMin: jetzt in Minuten seit Mitternacht (für „heute“), today: heutiger Tag.
import { computed, ref, watch } from 'vue'
import { getToday } from '@/api/client'
import { blockEndMinutes, findSlot, minutesOfDay } from '@/lib/dayPlan'
import { dayWord, hm } from '@/lib/dates'
import { t } from '@/lib/i18n'

interface Block { key: string; title: string; start: number; end: number }

export function useFreeSlot(day: () => string | null, minutes: () => number, today: () => string, nowMin: () => number) {
  const blocks = ref<Block[]>([])
  const win = ref({ start: 0, end: 24 * 60 })
  let seq = 0

  watch(day, async (d) => {
    if (!d) {
      blocks.value = []
      return
    }
    const n = ++seq
    const t = await getToday(d).catch(() => null)
    if (n !== seq || !t) {
      blocks.value = []
      return
    }
    const mins = (ms: number) => minutesOfDay(ms, t.timezone === 'Local' ? undefined : t.timezone)
    const at = (iso: string) => mins(Date.parse(iso))
    blocks.value = [
      ...t.events.map((e) => ({ key: `e${e.id}${e.occurrence_start}`, title: e.title, start: at(e.occurrence_start), end: blockEndMinutes(at(e.occurrence_start), at(e.occurrence_end)) })),
      ...t.tasks.filter((x) => x.planned_start_at).map((x) => ({ key: `t${x.id}`, title: x.title, start: at(x.planned_start_at!), end: at(x.planned_start_at!) + (x.estimated_minutes || 30) })),
    ].sort((a, b) => a.start - b.start)
    win.value = t.work_end > t.work_start ? { start: t.work_start, end: t.work_end } : { start: 0, end: 24 * 60 }
  }, { immediate: true })

  const slot = computed(() => {
    const d = day()
    if (!d) return null
    const from = d === today() ? nowMin() : win.value.start
    return findSlot(blocks.value, from, win.value.start, win.value.end, minutes() || 30)
  })

  // Lesbarer Hinweis (oder „kein freier Slot“); time ist leer, wenn es keinen Platz gibt.
  const hint = computed(() => {
    const d = day()
    if (!d) return null
    const s = slot.value
    if (!s) return { text: t('Kein freier Slot an diesem Tag'), time: '' }
    const time = hm(s.start)
    const a = s.prev?.title ?? ''
    const b = s.next?.title ?? ''
    const w = dayWord(d, today())
    const text = a && b ? t('Passt {day} um {time} zwischen {a} und {b}', { day: w, time, a, b })
      : a ? t('Passt {day} um {time} nach {a}', { day: w, time, a })
      : b ? t('Passt {day} um {time} vor {b}', { day: w, time, b })
      : t('Passt {day} um {time}', { day: w, time })
    return { text, time }
  })

  return { slot, hint, blocks, win }
}
