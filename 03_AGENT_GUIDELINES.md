# Agent Guidelines

## Projektverständnis

Du arbeitest an **Kairo**, einem Personal OS.

Es verbindet:

``` text
Calendar
Tasks
Habits
Projects
VSCodium
Time Tracking
```

Die zentrale Produktidee ist:

> **WebUI = Planen. VSCodium = Arbeiten.**

------------------------------------------------------------------------

# Produktgrenzen

## Calendar ist nicht Task

Ein Kalender-Event:

> "Vorlesung 10:00–12:00"

ist nicht automatisch eine Aufgabe.

## Habit ist nicht Task

Ein Habit:

> "30 Minuten lesen"

ist eine wiederkehrende Verhaltensregel.

## Project ist nicht Task

Ein Project ist Kontext.

Tasks sind konkrete Arbeit innerhalb dieses Kontextes.

------------------------------------------------------------------------

# Beziehung der Objekte

``` text
Project
 ├── Tasks
 ├── Resources
 └── VSCodium Workspace

CalendarEvent
 └── optional Project/Task

Habit
 └── HabitCompletions

Task
 ├── optional Project
 ├── Resources
 └── TimeEntries
```

------------------------------------------------------------------------

# Wichtigste UX-Regel

Der Nutzer soll nicht seine gesamte Produktivitätsdatenbank pflegen
müssen.

Das System soll möglichst viel Kontext liefern.

Beispiel:

Der Nutzer öffnet:

``` text
~/code/uni/activitytracker
```

Die Extension erkennt:

``` text
Project = Activitytracker
```

und zeigt:

``` text
Heute

1. Login reparieren
2. Backend-Ordnerstruktur aufräumen
3. Tests schreiben
```

------------------------------------------------------------------------

# VSCodium ist Arbeitsumgebung

Die Extension soll nicht versuchen, einen vollständigen IDE-Ersatz oder
ein zweites Notion zu bauen.

Sie soll:

-   Aufgaben anzeigen
-   Projekte erkennen
-   Ressourcen öffnen
-   Timer steuern
-   Status aktualisieren
-   relevanten Kontext anzeigen

------------------------------------------------------------------------

# WebUI ist Planungsumgebung

Die WebUI soll:

-   Kalender
-   Today View
-   Wochenplanung
-   Tasks
-   Habits
-   Projects
-   Zeitplanung

bereitstellen.

------------------------------------------------------------------------

# Backend ist Source of Truth

WebUI und Extension speichern Produktdaten nicht unabhängig voneinander.

``` text
WebUI ─┐
       ├── Go Backend ─── SQLite
Extension ─┘
```

------------------------------------------------------------------------

# Go-Regeln

Das Backend muss in Go geschrieben werden.

Bevorzugt:

-   idiomatisches Go
-   kleine Packages
-   klare Interfaces
-   `context.Context` für I/O
-   strukturierte Fehler
-   Tests für Business-Logik
-   keine unnötigen Frameworks

Keine riesige Enterprise-Architektur.

------------------------------------------------------------------------

# API-Regeln

HTTP Handler sollen keine komplexe Business-Logik enthalten.

Nicht:

``` text
HTTP Handler
 ├── DB Query
 ├── Business Rules
 ├── Timer Logic
 └── WebSocket Logic
```

Besser:

``` text
Handler
  ↓
Service
  ↓
Repository
  ↓
SQLite
```

------------------------------------------------------------------------

# Datenbank

Schemaänderungen ausschließlich über Migrationen.

Keine manuellen Tabellenänderungen als Teil der normalen Entwicklung.

------------------------------------------------------------------------

# Habits

Ein Habit muss eine Frequenz besitzen.

Beispiele:

``` text
daily
weekly
specific weekdays
x times per week
```

Die Habit-Logik muss so implementiert werden, dass spätere Frequenztypen
möglich sind.

------------------------------------------------------------------------

# Calendar

Der interne Kalender soll zunächst einfach und zuverlässig sein.

Keine unnötige Integration mit externen Kalenderdiensten im MVP.

------------------------------------------------------------------------

# Automatisierung

Automatisierung darf niemals zu falschen Daten führen.

Insbesondere:

> Workspace geöffnet ≠ automatisch gearbeitet.

Ein Timer soll deshalb nicht allein aufgrund des Öffnens eines Projektes
starten.

------------------------------------------------------------------------

# Zeittracking

Immer echte Zeitstempel verwenden.

Nicht:

``` text
timer += 1
```

Sondern:

``` text
started_at
ended_at
```

Pausen werden ebenfalls über Zeitintervalle abgebildet.

------------------------------------------------------------------------

# Keine Feature Inflation

Wenn ein Feature nicht mindestens einen dieser Punkte verbessert:

-   Planung
-   Kontext
-   tatsächliche Arbeit
-   Rückblick

ist es wahrscheinlich nicht prioritär.

------------------------------------------------------------------------

# Entwicklungsstil

Kleine vertikale Schritte.

Beispiel:

``` text
Task erstellen
↓
SQLite
↓
Go API
↓
WebUI
↓
VSCodium Extension
↓
Synchronisation
```

Nicht alle Ebenen gleichzeitig unfertig bauen.

Die Phasen in `04_AGENT_HANDOFF.md` und `05_ROADMAP.md` geben die grobe
Reihenfolge vor. Innerhalb einer Phase wird jede Funktion als vertikale
Scheibe gebaut (Migration → Repository → Service → API → UI), bevor die
nächste begonnen wird.

------------------------------------------------------------------------

# Definition of Done

Eine Funktion ist fertig, wenn:

-   sie funktioniert
-   Fehlerfälle behandelt werden
-   sie persistiert wird
-   WebUI und Extension konsistent sind (soweit die Funktion dort in
    der aktuellen Phase bereits existiert)
-   Tests für relevante Business-Logik existieren
-   die Lösung verständlich bleibt
-   keine unnötige Komplexität eingeführt wurde
