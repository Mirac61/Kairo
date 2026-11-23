<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
  type Habit,
  type Project,
} from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import { ymd } from '@/lib/dates'
import PageHeader from '@/components/PageHeader.vue'
import IconButton from '@/components/IconButton.vue'
import SegTabs from '@/components/SegTabs.vue'
import GoalCard from './components/GoalCard.vue'
import GoalEditDialog from './components/GoalEditDialog.vue'

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

// --- Filter & Zähler
const today = ymd(new Date())
const tab = ref<'active' | 'reached'>('active')

const displayedGoals = computed(() => (tab.value === 'active' ? longTerm.value : reached.value))

// --- Aktionen
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

const archive = (g: Goal, value: boolean) => void run(() => updateGoal(g.id, { archived: value }))

function handleAddStep(g: Goal, title: string) {
  void run(async () => {
    await createGoal({ title, parent_id: g.id })
    await load()
  })
}

function handleDeleteStep(s: Goal) {
  void run(async () => {
    await deleteGoal(s.id)
    await dropImage(s.image_path)
    await load()
  })
}

// --- Dialog
const dialog = ref(false)
const editingGoal = ref<Goal | null>(null)
const initialParentId = ref('')

function openNew(parent = '') {
  editingGoal.value = null
  initialParentId.value = parent
  dialog.value = true
}

function openEdit(g: Goal) {
  editingGoal.value = g
  initialParentId.value = ''
  dialog.value = true
}

function handleSaveGoal(payload: {
  id?: string
  body: Partial<Goal>
  file: File | null
  steps: { title: string; due_date: string }[]
}) {
  void run(async () => {
    const { id, body, file, steps } = payload
    const g = id ? await updateGoal(id, body) : await createGoal(body)
    for (const s of steps) await createGoal({ ...s, parent_id: g.id })
    const old = id ? (goals.value.find((x) => x.id === id)?.image_path ?? null) : null
    if (file) {
      await updateGoal(g.id, { image_path: await uploadImage(g.id, file) })
    }
    if (old && old !== (file ? null : body.image_path)) {
      await dropImage(old)
    }
    dialog.value = false
    await load()
  })
}

function handleDeleteGoal(g: Goal) {
  void run(async () => {
    const images = [g.image_path, ...stepsOf(g.id).map((s) => s.image_path)]
    await deleteGoal(g.id)
    await Promise.all(images.map(dropImage))
    dialog.value = false
    await load()
  })
}

function handleToggleArchiveGoal(g: Goal, value: boolean) {
  archive(g, value)
  dialog.value = false
}
</script>

<template>
  <div class="view-inner gl">
    <PageHeader :title="$t('Ziele')">
      <SegTabs
        v-model="tab"
        :label="$t('Filter')"
        :options="[
          { value: 'active', label: $t('Aktiv'), count: longTerm.length },
          {
            value: 'reached',
            label: $t('Erreicht'),
            count: reached.length,
            disabled: !reached.length,
          },
        ]"
      />
      <template #actions>
        <IconButton icon="plus" :label="$t('Neues Ziel')" @click="openNew()" />
      </template>
    </PageHeader>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>

    <p v-if="!goals.length" class="v-sub gl-empty">
      {{
        $t(
          'Noch keine Ziele. Leg ein großes an, gib ihm eine Frist und verknüpfe Schritte, ein Projekt oder eine Gewohnheit. Dann zeigt Kairo, ob du im Plan bist.',
        )
      }}
    </p>

    <div class="gl-grid">
      <GoalCard
        v-for="g in displayedGoals"
        :key="g.id"
        :goal="g"
        :steps="stepsOf(g.id)"
        :today="today"
        @edit="openEdit"
        @archive="archive"
        @add-step="handleAddStep"
        @delete-step="handleDeleteStep"
      />
    </div>

    <GoalEditDialog
      :open="dialog"
      :goal="editingGoal"
      :initial-parent-id="initialParentId"
      :goals="goals"
      :projects="projects"
      :habits="habits"
      @close="dialog = false"
      @save="handleSaveGoal"
      @delete="handleDeleteGoal"
      @toggle-archive="handleToggleArchiveGoal"
    />
  </div>
</template>

<style scoped>
.gl-empty {
  max-width: 560px;
}

.gl-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 440px), 1fr));
  align-items: start; /* eine aufgeklappte Karte zieht die Nachbarin nicht mit */
  gap: 14px;
}
</style>
