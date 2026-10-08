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

// Vor dem Mounten setzen, damit nichts aufblitzt: gespeichertes Theme, sonst System.
document.documentElement.dataset.theme =
  store.get('kairo-theme') ?? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')

createApp(App)
  .use(router)
  .use(PrimeVue, { theme: { preset, options: { darkModeSelector: '[data-theme="dark"]' } }, locale: de.de })
  .use(ConfirmationService)
  .mount('#app')
