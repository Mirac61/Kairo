# Development Roadmap

Der MVP umfasst die Phasen 0–5. Ab Phase 6 kommen Erweiterungen nach dem
MVP.

Innerhalb jeder Phase wird jede Funktion als vertikale Scheibe gebaut
(siehe `03_AGENT_GUIDELINES.md`, Entwicklungsstil).

------------------------------------------------------------------------

# Phase 0 — Foundation

Ziel: Repository und lokale Entwicklungsumgebung.

### Backend

-   Go module
-   SQLite
-   migrations
-   configuration
-   HTTP server
-   health endpoint
-   Host-/Origin-Prüfung und lokales Token
-   WebUI per embed ausliefern (Dev: Vite-Proxy)

### Frontend

-   Vue 3
-   TypeScript
-   Vite
-   grundlegendes Layout

### Extension

-   VSCodium extension scaffold
-   Activity Bar
-   hello/context view

------------------------------------------------------------------------

# Phase 1 — Core Data

Implementieren:

-   Projects
-   Tasks
-   Calendar Events
-   Habits
-   Habit Completions
-   Resources
-   TimeEntries

Noch keine komplexe Automatisierung.

------------------------------------------------------------------------

# Phase 2 — Today

Today Endpoint:

``` text
GET /api/today?date=YYYY-MM-DD
```

Response kombiniert:

-   Events
-   geplante Tasks
-   Habit Occurrences
-   Zeitinformationen

Die WebUI bekommt damit einen zentralen Tageskontext.

------------------------------------------------------------------------

# Phase 3 — Planning

Implementieren:

-   Tagesplanung
-   Wochenansicht
-   Drag & Drop für Tasks
-   Zeitbedarf
-   Überplanung sichtbar machen

------------------------------------------------------------------------

# Phase 4 — VSCodium

Implementieren:

-   Project Detection
-   Workspace Mapping
-   Current Task
-   Today Tasks
-   Resources
-   Start Task
-   Pause Task
-   Complete Task

------------------------------------------------------------------------

# Phase 5 — Real-time

WebSocket Events.

Beispiel:

``` text
WebUI:
Task gestartet

        ↓

Go Backend

        ↓

WebSocket

        ↓

VSCodium:
Task = IN_PROGRESS
Timer = running
```

Außerdem:

-   Autostart per LaunchAgent (kairo install)
-   „Backend offline“-Anzeige in der Extension

Mit Phase 5 ist der MVP abgeschlossen.

------------------------------------------------------------------------

# Phase 6 — Activity Detection

Nur wenn der Kern stabil ist.

Mögliche Signale:

-   aktiver Editor
-   Dokumentänderungen
-   Workspace
-   VSCodium-Fokus
-   manuelle Interaktion

Ziel:

Arbeitszeit möglichst realistisch erfassen.

Nicht:

> jede Sekunde in VSCodium als Arbeit zählen.

------------------------------------------------------------------------

# Phase 7 — Review

Dashboard für:

-   vergangene Arbeitszeit
-   erledigte Tasks
-   Habit-Streaks
-   Kalenderauslastung
-   Projektfortschritt

Der Review-Bereich soll Erkenntnisse liefern und kein weiteres
Managementsystem werden.

------------------------------------------------------------------------

# Phase 8 — Future

Mögliche spätere Funktionen:

-   Google Calendar Adapter
-   Apple Calendar Adapter
-   Import/Export
-   Backup
-   KI-gestützte Tagesplanung
-   intelligente Projekterkennung
-   automatische Kontextvorschläge

Diese Funktionen sind bewusst nicht Teil des MVP.
