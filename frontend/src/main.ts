import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import preset from './theme'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import '@fontsource/cormorant-garamond/latin-600.css'
import '@fontsource/instrument-serif/latin-400.css'
import '@fontsource/instrument-serif/latin-400-italic.css'
import de from 'primelocale/de.json'
import App from './App.vue'
import router from './router'
import './styles.css'
import './design.css'
import { store } from './lib/storage'
import { lang, t, tn, type Lang } from './lib/i18n'
import { applyPrefs, prefsPulled, pullPrefs, syncPrefs } from './lib/prefs'

// Vor dem Mounten setzen, damit nichts aufblitzt: Theme, Akzentfarbe, Bewegung.
applyPrefs()

// Sprache: gespeicherte Wahl, sonst die des Browsers.
lang.value = (store.get('kairo-lang') as Lang | null) ?? (navigator.language.startsWith('de') ? 'de' : 'en')

syncPrefs()
router.afterEach(() => { if (!prefsPulled()) void pullPrefs() })

const app = createApp(App)
app.config.globalProperties.$t = t
app.config.globalProperties.$tn = tn
app
  .use(router)
  .use(PrimeVue, { theme: { preset, options: { darkModeSelector: '[data-theme="dark"]' } }, locale: de.de })
  .use(ConfirmationService)
  .mount('#app')
