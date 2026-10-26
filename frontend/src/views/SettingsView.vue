<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { getHealth, getSettings, restartBackend, saveSettings, type Settings, type SettingsFile } from '@/api/client'
import { useLoader } from '@/composables/useLoader'
import { lang, t, type Lang } from '@/lib/i18n'
import { ACCENTS, PALETTES, prefs, type Accent, type Palette, type Theme } from '@/lib/prefs'
import { store } from '@/lib/storage'
import { navItems } from '@/router'

// Aufbau wie die Notizen: links die Abschnitte, rechts der gewählte. Der Abschnitt bleibt für den nächsten Besuch.
const SECTIONS = [
  { id: 'general', label: 'Allgemein', icon: 'gear' },
  { id: 'look', label: 'Darstellung', icon: 'sun' },
  { id: 'time', label: 'Zeit & Kalender', icon: 'cal' },
  { id: 'notes', label: 'Notizen', icon: 'note' },
  { id: 'system', label: 'System', icon: 'grid' },
] as const
type Section = (typeof SECTIONS)[number]['id']
const tab = ref<Section>(SECTIONS.find((s) => s.id === store.get('kairo-settings-tab'))?.id ?? 'general')
watch(tab, (v) => store.set('kairo-settings-tab', v))
const current = computed(() => SECTIONS.find((s) => s.id === tab.value)!)

const THEMES: { id: Theme; label: string }[] = [{ id: 'dark', label: 'Dunkel' }, { id: 'light', label: 'Hell' }, { id: 'system', label: 'System' }]
const PALETTE_NAMES: Record<Palette, string> = { neutral: 'Neutral', warm: 'Warm', slate: 'Kühl', plum: 'Pflaume' }
const ACCENT_NAMES: Record<Accent, string> = { blue: 'Blau', violet: 'Violett', aqua: 'Türkis', pink: 'Rosa', orange: 'Orange' }
const HOURS = Array.from({ length: 25 }, (_, h) => h)
const startHours = computed(() => HOURS.filter((h) => h < prefs.dayEnd))
const endHours = computed(() => HOURS.filter((h) => h > prefs.dayStart))
// Steht im Script, weil Vue {{ … }} im Template als Ausdruck liest.
const templateHint = computed(() => t('Notizen im Ordner „Vorlagen“ bietet Kairo in jeder leeren Notiz an. Platzhalter wie {{titel}} und {{datum}} werden ersetzt.'))
const clock = (h: number) => `${String(h).padStart(2, '0')}:00`

const data = ref<Settings | null>(null)
const form = ref<SettingsFile>({})
const version = ref('')
const restarting = ref(false)
const zones = Intl.supportedValuesOf('timeZone')
// Leere Felder im Formular bedeuten Standardwert, wie in config.json.
const dirty = computed(() => !!data.value && (['notes_dir', 'work_start', 'work_end', 'timezone', 'ntfy_lead'] as const).some((k) => (form.value[k] ?? '') !== (data.value!.settings[k] ?? '')))

const { error, load, run } = useLoader(async () => {
  data.value = await getSettings()
  form.value = { ...data.value.settings }
})

const save = () => run(() => saveSettings(form.value))

// Das Backend startet sich selbst neu; danach die Seite neu laden, damit alle Ansichten die neuen Werte holen.
async function restart() {
  restarting.value = true
  await run(restartBackend)
  for (let i = 0; i < 40; i++) {
    await new Promise((r) => setTimeout(r, 500))
    try {
      await getHealth()
      location.reload()
      return
    } catch { /* noch nicht wieder da */ }
  }
  restarting.value = false
  error.value = t('Das Backend ist nach dem Neustart nicht erreichbar.')
}

const locked = (k: keyof SettingsFile) => data.value?.env[k]

