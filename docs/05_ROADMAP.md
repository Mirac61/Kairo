# Development Roadmap

Der MVP umfasst die Phasen 0–5. Ab Phase 6 kommen Erweiterungen nach dem
MVP.

**Stand:** Die Phasen 0–7 sind gebaut. Dazu kamen nach dem MVP: der
Rückblick in der WebUI, ICS-Import, Papierkorb mit Rückgängig-Toast,
Projektfarben, Statusleiste, Befehle und URI-Handler in der Extension, das
SwiftBar-Plugin (`tools/menubar/`), Backups, die Einstellungsseite
(`config.json`), Notizen mit PDFs, Bildern, Vorlagen, Code-Viewer und
Zeichenwerkzeug, Zeichnungen (Excalidraw) in Notizen, Ziele, Journal,
Lernkarten, Gewohnheiten mit Dauer, GitHub-Issues in der Aufgaben-Ansicht,
Englisch/Deutsch,
Playwright-Smoke-Tests und Releases mit fertigen Binaries für macOS,
Linux und Windows. In Phase 8 steht nur noch, was fehlt, unter „Pflege“
der Stand der Aufräumarbeiten.

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

Danach gebaut:

-   Gewohnheiten: Dialog zum Anlegen und Bearbeiten mit allen Feldern
    (Rhythmus, Uhrzeit, Dauer, Ziel, Zeitraum), Anzeige in Kalender-Woche
    und -Tag mit Abhaken per Klick (siehe `01_PRODUCT_REQUIREMENTS.md`,
    3. Habits).
-   GitHub-Issues: offene Issues (gesamt und je Projekt über `git remote`)
    als Task anlegen, über die gh-CLI (siehe 2. Tasks).

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
    mit leerer Datenbank. Die Browser-Tests decken Aufgaben und Timer,
    Kalender, Gewohnheiten, GitHub-Issue-Import, Ordner-Upload,
    Code-Viewer, Notizen, PDF- und Bild-Markierungen, Ziele, Journal,
    Lernkarten, Einstellungen, Handy-Layout und Rundgang ab. Die
    GitHub-Issue-Liste wird für den UI-Test gemockt; der Import läuft
    gegen das echte Backend. `make e2e` läuft lokal und in der CI.
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

-   **Zeichnungen und Lizenzen:** `.excalidraw`-Dateien lassen sich im
    Notizordner anlegen, mit Autosave bearbeiten und per
    `![](skizze.excalidraw)` in Notizen einbetten. Excalidraw und die
    Schriften laufen lokal (kein CDN). `THIRD_PARTY_NOTICES.md` liegt im
    Release-Archiv und ist in den Einstellungen verlinkt.
-   **Pläne:** Arbeitspläne liegen in `docs/plans/` und sind per
    `.git/info/exclude` lokal ausgeschlossen, nicht im Repo.

## Offen

-   **Windows-Abnahme:** Das Backend wird für Windows gebaut und in der CI
    getestet. Ein manueller Start und ein kurzer Durchlauf der wichtigsten
    Abläufe unter Windows stehen noch aus.
-   **Bundle-Größe (gemessen 2026-11-24):** `dist` ist rund 16 MB groß
    (360 Dateien), der Build warnt vor Chunks über 500 kB. Diese
    (Mermaid, Cytoscape, pdf.js, KaTeX, Excalidraw) laden aber nur bei
    Bedarf, alle Ansichten sind lazy. Der Einstieg braucht 128 kB JS und
    14 kB CSS (gzip). Kaltstart lokal mit leerem Cache: Übertragung
    220–320 kB je Ansicht, Notizen 600 kB; First Contentful Paint unter
    100 ms. Kein Handlungsbedarf; erneut messen, wenn der Einstieg über
    250 kB gzip wächst oder der Start über VPN spürbar langsam wird.
-   **Dateiversionen (entschieden 2026-11-24):** Alte Fassungen ersetzter
    PDFs, Bilder und Zeichnungen liegen in `.trash/Fassungen/` und werden
    nach 30 Tagen gelöscht. Gelöschte Notizen in `.trash/` bleiben, weil
    die eingebauten Backups nur die Datenbank sichern (`pi-backup.sh`
    kopiert die Notizen zusätzlich auf den Mac). Fassungen von vor dieser
    Änderung liegen noch direkt in `.trash/` und lassen sich von Hand
    löschen.
-   **GitHub-E2E:** Der Smoke-Test prüft jetzt den Ablauf von der Issue-
    Anzeige bis zur Task samt Projekt und URL-Ressource. Die Issue-Liste
    wird dabei gemockt; den Abruf über `gh` decken Backend-Tests ab. Ein
    zusätzlicher Browser-Test gegen eine echte GitHub-Anmeldung wäre
    unnötig netzwerkabhängig.
-   **Formatprüfung:** ESLint läuft über `make check`. Prettier ist
    eingerichtet, `format:check` ist aber noch kein Teil dieses Gates
    und meldet derzeit Formatabweichungen in `CalendarToolbar.vue`,
    `SettingsSystemTab.vue` und `TaskQuickAdd.vue`.
-   **Komponenten:** `CalendarView.vue` hat aktuell rund 355 Zeilen,
    `NotesView.vue` rund 132. `FileEditor.vue` ist mit rund 935 Zeilen
    weiterhin groß. Weitere Aufteilung bietet sich an, wenn dort ohnehin
    eine Änderung ansteht.
