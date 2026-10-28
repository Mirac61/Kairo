<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { createResource, createTask, isOpen, type Project, type Task } from '@/api/client'
import { useFreeSlot } from '@/composables/useFreeSlot'
import { dayLabel, hm, ruleText } from '@/lib/dates'
import { t, tn } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'
import { parseQuickAdd, quickTokens, taskBody, type TokenKind } from '@/lib/quickAdd'

const props = defineProps<{
  projects: Project[]
  tasks: Task[]
  defaultProjectId?: string | null
  todayString: string
}>()

const emit = defineEmits<{
  (e: 'created', tasks: Task[], notificationText: string): void
  (e: 'error', message: string): void
}>()

const quick = ref('')
const quickEl = ref<HTMLInputElement>()
const mirror = ref<HTMLElement>()

const details = ref(false)
const noteText = ref('')
const subsText = ref('')
const issueRef = ref('')

const focused = ref(false)
const dismissed = ref(false)
const suggIdx = ref(0)

const activeProjects = computed(() => props.projects.filter((p) => p.status !== 'ARCHIVED'))
const quickParsed = computed(() =>
  parseQuickAdd(quick.value, activeProjects.value, props.todayString),
)
const tokens = computed(() => quickTokens(quick.value, activeProjects.value, props.todayString))

const segs = computed(() => {
  let tokenIndex = 0
  return quick.value
    .split(/(\s+)/)
    .filter(Boolean)
    .map((word) => ({
      w: word,
      kind: /^\s/.test(word) ? '' : (tokens.value[tokenIndex++]?.kind ?? ''),
    }))
})

function syncScroll() {
  if (mirror.value && quickEl.value) {
    mirror.value.scrollLeft = quickEl.value.scrollLeft
  }
}
watch(quick, () => void nextTick(syncScroll))

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

const PRIO_LABEL: Record<string, string> = {
  URGENT: t('Dringend'),
  HIGH: t('Hoch'),
  MEDIUM: t('Mittel'),
  LOW: t('Niedrig'),
}

