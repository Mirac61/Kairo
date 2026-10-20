<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  ApiError, createGoal, createNote, deleteGoal, deleteNote, listGoals, listHabits, listProjects, updateGoal, uploadFile,
  type Goal, type GoalKind, type Habit, type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { filesUrl } from '@/lib/noteFiles'
import { goalProgressText } from '@/lib/goals'
import { locale, t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

const KINDS: Record<GoalKind, string> = {
  NONE: t('Ohne'), PROJECT_TASKS: t('Aufgaben'), PROJECT_HOURS: t('Stunden'), HABIT_COUNT: t('Gewohnheit'),
}
const IMAGE_DIR = 'Ziele'

const goals = ref<Goal[]>([])
const projects = ref<Project[]>([])
const habits = ref<Habit[]>([])
const active = computed(() => goals.value.filter((g) => !g.archived))
const archived = computed(() => goals.value.filter((g) => g.archived))

const { error, load, run } = useLoader(async () => {
  const [gs, ps, hs] = await Promise.all([listGoals(), listProjects(), listHabits()])
  goals.value = gs
  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')
  habits.value = hs.filter((h) => h.active)
})
onMounted(load)
useLiveEvents(load)

const dialog = ref(false)
const empty = () => ({
  id: '', title: '', why: '', kind: 'NONE' as GoalKind, project: '', habit: '', target: 0,
  start: ymd(new Date()), due: '', image: null as string | null, file: null as File | null, archived: false,
})
const form = ref(empty())
const preview = computed(() => (form.value.file ? URL.createObjectURL(form.value.file) : form.value.image && filesUrl(form.value.image)))

function openNew() {
  form.value = empty()
  dialog.value = true
}
function openEdit(g: Goal) {
  form.value = {
    id: g.id, title: g.title, why: g.why, kind: g.kind, project: g.project_id ?? '', habit: g.habit_id ?? '', target: g.target,
    start: g.start_date, due: g.due_date ?? '', image: g.image_path, file: null, archived: g.archived,
  }
  dialog.value = true
}
function pickFile(e: Event) {
  form.value.file = (e.target as HTMLInputElement).files?.[0] ?? null
}
// Das alte Bild landet beim Ersetzen oder Entfernen in .trash/ im Notizordner.
const dropImage = (path: string | null) => path && deleteNote(path).catch(() => {})

async function uploadImage(id: string, file: File) {
  await createNote(IMAGE_DIR, true).catch((e) => { if (!(e instanceof ApiError && e.status === 409)) throw e })
  const ext = file.name.split('.').pop()?.toLowerCase() || 'jpg'
  // Zeitstempel im Namen: Upload überschreibt nicht, und der Browser lädt das neue Bild statt des gecachten.
  const path = `${IMAGE_DIR}/${id}-${Date.now()}.${ext}`
  await uploadFile(path, file)
  return path
}

const save = () =>
  void run(async () => {
    const f = form.value
    const body: Partial<Goal> = {
      title: f.title, why: f.why, kind: f.kind, project_id: f.project, habit_id: f.habit,
      target: f.target || 0, start_date: f.start, due_date: f.due, image_path: f.image ?? '',
    }
    const g = f.id ? await updateGoal(f.id, body) : await createGoal(body)
    const old = f.id ? goals.value.find((x) => x.id === f.id)?.image_path ?? null : null
    if (f.file) {
      await updateGoal(g.id, { image_path: await uploadImage(g.id, f.file) })
    }
    if (old && old !== (f.file ? null : f.image)) await dropImage(old)
    dialog.value = false
  })

const archive = (g: Goal, value: boolean) => void run(() => updateGoal(g.id, { archived: value }))
const remove = () =>
  void run(async () => {
    const f = form.value
    await deleteGoal(f.id)
    await dropImage(f.image)
    dialog.value = false
  })

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }))
function dueText(due: string) {
  const days = Math.round((new Date(due + 'T00:00').getTime() - new Date(ymd(new Date()) + 'T00:00').getTime()) / 864e5)
  const d = dateFmt.value.format(new Date(due + 'T00:00'))
  if (days < 0) return t('bis {date}, vorbei', { date: d })
  return t('bis {date}, noch {n} Tage', { date: d, n: days })
}
</script>

