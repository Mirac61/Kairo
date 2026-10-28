// Geometrie fürs Umblättern im Journal: Das Blatt faltet sich entlang einer Linie (Falz), die vom Außenrand
// zum Bund wandert; die untere Ecke läuft etwas voraus. Was hinter dem Falz liegt, klappt gespiegelt über
// den Falz und zeigt die Rückseite des Blatts, also die neue Seite.
//
// Gerechnet wird für das Blättern nach vorn (rechte Seite klappt nach links), in lokalen Koordinaten der
// jeweiligen Seite (W × H, y nach unten). Zurück ist dasselbe, an der Seitenmitte gespiegelt (mirror).
//
// Rollen: cut = die aufgehobene Seite (schrumpft bis zum Falz), flap = ihre Rückseite (die neue Seite auf der
// anderen Hälfte), covered = die Seite, auf die sich das Blatt legt, revealed = die Seite, die frei wird.
export type LeafRole = 'cut' | 'flap' | 'covered' | 'revealed'

export interface LeafFrame {
  clip: string
  transform: string
  shade: string
}

const TILT = 0.12 // größte Neigung des Falzes in rad (~7°); mehr hebt die Ecke über den Buchrand

export const easeTurn = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2)

// Matrix [a c e; b d f] wie CSS matrix(a, b, c, d, e, f)
type M = [number, number, number, number, number, number]

const r = (n: number) => Math.round(n * 100) / 100
const poly = (pts: [number, number][]) => `polygon(${pts.map(([x, y]) => `${r(x)}px ${r(y)}px`).join(', ')})`
const deg = (rad: number) => r((rad * 180) / Math.PI)

export function leafFrame(role: LeafRole, p: number, W: number, H: number, mirror = false): LeafFrame {
  const s = Math.sin(Math.PI * p) // Stärke von Schatten und Neigung, 0 an beiden Enden
  const c = W * (1 - p) // Falz in Höhe der Mitte, Abstand vom Bund
  const a = TILT * s
  const tan = Math.tan(a)
  const xt = c + (H / 2) * tan // Falz oben
  const xb = c - (H / 2) * tan // Falz unten (läuft voraus)
  const cos2 = Math.cos(2 * a)
  const sin2 = Math.sin(2 * a)
  // Länge der Verlaufslinie eines um a geneigten Verlaufs; Stopps werden in px entlang dieser Linie gemessen.
  const L = W * Math.cos(a) + H * Math.abs(Math.sin(a))

  let pts: [number, number][] = [[0, 0], [W, 0], [W, H], [0, H]]
  let m: M = [1, 0, 0, 1, 0, 0]
  let angle = 90
  let stops = 'transparent 0'

  if (role === 'cut') {
    pts = [[0, 0], [xt, 0], [xb, H], [0, H]]
    angle = 90 + deg(a)
    const f = L / 2 + (c - W / 2) * Math.cos(a)
    stops = `transparent ${r(f - 90)}px, rgb(0 0 0 / ${r(0.16 * s)}) ${r(f)}px`
  } else if (role === 'flap') {
    // Spiegelung am Falz nach der Spiegelung am Bund ergibt eine Drehung um 2a plus Verschiebung.
    pts = [[0, 0], [W - xt, 0], [W - xb, H], [0, H]]
    const bx = c + cos2 * c + sin2 * (H / 2)
    const by = H / 2 + sin2 * c - cos2 * (H / 2)
    m = [cos2, sin2, -sin2, cos2, W - W * cos2 + bx, -W * sin2 + by]
    angle = 270 - deg(a)
    const f = L / 2 - (W - c - W / 2) * Math.cos(a)
    // Am Falz wölbt sich das Papier ins Licht, zum Rand hin liegt es flacher und etwas im Schatten.
    stops = `rgb(255 255 255 / ${r(0.22 * s)}) ${r(f)}px, rgb(255 255 255 / 0) ${r(f + 36)}px, rgb(0 0 0 / ${r(0.1 * s)}) ${r(f + W * 0.7)}px`
  } else if (role === 'covered') {
    // Schatten des umklappenden Blatts vor seinem Außenrand
    const edge = -sin2 * (H / 2) + W - W * cos2 + c + cos2 * c + sin2 * (H / 2)
    angle = 90 + deg(2 * a)
    const f = L / 2 + (edge - W / 2)
    stops = `transparent ${r(f - 70)}px, rgb(0 0 0 / ${r(0.2 * s)}) ${r(f)}px`
  } else {
    // Schatten am Falz auf der frei werdenden Seite
    angle = 90 + deg(a)
    const f = L / 2 + (c - W / 2) * Math.cos(a)
    stops = `rgb(0 0 0 / ${r(0.22 * s)}) ${r(f)}px, transparent ${r(f + 80)}px`
  }

  if (mirror) {
    pts = pts.map(([x, y]) => [W - x, y])
    const [ma, mb, mc, md, me, mf] = m
    m = [ma, -mb, -mc, md, W - ma * W - me, mb * W + mf]
    angle = 360 - angle
  }
  return {
    clip: poly(pts),
    // Drehanteile genau runden: 0,01 Fehler wären am Falz schon 1px Versatz.
    transform: `matrix(${m.map((n, i) => (i < 4 ? Math.round(n * 1e5) / 1e5 : r(n))).join(', ')})`,
    shade: `linear-gradient(${r(angle)}deg, ${stops})`,
  }
}
