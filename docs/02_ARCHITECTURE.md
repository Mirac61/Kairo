# Technical Architecture

## Architektur

``` text
                       ┌──────────────────┐
                       │      SQLite      │
                       │   local source   │
                       │      of truth    │
                       └────────┬─────────┘
                                │
                         ┌──────▼──────┐
                         │ Go Backend  │
                         │             │
                         │ REST API    │
                         │ WebSocket   │
                         │ Business    │
                         │ Logic       │
                         └───┬─────┬───┘
                             │     │
             HTTP + WebSocket│     │HTTP + WebSocket
                             │     │
                  ┌──────────▼─┐ ┌─▼──────────────┐
                  │   WebUI    │ │ VSCodium       │
                  │ Vue/TS     │ │ Extension      │
                  └────────────┘ └────────────────┘
```

Beide Clients nutzen HTTP (REST) für Lesen und Ändern und WebSocket, um
Änderungen in Echtzeit zu empfangen.

------------------------------------------------------------------------

# Backend: Go

Das Backend soll in **Go** implementiert werden.

Warum Go:

-   kompiliertes einzelnes Binary
-   sehr geringer Ressourcenverbrauch
-   schnell
-   gute Nebenläufigkeit
-   ideal für einen lokalen Dienst
-   einfache Verteilung
-   keine Node-Runtime notwendig
-   WebSocket und HTTP gut geeignet

Das Backend soll als lokaler Prozess laufen.

Beispiel:

``` text
~/code/personal/Kairo
├── README.md
├── AGENTS.md
├── docs/          00_VISION.md … 05_ROADMAP.md
├── backend/       Go
├── frontend/      Vue 3 + TypeScript + Vite
└── extension/     VSCodium-Extension
```

Die Produktdaten (SQLite-Datei) liegen nicht im Repository, sondern im
Benutzerverzeichnis, z. B. `~/.local/share/kairo/kairo.db`. Der Pfad ist
konfigurierbar.

------------------------------------------------------------------------

# Empfohlener Go-Aufbau

``` text
backend/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── api/
│   ├── domain/
│   ├── service/
│   ├── repository/
│   ├── websocket/
│   └── config/
│
├── migrations/
└── go.mod
```

------------------------------------------------------------------------

# Layer

## domain

Reine Geschäftsobjekte.

Beispielsweise:

``` go
type Task struct {
    ID               string
    Title            string
    Description      string
    Status           TaskStatus
    Priority         TaskPriority
    EstimatedMinutes int
    PlannedDate      *string     // lokaler Tag, YYYY-MM-DD
    PlannedStartAt   *time.Time  // UTC
    ProjectID        *string
}
```

Keine HTTP-Logik im Domain-Modell.

------------------------------------------------------------------------

## repository

Datenbankzugriff.

Beispielsweise:

``` text
TaskRepository
ProjectRepository
HabitRepository
CalendarRepository
TimeEntryRepository
```

------------------------------------------------------------------------

## service

Geschäftslogik.

Beispielsweise:

``` text
TaskService
HabitService
CalendarService
PlanningService
TimeTrackingService
ProjectService
```

Hier liegen Regeln wie:

-   Task starten
-   Task abschließen
-   Habit abschließen
-   Zeit berechnen
-   Today View zusammenstellen

------------------------------------------------------------------------

## api

HTTP Handler und DTOs.

REST-Endpunkte beispielsweise:

``` text
GET    /api/tasks
POST   /api/tasks
GET    /api/tasks/:id
PATCH  /api/tasks/:id
DELETE /api/tasks/:id

POST   /api/tasks/:id/start
POST   /api/tasks/:id/pause
POST   /api/tasks/:id/complete

GET    /api/projects
POST   /api/projects

GET    /api/habits
POST   /api/habits

GET    /api/calendar/events
POST   /api/calendar/events

GET    /api/time-entries
POST   /api/time-entries

POST   /api/habits/:id/completions
DELETE /api/habits/:id/completions/:date

GET    /api/resources
POST   /api/resources
DELETE /api/resources/:id

GET    /api/today
```

