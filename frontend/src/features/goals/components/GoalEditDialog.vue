<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Goal, GoalKind, Habit, Project } from '@/api/client'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { filesUrl } from '@/lib/noteFiles'
import { spreadDue, topicLines } from '@/lib/goals'
import DeleteButton from '@/components/DeleteButton.vue'

const props = defineProps<{
  open: boolean
  goal?: Goal | null
  initialParentId?: string
  goals: Goal[]
  projects: Project[]
  habits: Habit[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (
    e: 'save',
    payload: {
      id?: string
      body: Partial<Goal>
      file: File | null
      steps: { title: string; due_date: string }[]
    },
  ): void
  (e: 'delete', goal: Goal): void
  (e: 'toggleArchive', goal: Goal, value: boolean): void
}>()

const KINDS: Record<GoalKind, string> = {
  NONE: 'Ohne',
  PROJECT_TASKS: 'Aufgaben',
  PROJECT_HOURS: 'Stunden',
  HABIT_COUNT: 'Gewohnheit',
}

function blankForm(parent = '') {
  return {
    id: '',
    title: '',
    why: '',
    kind: 'NONE' as GoalKind,
    project: '',
    habit: '',
    target: 0,
    parent,
    start: ymd(new Date()),
    due: '',
    image: null as string | null,
    file: null as File | null,
    archived: false,
    topics: '',
    priority: 'MEDIUM' as Goal['priority'],
  }
}

const form = ref(blankForm())
const fileInput = ref<HTMLInputElement>()
const fileObjectUrl = ref<string | null>(null)

watch(
  () => form.value.file,
  (file, _, onCleanup) => {
    if (!file) {
      fileObjectUrl.value = null
      return
    }
    const url = URL.createObjectURL(file)
    fileObjectUrl.value = url
    onCleanup(() => URL.revokeObjectURL(url))
  },
  { immediate: true },
)

watch(
  () => [props.open, props.goal, props.initialParentId] as const,
  ([isOpen, g, parentId]) => {
    if (!isOpen) return
    if (fileInput.value) fileInput.value.value = ''
    if (!g) {
      form.value = blankForm(parentId ?? '')
      return
    }
    form.value = {
      id: g.id,
      title: g.title,
      why: g.why,
      kind: g.kind,
      project: g.project_id ?? '',
      habit: g.habit_id ?? '',
      target: g.target,
      parent: g.parent_id ?? '',
      start: g.start_date,
      due: g.due_date ?? '',
      image: g.image_path,
      file: null,
      archived: g.archived,
      topics: '',
      priority: g.priority,
    }
  },
  { immediate: true },
)

const preview = computed(
  () => fileObjectUrl.value ?? (form.value.image ? filesUrl(form.value.image) : null),
)

const hasSteps = computed(
  () => !!form.value.id && props.goals.some((g) => g.parent_id === form.value.id),
)
const parents = computed(() => props.goals.filter((g) => !g.parent_id && g.id !== form.value.id))

// Nur ein neues Langzeitziel mit Frist in der Zukunft bekommt das Themenfeld.
const canPlan = computed(
  () => !form.value.id && !form.value.parent && form.value.due > ymd(new Date()),
)

function pickFile(e: Event) {
  form.value.file = (e.target as HTMLInputElement).files?.[0] ?? null
}

function removeImage() {
  form.value.file = null
  form.value.image = null
  if (fileInput.value) fileInput.value.value = ''
}

function save() {
  const f = form.value
  const body: Partial<Goal> = {
    title: f.title,
    why: f.why,
    kind: f.kind,
    project_id: f.project,
    habit_id: f.habit,
    parent_id: f.parent,
    target: f.target || 0,
    start_date: f.start,
    due_date: f.due,
    image_path: f.image ?? '',
    priority: f.priority,
  }
  // Themen (eine Zeile je Thema) werden Teilziele mit rückwärts verteilten Fristen, etwa für eine Klausur.
  const topics = canPlan.value ? topicLines(f.topics) : []
  const dues = spreadDue(ymd(new Date()), f.due, topics.length)
  emit('save', {
    id: f.id || undefined,
    body,
    file: f.file,
    steps: topics.map((title, i) => ({ title, due_date: dues[i]! })),
  })
}

function handleDelete() {
  const g = props.goals.find((x) => x.id === form.value.id)
  if (g) emit('delete', g)
}

function handleToggleArchive() {
  const g = props.goals.find((x) => x.id === form.value.id)
  if (g) emit('toggleArchive', g, !form.value.archived)
}
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="sheet-wrap"
    @mousedown.self="emit('close')"
  >
    <form
      class="sheet gl-sheet"
      :aria-label="form.id ? $t('Ziel bearbeiten') : $t('Neues Ziel')"
      @submit.prevent="save"
    >
      <div class="sheet-head">
        <span class="lbl">
          {{
            form.id
              ? $t('Ziel bearbeiten')
              : form.parent
                ? $t('Neues Teilziel')
                : $t('Neues Langzeitziel')
          }}
        </span>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="sheet-body">
        <input
          id="g-title"
          v-model="form.title"
          class="sheet-title"
          :placeholder="$t('z. B. Bachelor abschließen')"
          :aria-label="$t('Ziel')"
          autofocus
          required
        />

        <textarea
          id="g-why"
          v-model="form.why"
          class="sh-why"
          rows="2"
          :placeholder="$t('Ein Satz, der dich erinnert, wofür es sich lohnt')"
          :aria-label="$t('Warum?')"
        ></textarea>

        <div class="sheet-props">
          <label v-if="!hasSteps" class="sheet-row">
            <span>{{ $t('Gehört zu') }}</span>
            <select id="g-parent" v-model="form.parent" class="input">
              <option value="">{{ $t('Ist selbst ein Langzeitziel') }}</option>
              <option v-for="p in parents" :key="p.id" :value="p.id">{{ p.title }}</option>
            </select>
          </label>

          <label v-if="!form.parent" class="sheet-row">
            <span>{{ $t('Priorität') }}</span>
            <select id="g-prio" v-model="form.priority" class="input">
              <option value="URGENT">{{ $t('Dringend') }}</option>
              <option value="HIGH">{{ $t('Hoch') }}</option>
              <option value="MEDIUM">{{ $t('Mittel') }}</option>
              <option value="LOW">{{ $t('Niedrig') }}</option>
            </select>
          </label>

          <label class="sheet-row">
            <span>{{ $t('Fortschritt aus') }}</span>
            <select id="g-kind" v-model="form.kind" class="input">
              <option v-for="(l, v) in KINDS" :key="v" :value="v">{{ $t(l) }}</option>
            </select>
          </label>

          <label
            v-if="form.kind === 'PROJECT_TASKS' || form.kind === 'PROJECT_HOURS'"
            class="sheet-row"
          >
            <span>{{ $t('Projekt') }}</span>
            <select id="g-proj" v-model="form.project" class="input" required>
              <option value="" disabled>{{ $t('Projekt wählen') }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </label>

          <label v-if="form.kind === 'HABIT_COUNT'" class="sheet-row">
            <span>{{ $t('Gewohnheit') }}</span>
            <select id="g-habit" v-model="form.habit" class="input" required>
              <option value="" disabled>{{ $t('Gewohnheit wählen') }}</option>
              <option v-for="h in habits" :key="h.id" :value="h.id">{{ h.name }}</option>
            </select>
          </label>

          <label
            v-if="form.kind === 'PROJECT_HOURS' || form.kind === 'HABIT_COUNT'"
            class="sheet-row"
          >
            <span>{{
              form.kind === 'PROJECT_HOURS' ? $t('Ziel in Stunden') : $t('Wie oft?')
            }}</span>
            <input
              id="g-target"
              v-model.number="form.target"
              class="input mono"
              type="number"
              min="1"
              required
            />
          </label>

          <label
            v-if="form.kind === 'PROJECT_HOURS' || form.kind === 'HABIT_COUNT'"
            class="sheet-row"
          >
            <span>{{ $t('Zählt ab') }}</span>
            <input id="g-start" v-model="form.start" class="input" type="date" required />
          </label>

          <label class="sheet-row">
            <span>{{ $t('Bis') }}</span>
            <input id="g-due" v-model="form.due" class="input" type="date" />
          </label>

          <label v-if="canPlan" class="sheet-row gl-topics-row">
            <span>{{ $t('Themen') }}</span>
            <textarea
              id="g-topics"
              v-model="form.topics"
              class="input gl-topics"
              rows="3"
              :placeholder="$t('Ein Thema pro Zeile')"
            ></textarea>
          </label>

          <div class="sheet-row gl-img-cell">
            <label for="g-img" class="gl-lbl">{{ $t('Bild') }}</label>
            <div class="gl-img-row">
              <input
                id="g-img"
                ref="fileInput"
                class="input"
                type="file"
                accept="image/png,image/jpeg,image/gif,image/webp"
                @change="pickFile"
              />
              <button
                v-if="preview"
                type="button"
                class="btn btn-ghost btn-sm"
                @click="removeImage"
              >
                {{ $t('Entfernen') }}
              </button>
            </div>
          </div>
        </div>

        <img v-if="preview" class="gl-preview" :src="preview" alt="" />

        <p v-if="form.kind === 'NONE' && !form.parent" class="sh-note">
          {{
            $t(
              'Ohne eigene Messung zeigt ein Langzeitziel, wie viele seiner Teilziele erreicht sind.',
            )
          }}
        </p>
        <p v-if="canPlan && form.topics.trim()" class="sh-note">
          {{
            $t(
              'Jedes Thema wird ein Teilziel. Die Fristen verteilen sich bis drei Tage vor dem Termin, die letzten Tage bleiben zum Wiederholen.',
            )
          }}
        </p>
        <p v-if="form.kind === 'PROJECT_TASKS'" class="sh-note">
          {{ $t('Zählt erledigte von allen Aufgaben des Projekts.') }}
        </p>
      </div>

      <div class="sheet-foot">
        <DeleteButton
          v-if="form.id"
          ghost
          :text="
            hasSteps
              ? $t('„{title}“ samt Teilzielen löschen?', { title: form.title })
              : $t('„{title}“ löschen?', { title: form.title })
          "
          @confirm="handleDelete"
        />
        <button v-if="form.id" class="btn btn-ghost" type="button" @click="handleToggleArchive">
          {{ form.archived ? $t('Wieder öffnen') : $t('Erreicht') }}
        </button>
        <span class="spacer" />
        <button class="btn btn-ghost" type="button" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button class="btn btn-primary" type="submit">
          {{ form.id ? $t('Speichern') : $t('Anlegen') }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
/* .sheet-row .input ist auf eine Zeile ausgelegt; die Themen brauchen mehrere. */
.sheet-row.gl-topics-row {
  align-items: start;
  padding: 6px 0;
}
.gl-topics-row > span {
  padding-top: 7px;
}
.sheet-row .gl-topics {
  height: auto;
  min-height: 64px;
  padding: 6px;
  line-height: 1.45;
  resize: vertical;
}
.gl-sheet {
  --sheet-w: 420px;
  --sheet-lbl-w: 112px;
}
.sh-why {
  width: 100%;
  margin-top: 6px;
  padding: 6px 0;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: none;
  color: var(--tx-secondary);
  font: 400 13.5px/1.5 var(--font-ui);
  resize: vertical;
  min-height: 44px;
}
.sh-why::placeholder {
  color: var(--tx-muted);
}
.sh-why:focus-visible {
  outline: none;
  border-radius: 0;
  box-shadow: none;
  border-bottom-color: var(--a-focus);
}
.gl-lbl {
  flex: none;
  width: var(--sheet-lbl-w);
  color: var(--tx-muted);
}
.gl-img-cell {
  padding: 6px 0;
}
.gl-img-row {
  flex: 1;
  min-width: 0;
  display: flex;
  gap: 8px;
  align-items: center;
}
.gl-img-row input[type='file'] {
  flex: 1;
  min-width: 0;
  height: auto;
  padding: 4px 6px;
  font-size: 12px;
}
.gl-preview {
  display: block;
  width: 100%;
  max-height: 150px;
  margin-top: 12px;
  object-fit: cover;
  border-radius: var(--r-s);
  border: 1px solid var(--br-subtle);
}
.sh-note {
  margin: 12px 0 0;
  font: 400 12px/1.45 var(--font-ui);
  color: var(--tx-muted);
}
.spacer {
  flex: 1;
}
</style>
