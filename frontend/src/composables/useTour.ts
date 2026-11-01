import { ref } from 'vue'
import { store } from '@/lib/storage'

const DONE = 'kairo-tour-done'

// Rundgang beim ersten Start (Merker nur in diesem Browser) und jederzeit über die Einstellungen.
export const tourOpen = ref(false)
export const tourSeen = () => store.get(DONE) === '1'
export const markTourSeen = () => store.set(DONE, '1')
export const startTour = () => (tourOpen.value = true)
