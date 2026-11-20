<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  api,
  completeHabit,
  createHabit,
  listHabits,
  uncompleteHabit,
  updateHabit,
  type Habit,
  type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { iconOf } from '@/lib/habitIcons'
import { prefs } from '@/lib/prefs'
import HabitEditDialog from '@/features/habits/components/HabitEditDialog.vue'
import RitualRingItem from './RitualRingItem.vue'

defineProps<{
  projects: Project[]
}>()

const habits = ref<Habit[]>([])
const completions = ref<Record<string, Record<string, number | null>>>({})
const streaks = ref<Record<string, number>>({})
const today = ref(ymd(new Date()))
const busyIds = ref(new Set<string>())

const editOpen = ref(false)
const editingHabit = ref<Habit | null>(null)

const weekDays = computed(() => {
  const start = weekStart(new Date(`${today.value}T00:00:00`))
  return Array.from({ length: 7 }, (_, i) => ymd(addDays(start, i)))
})

const { error, load, run } = useLoader(async () => {
  today.value = ymd(new Date())
  const tStr = today.value
  const from = weekDays.value[0]!
  const [hs, rv] = await Promise.all([
    listHabits(),
    api<{ habits: { habit_id: string; streak: number }[] }>(
      `/review?from=${tStr}&to=${tStr}`,
    ).catch(() => ({ habits: [] })),
  ])
  const active = hs.filter((h) => h.active)
  const compRows = await Promise.all(
    active.map((h) =>
      api<{ date: string; value: number | null }[]>(
        `/habits/${h.id}/completions?from=${from}&to=${tStr}`,
      ).catch(() => []),
    ),
  )
  habits.value = active
  streaks.value = Object.fromEntries(rv.habits.map((h) => [h.habit_id, h.streak]))
  const map: Record<string, Record<string, number | null>> = {}
  active.forEach((h, i) => {
    const rec: Record<string, number | null> = {}
    for (const c of compRows[i] ?? []) rec[c.date] = c.value
    map[h.id] = rec
  })
  completions.value = map
})

onMounted(() => void load())
useLiveEvents(load)

const isGrid = computed(() => prefs.habitsDock === 'grid')
const ringSize = computed(() => (isGrid.value ? 44 : 52))
const ringRadius = computed(() => (ringSize.value - 5) / 2)
const ringCirc = computed(() => 2 * Math.PI * ringRadius.value)

function targetOf(h: Habit): number {
  return h.target_value && h.target_value > 0 ? h.target_value : 1
}

function todayVal(h: Habit): number {
  const rec = completions.value[h.id]
  if (!rec || !(today.value in rec)) return 0
  const v = rec[today.value]
  return v == null ? targetOf(h) : v
}

function dayDone(h: Habit, d: string): boolean {
  const rec = completions.value[h.id]
  if (!rec || !(d in rec)) return false
  const v = rec[d]
  return v == null ? true : v >= targetOf(h)
}

const ringItems = computed(() => {
  const circ = ringCirc.value
  return habits.value.map((h) => {
    const target = targetOf(h)
    const val = todayVal(h)
    const ratio = Number.isFinite(val) ? Math.max(0, Math.min(1, val / target)) : 0
    const isCounted = target > 1
    const st = streaks.value[h.id] ?? 0
    return {
      habit: h,
      icon: iconOf(h),
      ratio,
      isCounted,
      dashOffset: circ * (1 - ratio),
      stat: isCounted ? `${val}/${target}` : `${st}d`,
      days: weekDays.value.map((d) => ({ d, on: dayDone(h, d) })),
    }
  })
})

async function mutateHabit(id: string, fn: () => Promise<unknown>) {
  if (busyIds.value.has(id)) return
  busyIds.value = new Set(busyIds.value).add(id)
  try {
    await run(fn)
  } finally {
    const next = new Set(busyIds.value)
    next.delete(id)
    busyIds.value = next
  }
}

function onRingClick(h: Habit, e: MouseEvent) {
  if (e.shiftKey) {
    decrement(h)
    return
  }
  const tStr = ymd(new Date())
  const target = targetOf(h)
  const cur = todayVal(h)
  void mutateHabit(h.id, async () => {
    if (target > 1) {
      if (cur < target) await completeHabit(h.id, tStr, cur + 1)
      else await uncompleteHabit(h.id, tStr)
    } else if (cur >= 1) {
      await uncompleteHabit(h.id, tStr)
    } else {
      await completeHabit(h.id, tStr)
    }
  })
}

function decrement(h: Habit) {
  const tStr = ymd(new Date())
  const cur = todayVal(h)
  if (cur <= 0) return
  void mutateHabit(h.id, async () => {
    if (cur <= 1) await uncompleteHabit(h.id, tStr)
    else await completeHabit(h.id, tStr, cur - 1)
  })
}

