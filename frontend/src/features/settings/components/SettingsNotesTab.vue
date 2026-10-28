<script setup lang="ts">
import { computed } from 'vue'
import type { Settings, SettingsFile } from '@/api/client'
import { t } from '@/lib/i18n'

defineProps<{
  data: Settings | null
  locked: (k: keyof SettingsFile) => unknown
}>()

const notesDir = defineModel<string>('notesDir', { default: '' })

// Steht im Script, weil Vue {{ … }} im Template als Ausdruck liest.
const templateHint = computed(() =>
  t(
    'Notizen im Ordner „Vorlagen“ bietet Kairo in jeder leeren Notiz an. Platzhalter wie {{titel}} und {{datum}} werden ersetzt.',
  ),
)
</script>

<template>
  <div v-if="data" class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Ablage') }}</span
      ><span class="st-scope">{{ $t('Nach Neustart') }}</span>
    </div>
    <div class="st-row st-col">
      <div class="st-text">
        <label class="st-t" for="s-notes">{{ $t('Notizordner') }}</label>
        <div class="st-h">
          {{
            $t(
              'Markdown, PDFs und Bilder liegen als normale Dateien hier, VSCodium arbeitet im selben Ordner.',
            )
          }}
        </div>
      </div>
      <input
        id="s-notes"
        v-model="notesDir"
        class="input mono"
        :disabled="!!locked('notes_dir')"
        :placeholder="$t('Absoluter Pfad oder ~/…, muss existieren')"
      />
      <div v-if="locked('notes_dir')" class="st-h">
        {{ $t('Festgelegt durch') }} <code>{{ locked('notes_dir') }}</code>
      </div>
    </div>
  </div>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Gut zu wissen') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Vorlagen') }}</div>
        <div class="st-h">{{ templateHint }}</div>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Papierkorb') }}</div>
        <div class="st-h">
          {{ $t('Gelöschte Notizen landen in .trash im Notizordner.') }}
        </div>
      </div>
    </div>
  </div>
</template>
