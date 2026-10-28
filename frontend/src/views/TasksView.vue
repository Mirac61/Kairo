<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createResource,
  createTask,
  deleteTask,
  isOpen,
  listProjects,
  listResources,
  listTasks,
  restoreTask,
  taskAction,
  updateTask,
  type Project,
  type Resource,
  type Task,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { useShortcuts } from '@/composables/useShortcuts'
import { useTaskFilter } from '@/composables/useTaskFilter'
import { useTaskSelection } from '@/composables/useTaskSelection'
import { useUndo } from '@/composables/useUndo'
import { useFreeSlot } from '@/composables/useFreeSlot'
import { addDays, dayLabel, daysAgo, dm, hhmm, hm, ruleText, shortDay, ymd } from '@/lib/dates'
import { vDialog } from '@/lib/dialog'
import { t, tn } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'
import { parseQuickAdd, quickTokens, taskBody, type TokenKind } from '@/lib/quickAdd'
import GitHubIssues from '@/components/GitHubIssues.vue'
import SearchField from '@/components/SearchField.vue'
import SelectionBar from '@/components/SelectionBar.vue'
import TaskEditSheet from '@/components/TaskEditSheet.vue'

const STATUS_LABEL: Record<Task['status'], string> = {
  BACKLOG: t('Backlog'),
  PLANNED: t('Geplant'),
  IN_PROGRESS: t('Läuft'),
  PAUSED: t('Pausiert'),
  COMPLETED: t('Erledigt'),
  CANCELLED: t('Abgebrochen'),
}

const PRIO_LABEL: Record<string, string> = {
  URGENT: t('Dringend'),
  HIGH: t('Hoch'),
  MEDIUM: t('Mittel'),
  LOW: t('Niedrig'),
}

const EMPTY_MESSAGES: Record<string, string> = {
  alle: t('Alles erledigt – oder noch nichts angelegt.'),
  ueber: t('Nichts überfällig.'),
  heute: t('Heute ist nichts offen.'),
  woche: t('Die nächsten 7 Tage sind frei.'),
  spaeter: t('Dafür ist noch nichts eingeplant.'),
  ohne: t('Alles hat ein Datum.'),
  erledigt: t('Noch nichts erledigt.'),
}

const EXAMPLES = [
  t('Sport 30m @morgen'),
  t('Steuererklärung 2h @fr !hoch'),
  t('Mails beantworten 15m @heute'),
]

const tasks = ref<Task[]>([])
const projects = ref<Project[]>([])
const resources = ref<Resource[]>([])

const github = ref(false)
const viewMenu = ref(false)
const moreMenu = ref(false)
const openId = ref<string | null>(null)
const sel = ref<string | null>(null) // Fokus/Auswahl für Tastenkürzel

const quick = ref('')
const quickEl = ref<HTMLInputElement>()
const mirror = ref<HTMLElement>()

const details = ref(false)
const noteText = ref('')
const subsText = ref('')
const issueRef = ref('')

const route = useRoute()
const router = useRouter()
const { offer, setDone } = useUndo()

// Filter- und Gruppierungslogik über Composable
const {
  filter,
  projectId,
  search,
  view,
  SORTS,
  GROUPS,
  MAIN_FILTER_IDS,
  projectNameMap,
  todayString,
  dayOf,
  inScope,
  matches,
  counts,
  tabs,
  extra,
  rows,
  groups,
  flat,
  onTabKey,
} = useTaskFilter(tasks, projects)

const { error, load, run } = useLoader(async () => {
  ;[tasks.value, projects.value, resources.value] = await Promise.all([
    listTasks(),
    listProjects(),
    listResources(),
  ])
})

const selection = useTaskSelection(
  () => flat.value,
  run,
  () => ymd(addDays(new Date(), 1)),
)
const { picked, onClick: pickRow, onPointerDown, onPointerUp } = selection

// Wenn die aktuell ausgewählte Aufgabe verschwindet, rückt die Auswahl auf den Nachbarn
watch(flat, (now, before) => {
  if (!sel.value || now.some((t) => t.id === sel.value)) return
  const prevIndex = before.findIndex((t) => t.id === sel.value)
  const targetIndex = Math.min(prevIndex, now.length - 1)
  sel.value = now[targetIndex]?.id ?? null
})

const openTask = computed(() => tasks.value.find((t) => t.id === openId.value))

function prioLevel(task: Task): number {
  return ({ LOW: 1, MEDIUM: 2, HIGH: 3, URGENT: 3 } as const)[task.priority as 'LOW'] ?? 2
}

function overdue(task: Task): boolean {
  const d = dayOf(task)
  return isOpen(task) && d !== null && d < todayString()
}

// Rot nur, wenn hohe Priorität auch heute fällig oder überfällig ist
function urgent(task: Task): boolean {
  return prioLevel(task) === 3 && (matches(task, 'heute') || matches(task, 'ueber'))
}

function kidsOf(id: string): Task[] {
  return tasks.value.filter((t) => t.parent_task_id === id)
}

function progress(id: string): string {
  const children = kidsOf(id).filter((c) => c.status !== 'CANCELLED')
  if (!children.length) return ''
  const completed = children.filter((c) => c.status === 'COMPLETED').length
  return `${completed}/${children.length}`
}

function resOf(id: string): Resource[] {
  return resources.value.filter((r) => r.task_id === id)
}

const stats = computed(() => {
  const openTasks = tasks.value.filter(isOpen)
  const totalMinutes = openTasks.reduce((sum, task) => sum + task.estimated_minutes, 0)
  const overdueCount = openTasks.filter(overdue).length

  return [
    t('{n} offen', { n: openTasks.length }),
    t('{n} überfällig', { n: overdueCount }),
    totalMinutes ? t('{time} geschätzt', { time: hm(totalMinutes) }) : '',
  ]
    .filter(Boolean)
    .join(' · ')
})

// Quick-Add Hilfswerte
const activeProjects = computed(() => projects.value.filter((p) => p.status !== 'ARCHIVED'))
const quickParsed = computed(() => parseQuickAdd(quick.value, activeProjects.value, todayString()))
const tokens = computed(() => quickTokens(quick.value, activeProjects.value, todayString()))

const segs = computed(() => {
  let tokenIndex = 0
  return quick.value
    .split(/(\s+)/)
    .filter(Boolean)
    .map((word) => ({
      w: word,
      kind: /^\s/.test(word) ? '' : tokens.value[tokenIndex++]?.kind ?? '',
    }))
})

function syncScroll() {
  if (mirror.value && quickEl.value) {
    mirror.value.scrollLeft = quickEl.value.scrollLeft
  }
}
watch(quick, () => void nextTick(syncScroll))

