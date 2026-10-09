# Product Requirements

# 1. Calendar

Der Kalender unterstützt:

-   Tagesansicht
-   Wochenansicht
-   Events
-   wiederkehrende Events
-   Startzeit
-   Endzeit
-   Titel
-   Beschreibung
-   Ort
-   optionale URL
-   optionale Verknüpfung mit Project oder Task

### Calendar Event

``` text
id
title
description
start_at
end_at
location
url
project_id?
task_id?
recurrence_rule?
recurrence_exdates?
```

Ein Event muss nicht mit einem Task verbunden sein.

Kalender-Events sind feste Termine. Geplante Arbeitszeit für Tasks wird
über `planned_start_at` am Task abgebildet, nicht über Events.

`recurrence_rule` verwendet das RRULE-Format aus RFC 5545. Im MVP werden
unterstützt: `FREQ=DAILY|WEEKLY`, `BYDAY`, `INTERVAL`, `UNTIL`, `COUNT`.
Einzelne ausgelassene Termine stehen in `recurrence_exdates` (Liste von
Daten, als JSON gespeichert).

Die WebUI legt Termine mit Ort an und bearbeitet sie. Als Wiederholung
bietet sie „wöchentlich an ausgewählten Wochentagen“, optional mit
Enddatum (`FREQ=WEEKLY;BYDAY=…;UNTIL=…`). Andere Regeln (per API
angelegt) bleiben beim Bearbeiten in der WebUI unverändert. Änderungen
gelten für die ganze Serie. Beim Löschen einer Serie bietet die WebUI
„Nur dieser Termin“ (trägt den Tag in `recurrence_exdates` ein) oder
„Ganze Serie“ an; beides lässt sich per Rückgängig-Toast zurücknehmen.

**ICS-Import:** `POST /api/calendar/import` (Body: Text einer `.ics`-Datei,
in der WebUI „ICS importieren“). Die UID eines Termins (`external_uid`)
macht den Import wiederholbar: ein bekannter Termin wird aktualisiert, nicht
doppelt angelegt; Projekt, Task und URL bleiben dabei stehen. Unterstützt
sind `DAILY`- und `WEEKLY`-Regeln samt `EXDATE`; andere Regeln (z. B.
`MONTHLY`) kommen als Einzeltermin zum ersten Datum, `RECURRENCE-ID`-
Ausnahmen werden übersprungen, in der ICS fehlende Termine nicht gelöscht.
Ein Termin im Papierkorb wird nicht wiederbelebt (`skipped`, Notiz „im
Papierkorb“). Die Antwort nennt `created`, `updated`, `skipped`,
`unsupported_rules` und `notes`.

------------------------------------------------------------------------

# 2. Tasks

## Eigenschaften

``` text
id
title
description
status
priority
estimated_minutes
due_at?
planned_date?
planned_start_at?
project_id?
parent_task_id?
created_at
updated_at
completed_at?
```

`planned_date` ist der lokale Tag, an dem der Task geplant ist.
`planned_start_at` ist optional die geplante Startzeit an diesem Tag.
Zusammen mit `estimated_minutes` ergibt das den Zeitblock in der Today
View. Tasks ohne `planned_start_at` erscheinen in der Today View als
Liste ohne Uhrzeit.

### Status

``` text
BACKLOG
PLANNED
IN_PROGRESS
PAUSED
COMPLETED
CANCELLED
```

### Priorität

``` text
LOW
MEDIUM
HIGH
URGENT
```

Tasks können Ressourcen besitzen.

In der WebUI sind Titel, Beschreibung, Priorität, Projekt, Fälligkeit,
geplanter Tag, Uhrzeit und Schätzung bearbeitbar.

