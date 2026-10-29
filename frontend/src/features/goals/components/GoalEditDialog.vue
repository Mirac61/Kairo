<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Goal, GoalKind, Habit, Project } from '@/api/client'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { filesUrl } from '@/lib/noteFiles'
import { t } from '@/lib/i18n'
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
    },
  ): void
  (e: 'delete', goal: Goal): void
  (e: 'toggleArchive', goal: Goal, value: boolean): void
}>()

const KINDS: Record<GoalKind, string> = {
  NONE: t('Ohne'),
  PROJECT_TASKS: t('Aufgaben'),
  PROJECT_HOURS: t('Stunden'),
  HABIT_COUNT: t('Gewohnheit'),
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
  }
}

const form = ref(blankForm())

watch(
  () => [props.open, props.goal, props.initialParentId] as const,
  ([isOpen, g, parentId]) => {
    if (!isOpen) return
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
    }
  },
  { immediate: true },
)

const preview = computed(() =>
  form.value.file
    ? URL.createObjectURL(form.value.file)
    : form.value.image
      ? filesUrl(form.value.image)
      : null,
)

const hasSteps = computed(
  () => !!form.value.id && props.goals.some((g) => g.parent_id === form.value.id),
)
const parents = computed(() => props.goals.filter((g) => !g.parent_id && g.id !== form.value.id))

function pickFile(e: Event) {
  form.value.file = (e.target as HTMLInputElement).files?.[0] ?? null
}

function removeImage() {
  form.value.file = null
  form.value.image = null
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
  }
  emit('save', {
    id: f.id || undefined,
    body,
    file: f.file,
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
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <form
      class="dialog"
      :aria-label="form.id ? $t('Ziel bearbeiten') : $t('Neues Ziel')"
      @submit.prevent="save"
    >
      <div class="dlg-head">
        <h3>
          {{
            form.id
              ? $t('Ziel bearbeiten')
              : form.parent
                ? $t('Neues Teilziel')
                : $t('Neues Langzeitziel')
          }}
        </h3>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Schließen')"
          @click="emit('close')"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>
      <div class="dlg-body">
        <div class="field">
          <label for="g-title">{{ $t('Ziel') }}</label
          ><input
            id="g-title"
            v-model="form.title"
            class="input"
            :placeholder="$t('z. B. Bachelor abschließen')"
            autofocus
            required
          />
        </div>
        <div class="field">
          <label for="g-why">{{ $t('Warum?') }}</label
          ><textarea
            id="g-why"
            v-model="form.why"
            class="input"
            rows="2"
            :placeholder="$t('Ein Satz, der dich erinnert, wofür es sich lohnt')"
          ></textarea>
        </div>
        <div v-if="!hasSteps" class="field">
          <label for="g-parent">{{ $t('Gehört zu') }}</label>
          <select id="g-parent" v-model="form.parent" class="input">
            <option value="">{{ $t('Ist selbst ein Langzeitziel') }}</option>
            <option v-for="p in parents" :key="p.id" :value="p.id">{{ p.title }}</option>
          </select>
        </div>
        <div class="field">
          <label for="g-img">{{ $t('Bild') }}</label>
          <img v-if="preview" class="gl-preview" :src="preview" alt="" />
          <div class="gl-img-row">
            <input
              id="g-img"
              class="input"
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              @change="pickFile"
            />
            <button v-if="preview" type="button" class="btn btn-ghost" @click="removeImage">
              {{ $t('Entfernen') }}
            </button>
          </div>
        </div>
        <div class="field">
          <label id="g-kind">{{ $t('Fortschritt aus') }}</label>
          <div class="seg" role="group" aria-labelledby="g-kind">
            <button
              v-for="(l, v) in KINDS"
              :key="v"
              type="button"
              :aria-pressed="form.kind === v"
              @click="form.kind = v"
            >
              {{ l }}
            </button>
          </div>
        </div>
        <div v-if="form.kind === 'NONE' && !form.parent" class="v-sub">
          {{
            $t(
              'Ohne eigene Messung zeigt ein Langzeitziel, wie viele seiner Teilziele erreicht sind.',
            )
          }}
        </div>
        <div v-if="form.kind === 'PROJECT_TASKS' || form.kind === 'PROJECT_HOURS'" class="field">
          <label for="g-proj">{{ $t('Projekt') }}</label>
          <select id="g-proj" v-model="form.project" class="input" required>
            <option value="" disabled>{{ $t('Projekt wählen') }}</option>
            <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div v-if="form.kind === 'HABIT_COUNT'" class="field">
          <label for="g-habit">{{ $t('Gewohnheit') }}</label>
          <select id="g-habit" v-model="form.habit" class="input" required>
            <option value="" disabled>{{ $t('Gewohnheit wählen') }}</option>
            <option v-for="h in habits" :key="h.id" :value="h.id">{{ h.name }}</option>
          </select>
        </div>
        <div v-if="form.kind === 'PROJECT_HOURS' || form.kind === 'HABIT_COUNT'" class="field">
          <label for="g-target">{{
            form.kind === 'PROJECT_HOURS' ? $t('Ziel in Stunden') : $t('Wie oft?')
          }}</label>
          <input
            id="g-target"
            v-model.number="form.target"
            class="input"
            type="number"
            min="1"
            required
          />
        </div>
        <div v-if="form.kind === 'PROJECT_TASKS'" class="v-sub">
          {{ $t('Zählt erledigte von allen Aufgaben des Projekts.') }}
        </div>
        <div class="dlg-row">
          <div v-if="form.kind === 'PROJECT_HOURS' || form.kind === 'HABIT_COUNT'" class="field">
            <label for="g-start">{{ $t('Zählt ab') }}</label
            ><input id="g-start" v-model="form.start" class="input" type="date" required />
          </div>
          <div class="field">
            <label for="g-due">{{ $t('Bis') }}</label
            ><input id="g-due" v-model="form.due" class="input" type="date" />
          </div>
        </div>
      </div>
      <div class="dlg-foot">
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
        <span class="gl-spacer"></span>
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
.gl-preview {
  display: block;
  width: 100%;
  max-height: 160px;
  object-fit: cover;
  border-radius: var(--r-s);
  margin-bottom: 6px;
}
.gl-img-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.gl-img-row .input {
  flex: 1;
  min-width: 0;
}
.dlg-row {
  display: flex;
  gap: 12px;
}
.dlg-row .field {
  flex: 1;
}
.dlg-foot .gl-spacer {
  flex: 1;
}
</style>
