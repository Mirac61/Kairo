<script setup lang="ts">
import { nextTick, ref } from 'vue'
import type { Project } from '@/api/client'
import { WEEKDAYS, type Form } from '@/lib/eventForm'
import { vDialog } from '@/lib/dialog'

// Dialog zum Anlegen (Termin oder Task) und Bearbeiten von Terminen; form wird direkt bearbeitet.
const props = defineProps<{ form: Form; projects: Project[] }>()
defineEmits<{ save: []; remove: [onlyThis: boolean]; close: [] }>()

const kindOptions = [{ label: 'Termin', value: 'event' }, { label: 'Task', value: 'task' }]

const toggleDay = (code: string) => {
  const f = props.form
  f.days = f.days.includes(code) ? f.days.filter((d) => d !== code) : WEEKDAYS.map(([c]) => c).filter((c) => c === code || f.days.includes(c))
}

// Erster Fokus der Rückfrage liegt auf der sicheren Wahl.
const safeBtn = ref<HTMLElement>()
const askDelete = () => {
  props.form.ask = true
  void nextTick(() => safeBtn.value?.focus())
}
</script>

<template>
  <div v-dialog="() => $emit('close')" class="overlay open" @mousedown.self="$emit('close')">
    <form class="dialog" aria-labelledby="cal-dlg-title" @submit.prevent="$emit('save')">
      <div class="dlg-head"><h3 id="cal-dlg-title">{{ form.id ? $t('Termin bearbeiten') : $t('Neuer Eintrag') }}</h3></div>
      <div class="dlg-body">
        <div v-if="!form.id" class="seg">
          <button v-for="o in kindOptions" :key="o.value" type="button" :aria-pressed="form.kind === o.value" @click="form.kind = o.value">{{ $t(o.label) }}</button>
        </div>
        <div class="field"><label>{{ $t('Titel') }}</label><input v-model="form.title" class="input" :placeholder="$t('Titel')" autofocus /></div>
        <template v-if="form.kind === 'event'">
          <label class="lbl"><input v-model="form.allDay" type="checkbox" :disabled="form.keepTimes" /> {{ $t('Ganztägig') }}</label>
          <div class="dlg-row">
            <div class="field"><label>{{ form.repeat || form.customRule ? $t('Erster Termin') : $t('Datum') }}</label><input v-model="form.date" class="input" type="date" required /></div>
            <div v-if="form.allDay" class="field"><label>{{ $t('Bis (einschließlich)') }}</label><input v-model="form.endDate" class="input" type="date" :min="form.date" required /></div>
            <template v-else-if="!form.keepTimes">
              <div class="field"><label>{{ $t('Von') }}</label><input v-model="form.from" class="input" type="time" required /></div>
              <div class="field"><label>{{ $t('Bis') }}</label><input v-model="form.to" class="input" type="time" required /></div>
            </template>
          </div>
          <span v-if="form.keepTimes" class="lbl">{{ $t('Mehrtägiger Termin: Beginn und Ende bleiben unverändert.') }}</span>
          <div class="field"><label>{{ $t('Ort') }}</label><input v-model="form.location" class="input" :placeholder="$t('optional')" /></div>
          <span v-if="form.customRule" class="lbl">{{ $t('Serie mit eigener Regel ({rule}); sie bleibt unverändert.', { rule: form.customRule }) }}</span>
          <template v-else>
            <label class="lbl"><input v-model="form.repeat" type="checkbox" /> {{ $t('Wöchentlich wiederholen') }}</label>
            <template v-if="form.repeat">
              <div class="seg" role="group" :aria-label="$t('Wochentage')">
                <button v-for="[code, label] in WEEKDAYS" :key="code" type="button" :aria-pressed="form.days.includes(code)" @click="toggleDay(code)">{{ label }}</button>
              </div>
              <div class="field"><label>{{ $t('Endet am') }}</label><input v-model="form.until" class="input" type="date" :min="form.date" /></div>
            </template>
          </template>
          <span v-if="form.id && (form.repeat || form.customRule)" class="lbl">{{ $t('Änderungen gelten für die ganze Serie.') }}</span>
        </template>
        <div class="field">
          <label for="cal-project">{{ $t('Projekt') }}</label>
          <select id="cal-project" v-model="form.projectId" class="input">
            <option value="">{{ $t('Kein Projekt') }}</option>
            <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
      </div>
      <div v-if="form.ask" class="dlg-foot">
        <span class="q">{{ form.day ? $t('Nur diesen Termin oder die ganze Serie löschen?') : $t('Termin löschen?') }}</span>
        <button ref="safeBtn" type="button" class="btn btn-ghost" @click="form.ask = false">{{ $t('Abbrechen') }}</button>
        <button v-if="form.day" type="button" class="btn btn-secondary" @click="$emit('remove', true)">{{ $t('Nur dieser Termin') }}</button>
        <button type="button" class="btn btn-primary" @click="$emit('remove', false)">{{ form.day ? $t('Ganze Serie löschen') : $t('Termin löschen') }}</button>
      </div>
      <div v-else class="dlg-foot">
        <button v-if="form.id" type="button" class="btn btn-secondary" @click="askDelete">{{ $t('Löschen') }}</button>
        <span class="spacer" />
        <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Abbrechen') }}</button>
        <button type="submit" class="btn btn-primary">{{ form.id ? $t('Speichern') : $t('Anlegen') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.dlg-row { display: flex; gap: 12px; }
.dlg-foot { flex-wrap: wrap; }
.dlg-foot .spacer { flex: 1; }
.dlg-foot .q { flex: 1 0 100%; font-size: 13px; color: var(--tx-secondary); }
.dlg-row .field { flex: 1; }
</style>