------------------------------------------------------------------------

# WebSocket

WebSocket Events:

``` text
TASK_CREATED
TASK_UPDATED
TASK_DELETED
TASK_STARTED
TASK_PAUSED
TASK_COMPLETED

TIMER_STARTED
TIMER_STOPPED

PROJECT_CREATED
PROJECT_UPDATED
PROJECT_DELETED

HABIT_COMPLETED

CALENDAR_EVENT_CREATED
CALENDAR_EVENT_UPDATED
CALENDAR_EVENT_DELETED
```

Die WebUI und Extension können dadurch sofort reagieren.

Zusätzlich gibt es `HABIT_CREATED`, `HABIT_UPDATED`, `HABIT_DELETED` und
`HABIT_UNCOMPLETED`, damit auch Habit-Änderungen ankommen.

## Verbindung

`GET /ws` (WebSocket-Upgrade). Origin oder Token wie bei ändernden
Requests, sonst 403 oder 401. Der Server sendet pro Ereignis eine
Textnachricht:

``` json
{"type": "TIMER_STARTED", "id": "<Zeiteintrag>", "task_id": "<Task>"}
```

`id` ist das betroffene Objekt (bei Timer-Ereignissen der Zeiteintrag),
`task_id` kommt nur bei Timer-Ereignissen. Ereignisse tragen nur IDs; der
Client liest den neuen Stand per REST nach (z. B. `GET /api/today`).

-   Ereignisse entstehen im Service, erst nach erfolgreichem Speichern.
-   Ein Start, der eine andere Task pausiert, sendet `TIMER_STOPPED` und
    `TASK_PAUSED` für die alte, dann `TASK_STARTED` und `TIMER_STARTED`
    für die neue Task. Ein wirkungsloser Start (Timer läuft schon) und
    abgelehnte Aktionen senden nichts.
-   Verlässt eine Task per `PATCH` den Status `IN_PROGRESS`, kommen
    `TIMER_STOPPED` und `TASK_UPDATED`.
-   `TASK_DELETED` kann einen laufenden Timer mit löschen, ohne
    `TIMER_STOPPED`. Clients prüfen den Timer dann neu.
-   Es gibt keinen Verlauf. Nach einem Verbindungsabbruch liest der
    Client beim Wiederverbinden den Stand neu. Ein zu langsamer Client
    (64 Nachrichten Rückstand) wird getrennt.
-   Clients senden nichts. Der Server pingt alle 30 s.

------------------------------------------------------------------------

# Datenmodell

## Project

``` text
projects
---------
id
name
description
local_path
status
created_at
updated_at
```

## Task

``` text
tasks
-----
id
project_id
parent_task_id
title
description
status
priority
estimated_minutes
due_at
planned_date
planned_start_at
created_at
updated_at
completed_at
```

## Habit

``` text
habits
------
id
name
description
frequency_type
frequency_config
target_value
unit
preferred_time
start_date
end_date
active
created_at
```

`frequency_config` kann zunächst als JSON gespeichert werden.

Aktuelles Format je `frequency_type` (Wochentage als `MO`..`SU`):

``` text
DAILY              {}
WEEKLY             {"weekday": "WE"}            (ohne Angabe: Wochentag von start_date)
SPECIFIC_WEEKDAYS  {"weekdays": ["MO", "TH"]}
TIMES_PER_WEEK     {"times": 3}                 (1 bis 7)
```

Später kann daraus bei Bedarf ein stärker typisiertes Modell entstehen.

------------------------------------------------------------------------

## HabitCompletion

``` text
habit_completions
-----------------
id
habit_id
date
value
completed_at
note
```

`(habit_id, date)` ist eindeutig (Unique-Index).

------------------------------------------------------------------------

## CalendarEvent

``` text
calendar_events
---------------
id
title
description
start_at
end_at
location
url
project_id
task_id
recurrence_rule
recurrence_exdates
created_at
updated_at
```

