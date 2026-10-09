# Development Roadmap

Der MVP umfasst die Phasen 0–5. Ab Phase 6 kommen Erweiterungen nach dem
MVP.

**Stand:** Die Phasen 0–7 sind gebaut. Dazu kamen nach dem MVP: der
Wochenrückblick in der WebUI, ICS-Import, Papierkorb mit Rückgängig-Toast,
Projektfarben, Statusleiste, Befehle und URI-Handler in der Extension, das
SwiftBar-Plugin (`tools/menubar/`), Backups, die Einstellungsseite
(`config.json`), Notizen mit PDFs, Bildern, Vorlagen und
Zeichenwerkzeug, Englisch/Deutsch, Playwright-Smoke-Tests und Releases
mit fertigen Binaries für macOS, Linux und Windows. In Phase 8 steht nur noch, was fehlt, unter „Pflege“ der Stand
der Aufräumarbeiten.

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
-   Export (der ICS-Import ist gebaut)
-   KI-gestützte Tagesplanung
-   intelligente Projekterkennung
-   automatische Kontextvorschläge

Diese Funktionen sind bewusst nicht Teil des MVP.

------------------------------------------------------------------------

# Pflege

Technische Schulden und Aufräumarbeiten, keine neuen Funktionen.

## Erledigt

-   **Frontend-Tests:** `frontend/e2e/smoke.test.mjs` startet das Backend
    mit leerer Datenbank und prüft im Browser: Aufgabe anlegen, Timer
    starten und stoppen, Termin verschieben, Notiz speichern, PDF und
    Bild bezeichnen. `make e2e` lokal, eigener Job in der CI.
-   **Große Komponenten:** Aus `FileEditor.vue` sind Werkzeugleiste
    (`FileEditorBar.vue`), PDF-Rendern (`usePdfRender`), Verlauf
    (`useHistory`) und Speichern (`lib/fileSave.ts`) herausgelöst, aus
    `CalendarView.vue` der Termin-Dialog (`EventDialog.vue`) und die
    Speicherlogik (`requestOf` in `lib/eventForm.ts`), aus
    `NotesView.vue` der Markdown-Editor (`NoteEditor.vue`) und die
    Breite der geteilten Ansicht (`useSplitRatio`). Rechenteile der
    Startseite liegen in `lib/dayPlan.ts`, Pfadhelfer und Bildnamen der
    Notizen in `lib/noteFiles.ts`; alles mit Tests in `frontend/test/`.
-   **`quickAdd.ts` nur einmal:** Die Extension kompiliert
    `frontend/src/lib/quickAdd.ts` mit (`rootDir: ".."` in
    `extension/tsconfig.json`); Kopie und `cmp`-Prüfung sind weg.
-   **Einstieg ohne Build:** `.goreleaser.yaml` baut bei einem Tag `v*`
    Binaries für macOS, Linux und Windows (arm64/amd64) und hängt die Extension
    als `.vsix` an das Release (`.github/workflows/release.yml`).
-   **README:** auf Englisch, für neue Nutzer gekürzt.
-   **Windows und Linux:** Releases für alle drei Systeme, Autostart per
    systemd bzw. Autostart-Ordner, Neustart ohne `exec` unter Windows,
    Windows-Pfade in Ressourcen und Projekterkennung der Extension.
    Backend-Tests laufen in der CI auf Linux, macOS und Windows.
-   **Englisch/Deutsch:** Sprachwahl in den Einstellungen (sofort, je
    Browser). Die Schnelleingabe versteht zusätzlich `@today`,
    `@tomorrow`, englische Wochentage und `!high` usw.
-   **Veröffentlichen:** Die Reviews `06`/`07` sind aus dem Repo
    genommen. `AGENTS.md`, `03_AGENT_GUIDELINES.md` und
    `04_AGENT_HANDOFF.md` bleiben öffentlich, weil sie die Arbeit mit
    Coding-Agents am Projekt erleichtern. Die Screenshots in
    `docs/screenshots/` zeigen nur Mock-Daten aus
    `frontend/scripts/shot.mjs`.

## Offen

-   Die Extension und die Fehlermeldungen des Backends sind nur
    deutsch. Die Extension könnte `vscode.env.language` folgen.
-   Unter Windows ist das Backend nur cross-kompiliert und in der CI
    getestet, noch nicht von Hand ausprobiert.

-   `CalendarView.vue` und `FileEditor.vue` haben noch knapp 600 Zeilen,
    `NotesView.vue` gut 500. Der Rest hängt eng an FullCalendar, am
    Zeiger-Handling der Zeichenfläche bzw. am Dateibaum; weiter teilen,
    wenn eine Änderung dort ohnehin ansteht.
