<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { FrequencyConfig, Habit, Project } from '@/api/client'
import { vDialog } from '@/lib/dialog'
import { ymd } from '@/lib/dates'
import { habitColor } from '@/lib/projectColor'
import { HABIT_ICONS, iconKey } from '@/lib/habitIcons'
import { locale, t } from '@/lib/i18n'
import { getWeekdayLabels, WEEKDAYS, type HabitRhythm } from '@/lib/habits'

const props = defineProps<{
  open: boolean
  habit?: Habit | null
  projects: Project[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (
    e: 'save',
    payload: {
      id?: string
      body: {
        name: string
        description: string
        frequency_type: Habit['frequency_type']
        frequency_config: FrequencyConfig
        preferred_time: string
        duration_minutes: number
        target_value: number | null
        unit: string
        icon: string
        remind: boolean
        start_date: string
        end_date: string
        project_id: string | null
      }
    },
  ): void
}>()

const wdLabel = computed(() => getWeekdayLabels(locale.value))

const RHYTHMS = computed<Record<HabitRhythm, string>>(() => ({
  DAILY: t('Täglich'),
  SPECIFIC_WEEKDAYS: t('Bestimmte Tage'),
  TIMES_PER_WEEK: t('X-mal pro Woche'),
}))

function blankForm() {
  return {
    id: '',
    name: '',
    description: '',
    type: 'DAILY' as HabitRhythm,
    weekdays: ['MO'] as string[],
    times: 3,
    time: '',
    duration: 30,
    target: null as number | null,
    unit: '',
    icon: '',
    remind: false,
    start: ymd(new Date()),
    end: '',
    project: '',
  }
}

const form = ref(blankForm())

watch(
  () => [props.open, props.habit] as const,
  ([isOpen, h]) => {
    if (!isOpen) return
    if (!h) {
      form.value = blankForm()
      return
    }

    const c = h.frequency_config
    form.value = {
      ...blankForm(),
      id: h.id,
      name: h.name,
      description: h.description,
      type: h.frequency_type === 'WEEKLY' ? 'SPECIFIC_WEEKDAYS' : (h.frequency_type as HabitRhythm),
      weekdays: h.frequency_type === 'WEEKLY' ? [c.weekday ?? 'MO'] : (c.weekdays ?? ['MO']),
      times: c.times ?? 3,
      time: h.preferred_time ?? '',
      duration: h.duration_minutes,
      target: h.target_value,
      unit: h.unit,
      icon: h.icon ?? '',
      remind: h.remind,
      start: h.start_date,
      end: h.end_date ?? '',
      project: h.project_id ?? '',
    }
  },
  { immediate: true },
)

function toggleDay(d: string) {
  const list = form.value.weekdays
  form.value.weekdays = list.includes(d) ? list.filter((x) => x !== d) : [...list, d]
}

function buildConfig(): FrequencyConfig {
  const f = form.value
  if (f.type === 'SPECIFIC_WEEKDAYS') return { weekdays: f.weekdays }
  if (f.type === 'TIMES_PER_WEEK') return { times: f.times || 1 }
  return {}
}

function save() {
  const f = form.value
  const name = f.name.trim()
  if (!name) return

  const body = {
    name,
    description: f.description.trim(),
    frequency_type: f.type,
    frequency_config: buildConfig(),
    preferred_time: f.time,
    duration_minutes: f.duration || 30,
    target_value: f.target || (f.id ? 0 : null),
    unit: f.unit,
    icon: f.icon,
    remind: f.remind && !!f.time,
    start_date: f.start,
    end_date: f.end,
    project_id: f.project || (f.id ? '' : null),
  }

  emit('save', { id: f.id || undefined, body })
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
      class="sheet hb-sheet"
      :aria-label="form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit')"
      @submit.prevent="save"
    >
      <div class="sheet-head">
        <span class="lbl">{{ form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit') }}</span>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="sheet-body">
        <input
          id="h-name"
          v-model="form.name"
          class="sheet-title"
          :placeholder="$t('Neue Gewohnheit')"
          :aria-label="$t('Name')"
          autofocus
          required
        />

        <input
          id="h-desc"
          v-model="form.description"
          class="sh-sub-in"
          :placeholder="$t('Beschreibung')"
          :aria-label="$t('Beschreibung')"
        />

        <div class="sh-sec">
          <span id="h-icon" class="sh-sec-lbl">{{ $t('Symbol') }}</span>
          <div
            class="hd-icons"
            role="radiogroup"
            aria-labelledby="h-icon"
            :style="{ '--hc': form.id ? habitColor(form.id) : 'var(--a-blue-hi)' }"
          >
            <button
              v-for="(id, k) in HABIT_ICONS"
              :key="k"
              type="button"
              role="radio"
              :aria-checked="iconKey(form) === k"
              :aria-label="k"
              @click="form.icon = k"
            >
              <svg class="ic" aria-hidden="true"><use :href="`#${id}`" /></svg>
            </button>
          </div>
        </div>

        <div class="sheet-props">
          <div class="sheet-row sh-rhythm-row">
            <span id="h-type">{{ $t('Rhythmus') }}</span>
            <div class="seg sh-rhythm" role="group" aria-labelledby="h-type">
              <button
                v-for="(label, val) in RHYTHMS"
                :key="val"
                type="button"
                :aria-pressed="form.type === val"
                @click="form.type = val"
              >
                {{ label }}
              </button>
            </div>
          </div>

          <div v-if="form.type === 'SPECIFIC_WEEKDAYS'" class="sheet-row sh-days-row">
            <span>{{ $t('Wochentage') }}</span>
            <div class="sheet-days" role="group" :aria-label="$t('Wochentage')">
              <button
                v-for="d in WEEKDAYS"
                :key="d"
                type="button"
                :aria-pressed="form.weekdays.includes(d)"
                @click="toggleDay(d)"
              >
                {{ wdLabel[d] }}
              </button>
            </div>
          </div>

          <label v-if="form.type === 'TIMES_PER_WEEK'" class="sheet-row">
            <span>{{ $t('Pro Woche') }}</span>
            <input
              id="h-times"
              v-model.number="form.times"
              class="input mono"
              type="number"
              min="1"
              max="7"
            />
          </label>

          <label class="sheet-row">
            <span>{{ $t('Uhrzeit') }}</span>
            <input id="h-time" v-model="form.time" class="input mono" type="time" />
          </label>

          <label class="sheet-row">
            <span>{{ $t('Dauer (Min.)') }}</span>
            <input
              id="h-dur"
              v-model.number="form.duration"
              class="input mono"
              type="number"
              min="5"
              max="1440"
              step="5"
              :disabled="!form.time && !form.project"
            />
          </label>

          <label class="sheet-row sh-switch-row" :class="{ disabled: !form.time }">
            <span>{{ $t('Erinnerung') }}</span>
            <button
              type="button"
              role="switch"
              class="sheet-switch"
              :aria-checked="form.remind && !!form.time"
              :aria-label="$t('Zur Uhrzeit erinnern, solange offen (ntfy)')"
              :disabled="!form.time"
              @click="form.remind = !form.remind"
            >
              <i></i>
            </button>
          </label>

          <label class="sheet-row">
            <span>{{ $t('Aus Zeiterfassung') }}</span>
            <select id="h-proj" v-model="form.project" class="input">
              <option value="">{{ $t('Nur von Hand abhaken') }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </label>

          <div class="sheet-row">
            <span>{{ $t('Ziel') }}</span>
            <input
              id="h-target"
              v-model.number="form.target"
              class="input mono sh-num"
              type="number"
              min="1"
              :placeholder="$t('optional')"
              :aria-label="$t('Ziel')"
            />
            <input
              id="h-unit"
              v-model="form.unit"
              class="input"
              :placeholder="$t('z. B. Seiten')"
              :aria-label="$t('Einheit')"
            />
          </div>

          <label class="sheet-row">
            <span>{{ $t('Beginnt am') }}</span>
            <input id="h-start" v-model="form.start" class="input" type="date" required />
          </label>

          <label class="sheet-row">
            <span>{{ $t('Endet am') }}</span>
            <input id="h-end" v-model="form.end" class="input" type="date" :min="form.start" />
          </label>
        </div>

        <p class="sh-note">
          {{ $t('Mit Uhrzeit erscheint die Gewohnheit als Block im Kalender.') }}
        </p>
        <p v-if="form.project" class="sh-note">
          {{
            $t(
              'Erledigt sich selbst, sobald an einem Tag so viele Minuten im Projekt erfasst sind wie die Dauer.',
            )
          }}
        </p>
      </div>

      <div class="sheet-foot">
        <span class="spacer" />
        <button class="btn btn-ghost" type="button" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          :disabled="form.type === 'SPECIFIC_WEEKDAYS' && !form.weekdays.length"
        >
          {{ form.id ? $t('Speichern') : $t('Anlegen') }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.hb-sheet {
  --sheet-w: 420px;
  --sheet-lbl-w: 112px;
}
.sh-sub-in {
  width: 100%;
  padding: 6px 0;
  margin-top: 2px;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: none;
  color: var(--tx-secondary);
  font: 400 13.5px/1.4 var(--font-ui);
}
.sh-sub-in::placeholder {
  color: var(--tx-muted);
}
.sh-sub-in:focus-visible {
  outline: none;
  border-radius: 0;
  box-shadow: none;
  border-bottom-color: var(--a-focus);
}
.sh-sec {
  margin-top: 16px;
}
.sh-sec-lbl {
  display: block;
  margin-bottom: 8px;
  font: 500 11.5px/1 var(--font-ui);
  color: var(--tx-muted);
}
.hd-icons {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.hd-icons button {
  width: 30px;
  height: 30px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: none;
  color: var(--tx-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    background var(--dur) var(--ease),
    color var(--dur) var(--ease),
    border-color var(--dur) var(--ease);
}
.hd-icons button:hover {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.hd-icons button[aria-checked='true'] {
  background: color-mix(in srgb, var(--hc) 16%, transparent);
  color: var(--hc);
  border-color: color-mix(in srgb, var(--hc) 45%, transparent);
}
.sh-rhythm-row {
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding: 10px 0;
}
.sh-rhythm-row > span {
  width: auto;
}
.sh-rhythm {
  width: 100%;
  display: flex;
}
.sh-rhythm button {
  flex: 1;
  padding: 6px 8px;
  font-size: 12px;
  white-space: nowrap;
}
.sh-days-row {
  padding: 6px 0;
}
.sh-switch-row {
  cursor: pointer;
}
.sh-switch-row.disabled {
  cursor: not-allowed;
}
.sh-num {
  flex: 0 0 88px;
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
