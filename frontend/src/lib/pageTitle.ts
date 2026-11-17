let base = 'Kairo'

// Tab-Titel (label schon übersetzt): "Ansicht · Kairo", im Notizen-Editor "Dokument · Kairo".
export function setViewTitle(label?: string) {
  base = label ? `${label} · Kairo` : 'Kairo'
  document.title = base
}

// Dateiname ohne Ordner und Endung; null stellt den Titel der Ansicht wieder her.
export function setDocTitle(path: string | null) {
  const name = path
    ?.split('/')
    .pop()
    ?.replace(/\.[^.]+$/, '')
  document.title = name ? `${name} · Kairo` : base
}
