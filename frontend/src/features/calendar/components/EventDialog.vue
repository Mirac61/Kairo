<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import type { Project } from '@/api/client'
import { WEEKDAYS, type Form, type Freq } from '@/lib/eventForm'
import { vDialog } from '@/lib/dialog'

// Seitenpanel zum Anlegen (Termin oder Task) und Bearbeiten von Terminen – gleicher Stil wie TaskEditSheet,
// damit der Kalender beim Bearbeiten sichtbar bleibt.
const props = defineProps<{ form: Form; projects: Project[] }>()
defineEmits<{
  save: [form: Form]
  remove: [onlyThis: boolean, form: Form]
  close: []
}>()

const form = reactive<Form>({ ...props.form, days: [...props.form.days] })
watch(
  () => props.form,
  (next) => {
    Object.assign(form, next, { days: [...next.days] })
  },
)

const kindOptions = [
  { label: 'Termin', value: 'event' },
  { label: 'Task', value: 'task' },
]
const FREQS: [Freq, string][] = [
  ['WEEKLY', 'Wöchentlich'],
  ['MONTHLY', 'Monatlich'],
  ['YEARLY', 'Jährlich'],
]

const repeatChoice = computed<'' | Freq>({
  get: () => (form.repeat ? form.freq : ''),
  set: (v) => {
    if (!v) {
      form.repeat = false
    } else {
      form.repeat = true
      form.freq = v
    }
  },
})

const toggleDay = (code: string) => {
  form.days = form.days.includes(code)
    ? form.days.filter((d) => d !== code)
    : WEEKDAYS.map(([c]) => c).filter((c) => c === code || form.days.includes(c))
}

// Erster Fokus der Rückfrage liegt auf der sicheren Wahl.
const safeBtn = ref<HTMLElement>()
const askDelete = () => {
  form.ask = true
  void nextTick(() => safeBtn.value?.focus())
}
</script>

