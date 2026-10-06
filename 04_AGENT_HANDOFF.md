# Agent Handoff

## Lies diese Dateien zuerst

1.  `00_VISION.md`
2.  `01_PRODUCT_REQUIREMENTS.md`
3.  `02_ARCHITECTURE.md`
4.  `03_AGENT_GUIDELINES.md`

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
```

------------------------------------------------------------------------

# Fünf Kernbereiche

``` text
Calendar
Tasks
Habits
Projects
VSCodium Workspace
```

Dazu kommt Time Tracking als Querschnittsthema über Tasks und Projects.

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

Empfohlene Struktur:

``` text
backend/
├── cmd/server/
├── internal/domain/
├── internal/service/
├── internal/repository/
├── internal/api/
├── internal/websocket/
└── migrations/
```

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