// Vorlauf der ntfy-Erinnerung in Minuten; das Backend liefert eine Go-Dauer wie „10m0s“, geschrieben wird dieselbe Form.
const leadMin = computed({
  get: () => [...(form.value.ntfy_lead ?? '').matchAll(/(\d+)([hm])/g)].reduce((n, [, v, u]) => n + Number(v) * (u === 'h' ? 60 : 1), 0),
  set: (v: number) => {
    const m = Math.max(0, Math.round(v || 0))
    form.value.ntfy_lead = m === 0 ? '0s' : `${m >= 60 ? `${Math.floor(m / 60)}h` : ''}${m % 60}m0s`
  },
})

// Die Sprache gilt sofort und auf allen Geräten (über /api/prefs); sie steht nicht in config.json.
function setLang(v: Lang) {
  lang.value = v
  store.set('kairo-lang', v)
}

onMounted(() => {
  void load()
  getHealth().then((h) => (version.value = h.version), () => {})
})
</script>

<template>
  <div class="st">
    <aside class="st-pane">
      <div class="st-head"><span class="lbl">{{ $t('Einstellungen') }}</span></div>
      <nav class="st-nav" :aria-label="$t('Abschnitte')">
        <button
          v-for="s in SECTIONS" :key="s.id" type="button" class="nav-item"
          :aria-current="tab === s.id ? 'page' : undefined" @click="tab = s.id"
        ><svg class="ic"><use :href="`#i-${s.icon}`" /></svg><span>{{ $t(s.label) }}</span></button>
      </nav>
    </aside>

    <section class="st-main">
      <header class="ed-head">
        <div class="ed-path"><span>{{ $t('Einstellungen') }}</span><span class="last">{{ $t(current.label) }}</span></div>
        <span v-if="dirty" class="ed-dirty"><i></i>{{ $t('Nicht gespeichert') }}</span>
      </header>

      <div class="st-scroll">
        <div class="st-body">
          <div v-if="error" class="badge st-err" role="alert">{{ error }}</div>

          <template v-if="tab === 'general'">
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Allgemein') }}</span><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span></div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Sprache') }}</div><div class="st-h">{{ $t('Sprache der Oberfläche. Die Schnelleingabe versteht beide.') }}</div></div>
                <div class="seg">
                  <button type="button" :aria-pressed="lang === 'de'" @click="setLang('de')">Deutsch</button>
                  <button type="button" :aria-pressed="lang === 'en'" @click="setLang('en')">English</button>
                </div>
              </div>
              <div class="st-row">
                <div class="st-text"><label class="st-t" for="s-start">{{ $t('Startseite') }}</label><div class="st-h">{{ $t('Diese Ansicht öffnet Kairo beim Start.') }}</div></div>
                <select id="s-start" v-model="prefs.start" class="input st-ctl">
                  <option v-for="n in navItems.filter((n) => n.path !== '/trash')" :key="n.path" :value="n.path">{{ $t(n.label) }}</option>
                </select>
              </div>
            </div>
          </template>

          <template v-else-if="tab === 'look'">
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Theme') }}</span><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span></div>
              <div class="st-themes" role="radiogroup" :aria-label="$t('Theme')">
                <button
                  v-for="th in THEMES" :key="th.id" type="button" role="radio" class="st-theme" :class="`th-${th.id}`"
                  :aria-checked="prefs.theme === th.id" @click="prefs.theme = th.id"
                >
                  <span class="th-prev" aria-hidden="true">
                    <span class="th-side"></span>
                    <span class="th-page"><i class="th-bar"></i><i class="th-line"></i><i class="th-line short"></i><i class="th-acc"></i></span>
                  </span>
                  <span class="th-name">{{ $t(th.label) }}</span>
                </button>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Farbstimmung') }}</div><div class="st-h">{{ $t('Temperatur der Flächen und Texte. Der Akzent bleibt separat.') }}</div></div>
                <div class="st-palettes" role="radiogroup" :aria-label="$t('Farbstimmung')">
                  <button
                    v-for="p in PALETTES" :key="p" type="button" role="radio" class="st-palette" :class="`pal-${p}`"
                    :aria-checked="prefs.palette === p" @click="prefs.palette = p"
                  >
                    <span class="pal-dots" aria-hidden="true"><i></i><i></i><i></i></span>
                    <span class="pal-name">{{ $t(PALETTE_NAMES[p]) }}</span>
                  </button>
                </div>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Akzentfarbe') }}</div><div class="st-h">{{ $t('Für Hauptaktionen, Auswahl, erfasste Zeit und die Jetzt-Linie.') }}</div></div>
                <div class="st-swatches" role="radiogroup" :aria-label="$t('Akzentfarbe')">
                  <button
                    v-for="(hue, a) in ACCENTS" :key="a" type="button" role="radio" class="st-swatch" :style="{ '--sw': `oklch(0.68 0.16 ${hue})` }"
                    :aria-checked="prefs.accent === a" :aria-label="$t(ACCENT_NAMES[a])" :data-tip="$t(ACCENT_NAMES[a])" @click="prefs.accent = a"
                  ></button>
                </div>
              </div>
              <div class="st-row">
                <div class="st-text"><label class="st-t" for="s-motion">{{ $t('Bewegung reduzieren') }}</label><div class="st-h">{{ $t('Schaltet Übergänge und Animationen ab.') }}</div></div>
                <input id="s-motion" v-model="prefs.motion" type="checkbox" role="switch" class="st-switch" />
              </div>
            </div>
          </template>

          <template v-else-if="tab === 'time'">
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Kalender') }}</span><span class="st-scope">{{ $t('Sofort, alle Geräte') }}</span></div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Wochenbeginn') }}</div><div class="st-h">{{ $t('Erster Tag in Woche und Monat.') }}</div></div>
                <div class="seg">
                  <button type="button" :aria-pressed="prefs.weekStart === 1" @click="prefs.weekStart = 1">{{ $t('Montag') }}</button>
                  <button type="button" :aria-pressed="prefs.weekStart === 0" @click="prefs.weekStart = 0">{{ $t('Sonntag') }}</button>
                </div>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Sichtbarer Tagesbereich') }}</div><div class="st-h">{{ $t('Stunden im Zeitraster des Kalenders und im Tagesplan.') }}</div></div>
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
                <div class="st-text"><span class="st-t">{{ $t('Tage in der Wochenansicht') }}</span><div class="st-h">{{ $t('Heute steht in der Mitte, davor und danach je gleich viele Tage.') }}</div></div>
                <div class="seg" role="group" :aria-label="$t('Tage in der Wochenansicht')">
                  <button v-for="n in ([5, 7] as const)" :key="n" type="button" :aria-pressed="prefs.calDays === n" @click="prefs.calDays = n">{{ $t('{n} Tage', { n }) }}</button>
                </div>
              </div>
              <div class="st-row">
                <div class="st-text"><label class="st-t" for="s-cal-habits">{{ $t('Gewohnheiten in der Woche') }}</label><div class="st-h">{{ $t('In der Tagesansicht stehen sie immer, in der Woche füllen sie sonst den Ganztags-Streifen.') }}</div></div>
                <input id="s-cal-habits" v-model="prefs.calHabits" type="checkbox" role="switch" class="st-switch" />
              </div>
            </div>

            <div v-if="data" class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Arbeitszeit') }}</span><span class="st-scope">{{ $t('Nach Neustart') }}</span></div>
              <div class="st-row">
                <div class="st-text">
                  <div class="st-t">{{ $t('Arbeitszeitfenster') }}</div>
                  <div class="st-h">{{ $t('Daraus berechnet die Startseite die freie Zeit des Tages.') }}</div>
                  <div v-if="locked('work_start') || locked('work_end')" class="st-h">{{ $t('Festgelegt durch') }} <code>KAIRO_WORK_START</code>/<code>KAIRO_WORK_END</code></div>
                </div>
                <div class="st-pair">
                  <input v-model="form.work_start" type="time" class="input" :aria-label="$t('Arbeitszeit von')" :disabled="!!locked('work_start')" />
                  <span class="st-h">{{ $t('bis') }}</span>
                  <input v-model="form.work_end" type="time" class="input" :aria-label="$t('Arbeitszeit bis')" :disabled="!!locked('work_end')" />
                </div>
              </div>
              <div class="st-row">
                <div class="st-text">
                  <label class="st-t" for="s-tz">{{ $t('Zeitzone') }}</label>
                  <div class="st-h">{{ $t('Bestimmt, wann ein Tag beginnt und endet, auch für Wiederholungen und den Wochenrückblick.') }}</div>
                  <div v-if="locked('timezone')" class="st-h">{{ $t('Festgelegt durch') }} <code>{{ locked('timezone') }}</code></div>
                </div>
                <input id="s-tz" v-model="form.timezone" class="input st-ctl" list="s-zones" :disabled="!!locked('timezone')" :placeholder="$t('Systemzeitzone')" />
                <datalist id="s-zones"><option v-for="z in zones" :key="z" :value="z" /></datalist>
              </div>
            </div>

            <div v-if="data" class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Erinnerungen') }}</span><span class="st-scope">{{ $t('Nach Neustart') }}</span></div>
              <div class="st-row">
                <div class="st-text">
                  <label class="st-t" for="s-lead">{{ $t('Vorlauf in Minuten') }}</label>
                  <div class="st-h">{{ $t('So lange vor Beginn eines Termins oder einer geplanten Aufgabe kommt die ntfy-Meldung, einmal. 0 = bei Beginn.') }}</div>
                  <div v-if="locked('ntfy_lead')" class="st-h">{{ $t('Festgelegt durch') }} <code>{{ locked('ntfy_lead') }}</code></div>
                </div>
                <input id="s-lead" v-model.number="leadMin" type="number" min="0" max="1440" step="1" class="input st-ctl" :disabled="!!locked('ntfy_lead')" />
              </div>
            </div>
          </template>

          <template v-else-if="tab === 'notes'">
            <div v-if="data" class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Ablage') }}</span><span class="st-scope">{{ $t('Nach Neustart') }}</span></div>
              <div class="st-row st-col">
                <div class="st-text">
                  <label class="st-t" for="s-notes">{{ $t('Notizordner') }}</label>
                  <div class="st-h">{{ $t('Markdown, PDFs und Bilder liegen als normale Dateien hier, VSCodium arbeitet im selben Ordner.') }}</div>
                </div>
                <input id="s-notes" v-model="form.notes_dir" class="input mono" :disabled="!!locked('notes_dir')" :placeholder="$t('Absoluter Pfad oder ~/…, muss existieren')" />
                <div v-if="locked('notes_dir')" class="st-h">{{ $t('Festgelegt durch') }} <code>{{ locked('notes_dir') }}</code></div>
              </div>
            </div>
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Gut zu wissen') }}</span></div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Vorlagen') }}</div><div class="st-h">{{ templateHint }}</div></div>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Papierkorb') }}</div><div class="st-h">{{ $t('Gelöschte Notizen landen in .trash im Notizordner.') }}</div></div>
              </div>
            </div>
          </template>

          <template v-else>
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Kairo') }}</span></div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Version') }}</div></div>
                <code class="st-val">{{ version || '–' }}</code>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Lizenzen') }}</div><div class="st-h">{{ $t('Kairo steht unter MIT. Hinweise zu Excalidraw, React und den Schriften.') }}</div></div>
                <a class="btn btn-secondary st-link" href="/licenses.txt" target="_blank" rel="noopener">{{ $t('Anzeigen') }}</a>
              </div>
              <div class="st-row st-col">
                <div class="st-text"><div class="st-t">{{ $t('Konfigurationsdatei') }}</div><div class="st-h">{{ $t('Hier speichert Kairo Notizordner, Arbeitszeit und Zeitzone. Umgebungsvariablen haben Vorrang.') }}</div></div>
                <code class="st-val">{{ data?.path ?? '–' }}</code>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Backend neu starten') }}</div><div class="st-h">{{ data?.restart_needed ? $t('Geänderte Werte warten auf einen Neustart.') : $t('Alle gespeicherten Werte sind aktiv.') }}</div></div>
                <button type="button" class="btn btn-secondary" :disabled="restarting" @click="restart">{{ restarting ? $t('Startet neu …') : $t('Neu starten') }}</button>
              </div>
            </div>
            <div class="st-group">
              <div class="st-ghead"><span class="lbl">{{ $t('Daten') }}</span></div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Automatische Backups') }}</div><div class="st-h">{{ $t('Beim Start sichert Kairo die Datenbank nach backups/ neben der Datenbank, wenn das letzte Backup älter als einen Tag ist oder ein Update ansteht. Die neuesten 14 bleiben.') }}</div></div>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Backup von Hand') }}</div><div class="st-h">{{ $t('Im Terminal, schreibt eine Kopie in denselben Ordner oder an einen eigenen Pfad.') }}</div></div>
                <code class="st-val">kairo backup [datei]</code>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Backup einspielen') }}</div><div class="st-h">{{ $t('Im Terminal bei gestopptem Kairo. Die bisherige Datenbank bleibt als .before-restore-… daneben liegen.') }}</div></div>
                <code class="st-val">kairo restore &lt;datei&gt;</code>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Export') }}</div><div class="st-h">{{ $t('Alle Daten als JSON, Termine und Aufgaben mit Frist als ICS, Aufgaben als CSV, Notizen als ZIP.') }}</div></div>
                <span class="row">
                  <a v-for="f in ['json', 'ics', 'csv', 'zip']" :key="f" class="btn btn-secondary st-link" :href="`/api/export.${f}`" download>{{ f.toUpperCase() }}</a>
                </span>
              </div>
              <div class="st-row">
                <div class="st-text"><div class="st-t">{{ $t('Export einspielen') }}</div><div class="st-h">{{ $t('Eine exportierte JSON-Datei wiederherstellen (im Terminal bei gestopptem Kairo).') }}</div></div>
                <code class="st-val">kairo import &lt;datei&gt;</code>
              </div>
            </div>
          </template>
        </div>
      </div>

      <div v-if="dirty || (data?.restart_needed && tab !== 'system')" class="st-bar" role="status">
        <span class="st-h">{{ dirty ? $t('Änderungen gelten nach einem Neustart.') : $t('Gespeichert. Die neuen Werte gelten nach dem Neustart.') }}</span>
        <span class="row">
          <button v-if="dirty" type="button" class="btn btn-ghost" @click="form = { ...data!.settings }">{{ $t('Verwerfen') }}</button>
          <button v-if="dirty" type="button" class="btn btn-primary" @click="save">{{ $t('Speichern') }}</button>
          <button v-else type="button" class="btn btn-secondary" :disabled="restarting" @click="restart">{{ restarting ? $t('Startet neu …') : $t('Jetzt neu starten') }}</button>
        </span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.st-link { text-decoration: none; }
