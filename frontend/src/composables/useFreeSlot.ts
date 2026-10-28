// Zeitvorschlag für eine Aufgabe: erster freier Platz am Zieltag, über die Daten von
// GET /api/today?date=… gerechnet. day: Zieltag oder null, minutes: Dauer (0 = 30 Minuten),
// nowMin: jetzt in Minuten seit Mitternacht (für „heute“), today: heutiger Tag.

import { computed, ref, watch } from 'vue'
import { getToday } from '@/api/client'
import { blockEndMinutes, findSlot, minutesOfDay } from '@/lib/dayPlan'
import { dayWord, hm } from '@/lib/dates'
import { t } from '@/lib/i18n'

export interface FreeSlotBlock {
  key: string
  title: string
  start: number
  end: number
}

export interface FreeSlotHint {
  text: string
  time: string
}

export function useFreeSlot(
  day: () => string | null,
  minutes: () => number,
  today: () => string,
  nowMin: () => number
) {
  const blocks = ref<FreeSlotBlock[]>([])
  const win = ref({ start: 0, end: 24 * 60 })
  let seq = 0

  watch(
    day,
    async (targetDay) => {
      if (!targetDay) {
        blocks.value = []
        return
      }

      const requestSeq = ++seq
      const todayData = await getToday(targetDay).catch(() => null)

      if (requestSeq !== seq || !todayData) {
        blocks.value = []
        return
      }

      const tz = todayData.timezone === 'Local' ? undefined : todayData.timezone
      const toMinutes = (iso: string) => minutesOfDay(Date.parse(iso), tz)

      const eventBlocks: FreeSlotBlock[] = todayData.events.map((e) => {
        const start = toMinutes(e.occurrence_start)
        const end = toMinutes(e.occurrence_end)
        return {
          key: `e${e.id}${e.occurrence_start}`,
          title: e.title,
          start,
          end: blockEndMinutes(start, end),
        }
      })

      const taskBlocks: FreeSlotBlock[] = todayData.tasks
        .filter((x) => x.planned_start_at)
        .map((x) => {
          const start = toMinutes(x.planned_start_at!)
          const duration = x.estimated_minutes || 30
          return {
            key: `t${x.id}`,
            title: x.title,
            start,
            end: start + duration,
          }
        })

      blocks.value = [...eventBlocks, ...taskBlocks].sort((a, b) => a.start - b.start)

      win.value =
        todayData.work_end > todayData.work_start
          ? { start: todayData.work_start, end: todayData.work_end }
          : { start: 0, end: 24 * 60 }
    },
    { immediate: true }
  )

  const slot = computed(() => {
    const d = day()
    if (!d) return null
    const from = d === today() ? nowMin() : win.value.start
    return findSlot(blocks.value, from, win.value.start, win.value.end, minutes() || 30)
  })

  // Lesbarer Hinweis (oder „kein freier Slot“); time ist leer, wenn es keinen Platz gibt.
  const hint = computed<FreeSlotHint | null>(() => {
    const d = day()
    if (!d) return null

    const s = slot.value
    if (!s) {
      return { text: t('Kein freier Slot an diesem Tag'), time: '' }
    }

    const time = hm(s.start)
    const prevTitle = s.prev?.title ?? ''
    const nextTitle = s.next?.title ?? ''
    const dayName = dayWord(d, today())

    let text: string
    if (prevTitle && nextTitle) {
      text = t('Passt {day} um {time} zwischen {a} und {b}', {
        day: dayName,
        time,
        a: prevTitle,
        b: nextTitle,
      })
    } else if (prevTitle) {
      text = t('Passt {day} um {time} nach {a}', {
        day: dayName,
        time,
        a: prevTitle,
      })
    } else if (nextTitle) {
      text = t('Passt {day} um {time} vor {b}', {
        day: dayName,
        time,
        b: nextTitle,
      })
    } else {
      text = t('Passt {day} um {time}', { day: dayName, time })
    }

    return { text, time }
  })

  return { slot, hint, blocks, win }
}
