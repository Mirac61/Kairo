<script setup lang="ts">
import { lang, type Lang } from '@/lib/i18n'
import { prefs } from '@/lib/prefs'
import { store } from '@/lib/storage'
import { navItems } from '@/router'

function setLang(v: Lang) {
  lang.value = v
  store.set('kairo-lang', v)
}
</script>

<template>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Allgemein') }}</span
      ><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Sprache') }}</div>
        <div class="st-h">
          {{ $t('Sprache der Oberfläche. Die Schnelleingabe versteht beide.') }}
        </div>
      </div>
      <div class="seg">
        <button type="button" :aria-pressed="lang === 'de'" @click="setLang('de')">Deutsch</button>
        <button type="button" :aria-pressed="lang === 'en'" @click="setLang('en')">English</button>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-start">{{ $t('Startseite') }}</label>
        <div class="st-h">{{ $t('Diese Ansicht öffnet Kairo beim Start.') }}</div>
      </div>
      <select id="s-start" v-model="prefs.start" class="input st-ctl">
        <option
          v-for="n in navItems.filter((n) => n.path !== '/trash')"
          :key="n.path"
          :value="n.path"
        >
          {{ $t(n.label) }}
        </option>
      </select>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-name">{{ $t('Name') }}</label>
        <div class="st-h">{{ $t('Für die Begrüßung auf der Startseite.') }}</div>
      </div>
      <input
        id="s-name"
        v-model.trim="prefs.name"
        class="input st-ctl"
        type="text"
        autocomplete="given-name"
        :placeholder="$t('z. B. Alex')"
      />
    </div>
  </div>
</template>
