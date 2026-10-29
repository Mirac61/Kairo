<script setup lang="ts">
import type { Review, Task } from '@/api/client'
import { daysAgo, ymd } from '@/lib/dates'
import { t } from '@/lib/i18n'
import { projectColor } from '@/lib/projectColor'
import type { DoneTask } from '../types'

const props = defineProps<{
  completed: DoneTask[]
  overdue: Task[]
  projects: Review['projects']
}>()

const emit = defineEmits<{
  (e: 'plan', task: Task): void
}>()

const WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map((d) => t(d))
const today = () => ymd(new Date())

const projectName = (id: string | null) =>
  props.projects.find((p) => p.project_id === id)?.name
</script>

<template>
  <section class="rv-group">
    <h2 class="rv-gh">
      {{ $t('Erledigt') }} <span class="count">{{ completed.length || '' }}</span>
    </h2>
    <div v-for="task in completed" :key="task.id" class="rv-row rv-task">
      <span
        class="cb ring"
        role="img"
        aria-checked="true"
        :aria-label="$t('Erledigt')"
      ></span>
      <span class="t">{{ task.title }}</span>
      <span class="chip" :style="{ '--chip-c': projectColor(task.project_id) }"
        ><template v-if="projectName(task.project_id)"
          ><span class="cdot"></span
          ><span class="rv-ell">{{ projectName(task.project_id) }}</span></template
        ></span
      >
      <span class="rv-n rv-wd">{{
        WD[(new Date(task.completed_at!).getDay() + 6) % 7]
      }}</span>
    </div>
    <div v-if="!completed.length" class="rv-empty">{{ $t('Nichts erledigt.') }}</div>
  </section>

  <section class="rv-group">
    <h2 class="rv-gh">
      {{ $t('Überfällig geworden') }} <span class="count">{{ overdue.length || '' }}</span>
    </h2>
    <div v-for="task in overdue" :key="task.id" class="rv-row rv-task rv-od">
      <span class="cb ring" aria-hidden="true"></span>
      <span class="t">{{ task.title }}</span>
      <span class="chip" :style="{ '--chip-c': projectColor(task.project_id) }"
        ><template v-if="projectName(task.project_id)"
          ><span class="cdot"></span
          ><span class="rv-ell">{{ projectName(task.project_id) }}</span></template
        ></span
      >
      <span class="rv-n od">{{ daysAgo(task.planned_date!, today()) }}</span>
      <button
        type="button"
        class="btn btn-ghost rv-plan"
        :aria-label="`Einplanen: ${task.title}`"
        @click="emit('plan', task)"
      >
        {{ $t('Heute') }}
      </button>
    </div>
    <div v-if="!overdue.length" class="rv-empty">{{ $t('Nichts überfällig.') }}</div>
  </section>
</template>

<style scoped>
.rv-group + .rv-group {
  margin-top: 32px;
}
.rv-ell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rv-gh {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  margin: 0 0 4px;
  font: 600 14.5px/1.3 var(--font-ui);
  letter-spacing: -0.01em;
  color: var(--tx-primary);
}
.rv-gh .count {
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.rv-row {
  display: grid;
  align-items: center;
  column-gap: 14px;
  min-height: 42px;
  padding: 6px 12px;
  margin: 0 -12px;
  border-radius: var(--r-s);
  transition: background var(--dur) ease-out;
}
.rv-row:hover {
  background: color-mix(in oklab, var(--tx-primary) 4%, transparent);
}
.rv-row .t {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font: 500 14px/1.4 var(--font-ui);
  color: var(--tx-primary);
}
.rv-row .chip {
  min-width: 0;
  justify-content: flex-end;
}
.rv-task {
  grid-template-columns: 18px minmax(0, 1fr) auto 28px;
}
.rv-task .cb {
  pointer-events: none;
}
.rv-task:not(.rv-od) .t {
  color: var(--tx-secondary);
}
.rv-wd {
  text-align: right;
  font-family: var(--font-ui);
}
.rv-od {
  grid-template-columns: 18px minmax(0, 1fr) auto auto auto;
}
.rv-od .od {
  color: var(--a-red);
  font-family: var(--font-ui);
}
.rv-plan {
  height: 26px;
  padding: 0 10px;
  color: var(--a-blue-tx);
  opacity: 0;
  transition: opacity var(--dur) ease-out;
}
.rv-od:hover .rv-plan,
.rv-plan:focus-visible {
  opacity: 1;
}
@media (hover: none) {
  .rv-plan {
    opacity: 1;
  }
}
.rv-empty {
  padding: 10px 0;
  font: 400 13px/1.5 var(--font-ui);
  color: var(--tx-muted);
}

@media (max-width: 640px) {
  .rv-task .chip {
    display: none;
  }
  .rv-task {
    grid-template-columns: 18px minmax(0, 1fr) 28px;
  }
  .rv-od {
    grid-template-columns: 18px minmax(0, 1fr) auto auto;
  }
}
</style>