<template>
  <div class="view-inner">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">{{ $t('Ziele') }}</h1>
        <div class="v-sub">{{ $t('Wofür du das alles machst.') }}</div>
      </div>
      <button class="btn btn-primary" type="button" @click="openNew"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neues Ziel') }}</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <p v-if="!goals.length" class="v-sub gl-empty">
      {{ $t('Noch keine Ziele. Leg ein großes Ziel an, gib ihm ein Bild und einen Satz, warum es dir wichtig ist.') }}
    </p>

    <div class="gl-grid">
      <article v-for="g in active" :key="g.id" class="card gl-card">
        <button type="button" class="gl-open" :aria-label="$t('„{title}“ bearbeiten', { title: g.title })" @click="openEdit(g)">
          <img v-if="g.image_path" class="gl-img" :src="filesUrl(g.image_path)" alt="" />
          <div class="gl-body">
            <h2 class="gl-title">{{ g.title }}</h2>
            <blockquote v-if="g.why" class="gl-why">{{ g.why }}</blockquote>
            <template v-if="g.progress">
              <div class="pbar" role="progressbar" :aria-valuenow="g.progress.percent" aria-valuemin="0" aria-valuemax="100" :aria-label="g.title">
                <div class="pbar-fill" :style="{ width: g.progress.percent + '%' }"></div>
              </div>
              <div class="gl-meta"><b>{{ g.progress.percent }} %</b><span>{{ goalProgressText(g) }}</span></div>
            </template>
            <div v-if="g.due_date" class="gl-due">{{ dueText(g.due_date) }}</div>
          </div>
        </button>
      </article>
    </div>

    <template v-if="archived.length">
      <h2 class="col-title gl-arch-head">{{ $t('Erreicht und archiviert') }}</h2>
      <div class="gl-arch">
        <div v-for="g in archived" :key="g.id" class="gl-arch-row">
          <span>{{ g.title }}</span>
          <button type="button" class="btn btn-ghost" @click="archive(g, false)">{{ $t('Zurückholen') }}</button>
          <button type="button" class="btn btn-ghost" @click="openEdit(g)">{{ $t('Bearbeiten') }}</button>
        </div>
      </div>
    </template>

    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <form class="dialog" :aria-label="form.id ? $t('Ziel bearbeiten') : $t('Neues Ziel')" @submit.prevent="save">
        <div class="dlg-head">
          <h3>{{ form.id ? $t('Ziel bearbeiten') : $t('Neues Ziel') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="field"><label for="g-title">{{ $t('Ziel') }}</label><input id="g-title" v-model="form.title" class="input" :placeholder="$t('z. B. Bachelor abschließen')" autofocus required /></div>
          <div class="field"><label for="g-why">{{ $t('Warum?') }}</label><textarea id="g-why" v-model="form.why" class="input" rows="2" :placeholder="$t('Ein Satz, der dich erinnert, wofür es sich lohnt')"></textarea></div>
          <div class="field">
            <label for="g-img">{{ $t('Bild') }}</label>
            <img v-if="preview" class="gl-preview" :src="preview" alt="" />
            <div class="gl-img-row">
              <input id="g-img" class="input" type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="pickFile" />
              <button v-if="preview" type="button" class="btn btn-ghost" @click="form.file = null; form.image = null">{{ $t('Entfernen') }}</button>
            </div>
          </div>
          <div class="field">
            <label id="g-kind">{{ $t('Fortschritt aus') }}</label>
            <div class="seg" role="group" aria-labelledby="g-kind">
              <button v-for="(l, v) in KINDS" :key="v" type="button" :aria-pressed="form.kind === v" @click="form.kind = v">{{ l }}</button>
            </div>
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
            <label for="g-target">{{ form.kind === 'PROJECT_HOURS' ? $t('Ziel in Stunden') : $t('Wie oft?') }}</label>
            <input id="g-target" v-model.number="form.target" class="input" type="number" min="1" required />
          </div>
          <div v-if="form.kind === 'PROJECT_TASKS'" class="v-sub">{{ $t('Zählt erledigte von allen Aufgaben des Projekts.') }}</div>
          <div class="dlg-row">
            <div v-if="form.kind === 'PROJECT_HOURS' || form.kind === 'HABIT_COUNT'" class="field"><label for="g-start">{{ $t('Zählt ab') }}</label><input id="g-start" v-model="form.start" class="input" type="date" required /></div>
            <div class="field"><label for="g-due">{{ $t('Bis') }}</label><input id="g-due" v-model="form.due" class="input" type="date" /></div>
          </div>
        </div>
        <div class="dlg-foot">
          <DeleteButton v-if="form.id" ghost :text="$t('„{title}“ löschen?', { title: form.title })" @confirm="remove" />
          <button v-if="form.id && !form.archived" class="btn btn-ghost" type="button"
            @click="archive(goals.find((g) => g.id === form.id)!, true); dialog = false">{{ $t('Archivieren') }}</button>
          <span class="gl-spacer"></span>
          <button class="btn btn-ghost" type="button" @click="dialog = false">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit">{{ form.id ? $t('Speichern') : $t('Anlegen') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.gl-empty { margin-top: 8px; }
.gl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); align-items: start; gap: 16px; }
.gl-card { padding: 0; overflow: hidden; }
.gl-open { display: block; width: 100%; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.gl-img { display: block; width: 100%; aspect-ratio: 16 / 9; object-fit: cover; }
.gl-body { display: grid; gap: 10px; padding: 16px 18px 18px; }
.gl-title { margin: 0; font: 600 18px/1.3 var(--font-ui); color: var(--tx-primary); }
.gl-why { margin: 0; padding-left: 12px; border-left: 2px solid var(--br-default); font: italic 400 14px/1.45 var(--font-ui); color: var(--tx-secondary); }
.gl-meta { display: flex; justify-content: space-between; gap: 8px; font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.gl-meta b { font-weight: 600; color: var(--tx-primary); font-variant-numeric: tabular-nums; }
.gl-due { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.gl-arch-head { margin: 28px 0 8px; }
.gl-arch { display: grid; gap: 4px; }
.gl-arch-row { display: flex; align-items: center; gap: 8px; color: var(--tx-muted); }
.gl-arch-row span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.gl-preview { display: block; width: 100%; max-height: 160px; object-fit: cover; border-radius: var(--r-s, 6px); margin-bottom: 6px; }
.gl-img-row { display: flex; gap: 8px; align-items: center; }
.gl-img-row .input { flex: 1; min-width: 0; }
.dlg-row { display: flex; gap: 12px; }
.dlg-row .field { flex: 1; }
.dlg-foot .gl-spacer { flex: 1; }
</style>