**Papierkorb:** Löschen einer Task (samt Teilaufgaben), eines Termins oder
eines Habits legt es in den Papierkorb (`DELETE` → 204, `POST …/restore`,
`GET /api/trash`); die WebUI löscht ohne Rückfrage und zeigt dafür ein
Rückgängig-Toast, die Ansicht „Papierkorb“ stellt wieder her oder löscht
endgültig (`?permanent=true`, mit Rückfrage). Läuft ein Timer auf der Task,
lässt sie sich nicht löschen (409, erst pausieren). Eine Task, an der (oder
an deren Subtasks) Zeiteinträge hängen, lässt sich nicht endgültig löschen
(409), weil die Einträge sonst mitgelöscht würden; stattdessen wird sie
abgebrochen (`CANCELLED`). Die erfasste Zeit einer Task im Papierkorb zählt
in den Summen weiter. Projekte werden weiterhin hart gelöscht.

------------------------------------------------------------------------

# 3. Habits

Habits sind wiederkehrende Verhaltensregeln.

Beispiel:

``` text
Habit:
"30 Minuten lesen"

Frequenz:
daily

Ziel:
30 Minuten
```

Oder:

``` text
Habit:
"Sport"

Frequenz:
3x pro Woche
```

Ein Habit braucht:

``` text
id
name
description
frequency_type
frequency_config
target_value?
unit?
preferred_time?
start_date
end_date?
active
created_at
```

`frequency_type` ist z. B. `DAILY`, `WEEKLY`, `SPECIFIC_WEEKDAYS` oder
`TIMES_PER_WEEK`. `frequency_config` enthält die Details (z. B.
Wochentage oder Anzahl) als JSON (siehe `02_ARCHITECTURE.md`).

`preferred_time` ist eine optionale lokale Uhrzeit (`HH:MM`).

Aus der Regel ergeben sich Habit Occurrences (berechnet, siehe Regeln).
Wenn der Nutzer ein Habit abhakt, wird eine HabitCompletion gespeichert.

``` text
HabitCompletion
├── id
├── habit_id
├── date
├── value?
├── completed_at
└── note?
```

Der Nutzer soll Habits abhaken können, ohne jedes Mal eine komplett neue
Task manuell anzulegen.

## Regeln

-   Habit Occurrences werden aus der Regel berechnet und nicht
    gespeichert. Gespeichert werden nur HabitCompletions.
-   Pro Habit gibt es höchstens eine Completion pro Tag.
-   `TIMES_PER_WEEK` erscheint täglich mit Fortschritt (z. B. „1/3 diese
    Woche“), bis das Wochenziel erreicht ist. Die Woche läuft von Montag
    bis Sonntag.
-   Habits mit `preferred_time` erscheinen in der Zeitleiste der Today
    View, Habits ohne Uhrzeit in einer Liste „Heute noch offen“.

------------------------------------------------------------------------

# 4. Projects

Ein Project besitzt:

``` text
id
name
description
local_path?
status
created_at
updated_at
```

Status:

``` text
ACTIVE
PAUSED
COMPLETED
ARCHIVED
```

Ein Projekt kann besitzen:

-   Tasks
-   Ressourcen
-   Zeitentries
-   VSCodium Workspace (über `local_path`)
-   URLs
-   Dokumentreferenzen

`local_path` muss ein absoluter Pfad (oder `~/…`) sein und existieren; das
Backend prüft das beim Anlegen und beim Ändern des Pfads. Jedes Projekt hat
eine Farbe aus einer festen Palette (`color`); die WebUI zeigt sie überall
und bietet sie im Projektdialog zur Auswahl an, ein neues Projekt bekommt die
am wenigsten benutzte.

Ein Projekt mit direkten Zeiteinträgen lässt sich nicht löschen (409);
stattdessen wird es archiviert. Beim Löschen eines Projekts bleiben
seine Tasks erhalten (ohne Projekt).

------------------------------------------------------------------------

# 5. Resources

Ressourcen können an Projects oder Tasks hängen.

Typen:

``` text
FILE
FOLDER
URL
```

Beispiel:

``` text
Task:
AlgoDat Musterprüfung A

Resources:
FILE   ~/Documents/Uni/AlgoDat/Pruefungen/AuD_Musterprf_A.pdf
FILE   ~/Documents/Uni/AlgoDat/Pruefungen/AuD_Musterprf_A_l.pdf
FOLDER ~/Documents/Uni/AlgoDat/CodeAlgorithmen
URL    https://moodle...
```

