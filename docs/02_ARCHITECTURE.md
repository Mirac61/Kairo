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
```

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
