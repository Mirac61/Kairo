import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import preset from './theme'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import de from 'primelocale/de.json'
import App from './App.vue'
import router from './router'
import './styles.css'

createApp(App)
  .use(router)
  .use(PrimeVue, { theme: { preset, options: { darkModeSelector: 'system' } }, locale: de.de })
  .use(ConfirmationService)
  .mount('#app')
