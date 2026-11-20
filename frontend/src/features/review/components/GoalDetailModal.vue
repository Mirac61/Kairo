<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Goal, Habit, Project } from '@/api/client'
import { vDialog } from '@/lib/dialog'
import { goalPercent } from '../goalUtils'
import GoalCoverBanner from './GoalCoverBanner.vue'
import GoalInspectorRail from './GoalInspectorRail.vue'

const props = defineProps<{
  goal: Goal | null
  steps: Goal[]
  projects: Project[]
  habits: Habit[]
  weeklyText: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saveWhy', goal: Goal, why: string): void
  (e: 'toggleStep', step: Goal, archived: boolean): void
  (e: 'addStep', goal: Goal, title: string): void
  (e: 'deleteStep', step: Goal): void
  (e: 'uploadMedia', goal: Goal, file: File): void
  (e: 'clearMedia', goal: Goal): void
  (e: 'edit', goal: Goal): void
  (e: 'archive', goal: Goal, archived: boolean): void
  (e: 'delete', goal: Goal): void
}>()

const whyDraft = ref('')
const whyDirty = ref(false)
const newStep = ref('')
const moodIdx = ref(0)
const hideCover = ref(false)

// Nur bei Ziel-Wechsel (id) komplett zurücksetzen, damit Live-Events / Schritt-Klicks keine ungespeicherten Notizen löschen.
watch(
  () => props.goal?.id,
  () => {
    whyDraft.value = props.goal?.why ?? ''
    whyDirty.value = false
    newStep.value = ''
    hideCover.value = false
  },
  { immediate: true },
)

watch(
  () => props.goal?.why,
  (nextWhy) => {
    if (!whyDirty.value) whyDraft.value = nextWhy ?? ''
  },
)

function flushWhy() {
  if (!props.goal) return
  whyDirty.value = false
  if (whyDraft.value !== (props.goal.why ?? '')) {
    emit('saveWhy', props.goal, whyDraft.value)
  }
}

function handleClose() {
  flushWhy()
  emit('close')
}

const progressPercent = computed(() => (props.goal ? goalPercent(props.goal, props.steps) : 0))
const doneStepsCount = computed(() => props.steps.filter((s) => s.archived).length)
const projectName = computed(() => {
  const id = props.goal?.project_id
  return id ? (props.projects.find((p) => p.id === id)?.name ?? '—') : '—'
})
const habitName = computed(() => {
  const id = props.goal?.habit_id
  return id ? (props.habits.find((h) => h.id === id)?.name ?? '—') : '—'
})

function submitStep() {
  if (!props.goal) return
  const title = newStep.value.trim()
  if (!title) return
  newStep.value = ''
  emit('addStep', props.goal, title)
}

