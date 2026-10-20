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
import { habitColor } from '@/lib/projectColor'
import { locale, t } from '@/lib/i18n'
import DeleteButton from '@/components/DeleteButton.vue'

const KINDS: Record<GoalKind, string> = {
  NONE: t('Ohne'), PROJECT_TASKS: t('Aufgaben'), PROJECT_HOURS: t('Stunden'), HABIT_COUNT: t('Gewohnheit'),
}
const IMAGE_DIR = 'Ziele'
// Langzeitziele wechseln ihre Anordnung, damit das Board nicht wie eine Tabelle aussieht.
const LAYOUTS = ['split', 'overlay', 'split rev'] as const
// Teilziele mit Bild bekommen unterschiedliche Formate (stabil je Ziel).
const ASPECTS = ['4 / 3', '1 / 1', '5 / 4', '16 / 10']

const goals = ref<Goal[]>([])
const projects = ref<Project[]>([])
const habits = ref<Habit[]>([])
const longTerm = computed(() => goals.value.filter((g) => !g.parent_id && !g.archived))
const reached = computed(() => goals.value.filter((g) => !g.parent_id && g.archived))
const stepsOf = (id: string) =>
  goals.value.filter((g) => g.parent_id === id).sort((a, b) => Number(a.archived) - Number(b.archived))

const { error, load, run } = useLoader(async () => {
  const [gs, ps, hs] = await Promise.all([listGoals(), listProjects(), listHabits()])
  goals.value = gs
  projects.value = ps.filter((p) => p.status !== 'ARCHIVED')
  habits.value = hs.filter((h) => h.active)
})
onMounted(load)
useLiveEvents(load)

const hash = (s: string) => [...s].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7)
const accent = (g: Goal) => habitColor(g.id)
const aspect = (g: Goal) => ASPECTS[hash(g.id) % ASPECTS.length]
const layout = (g: Goal, i: number) => (g.image_path ? LAYOUTS[i % LAYOUTS.length] : 'quote')

// --- Dialog
const dialog = ref(false)
const empty = (parent = '') => ({
  id: '', title: '', why: '', kind: 'NONE' as GoalKind, project: '', habit: '', target: 0, parent,
  start: ymd(new Date()), due: '', image: null as string | null, file: null as File | null, archived: false,
})
const form = ref(empty())
const preview = computed(() => (form.value.file ? URL.createObjectURL(form.value.file) : form.value.image && filesUrl(form.value.image)))
// Ein Langzeitziel mit Teilzielen bleibt oben; als Eltern taugen nur andere Langzeitziele.
const hasSteps = computed(() => !!form.value.id && goals.value.some((g) => g.parent_id === form.value.id))
const parents = computed(() => goals.value.filter((g) => !g.parent_id && g.id !== form.value.id))