const sugg = computed(() => {
  const match = focused.value && !dismissed.value ? /(?:^|\s)([#@!])(\S*)$/.exec(quick.value) : null
  if (!match) return { title: '', items: [] }

  const prefix = match[1]!
  const query = match[2]!.toLowerCase()

  let list: { v: string; label: string; sub: string; color: string }[]

  if (prefix === '#') {
    list = activeProjects.value.map((p) => ({
      v: `#${p.name.replace(/\s+/g, '')}`,
      label: p.name,
      sub: t('{n} offen', {
        n: props.tasks.filter((x) => x.project_id === p.id && isOpen(x)).length,
      }),
      color: projectColor(p.id),
    }))
  } else if (prefix === '@') {
    list = [
      ...DAY_WORDS.map((w) => ({
        v: `@${w}`,
        label: `@${w}`,
        sub: dayLabel(parseQuickAdd(`@${w}`, [], props.todayString).date!, props.todayString),
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
  const rest = tokens.value
    .filter((x) => x.kind !== kind)
    .map((x) => x.w)
    .join(' ')
  quick.value = [rest, val].filter(Boolean).join(' ') + (val.length > 1 ? ' ' : '')
  quickEl.value?.focus()
}

const quickChips = computed(() => {
  const q = quickParsed.value
  const whenFormatted = [
    q.date && `${dayLabel(q.date, props.todayString)}${q.time ? ` ${q.time}` : ''}`,
    q.rule && `↻ ${ruleText(q.rule)}`,
  ]
    .filter(Boolean)
    .join(' · ')

  const nextDuration = q.minutes ? DURATION_STEPS[DURATION_STEPS.indexOf(q.minutes) + 1] : 30
  const durationValue = nextDuration
    ? nextDuration >= 60
      ? `${nextDuration / 60}h`
      : `${nextDuration}m`
    : ''

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
  return q.title && !q.time ? (q.date ?? props.todayString) : null
})

const currentMinuteOfDay = () => {
  const d = new Date()
  return d.getHours() * 60 + d.getMinutes()
}

const {
  hint: slotHint,
  slot,
  blocks,
  win,
} = useFreeSlot(
  () => slotDay.value,
  () => quickParsed.value.minutes,
  () => props.todayString,
  currentMinuteOfDay,
)

function computeSpan(startMin: number, endMin: number) {
  const { start, end } = win.value
  const leftBound = Math.max(startMin, start)
  const leftPercent = ((leftBound - start) / (end - start)) * 100
  const widthPercent = (Math.max(Math.min(endMin, end) - leftBound, 0) / (end - start)) * 100
  return { left: `${leftPercent}%`, width: `${widthPercent}%` }
}

async function addTask(planWithFreeSlot = false) {
  const q = quickParsed.value
  if (!q.title) return

  const issue = parseIssueReference(issueRef.value)
  if (issue === null) {
    emit('error', t('GitHub-Issue als owner/repo#12 oder als Link angeben.'))
    return
  }

  const plannedTime =
    planWithFreeSlot && slotHint.value?.time ? slotHint.value.time.padStart(5, '0') : null
  const note = noteText.value.trim()
  const subtaskTitles = subsText.value
    .split('\n')
    .map((x) => x.trim())
    .filter(Boolean)

  try {
    const taskPayload = {
      project_id: props.defaultProjectId ?? null,
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

    emit('created', [createdTask], t('„{title}“ angelegt', { title: createdTask.title }))
  } catch (err) {
    emit('error', err instanceof Error ? err.message : String(err))
  }
}

async function onPaste(event: ClipboardEvent) {
  const lines = (event.clipboardData?.getData('text') ?? '')
    .split(/\r?\n/)
    .map((x) => x.trim())
    .filter(Boolean)

  if (lines.length < 2) return
  event.preventDefault()

  const quickItems = lines
    .map((line) => parseQuickAdd(line, activeProjects.value, props.todayString))
    .filter((q) => q.title)

  try {
    const createdList: Task[] = []
    for (const item of quickItems) {
      const task = await createTask({
        project_id: props.defaultProjectId ?? null,
        ...taskBody(item),
        title: item.title,
      })
      createdList.push(task)
    }
    if (createdList.length) {
      emit(
        'created',
        createdList,
        tn(createdList.length, '{n} Aufgabe angelegt', '{n} Aufgaben angelegt'),
      )
    }
  } catch (err) {
    emit('error', err instanceof Error ? err.message : String(err))
  }
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

defineExpose({
  focus: () => quickEl.value?.focus(),
  setQuick: (val: string) => {
    quick.value = val
    quickEl.value?.focus()
  },
  quickEl,
})
</script>

<template>
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
            >{{ g.w }}</span
          >
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
        <span class="cdot" :style="{ background: item.color || 'var(--tx-muted)' }"></span>
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
        <b aria-hidden="true">{{ chip.lead }}</b
        >{{ chip.label }}
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
        {{
          $t(
            'Kürzel: # Projekt · @ Fällig · ! Priorität · 30m / 1h Dauer · mehrere Zeilen einfügen legt mehrere Aufgaben an',
          )
        }}
      </span>
    </div>
  </form>
</template>

<style scoped>
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
  box-shadow:
    0 0 0 1px var(--a-focus),
    0 0 0 4px var(--ring);
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
.tk-mirror [class^='k-'] {
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
  box-shadow:
    0 0 0 1px var(--br-default),
    var(--shadow-pop);
}
.tk-sugg li[role='option'] {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 6px;
  font: 400 13px/1 var(--font-ui);
  cursor: pointer;
}
.tk-sugg li[aria-selected='true'] {
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
  opacity: 0.7;
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
.tk-tip {
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
</style>
