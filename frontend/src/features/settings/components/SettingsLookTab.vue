<script setup lang="ts">
import { ACCENTS, PALETTES, prefs, type Accent, type Palette, type Theme } from '@/lib/prefs'

const THEMES: { id: Theme; label: string }[] = [
  { id: 'dark', label: 'Dunkel' },
  { id: 'light', label: 'Hell' },
  { id: 'system', label: 'System' },
]
const PALETTE_NAMES: Record<Palette, string> = {
  graphit: 'Graphit',
  slate2: 'Schiefer',
  neutral: 'Neutral',
  carbonfox: 'Carbonfox',
  dragon: 'Kanagawa Dragon',
}
const ACCENT_NAMES: Record<Accent, string> = {
  blue: 'Blau',
  violet: 'Violett',
  aqua: 'Türkis',
  pink: 'Rosa',
  orange: 'Orange',
}
</script>

<template>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Theme') }}</span
      ><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span>
    </div>
    <div class="st-themes" role="radiogroup" :aria-label="$t('Theme')">
      <button
        v-for="th in THEMES"
        :key="th.id"
        type="button"
        role="radio"
        class="st-theme"
        :class="`th-${th.id}`"
        :aria-checked="prefs.theme === th.id"
        @click="prefs.theme = th.id"
      >
        <span class="th-prev" aria-hidden="true">
          <span class="th-side"></span>
          <span class="th-page"
            ><i class="th-bar"></i><i class="th-line"></i><i class="th-line short"></i
            ><i class="th-acc"></i
          ></span>
        </span>
        <span class="th-name">{{ $t(th.label) }}</span>
      </button>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Farbstimmung') }}</div>
        <div class="st-h">
          {{
            $t(
              'Flächen, Texte und Farben. Carbonfox und Kanagawa Dragon bringen ihren eigenen Akzent mit.',
            )
          }}
        </div>
      </div>
      <div class="st-palettes" role="radiogroup" :aria-label="$t('Farbstimmung')">
        <button
          v-for="p in PALETTES"
          :key="p"
          type="button"
          role="radio"
          class="st-palette"
          :class="`pal-${p}`"
          :aria-checked="prefs.palette === p"
          @click="prefs.palette = p"
        >
          <span class="pal-dots" aria-hidden="true"><i></i><i></i><i></i></span>
          <span class="pal-name">{{ $t(PALETTE_NAMES[p]) }}</span>
        </button>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Akzentfarbe') }}</div>
        <div class="st-h">
          {{ $t('Für Hauptaktionen, Auswahl, erfasste Zeit und die Jetzt-Linie.') }}
        </div>
      </div>
      <div class="st-swatches" role="radiogroup" :aria-label="$t('Akzentfarbe')">
        <button
          v-for="(hue, a) in ACCENTS"
          :key="a"
          type="button"
          role="radio"
          class="st-swatch"
          :style="{ '--sw': `oklch(0.68 0.16 ${hue})` }"
          :aria-checked="prefs.accent === a"
          :aria-label="$t(ACCENT_NAMES[a])"
          :data-tip="$t(ACCENT_NAMES[a])"
          @click="prefs.accent = a"
        ></button>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-motion">{{ $t('Bewegung reduzieren') }}</label>
        <div class="st-h">{{ $t('Schaltet Übergänge und Animationen ab.') }}</div>
      </div>
      <input id="s-motion" v-model="prefs.motion" type="checkbox" role="switch" class="st-switch" />
    </div>
  </div>
</template>
