/// <reference types="vite/client" />

declare module 'vue' {
  interface ComponentCustomProperties {
    $t: typeof import('./src/lib/i18n').t
    $tn: typeof import('./src/lib/i18n').tn
  }
}

export {}
