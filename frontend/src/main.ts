import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import Aura from '@primeuix/themes/aura'
import de from 'primelocale/de.json'
import App from './App.vue'
import router from './router'
import './styles.css'

createApp(App)
  .use(router)
  .use(PrimeVue, { theme: { preset: Aura, options: { darkModeSelector: 'system' } }, locale: de.de })
  .use(ConfirmationService)
  .mount('#app')
