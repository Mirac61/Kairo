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

GET    /api/projects
POST   /api/projects

GET    /api/habits
POST   /api/habits

GET    /api/calendar/events
POST   /api/calendar/events

GET    /api/today
```

------------------------------------------------------------------------

# WebSocket

WebSocket Events:

``` text
TASK_STARTED
TASK_PAUSED
TASK_COMPLETED

TIMER_STARTED
TIMER_STOPPED

PROJECT_CHANGED

HABIT_COMPLETED

CALENDAR_EVENT_CREATED
CALENDAR_EVENT_UPDATED
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
workspace_path
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

------------------------------------------------------------------------

# Konfigurierbarkeit

Port, Datenbankpfad und ggf. Log-Level sollen über Konfiguration
steuerbar sein.

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