function handleUpload(file: File) {
  if (!props.goal) return
  hideCover.value = false
  emit('uploadMedia', props.goal, file)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="goal"
      v-dialog="handleClose"
      class="gdm-backdrop"
      role="presentation"
      @mousedown.self="handleClose"
    >
      <div class="dialog gdm-sheet" :aria-label="goal.title">
        <button type="button" class="gdm-close" :aria-label="$t('Schließen')" @click="handleClose">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
          >
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        <GoalCoverBanner
          v-if="!hideCover"
          :image-path="goal.image_path"
          :mood-idx="moodIdx"
          @upload="handleUpload"
          @clear="emit('clearMedia', goal)"
          @next-mood="moodIdx = (moodIdx + 1) % 3"
          @hide="hideCover = true"
        />

        <div class="gdm-body">
          <div class="gdm-main">
            <div class="gdm-head">
              <div class="gdm-title-row">
                <h2 class="gdm-title">{{ goal.title }}</h2>
                <button
                  v-if="hideCover"
                  type="button"
                  class="gdm-show-cover"
                  @click="hideCover = false"
                >
                  + {{ $t('Bild / Video') }}
                </button>
              </div>
              <div class="gdm-prog">
                <div class="gdm-prog-track">
                  <div class="gdm-prog-fill" :style="{ width: `${progressPercent}%` }"></div>
                </div>
                <span class="gdm-prog-pct">{{ progressPercent }} %</span>
              </div>
            </div>

            <div class="gdm-sec">
              <label for="gdm-new-step" class="gdm-sec-lbl">{{ $t('Meilensteine') }}</label>
              <div v-if="steps.length" class="gdm-steps">
                <div v-for="s in steps" :key="s.id" class="gdm-step">
                  <button
                    type="button"
                    class="gdm-step-btn"
                    :class="{ 'is-done': s.archived }"
                    :aria-pressed="s.archived"
                    @click="emit('toggleStep', s, !s.archived)"
                  >
                    <span class="gdm-chk" aria-hidden="true">✓</span>
                    <span class="gdm-step-txt">{{ s.title }}</span>
                  </button>
                  <button
                    type="button"
                    class="gdm-step-del"
                    :aria-label="$t('Löschen')"
                    @click="emit('deleteStep', s)"
                  >
                    ×
                  </button>
                </div>
              </div>
              <form class="gdm-add-step" @submit.prevent="submitStep">
                <span class="gdm-plus" aria-hidden="true">+</span>
                <input
                  id="gdm-new-step"
                  v-model="newStep"
                  type="text"
                  class="gdm-step-inp"
                  :placeholder="$t('Neuen Schritt hinzufügen …')"
                />
              </form>
            </div>

            <div class="gdm-sec gdm-notes-sec">
              <label for="gdm-notes" class="gdm-sec-lbl">{{ $t('Notizen') }}</label>
              <textarea
                id="gdm-notes"
                v-model="whyDraft"
                rows="6"
                class="gdm-notes"
                :placeholder="$t('Gedanken, Strategie, Links oder Warum zu diesem Ziel …')"
                @input="whyDirty = true"
                @blur="flushWhy"
              ></textarea>
            </div>
          </div>

          <GoalInspectorRail
            :goal="goal"
            :progress-percent="progressPercent"
            :done-steps-count="doneStepsCount"
            :total-steps-count="steps.length"
            :weekly-text="weeklyText"
            :project-name="projectName"
            :habit-name="habitName"
            @edit="emit('edit', $event)"
            @archive="(g, v) => emit('archive', g, v)"
            @delete="emit('delete', $event)"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.gdm-backdrop {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: var(--scrim);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.gdm-sheet {
  position: relative;
  width: min(980px, 84vw);
  max-height: 86vh;
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  border-radius: 16px;
  box-shadow: var(--shadow-pop);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.gdm-close {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 4;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(14, 14, 18, 0.72);
  color: #f4f3f0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  backdrop-filter: blur(6px);
}
.gdm-close:hover {
  background: rgba(14, 14, 18, 0.9);
  color: #ffffff;
}
.gdm-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow-y: auto;
}
.gdm-main {
  flex: 1;
  min-width: 0;
  padding: 28px 36px 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.gdm-title-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.gdm-title {
  margin: 0;
  font: 600 22px/1.25 var(--font-ui);
  letter-spacing: -0.02em;
  color: var(--tx-primary);
}
.gdm-show-cover {
  border: 0;
  background: none;
  color: var(--tx-muted);
  font: 500 12px/1 var(--font-ui);
  cursor: pointer;
  white-space: nowrap;
}
.gdm-show-cover:hover {
  color: var(--tx-primary);
}
.gdm-prog {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
}
.gdm-prog-track {
  flex: 1;
  height: 4px;
  border-radius: 99px;
  background: var(--bg-2);
  overflow: hidden;
}
.gdm-prog-fill {
  height: 100%;
  border-radius: 99px;
  background: var(--a-blue-hi);
  transition: width 260ms var(--ease);
}
.gdm-prog-pct {
  font: 500 12px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--tx-secondary);
}
.gdm-sec {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.gdm-sec-lbl {
  font: 500 11px/1 var(--font-ui);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--tx-muted);
}
.gdm-steps {
  display: flex;
  flex-direction: column;
}
.gdm-step {
  display: flex;
  align-items: center;
  gap: 6px;
  border-bottom: 1px solid var(--br-subtle);
}
.gdm-step-btn {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 2px;
  border: 0;
  background: none;
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  text-align: left;
  cursor: pointer;
}
.gdm-chk {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1.5px solid var(--br-strong);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: transparent;
  flex-shrink: 0;
}
.gdm-step-btn.is-done .gdm-chk {
  background: var(--a-blue-hi);
  border-color: var(--a-blue-hi);
  color: var(--tx-on-accent);
}
.gdm-step-btn.is-done .gdm-step-txt {
  color: var(--tx-muted);
  text-decoration: line-through;
}
.gdm-step-del {
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--tx-muted);
  font-size: 16px;
  cursor: pointer;
  opacity: 0;
  transition: opacity 140ms ease;
}
.gdm-step:hover .gdm-step-del,
.gdm-step-del:focus-visible {
  opacity: 1;
}
@media (hover: none) {
  .gdm-step-del {
    opacity: 1;
  }
}
.gdm-step-del:hover {
  background: var(--bg-2);
  color: var(--a-red);
}
.gdm-add-step {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 2px;
}
.gdm-plus {
  width: 16px;
  text-align: center;
  color: var(--tx-muted);
}
.gdm-step-inp {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  color: var(--tx-primary);
  font: 400 13.5px/1.4 var(--font-ui);
  padding: 4px 0;
}
.gdm-step-inp:focus-visible {
  outline: none;
  border-bottom: 1.5px solid var(--a-focus);
}
.gdm-notes {
  width: 100%;
  min-height: 120px;
  padding: 10px 12px;
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-s);
  background: var(--bg-0);
  color: var(--tx-primary);
  font: 400 13.5px/1.6 var(--font-ui);
  resize: vertical;
}
.gdm-notes:focus-visible {
  outline: none;
  border-color: var(--a-focus);
}
@media (max-width: 780px) {
  .gdm-backdrop {
    padding: 10px;
  }
  .gdm-sheet {
    width: 100%;
    max-height: 92vh;
  }
  .gdm-body {
    flex-direction: column;
  }
  .gdm-main {
    padding: 20px;
  }
}
</style>
