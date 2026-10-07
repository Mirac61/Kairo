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
DELETE /api/tasks/:id            (Papierkorb; ?permanent=true endgültig)
POST   /api/tasks/:id/restore

POST   /api/tasks/:id/start
POST   /api/tasks/:id/pause
POST   /api/tasks/:id/complete

GET    /api/projects
POST   /api/projects

GET    /api/habits
POST   /api/habits

GET    /api/calendar/events
POST   /api/calendar/events
POST   /api/calendar/import

GET    /api/time-entries
POST   /api/time-entries

POST   /api/habits/:id/completions
DELETE /api/habits/:id/completions/:date
DELETE /api/habits/:id           (Papierkorb; ?permanent=true endgültig)
POST   /api/habits/:id/restore
DELETE /api/calendar/events/:id  (Papierkorb; ?permanent=true endgültig)
POST   /api/calendar/events/:id/restore
GET    /api/trash

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
-   Die `*_DELETED`-Ereignisse von Tasks, Terminen und Habits melden auch
    das Verschieben in den Papierkorb; `restore` sendet `TASK_CREATED`,
    `CALENDAR_EVENT_CREATED` bzw. `HABIT_CREATED`.
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
color
created_at
updated_at
```

`local_path` muss ein absoluter Pfad (oder mit `~` beginnend) sein und
existieren; das Backend prüft das beim Anlegen und beim Ändern des
Pfads, gespeichert wird die Eingabe (die Extension löst `~` selbst
auf). `color` ist ein Name aus der Palette `violet`, `blue`, `orange`,
`aqua`, `pink`, `yellow`, `green`, `red`; ein neues Projekt ohne Angabe
bekommt die am wenigsten benutzte Farbe.

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
external_uid
created_at
updated_at
```

`external_uid` ist die UID aus einem ICS-Import (eindeutig, sonst NULL). Ein
erneuter Import aktualisiert den Termin, statt ihn doppelt anzulegen.
`POST /api/calendar/import` nimmt den rohen `.ics`-Text (höchstens 5 MiB) und
antwortet mit `{created, updated, skipped, unsupported_rules, notes}`. Serien
importiert er nur mit den Regeln, die `recurrence_rule` kennt; alles andere
wird ein Einzeltermin mit Notiz. Fehlende Termine löscht er nicht.

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

`type` ist `FILE`, `FOLDER` oder `URL`. Beim Anlegen darf er fehlen: `http(s)://`
ist `URL`, ein vorhandenes Verzeichnis `FOLDER`, alles andere `FILE`. Genau eines von `task_id` und
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

# Papierkorb

Tasks, Termine und Habits werden beim Löschen nur markiert
(`deleted_at`, Migration 0009; `NULL` = aktiv). Projekte, Zeiteinträge
und Ressourcen werden weiterhin hart gelöscht.

-   `DELETE /api/tasks|calendar/events|habits/:id` antwortet `204` und
    setzt `deleted_at`. Bei einer Task tragen auch ihre Teilaufgaben
    denselben Zeitstempel. Läuft ein Timer auf der Task oder einer
    Teilaufgabe, kommt `409` („erst pausieren“).
-   `?permanent=true` löscht endgültig, auch aus dem Papierkorb. Eine
    Task mit Zeiteinträgen bleibt `409` („stattdessen abbrechen“), weil
    `ON DELETE CASCADE` die Zeit sonst mitlöschen würde.
-   `POST …/:id/restore` antwortet `200` mit dem Objekt und ist
    idempotent. Eine Task holt die mit ihr gelöschten Teilaufgaben
    (gleicher `deleted_at`) zurück, früher einzeln gelöschte nicht.
-   `GET /api/trash` liefert `{tasks, events, habits}`, jeweils neueste
    zuerst, mit `deleted_at`. Zusammen mit der Elterntask gelöschte
    Teilaufgaben stehen nicht einzeln darin.
-   Alle Lese-Abfragen blenden Papierkorb-Zeilen aus (Listen, `GET` →
    `404`, `/api/today`, Occurrences, Habit-Fälligkeit, Rückblick);
    `PATCH`, `start`, `pause` und `complete` auf ihnen sind `404`. Die
    Zeit auf Papierkorb-Tasks zählt in den Summen weiter, und Termine
    behalten ihren `task_id` (erst das endgültige Löschen löst ihn).
-   Der ICS-Import belebt Termine im Papierkorb nicht wieder und legt
    sie nicht doppelt an (`skipped`, Notiz „im Papierkorb“).

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
work_minutes         Länge des Arbeitsfensters an dem Tag
free_minutes         Rest des Arbeitsfensters (heute ab jetzt, sonst ganz)
                     minus Termine darin minus offene geplante Tasks
                     (ohne Schätzung 30 Min), mindestens 0
overplanned_minutes  der negative Rest davon (sonst 0)
unestimated_tasks    Anzahl der Tasks ohne Schätzung in free_minutes
```

## GET /api/calendar/occurrences

`?from=…&to=…` (beide Pflicht, RFC 3339, ein `+` im Offset als `%2B`).
Liefert die konkreten Termine, die das Fenster berühren, nach Beginn
sortiert, Serien aufgelöst. Format wie `events` in `GET /api/today`
(Termin plus `occurrence_start` und `occurrence_end`). Die Kalenderansicht
der WebUI liest damit Wochen und Monate in einem Aufruf.

## Arbeitszeitfenster

Ein festes Fenster pro Tag, gleich für alle Wochentage. Es kommt aus der
Konfiguration: `KAIRO_WORK_START` und `KAIRO_WORK_END` (`HH:MM`, Ortszeit
der konfigurierten Zeitzone, Standard `09:00` bis `17:00`; Ende muss nach
dem Start liegen). An Zeitumstellungen ist das Fenster entsprechend
länger oder kürzer.

Termine zählen nur mit dem Anteil im Fenster. Geplante Tasks zählen mit
ihrer vollen Schätzung, auch wenn ihre Uhrzeit außerhalb des Fensters
liegt, und überlappen sich nicht mit Terminen: Die Rechnung ist eine
Summe, keine Slot-Suche. Pro Wochentag abweichende Zeiten und eine
Vorschlagslogik für freie Slots gibt es nicht; das kommt erst, wenn es
gebraucht wird.

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
    Gebaut wird mit `go build -o ~/.local/bin/kairo ./cmd/server`; der
    Agent (`app.kairo.backend`) startet genau dieses Binary mit den beim
    Installieren gesetzten `KAIRO_*`-Variablen, hält es am Laufen
    (`KeepAlive`) und schreibt das Log nach `~/Library/Logs/kairo.log`.
    Nach einem Update das Binary ersetzen und `kairo install` erneut
    ausführen. Ein Binary von `go run` wird abgelehnt.
-   Läuft das Backend nicht, zeigt die Extension „Backend offline“ und
    versucht regelmäßig, sich neu zu verbinden.
-   WebUI: Das Go-Binary liefert die gebaute WebUI selbst aus (`embed`),
    erreichbar unter `http://127.0.0.1:<port>/`.
-   Entwicklung: Der Vite-Dev-Server leitet `/api` und `/ws` an das
    Backend weiter. Dadurch ist kein CORS nötig.

------------------------------------------------------------------------

# Konfigurierbarkeit

Port, Datenbankpfad, Zeitzone, Arbeitszeitfenster und ggf. Log-Level
sollen über Konfiguration steuerbar sein.

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