// Auto-Vervollständigung der Schnelleingabe
const focused = ref(false)
const dismissed = ref(false)
const suggIdx = ref(0)
watch(quick, () => {
  dismissed.value = false
  suggIdx.value = 0
})

const DAY_WORDS = [
  'heute',
  'morgen',
  'übermorgen',
  'montag',
  'dienstag',
  'mittwoch',
  'donnerstag',
  'freitag',
  'samstag',
  'sonntag',
]
const RULE_WORDS = ['täglich', 'werktags', 'wöchentlich', 'monatlich']
const PRIO_WORDS = [
  ['dringend', 'URGENT'],
  ['hoch', 'HIGH'],
  ['mittel', 'MEDIUM'],
  ['niedrig', 'LOW'],
]
const SUGG_TITLE: Record<string, string> = {
  '#': t('Projekt'),
  '@': t('Fällig'),
  '!': t('Priorität'),
}

const sugg = computed(() => {
  const match = focused.value && !dismissed.value ? /(?:^|\s)([#@!])(\S*)$/.exec(quick.value) : null
  if (!match) return { title: '', items: [] }

  const prefix = match[1]!
  const query = match[2]!.toLowerCase()

  let list: { v: string; label: string; sub: string; color: string }[] = []

  if (prefix === '#') {
    list = activeProjects.value.map((p) => ({
      v: `#${p.name.replace(/\s+/g, '')}`,
      label: p.name,
      sub: t('{n} offen', {
        n: tasks.value.filter((x) => x.project_id === p.id && isOpen(x)).length,
      }),
      color: projectColor(p.id),
    }))
  } else if (prefix === '@') {
    list = [
      ...DAY_WORDS.map((w) => ({
        v: `@${w}`,
        label: `@${w}`,
        sub: dayLabel(parseQuickAdd(`@${w}`, [], todayString()).date!, todayString()),
        color: '',
      })),
      ...RULE_WORDS.map((w) => ({
        v: `@${w}`,
        label: `@${w}`,
        sub: t('Wiederholung'),
        color: '',
      })),
    ]
  } else {
    list = PRIO_WORDS.map(([w, k]) => ({
      v: `!${w}`,
      label: `!${w}`,
      sub: PRIO_LABEL[k!]!,
      color: '',
    }))
  }

  const filteredItems = list
    .filter((item) => item.v.slice(1).toLowerCase().startsWith(query))
    .slice(0, 8)

  const lastTokenKind = tokens.value.at(-1)?.kind
  const isComplete =
    filteredItems.length === 1 &&
    (filteredItems[0]!.v.toLowerCase() === `${prefix}${query}` ||
      (Boolean(lastTokenKind) && lastTokenKind !== 'bad'))

  return {
    title: SUGG_TITLE[prefix]!,
    items: isComplete ? [] : filteredItems,
  }
})

const suggSel = computed(() => Math.min(suggIdx.value, sugg.value.items.length - 1))

function acceptSuggestion(value: string) {
  quick.value = quick.value.replace(/[#@!]\S*$/, `${value} `)
  quickEl.value?.focus()
}

const DURATION_STEPS = [15, 30, 60, 120]

function setQuickToken(kind: TokenKind, val: string) {
  const rest = tokens.value.filter((x) => x.kind !== kind).map((x) => x.w).join(' ')
  quick.value = [rest, val].filter(Boolean).join(' ') + (val.length > 1 ? ' ' : '')
  quickEl.value?.focus()
}

const quickChips = computed(() => {
  const q = quickParsed.value
  const whenFormatted = [
    q.date && `${dayLabel(q.date, todayString())}${q.time ? ` ${q.time}` : ''}`,
    q.rule && `↻ ${ruleText(q.rule)}`,
  ]
    .filter(Boolean)
    .join(' · ')

  const nextDuration = q.minutes ? DURATION_STEPS[DURATION_STEPS.indexOf(q.minutes) + 1] : 30
  const durationValue = nextDuration ? (nextDuration >= 60 ? `${nextDuration / 60}h` : `${nextDuration}m`) : ''

  const nextPrioMap: Record<string, string> = { '': '!hoch', HIGH: '!mittel', MEDIUM: '!niedrig' }
  const prioValue = nextPrioMap[q.priority ?? ''] ?? ''

  return [
    {
      kind: 'project' as const,
      lead: '#',
      label: q.project?.name ?? t('Projekt'),
      on: Boolean(q.project),
      color: q.project ? projectColor(q.project.id) : '',
      val: '#',
    },
    {
      kind: 'date' as const,
      lead: '@',
      label: whenFormatted || t('Fällig'),
      on: Boolean(whenFormatted),
      color: '',
      val: '@',
    },
    {
      kind: 'dur' as const,
      lead: '~',
      label: q.minutes ? hm(q.minutes) : t('Dauer'),
      on: Boolean(q.minutes),
      color: '',
      val: durationValue,
    },
    {
      kind: 'prio' as const,
      lead: '!',
      label: q.priority ? PRIO_LABEL[q.priority]! : t('Priorität'),
      on: Boolean(q.priority),
      color: '',
      val: prioValue,
    },
  ]
})

function parseIssueReference(raw: string) {
  const s = raw.trim()
  if (!s) return undefined
  const match = /^([\w.-]+\/[\w.-]+)#(\d+)$/.exec(s)
  if (match) return { url: `https://github.com/${match[1]}/issues/${match[2]}`, label: s }
  return /^https?:\/\/\S+$/.test(s) ? { url: s, label: '' } : null
}

const slotDay = computed(() => {
  const q = quickParsed.value
  return q.title && !q.time ? (q.date ?? todayString()) : null
})

const currentMinuteOfDay = () => {
  const d = new Date()
  return d.getHours() * 60 + d.getMinutes()
}

const { hint: slotHint, slot, blocks, win } = useFreeSlot(
  () => slotDay.value,
  () => quickParsed.value.minutes,
  todayString,
  currentMinuteOfDay,
)

function computeSpan(startMin: number, endMin: number) {
  const { start, end } = win.value
  const leftBound = Math.max(startMin, start)
  const leftPercent = ((leftBound - start) / (end - start)) * 100
  const widthPercent = (Math.max(Math.min(endMin, end) - leftBound, 0) / (end - start)) * 100
  return { left: `${leftPercent}%`, width: `${widthPercent}%` }
}

const editSlotDay = computed(() => {
  const t = openTask.value
  return t && t.planned_date && !t.planned_start_at ? t.planned_date : null
})

const { hint: editSlotHint } = useFreeSlot(
  () => editSlotDay.value,
  () => openTask.value?.estimated_minutes ?? 0,
  todayString,
  currentMinuteOfDay,
)

function takeEditSlot(time: string) {
  if (openTask.value) {
    setTime(openTask.value, time)
  }
}

const defaultProject = () => (projectId.value === 'alle' ? null : projectId.value)

function addTask(planWithFreeSlot = false) {
  const q = quickParsed.value
  if (!q.title) return

  const issue = parseIssueReference(issueRef.value)
  if (issue === null) {
    error.value = t('GitHub-Issue als owner/repo#12 oder als Link angeben.')
    return
  }

  const plannedTime =
    planWithFreeSlot && slotHint.value?.time ? slotHint.value.time.padStart(5, '0') : null
  const note = noteText.value.trim()
  const subtaskTitles = subsText.value
    .split('\n')
    .map((x) => x.trim())
    .filter(Boolean)

  void run(async () => {
    const taskPayload = {
      project_id: defaultProject(),
      ...taskBody(plannedTime ? { ...q, date: slotDay.value, time: plannedTime } : q),
      title: q.title,
      ...(note ? { description: note } : {}),
    }

    const createdTask = await createTask(taskPayload)
    for (const subTitle of subtaskTitles) {
      await createTask({
        title: subTitle,
        parent_task_id: createdTask.id,
        project_id: createdTask.project_id,
      })
    }
    if (issue) {
      await createResource({
        task_id: createdTask.id,
        type: 'URL',
        target: issue.url,
        label: issue.label,
      })
    }

    quick.value = ''
    noteText.value = ''
    subsText.value = ''
    issueRef.value = ''
    details.value = false

    handleTasksCreated([createdTask], t('„{title}“ angelegt', { title: createdTask.title }))
  })
}

function onPaste(event: ClipboardEvent) {
  const lines = (event.clipboardData?.getData('text') ?? '')
    .split(/\r?\n/)
    .map((x) => x.trim())
    .filter(Boolean)

  if (lines.length < 2) return
  event.preventDefault()

  const quickItems = lines
    .map((line) => parseQuickAdd(line, activeProjects.value, todayString()))
    .filter((q) => q.title)

  void run(async () => {
    const createdList: Task[] = []
    for (const item of quickItems) {
      const task = await createTask({
        project_id: defaultProject(),
        ...taskBody(item),
        title: item.title,
      })
      createdList.push(task)
    }
    if (createdList.length) {
      handleTasksCreated(
        createdList,
        tn(createdList.length, '{n} Aufgabe angelegt', '{n} Aufgaben angelegt'),
      )
    }
  })
}

const freshCreatedIds = ref<string[]>([])

function handleTasksCreated(list: Task[], notificationText: string) {
  const first = list[0]!
  freshCreatedIds.value = list.map((task) => task.id)
  setTimeout(() => {
    freshCreatedIds.value = []
  }, 1800)

  const isFirstVisible = inScope(first) && matches(first, filter.value)

  offer(
    notificationText,
    () =>
      run(async () => {
        for (const task of list) {
          await deleteTask(task.id, true)
        }
      }),
    isFirstVisible ? undefined : () => focusTask(first.id),
  )
}

function onQuickKeyDown(event: KeyboardEvent) {
  if (event.isComposing) return

  const { items } = sugg.value
  if (items.length && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
    event.preventDefault()
    const direction = event.key === 'ArrowDown' ? 1 : items.length - 1
    suggIdx.value = (suggSel.value + direction) % items.length
  } else if (
    items.length &&
    ((event.key === 'Tab' && !event.shiftKey) ||
      (event.key === 'Enter' && !event.metaKey && !event.ctrlKey))
  ) {
    event.preventDefault()
    acceptSuggestion(items[suggSel.value]!.v)
  } else if (items.length && event.key === 'Escape') {
    dismissed.value = true
  } else if (event.key === 'Enter') {
    event.preventDefault()
    addTask(event.metaKey || event.ctrlKey)
  } else if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'd') {
    event.preventDefault()
    details.value = !details.value
  } else if (event.key === 'Escape') {
    if (quick.value) {
      quick.value = ''
    } else {
      quickEl.value?.blur()
    }
  }
}

function toggleTaskStatus(task: Task) {
  if (task.status === 'COMPLETED') {
    return run(() => updateTask(task.id, { status: task.planned_date ? 'PLANNED' : 'BACKLOG' }))
  }
  return setDone(task, 'COMPLETED', run)
}

function trashTask(task: Task) {
  void run(async () => {
    await deleteTask(task.id)
    offer(t('„{title}“ gelöscht', { title: task.title }), () => run(() => restoreTask(task.id)))
    if (openId.value === task.id) {
      openId.value = null
    }
  })
}

function formatTaskWhen(task: Task): { text: string; tip: string } {
  const d = dayOf(task)
  if (!d) return { text: '', tip: '' }

  if (overdue(task)) {
    return { text: daysAgo(d, todayString()), tip: `${t('Fällig')} ${shortDay(d)}` }
  }
  if (!task.planned_date) {
    return { text: `${t('fällig')} ${dm(d)}`, tip: '' }
  }

  const timeStr = task.planned_start_at ? hhmm(new Date(task.planned_start_at)) : ''
  return {
    text: `${dayLabel(d, todayString())} ${timeStr}`.trim(),
    tip: '',
  }
}

function setDate(task: Task, date: string) {
  const time = task.planned_start_at ? hhmm(new Date(task.planned_start_at)) : ''
  void run(() =>
    updateTask(task.id, {
      planned_date: date,
      planned_start_at: date && time ? new Date(`${date}T${time}`).toISOString() : '',
    }),
  )
}

function setTime(task: Task, time: string) {
  if (!task.planned_date) return
  void run(() =>
    updateTask(task.id, {
      planned_start_at: time ? new Date(`${task.planned_date}T${time}`).toISOString() : '',
    }),
  )
}

function setEstimate(task: Task, minutes: string) {
  void run(() => updateTask(task.id, { estimated_minutes: Number(minutes) || 0 }))
}

function setField(
  task: Task,
  field: 'title' | 'description' | 'priority' | 'project_id' | 'status' | 'recurrence_rule',
  value: string,
) {
  if (field === 'title' && !value.trim()) return
  if (field === 'status' && value === 'CANCELLED') return setDone(task, 'CANCELLED', run)
  void run(() => updateTask(task.id, { [field]: value }))
}

function setDue(task: Task, day: string) {
  void run(() =>
    updateTask(task.id, {
      due_at: day ? new Date(`${day}T23:59:00`).toISOString() : '',
    }),
  )
}

function moveSelection(direction: number) {
  const list = flat.value
  if (!list.length) return

  const currentIndex = list.findIndex((t) => t.id === sel.value)
  const nextIndex =
    currentIndex < 0
      ? direction > 0
        ? 0
        : list.length - 1
      : Math.min(Math.max(currentIndex + direction, 0), list.length - 1)

  sel.value = list[nextIndex]!.id
  if (openId.value) {
    openId.value = sel.value
  }

  void nextTick(() => {
    document.querySelector(`[data-task="${sel.value}"]`)?.scrollIntoView({ block: 'nearest' })
  })
}

useShortcuts((event) => {
  const targetEl = event.target as HTMLElement
  const currentTask = flat.value.find((x) => x.id === sel.value)

  if (event.key === 'n') {
    event.preventDefault()
    quickEl.value?.focus()
  } else if (event.key === 'j') {
    moveSelection(1)
  } else if (event.key === 'k') {
    moveSelection(-1)
  } else if (event.key === 'Enter' && currentTask && !targetEl.closest('button, a')) {
    openId.value = openId.value === currentTask.id ? null : currentTask.id
  } else if (event.key === 'x' && currentTask) {
    void toggleTaskStatus(currentTask)
  } else if (event.key === 's' && currentTask && isOpen(currentTask)) {
    void run(() =>
      taskAction(currentTask.id, currentTask.status === 'IN_PROGRESS' ? 'pause' : 'start'),
    )
  } else if (event.key === 't' && currentTask && isOpen(currentTask)) {
    setDate(currentTask, todayString())
  } else if (/^[1-7]$/.test(event.key)) {
    filter.value = MAIN_FILTER_IDS[Number(event.key) - 1]!
  }
})

function focusTask(id: unknown) {
  const target = tasks.value.find((x) => x.id === id)
  if (!target) return

  filter.value =
    target.status === 'COMPLETED'
      ? 'erledigt'
      : target.status === 'CANCELLED'
        ? 'abgebrochen'
        : 'alle'
  projectId.value = 'alle'
  search.value = ''
  openId.value = target.id
  sel.value = target.id

  void nextTick(() => {
    document.querySelector(`[data-task="${target.id}"]`)?.scrollIntoView({ block: 'center' })
  })
}

function focusNewFromRoute() {
  if (!route.query.new) return
  if (typeof route.query.text === 'string') {
    quick.value = route.query.text
  }
  void nextTick(() => quickEl.value?.focus())
  void router.replace({ query: {} })
}
watch(() => route.query.new, focusNewFromRoute)

function filterFromRoute() {
  const queryFilter = route.query.filter
  if (typeof queryFilter === 'string') {
    filter.value = queryFilter
  }
}
watch(() => route.query.filter, filterFromRoute, { immediate: true })

onMounted(() => {
  focusNewFromRoute()
  void load().then(() => {
    if (route.query.task) {
      focusTask(route.query.task)
    }
  })
})

useLiveEvents(load)
</script>

<template>
  <div class="view-inner" :class="{ 'tk-split': openTask }">
    <div class="tk-left">
      <div class="v-head v-head-row tk-head">
        <h1 class="v-title">{{ $t('Aufgaben') }}</h1>
        <span class="spacer"></span>
        <p class="tk-stats">{{ stats }}</p>
      </div>

      <div v-if="error" class="badge" role="alert">{{ error }}</div>

      <!-- Quick Add Formular -->
      <form class="tk-comp" :class="{ focus: focused }" @submit.prevent="addTask()">
        <div class="tk-line">
          <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>
          <div class="tk-field">
            <div ref="mirror" class="tk-mirror" aria-hidden="true">
              <span
                v-for="(g, i) in segs"
                :key="i"
                :class="g.kind && `k-${g.kind}`"
                :style="
                  g.kind === 'project' && quickParsed.project
                    ? { '--tc': projectColor(quickParsed.project.id) }
                    : undefined
                "
              >{{ g.w }}</span>
            </div>
            <input
              ref="quickEl"
              v-model="quick"
              type="text"
              role="combobox"
              aria-autocomplete="list"
              :aria-expanded="sugg.items.length > 0"
              aria-controls="tk-sugg"
              :aria-activedescendant="sugg.items.length ? `tk-sugg-${suggSel}` : undefined"
              :placeholder="$t('Neue Aufgabe … z. B. Übungsblatt 5 #Uni 1h @fr !hoch')"
              :aria-label="$t('Neue Aufgabe hinzufügen')"
              aria-keyshortcuts="n"
              spellcheck="false"
              autocomplete="off"
              @keydown="onQuickKeyDown"
              @paste="onPaste"
              @focus="focused = true"
              @blur="focused = false"
              @scroll="syncScroll"
              @keyup="syncScroll"
              @click="syncScroll"
            />
          </div>
        </div>

        <ul
          v-if="sugg.items.length"
          id="tk-sugg"
          class="tk-sugg"
          role="listbox"
          :aria-label="sugg.title"
        >
          <li
            v-for="(item, i) in sugg.items"
            :id="`tk-sugg-${i}`"
            :key="item.v"
            role="option"
            :aria-selected="i === suggSel"
            @mousedown.prevent="acceptSuggestion(item.v)"
          >
            <span
              class="cdot"
              :style="{ background: item.color || 'var(--tx-muted)' }"
            ></span>
            {{ item.label }}
            <span class="sub">{{ item.sub }}</span>
          </li>
          <li class="tk-sugg-help" aria-hidden="true">
            {{ $t('↑↓ wählen · Tab übernehmen') }}
          </li>
        </ul>

        <div class="tk-chips">
          <button
            v-for="chip in quickChips"
            :key="chip.kind"
            type="button"
            class="tk-chip"
            :class="{ on: chip.on }"
            :style="chip.color ? { '--cc': chip.color } : undefined"
            @mousedown.prevent
            @click="setQuickToken(chip.kind, chip.val)"
          >
            <b aria-hidden="true">{{ chip.lead }}</b>{{ chip.label }}
          </button>
          <button
            type="button"
            class="tk-chip"
            :aria-expanded="details"
            aria-keyshortcuts="Meta+D"
            @mousedown.prevent
            @click="details = !details"
          >
            {{ $t('Details') }}<kbd class="key" aria-hidden="true">⌘D</kbd>
          </button>
        </div>

        <div v-if="details" class="tk-details">
          <label class="field">
            <span>{{ $t('Notiz') }}</span>
            <textarea
              v-model="noteText"
              class="input"
              rows="2"
              :placeholder="$t('Kontext, Link, Gedanke …')"
            ></textarea>
          </label>
          <label class="field">
            <span>{{ $t('Teilaufgaben, eine pro Zeile') }}</span>
            <textarea v-model="subsText" class="input" rows="2"></textarea>
          </label>
          <label class="field">
            <span>{{ $t('GitHub-Issue verknüpfen') }}</span>
            <input v-model="issueRef" class="input" placeholder="owner/repo#12" />
          </label>
        </div>

        <div class="tk-foot">
          <template v-if="quickParsed.title">
            <span><kbd class="key">↵</kbd>{{ $t('Anlegen') }}</span>
            <button
              v-if="slotHint?.time"
              type="button"
              class="tk-plan"
              @mousedown.prevent
              @click="addTask(true)"
            >
              <kbd class="key">⌘↵</kbd>{{ slotHint.text }}
            </button>
            <span v-else-if="slotHint">{{ slotHint.text }}</span>
            <span v-if="slotDay" class="tk-day" aria-hidden="true">
              {{ Math.floor(win.start / 60) }}
              <span class="tk-day-bar">
                <i v-for="b in blocks" :key="b.key" :style="computeSpan(b.start, b.end)"></i>
                <i
                  v-if="slot"
                  class="slot"
                  :style="computeSpan(slot.start, slot.start + (quickParsed.minutes || 30))"
                ></i>
              </span>
              {{ Math.ceil(win.end / 60) }}
            </span>
          </template>
          <span v-else class="tk-tip">
            {{ $t('Kürzel: # Projekt · @ Fällig · ! Priorität · 30m / 1h Dauer · mehrere Zeilen einfügen legt mehrere Aufgaben an') }}
          </span>
        </div>
      </form>

      <!-- Toolbar & Filter -->
      <div class="tk-toolbar">
        <div class="filterbar tk-bar">
          <div class="seg tk-tabs" role="tablist" :aria-label="$t('Filter')" @keydown="onTabKey">
            <button
              v-for="f in tabs"
              :id="`tk-tab-${f.id}`"
              :key="f.id"
              type="button"
              role="tab"
              :aria-selected="filter === f.id"
              :tabindex="filter === f.id ? 0 : -1"
              @click="filter = f.id"
            >
              {{ f.label }}
              <span v-if="counts[f.id]" class="count" :class="{ od: f.id === 'ueber' }">
                {{ counts[f.id] }}
              </span>
            </button>
          </div>
          <span class="spacer"></span>
          <SearchField v-model="search" :label="$t('Aufgaben durchsuchen')" />
        </div>

        <div class="tk-opts">
          <span class="spacer"></span>
          <select
            class="input"
            :value="extra.some((f) => f.id === filter) ? filter : ''"
            :aria-label="$t('Weitere Filter')"
            @change="(e) => filter = (e.target as HTMLSelectElement).value || 'alle'"
          >
            <option value="">{{ $t('Weitere Filter') }}</option>
            <option v-for="f in extra" :key="f.id" :value="f.id">
              {{ f.label }} ({{ counts[f.id] }})
            </option>
          </select>

          <div class="tk-desktop-opts">
            <div class="menu-wrap" @keydown.esc="viewMenu = false">
              <button
                class="btn"
                type="button"
                :aria-expanded="viewMenu"
                @click="viewMenu = !viewMenu"
              >
                {{ $t('Ansicht') }}
                <template v-if="projectId !== 'alle'">
                  · {{ projects.find((p) => p.id === projectId)?.name }}
                </template>
                <svg class="ic tk-chev" aria-hidden="true"><use href="#i-chev" /></svg>
              </button>

              <template v-if="viewMenu">
                <div class="menu-scrim" @click="viewMenu = false"></div>
                <div class="menu">
                  <label>{{ $t('Projekt') }}
                    <select v-model="projectId" class="input">
                      <option value="alle">{{ $t('Alle') }}</option>
                      <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                    </select>
                  </label>
                  <label>{{ $t('Gruppe') }}
                    <select v-model="view.group" class="input">
                      <option v-for="[val, lbl] in GROUPS" :key="val" :value="val">{{ lbl }}</option>
                    </select>
                  </label>
                  <label>{{ $t('Sortierung') }}
                    <select v-model="view.sort" class="input">
                      <option v-for="[val, lbl] in SORTS" :key="val" :value="val">{{ lbl }}</option>
                    </select>
                  </label>
                </div>
              </template>
            </div>

            <button class="btn" type="button" @click="github = true">
              {{ $t('GitHub-Issues') }}
            </button>
            <button
              class="btn tk-pick"
              type="button"
              :aria-pressed="selection.mode.value"
              @click="selection.mode.value = !selection.mode.value"
            >
              {{ $t('Auswählen') }}
            </button>
          </div>

          <!-- Mobil-Menü -->
          <div class="tk-mobile-more menu-wrap" @keydown.esc="moreMenu = false">
            <button
              class="btn tk-more-btn"
              type="button"
              :aria-expanded="moreMenu"
              :title="$t('Weitere Optionen')"
              :aria-label="$t('Weitere Optionen')"
              @click="moreMenu = !moreMenu"
            >
              <svg class="ic" viewBox="0 0 24 24" aria-hidden="true" width="16" height="16">
                <circle cx="12" cy="12" r="1.5" fill="currentColor"/>
                <circle cx="6" cy="12" r="1.5" fill="currentColor"/>
                <circle cx="18" cy="12" r="1.5" fill="currentColor"/>
              </svg>
            </button>

            <template v-if="moreMenu">
              <div class="menu-scrim" @click="moreMenu = false"></div>
              <div class="menu tk-more-menu">
                <label>{{ $t('Projekt') }}
                  <select v-model="projectId" class="input">
                    <option value="alle">{{ $t('Alle') }}</option>
                    <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                  </select>
                </label>
                <label>{{ $t('Gruppe') }}
                  <select v-model="view.group" class="input">
                    <option v-for="[val, lbl] in GROUPS" :key="val" :value="val">{{ lbl }}</option>
                  </select>
                </label>
                <label>{{ $t('Sortierung') }}
                  <select v-model="view.sort" class="input">
                    <option v-for="[val, lbl] in SORTS" :key="val" :value="val">{{ lbl }}</option>
                  </select>
                </label>
                <div class="tk-more-actions">
                  <button
                    class="btn btn-secondary"
                    type="button"
                    @click="github = true; moreMenu = false"
                  >
                    {{ $t('GitHub-Issues') }}
                  </button>
                  <button
                    class="btn tk-pick"
                    type="button"
                    :class="{ 'btn-primary': selection.mode.value }"
                    :aria-pressed="selection.mode.value"
                    @click="selection.mode.value = !selection.mode.value; moreMenu = false"
                  >
                    {{ $t('Auswählen') }}
                  </button>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>

      <!-- Aufgabenliste -->
      <div class="tasks-layout" :class="{ 'with-aside': openTask }">
        <div class="tasks-main" role="tabpanel" :aria-labelledby="`tk-tab-${filter}`">
          <div v-if="!rows.length" class="tk-empty">
            <span class="tk-empty-ic" aria-hidden="true">
              <svg class="ic"><use href="#i-check" /></svg>
            </span>
            <p class="tk-empty-h">
              {{
                search || projectId !== 'alle'
                  ? $t('Keine Aufgaben.')
                  : EMPTY_MESSAGES[filter] ?? $t('Keine Aufgaben.')
              }}
            </p>
            <template v-if="(filter === 'alle' || filter === 'ohne') && !search">
              <p class="tk-ex-h">{{ $t('Probier eine Zeile aus – ein Klick füllt sie ins Feld.') }}</p>
              <div class="tk-ex">
                <button
                  v-for="example in EXAMPLES"
                  :key="example"
                  type="button"
                  @click="quick = example; quickEl?.focus()"
                >
                  <span
                    v-for="(word, i) in example.split(' ')"
                    :key="i"
                    :class="{ tok: /^[#@!]|^\d+[mh]$/.test(word) }"
                  >{{ word }}</span>
                </button>
              </div>
            </template>
          </div>

          <div v-if="rows.length" class="tk-cols" aria-hidden="true">
            <span class="c-t">{{ $t('Aufgabe') }}</span>
            <span class="c-p">{{ $t('Projekt') }}</span>
            <span class="c-d">{{ $t('Dauer') }}</span>
            <span class="c-i">{{ $t('Prio') }}</span>
            <span class="c-f">{{ $t('Fällig') }}</span>
          </div>

          <div v-for="group in groups" :key="group.key" class="tgroup">
            <span
              v-if="group.title"
              class="lbl"
              :class="{ od: view.group === 'due' && group.key === '0' }"
            >
              {{ group.title }}
              <span class="count">{{ group.rows.length }}</span>
              <span v-if="group.minutes" class="g-sum">
                {{ $t('{time} geschätzt', { time: hm(group.minutes) }) }}
              </span>
            </span>

            <div class="tasklist">
              <template v-for="task in group.rows" :key="task.id">
                <div
                  class="task-row"
                  :class="{
                    done: !isOpen(task),
                    selected: sel === task.id,
                    picked: picked.includes(task.id),
                    fresh: freshCreatedIds.includes(task.id),
                  }"
                  :data-task="task.id"
                  @click.capture="pickRow($event, task.id)"
                  @mousedown.shift.prevent
                  @pointerdown="onPointerDown($event, task.id)"
                  @pointerup="onPointerUp"
                  @pointercancel="onPointerUp"
                >
                  <button
                    type="button"
                    class="cb sq"
                    role="checkbox"
                    :aria-checked="task.status === 'COMPLETED'"
                    :aria-label="$t('{title} erledigt', { title: task.title })"
                    :disabled="task.status === 'CANCELLED'"
                    @click="toggleTaskStatus(task)"
                  ></button>

                  <span
                    v-if="task.project_id"
                    class="pdot"
                    :style="{ '--pdot-c': projectColor(task.project_id) }"
                    :title="projectNameMap.get(task.project_id)"
                    :aria-label="projectNameMap.get(task.project_id)"
                  ></span>

                  <button
                    type="button"
                    class="t row-title"
                    :aria-expanded="openId === task.id"
                    @click="openId = openId === task.id ? null : task.id; sel = task.id;"
                  >
                    {{ task.title }}
                  </button>

                  <span
                    v-if="task.recurrence_rule"
                    class="rec-i"
                    role="img"
                    :aria-label="$t('Wiederholt sich: {rule}', { rule: ruleText(task.recurrence_rule) })"
                    :data-tip="ruleText(task.recurrence_rule)"
                  >↻</span>

                  <span
                    v-if="progress(task.id)"
                    class="badge mono"
                    role="img"
                    :aria-label="`${$t('Teilaufgaben erledigt')}: ${progress(task.id)}`"
                  >
                    {{ progress(task.id) }}
                  </span>

                  <span
                    v-if="task.status === 'IN_PROGRESS' || task.status === 'PAUSED'"
                    class="status-l"
                    :class="{ run: task.status === 'IN_PROGRESS' }"
                  >
                    <span class="cdot"></span>{{ STATUS_LABEL[task.status] }}
                  </span>

                  <span class="pcol">
                    <span class="chip" :style="{ '--chip-c': projectColor(task.project_id) }">
                      <template v-if="task.project_id">
                        <span class="cdot"></span>{{ projectNameMap.get(task.project_id) }}
                      </template>
                    </span>
                  </span>

                  <span class="dur">
                    {{ task.estimated_minutes ? hm(task.estimated_minutes) : '' }}
                  </span>

                  <span
                    class="prio-i"
                    :class="{ high: urgent(task) }"
                    role="img"
                    :aria-label="`${$t('Priorität')}: ${PRIO_LABEL[task.priority] ?? task.priority}`"
                  >
                    <i v-for="n in 3" :key="n" :class="{ on: n <= prioLevel(task) }"></i>
                  </span>

                  <span
                    class="due"
                    :class="{ od: overdue(task) }"
                    :data-tip="formatTaskWhen(task).tip || undefined"
                  >
                    {{ formatTaskWhen(task).text }}
                  </span>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>

      <SelectionBar
        v-if="picked.length || selection.mode.value"
        :count="picked.length"
        :projects="projects.filter((p) => p.status !== 'ARCHIVED')"
        @tomorrow="selection.toTomorrow()"
        @project="selection.toProject"
        @trash="selection.trash"
        @clear="selection.clear"
      />

      <div v-if="!rows.length" class="keys" :aria-label="$t('Tastenkürzel')">
        <span><kbd class="key">n</kbd>{{ $t('Neu') }}</span>
        <span><kbd class="key">j</kbd><kbd class="key">k</kbd>{{ $t('Wählen') }}</span>
        <span><kbd class="key">x</kbd>{{ $t('Erledigt') }}</span>
        <span><kbd class="key">Enter</kbd>{{ $t('Öffnen') }}</span>
        <span><kbd class="key">s</kbd>{{ $t('Start/Pause') }}</span>
        <span><kbd class="key">t</kbd>{{ $t('Heute') }}</span>
        <span><kbd class="key">1</kbd>–<kbd class="key">7</kbd>{{ $t('Reiter') }}</span>
      </div>
    </div>

    <!-- Modale Detailansicht als ausgelagerte Komponente -->
    <TaskEditSheet
      v-if="openTask"
      :task="openTask"
      :projects="projects"
      :resources="resOf(openTask.id)"
      :sub-tasks="kidsOf(openTask.id)"
      :progress-text="progress(openTask.id)"
      :edit-slot-hint="editSlotHint"
      :today-str="todayString()"
      @close="openId = null"
      @update-field="(field, val) => setField(openTask!, field, val)"
      @set-date="(d) => setDate(openTask!, d)"
      @set-time="(t) => setTime(openTask!, t)"
      @set-due="(d) => setDue(openTask!, d)"
      @set-estimate="(m) => setEstimate(openTask!, m)"
      @toggle-subtask="toggleTaskStatus"
      @add-subtask="(title) => {
        void run(async () => {
          await createTask({ title, parent_task_id: openTask!.id, project_id: openTask!.project_id })
        })
      }"
      @trash="trashTask(openTask)"
      @take-edit-slot="takeEditSlot"
      @run="run"
      @reload="load"
    />

    <!-- GitHub-Dialog -->
    <div
      v-if="github"
      v-dialog="() => github = false"
      class="overlay open"
      @mousedown.self="github = false"
    >
      <div class="dialog gh-dlg" role="dialog" :aria-label="$t('GitHub-Issues')">
        <div class="dlg-head">
          <h3>{{ $t('GitHub-Issues') }}</h3>
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Schließen')"
            @click="github = false"
          >
            <svg class="ic"><use href="#i-x" /></svg>
          </button>
        </div>
        <GitHubIssues
          class="dlg-body"
          :projects="projects"
          @open="(id) => { github = false; void load().then(() => focusTask(id)) }"
          @changed="load"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.gh-dlg {
  width: 680px;
}
.keys {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 20px;
  justify-content: center;
  margin-top: 24px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.keys span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.keys .key {
  min-width: 20px;
  text-align: center;
  color: var(--tx-muted);
}
.rec-i {
  color: var(--tx-muted);
  font-size: 13px;
  line-height: 1;
}
.task-row.picked {
  background: var(--bg-selected);
}
.task-row.selected {
  background: var(--bg-2);
  box-shadow: inset 0 0 0 1px var(--br-default);
}
.task-row:hover:not(.selected) {
  background: var(--bg-hover);
}
.row-title {
  text-align: left;
  background: none;
  border: 0;
  color: inherit;
  cursor: pointer;
}

.tasks-layout {
  margin-top: 16px;
}

@media (min-width: 1001px) {
  .view-inner.tk-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 380px;
    max-width: none;
    padding: 0;
  }
  .tk-split .tk-left {
    min-width: 0;
    padding: 32px 40px 64px 32px;
  }
  .tk-cols {
    position: sticky;
    top: 56px;
    z-index: 2;
  }
}

.tk-tabs {
  flex-wrap: wrap;
}
.tk-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 8px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--br-subtle);
}
.tk-toolbar .tk-bar {
  flex: 1 1 auto;
  margin-bottom: 0;
}
.tk-opts {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  margin-left: auto;
}

