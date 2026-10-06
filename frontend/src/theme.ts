import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Vercel x Kanagawa Dragon: warm near-black surfaces, softened dragon accents.
const neutral = {
  50: '#f4f2ee', 100: '#ece9e3', 200: '#dedad2', 300: '#c9c4ba', 400: '#a39f95',
  500: '#6f6c64', 600: '#55534d', 700: '#2b2a27', 800: '#1f1e1c', 900: '#151413', 950: '#0b0a0a',
}

export default definePreset(Aura, {
  semantic: {
    primary: neutral,
    formField: { borderRadius: '6px' },
    colorScheme: {
      dark: {
        surface: {
          0: '#c5c9c5', 50: '#b5b8b2', 100: '#a6a69c', 200: '#8f9489', 300: '#7d857b', 400: '#737c73',
          500: '#5a5f58', 600: '#403f3b', 700: '#2e2d2a', 800: '#1c1b19', 900: '#121110', 950: '#0b0a0a',
        },
        primary: {
          color: '#c5c9c5', contrastColor: '#0b0a0a', hoverColor: '#dfe2de', activeColor: '#ffffff',
        },
        highlight: { background: 'rgba(139,164,176,0.14)', focusBackground: 'rgba(139,164,176,0.2)', color: '#8ba4b0', focusColor: '#8ba4b0' },
      },
      light: {
        surface: {
          0: '#ffffff', 50: '#f4f2ee', 100: '#ece9e3', 200: '#dedad2', 300: '#c9c4ba', 400: '#a39f95',
          500: '#6f6c64', 600: '#55534d', 700: '#2b2a27', 800: '#1f1e1c', 900: '#151413', 950: '#0b0a0a',
        },
        primary: {
          color: '#1f1e1c', contrastColor: '#f4f2ee', hoverColor: '#000000', activeColor: '#000000',
        },
        highlight: { background: 'rgba(77,112,128,0.12)', focusBackground: 'rgba(77,112,128,0.18)', color: '#4d7080', focusColor: '#4d7080' },
      },
    },
  },
})