function openNew(parent = '') {
  form.value = empty(parent)
  dialog.value = true
}
function openEdit(g: Goal) {
  form.value = {
    id: g.id, title: g.title, why: g.why, kind: g.kind, project: g.project_id ?? '', habit: g.habit_id ?? '', target: g.target,
    parent: g.parent_id ?? '', start: g.start_date, due: g.due_date ?? '', image: g.image_path, file: null, archived: g.archived,
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
      title: f.title, why: f.why, kind: f.kind, project_id: f.project, habit_id: f.habit, parent_id: f.parent,
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
// Löschen nimmt die Teilziele mit (Backend), ihre Bilder räumen wir hier weg.
const remove = () =>
  void run(async () => {
    const f = form.value
    const images = [f.image, ...stepsOf(f.id).map((s) => s.image_path)]
    await deleteGoal(f.id)
    await Promise.all(images.map(dropImage))
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
  <div class="view-inner gl">
    <div class="v-head v-head-row">
      <div>
        <h1 class="v-title">{{ $t('Ziele') }}</h1>
        <div class="v-sub">{{ $t('Wofür du das alles machst.') }}</div>
      </div>
      <button class="btn btn-primary" type="button" @click="openNew()"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neues Langzeitziel') }}</button>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <p v-if="!goals.length" class="gl-empty">
      {{ $t('Noch keine Ziele. Fang mit einem großen an: Wo willst du in ein, zwei Jahren stehen? Gib ihm ein Bild und einen Satz, warum es dir wichtig ist. Darunter legst du die Schritte an, die dich hinbringen.') }}
    </p>

    <section v-for="(g, i) in longTerm" :key="g.id" class="gl-block" :style="{ '--gc': accent(g) }">
      <!-- Langzeitziel -->
      <article class="gl-hero" :class="layout(g, i)">
        <div v-if="g.image_path" class="gl-hero-img"><img :src="filesUrl(g.image_path)" alt="" /></div>
        <div class="gl-hero-text">
          <span class="gl-kicker">{{ $t('Langzeitziel') }}<template v-if="g.due_date"> · {{ dueText(g.due_date) }}</template></span>
          <h2 class="gl-hero-title">{{ g.title }}</h2>
          <blockquote v-if="g.why" class="gl-hero-why">„{{ g.why }}“</blockquote>
          <div v-if="g.progress" class="gl-hero-prog">
            <div class="gl-ring" role="progressbar" :aria-valuenow="g.progress.percent" aria-valuemin="0" aria-valuemax="100" :aria-label="g.title"
              :style="{ '--p': g.progress.percent }"></div>
            <span><b>{{ g.progress.percent }} %</b> · {{ goalProgressText(g) }}</span>
          </div>
          <div class="gl-hero-act">
            <button type="button" class="btn btn-secondary" @click="openNew(g.id)"><svg class="ic"><use href="#i-plus" /></svg>{{ $t('Teilziel') }}</button>
            <button type="button" class="btn btn-ghost" @click="openEdit(g)">{{ $t('Bearbeiten') }}</button>
          </div>
        </div>
      </article>

      <!-- Teilziele -->
      <div v-if="stepsOf(g.id).length" class="gl-steps">
        <span class="gl-steps-head">{{ $t('Der Weg dahin') }}</span>
        <div class="gl-masonry">
          <article v-for="s in stepsOf(g.id)" :key="s.id" class="gl-step" :class="{ done: s.archived }">
            <button type="button" class="gl-step-open" :aria-label="$t('„{title}“ bearbeiten', { title: s.title })" @click="openEdit(s)">
              <img v-if="s.image_path" class="gl-step-img" :src="filesUrl(s.image_path)" alt="" :style="{ aspectRatio: aspect(s) }" />
              <span class="gl-step-body">
                <b class="gl-step-title">{{ s.title }}</b>
                <q v-if="s.why" class="gl-step-why">{{ s.why }}</q>
                <template v-if="s.progress && !s.archived">
                  <span class="pbar"><span class="pbar-fill" :style="{ width: s.progress.percent + '%' }"></span></span>
                  <small>{{ s.progress.percent }} % · {{ goalProgressText(s) }}</small>
                </template>
                <small v-if="s.due_date && !s.archived">{{ dueText(s.due_date) }}</small>
              </span>
            </button>
            <button type="button" class="gl-check" role="checkbox" :aria-checked="s.archived"
              :aria-label="s.archived ? $t('„{title}“ wieder öffnen', { title: s.title }) : $t('„{title}“ erreicht', { title: s.title })"
              @click="archive(s, !s.archived)"><svg v-if="s.archived" viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg></button>
          </article>
        </div>
      </div>
      <button v-else type="button" class="gl-steps-empty" @click="openNew(g.id)">
        {{ $t('Was ist der erste Schritt dahin? Leg ein Teilziel an.') }}
      </button>
    </section>

    <template v-if="reached.length">
      <h2 class="col-title gl-arch-head">{{ $t('Erreicht') }}</h2>
      <div class="gl-arch">
        <button v-for="g in reached" :key="g.id" type="button" class="gl-arch-chip" :style="{ '--gc': accent(g) }" @click="openEdit(g)">
          <svg viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>{{ g.title }}
        </button>
      </div>
    </template>

    <div v-if="dialog" v-dialog="() => (dialog = false)" class="overlay open" @mousedown.self="dialog = false">
      <form class="dialog" :aria-label="form.id ? $t('Ziel bearbeiten') : $t('Neues Ziel')" @submit.prevent="save">
        <div class="dlg-head">
          <h3>{{ form.id ? $t('Ziel bearbeiten') : form.parent ? $t('Neues Teilziel') : $t('Neues Langzeitziel') }}</h3>
          <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="dialog = false"><svg class="ic"><use href="#i-x" /></svg></button>
        </div>
        <div class="dlg-body">
          <div class="field"><label for="g-title">{{ $t('Ziel') }}</label><input id="g-title" v-model="form.title" class="input" :placeholder="$t('z. B. Bachelor abschließen')" autofocus required /></div>
          <div class="field"><label for="g-why">{{ $t('Warum?') }}</label><textarea id="g-why" v-model="form.why" class="input" rows="2" :placeholder="$t('Ein Satz, der dich erinnert, wofür es sich lohnt')"></textarea></div>
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
          <div v-if="form.kind === 'NONE' && !form.parent" class="v-sub">{{ $t('Ohne eigene Messung zeigt ein Langzeitziel, wie viele seiner Teilziele erreicht sind.') }}</div>
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
          <DeleteButton v-if="form.id" ghost :text="hasSteps ? $t('„{title}“ samt Teilzielen löschen?', { title: form.title }) : $t('„{title}“ löschen?', { title: form.title })" @confirm="remove" />
          <button v-if="form.id" class="btn btn-ghost" type="button"
            @click="archive(goals.find((g) => g.id === form.id)!, !form.archived); dialog = false">{{ form.archived ? $t('Wieder öffnen') : $t('Erreicht') }}</button>
          <span class="gl-spacer"></span>
          <button class="btn btn-ghost" type="button" @click="dialog = false">{{ $t('Abbrechen') }}</button>
          <button class="btn btn-primary" type="submit">{{ form.id ? $t('Speichern') : $t('Anlegen') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.gl { --serif: 'Iowan Old Style', 'Palatino Linotype', Palatino, Georgia, ui-serif, serif; }
.gl-empty { max-width: 560px; font: italic 400 17px/1.6 var(--serif); color: var(--tx-secondary); }

/* Langzeitziel */
.gl-block { margin-bottom: 56px; }
.gl-hero {
  display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); align-items: stretch; overflow: hidden;
  border-radius: 18px; border: 1px solid color-mix(in srgb, var(--gc) 25%, var(--br-default));
  background: linear-gradient(135deg, color-mix(in srgb, var(--gc) 14%, var(--bg-1)), var(--bg-1) 60%);
}
.gl-hero.rev { grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr); }
.gl-hero.rev .gl-hero-img { order: 2; }
.gl-hero-img { min-height: 300px; }
.gl-hero-img img { display: block; width: 100%; height: 100%; object-fit: cover; }
.gl-hero-text { display: grid; align-content: center; gap: 14px; padding: 36px 40px; }
.gl-kicker { font: 600 11px/1 var(--font-ui); letter-spacing: 0.12em; text-transform: uppercase; color: var(--gc); }
.gl-hero-title { margin: 0; font: 600 30px/1.15 var(--font-ui); letter-spacing: -0.02em; color: var(--tx-primary); }
.gl-hero-why { margin: 0; font: italic 400 19px/1.5 var(--serif); color: var(--tx-secondary); }
.gl-hero-prog { display: flex; align-items: center; gap: 12px; font: 400 13px/1.3 var(--font-ui); color: var(--tx-muted); }
.gl-hero-prog b { font-weight: 600; color: var(--tx-primary); }
.gl-ring {
  flex: none; width: 40px; height: 40px; border-radius: 50%;
  background: conic-gradient(var(--gc) calc(var(--p) * 1%), color-mix(in srgb, var(--gc) 18%, transparent) 0);
  -webkit-mask: radial-gradient(farthest-side, transparent calc(100% - 5px), #000 calc(100% - 4px));
  mask: radial-gradient(farthest-side, transparent calc(100% - 5px), #000 calc(100% - 4px));
}
.gl-hero-act { display: flex; gap: 8px; margin-top: 6px; }

/* Bild füllt alles, Text liegt unten darüber */
.gl-hero.overlay { position: relative; grid-template-columns: 1fr; min-height: 440px; background: var(--bg-0); }
.gl-hero.overlay .gl-hero-img { position: absolute; inset: 0; }
.gl-hero.overlay .gl-hero-text {
  position: relative; align-self: end; padding: 140px 40px 36px;
  background: linear-gradient(180deg, transparent, color-mix(in srgb, var(--bg-0) 90%, transparent) 50%);
}
.gl-hero.overlay .gl-hero-why { max-width: 48ch; }

/* Ohne Bild: großes Zitat auf Farbe */
.gl-hero.quote { grid-template-columns: 1fr; background: linear-gradient(120deg, color-mix(in srgb, var(--gc) 30%, var(--bg-1)), var(--bg-1) 75%); }
.gl-hero.quote .gl-hero-text { padding: 48px 48px 40px; }
.gl-hero.quote .gl-hero-why { font-size: 26px; line-height: 1.4; color: var(--tx-primary); max-width: 34ch; }

/* Teilziele: Collage mit unterschiedlichen Höhen, links an einer Linie aufgehängt */
.gl-steps { margin: 0 0 0 28px; padding: 22px 0 0 24px; border-left: 2px solid color-mix(in srgb, var(--gc) 40%, transparent); }
.gl-steps-head { display: block; margin-bottom: 12px; font: 600 11px/1 var(--font-ui); letter-spacing: 0.12em; text-transform: uppercase; color: var(--tx-muted); }
.gl-masonry { columns: 3 220px; column-gap: 14px; }
.gl-step {
  position: relative; break-inside: avoid; margin-bottom: 14px; overflow: hidden; border-radius: 12px;
  border: 1px solid var(--br-default); background: var(--bg-1);
}
.gl-step:nth-child(3n + 2) { background: color-mix(in srgb, var(--gc) 9%, var(--bg-1)); }
.gl-step-open { display: block; width: 100%; padding: 0; border: 0; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.gl-step-img { display: block; width: 100%; object-fit: cover; }
.gl-step-body { display: grid; gap: 8px; padding: 14px 44px 14px 16px; }
.gl-step-title { font: 600 15px/1.35 var(--font-ui); color: var(--tx-primary); }
.gl-step-why { font: italic 400 14px/1.45 var(--serif); color: var(--tx-secondary); }
.gl-step small { font: 400 12px/1.2 var(--font-ui); color: var(--tx-muted); }
.gl-step .pbar { display: block; }
.gl-step .pbar-fill { display: block; background: var(--gc); }
.gl-check {
  position: absolute; right: 12px; bottom: 12px; display: grid; place-items: center; width: 22px; height: 22px; padding: 0;
  border-radius: 50%; border: 1.5px solid color-mix(in srgb, var(--gc) 60%, var(--br-default)); background: var(--bg-1); cursor: pointer;
}
.gl-check svg { width: 14px; height: 14px; fill: none; stroke: var(--bg-0); stroke-width: 2.4; }
.gl-step.done .gl-check { background: var(--gc); border-color: var(--gc); }
.gl-step.done { opacity: 0.6; }
.gl-step.done .gl-step-title { text-decoration: line-through; text-decoration-color: var(--gc); }
.gl-steps-empty {
  display: block; margin: 0 0 0 28px; padding: 18px 0 0 24px; border: 0; border-left: 2px dashed color-mix(in srgb, var(--gc) 40%, transparent);
  background: none; color: var(--tx-muted); font: italic 400 15px/1.5 var(--serif); text-align: left; cursor: pointer;
}
.gl-steps-empty:hover { color: var(--tx-primary); }

/* Erreicht */
.gl-arch-head { margin: 8px 0 12px; }
.gl-arch { display: flex; flex-wrap: wrap; gap: 8px; }
.gl-arch-chip {
  display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px; border-radius: 999px; cursor: pointer;
  border: 1px solid color-mix(in srgb, var(--gc) 35%, transparent); background: color-mix(in srgb, var(--gc) 10%, transparent);
  color: var(--tx-primary); font: 500 13px/1 var(--font-ui);
}
.gl-arch-chip svg { width: 14px; height: 14px; fill: none; stroke: var(--gc); stroke-width: 2.4; }

/* Dialog */
.gl-preview { display: block; width: 100%; max-height: 160px; object-fit: cover; border-radius: var(--r-s); margin-bottom: 6px; }
.gl-img-row { display: flex; gap: 8px; align-items: center; }
.gl-img-row .input { flex: 1; min-width: 0; }
.dlg-row { display: flex; gap: 12px; }
.dlg-row .field { flex: 1; }
.dlg-foot .gl-spacer { flex: 1; }

@media (max-width: 760px) {
  .gl-hero, .gl-hero.rev { grid-template-columns: 1fr; }
  .gl-hero.rev .gl-hero-img { order: 0; }
  .gl-hero-img { min-height: 0; aspect-ratio: 16 / 10; }
  .gl-hero-text, .gl-hero.quote .gl-hero-text { padding: 22px 20px; }
  .gl-hero.overlay { min-height: 380px; }
  .gl-hero.overlay .gl-hero-img { aspect-ratio: auto; }
  .gl-hero.overlay .gl-hero-text { padding: 100px 20px 22px; }
  .gl-hero-title { font-size: 24px; }
  .gl-hero.quote .gl-hero-why { font-size: 21px; }
  .gl-steps, .gl-steps-empty { margin-left: 12px; padding-left: 14px; }
  .gl-block { margin-bottom: 40px; }
}
</style>
