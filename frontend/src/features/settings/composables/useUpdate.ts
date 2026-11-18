import { onMounted, ref, type Ref } from 'vue'
import { applyUpdate, checkUpdate, errorMessage, getHealth, type UpdateStatus } from '@/api/client'
import { t } from '@/lib/i18n'

// Prüft beim Öffnen der Systemeinstellungen, ob auf GitHub eine neuere Fassung
// liegt, und installiert sie auf Wunsch. Das Backend tauscht sein Binary aus und
// startet neu; die Seite lädt neu, sobald die neue Fassung antwortet.
export function useUpdate(current: Ref<string>) {
  const status = ref<UpdateStatus | null>(null)
  const error = ref('')
  const busy = ref(false)

  async function check() {
    busy.value = true
    error.value = ''
    try {
      status.value = await checkUpdate()
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  async function install() {
    if (!status.value?.available) return
    busy.value = true
    error.value = ''
    try {
      await applyUpdate()
      for (let i = 0; i < 60; i++) {
        await new Promise((r) => setTimeout(r, 500))
        try {
          if ((await getHealth()).version !== current.value) {
            location.reload()
            return
          }
        } catch {
          /* Backend startet noch */
        }
      }
      error.value = t('Die neue Fassung läuft noch nicht. Kairo einmal neu starten.')
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  onMounted(() => void check())

  return { status, error, busy, check, install }
}
