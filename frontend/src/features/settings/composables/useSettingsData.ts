import { computed, onMounted, ref } from 'vue'
import {
  getHealth,
  getSettings,
  restartBackend,
  saveSettings,
  type Settings,
  type SettingsFile,
} from '@/api/client'
import { useLoader } from '@/composables/useLoader'
import { t } from '@/lib/i18n'

export function useSettingsData() {
  const data = ref<Settings | null>(null)
  const form = ref<SettingsFile>({})
  const version = ref('')
  const restarting = ref(false)

  // Leere Felder im Formular bedeuten Standardwert, wie in config.json.
  const dirty = computed(
    () =>
      !!data.value &&
      (['notes_dir', 'work_start', 'work_end', 'timezone', 'ntfy_lead'] as const).some(
        (k) => (form.value[k] ?? '') !== (data.value!.settings[k] ?? ''),
      ),
  )

  const { error, load, run } = useLoader(async () => {
    data.value = await getSettings()
    form.value = { ...data.value.settings }
  })

  const save = () => run(() => saveSettings(form.value))

  const resetForm = () => {
    if (data.value) {
      form.value = { ...data.value.settings }
    }
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
      } catch {
        /* noch nicht wieder da */
      }
    }
    restarting.value = false
    error.value = t('Das Backend ist nach dem Neustart nicht erreichbar.')
  }

  const locked = (k: keyof SettingsFile) => data.value?.env[k]

  // Vorlauf der ntfy-Erinnerung in Minuten; das Backend liefert eine Go-Dauer wie „10m0s“, geschrieben wird dieselbe Form.
  const leadMin = computed({
    get: () =>
      [...(form.value.ntfy_lead ?? '').matchAll(/(\d+)([hm])/g)].reduce(
        (n, [, v, u]) => n + Number(v) * (u === 'h' ? 60 : 1),
        0,
      ),
    set: (v: number) => {
      const m = Math.max(0, Math.round(v || 0))
      form.value.ntfy_lead =
        m === 0 ? '0s' : `${m >= 60 ? `${Math.floor(m / 60)}h` : ''}${m % 60}m0s`
    },
  })

  onMounted(() => {
    void load()
    getHealth().then(
      (h) => (version.value = h.version),
      () => {},
    )
  })

  return {
    data,
    form,
    version,
    restarting,
    dirty,
    error,
    save,
    resetForm,
    restart,
    locked,
    leadMin,
  }
}