.st { display: grid; grid-template-columns: 260px minmax(0, 1fr); height: 100%; }
.st-pane { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--br-subtle); background: var(--bg-1); }
.st-head { display: flex; align-items: center; height: 48px; padding: 0 16px; border-bottom: 1px solid var(--br-subtle); }
.st-nav { display: flex; flex-direction: column; gap: 2px; padding: 8px; }
.st-nav .nav-item { width: 100%; text-align: left; }

.st-main { position: relative; display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.ed-head { display: flex; align-items: center; gap: 12px; height: 48px; padding: 0 16px; border-bottom: 1px solid var(--br-subtle); }
.ed-path { flex: 1; min-width: 0; display: flex; align-items: center; overflow: hidden; white-space: nowrap; font: 400 13px/1 var(--font-ui); color: var(--tx-muted); }
.ed-path span + span::before { content: '/'; margin: 0 6px; color: var(--tx-disabled); }
.ed-path .last { color: var(--tx-primary); font-weight: 500; }
.ed-dirty { display: inline-flex; align-items: center; gap: 6px; font: 400 12px/1 var(--font-ui); color: var(--tx-secondary); }
.ed-dirty i { width: 6px; height: 6px; border-radius: 50%; background: var(--a-blue-hi); }

.st-scroll { flex: 1; overflow-y: auto; }
.st-body { display: grid; gap: 24px; max-width: 720px; padding: 28px 32px 48px; }
.st-err { justify-self: start; color: var(--a-red); white-space: normal; }

.st-group { border: 1px solid var(--br-subtle); border-radius: var(--r-m); background: var(--bg-1); }
.st-ghead { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--br-subtle); }
.st-scope { font: 400 12px/1 var(--font-ui); color: var(--tx-muted); }
.st-row { display: flex; align-items: center; justify-content: space-between; gap: 24px; min-height: 60px; padding: 14px 16px; }
.st-row + .st-row, .st-themes + .st-row { border-top: 1px solid var(--br-subtle); }
.st-row.st-col { flex-direction: column; align-items: stretch; gap: 10px; }
.st-text { display: grid; gap: 4px; min-width: 0; }
.st-t { font: 500 14px/1.3 var(--font-ui); color: var(--tx-primary); }
.st-h { font: 400 13px/1.45 var(--font-ui); color: var(--tx-muted); }
.st-ctl { width: 220px; flex-shrink: 0; }
.st-pair { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.st-pair .input { width: 100px; }
.st-val { font: 400 12px/1.4 var(--font-mono); color: var(--tx-secondary); overflow-wrap: anywhere; }
.st-row:not(.st-col) > .st-val { flex-shrink: 0; white-space: nowrap; }
.mono { font-family: var(--font-mono); font-size: 13px; }

/* Theme-Kacheln: kleine Vorschau mit den Farben des jeweiligen Themes; der Akzent folgt der Wahl darunter. */
.st-themes { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; padding: 16px; }
.st-theme { display: grid; gap: 8px; padding: 8px; border: 1px solid var(--br-default); border-radius: var(--r-m); text-align: left; transition: border-color var(--dur) ease-out, box-shadow var(--dur) ease-out; }
.st-theme:hover { border-color: var(--br-strong); }
.st-theme[aria-checked='true'] { border-color: var(--a-blue-hi); box-shadow: 0 0 0 1px var(--a-blue-hi); }
.th-name { padding: 0 2px 2px; font: 500 13px/1 var(--font-ui); color: var(--tx-primary); }
.th-prev { --p0: #000; --p1: #0b0b0b; --p2: #1e1e1e; display: flex; height: 76px; overflow: hidden; border-radius: var(--r-s); border: 1px solid var(--br-subtle); background: var(--p0); }
.th-light .th-prev { --p0: #fafafa; --p1: #fff; --p2: #e6e6e6; }
.th-system .th-prev { background: linear-gradient(135deg, #000 50%, #fafafa 50%); }
.th-system .th-side { background: transparent; }
.th-side { width: 28%; background: var(--p1); border-right: 1px solid var(--p2); }
.th-page { position: relative; flex: 1; display: flex; flex-direction: column; gap: 6px; padding: 10px; }
.th-page i { display: block; height: 6px; border-radius: 3px; background: var(--p2); }
.th-page .th-bar { width: 50%; height: 8px; }
.th-page .short { width: 60%; }
.th-page .th-acc { position: absolute; right: 10px; bottom: 10px; width: 22px; height: 10px; background: var(--a-blue); }
.th-system .th-page i:not(.th-acc) { background: #8f8f8f55; }

/* Farbstimmung: drei Farbflächen als Vorschau, darunter der Name. */
.st-palettes { display: flex; flex-wrap: wrap; gap: 8px; flex-shrink: 0; }
.st-palette { display: grid; gap: 6px; padding: 7px 8px 6px; border: 1px solid var(--br-default); border-radius: var(--r-s); background: var(--bg-1); transition: border-color var(--dur) ease-out, box-shadow var(--dur) ease-out; }
.st-palette:hover { border-color: var(--br-strong); }
.st-palette[aria-checked='true'] { border-color: var(--a-blue-hi); box-shadow: 0 0 0 1px var(--a-blue-hi); }
.pal-dots { display: flex; gap: 3px; }
.pal-dots i { width: 15px; height: 15px; border-radius: 3px; box-shadow: inset 0 0 0 1px rgba(255,255,255,.07); }
.pal-neutral { --q1: #000; --q2: #2a2a2a; --q3: #f2f2f2; }
.pal-warm { --q1: #0d0c0b; --q2: #332f2b; --q3: #d6d2c8; }
.pal-slate { --q1: #080c12; --q2: #2a3745; --q3: #dce5ef; }
.pal-plum { --q1: #191724; --q2: #3a3552; --q3: #e0def4; }
:root[data-theme="light"] .pal-neutral { --q1: #fafafa; --q2: #d1d1d1; --q3: #171717; }
:root[data-theme="light"] .pal-warm { --q1: #f3efe6; --q2: #d2caba; --q3: #2a2720; }
:root[data-theme="light"] .pal-slate { --q1: #f3f6fa; --q2: #cfd8e3; --q3: #1c2530; }
:root[data-theme="light"] .pal-plum { --q1: #f7f4fb; --q2: #d5cbe5; --q3: #241f2e; }
.pal-dots i:nth-child(1) { background: var(--q1); }
.pal-dots i:nth-child(2) { background: var(--q2); }
.pal-dots i:nth-child(3) { background: var(--q3); }
.pal-name { font: 500 12px/1 var(--font-ui); color: var(--tx-secondary); }
.st-palette[aria-checked='true'] .pal-name { color: var(--tx-primary); }

.st-swatches { display: flex; gap: 10px; flex-shrink: 0; }
.st-swatch { width: 24px; height: 24px; border-radius: 50%; background: var(--sw); box-shadow: 0 0 0 1px var(--br-default); transition: box-shadow var(--dur) ease-out; }
.st-swatch[aria-checked='true'] { box-shadow: 0 0 0 2px var(--bg-1), 0 0 0 4px var(--sw); }
.st-swatch:focus-visible, .st-theme:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }

.st-switch { appearance: none; position: relative; flex-shrink: 0; width: 36px; height: 20px; border-radius: 10px; background: var(--bg-3); border: 1px solid var(--br-default); cursor: pointer; transition: background var(--dur) ease-out; }
.st-switch::after { content: ''; position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: var(--tx-secondary); transition: transform var(--dur) ease-out, background var(--dur) ease-out; }
.st-switch:checked { background: var(--a-blue); border-color: transparent; }
.st-switch:checked::after { transform: translateX(16px); background: #fff; }
.st-switch:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--ring); }

.st-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 10px 16px 10px 32px; border-top: 1px solid var(--br-subtle); background: var(--bg-1); }

@media (max-width: 760px) {
  .st { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
  .st-pane { border-right: 0; border-bottom: 1px solid var(--br-subtle); }
  .st-head { display: none; }
  .st-nav { flex-direction: row; overflow-x: auto; }
  .st-nav .nav-item { width: auto; flex-shrink: 0; }
  .st-body { padding: 20px 16px 40px; }
  .st-bar { padding-left: 16px; }
  .st-row { flex-direction: column; align-items: stretch; gap: 10px; }
  .st-ctl { width: 100%; }
  .st-themes { grid-template-columns: 1fr 1fr 1fr; gap: 8px; padding: 12px; }
}
</style>