<template>
  <div v-dialog="() => $emit('close')" class="sheet-wrap" @mousedown.self="$emit('close')">
    <form
      class="sheet ev-sheet"
      aria-labelledby="cal-dlg-title"
      @submit.prevent="$emit('save', form)"
    >
      <div class="sheet-head">
        <span id="cal-dlg-title" class="lbl">{{
          form.id ? $t('Termin') : $t('Neuer Eintrag')
        }}</span>
        <button
          class="icon-btn"
          type="button"
          :aria-label="$t('Schließen')"
          @click="$emit('close')"
        >
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="sheet-body">
        <div v-if="!form.id" class="seg ev-kind">
          <button
            v-for="o in kindOptions"
            :key="o.value"
            type="button"
            :aria-pressed="form.kind === o.value"
            @click="form.kind = o.value"
          >
            {{ $t(o.label) }}
          </button>
        </div>

        <input
          v-model="form.title"
          class="sheet-title"
          :placeholder="$t('Titel')"
          :aria-label="$t('Titel')"
          autofocus
        />

        <div class="sheet-props">
          <template v-if="form.kind === 'event'">
            <label class="sheet-row">
              <span>{{ form.repeat || form.customRule ? $t('Erster Termin') : $t('Datum') }}</span>
              <input v-model="form.date" class="input" type="date" required />
            </label>

            <label v-if="form.allDay" class="sheet-row">
              <span>{{ $t('Bis (einschließlich)') }}</span>
              <input v-model="form.endDate" class="input" type="date" :min="form.date" required />
            </label>

            <div v-else-if="!form.keepTimes" class="sheet-row">
              <span>{{ $t('Zeit') }}</span>
              <input
                v-model="form.from"
                class="input mono"
                type="time"
                :aria-label="$t('Von')"
                required
              />
              <em aria-hidden="true">–</em>
              <input
                v-model="form.to"
                class="input mono"
                type="time"
                :aria-label="$t('Bis')"
                required
              />
            </div>

            <label class="sheet-row ev-switch-row" :class="{ disabled: form.keepTimes }">
              <span>{{ $t('Ganztägig') }}</span>
              <button
                type="button"
                role="switch"
                class="sheet-switch"
                :aria-checked="form.allDay"
                :disabled="form.keepTimes"
                @click="form.allDay = !form.allDay"
              >
                <i></i>
              </button>
            </label>

            <label class="sheet-row">
              <span>{{ $t('Ort') }}</span>
              <input v-model="form.location" class="input" :placeholder="$t('optional')" />
            </label>

            <label v-if="!form.customRule" class="sheet-row">
              <span>{{ $t('Wiederholung') }}</span>
              <select v-model="repeatChoice" class="input">
                <option value="">{{ $t('Nicht wiederholen') }}</option>
                <option v-for="[fq, label] in FREQS" :key="fq" :value="fq">{{ $t(label) }}</option>
              </select>
            </label>

            <template v-if="!form.customRule && form.repeat">
              <div v-if="form.freq === 'WEEKLY'" class="sheet-row ev-days-row">
                <span>{{ $t('Wochentage') }}</span>
                <div class="sheet-days" role="group" :aria-label="$t('Wochentage')">
                  <button
                    v-for="[code, label] in WEEKDAYS"
                    :key="code"
                    type="button"
                    :aria-pressed="form.days.includes(code)"
                    @click="toggleDay(code)"
                  >
                    {{ $t(label) }}
                  </button>
                </div>
              </div>

              <label class="sheet-row">
                <span>{{ $t('Endet am') }}</span>
                <input v-model="form.until" class="input" type="date" :min="form.date" />
              </label>
            </template>
          </template>

          <label class="sheet-row">
            <span>{{ $t('Projekt') }}</span>
            <select v-model="form.projectId" class="input">
              <option value="">{{ $t('Kein Projekt') }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </label>

          <label v-if="form.kind === 'event'" class="sheet-row ev-switch-row">
            <span>{{ $t('Erklär-Check') }}</span>
            <button
              type="button"
              role="switch"
              class="sheet-switch"
              :aria-checked="form.explainCheck"
              :title="$t('Nach jedem Termin eine Aufgabe „5 Min erklären“ für denselben Tag')"
              @click="form.explainCheck = !form.explainCheck"
            >
              <i></i>
            </button>
          </label>
        </div>

        <p v-if="form.kind === 'event' && form.keepTimes" class="ev-note">
          {{ $t('Mehrtägiger Termin: Beginn und Ende bleiben unverändert.') }}
        </p>

        <p v-if="form.kind === 'event' && form.customRule" class="ev-note">
          {{
            $t('Serie mit eigener Regel ({rule}); sie bleibt unverändert.', {
              rule: form.customRule,
            })
          }}
        </p>

        <p
          v-if="form.kind === 'event' && form.id && (form.repeat || form.customRule)"
          class="ev-note"
        >
          {{ $t('Änderungen gelten für die ganze Serie.') }}
        </p>
      </div>

      <div v-if="form.ask" class="sheet-foot is-ask">
        <span class="ev-q">{{
          form.day ? $t('Nur diesen Termin oder die ganze Serie löschen?') : $t('Termin löschen?')
        }}</span>
        <div class="ev-ask-btns">
          <button ref="safeBtn" type="button" class="btn btn-ghost" @click="form.ask = false">
            {{ $t('Abbrechen') }}
          </button>
          <button
            v-if="form.day"
            type="button"
            class="btn btn-secondary"
            @click="$emit('remove', true, form)"
          >
            {{ $t('Nur dieser Termin') }}
          </button>
          <button type="button" class="btn btn-primary" @click="$emit('remove', false, form)">
            {{ form.day ? $t('Ganze Serie löschen') : $t('Termin löschen') }}
          </button>
        </div>
      </div>

      <div v-else class="sheet-foot">
        <button v-if="form.id" type="button" class="btn btn-ghost ev-del" @click="askDelete">
          {{ $t('Löschen') }}
        </button>
        <span class="spacer" />
        <button type="button" class="btn btn-ghost" @click="$emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button type="submit" class="btn btn-primary">
          {{ form.id ? $t('Speichern') : $t('Anlegen') }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.ev-sheet {
  --sheet-w: 380px;
  --sheet-lbl-w: 104px;
}
.ev-kind {
  margin-bottom: 14px;
  width: max-content;
}
.ev-switch-row {
  cursor: pointer;
}
.ev-switch-row.disabled {
  cursor: not-allowed;
}
.ev-days-row {
  padding: 6px 0;
}
.ev-note {
  margin: 12px 0 0;
  font: 400 12px/1.45 var(--font-ui);
  color: var(--tx-muted);
}
.sheet-foot.is-ask {
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
}
.ev-ask-btns {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}
.ev-q {
  font: 400 13px/1.4 var(--font-ui);
  color: var(--tx-secondary);
}
.ev-del:hover,
.ev-del:focus-visible {
  background: color-mix(in srgb, var(--a-red) 14%, transparent);
  color: var(--a-red);
}
.spacer {
  flex: 1;
}
</style>
