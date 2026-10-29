<script setup lang="ts">
import { ref } from 'vue'
import { hm } from '@/lib/dates'

withDefaults(
  defineProps<{
    title: string
    week: number
    view: string
    views: readonly (readonly [string, string, string])[]
    spanKey?: string
    isWeek: boolean
    isDay: boolean
    dayStat: { plan: number; ist: number; free: number } | null
    calDays: number
  }>(),
  {
    spanKey: 'timeGridSpan',
  },
)

const emit = defineEmits<{
  (e: 'prev'): void
  (e: 'next'): void
  (e: 'today'): void
  (e: 'changeView', view: string): void
  (e: 'changeDays', days: 5 | 7): void
  (e: 'importFile', file: File): void
  (e: 'newEntry'): void
}>()

const calMenu = ref(false)
const fileInput = ref<HTMLInputElement>()

function openImportIcs() {
  calMenu.value = false
  fileInput.value?.click()
}

function onFileInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) {
    emit('importFile', file)
  }
}

function onSetDays(n: 5 | 7) {
  emit('changeDays', n)
}
</script>

<template>
  <div class="cal-toolbar">
    <div class="cal-nav">
      <button
        class="icon-btn"
        :aria-label="$t('Zurück')"
        :title="$t('Zurück (←)')"
        @click="$emit('prev')"
      >
        <svg class="ic"><use href="#i-left" /></svg>
      </button>
      <button
        class="icon-btn"
        :aria-label="$t('Weiter')"
        :title="$t('Weiter (→)')"
        @click="$emit('next')"
      >
        <svg class="ic"><use href="#i-right" /></svg>
      </button>
    </div>

    <button
      class="btn btn-ghost"
      :title="$t('Heute (t)')"
      aria-keyshortcuts="t"
      @click="$emit('today')"
    >
      {{ $t('Heute') }}
    </button>

    <h1 class="v-title cal-title">{{ title }}</h1>
    <span v-if="week" class="cal-kw">{{ $t('KW') }} {{ week }}</span>
    <span class="spacer" />

    <div class="seg">
      <button
        v-for="[v, l, k] in views"
        :key="v"
        :aria-pressed="view === v || (v === spanKey && isWeek)"
        :title="`${l} (${k})`"
        :aria-keyshortcuts="k"
        @click="$emit('changeView', v)"
      >
        {{ l }}
      </button>
    </div>

    <input
      ref="fileInput"
      type="file"
      accept=".ics,text/calendar"
      hidden
      @change="onFileInputChange"
    />

    <div class="menu-wrap" @keydown.esc="calMenu = false">
      <button
        class="icon-btn"
        type="button"
        :aria-label="$t('Weitere Aktionen')"
        :aria-expanded="calMenu"
        @click="calMenu = !calMenu"
      >
        <svg class="ic"><use href="#i-more" /></svg>
      </button>

      <template v-if="calMenu">
        <div class="menu-scrim" @click="calMenu = false"></div>
        <div class="menu">
          <div v-if="isWeek" class="menu-row">
            {{ $t('Tage in der Wochenansicht') }}
            <span class="seg" role="group" :aria-label="$t('Tage in der Wochenansicht')">
              <button
                v-for="n in [5, 7] as const"
                :key="n"
                type="button"
                :aria-pressed="calDays === n"
                @click="onSetDays(n)"
              >
                {{ n }}
              </button>
            </span>
          </div>
          <button class="menu-i" type="button" @click="openImportIcs">
            {{ $t('ICS importieren') }} …
          </button>
        </div>
      </template>
    </div>

    <button
      class="btn btn-primary"
      :title="$t('Neuer Eintrag (n)')"
      aria-keyshortcuts="n"
      @click="$emit('newEntry')"
    >
      <svg class="ic"><use href="#i-plus" /></svg>{{ $t('Neuer Eintrag') }}
    </button>
  </div>

  <div v-if="isDay && dayStat" class="cal-stats">
    <span
      >{{ $t('geplant') }} <b>{{ hm(dayStat.plan) }}</b></span
    >
    <span
      >{{ $t('erfasst') }} <b>{{ hm(dayStat.ist) }}</b></span
    >
    <span
      >{{ $t('frei') }} <b>{{ hm(dayStat.free) }}</b></span
    >
  </div>
</template>

<style scoped>
.cal-kw {
  font: 400 13px/1 var(--font-ui);
  color: var(--tx-muted);
}
.cal-stats {
  display: flex;
  gap: 16px;
  padding: 0 10px 12px 56px;
  font: 400 13px/1 var(--font-ui);
  font-variant-numeric: tabular-nums;
  color: var(--tx-muted);
}
.cal-stats b {
  margin-left: 4px;
  font-weight: 600;
  color: var(--tx-primary);
}

@media (max-width: 760px) {
  .cal-toolbar :deep(.btn-secondary) {
    display: none;
  }
  .cal-title {
    flex: 1 1 auto;
    font-size: 18px;
  }
  .cal-toolbar .spacer {
    display: none;
  }
}
</style>
