<script setup lang="ts">
import { computed, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import type { Goal } from '@/api/client'
import { goalPace, goalProgressText } from '@/lib/goals'
import { locale, t } from '@/lib/i18n'
import { filesUrl } from '@/lib/noteFiles'

const props = defineProps<{
  goal: Goal
  steps: Goal[]
  today: string
}>()

const emit = defineEmits<{
  (e: 'edit', goal: Goal): void
  (e: 'archive', goal: Goal, value: boolean): void
  (e: 'addStep', goal: Goal, title: string): void
  (e: 'deleteStep', step: Goal): void
}>()

const shortDate = (d: string) =>
  new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(
    new Date(d + 'T00:00'),
  )

const STATE = { ok: t('Im Plan'), tight: t('Knapp'), off: t('Hinterher') }

const dayNo = (d: string) => Date.parse(d + 'T00:00:00Z') / 864e5

const chartData = computed(() => {
  const g = props.goal
  const from = dayNo(g.start_date)
  const to = dayNo(g.due_date && g.due_date > g.start_date ? g.due_date : props.today)
  const x = (d: string) =>
    to > from ? Math.min(100, ((dayNo(d) - from) / (to - from)) * 100) : 100
  return {
    line: (g.history ?? []).map((h) => `${x(h.date).toFixed(2)},${100 - h.percent}`).join(' '),
    now: x(props.today),
  }
})

const pace = computed(() => (props.goal.archived ? null : goalPace(props.goal, props.today)))
const next = computed(() => props.steps.find((s) => !s.archived))

const draft = ref('')

function handleAddStep() {
  const title = draft.value.trim()
  if (!title) return
  draft.value = ''
  emit('addStep', props.goal, title)
}

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
    accept: () => {
      emit('deleteStep', s)
    },
  })
}
</script>

<template>
  <article class="card gl-card">
    <header class="gl-card-h">
      <img v-if="goal.image_path" class="gl-thumb" :src="filesUrl(goal.image_path)" alt="" />
      <button type="button" class="gl-card-t" @click="emit('edit', goal)">{{ goal.title }}</button>
      <span v-if="pace" class="gl-state" :class="`gl-${pace.state}`"><i></i>{{ STATE[pace.state] }}</span>
      <span v-else-if="goal.archived" class="gl-state gl-ok"><i></i>{{ $t('Erreicht') }}</span>
      <span v-else-if="!goal.due_date" class="gl-state">{{ $t('Ohne Frist') }}</span>
    </header>
    <p v-if="goal.why" class="gl-why">{{ goal.why }}</p>

    <template v-if="goal.progress">
      <div class="gl-num">
        <b>{{ goal.progress.percent }}%</b>
        <span class="gl-muted">{{ goalProgressText(goal) }}</span>
        <span v-if="pace" class="gl-hint">{{ pace.hint }}</span>
      </div>
      <div
        class="gl-bar"
        role="progressbar"
        :aria-valuenow="goal.progress.percent"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="$t('Fortschritt von „{title}“', { title: goal.title })"
      >
        <i
          :class="pace ? `gl-${pace.state}` : ''"
          :style="{ width: `${goal.progress.percent}%` }"
        ></i>
        <b
          v-if="pace"
          :style="{ left: `${pace.pace}%` }"
          :title="$t('Soll heute: {n} %', { n: pace.pace })"
        ></b>
      </div>
      <figure v-if="goal.history?.length > 1" class="gl-chart">
        <svg
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
          role="img"
          :aria-label="
            pace
              ? $t('Verlauf: {p} % erreicht, Soll heute {s} %', {
                  p: goal.progress.percent,
                  s: pace.pace,
                })
              : $t('Verlauf: {p} % erreicht', { p: goal.progress.percent })
          "
        >
          <line x1="0" y1="50" x2="100" y2="50" class="grid" />
          <line v-if="goal.due_date" x1="0" y1="100" x2="100" y2="0" class="soll" />
          <line
            v-if="goal.due_date && chartData.now < 100"
            :x1="chartData.now"
            y1="0"
            :x2="chartData.now"
            y2="100"
            class="now"
          />
          <polyline :points="chartData.line" class="ist" />
        </svg>
        <figcaption>
          <span>{{ shortDate(goal.start_date) }}</span>
          <span
            v-if="goal.due_date && chartData.now < 100"
            :style="{ left: `${chartData.now}%` }"
            class="gl-cap-now"
            >{{ $t('heute') }}</span
          >
          <span>{{ goal.due_date ? shortDate(goal.due_date) : $t('heute') }}</span>
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
        @click="emit('archive', next, true)"
      ></button>
      <button type="button" class="gl-step-t" @click="emit('edit', next)">{{ next.title }}</button>
      <span class="gl-muted">{{ $t('Als nächstes') }}</span>
    </div>
    <details v-if="steps.length" class="gl-steps">
      <summary>
        {{ $t('Alle Schritte') }}
        <span class="count">{{ steps.filter((s) => s.archived).length }}/{{ steps.length }}</span>
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
            @click="emit('archive', st, !st.archived)"
          >
            <svg v-if="st.archived" viewBox="0 0 24 24">
              <path d="M5 12.5l4.5 4.5L19 7.5" />
            </svg>
          </button>
          <button type="button" class="gl-step-t" @click="emit('edit', st)">
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
      v-if="!goal.archived"
      v-model="draft"
      class="gl-add"
      :placeholder="
        steps.length ? $t('Schritt hinzufügen …') : $t('Ersten Schritt hinzufügen …')
      "
      :aria-label="$t('Schritt zu „{title}“ hinzufügen', { title: goal.title })"
      @keydown.enter.prevent="handleAddStep"
    />
  </article>
</template>

<style scoped>
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
</style>