@media (max-width: 900px) {
  .tk-toolbar {
    display: block;
  }
  .tk-opts {
    margin-top: 8px;
  }
}

.tk-stats {
  margin: 0 4px 3px 0;
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.tk-pick {
  height: 28px;
  padding: 0 12px;
  border: 0;
  border-radius: 999px;
  background: none;
  box-shadow: none;
  color: var(--tx-secondary);
  font-size: 12px;
}
.tk-pick:hover,
.tk-pick[aria-pressed="true"] {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.tk-tip {
  display: none;
}
.tk-opts select {
  height: 28px;
  width: auto;
  padding: 0 26px 0 10px;
  font-size: 12px;
  border-radius: 999px;
}
.tk-by {
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.spacer {
  flex: 1;
}
.tk-tabs .count.od {
  color: var(--a-red);
}
.tk-opts .menu select {
  width: 150px;
}
.tk-chev {
  width: 12px;
  height: 12px;
  margin-left: 4px;
  transform: rotate(90deg);
}
.tk-opts .btn,
.tk-opts select {
  height: 28px;
  padding: 0 12px;
  font-size: 12px;
  border-radius: 999px;
  border: 0;
  background-color: transparent;
  box-shadow: none;
  color: var(--tx-secondary);
}
.tk-opts select {
  padding-right: 26px;
  cursor: pointer;
}
.tk-opts .btn:hover,
.tk-opts select:hover {
  background-color: var(--bg-2);
  color: var(--tx-primary);
}

.tk-comp {
  position: relative;
  margin-bottom: 28px;
  padding: 16px 18px 14px;
  border-radius: 14px;
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  transition: box-shadow var(--dur);
}
.tk-comp.focus .tk-tip {
  display: block;
}
.tk-comp.focus .tk-foot:has(> .tk-tip) {
  margin-top: 10px;
}
.tk-comp:not(.focus) .tk-foot:has(> .tk-tip) {
  display: none;
}
.tk-comp.focus {
  box-shadow: 0 0 0 1px var(--a-focus), 0 0 0 4px var(--ring);
}
.tk-line {
  display: flex;
  align-items: center;
  gap: 10px;
}
.tk-line .ic {
  color: var(--tx-muted);
  flex: none;
}
.tk-field {
  position: relative;
  flex: 1;
  min-width: 0;
}
.tk-field input,
.tk-mirror {
  font: 400 15px/24px var(--font-ui);
  letter-spacing: 0;
  padding: 0;
  border: 0;
  margin: 0;
}
.tk-field input {
  position: relative;
  display: block;
  width: 100%;
  height: 24px;
  background: transparent;
  color: transparent;
  caret-color: var(--tx-primary);
  outline: none;
}
.tk-field input::placeholder {
  color: var(--tx-muted);
}
.tk-field input::selection {
  background: var(--bg-selected);
  color: transparent;
}
.tk-mirror {
  position: absolute;
  inset: 0;
  white-space: pre;
  overflow: hidden;
  color: var(--tx-primary);
  pointer-events: none;
}
.tk-mirror span {
  --tc: var(--tx-primary);
  color: var(--tc);
  border-radius: 4px;
}
.tk-mirror [class^="k-"] {
  background: color-mix(in srgb, var(--tc) 16%, transparent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--tc) 16%, transparent);
}
.tk-mirror .k-date {
  --tc: var(--a-blue);
}
.tk-mirror .k-dur {
  --tc: var(--p-green);
}
.tk-mirror .k-prio {
  --tc: var(--p-orange);
}
.tk-mirror .k-bad {
  --tc: var(--a-red);
  background: none;
  box-shadow: none;
  text-decoration: underline wavy;
  text-underline-offset: 4px;
}
.tk-comp.focus .tk-mirror .k-bad:last-child {
  --tc: var(--tx-secondary);
  text-decoration: none;
}
.tk-sugg {
  position: absolute;
  z-index: 30;
  top: 48px;
  left: 42px;
  min-width: 260px;
  margin: 0;
  padding: 4px;
  list-style: none;
  border-radius: 10px;
  background: var(--bg-2);
  box-shadow: 0 0 0 1px var(--br-default), var(--shadow-pop);
}
.tk-sugg li[role="option"] {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 6px;
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
}
.tk-sugg li[aria-selected="true"] {
  background: var(--bg-hover);
}
.tk-sugg .cdot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex: none;
}
.tk-sugg .sub {
  margin-left: auto;
  padding-left: 16px;
  color: var(--tx-muted);
}
.tk-sugg-help {
  padding: 6px 10px 4px;
  font: 400 11px/1 var(--font-ui);
  color: var(--tx-muted);
}
.tk-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 10px 0 0 26px;
}
.tk-chip {
  --cc: var(--tx-secondary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 24px;
  padding: 0 9px;
  border: 1px dashed var(--br-default);
  border-radius: 999px;
  background: none;
  color: var(--tx-muted);
  font: 400 12px/1 var(--font-ui);
  cursor: pointer;
}
.tk-chip b {
  font-weight: 500;
  opacity: .7;
}
.tk-chip:hover {
  color: var(--tx-primary);
}
.tk-chip.on {
  border-style: solid;
  border-color: color-mix(in srgb, var(--cc) 35%, transparent);
  background: color-mix(in srgb, var(--cc) 12%, transparent);
  color: var(--cc);
}
.tk-chip .key {
  padding: 1px 4px;
  font-size: 10px;
}
.tk-details {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 10px;
  margin: 12px 0 0 26px;
}
.tk-details .field {
  display: grid;
  gap: 4px;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-secondary);
}
.tk-foot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin: 12px 0 0 26px;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.tk-foot > span,
.tk-plan {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tk-plan {
  padding: 0;
  border: 0;
  background: none;
  color: var(--tx-secondary);
  font: inherit;
  cursor: pointer;
}
.tk-plan:hover {
  color: var(--tx-primary);
}
.tk-foot .key {
  padding: 2px 5px;
  font-size: 11px;
}
.tk-day {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}
.tk-day-bar {
  position: relative;
  display: block;
  width: 140px;
  height: 6px;
  border-radius: 3px;
  background: var(--bg-2);
  overflow: hidden;
}
.tk-day-bar i {
  position: absolute;
  top: 0;
  bottom: 0;
  background: var(--br-strong);
}
.tk-day-bar i.slot {
  background: var(--a-blue);
}
.tk-sum {
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
  font-variant-numeric: tabular-nums;
}
.tk-empty {
  display: grid;
  gap: 8px;
  justify-items: center;
  text-align: center;
  padding: 64px 0 48px;
}
.tk-empty-ic {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin-bottom: 8px;
  border-radius: 50%;
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  color: var(--tx-muted);
}
.tk-empty-h {
  margin: 0;
  font: 500 15px/1.4 var(--font-ui);
  letter-spacing: -0.005em;
  color: var(--tx-primary);
}
.tk-ex-h {
  margin: 0 0 8px;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.tk-ex {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  max-width: 560px;
}
.tk-ex button {
  display: inline-flex;
  gap: 5px;
  height: 32px;
  align-items: center;
  padding: 0 14px;
  border: 0;
  border-radius: 999px;
  background: var(--bg-1);
  box-shadow: 0 0 0 1px var(--br-default);
  color: var(--tx-secondary);
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
  transition: background var(--dur), box-shadow var(--dur), color var(--dur);
}
.tk-ex button:hover {
  background: var(--bg-2);
  box-shadow: 0 0 0 1px var(--br-strong);
  color: var(--tx-primary);
}
.tk-ex .tok {
  color: var(--tx-primary);
  font-weight: 500;
}
.task-row.fresh {
  background: var(--bg-selected);
  transition: background 1s;
}
.tk-desktop-opts {
  display: flex;
  align-items: center;
  gap: 4px;
}
.tk-mobile-more {
  display: none;
}

@media (max-width: 640px) {
  .tk-stats {
    display: none !important;
  }
  .tk-desktop-opts {
    display: none !important;
  }
  .tk-mobile-more {
    display: block;
  }
  .tk-more-btn {
    display: grid;
    place-items: center;
    width: 32px;
    height: 28px;
    padding: 0 !important;
    border-radius: 999px;
  }
  .tk-more-menu {
    width: 220px;
    right: 0;
    left: auto;
  }
  .tk-more-actions {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--br-subtle);
  }
  .tk-more-actions .btn {
    width: 100%;
    justify-content: center;
    font-size: 12px;
  }
  .tk-sum,
  .tk-chip .key,
  .tk-foot .key,
  .tk-day,
  .tk-sugg-help {
    display: none;
  }
  .tk-details {
    grid-template-columns: 1fr;
  }
  .tk-sugg {
    left: 16px;
    right: 16px;
    min-width: 0;
  }
  .tk-head {
    flex-wrap: wrap;
  }
  .tk-stats {
    flex-basis: 100%;
    order: 3;
    margin: 6px 0 0;
    font-size: 14px;
    color: var(--tx-secondary);
  }
  .v-head-row .btn-primary,
  .tk-by,
  .tk-pick {
    display: none;
  }
  .tk-tabs {
    mask-image: linear-gradient(to right, #000 calc(100% - 32px), transparent);
  }
  .tk-opts::after {
    content: "";
    position: sticky;
    right: 0;
    flex: none;
    align-self: stretch;
    width: 32px;
    margin-left: -32px;
    background: linear-gradient(to right, transparent, var(--bg-0));
    pointer-events: none;
  }
  .tk-tabs {
    flex: 1 1 0;
    min-width: 0;
    flex-wrap: nowrap;
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tk-tabs button {
    flex: none;
  }
  .tk-opts {
    margin-top: 12px;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tk-opts > * {
    flex: none;
  }
  .tk-bar .spacer {
    display: none;
  }
  .tk-bar > .input {
    flex-basis: 100%;
    max-width: none;
  }
}

.btn-primary .key {
  margin-left: 6px;
  background: transparent;
  color: inherit;
  border-color: currentColor;
  opacity: .85;
}
.tgroup > .lbl.od {
  color: var(--a-red);
}
.g-sum {
  margin-left: auto;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
</style>
