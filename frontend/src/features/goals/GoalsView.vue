<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import {
  ApiError,
  createGoal,
  createNote,
  deleteGoal,
  deleteNote,
  listGoals,
  listHabits,
  listProjects,
  updateGoal,
  uploadFile,
  type Goal,
  type GoalKind,
  type Habit,
  type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { filesUrl } from '@/lib/noteFiles'
import { goalPace, goalProgressText } from '@/lib/goals'
import { locale, t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

const KINDS: Record<GoalKind, string> = {
  NONE: t('Ohne'),
  PROJECT_TASKS: t('Aufgaben'),
  PROJECT_HOURS: t('Stunden'),
  HABIT_COUNT: t('Gewohnheit'),
}
const IMAGE_DIR = 'Ziele'

const goals = ref<Goal[]>([])
const projects = ref<Project[]>([])
const habits = ref<Habit[]>([])
const longTerm = computed(() => goals.value.filter((g) => !g.parent_id && !g.archived))
const reached = computed(() => goals.value.filter((g) => !g.parent_id && g.archived))
const stepsOf = (id: string) =>
  goals.value
    .filter((g) => g.parent_id === id)
    .sort((a, b) => Number(a.archived) - Number(b.archived))

const { error, load, run } = useLoader(async () => {
  const [gs, ps, hs] = await Promise.all([listGoals(), listProjects(), listHabits()])
  goals.value = gs
  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')
  habits.value = hs.filter((h) => h.active)
})
onMounted(load)
useLiveEvents(load)

// --- Karten
const today = ymd(new Date())
const shortDate = (d: string) =>
  new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(
    new Date(d + 'T00:00'),
  )
const nextStep = (g: Goal) => stepsOf(g.id).find((s) => !s.archived)
const STATE = { ok: t('Im Plan'), tight: t('Knapp'), off: t('Hinterher') }
const tab = ref<'active' | 'reached'>('active')

// Diagramm in 0..100 auf beiden Achsen: x von start_date bis zur Frist (ohne Frist bis heute), y in Prozent.
const dayNo = (d: string) => Date.parse(d + 'T00:00:00Z') / 864e5
function chart(g: Goal) {
  const from = dayNo(g.start_date)
  const to = dayNo(g.due_date && g.due_date > g.start_date ? g.due_date : today)
  const x = (d: string) =>
    to > from ? Math.min(100, ((dayNo(d) - from) / (to - from)) * 100) : 100
  return {
    line: (g.history ?? []).map((h) => `${x(h.date).toFixed(2)},${100 - h.percent}`).join(' '),
    now: x(today),
  }
}
const cards = computed(() =>
  (tab.value === 'active' ? longTerm.value : reached.value).map((g) => ({
    g,
    pace: g.archived ? null : goalPace(g, today),
    chart: chart(g),
    steps: stepsOf(g.id),
    next: nextStep(g),
  })),
)
const counts = computed(() => {
  const c = { ok: 0, tight: 0, off: 0 }
  for (const g of longTerm.value) {
    const p = goalPace(g, today)
    if (p) c[p.state]++
  }
  return c
})

// Schritte direkt auf der Karte anlegen, abhaken, löschen
const drafts = reactive<Record<string, string>>({})
const addStep = (g: Goal) =>
  void run(async () => {
    const title = drafts[g.id]?.trim()
    if (!title) return
    drafts[g.id] = ''
    await createGoal({ title, parent_id: g.id })
    await load()
  })
const confirm = useConfirm()
function removeStep(e: Event, s: Goal) {
  confirm.require({
    target: e.currentTarget as HTMLElement,
    message: t('„{title}“ löschen?', { title: s.title }),
    acceptLabel: t('Löschen'),
    rejectLabel: t('Abbrechen'),
    defaultFocus: 'reject',
    acceptProps: { size: 'small' },
    rejectProps: { severity: 'secondary', size: 'small', text: true },
    accept: () =>
      void run(async () => {
        await deleteGoal(s.id)
        await dropImage(s.image_path)
        await load()
      }),
  })
}

// --- Dialog
const dialog = ref(false)
const empty = (parent = '') => ({
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
})
const form = ref(empty())
const preview = computed(() =>
  form.value.file
    ? URL.createObjectURL(form.value.file)
    : form.value.image && filesUrl(form.value.image),
)
// Ein Langzeitziel mit Teilzielen bleibt oben; als Eltern taugen nur andere Langzeitziele.
const hasSteps = computed(
  () => !!form.value.id && goals.value.some((g) => g.parent_id === form.value.id),
)
const parents = computed(() => goals.value.filter((g) => !g.parent_id && g.id !== form.value.id))

function openNew(parent = '') {
  form.value = empty(parent)
  dialog.value = true
}
function openEdit(g: Goal) {
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
  dialog.value = true
}
function pickFile(e: Event) {
  form.value.file = (e.target as HTMLInputElement).files?.[0] ?? null
}
// Das alte Bild landet beim Ersetzen oder Entfernen in .trash/ im Notizordner.
const dropImage = (path: string | null) => path && deleteNote(path).catch(() => {})

async function uploadImage(id: string, file: File) {
  await createNote(IMAGE_DIR, true).catch((e) => {
    if (!(e instanceof ApiError && e.status === 409)) throw e
  })
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
    const g = f.id ? await updateGoal(f.id, body) : await createGoal(body)
    const old = f.id ? (goals.value.find((x) => x.id === f.id)?.image_path ?? null) : null
    if (f.file) {
      await updateGoal(g.id, { image_path: await uploadImage(g.id, f.file) })
    }
    if (old && old !== (f.file ? null : f.image)) await dropImage(old)
    dialog.value = false
  })

const archive = (g: Goal, value: boolean) => void run(() => updateGoal(g.id, { archived: value }))

function removeImage() {
  form.value.file = null
  form.value.image = null
}

function toggleArchiveGoal() {
  const g = goals.value.find((x) => x.id === form.value.id)
  if (g) archive(g, !form.value.archived)
  dialog.value = false
}
// Löschen nimmt die Teilziele mit (Backend), ihre Bilder räumen wir hier weg.
const remove = () =>
  void run(async () => {
    const f = form.value
    const images = [f.image, ...stepsOf(f.id).map((s) => s.image_path)]
    await deleteGoal(f.id)
    await Promise.all(images.map(dropImage))
    dialog.value = false
  })
</script>

<template>
  <div class="view-inner gl">
    <div class="gl-head">
      <div class="v-head">
        <h1 class="v-title">{{ $t('Ziele') }}</h1>
        <p v-if="counts.ok + counts.tight + counts.off" class="v-sub">
          <span :class="{ 'gl-ok': counts.ok > 0 }">{{ $t('{n} im Plan', { n: counts.ok }) }}</span>
          ·
          <span :class="{ 'gl-tight': counts.tight > 0 }">{{
            $t('{n} knapp', { n: counts.tight })
          }}</span>
          ·
          <span :class="{ 'gl-off': counts.off > 0 }">{{
            $t('{n} hinterher', { n: counts.off })
          }}</span
          >. {{ $t('Der Strich im Balken zeigt, wo du heute stehen müsstest.') }}
        </p>
      </div>
      <div class="seg" role="group" :aria-label="$t('Filter')">
        <button type="button" :aria-pressed="tab === 'active'" @click="tab = 'active'">
          {{ $t('Aktiv') }} <span class="count">{{ longTerm.length }}</span>
        </button>
        <button
          type="button"
          :aria-pressed="tab === 'reached'"
          :disabled="!reached.length"
          @click="tab = 'reached'"
        >
          {{ $t('Erreicht') }} <span class="count">{{ reached.length }}</span>
        </button>
      </div>
      <button type="button" class="btn btn-primary" @click="openNew()">
        <svg class="ic" aria-hidden="true"><use href="#i-plus" /></svg>{{ $t('Neues Ziel') }}
      </button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <p v-if="!goals.length" class="v-sub gl-empty">
      {{
        $t(
          'Noch keine Ziele. Leg ein großes an, gib ihm eine Frist und verknüpfe Schritte, ein Projekt oder eine Gewohnheit. Dann zeigt Kairo, ob du im Plan bist.',
        )
      }}
    </p>

    <div class="gl-grid">
      <article v-for="{ g, pace, chart: c, steps, next } in cards" :key="g.id" class="card gl-card">
        <header class="gl-card-h">
          <img v-if="g.image_path" class="gl-thumb" :src="filesUrl(g.image_path)" alt="" />
          <button type="button" class="gl-card-t" @click="openEdit(g)">{{ g.title }}</button>
          <span v-if="pace" class="gl-state" :class="`gl-${pace.state}`"
            ><i></i>{{ STATE[pace.state] }}</span
          >
          <span v-else-if="g.archived" class="gl-state gl-ok"><i></i>{{ $t('Erreicht') }}</span>
          <span v-else-if="!g.due_date" class="gl-state">{{ $t('Ohne Frist') }}</span>
        </header>
        <p v-if="g.why" class="gl-why">{{ g.why }}</p>

        <template v-if="g.progress">
          <div class="gl-num">
            <b>{{ g.progress.percent }}%</b>
            <span class="gl-muted">{{ goalProgressText(g) }}</span>
            <span v-if="pace" class="gl-hint">{{ pace.hint }}</span>
          </div>
          <div
            class="gl-bar"
            role="progressbar"
            :aria-valuenow="g.progress.percent"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="$t('Fortschritt von „{title}“', { title: g.title })"
          >
            <i
              :class="pace ? `gl-${pace.state}` : ''"
              :style="{ width: `${g.progress.percent}%` }"
            ></i>
            <b
              v-if="pace"
              :style="{ left: `${pace.pace}%` }"
              :title="$t('Soll heute: {n} %', { n: pace.pace })"
            ></b>
          </div>
          <figure v-if="g.history?.length > 1" class="gl-chart">
            <svg
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
              role="img"
              :aria-label="
                pace
                  ? $t('Verlauf: {p} % erreicht, Soll heute {s} %', {
                      p: g.progress.percent,
                      s: pace.pace,
                    })
                  : $t('Verlauf: {p} % erreicht', { p: g.progress.percent })
              "
            >
              <line x1="0" y1="50" x2="100" y2="50" class="grid" />
              <line v-if="g.due_date" x1="0" y1="100" x2="100" y2="0" class="soll" />
              <line
                v-if="g.due_date && c.now < 100"
                :x1="c.now"
                y1="0"
                :x2="c.now"
                y2="100"
                class="now"
              />
              <polyline :points="c.line" class="ist" />
            </svg>
            <figcaption>
              <span>{{ shortDate(g.start_date) }}</span>
              <span
                v-if="g.due_date && c.now < 100"
                :style="{ left: `${c.now}%` }"
                class="gl-cap-now"
                >{{ $t('heute') }}</span
              >
              <span>{{ g.due_date ? shortDate(g.due_date) : $t('heute') }}</span>
            </figcaption>
          </figure>
        </template>
        <p v-else class="gl-muted">
          {{
            $t(
              'Noch kein Fortschritt. Leg Schritte an oder miss das Ziel an einem Projekt oder einer Gewohnheit.',
            )
          }}
        </p>

        <div v-if="next" class="gl-next">
          <button
            type="button"
            class="gl-box"
            role="checkbox"
            :aria-checked="false"
            :aria-label="$t('„{title}“ erreicht', { title: next.title })"
            @click="archive(next, true)"
          ></button>
          <button type="button" class="gl-step-t" @click="openEdit(next)">{{ next.title }}</button>
          <span class="gl-muted">{{ $t('Als nächstes') }}</span>
        </div>
        <details v-if="steps.length" class="gl-steps">
          <summary>
            {{ $t('Alle Schritte') }}
            <span class="count"
              >{{ steps.filter((s) => s.archived).length }}/{{ steps.length }}</span
            >
          </summary>
          <ul class="gl-list">
            <li v-for="st in steps" :key="st.id" :class="{ done: st.archived }">
              <button
                type="button"
                class="gl-box"
                role="checkbox"
                :aria-checked="st.archived"
                :aria-label="
                  st.archived
                    ? $t('„{title}“ wieder öffnen', { title: st.title })
                    : $t('„{title}“ erreicht', { title: st.title })
                "
                @click="archive(st, !st.archived)"
              >
                <svg v-if="st.archived" viewBox="0 0 24 24">
                  <path d="M5 12.5l4.5 4.5L19 7.5" />
                </svg>
              </button>
              <button type="button" class="gl-step-t" @click="openEdit(st)">
                {{ st.title
                }}<small v-if="st.progress && !st.archived"> · {{ st.progress.percent }}%</small>
              </button>
              <button
                type="button"
                class="gl-x"
                :aria-label="$t('„{title}“ löschen', { title: st.title })"
                @click="removeStep($event, st)"
              >
                ×
              </button>
            </li>
          </ul>
        </details>
        <input
          v-if="!g.archived"
          v-model="drafts[g.id]"
          class="gl-add"
          :placeholder="
            steps.length ? $t('Schritt hinzufügen …') : $t('Ersten Schritt hinzufügen …')
          "
          :aria-label="$t('Schritt zu „{title}“ hinzufügen', { title: g.title })"
          @keydown.enter.prevent="addStep(g)"
        />
      </article>
    </div>

    <div
      v-if="dialog"
      v-dialog="() => (dialog = false)"
      class="overlay open"
      @mousedown.self="dialog = false"
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
            @click="dialog = false"
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
            @confirm="remove"
          />
          <button v-if="form.id" class="btn btn-ghost" type="button" @click="toggleArchiveGoal">
            {{ form.archived ? $t('Wieder öffnen') : $t('Erreicht') }}
          </button>
          <span class="gl-spacer"></span>
          <button class="btn btn-ghost" type="button" @click="dialog = false">
            {{ $t('Abbrechen') }}
          </button>
          <button class="btn btn-primary" type="submit">
            {{ form.id ? $t('Speichern') : $t('Anlegen') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.gl-head {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 12px 16px;
  margin-bottom: 20px;
}
.gl-head .v-head {
  flex: 1 1 360px;
  margin: 0;
}
.gl-empty {
  max-width: 560px;
}
.gl-muted {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-muted);
}
.gl-ok {
  --c: var(--a-run);
  color: var(--c);
}
.gl-tight {
  --c: var(--p-yellow);
  color: var(--c);
}
.gl-off {
  --c: var(--a-red);
  color: var(--c);
}

.gl-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 440px), 1fr));
  gap: 14px;
}
.gl-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
  padding: 18px 20px;
}
.gl-card-h {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.gl-thumb {
  flex: none;
  width: 28px;
  height: 28px;
  border-radius: var(--r-s);
  object-fit: cover;
}
.gl-card-t {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  text-align: left;
  font: 600 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gl-card-t:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.gl-state {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font: 500 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
.gl-state i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c, var(--tx-muted));
}
.gl-why {
  margin: -6px 0 0;
  font: 400 13px/1.45 var(--font-ui);
  color: var(--tx-secondary);
}

.gl-num {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 4px 10px;
}
.gl-num b {
  font: 600 28px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  color: var(--tx-primary);
}
.gl-hint {
  margin-left: auto;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.gl-bar {
  position: relative;
  height: 6px;
  border-radius: 3px;
  background: var(--bg-3);
}
.gl-bar i {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: 3px;
  background: var(--c, var(--a-blue-hi));
}
.gl-bar b {
  position: absolute;
  top: -5px;
  width: 2px;
  height: 16px;
  margin-left: -1px;
  border-radius: 1px;
  background: var(--tx-primary);
}

.gl-chart {
  margin: 0;
}
.gl-chart svg {
  display: block;
  width: 100%;
  height: 88px;
  overflow: visible;
}
.gl-chart line,
.gl-chart polyline {
  vector-effect: non-scaling-stroke;
  fill: none;
}
.gl-chart .grid {
  stroke: var(--br-subtle);
}
.gl-chart .soll {
  stroke: var(--tx-disabled);
  stroke-width: 1.5;
  stroke-dasharray: 4 5;
}
.gl-chart .now {
  stroke: var(--br-strong);
}
.gl-chart .ist {
  stroke: var(--tx-primary);
  stroke-width: 2;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.gl-chart figcaption {
  position: relative;
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  font: 400 12px/1 var(--font-ui);
  color: var(--tx-muted);
}
.gl-cap-now {
  position: absolute;
  transform: translateX(-50%);
}

.gl-next {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 40px;
  padding: 0 12px;
  border-radius: var(--r-s);
  background: var(--bg-2);
}
.gl-steps summary {
  cursor: pointer;
  font: 500 13px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.gl-steps summary:hover {
  color: var(--tx-primary);
}
.gl-list {
  display: grid;
  gap: 4px;
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
}
.gl-list li {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 32px;
}
.gl-box {
  flex: none;
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border-radius: 5px;
  border: 1.5px solid var(--br-strong);
  background: none;
  cursor: pointer;
}
.gl-box:hover {
  border-color: var(--a-blue-hi);
}
.gl-box svg {
  width: 12px;
  height: 12px;
  fill: none;
  stroke: var(--bg-0);
  stroke-width: 3;
}
.done .gl-box {
  border-color: var(--a-blue-hi);
  background: var(--a-blue-hi);
}
.gl-step-t {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  cursor: pointer;
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.gl-step-t small {
  color: var(--tx-muted);
}
.done .gl-step-t {
  color: var(--tx-muted);
  text-decoration: line-through;
}
.gl-x {
  flex: none;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: var(--r-s);
  border: 0;
  background: none;
  cursor: pointer;
  font: 400 16px/1 var(--font-ui);
  color: var(--tx-muted);
  opacity: 0;
  transition:
    opacity var(--dur),
    color var(--dur);
}
.gl-list li:hover .gl-x,
.gl-x:focus-visible {
  opacity: 1;
}
.gl-x:hover {
  color: var(--a-red);
}
.gl-add {
  height: 38px;
  padding: 0 12px;
  border: 0;
  border-radius: var(--r-s);
  background: var(--bg-2);
  outline: none;
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-primary);
}
.gl-add:focus-visible {
  box-shadow: 0 0 0 2px var(--ring);
}
.gl-add::placeholder {
  color: var(--tx-muted);
}
@media (hover: none) {
  .gl-x {
    opacity: 1;
  }
}

/* Dialog */
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