------------------------------------------------------------------------

## Resource

``` text
resources
---------
id
task_id
project_id
type
target
label
created_at
```

`type` ist `FILE`, `FOLDER` oder `URL`. Genau eines von `task_id` und
`project_id` ist gesetzt (CHECK). Löscht man die Task oder das Projekt,
werden seine Ressourcen mit gelöscht (`ON DELETE CASCADE`).

------------------------------------------------------------------------

## TimeEntry

``` text
time_entries
------------
id
task_id
project_id
started_at
ended_at
source
```

Höchstens ein Eintrag mit `ended_at IS NULL` (partieller Unique-Index).

`source` ist `MANUAL`, `VSCODIUM` oder `AUTOMATIC`. Genau eines von
`task_id` und `project_id` ist gespeichert (CHECK): Ein Task-Eintrag
speichert kein Projekt, `project_id` in der API ist dann das Projekt der
Task zum Zeitpunkt der Abfrage. Löscht man Task oder Projekt, werden die
Einträge mit gelöscht (`ON DELETE CASCADE`). Zeitpunkte stehen in dieser
Tabelle mit fester Breite (`YYYY-MM-DDTHH:MM:SS.mmmZ`), damit Textvergleich
und Zeitreihenfolge übereinstimmen.

Start, Pause und Abschluss laufen in einer Transaktion (Task-Status und
Zeiteintrag ändern sich gemeinsam). Die Datenbank öffnet Transaktionen mit
`_txlock=immediate`, damit sich gleichzeitige Timer-Wechsel
hintereinander einreihen.

API:

``` text
GET  /api/time-entries   ?task_id= ?project_id= ?from= ?to= ?running=true
POST /api/time-entries   {task_id|project_id, started_at, ended_at, source?}
POST /api/tasks/:id/start     {source?}   (Body optional)
POST /api/tasks/:id/pause
POST /api/tasks/:id/complete
```

`from` ist inklusive, `to` exklusiv, beide gelten für `started_at`
(RFC 3339, ein `+` im Offset als `%2B` kodieren). `?running=true` liefert
den laufenden Timer. Start, Pause und Abschluss antworten mit
`{"task": …, "time_entry": … | null}`.

------------------------------------------------------------------------

# Zeit und Zeitzonen

-   Zeitpunkte (`*_at`) werden in UTC im Format RFC 3339 gespeichert.
-   Tage (`planned_date`, `start_date`, `end_date`,
    `habit_completions.date`) werden als lokales Datum `YYYY-MM-DD`
    gespeichert.
-   Uhrzeiten ohne Datum (`preferred_time`) werden als lokale Zeit
    `HH:MM` gespeichert.
-   Die Zeitzone (IANA, z. B. `Europe/Berlin`) steht in der
    Konfiguration. Standard ist die Systemzeitzone.
-   Die Today View und `GET /api/today?date=…` rechnen in dieser
    Zeitzone.

## GET /api/today

`?date=YYYY-MM-DD`, Standard ist heute in der konfigurierten Zeitzone.
Die Antwort ist nur lesend und enthält:

``` text
date, timezone, day_start, day_end   Tag [day_start, day_end) in UTC
                                     (an Zeitumstellungen 23 oder 25 h)
events               Termine, die den Tag berühren, nach Beginn sortiert.
                     Wie GET /api/calendar/events plus occurrence_start
                     und occurrence_end (der konkrete Termin; bei Serien
                     gehören start_at/end_at zur ersten Wiederholung)
tasks                für den Tag geplante Tasks ohne CANCELLED; zuerst mit
                     planned_start_at nach Uhrzeit, dann ohne Uhrzeit
active_tasks         IN_PROGRESS-Tasks, die nicht für den Tag geplant sind
habits               fällige Habits (Habit-Felder plus done und
                     week_progress bei TIMES_PER_WEEK)
running_time_entry   laufender Timer oder null (unabhängig vom Tag)
planned_minutes      Summe estimated_minutes von tasks
calendar_minutes     belegte Zeit der Termine im Tag; Überlappungen zählen
                     einmal, Termine über Mitternacht anteilig
tracked_minutes      erfasste Zeit der Einträge, die an dem Tag gestartet
                     sind; ein laufender Timer zählt bis jetzt
```

