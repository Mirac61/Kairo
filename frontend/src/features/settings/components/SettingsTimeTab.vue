<script setup lang="ts">
import { computed } from 'vue'
import type { Settings, SettingsFile } from '@/api/client'
import { prefs } from '@/lib/prefs'

defineProps<{
  data: Settings | null
  locked: (k: keyof SettingsFile) => unknown
}>()

const workStart = defineModel<string>('workStart', { default: '' })
const workEnd = defineModel<string>('workEnd', { default: '' })
const timezone = defineModel<string>('timezone', { default: '' })
const leadMin = defineModel<number>('leadMin', { default: 0 })

const HOURS = Array.from({ length: 25 }, (_, h) => h)
const startHours = computed(() => HOURS.filter((h) => h < prefs.dayEnd))
const endHours = computed(() => HOURS.filter((h) => h > prefs.dayStart))
const clock = (h: number) => `${String(h).padStart(2, '0')}:00`
const zones = Intl.supportedValuesOf('timeZone')
</script>

<template>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Kalender') }}</span
      ><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Wochenbeginn') }}</div>
        <div class="st-h">{{ $t('Erster Tag in Woche und Monat.') }}</div>
      </div>
      <div class="seg">
        <button type="button" :aria-pressed="prefs.weekStart === 1" @click="prefs.weekStart = 1">
          {{ $t('Montag') }}
        </button>
        <button type="button" :aria-pressed="prefs.weekStart === 0" @click="prefs.weekStart = 0">
          {{ $t('Sonntag') }}
        </button>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Sichtbarer Tagesbereich') }}</div>
        <div class="st-h">
          {{ $t('Stunden im Zeitraster des Kalenders und im Tagesplan.') }}
        </div>
      </div>
      <div class="st-pair">
        <select v-model.number="prefs.dayStart" class="input" :aria-label="$t('Tagesbereich von')">
          <option v-for="h in startHours" :key="h" :value="h">{{ clock(h) }}</option>
        </select>
        <span class="st-h">{{ $t('bis') }}</span>
        <select v-model.number="prefs.dayEnd" class="input" :aria-label="$t('Tagesbereich bis')">
          <option v-for="h in endHours" :key="h" :value="h">{{ clock(h) }}</option>
        </select>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <span class="st-t">{{ $t('Tage in der Wochenansicht') }}</span>
        <div class="st-h">
          {{ $t('Heute steht in der Mitte, davor und danach je gleich viele Tage.') }}
        </div>
      </div>
      <div class="seg" role="group" :aria-label="$t('Tage in der Wochenansicht')">
        <button
          v-for="n in [5, 7] as const"
          :key="n"
          type="button"
          :aria-pressed="prefs.calDays === n"
          @click="prefs.calDays = n"
        >
          {{ $t('{n} Tage', { n }) }}
        </button>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-cal-habits">{{ $t('Gewohnheiten in der Woche') }}</label>
        <div class="st-h">
          {{
            $t(
              'In der Tagesansicht stehen sie immer, in der Woche füllen sie sonst den Ganztags-Streifen.',
            )
          }}
        </div>
      </div>
      <input
        id="s-cal-habits"
        v-model="prefs.calHabits"
        type="checkbox"
        role="switch"
        class="st-switch"
      />
    </div>
  </div>

  <div v-if="data" class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Arbeitszeit') }}</span
      ><span class="st-scope">{{ $t('Nach Neustart') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Arbeitszeitfenster') }}</div>
        <div class="st-h">
          {{ $t('Daraus berechnet die Startseite die freie Zeit des Tages.') }}
        </div>
        <div v-if="locked('work_start') || locked('work_end')" class="st-h">
          {{ $t('Festgelegt durch') }} <code>KAIRO_WORK_START</code>/<code>KAIRO_WORK_END</code>
        </div>
      </div>
      <div class="st-pair">
        <input
          v-model="workStart"
          type="time"
          class="input"
          :aria-label="$t('Arbeitszeit von')"
          :disabled="!!locked('work_start')"
        />
        <span class="st-h">{{ $t('bis') }}</span>
        <input
          v-model="workEnd"
          type="time"
          class="input"
          :aria-label="$t('Arbeitszeit bis')"
          :disabled="!!locked('work_end')"
        />
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-tz">{{ $t('Zeitzone') }}</label>
        <div class="st-h">
          {{
            $t(
              'Bestimmt, wann ein Tag beginnt und endet, auch für Wiederholungen und den Wochenrückblick.',
            )
          }}
        </div>
        <div v-if="locked('timezone')" class="st-h">
          {{ $t('Festgelegt durch') }} <code>{{ locked('timezone') }}</code>
        </div>
      </div>
      <input
        id="s-tz"
        v-model="timezone"
        class="input st-ctl"
        list="s-zones"
        :disabled="!!locked('timezone')"
        :placeholder="$t('Systemzeitzone')"
      />
      <datalist id="s-zones"><option v-for="z in zones" :key="z" :value="z" /></datalist>
    </div>
  </div>

  <div v-if="data" class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Erinnerungen') }}</span
      ><span class="st-scope">{{ $t('Nach Neustart') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <label class="st-t" for="s-lead">{{ $t('Vorlauf in Minuten') }}</label>
        <div class="st-h">
          {{
            $t(
              'So lange vor Beginn eines Termins oder einer geplanten Aufgabe kommt die ntfy-Meldung, einmal. 0 = bei Beginn.',
            )
          }}
        </div>
        <div v-if="locked('ntfy_lead')" class="st-h">
          {{ $t('Festgelegt durch') }} <code>{{ locked('ntfy_lead') }}</code>
        </div>
      </div>
      <input
        id="s-lead"
        v-model.number="leadMin"
        type="number"
        min="0"
        max="1440"
        step="1"
        class="input st-ctl"
        :disabled="!!locked('ntfy_lead')"
      />
    </div>
  </div>
</template>
