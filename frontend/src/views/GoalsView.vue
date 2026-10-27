<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import {
  ApiError, createGoal, createNote, deleteGoal, deleteNote, listGoals, listHabits, listProjects, updateGoal, uploadFile,
  type Goal, type GoalKind, type Habit, type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { vDialog } from '@/lib/dialog'
import { addDays, weekStart, ymd } from '@/lib/dates'
import { prefs } from '@/lib/prefs'
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

// --- Board
const today = ymd(new Date())
const headDate = computed(() => new Intl.DateTimeFormat(locale.value, { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date()))
// ISO-Kalenderwoche
const kw = (() => {
  const n = new Date()
  const d = new Date(Date.UTC(n.getFullYear(), n.getMonth(), n.getDate()))
  d.setUTCDate(d.getUTCDate() + 4 - (d.getUTCDay() || 7))
  return Math.ceil(((d.getTime() - Date.UTC(d.getUTCFullYear(), 0, 1)) / 864e5 + 1) / 7)
})()
const daysTo = (due: string) => Math.round((new Date(due + 'T00:00').getTime() - new Date(today + 'T00:00').getTime()) / 864e5)
const shortDate = (d: string) => new Intl.DateTimeFormat(locale.value, { day: '2-digit', month: '2-digit', year: 'numeric' }).format(new Date(d + 'T00:00'))
const nextStep = (g: Goal) => stepsOf(g.id).find((s) => !s.archived)

// Drangeblieben: die Tage dieser Woche zum Abhaken, gespeichert in den Einstellungen (geräteübergreifend).
const week = computed(() => {
  const on = new Set(prefs.goalDays.split(' '))
  return Array.from({ length: 7 }, (_, i) => {
    const day = addDays(weekStart(), i)
    const d = ymd(day)
    return { d, label: new Intl.DateTimeFormat(locale.value, { weekday: 'short' }).format(day).slice(0, 2), on: on.has(d) }
  })
})
// Ältere Wochen fallen beim Umschalten weg, damit die Einstellungen klein bleiben.
function toggleDay(d: string) {
  prefs.goalDays = week.value.filter((x) => x.on !== (x.d === d)).map((x) => x.d).join(' ')
}

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
    acceptLabel: t('Löschen'), rejectLabel: t('Abbrechen'), defaultFocus: 'reject',
    acceptProps: { size: 'small' }, rejectProps: { severity: 'secondary', size: 'small', text: true },
    accept: () => void run(async () => {
      await deleteGoal(s.id)
      await dropImage(s.image_path)
      await load()
    }),
  })
}

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

</script>

