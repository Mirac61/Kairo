<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { markTourSeen } from '@/composables/useTour'

// Rundgang in zwei Teilen: erst ein Willkommensdialog über der abgedunkelten App, dann ein Spotlight, das durch die echte Oberfläche führt.
// Bei Navigationspunkten zählt auch ein Klick auf den markierten Punkt (er wird wirklich ausgelöst).
// Ein Ziel, das auf dem Handy nicht sichtbar ist (Seitenleiste weg, Punkt unter „Mehr“), fällt auf eine zentrierte Karte zurück.
const emit = defineEmits<{ close: [] }>()
const router = useRouter()

type Step = { to?: string; target?: string; side?: 'bottom'; title: string; text: string }
const steps: Step[] = [
  {
    to: '/today',
    title: 'Start',
    text: 'Dein Tag auf einen Blick: was heute ansteht, welche Gewohnheiten offen sind und wie die Zeit verplant ist.',
  },
  {
    to: '/tasks',
    title: 'Aufgaben',
    text: 'Schreib eine Aufgabe in einem Satz, zum Beispiel „Sport 30m @morgen @14:30 #Kairo !hoch“. Dauer, Tag, Uhrzeit, Projekt und Priorität erkennt Kairo selbst.',
  },
  {
    to: '/calendar',
    title: 'Kalender',
    text: 'Termine und geplante Aufgaben in einer Woche. Aufgaben ohne Zeitpunkt ziehst du in einen freien Platz.',
  },
  {
    to: '/habits',
    title: 'Gewohnheiten',
    text: 'Hake täglich ab, was du dir vorgenommen hast. Die Serie zeigt, wie lange du dranbleibst.',
  },
  {
    to: '/notes',
    title: 'Notizen',
    text: 'Markdown-Notizen mit Verknüpfungen. PDFs und Bilder kannst du hier ansehen und bezeichnen; hat eine PDF ein Inhaltsverzeichnis, springst du über die Liste links zur Stelle.',
  },
  {
    target: '.tb-search',
    side: 'bottom',
    title: 'Suchen und neu anlegen',
    text: 'Mit ⌘K (Strg + K) suchst du in allem. Findet die Suche nichts, legt sie daraus eine Aufgabe an.',
  },
  {
    target: '.sb-help',
    title: 'Tastenkürzel',
    text: 'Mit ? siehst du alle Kürzel. Mit g und einem Buchstaben springst du direkt in eine Ansicht, etwa g a für Aufgaben. Diesen Rundgang findest du später in den Einstellungen wieder.',
  },
]

const GAP = 18 // Abstand Karte ↔ Ziel (Platz für den Pfeil)
const MARGIN = 16
const phase = ref<'intro' | 'tour'>('intro')
const i = ref(0) // Schritt im Spotlight
const finished = ref(false) // kurzer Haken nach dem letzten Schritt, dann zu
const step = computed(() => steps[i.value]!)
const last = computed(() => i.value === steps.length - 1)
const rect = ref<DOMRect | null>(null)
const card = ref<HTMLElement>()
const introEl = ref<HTMLElement>()
const cardH = ref(200)
let closeTimer = 0

// Erstes sichtbares Element: Seitenleiste am Desktop, Tableiste am Handy.
function find(st: Step): HTMLElement | null {
  const sel = st.target ?? `.sidebar a[href="${st.to}"], .tabbar a[href="${st.to}"]`
  return (
    [...document.querySelectorAll<HTMLElement>(sel)].find(
      (el) => el.getBoundingClientRect().width > 0,
    ) ?? null
  )
}
function measure() {
  if (phase.value !== 'tour') return
  rect.value = find(step.value)?.getBoundingClientRect() ?? null
  cardH.value = card.value?.offsetHeight ?? cardH.value
}

async function show() {
  if (step.value.to) await router.push(step.value.to)
  await nextTick()
  measure()
  await nextTick()
  measure() // zweites Mal mit der Höhe des neuen Inhalts
  card.value?.querySelector<HTMLElement>('[data-next]')?.focus({ preventScroll: true })
}
watch(i, show)

