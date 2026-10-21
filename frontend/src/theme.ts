import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Alle Farben kommen aus design.css (--bg-*, --br-*, --tx-*, --a-*). So folgen auch
// PrimeVue-Flächen (Bestätigung, Hinweise) dem Theme und der Farbstimmung.
const ramp = {
  50: 'var(--tx-primary)',
  100: 'var(--tx-primary)',
  200: 'var(--tx-secondary)',
  300: 'var(--tx-secondary)',
  400: 'var(--tx-muted)',
  500: 'var(--tx-muted)',
  600: 'var(--tx-disabled)',
  700: 'var(--br-strong)',
  800: 'var(--br-default)',
  900: 'var(--br-subtle)',
  950: 'var(--bg-1)',
}

const highlight = {
  background: 'color-mix(in srgb, var(--a-blue-hi) 14%, transparent)',
  focusBackground: 'color-mix(in srgb, var(--a-blue-hi) 20%, transparent)',
  color: 'var(--a-blue-tx)',
  focusColor: 'var(--a-blue-tx)',
}

export default definePreset(Aura, {
  semantic: {
    primary: ramp,
    formField: { borderRadius: '6px' },
    colorScheme: {
      dark: {
        surface: {
          0: 'var(--tx-primary)',
          50: 'var(--tx-primary)',
          100: 'var(--tx-secondary)',
          200: 'var(--tx-secondary)',
          300: 'var(--tx-muted)',
          400: 'var(--tx-muted)',
          500: 'var(--tx-disabled)',
          600: 'var(--br-strong)',
          700: 'var(--br-default)',
          800: 'var(--bg-3)',
          900: 'var(--bg-1)',
          950: 'var(--bg-0)',
        },
        primary: {
          color: 'var(--tx-primary)',
          contrastColor: 'var(--bg-0)',
          hoverColor: 'var(--tx-primary)',
          activeColor: 'var(--tx-primary)',
        },
        highlight,
      },
      light: {
        surface: {
          0: 'var(--bg-1)',
          50: 'var(--bg-0)',
          100: 'var(--bg-2)',
          200: 'var(--bg-3)',
          300: 'var(--br-strong)',
          400: 'var(--tx-disabled)',
          500: 'var(--tx-muted)',
          600: 'var(--tx-secondary)',
          700: 'var(--tx-primary)',
          800: 'var(--tx-primary)',
          900: 'var(--tx-primary)',
          950: 'var(--tx-primary)',
        },
        primary: {
          color: 'var(--tx-primary)',
          contrastColor: 'var(--bg-1)',
          hoverColor: 'var(--tx-primary)',
          activeColor: 'var(--tx-primary)',
        },
        highlight,
      },
    },
  },
})