<template>
  <div class="view-inner gl">
    <header class="gl-head">
      <div class="gl-head-l">
        <span class="gl-date">{{ headDate }} · {{ $t('KW') }} {{ kw }}</span>
        <h1 class="gl-title">{{ $t('Wofür ich das') }} <em>{{ $t('alles') }}</em> {{ $t('mache.') }}</h1>
      </div>
      <label class="gl-focus gl-tilt">
        <span class="gl-label">{{ $t('Fokus diese Woche') }}</span>
        <input v-model.lazy="prefs.goalFocus" :placeholder="$t('Was zählt diese Woche?')" maxlength="200" />
      </label>
    </header>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <p v-if="!goals.length" class="gl-empty">
      {{ $t('Noch keine Ziele. Fang mit einem großen an: Wo willst du in ein, zwei Jahren stehen? Gib ihm ein Bild und einen Satz, warum es dir wichtig ist. Darunter legst du die Schritte an, die dich hinbringen.') }}
    </p>

    <div class="gl-wall">
      <!-- Je Langzeitziel: Bild plus Notizkarte; jede zweite Reihe eingerückt, mit Countdown links -->
      <section v-for="(g, i) in longTerm" :key="g.id" class="gl-row" :class="i % 2 ? 'b' : 'a'">
        <div v-if="i % 2 && g.due_date && daysTo(g.due_date) >= 0" class="gl-count gl-tilt">
          <span class="gl-count-s">{{ $t('Noch') }}</span>
          <b>{{ daysTo(g.due_date) }}</b>
          <span class="gl-count-l">{{ $t('Tage bis „{title}“', { title: g.title }) }}</span>
          <span class="gl-count-s">≈ {{ $t('{n} Wochen', { n: Math.round(daysTo(g.due_date) / 7) }) }}</span>
        </div>

        <button type="button" class="gl-pic" :class="{ bare: !g.image_path }" :aria-label="$t('„{title}“ bearbeiten', { title: g.title })" @click="openEdit(g)">
          <img v-if="g.image_path" :src="filesUrl(g.image_path)" alt="" />
          <span class="gl-pic-text">
            <span class="gl-chips">
              <span class="gl-chip">{{ String(i + 1).padStart(2, '0') }} · {{ $t('Langzeitziel') }}</span>
              <span v-if="g.due_date" class="gl-chip soft">{{ $t('bis {date}', { date: shortDate(g.due_date) }) }}</span>
            </span>
            <span class="gl-pic-title">{{ g.title }}</span>
          </span>
        </button>

        <div class="gl-note gl-tilt">
          <p v-if="g.why" class="gl-why">„{{ g.why }}“</p>
          <div class="gl-prog">
            <div class="gl-segs" aria-hidden="true">
              <template v-if="stepsOf(g.id).length"><span v-for="st in stepsOf(g.id)" :key="st.id" :class="{ on: st.archived }"></span></template>
              <span v-else :style="{ background: `linear-gradient(90deg, var(--gl-a) ${g.progress?.percent ?? 0}%, var(--bg-3) 0)` }"></span>
            </div>
            <span class="gl-muted">{{ g.progress ? `${g.progress.percent} % · ${goalProgressText(g)}` : $t('Noch keine Schritte') }}</span>
          </div>
          <div v-if="nextStep(g)" class="gl-next">
            <span class="gl-label">{{ $t('Als nächstes') }}</span>
            <b>{{ nextStep(g)!.title }}</b>
          </div>
          <p v-else-if="!stepsOf(g.id).length" class="gl-muted gl-hint">{{ $t('Noch kein Schritt. Was geht heute in 20 Minuten?') }}</p>
          <ul v-if="stepsOf(g.id).length" class="gl-list">
            <li v-for="st in stepsOf(g.id)" :key="st.id" :class="{ done: st.archived }">
              <button type="button" class="gl-box" role="checkbox" :aria-checked="st.archived"
                :aria-label="st.archived ? $t('„{title}“ wieder öffnen', { title: st.title }) : $t('„{title}“ erreicht', { title: st.title })"
                @click="archive(st, !st.archived)"><svg v-if="st.archived" viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg></button>
              <button type="button" class="gl-step-t" @click="openEdit(st)">{{ st.title }}<small v-if="st.progress && !st.archived"> · {{ st.progress.percent }} %</small></button>
              <button type="button" class="gl-x" :aria-label="$t('„{title}“ löschen', { title: st.title })" @click="removeStep($event, st)">×</button>
            </li>
          </ul>
          <input v-model="drafts[g.id]" class="gl-add" :placeholder="stepsOf(g.id).length ? $t('+ Schritt hinzufügen') : $t('+ Erster Schritt …')"
            :aria-label="$t('Schritt zu „{title}“ hinzufügen', { title: g.title })" @keydown.enter.prevent="addStep(g)" />
        </div>
      </section>

      <section class="gl-row foot">
        <div class="gl-streak gl-tilt">
          <div class="gl-streak-h"><b>{{ $t('Drangeblieben') }}</b><span class="gl-muted">{{ $t('{n} von 7 Tagen', { n: week.filter((d) => d.on).length }) }}</span></div>
          <div class="gl-days">
            <button v-for="d in week" :key="d.d" type="button" :aria-pressed="d.on" :class="{ today: d.d === today, future: d.d > today }" @click="toggleDay(d.d)">{{ d.label }}</button>
          </div>
          <span class="gl-muted">{{ $t('Tag antippen, wenn du was für deine Ziele getan hast.') }}</span>
        </div>
        <p class="gl-motto gl-tilt">{{ $t('Nicht perfekt.') }}<br /><em>{{ $t('Nur weiter.') }}</em></p>
        <button type="button" class="gl-new gl-tilt" @click="openNew()">
          <b>{{ $t('+ Neues Ziel') }}</b><span>{{ $t('Was kommt als Nächstes an die Wand?') }}</span>
        </button>
      </section>
    </div>

    <template v-if="reached.length">
      <h2 class="col-title gl-arch-head">{{ $t('Erreicht') }}</h2>
      <div class="gl-arch">
        <button v-for="g in reached" :key="g.id" type="button" class="gl-arch-chip" @click="openEdit(g)">
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
.gl {
  --serif: 'Instrument Serif', Georgia, serif;
  --gl-a: var(--a-blue-tx); /* Akzent des Boards, folgt der Akzentfarbe aus den Einstellungen */
  --gl-ink: #12142a; /* Text auf Akzentflächen */
  --tilt: 1; /* Schräglage der Karten wie auf einer Pinnwand */
  max-width: 1320px;
}
.gl-muted { font: 400 13px/1.4 var(--font-ui); color: var(--tx-muted); }
.gl-label { font: 600 13px/1.2 var(--font-ui); color: var(--gl-a); }
.gl-empty { max-width: 560px; font: italic 400 19px/1.5 var(--serif); color: var(--tx-secondary); }
.gl-tilt { transform: rotate(calc(var(--tilt) * var(--r, 0deg))); }

