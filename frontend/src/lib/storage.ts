// localStorage kann fehlen oder werfen (privater Modus, gesperrte Website-Daten). Dann gilt der Standardwert.
export const store = {
  get(key: string): string | null {
    try { return localStorage.getItem(key) } catch { return null }
  },
  set(key: string, value: string) {
    try { localStorage.setItem(key, value) } catch { /* privater Modus */ }
  },
}
