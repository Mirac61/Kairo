<script setup lang="ts">
import { ref, watch } from 'vue'
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

const wdLabel = getWeekdayLabels(locale.value)

const RHYTHMS: Record<HabitRhythm, string> = {
  DAILY: t('Täglich'),
  SPECIFIC_WEEKDAYS: t('Bestimmte Tage'),
  TIMES_PER_WEEK: t('X-mal pro Woche'),
}

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
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <form
      class="dialog"
      :aria-label="form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit')"
      @submit.prevent="save"
    >
      <div class="dlg-head">
        <h3>{{ form.id ? $t('Gewohnheit bearbeiten') : $t('Neue Gewohnheit') }}</h3>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="dlg-body">
        <div class="field">
          <label for="h-name">{{ $t('Name') }}</label>
          <input
            id="h-name"
            v-model="form.name"
            class="input"
            :placeholder="$t('Neue Gewohnheit')"
            autofocus
            required
          />
        </div>

        <div class="field">
          <label for="h-desc">{{ $t('Beschreibung') }}</label>
          <input
            id="h-desc"
            v-model="form.description"
            class="input"
            :placeholder="$t('optional')"
          />
        </div>

        <div class="field">
          <label id="h-icon">{{ $t('Symbol') }}</label>
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

        <div class="field">
          <label id="h-type">{{ $t('Rhythmus') }}</label>
          <div class="seg" role="group" aria-labelledby="h-type">
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

        <div
          v-if="form.type === 'SPECIFIC_WEEKDAYS'"
          class="seg"
          role="group"
          :aria-label="$t('Wochentage')"
        >
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

        <div v-if="form.type === 'TIMES_PER_WEEK'" class="field">
          <label for="h-times">{{ $t('Pro Woche') }}</label>
          <input
            id="h-times"
            v-model.number="form.times"
            class="input"
            type="number"
            min="1"
            max="7"
          />
        </div>

        <div class="dlg-row">
          <div class="field">
            <label for="h-time">{{ $t('Uhrzeit') }}</label>
            <input id="h-time" v-model="form.time" class="input" type="time" />
          </div>
          <div class="field">
            <label for="h-dur">{{ $t('Dauer (Min.)') }}</label>
            <input
              id="h-dur"
              v-model.number="form.duration"
              class="input"
              type="number"
              min="5"
              max="1440"
              step="5"
              :disabled="!form.time && !form.project"
            />
          </div>
        </div>
        <div class="v-sub">
          {{ $t('Mit Uhrzeit erscheint die Gewohnheit als Block im Kalender.') }}
        </div>

        <label class="lbl">
          <input v-model="form.remind" type="checkbox" :disabled="!form.time" />
          {{ $t('Zur Uhrzeit erinnern, solange offen (ntfy)') }}
        </label>

        <div class="field">
          <label for="h-proj">{{ $t('Aus Zeiterfassung') }}</label>
          <select id="h-proj" v-model="form.project" class="input">
            <option value="">{{ $t('Nur von Hand abhaken') }}</option>
            <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div v-if="form.project" class="v-sub">
          {{
            $t(
              'Erledigt sich selbst, sobald an einem Tag so viele Minuten im Projekt erfasst sind wie die Dauer.',
            )
          }}
        </div>

        <div class="dlg-row">
          <div class="field">
            <label for="h-target">{{ $t('Ziel') }}</label>
            <input
              id="h-target"
              v-model.number="form.target"
              class="input"
              type="number"
              min="1"
              :placeholder="$t('optional')"
            />
          </div>
          <div class="field">
            <label for="h-unit">{{ $t('Einheit') }}</label>
            <input
              id="h-unit"
              v-model="form.unit"
              class="input"
              :placeholder="$t('z. B. Seiten')"
            />
          </div>
        </div>

        <div class="dlg-row">
          <div class="field">
            <label for="h-start">{{ $t('Beginnt am') }}</label>
            <input id="h-start" v-model="form.start" class="input" type="date" required />
          </div>
          <div class="field">
            <label for="h-end">{{ $t('Endet am') }}</label>
            <input id="h-end" v-model="form.end" class="input" type="date" :min="form.start" />
          </div>
        </div>
      </div>

      <div class="dlg-foot">
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
.dlg-row {
  display: flex;
  gap: 12px;
}
.dlg-row .field {
  flex: 1;
}

.hd-icons {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(36px, 1fr));
  gap: 6px;
}
.hd-icons button {
  display: grid;
  place-items: center;
  aspect-ratio: 1;
  border: 1px solid var(--br-default);
  border-radius: 9px;
  background: var(--bg-1);
  color: var(--tx-secondary);
  cursor: pointer;
}
.hd-icons button:hover {
  border-color: var(--br-strong);
  color: var(--tx-primary);
}
.hd-icons button[aria-checked='true'] {
  border-color: var(--hc);
  color: var(--hc);
  background: color-mix(in srgb, var(--hc) 14%, transparent);
}
.hd-icons .ic {
  width: 18px;
  height: 18px;
}
</style>
