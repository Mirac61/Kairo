<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { store } from '@/lib/storage'
import SettingsGeneralTab from './components/SettingsGeneralTab.vue'
import SettingsLookTab from './components/SettingsLookTab.vue'
import SettingsNotesTab from './components/SettingsNotesTab.vue'
import SettingsSystemTab from './components/SettingsSystemTab.vue'
import SettingsTimeTab from './components/SettingsTimeTab.vue'
import { useSettingsData } from './composables/useSettingsData'
import './settings.css'

// Aufbau wie die Notizen: links die Abschnitte, rechts der gewählte. Der Abschnitt bleibt für den nächsten Besuch.
const SECTIONS = [
  { id: 'general', label: 'Allgemein', icon: 'gear' },
  { id: 'look', label: 'Darstellung', icon: 'sun' },
  { id: 'time', label: 'Zeit & Kalender', icon: 'cal' },
  { id: 'notes', label: 'Notizen', icon: 'note' },
  { id: 'system', label: 'System', icon: 'grid' },
] as const
type Section = (typeof SECTIONS)[number]['id']

const tab = ref<Section>(
  SECTIONS.find((s) => s.id === store.get('kairo-settings-tab'))?.id ?? 'general',
)
watch(tab, (v) => store.set('kairo-settings-tab', v))
const current = computed(() => SECTIONS.find((s) => s.id === tab.value)!)

const { data, form, version, restarting, dirty, error, save, resetForm, restart, locked, leadMin } =
  useSettingsData()
</script>

<template>
  <div class="st settings-view">
    <aside class="st-pane">
      <div class="st-head">
        <span class="lbl">{{ $t('Einstellungen') }}</span>
      </div>
      <nav class="st-nav" :aria-label="$t('Abschnitte')">
        <button
          v-for="s in SECTIONS"
          :key="s.id"
          type="button"
          class="nav-item"
          :aria-current="tab === s.id ? 'page' : undefined"
          @click="tab = s.id"
        >
          <svg class="ic"><use :href="`#i-${s.icon}`" /></svg><span>{{ $t(s.label) }}</span>
        </button>
      </nav>
    </aside>

    <section class="st-main">
      <header class="ed-head">
        <div class="ed-path">
          <span>{{ $t('Einstellungen') }}</span
          ><span class="last">{{ $t(current.label) }}</span>
        </div>
        <span v-if="dirty" class="ed-dirty"><i></i>{{ $t('Nicht gespeichert') }}</span>
      </header>

      <div class="st-scroll">
        <div class="st-body">
          <div v-if="error" class="badge st-err" role="alert">{{ error }}</div>

          <SettingsGeneralTab v-if="tab === 'general'" />
          <SettingsLookTab v-else-if="tab === 'look'" />
          <SettingsTimeTab
            v-else-if="tab === 'time'"
            v-model:work-start="form.work_start"
            v-model:work-end="form.work_end"
            v-model:timezone="form.timezone"
            v-model:lead-min="leadMin"
            :data="data"
            :locked="locked"
          />
          <SettingsNotesTab
            v-else-if="tab === 'notes'"
            v-model:notes-dir="form.notes_dir"
            :data="data"
            :locked="locked"
          />
          <SettingsSystemTab
            v-else
            :version="version"
            :data="data"
            :restarting="restarting"
            @restart="restart"
          />
        </div>
      </div>

      <div v-if="dirty || (data?.restart_needed && tab !== 'system')" class="st-bar" role="status">
        <span class="st-h">{{
          dirty
            ? $t('Änderungen gelten nach einem Neustart.')
            : $t('Gespeichert. Die neuen Werte gelten nach dem Neustart.')
        }}</span>
        <span class="row">
          <button v-if="dirty" type="button" class="btn btn-ghost" @click="resetForm">
            {{ $t('Verwerfen') }}
          </button>
          <button v-if="dirty" type="button" class="btn btn-primary" @click="save">
            {{ $t('Speichern') }}
          </button>
          <button
            v-else
            type="button"
            class="btn btn-secondary"
            :disabled="restarting"
            @click="restart"
          >
            {{ restarting ? $t('Startet neu …') : $t('Jetzt neu starten') }}
          </button>
        </span>
      </div>
    </section>
  </div>
</template>
