import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import preset from './theme'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import '@fontsource/cormorant-garamond/latin-600.css'
import de from 'primelocale/de.json'
import App from './App.vue'
import router from './router'
import './styles.css'
import './design.css'
import { store } from './lib/storage'
import { lang, t, tn, type Lang } from './lib/i18n'
import { applyPrefs } from './lib/prefs'

// Vor dem Mounten setzen, damit nichts aufblitzt: Theme, Akzentfarbe, Bewegung.
applyPrefs()

// Sprache: gespeicherte Wahl, sonst die des Browsers.
lang.value = (store.get('kairo-lang') as Lang | null) ?? (navigator.language.startsWith('de') ? 'de' : 'en')

const app = createApp(App)
app.config.globalProperties.$t = t
app.config.globalProperties.$tn = tn
app
  .use(router)
  .use(PrimeVue, { theme: { preset, options: { darkModeSelector: '[data-theme="dark"]' } }, locale: de.de })
  .use(ConfirmationService)
  .mount('#app')
