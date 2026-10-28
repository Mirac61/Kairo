<script setup lang="ts">
import type { Settings } from '@/api/client'

defineProps<{
  version: string
  data: Settings | null
  restarting: boolean
}>()

defineEmits<{
  restart: []
}>()
</script>

<template>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Kairo') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Version') }}</div>
      </div>
      <code class="st-val">{{ version || '–' }}</code>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Lizenzen') }}</div>
        <div class="st-h">
          {{ $t('Kairo steht unter MIT. Hinweise zu Excalidraw, React und den Schriften.') }}
        </div>
      </div>
      <a class="btn btn-secondary st-link" href="/licenses.txt" target="_blank" rel="noopener">{{
        $t('Anzeigen')
      }}</a>
    </div>
    <div class="st-row st-col">
      <div class="st-text">
        <div class="st-t">{{ $t('Konfigurationsdatei') }}</div>
        <div class="st-h">
          {{
            $t(
              'Hier speichert Kairo Notizordner, Arbeitszeit und Zeitzone. Umgebungsvariablen haben Vorrang.',
            )
          }}
        </div>
      </div>
      <code class="st-val">{{ data?.path ?? '–' }}</code>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Backend neu starten') }}</div>
        <div class="st-h">
          {{
            data?.restart_needed
              ? $t('Geänderte Werte warten auf einen Neustart.')
              : $t('Alle gespeicherten Werte sind aktiv.')
          }}
        </div>
      </div>
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="restarting"
        @click="$emit('restart')"
      >
        {{ restarting ? $t('Startet neu …') : $t('Neu starten') }}
      </button>
    </div>
  </div>
  <div class="st-group">
    <div class="st-ghead">
      <span class="lbl">{{ $t('Daten') }}</span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Automatische Backups') }}</div>
        <div class="st-h">
          {{
            $t(
              'Beim Start sichert Kairo die Datenbank nach backups/ neben der Datenbank, wenn das letzte Backup älter als einen Tag ist oder ein Update ansteht. Die neuesten 14 bleiben.',
            )
          }}
        </div>
      </div>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Backup von Hand') }}</div>
        <div class="st-h">
          {{
            $t('Im Terminal, schreibt eine Kopie in denselben Ordner oder an einen eigenen Pfad.')
          }}
        </div>
      </div>
      <code class="st-val">kairo backup [datei]</code>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Backup einspielen') }}</div>
        <div class="st-h">
          {{
            $t(
              'Im Terminal bei gestopptem Kairo. Die bisherige Datenbank bleibt als .before-restore-… daneben liegen.',
            )
          }}
        </div>
      </div>
      <code class="st-val">kairo restore &lt;datei&gt;</code>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Export') }}</div>
        <div class="st-h">
          {{
            $t(
              'Alle Daten als JSON, Termine und Aufgaben mit Frist als ICS, Aufgaben als CSV, Notizen als ZIP.',
            )
          }}
        </div>
      </div>
      <span class="row">
        <a
          v-for="f in ['json', 'ics', 'csv', 'zip']"
          :key="f"
          class="btn btn-secondary st-link"
          :href="`/api/export.${f}`"
          download
          >{{ f.toUpperCase() }}</a
        >
      </span>
    </div>
    <div class="st-row">
      <div class="st-text">
        <div class="st-t">{{ $t('Export einspielen') }}</div>
        <div class="st-h">
          {{
            $t('Eine exportierte JSON-Datei wiederherstellen (im Terminal bei gestopptem Kairo).')
          }}
        </div>
      </div>
      <code class="st-val">kairo import &lt;datei&gt;</code>
    </div>
  </div>
</template>