/* Kopf */
.gl-head { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: 28px 24px; align-items: end; margin-bottom: clamp(32px, 4vw, 56px); }
.gl-head-l { grid-column: 1 / 9; display: grid; gap: 14px; min-width: 0; }
.gl-date { font: 400 15px/1 var(--font-ui); color: var(--tx-secondary); }
.gl-title { margin: 0; font: 400 clamp(48px, 6.4vw, 96px)/0.92 var(--serif); letter-spacing: -0.02em; color: var(--tx-primary); text-wrap: balance; }
.gl-title em, .gl-motto em { font-style: italic; color: var(--gl-a); }
.gl-focus { grid-column: 9 / 13; display: grid; gap: 8px; padding: 18px 20px; border-radius: 18px; border: 1px solid var(--br-default); background: var(--bg-1); --r: 1deg; }
.gl-focus input { width: 100%; padding: 0; border: 0; outline: none; background: none; font: italic 400 26px/1.2 var(--serif); color: var(--tx-primary); }
.gl-focus input::placeholder { color: var(--tx-muted); }

/* Wand: je Ziel eine Reihe im 12er-Raster, Notizkarte überlappt das Bild */
.gl-wall { display: grid; gap: clamp(32px, 4vw, 56px); }
.gl-row { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: 24px; align-items: start; }
.gl-row > * { grid-row: 1; }
.gl-row.a .gl-pic { grid-column: 1 / 8; height: clamp(320px, 36vw, 500px); }
.gl-row.a .gl-note { grid-column: 8 / 13; margin: clamp(28px, 4vw, 60px) 0 0 clamp(-56px, -4vw, -20px); --r: 1deg; }
.gl-row.b .gl-pic { grid-column: 4 / 10; height: clamp(300px, 33vw, 450px); }
.gl-row.b .gl-note { grid-column: 9 / 13; margin: clamp(60px, 7vw, 110px) 0 0 clamp(24px, 4vw, 64px); --r: -1.2deg; }