function openNew() {
  editingHabit.value = null
  editOpen.value = true
}

function openEdit(h: Habit) {
  editingHabit.value = h
  editOpen.value = true
}

function handleSave({ id, body }: { id?: string; body: Partial<Habit> }) {
  void run(async () => {
    if (id) await updateHabit(id, body)
    else await createHabit(body)
    editOpen.value = false
  })
}
</script>

<template>
  <section class="rr-dock" :class="{ 'is-grid': isGrid }" :aria-label="$t('Gewohnheiten')">
    <div v-if="error" class="badge rr-err" role="alert">{{ error }}</div>

    <div :class="isGrid ? 'rr-grid' : 'rr-scroll-wrap'">
      <div :class="isGrid ? 'rr-grid-inner' : 'rr-strip'">
        <RitualRingItem
          v-for="item in ringItems"
          :key="item.habit.id"
          :habit="item.habit"
          :icon="item.icon"
          :ratio="item.ratio"
          :is-counted="item.isCounted"
          :stat="item.stat"
          :days="item.days"
          :ring-size="ringSize"
          :ring-radius="ringRadius"
          :ring-circ="ringCirc"
          :dash-offset="item.dashOffset"
          :busy="busyIds.has(item.habit.id)"
          @click="onRingClick(item.habit, $event)"
          @decrement="decrement(item.habit)"
          @edit="openEdit(item.habit)"
        />

        <button
          v-if="isGrid"
          type="button"
          class="rr-new"
          :aria-label="$t('Neue Gewohnheit')"
          @click="openNew"
        >
          <span class="rr-ring-new" :style="{ width: `${ringSize}px`, height: `${ringSize}px` }">
            <svg class="rr-ic" aria-hidden="true"><use href="#i-plus" /></svg>
          </span>
          <span class="rr-new-lbl">{{ $t('Neu') }}</span>
        </button>
      </div>
      <div v-if="!isGrid" class="rr-fade" aria-hidden="true"></div>
    </div>

    <template v-if="!isGrid">
      <div class="rr-pin-sep" aria-hidden="true"></div>
      <button type="button" class="rr-new" :aria-label="$t('Neue Gewohnheit')" @click="openNew">
        <span class="rr-ring-new" :style="{ width: `${ringSize}px`, height: `${ringSize}px` }">
          <svg class="rr-ic" aria-hidden="true"><use href="#i-plus" /></svg>
        </span>
        <span class="rr-new-lbl">{{ $t('Neu') }}</span>
      </button>
    </template>

    <HabitEditDialog
      :open="editOpen"
      :habit="editingHabit"
      :projects="projects"
      @close="editOpen = false"
      @save="handleSave"
    />
  </section>
</template>

<style scoped>
.rr-dock {
  flex-shrink: 0;
  border-top: 1px solid var(--br-subtle);
  background: var(--bg-0);
  padding: 18px 0 14px;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 14px;
}
.rr-dock.is-grid {
  display: block;
  padding: 18px 0 16px;
}
.rr-err {
  width: 100%;
  margin-bottom: 4px;
}
.rr-scroll-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
}
.rr-strip {
  display: flex;
  align-items: flex-start;
  gap: 20px;
  overflow-x: auto;
  padding: 2px 56px 10px 2px;
  scrollbar-width: thin;
  scrollbar-color: var(--br-default) transparent;
}
.rr-strip::-webkit-scrollbar {
  height: 3px;
}
.rr-strip::-webkit-scrollbar-thumb {
  background: var(--br-default);
  border-radius: 2px;
}
.rr-fade {
  pointer-events: none;
  position: absolute;
  top: 0;
  right: 0;
  bottom: 10px;
  width: 64px;
  background: linear-gradient(90deg, transparent, var(--bg-0) 92%);
}
.rr-pin-sep {
  width: 1px;
  height: 76px;
  background: var(--br-subtle);
  flex-shrink: 0;
  margin-top: 4px;
}
.rr-grid-inner {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(118px, 1fr));
  gap: 18px 20px;
  max-height: 224px;
  overflow-y: auto;
  padding-right: 4px;
}
.rr-new {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  padding: 2px 4px;
  color: inherit;
  cursor: pointer;
  min-width: 92px;
}
.rr-new:focus-visible {
  outline: 1px solid var(--a-blue-hi);
  outline-offset: 4px;
  border-radius: 8px;
}
.rr-ring-new {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1.5px dashed var(--br-strong);
  transition: transform 140ms ease;
}
.rr-new:hover .rr-ring-new {
  transform: scale(1.04);
  border-color: var(--tx-secondary);
}
.rr-ic {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
  color: var(--tx-secondary);
}
.rr-new:hover .rr-ic {
  color: var(--tx-primary);
}
.rr-new-lbl {
  font-size: 12px;
  font-weight: 500;
  color: var(--tx-secondary);
}
</style>
