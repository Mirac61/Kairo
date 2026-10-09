# Kairo — Vision

## Produktidee

**Kairo** ist ein persönliches **Operating System für Planung und
Arbeit** (Personal OS).

Es ist kein Notion-Klon, kein Obsidian-Klon und keine klassische
To-do-App.

Die zentrale Idee lautet:

> **WebUI = Planen. VSCodium = Arbeiten.**

Die WebUI ist das zentrale Planungs- und Steuerzentrum für das
persönliche Leben.

VSCodium ist der tatsächliche digitale Arbeitsplatz für Projekte,
Dateien, Dokumente und Programmierung.

Das System verbindet beides.

------------------------------------------------------------------------

# Die fünf Kernbereiche

## 1. Calendar

Der Kalender beschreibt Dinge, die zu einer bestimmten Zeit stattfinden.

Beispiele:

-   Vorlesungen
-   Termine
-   Meetings
-   Prüfungen
-   Arzttermine
-   feste private Termine
-   Zeitblöcke

Ein Kalender-Event ist nicht automatisch eine Aufgabe.

------------------------------------------------------------------------

## 2. Tasks

Tasks beschreiben Dinge, die erledigt werden müssen.

Beispiele:

-   AlgoDat Musterprüfung A durchrechnen
-   Bewerbung schreiben
-   Bug im Activitytracker beheben
-   AlgoDat Kapitel 5 (Suchverfahren) lernen

Tasks können geplant, priorisiert und Projekten zugeordnet werden.

------------------------------------------------------------------------

## 3. Habits

Habits beschreiben wiederkehrende Verhaltensweisen.

Beispiele:

-   täglich 30 Minuten lesen
-   dreimal pro Woche Sport
-   jeden Abend planen
-   morgens lernen

Ein Habit ist keine normale Aufgabe mit einem weit entfernten
Deadline-Datum.

Das System erzeugt bzw. verfolgt Habit-Occurrences anhand einer Regel.

------------------------------------------------------------------------

## 4. Projects

Projects bündeln Arbeit und Kontext.

Beispiele:

-   AlgoDat
-   Software Testing
-   Activitytracker
-   Kairo (dieses Projekt)
-   Bewerbungen

Ein Projekt kann Tasks, Ressourcen, Dateien, URLs und einen lokalen
Ordner besitzen.

------------------------------------------------------------------------

## 5. VSCodium Workspace

VSCodium ist der tatsächliche Arbeitsbereich.

Ein Projekt kann mit einem lokalen Pfad verbunden sein:

``` text
~/Documents/Uni/AlgoDat          Uni-Material (Skripte, Prüfungen, Notizen)
~/code/uni/activitytracker       Uni-Code-Projekt
~/code/personal/Kairo            privates Code-Projekt
~/Documents/bewerbungen          Bewerbungen (Typst)
```

Wenn eine Aufgabe zu diesem Projekt gestartet wird, kann die
VSCodium-Extension den passenden Workspace und die relevanten Ressourcen
öffnen.

------------------------------------------------------------------------

# Die zentrale Verbindung

``` text
                            KAIRO
                              │
             ┌────────────────┼────────────────┐
             │                │                │
          CALENDAR          TASKS            HABITS
             │                │                │
             └────────────────┼────────────────┘
                              │
                           PROJECTS
                              │
                              ▼
                         VSCODIUM
                              │
                              ▼
                           ARBEIT
```

------------------------------------------------------------------------

# Ein typischer Tag

``` text
08:30  📅 AlgoDat Vorlesung
10:15  🔴 AlgoDat Musterprüfung A bearbeiten
12:30  📅 Mittagessen
14:00  💻 Activitytracker Login reparieren
18:00  🔁 Sport
22:30  🔁 Lesen
```

Die Today View soll daraus einen verständlichen Tagesüberblick machen.

Sie beantwortet:

> Was passiert heute?

> Was muss ich heute erledigen?

> Welche Gewohnheiten stehen an?

> Woran sollte ich jetzt arbeiten?

------------------------------------------------------------------------

# WebUI

Die WebUI ist hauptsächlich für:

-   Tagesplanung
-   Wochenplanung
-   Kalender
-   Tasks
-   Habits
-   Projekte
-   Prioritäten
-   Zeitplanung
-   Rückblicke
-   Notizen (Markdown, PDFs und Bilder in einem normalen Ordner)

gedacht.

Sie soll nicht versuchen, VSCodium zu ersetzen.

------------------------------------------------------------------------

# VSCodium

Die Extension ist hauptsächlich für:

-   aktuellen Projektkontext
-   aktuelle Aufgabe
-   heutige relevante Aufgaben
-   Projektdateien
-   Dokumente
-   Timer
-   Arbeitsstatus
-   Ressourcen

gedacht.

------------------------------------------------------------------------

# Kernprinzip

Der Nutzer soll möglichst schnell von:

``` text
Was muss ich machen?
```

zu:

``` text
Wo muss ich dafür arbeiten?
```

zu:

``` text
Ich arbeite jetzt.
```

kommen.

------------------------------------------------------------------------

# Local-first

Das System soll zunächst vollständig lokal funktionieren.

Keine zwingende Cloud.

Keine zwingende Registrierung.

Daten gehören dem Nutzer.

------------------------------------------------------------------------

# Produktphilosophie

Nicht möglichst viele Features bauen.

Stattdessen:

-   schnell
-   klar
-   lokal
-   kontextbezogen
-   automatisiert
-   transparent

Das Produkt soll sich wie ein persönliches Betriebssystem anfühlen und
nicht wie eine weitere Datenbank, die gepflegt werden muss.