.gl-pic {
  position: relative; z-index: 1; display: block; overflow: hidden; padding: 0; border: 0; border-radius: 24px; cursor: pointer;
  background: var(--bg-2); color: var(--tx-primary); text-align: left; font: inherit;
}
.gl-pic.bare { background: radial-gradient(120% 90% at 0% 0%, color-mix(in srgb, var(--gl-a) 16%, var(--bg-2)), var(--bg-1)); }
.gl-pic img { display: block; width: 100%; height: 100%; object-fit: cover; }
.gl-pic-text {
  position: absolute; inset: auto 0 0; display: grid; gap: 10px; padding: 90px clamp(20px, 2.5vw, 32px) 26px;
  background: linear-gradient(to top, rgba(14, 15, 20, 0.88), rgba(14, 15, 20, 0)); color: #f3f3f6;
}
.gl-pic.bare .gl-pic-text { background: none; color: var(--tx-primary); }
.gl-pic-title { font: 400 clamp(34px, 4vw, 56px)/1 var(--serif); text-wrap: balance; }
.gl-pic:hover .gl-pic-title { text-decoration: underline; text-decoration-thickness: 1px; text-underline-offset: 6px; }
.gl-chips { display: flex; flex-wrap: wrap; gap: 8px; }
.gl-chip { padding: 5px 11px; border-radius: 999px; font: 600 13px/1.2 var(--font-ui); color: var(--gl-ink); background: var(--gl-a); }
.gl-chip.soft { color: #f3f3f6; background: rgba(255, 255, 255, 0.14); }

/* Notizkarte */
.gl-note, .gl-streak {
  position: relative; z-index: 2; display: grid; gap: 16px; padding: 24px; border-radius: 22px;
  border: 1px solid var(--br-default); background: var(--bg-2); box-shadow: 0 30px 60px -24px rgba(0, 0, 0, 0.6);
}
.gl-why { margin: 0; font: italic 400 23px/1.25 var(--serif); color: var(--tx-primary); text-wrap: pretty; }
.gl-prog { display: grid; gap: 8px; }
.gl-segs { display: flex; gap: 4px; height: 6px; }
.gl-segs span { flex: 1; border-radius: 3px; background: var(--bg-3); }
.gl-segs span.on { background: var(--gl-a); }
.gl-next { display: grid; gap: 3px; padding: 14px 16px; border-radius: 14px; background: color-mix(in srgb, var(--gl-a) 12%, transparent); }
.gl-next b { font: 600 18px/1.3 var(--font-ui); color: var(--tx-primary); }
.gl-hint { margin: 0; font-size: 15px; }
.gl-list { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
.gl-list li { display: flex; align-items: flex-start; gap: 12px; }
.gl-box { flex: none; display: grid; place-items: center; width: 20px; height: 20px; margin-top: 1px; padding: 0; border-radius: 6px; border: 1.5px solid var(--gl-a); background: none; cursor: pointer; }
.gl-box svg { width: 14px; height: 14px; fill: none; stroke: var(--gl-ink); stroke-width: 3; }
.done .gl-box { background: var(--gl-a); }
.gl-step-t { flex: 1; min-width: 0; padding: 0; border: 0; background: none; text-align: left; cursor: pointer; font: 400 15px/1.4 var(--font-ui); color: var(--tx-primary); }
.gl-step-t small { color: var(--tx-muted); }
.done .gl-step-t { color: var(--tx-muted); text-decoration: line-through; }
.gl-x { flex: none; padding: 0 4px; border: 0; background: none; cursor: pointer; font: 400 17px/1 var(--font-ui); color: var(--tx-muted); }
.gl-x:hover { color: var(--tx-primary); }
.gl-add { padding: 12px 14px; border-radius: 12px; border: 1px solid var(--br-default); background: var(--bg-1); outline: none; font: 400 15px/1.2 var(--font-ui); color: var(--tx-primary); }
.gl-add:focus { border-color: var(--gl-a); }
.gl-add::placeholder { color: var(--tx-muted); }

/* Countdown-Sticker */
.gl-count {
  grid-column: 1 / 4; margin-top: clamp(32px, 4vw, 56px); --r: -2.2deg;
  position: relative; z-index: 2; display: grid; gap: 2px; padding: 22px 24px; border-radius: 22px;
  color: var(--gl-ink); background: var(--gl-a); box-shadow: 0 24px 40px -20px color-mix(in srgb, var(--gl-a) 40%, transparent);
}
.gl-count b { font: 400 clamp(84px, 8vw, 116px)/0.85 var(--serif); font-variant-numeric: tabular-nums; }
.gl-count-s { font: 600 14px/1.3 var(--font-ui); }
.gl-count-l { font: 600 15px/1.3 var(--font-ui); }

/* Fußreihe */
.gl-row.foot .gl-streak { grid-column: 1 / 6; gap: 14px; padding: 20px 22px; box-shadow: none; --r: 0.8deg; }
.gl-streak-h { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; font: 600 16px/1.2 var(--font-ui); color: var(--tx-primary); }
.gl-days { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 6px; }
.gl-days button { height: 44px; padding: 0; border-radius: 12px; border: 1.5px solid var(--br-strong); background: none; cursor: pointer; font: 600 13px/1 var(--font-ui); color: var(--tx-secondary); }
.gl-days button.future { color: var(--tx-muted); }
.gl-days button.today { border-color: var(--tx-primary); }
.gl-days button[aria-pressed='true'] { border-color: var(--gl-a); background: var(--gl-a); color: var(--gl-ink); }
.gl-motto { grid-column: 6 / 10; align-self: center; margin: 0 0 0 clamp(16px, 3vw, 40px); font: 400 clamp(40px, 4.6vw, 64px)/0.98 var(--serif); color: var(--tx-primary); text-wrap: balance; --r: -1.8deg; }
.gl-new {
  grid-column: 10 / 13; align-self: end; display: grid; justify-items: start; gap: 4px; padding: 22px; border-radius: 22px;
  border: 1.5px dashed var(--br-strong); background: none; cursor: pointer; text-align: left; font: 400 14px/1.4 var(--font-ui); color: var(--tx-secondary);
  transition: border-color var(--dur), color var(--dur); --r: 1.5deg;
}
.gl-new b { font: 600 16px/1.3 var(--font-ui); color: var(--tx-primary); }
.gl-new:hover { border-color: var(--gl-a); color: var(--tx-primary); }

/* Erreicht */
.gl-arch-head { margin-top: 56px; }
.gl-arch { display: flex; flex-wrap: wrap; gap: 8px; }
.gl-arch-chip {
  display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px; border-radius: 999px; cursor: pointer;
  border: 1px solid var(--br-default); background: var(--bg-1); color: var(--tx-primary); font: 500 13px/1 var(--font-ui);
}
.gl-arch-chip svg { width: 14px; height: 14px; fill: none; stroke: var(--gl-a); stroke-width: 2.4; }

/* Dialog */
.gl-preview { display: block; width: 100%; max-height: 160px; object-fit: cover; border-radius: var(--r-s); margin-bottom: 6px; }
.gl-img-row { display: flex; gap: 8px; align-items: center; }
.gl-img-row .input { flex: 1; min-width: 0; }
.dlg-row { display: flex; gap: 12px; }
.dlg-row .field { flex: 1; }
.dlg-foot .gl-spacer { flex: 1; }

@media (max-width: 760px) {
  .gl { --tilt: 0; }
  .gl-head, .gl-row { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .gl-head-l, .gl-focus, .gl-row > *, .gl-row.a .gl-pic, .gl-row.b .gl-pic, .gl-row.a .gl-note, .gl-row.b .gl-note,
  .gl-row.foot .gl-streak, .gl-motto, .gl-new { grid-column: 1; grid-row: auto; margin: 0; }
  .gl-row.a .gl-pic, .gl-row.b .gl-pic { height: 300px; }
  .gl-row.a .gl-note, .gl-row.b .gl-note { margin: -40px 12px 0; }
  .gl-count { display: none; }
  .gl-pic-text { padding-bottom: 56px; }
  .gl-motto { font-size: 40px; }
}
</style>
