<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getHealth, getSettings, restartBackend, saveSettings, type Settings, type SettingsFile } from '@/api/client'
import { useLoader } from '@/composables/useLoader'
import { lang, t, type Lang } from '@/lib/i18n'
import { store } from '@/lib/storage'

const data = ref<Settings | null>(null)
const form = ref<SettingsFile>({})
const saved = ref(false)
const restarting = ref(false)
const zones = Intl.supportedValuesOf('timeZone')

const { error, load, run } = useLoader(async () => {
  data.value = await getSettings()
  form.value = { ...data.value.settings }
})

async function save() {
  saved.value = await run(() => saveSettings(form.value))
}

// Das Backend startet sich selbst neu; danach die Seite neu laden, damit alle Ansichten die neuen Werte holen.
async function restart() {
  restarting.value = true
  await run(restartBackend)
  for (let i = 0; i < 40; i++) {
    await new Promise((r) => setTimeout(r, 500))
    try {
      await getHealth()
      location.reload()
      return
    } catch { /* noch nicht wieder da */ }
  }
  restarting.value = false
  error.value = t('Das Backend ist nach dem Neustart nicht erreichbar.')
}

const locked = (k: keyof SettingsFile) => data.value?.env[k]

// Die Sprache gilt sofort und nur in diesem Browser; sie steht nicht in config.json.
function setLang(v: Lang) {
  lang.value = v
  store.set('kairo-lang', v)
}

onMounted(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">{{ $t('Einstellungen') }}</h1>
      <div class="v-sub">{{ $t('Gespeichert in') }} <code>{{ data?.path }}</code>{{ $t('. Änderungen gelten nach einem Neustart.') }}</div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div class="set-form set-lang">
      <div class="field">
        <label for="s-lang">{{ $t('Sprache') }}</label>
        <select id="s-lang" class="input" :value="lang" @change="setLang(($event.target as HTMLSelectElement).value as Lang)">
          <option value="de">Deutsch</option>
          <option value="en">English</option>
        </select>
        <span class="set-hint">{{ $t('Gilt sofort, nur in diesem Browser.') }}</span>
      </div>
    </div>
    <form v-if="data" class="set-form" @submit.prevent="save">
      <div class="field">
        <label for="s-notes">{{ $t('Notizordner') }}</label>
        <input id="s-notes" v-model="form.notesDir" class="input" :disabled="!!locked('notesDir')" :placeholder="$t('Absoluter Pfad oder ~/…, muss existieren')" />
        <span v-if="locked('notesDir')" class="set-hint">{{ $t('Festgelegt durch') }} <code>{{ locked('notesDir') }}</code></span>
      </div>
      <div class="set-row">
        <div class="field">
          <label for="s-start">{{ $t('Arbeitszeit von') }}</label>
          <input id="s-start" v-model="form.workStart" type="time" class="input" :disabled="!!locked('workStart')" />
        </div>
        <div class="field">
          <label for="s-end">{{ $t('bis') }}</label>
          <input id="s-end" v-model="form.workEnd" type="time" class="input" :disabled="!!locked('workEnd')" />
        </div>
      </div>
      <span v-if="locked('workStart') || locked('workEnd')" class="set-hint">{{ $t('Festgelegt durch') }} <code>KAIRO_WORK_START</code>/<code>KAIRO_WORK_END</code></span>
      <div class="field">
        <label for="s-tz">{{ $t('Zeitzone') }}</label>
        <input id="s-tz" v-model="form.timezone" class="input" list="s-zones" :disabled="!!locked('timezone')" :placeholder="$t('Systemzeitzone')" />
        <datalist id="s-zones"><option v-for="z in zones" :key="z" :value="z" /></datalist>
        <span v-if="locked('timezone')" class="set-hint">{{ $t('Festgelegt durch') }} <code>{{ locked('timezone') }}</code></span>
      </div>
      <div class="row">
        <button type="submit" class="btn btn-primary">{{ $t('Speichern') }}</button>
        <button v-if="data.restart_needed" type="button" class="btn btn-secondary" :disabled="restarting" @click="restart">
          {{ restarting ? $t('Startet neu …') : $t('Jetzt neu starten') }}
        </button>
        <span v-if="saved && data.restart_needed" class="set-hint" role="status">{{ $t('Gespeichert. Die neuen Werte gelten nach dem Neustart.') }}</span>
      </div>
    </form>
  </div>
</template>

<style scoped>
.set-form { display: grid; gap: 18px; max-width: 520px; }
.set-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.set-hint { font-size: 12px; color: var(--tx-muted); }
.set-lang { margin-bottom: 28px; }
</style>
