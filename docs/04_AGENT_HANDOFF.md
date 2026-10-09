# Agent Handoff

## Lies diese Dateien zuerst

1.  `docs/00_VISION.md`
2.  `docs/01_PRODUCT_REQUIREMENTS.md`
3.  `docs/02_ARCHITECTURE.md`
4.  `docs/03_AGENT_GUIDELINES.md`

------------------------------------------------------------------------

# Kurzfassung

Wir bauen **Kairo**, ein lokales Personal OS.

Das Repository liegt unter `~/code/personal/Kairo`.

Es besteht aus:

``` text
WebUI
  → Planung

Go Backend
  → Business Logic + API + Synchronisation

SQLite
  → lokale Daten

VSCodium Extension
  → tatsächlicher Arbeitskontext

Notizordner
  → Markdown, PDFs und Bilder als normale Dateien
```

**Stand:** MVP (Phasen 0–5) und die Phasen 6–7 sind gebaut, dazu Notizen,
Einstellungen, Englisch/Deutsch und Releases für macOS, Linux und
Windows. Was noch fehlt, steht in `05_ROADMAP.md` (Phase 8 und Pflege).

------------------------------------------------------------------------

# Fünf Kernbereiche

``` text
Calendar
Tasks
Habits
Projects
VSCodium Workspace
```

Dazu kommt Time Tracking als Querschnittsthema über Tasks und Projects,
und Notizen als Dateien neben der Datenbank.

------------------------------------------------------------------------

# Produktfluss

``` text
WEBUI
   │
   ├── Kalender
   ├── Tagesplanung
   ├── Tasks
   ├── Habits
   └── Projects
           │
           ▼
      AUFGABE STARTEN
           │
           ▼
       VSCODIUM
           │
           ├── Workspace öffnen
           ├── Dateien anzeigen
           ├── Ressourcen öffnen
           ├── Timer
           └── Arbeitsstatus
           │
           ▼
       GO BACKEND
           │
           ▼
         SQLITE
           │
           ▼
        WEBUI
      aktualisiert
```

------------------------------------------------------------------------

# Backend-Vorgabe

Das Backend MUSS in Go implementiert werden.

Nicht Node.js.

Struktur:

``` text
backend/
├── cmd/server/
├── internal/domain/
├── internal/service/
├── internal/repository/
├── internal/api/
├── internal/realtime/
├── internal/notes/
└── migrations/
```

Die übrigen Pakete stehen in `02_ARCHITECTURE.md` (Go-Aufbau).

------------------------------------------------------------------------

# MVP-Reihenfolge

Die Schritte geben die grobe Reihenfolge vor. Innerhalb eines Schritts
wird jede Funktion als vertikale Scheibe gebaut (siehe
`03_AGENT_GUIDELINES.md`, Entwicklungsstil). Details stehen in
`05_ROADMAP.md`.

## 1

Go Backend

-   SQLite
-   Projects
-   Tasks
-   Calendar Events
-   Habits
-   Habit Completions
-   Resources
-   TimeEntries

## 2

WebUI

-   Today
-   Calendar
-   Tasks
-   Habits
-   Projects

## 3

VSCodium

-   Activity Bar
-   Project erkennen
-   Tasks anzeigen
-   Task starten
-   Timer
-   Ressourcen öffnen

## 4

Synchronisation

-   WebSocket
-   Status
-   Zeit

Damit ist der MVP abgeschlossen.

------------------------------------------------------------------------

# Nach dem MVP

Automatisierung (erst wenn der Kern stabil ist)

-   Aktivitätserkennung
-   intelligente Vorschläge
-   bessere Tagesplanung

------------------------------------------------------------------------

# Entscheidungsgrundsatz

Bei jeder technischen Entscheidung:

> Unterstützt das die Verbindung zwischen Planung und tatsächlicher
> Arbeit?

Wenn nein:

> Nicht priorisieren.

------------------------------------------------------------------------

# Zielbild

Am Ende soll der Nutzer sagen können:

> "Ich muss heute AlgoDat, Software Testing und den Activitytracker machen."

(Langfristiges Zielbild: Die Freitext-Eingabe gehört nicht zum MVP. Im
MVP plant der Nutzer diese Dinge selbst in der WebUI.)

Das System übersetzt das in:

``` text
Kalender
+
Tasks
+
Habits
+
Projects
```

und wenn der Nutzer tatsächlich arbeitet:

``` text
Aufgabe auswählen
↓
VSCodium
↓
richtiger Workspace
↓
richtige Dateien
↓
arbeiten
↓
Zeit erfassen
↓
Aufgabe fertig
↓
Plan aktualisiert
```

Das ist der Kern des Produkts.
