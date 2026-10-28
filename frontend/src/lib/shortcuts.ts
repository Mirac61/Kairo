// Alle Tastenkürzel an einer Stelle: die ?-Übersicht (components/ShortcutHelp.vue) liest diese Tabelle.
// Wer in einer Ansicht ein Kürzel ergänzt oder ändert, trägt es hier mit ein. Texte laufen beim Anzeigen durch $t.

// g und dann ein Buchstabe springt in eine Ansicht.
export const GOTO: Record<string, string> = {
  s: '/today',
  k: '/calendar',
  a: '/tasks',
  h: '/habits',
  p: '/projects',
  n: '/notes',
  z: '/goals',
  j: '/journal',
  l: '/cards',
  w: '/review',
  e: '/settings',
}

export const SHORTCUTS: { title: string; keys: [string, string][] }[] = [
  {
    title: 'Überall',
    keys: [
      ['⌘ K', 'Suchen oder neu anlegen'],
      ['?', 'Diese Übersicht'],
      ['/', 'Suchfeld der Ansicht'],
    ],
  },
  {
    title: 'Aufgaben',
    keys: [
      ['n', 'Neue Aufgabe'],
      ['j / k', 'Nächste / vorige'],
      ['Enter', 'Öffnen'],
      ['x', 'Erledigt'],
      ['s', 'Timer starten oder pausieren'],
      ['t', 'Auf heute legen'],
    ],
  },
  {
    title: 'Gewohnheiten',
    keys: [
      ['n', 'Neue Gewohnheit'],
      ['j / k', 'Nächste / vorige'],
      ['x', 'Abhaken'],
      ['← / →', 'Voriger / nächster Tag'],
    ],
  },
  {
    title: 'Kalender',
    keys: [
      ['n', 'Neuer Eintrag'],
      ['t', 'Heute'],
      ['← / →', 'Zurück / weiter'],
      ['m / w / d', 'Monat / Woche / Tag'],
    ],
  },
  {
    title: 'Lernkarten',
    keys: [
      ['1–9', 'Stapel lernen'],
      ['n', 'Neue Karte'],
      ['b', 'Karten durchsuchen'],
      ['Leertaste', 'Antwort zeigen'],
      ['1–4', 'Bewerten'],
      ['z', 'Rückgängig'],
    ],
  },
]