function done() {
  markTourSeen()
  emit('close')
}
function finish() {
  if (finished.value) return
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return done()
  finished.value = true
  closeTimer = window.setTimeout(done, 1100)
}
function startTour() {
  phase.value = 'tour'
  void show()
}
function next() {
  if (phase.value === 'intro') startTour()
  else if (last.value) finish()
  else i.value++
}
function back() {
  if (phase.value === 'intro' || finished.value) return
  if (i.value > 0) i.value--
  else phase.value = 'intro'
}

// Karte neben dem Ziel (rechts, sonst unten oder oben), der Pfeil zeigt auf dessen Mitte.
const place = computed(() => {
  const r = rect.value
  const W = Math.min(340, innerWidth - 2 * MARGIN)
  const H = cardH.value
  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(v, hi))
  if (!r)
    return { side: 'none', top: (innerHeight - H) / 2, left: (innerWidth - W) / 2, W, arrow: 0 }
  const cx = r.left + r.width / 2
  const cy = r.top + r.height / 2
  if (r.right + GAP + W <= innerWidth - MARGIN && step.value.side !== 'bottom') {
    const top = clamp(cy - H / 2, MARGIN, innerHeight - H - MARGIN)
    return { side: 'right', top, left: r.right + GAP, W, arrow: clamp(cy - top, 24, H - 24) }
  }
  const below = r.bottom + GAP + H <= innerHeight - MARGIN
  const left = clamp(cx - W / 2, MARGIN, innerWidth - W - MARGIN)
  return {
    side: below ? 'bottom' : 'top',
    top: below ? r.bottom + GAP : Math.max(MARGIN, r.top - GAP - H),
    left,
    W,
    arrow: clamp(cx - left, 24, W - 24),
  }
})
const spot = computed(() => {
  const r = rect.value
  const [x, y, w, h] = r
    ? [r.left - 4, r.top - 4, r.width + 8, r.height + 8]
    : [innerWidth / 2, innerHeight / 2, 0, 0]
  return { top: `${y}px`, left: `${x}px`, width: `${w}px`, height: `${h}px` }
})
const cardStyle = computed(() =>
  innerWidth <= 640
    ? {}
    : {
        top: `${place.value.top}px`,
        left: `${place.value.left}px`,
        width: `${place.value.W}px`,
        '--arrow': `${place.value.arrow}px`,
      },
)

// Klick auf den markierten Navigationspunkt löst ihn aus und geht weiter.
function onHit(e: MouseEvent) {
  const r = rect.value
  const el = step.value.to ? find(step.value) : null
  if (!r || !el) return
  if (e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom) {
    el.click()
    next()
  }
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') done()
  else if (e.key === 'ArrowRight') next()
  else if (e.key === 'ArrowLeft') back()
  else return
  e.preventDefault()
  e.stopImmediatePropagation()
}
onMounted(() => {
  window.addEventListener('keydown', onKey, true)
  window.addEventListener('resize', measure)
  void nextTick(() => introEl.value?.querySelector<HTMLElement>('[data-next]')?.focus())
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey, true)
  window.removeEventListener('resize', measure)
  clearTimeout(closeTimer)
})
</script>