Die Anwendung soll Dateien nicht unnötig kopieren.

Sie speichert Referenzen.

## Regeln

-   Eine Ressource gehört genau einer Task oder genau einem Projekt.
    Wird der Besitzer gelöscht, wird die Ressource mit gelöscht.
-   `FILE` und `FOLDER` brauchen einen absoluten Pfad (`/…`) oder einen
    Pfad mit `~/…`. Der Pfad wird nicht auf Existenz geprüft, weil die
    Ressource auch auf ein nicht eingehängtes Laufwerk zeigen darf.
-   `URL` muss eine `http`- oder `https`-URL sein.
-   `label` ist optionaler Anzeigetext. Ressourcen werden nicht geändert,
    sondern gelöscht und neu angelegt (`GET`, `POST`, `DELETE`).
-   Notizen sind keine Ressourcen; sie liegen als Dateien im Notizordner
    (siehe 11. Notizen).

------------------------------------------------------------------------

# 6. Today View

Die wichtigste Ansicht der WebUI.

Sie kombiniert:

-   Calendar Events
-   geplante Tasks
-   überfällige Tasks: offen und mit `planned_date` vor dem Tag
    (`overdue` in `GET /api/today`); sie stehen oben und lassen sich mit
    „→ Heute“ oder „→ Morgen“ verschieben (die alte Uhrzeit entfällt)
-   Habit Occurrences
-   aktuelle Tasks
-   Zeitplanung
-   die Zeiteinträge des Tages; Start und Ende sind korrigierbar,
    beendete Einträge lassen sich löschen

Beispiel:

``` text
TODAY

08:30  📅 AlgoDat Vorlesung

10:15  🔴 AlgoDat Musterprüfung A
       90 min

12:30  📅 Mittagessen

14:00  💻 Activitytracker Login
       60 min

18:00  🔁 Sport

22:30  🔁 Lesen
```

------------------------------------------------------------------------

# 7. Tagesplanung

Der Nutzer kann Tasks in den Tag ziehen: in der Today-Ansicht aus der
Aufgabenliste in den Tagesplan (überlappende Blöcke stehen nebeneinander),
im Kalender aus der Leiste „Ungeplant“. Beides lässt sich per
Rückgängig-Toast zurücknehmen.

Die Anwendung zeigt:

``` text
Geplante Zeit: 4h 30m
Kalender:       2h
Freie Zeit:     3h
```

Das System soll Überplanung sichtbar machen.

Es soll nicht automatisch den gesamten Tag umplanen, ohne dass der
Nutzer dies möchte.

------------------------------------------------------------------------

# 8. Zeittracking

TimeEntries:

``` text
id
task_id?
project_id?
started_at
ended_at?
source
```

Source:

``` text
MANUAL
VSCODIUM
AUTOMATIC
```

Zeit wird aus echten Zeitstempeln berechnet.

Nicht über einen einfachen Counter.

Es läuft immer höchstens ein Timer, d. h. es gibt höchstens einen
TimeEntry ohne `ended_at`. Wird ein anderer Task gestartet, wird der
laufende automatisch pausiert: sein TimeEntry wird geschlossen, sein
Status wird `PAUSED`, danach startet der neue Task.

## Regeln

-   **Start:** setzt die Task auf `IN_PROGRESS` und öffnet einen
    TimeEntry. Läuft die Task schon, ändert sich nichts. Tasks mit Status
    `COMPLETED` oder `CANCELLED` lassen sich nicht starten (409);
    wieder öffnen geht über `PATCH` auf den Status.
-   **Pause:** schließt den TimeEntry und setzt die Task auf `PAUSED`.
    Läuft für die Task kein Timer, ist das ein Konflikt (409). Optional
    nimmt Pause `ended_at` (nicht in der Zukunft; vor dem Start gilt der
    Start). Die Extension nutzt das bei der Rückfrage nach Inaktivität:
    Der Eintrag endet zum Zeitpunkt der letzten Aktivität, nicht jetzt.