Freie Zeit und Überplanung fehlen bewusst: Sie brauchen ein Zeitfenster
für Arbeitszeit, das es noch nicht gibt (Phase 3, Planning). Die Today
View kann `planned_minutes` und `calendar_minutes` schon anzeigen.

------------------------------------------------------------------------

# SQLite

SQLite ist die primäre Datenbank im MVP.

Vorteile:

-   lokal
-   eine Datei
-   keine externe Datenbank notwendig
-   einfaches Backup
-   schnell genug für eine persönliche Anwendung

Die Datenbankdatei soll nicht im Frontend liegen.

------------------------------------------------------------------------

# API-Prinzip

Das Backend ist die zentrale Source of Truth.

Nicht:

``` text
WebUI speichert eigene Daten
Extension speichert eigene Daten
```

Sondern:

``` text
WebUI ─┐
       ├──> Go Backend ───> SQLite
Extension ─┘
```

------------------------------------------------------------------------

# VSCodium

Die Extension darf direkt mit dem lokalen Dateisystem arbeiten, wenn
dies für ihre Aufgabe notwendig ist.

Sie soll aber die Produktdaten nicht selbst dauerhaft verwalten.

Produktdaten gehören ins Backend.

------------------------------------------------------------------------

# Lokale Sicherheit

Das Backend soll standardmäßig nur auf localhost lauschen.

Beispielsweise:

``` text
127.0.0.1:<port>
```

Keine öffentliche Bind-Adresse im MVP.

Ein localhost-Bind allein reicht nicht, weil Webseiten im Browser
ebenfalls 127.0.0.1 ansprechen können. Deshalb:

-   Der `Host`-Header muss `127.0.0.1:<port>` oder `localhost:<port>`
    sein (Schutz gegen DNS-Rebinding).
-   Ändernde Requests und WebSocket-Verbindungen werden nur akzeptiert,
    wenn `Origin` der eigene Origin ist.
-   Die Extension authentifiziert sich zusätzlich mit einem lokalen
    Token (`Authorization: Bearer …`). Das Token wird beim ersten Start
    in `~/.config/kairo/token` mit Dateirechten `0600` erzeugt.

------------------------------------------------------------------------

# Betrieb

-   Autostart: `kairo install` richtet einen macOS-LaunchAgent
    (`~/Library/LaunchAgents/`) ein, sodass das Backend beim Login
    startet. `kairo uninstall` entfernt ihn.
-   Läuft das Backend nicht, zeigt die Extension „Backend offline“ und
    versucht regelmäßig, sich neu zu verbinden.
-   WebUI: Das Go-Binary liefert die gebaute WebUI selbst aus (`embed`),
    erreichbar unter `http://127.0.0.1:<port>/`.
-   Entwicklung: Der Vite-Dev-Server leitet `/api` und `/ws` an das
    Backend weiter. Dadurch ist kein CORS nötig.

------------------------------------------------------------------------

# Konfigurierbarkeit

Port, Datenbankpfad, Zeitzone und ggf. Log-Level sollen über
Konfiguration steuerbar sein.

------------------------------------------------------------------------

# Erweiterbarkeit

Die Architektur soll später ermöglichen:

``` text
SQLite
   ↓
PostgreSQL
```

ohne die gesamte Business-Logik umzuschreiben.

Deshalb Repository und Domain sauber trennen.

------------------------------------------------------------------------

# Kalender-Synchronisation

Im MVP ist der interne Kalender die Source of Truth.

Externe Kalender können später über Adapter angebunden werden.

Beispielsweise:

``` text
CalendarProvider
├── LocalCalendar
├── GoogleCalendar
└── AppleCalendar
```

Aber diese Integrationen gehören NICHT in den ersten MVP.