<template>
  <div class="tour" role="dialog" aria-modal="true" :aria-label="$t('Rundgang')">
    <!-- ohne out-in: Abdunklung des Dialogs und des Spotlights blenden ineinander, die App blitzt nicht auf -->
    <Transition name="tour-phase">
      <div v-if="phase === 'intro'" ref="introEl" class="tour-intro">
        <div class="dialog tour-welcome">
          <div class="dlg-body">
            <span class="logo-mark tour-logo" aria-hidden="true"><i></i></span>
            <div>
              <h2>{{ $t('Willkommen bei Kairo') }}</h2>
              <p>
                {{
                  $t(
                    'Aufgaben, Kalender, Gewohnheiten und Notizen an einem Ort. Gebaut für Leute, die ihren Tag planen wollen, ohne fünf Apps zu jonglieren.',
                  )
                }}
              </p>
            </div>
          </div>
          <div class="dlg-foot">
            <button
              type="button"
              class="btn btn-secondary"
              :data-tip="$t('Rundgang beenden (Esc)')"
              @click="done"
            >
              {{ $t('Überspringen') }}
            </button>
            <button type="button" class="btn btn-primary" data-next @click="next">
              {{ $t('Rundgang starten') }}
            </button>
          </div>
        </div>
      </div>

      <div v-else class="tour-live">
        <div class="tour-hit" @click="onHit"></div>
        <div class="tour-spot" :class="{ on: rect }" :style="spot"></div>
        <div ref="card" class="tour-card" :class="`side-${place.side}`" :style="cardStyle">
          <span v-if="place.side !== 'none'" class="tour-arrow" aria-hidden="true"></span>
          <button
            type="button"
            class="icon-btn tour-x"
            :aria-label="$t('Rundgang beenden')"
            :data-tip="$t('Rundgang beenden (Esc)')"
            @click="done"
          >
            <svg class="ic"><use href="#i-x" /></svg>
          </button>
          <div v-if="finished" class="tour-body tour-done" role="status">
            <svg class="tour-check" viewBox="0 0 24 24" aria-hidden="true">
              <path pathLength="1" d="M5 12.5l4.5 4.5L19 7.5" />
            </svg>
            <h3>{{ $t('Alles bereit') }}</h3>
          </div>
          <div v-else :key="i" class="tour-body">
            <div class="tour-n">{{ i + 1 }} / {{ steps.length }}</div>
            <h3>{{ $t(step.title) }}</h3>
            <p>{{ $t(step.text) }}</p>
            <p v-if="step.to && rect" class="tour-hint">
              {{ $t('Probier es aus: Klick auf den markierten Punkt.') }}
            </p>
          </div>
          <div v-if="!finished" class="tour-foot">
            <button type="button" class="btn btn-secondary" @click="back">
              {{ $t('Zurück') }}
            </button>
            <span class="tour-gap"></span>
            <button type="button" class="btn btn-primary" data-next @click="next">
              {{ last ? $t('Los geht’s') : $t('Weiter') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.tour {
  --ease: cubic-bezier(0.2, 0.8, 0.2, 1);
  position: fixed;
  inset: 0;
  z-index: 1000;
}

/* ---------- Willkommen: ein Dialog wie überall in Kairo; die Sonne im Logo geht einmal auf */
.tour-intro {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 16px;
  overflow-y: auto;
  background: var(--scrim);
}
.tour-welcome {
  width: 540px;
  /* warmer Schein in der Ecke des Logos, Farbe der Logo-Sonne */
  background:
    radial-gradient(
      90% 70% at 0% 0%,
      color-mix(in oklab, oklch(0.64 0.17 38) 9%, transparent),
      transparent 70%
    ),
    var(--bg-1);
}
.tour-welcome .dlg-body {
  gap: 20px;
  padding: 28px 28px 8px;
}
.tour-welcome .dlg-foot {
  padding: 20px 28px 24px;
}
.tour-logo {
  zoom: 2.25;
}
.tour-logo::before {
  animation: tour-sun 1.1s var(--ease) 0.15s both;
}
.tour-welcome h2 {
  margin: 0 0 8px;
  font: 600 22px/1.25 var(--font-ui);
  letter-spacing: -0.02em;
  color: var(--tx-primary);
}
.tour-welcome p {
  margin: 0;
  font: 400 15px/1.6 var(--font-ui);
  color: var(--tx-secondary);
}

/* ---------- Spotlight durch die echte Oberfläche */
.tour-live,
.tour-hit {
  position: absolute;
  inset: 0;
}
.tour-spot {
  position: absolute;
  border-radius: 10px;
  box-shadow: 0 0 0 9999px rgb(8 10 14 / 0.62);
  pointer-events: none;
  transition:
    top 0.45s var(--ease),
    left 0.45s var(--ease),
    width 0.45s var(--ease),
    height 0.45s var(--ease);
}
.tour-spot.on {
  outline: 2px solid var(--a-blue-hi);
  outline-offset: 0;
}
.tour-card {
  position: absolute;
  box-sizing: border-box;
  max-width: calc(100vw - 32px);
  padding: 18px;
  border: 1px solid var(--br-default);
  border-radius: 12px;
  background: var(--bg-1);
  box-shadow: 0 12px 40px rgb(0 0 0 / 0.35);
  transition:
    top 0.45s var(--ease),
    left 0.45s var(--ease);
}
.tour-arrow {
  position: absolute;
  width: 12px;
  height: 12px;
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  transform: rotate(45deg);
  transition:
    top 0.45s var(--ease),
    left 0.45s var(--ease);
}
.side-right .tour-arrow {
  left: -7px;
  top: calc(var(--arrow) - 6px);
  border-right: 0;
  border-top: 0;
}
.side-bottom .tour-arrow {
  top: -7px;
  left: calc(var(--arrow) - 6px);
  border-right: 0;
  border-bottom: 0;
}
.side-top .tour-arrow {
  bottom: -7px;
  left: calc(var(--arrow) - 6px);
  border-left: 0;
  border-top: 0;
}
.tour-x {
  position: absolute;
  top: 8px;
  right: 8px;
}
.tour-body {
  animation: tour-fade 0.3s ease-out both;
}
.tour-n {
  font: 400 11px/1 var(--font-mono);
  color: var(--tx-muted);
}
.tour-card h3 {
  margin: 8px 0 6px;
  font: 600 15px/1.3 var(--font-ui);
  color: var(--tx-primary);
}
.tour-card p {
  margin: 0;
  font: 400 13px/1.55 var(--font-ui);
  color: var(--tx-secondary);
}
.tour-card .tour-hint {
  margin-top: 10px;
  font-weight: 500;
  color: var(--a-blue-hi);
}
.tour-foot {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}
.tour-gap {
  flex: 1;
}

/* ---------- Abschluss: Kreis springt auf, der Haken zeichnet sich */
.tour-done {
  display: flex;
  align-items: center;
  gap: 12px;
}
.tour-card .tour-done h3 {
  margin: 0;
}
.tour-check {
  flex: none;
  width: 36px;
  height: 36px;
  padding: 7px;
  box-sizing: border-box;
  border-radius: 50%;
  background: color-mix(in srgb, var(--a-blue-hi) 16%, transparent);
  fill: none;
  stroke: var(--a-blue-hi);
  stroke-width: 2.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  animation: tour-pop 0.4s var(--ease) both;
}
.tour-check path {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: tour-draw 0.45s var(--ease) 0.15s forwards;
}

/* ---------- Übergang Willkommen → Spotlight */
.tour-phase-enter-active,
.tour-phase-leave-active {
  transition: opacity 0.3s ease-out;
}
.tour-phase-enter-from,
.tour-phase-leave-to {
  opacity: 0;
}
@keyframes tour-fade {
  from {
    opacity: 0;
  }
}
@keyframes tour-sun {
  from {
    translate: -3px 11px; /* hinter dem Berg */
  }
}
@keyframes tour-pop {
  from {
    opacity: 0;
    scale: 0.6;
  }
}
@keyframes tour-draw {
  to {
    stroke-dashoffset: 0;
  }
}

@media (max-width: 640px) {
  .tour-card {
    top: auto;
    bottom: 88px;
    left: 16px;
    right: 16px;
    width: auto;
  }
  .tour-arrow {
    display: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  .tour *,
  .tour-logo::before,
  .tour-card,
  .tour-spot {
    animation: none !important;
    transition: none !important;
  }
}
</style>