-   **Complete:** schließt einen laufenden TimeEntry der Task und setzt
    sie auf `COMPLETED`. Eine schon abgeschlossene Task bleibt
    unverändert, eine abgebrochene ist ein Konflikt (409). Läuft der
    Timer einer anderen Task, bleibt er unberührt.
-   Verlässt eine Task per `PATCH` den Status `IN_PROGRESS`, wird ihr
    laufender Timer beendet. So läuft nie ein Timer auf einer
    abgeschlossenen oder pausierten Task weiter.
-   Ein Timer startet nur durch ausdrückliches Starten, nie durch das
    Öffnen eines Workspace.
-   `POST /api/time-entries` erfasst nur nachträglich einen beendeten
    Eintrag: `started_at` und `ended_at` sind Pflicht, `ended_at` liegt
    nach `started_at` und nicht in der Zukunft. Angegeben wird genau eines
    von `task_id` und `project_id`. `source` ist standardmäßig `MANUAL`.
-   `PATCH /api/time-entries/{id}` korrigiert `started_at` und
    `ended_at` (Ende nach Start, nichts in der Zukunft). Einen laufenden
    Timer beenden nur Pause und Complete (409).
-   `DELETE /api/time-entries/{id}` löscht einen beendeten Eintrag; ein
    laufender Timer ist ein Konflikt (409).

------------------------------------------------------------------------

# 9. VSCodium Integration

Die Extension stellt eine Activity-Bar-Ansicht bereit.

Sie zeigt:

``` text
CURRENT TASK

AlgoDat — Musterprüfung A

90 min geplant
32 min gearbeitet

[Pause]
[Erledigt]

Resources

📄 AuD_Musterprf_A.pdf
📄 AuD_Musterprf_A_l.pdf
📁 CodeAlgorithmen
🌐 Moodle

NEXT

Activitytracker — Login reparieren
```

Stand der Extension: Die Ansicht hat die Tabs „Heute“ und „Projekte“. „Heute“
zeigt oben den Timer (Task und Projekt vor der Zeit; läuft er für ein anderes
Projekt als den Workspace, mit Hinweis und Sprung dorthin), darunter die
Agenda (Tasks des Workspace-Projekts hervorgehoben), die Ressourcen und
weiteren offenen Tasks des Projekts und eine Schnelleingabe („30 min Sport“),
die das Zielprojekt nennt. Dazu kommen eine Statusleiste mit Timer, Befehle
(Task starten, Pause, Abschließen, Neue Task, Projekt und Ressource öffnen,
Ordner mit Projekt verknüpfen) und ein URI-Handler, über den die WebUI
„In VSCodium starten“ auslöst (`/start?task=…`, `/open?project=…`).
Statistik und Rückblick stehen nur in der WebUI.

------------------------------------------------------------------------

# 10. Workspace-Erkennung

Wenn der aktuelle Workspace einem Project entspricht:

``` text
~/code/uni/activitytracker
        ↓
Project: Activitytracker

~/Documents/Uni/AlgoDat
        ↓
Project: AlgoDat
```

Ein Workspace gilt auch dann als erkannt, wenn ein Unterordner geöffnet
wird (z. B. `~/Documents/Uni/AlgoDat/CodeAlgorithmen` → AlgoDat).

Die Erkennung nutzt `local_path`; ein führendes `~` steht für das
Home-Verzeichnis. Passen mehrere Projekte, gewinnt der längste passende
Pfad.

Ist ein Projekt erkannt, zeigt die Extension ganz oben seine Ressourcen
(zum Öffnen) und seine offenen Tasks (zum Starten und Abhaken).

Die Extension soll nicht automatisch jede Aktivität als Arbeitszeit
zählen.

------------------------------------------------------------------------

# 11. Notizen

Notizen sind normale Dateien in einem Ordner (Standard `~/Kairo`, änderbar
in den Einstellungen). Kairo speichert nichts davon in SQLite, damit
dieselben Dateien parallel in VSCodium bearbeitet werden können.

-   Der Dateibaum zeigt Ordner, Notizen (`.md`), PDFs und Bilder (png,
    jpg, gif, webp). Versteckte Einträge (`.git`, `.obsidian`, `.trash`)
    und SVG (kann Skript enthalten) fehlen.
-   Anlegen, Umbenennen, Verschieben per Ziehen, Löschen. Gelöschtes
    landet in `.trash/` im Notizordner. Eine neue Notiz beginnt mit dem
    heutigen Datum im Namen.
-   PDFs, Bilder und Markdown lassen sich aus dem Finder in den Baum
    ziehen. Eingefügte Bilder landen in `assets/` neben der Notiz und
    werden relativ verlinkt, damit VSCodium sie genauso zeigt.
-   Vorlagen sind Notizen im Ordner `Vorlagen`; eine leere Notiz bietet
    sie an. Platzhalter: `{{titel}}`, `{{ordner}}`, `{{datum}}`,
    `{{wochentag}}`, `{{uhrzeit}}`.
-   PDFs und Bilder lassen sich beschriften (Stift, Textmarker, Linie,
    Pfeil, Rechteck, Ellipse, Text); gespeichert wird in die Datei.
-   Geteilte Ansicht: PDF oder Bild links, Notiz rechts, Breite
    verschiebbar. Beide bleiben geladen, ein Wechsel behält Scrollposition
    und Ungespeichertes.
-   Ändert sich eine Datei außerhalb von Kairo, lädt die WebUI sie still
    neu, wenn nichts ungespeichert ist; sonst fragt sie beim Speichern
    (Überschreiben oder Verwerfen).

------------------------------------------------------------------------

# 12. Einstellungen und Sprache

-   Die Seite Einstellungen ändert Notizordner, Arbeitszeitfenster und
    Zeitzone. Die Werte gelten nach einem Neustart, den die Seite per
    Knopf auslöst. Der Notizordner muss existieren und beschreibbar sein.
-   Port, Datenbankpfad und Token bleiben nur per Umgebung einstellbar.
-   Die WebUI gibt es auf Deutsch und Englisch; die Wahl gilt sofort und
    je Browser. Die Schnelleingabe versteht beide (`@morgen`/`@tomorrow`,
    `!hoch`/`!high`).

------------------------------------------------------------------------

# 13. MVP

Der MVP benötigt:

### Backend

-   Go
-   SQLite
-   REST API
-   WebSocket
-   Projects
-   Tasks
-   Habits
-   Habit Completions
-   Calendar Events
-   Resources
-   TimeEntries
-   Backups: `kairo backup` jederzeit; beim Serverstart automatisch vor
    den Migrationen, wenn eine aussteht oder das letzte Backup älter als
    24 h ist (in `backups/` neben der Datenbank, die neuesten 14 bleiben)

### WebUI

-   Today
-   Calendar (mit ICS-Import)
-   Tasks (Suche, Gruppierung, Tastenkürzel)
-   Habits
-   Projects
-   Wochenrückblick (`/review`)
-   Papierkorb (`/trash`) und Rückgängig-Toast

### Extension

-   Activity Bar
-   aktuelles Project
-   Tasks
-   Task starten/pausieren/beenden
-   Timer
-   Workspace-Erkennung
-   Ressourcen öffnen
-   Statusleiste, Befehle, URI-Handler

Der MVP entspricht den Phasen 0–5 in `05_ROADMAP.md`.

------------------------------------------------------------------------

# Nicht-MVP

Zunächst nicht bauen:

-   Cloud
-   Accounts
-   Teamfunktionen
-   Mobile App
-   komplexes Notizsystem (Backlinks, Volltextsuche, Sync); gebaut ist
    nur der Dateibaum aus 11. Notizen
-   eigener Kalender-Sync zu jedem Anbieter
-   KI-Agent
-   Social Features
-   Habit-Gamification
-   Review-Dashboard über den Wochenrückblick hinaus (der Rückblick selbst ist gebaut, Phase 7)
-   automatische Aktivitätserkennung (nur als Rückfrage bei Inaktivität gebaut, Phase 6)
