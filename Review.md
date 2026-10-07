# Kairo – UX- und Produkt-Review

Stand: 7. Oktober 2026, Commit `d24e029`. Geprüft wurden Doku (AGENTS.md, docs/00–05, READMEs), der
komplette Oberflächen-Code (WebUI, Extension) und die Backend-API, soweit die UI sie nutzt.

**Wie geprüft wurde**

- Backend als eigenes Binary mit **temporärer Datenbank** (Port 8799, `KAIRO_DB_PATH` im Temp-Ordner,
  eigenes Token, `Europe/Berlin`) gestartet und per API mit Testdaten gefüllt: zwei Projekte mit
  Ressourcen, Tasks (mit und ohne Uhrzeit, eine überfällige, eine im Backlog), eine wöchentliche
  Vorlesungsserie, ein Einzeltermin, zwei Habits, laufende Timer, ein nachgetragener Zeiteintrag.
  Deine echte DB und dein laufendes Backend auf 8742 habe ich nicht angefasst.
- WebUI über einen Vite-Dev-Server gegen dieses Backend geladen und mit Headless-Chromium
  durchgeklickt: alle fünf Ansichten in Dark und Light, eine aufgeklappte Task, 800 px Breite und die
  Tab-Reihenfolge per Tastatur.
- Extension: Die Webview (`media/view.js` + `view.css`) habe ich in 300 px Breite mit nachgebauter
  VSCodium-API und Dark-Theme-Variablen gerendert, alle fünf Tabs plus Offline-Zustand.
- Abläufe per API geprüft: Löschen mit Zeiteinträgen, Start einer abgebrochenen Task, Zeiteintrag mit
  Ende vor Start, negative Schätzung, Projekt mit Zeiteinträgen löschen, Schreibzugriff ohne Token.

**Nicht geprüft** (ausdrücklich):

- Die Extension im echten VSCodium: Aktivierung, Workspace-Erkennung, Öffnen von Dateien und Ordnern,
  `revealFileInOS`, die Inaktivitäts-Rückfrage.
- Echtzeit-Sync zwischen zwei offenen Clients.
- `kairo install` (LaunchAgent).
- Screenreader und Extension im Light-Theme.

Diese Punkte bewerte ich nur anhand des Codes. Die Uhr im Browser stand für die Screenshots
fest auf 10:20 Uhr. Absolute Timer-Werte in den Screenshots sind deshalb Artefakte und fließen nicht
in die Bewertung ein.

---

## 1. Kurzfazit

Kairo ist ein **sauber gebautes, ehrliches Fundament** mit einer klaren, richtigen Idee: Planen im
Browser, Arbeiten im Editor, dazwischen ein lokales Backend als einzige Wahrheit. Das Backend ist für
ein Privatprojekt ungewöhnlich solide. Es hat echte Zeitstempel, höchstens einen laufenden Timer,
409 statt stillem Datenverlust, automatische Backups und Token plus Origin-Prüfung. Das
Zeittracking-Modell inklusive rückdatierter Pause nach Inaktivität ist durchdachter als bei Toggl.
**Das größte Problem:** Das Alleinstellungsmerkmal, die Brücke Planen ↔ Arbeiten, ist bisher nur
angedeutet. Eine Task zu starten öffnet keinen Workspace und keine Ressourcen. Im Editor gibt es
keinen Timer in der Statusleiste und keine Befehle in der Befehlspalette. Task-Ressourcen lassen sich
in keiner Oberfläche anlegen. Stattdessen sind Stats-Heatmaps, ein Dateibaum und Habit-Kacheln
entstanden, die es woanders schon besser gibt. Die WebUI sieht ruhig und hochwertig aus, ist aber
interaktiv dünn: kein Undo, keine Tastaturkürzel, kein Ziehen aus dem Backlog in den Tag, keine
Möglichkeit, eine Task abzubrechen, obwohl die App genau das beim Löschen verlangt. **Weitermachen
lohnt sich**, aber nur, wenn die nächsten Wochen fast ausschließlich in die Brücke fließen und nicht
in weitere Ansichten.

---

## 2. Bewertungstabelle

| Bereich | Note | Begründung |
|---|---:|---|
| Produktidee/Vision | 7 | Klare, eigenständige These („Was muss ich machen? → Wo? → Ich arbeite jetzt“), die keine bestehende Tool-Kombination sauber abdeckt. Die Zielgruppe Student plus Entwickler ist schmal, aber echt. |
| Scope-Fokus | 5 | Die Doku ist vorbildlich fokussiert, die Umsetzung driftet: Stats-Tab, Dokumente-Baum und 28-Tage-Habit-Raster sind gebaut, während Kernversprechen der Vision (Workspace beim Start öffnen, Task-Ressourcen) fehlen. |
| Informationsarchitektur | 5 | Die WebUI-Navigation ist klar, aber die Benennung schwankt („Start“ vs. Today, „Aufgaben“ vs. „Task“). Die Extension packt fünf Tabs in ein schmales Panel, mit Doppelungen (Projekt-Tasks in „Jetzt“ und „Heute“). Review existiert nur als API. |
| Interaktionsdesign | 4 | Quick-Add und Inline-Bearbeitung sind ordentlich. Es fehlen Undo, Tastaturkürzel, Rückmeldung nach dem Speichern, Ziehen aus dem Backlog und Ausnahmen für Serien. Ein Termin oder eine ganze Serie wird mit einem Klick ohne Rückfrage gelöscht. |
| Visuelles Design | 7 | Eigenständige, ruhige Ästhetik (Kanagawa/Vercel), gute Typografie und Mono-Zahlen, Dark und Light stimmig. Kleine Fehler: unterstrichene Navigation, unsichtbare Kalender-Icons, überlappende Blöcke im Tagesplan. |
| Barrierefreiheit | 4 | Es gibt Focus-Ringe, ARIA-Rollen an Checkboxen und `prefers-reduced-motion`. Platzhalter und deaktivierte Texte haben nur etwa 2:1 Kontrast. Dialoge haben keine Fokusfalle, der Kalenderdialog nicht einmal `role="dialog"`. Die Buttons „Start“ und „Fertig“ haben keinen Task-Bezug im Namen. |
| WebUI gesamt | 5 | Schön, aber eher ein Viewer mit Formularen als ein Planungswerkzeug. Der wichtigste Bildschirm („Start“) beantwortet die Frage „Woran arbeite ich jetzt?“ nicht. |
| Extension gesamt | 5 | „Jetzt“ mit Projektressourcen und Timer ist der beste Teil des Produkts. Der Rest dupliziert vorhandene Funktionen (Dateibaum, Stats). Statusleiste, Befehle und Workspace-Öffnen fehlen. |
| Integration Planen↔Arbeiten | 4 | Es gibt Sync über WebSocket, Workspace-Erkennung und gemeinsame Timer. Aber der Weg „Task in der WebUI starten → richtiger Workspace und richtige Dateien in VSCodium“ existiert nicht. |
| Zeittracking | 6 | Ehrliches Datenmodell, Korrektur von Start und Ende, rückdatierte Pause bei Inaktivität: stark. Es fehlt „Zeit nachtragen“ in der UI, Korrektur gibt es nur für heute, und beim Löschen eines Projekts verlieren alte Einträge still ihre Zuordnung. |
| Onboarding/Setup | 3 | Ein neuer Nutzer braucht Go, Node, die richtige Build-Reihenfolge, F5 in einem Extension-Entwicklungsfenster und tippt Projektpfade von Hand. Es gibt keinen ersten Lauf, keinen Import und keine `.vsix`. |
| Robustheit/Datensicherheit | 7 | Backups, Migrationen, Token mit 0600, Host- und Origin-Prüfung, 409-Regeln: sehr gut. Abzug für fehlendes Undo und Papierkorb, Löschen ohne Rückfrage im Kalender und keinen Export. |
| Code-Qualität aus Produktsicht | 7 | Klein, lesbar und getestet (Backend). Es gibt Doppellogik (Projektfarbe zweimal, Stats in der Extension parallel zu `/api/review`), drei Token-Systeme fürs Styling und PrimeVue für ganze zwei Komponenten. Das Frontend hat keine Tests. |
| Doku | 6 | Ausführlich, konsistent formuliert und gut für Agenten. Sie verspricht aber Dinge, die die App nicht tut (Habits in der Zeitleiste, Tasks in den Tag ziehen, Workspace beim Start öffnen). Stats- und Dokumente-Tab kommen in der Doku gar nicht vor, und die Extension-README ist veraltet. |
| **Gesamtnote** | **5** | Brauchbar und handwerklich sauber, aber noch unauffällig. Das, was Kairo besser machen könnte als Things plus Toggl plus Explorer, ist noch nicht gebaut. |

---

## 3. Was aktuell schlecht ist

| # | Fundstelle | Problem | Auswirkung | Schwere |
|---|---|---|---|---|
| 1 | `TasksView.vue:103-106`, `TaskActions.vue` | Es gibt keine Möglichkeit, eine Task auf `CANCELLED` (oder `BACKLOG`) zu setzen. Löschen einer Task mit Zeiteinträgen liefert „Konflikt: Task hat Zeiteinträge, stattdessen abbrechen“ (per API verifiziert), aber es gibt keinen Abbrechen-Button. Abgebrochene Tasks erscheinen in keinem Filter. | Der Nutzer steckt in einer Sackgasse: Die App sagt ihm, was zu tun ist, bietet es aber nicht an. Nicht mehr relevante Tasks bleiben für immer in „Alle“ stehen. | **kritisch** |
| 2 | `CalendarView.vue:176-180, 289` | „Löschen“ im Termin-Dialog löscht sofort, ohne Rückfrage und ohne Undo, bei Serien die **ganze Serie**. | Ein Fehlklick löscht den kompletten Semester-Stundenplan. | **kritisch** |
| 3 | Extension gesamt (`package.json` contributes) | Kein Statusleisten-Eintrag, keine Befehle zum Starten, Pausieren, Abschließen oder Wechseln der Task, keine Tastenkürzel. Es gibt nur „Aktualisieren“ und „WebUI öffnen“. | Der Timer ist nur sichtbar, wenn die Kairo-Seitenleiste offen ist. Im Editor, wo laut Vision gearbeitet wird, ist Kairo unsichtbar. Das ist der größte Hebel des Produkts und er bleibt ungenutzt. | **hoch** |
| 4 | `contextView.ts:87-88, 253-260`; Vision „Wenn eine Aufgabe gestartet wird, kann die Extension den passenden Workspace und die relevanten Ressourcen öffnen“ | Eine Task zu starten (WebUI oder Extension) öffnet nichts. Den Workspace öffnet nur der Projekt-Button im Tab „Projekte“. | Der Kernfluss der Vision „Aufgabe auswählen → richtiger Workspace → richtige Dateien“ existiert nicht. | **hoch** |
| 5 | `client.ts:188-194` (kein `task_id`), `ProjectsView.vue:153-165`, `backendClient.ts:30` | Das Backend kann Ressourcen pro Task, aber keine UI legt sie an oder zeigt sie. | Das Paradebeispiel aus der PRD („Musterprüfung A → PDF, Lösung, Moodle“) lässt sich nicht abbilden. | **hoch** |
| 6 | `TodayView.vue:106-108, 185-196` | Der große Fokusbereich („In 2 Std 10 Min · Mittagessen“) betrachtet nur Termine (`kind === 'event'`). Der laufende Timer steht klein als fünfte Kennzahl, die nächste geplante Task taucht nicht auf. | Die wichtigste Frage der Vision („Woran sollte ich jetzt arbeiten?“) beantwortet der wichtigste Bildschirm nicht. Stattdessen nimmt die größte Schrift der App das Mittagessen ein. | **hoch** |
| 7 | WebUI: kein Formular für `POST /api/time-entries` | Zeit lässt sich nicht nachtragen (vergessener Timer, Lernen am Papier). Korrigieren geht nur für Einträge, die heute begonnen haben (`TodayView.vue:30`). | Lücken im Zeittracking sind dauerhaft. Gestern lässt sich nicht mehr korrigieren, das untergräbt das Vertrauen in Stats und Review. | **hoch** |
| 8 | PRD §7 „Der Nutzer kann Tasks in den Tag ziehen“; `CalendarView.vue:46-47` | Ziehen geht nur für Tasks, die schon ein `planned_date` haben. Es gibt keine Backlog-Leiste, aus der man Tasks in Kalender oder Tagesplan zieht. Der Tagesplan auf „Start“ ist reine Anzeige. | Tagesplanung bedeutet: Datum und Uhrzeit pro Task in Formularfelder tippen. Sunsama und Akiflow leben genau von dieser Geste. | **hoch** |
| 9 | Ganze App | Kein Undo für Erledigen, Löschen, Verschieben oder Zeitkorrektur. Es gibt nur ein ConfirmPopup beim Löschen. | Fehler korrigieren heißt rekonstruieren. Bei Zeiteinträgen und Habits (Löschen kaskadiert alle Completions, `0005_habits.sql:20`) ist die Historie weg. | hoch |
| 10 | `core.ts:136-138` | Fehler aus der Extension zeigen nur `HTTP 409` statt der Backend-Meldung (`{"error": …}`). | Die Meldung „Kairo: HTTP 409“ sagt dem Nutzer nichts. | mittel |
| 11 | `TodayView.vue:283-288` | Blöcke im Tagesplan werden absolut positioniert, ohne Spalten bei Überlappung. Im Test verdeckt „WebSocket-Bug fixen“ (10:30) die Task „Musterprüfung A“ (10:15) vollständig. | Überplanung, die die App sichtbar machen will, verschwindet im Tagesplan buchstäblich. | mittel |
| 12 | `CalendarView.vue:42, 285` | Serien sind nicht verschiebbar, Änderungen und Löschen gelten immer für alle Termine. `recurrence_exdates` gibt es im Backend, aber nicht in der UI. | „Vorlesung fällt diese Woche aus“, der häufigste Fall im Uni-Alltag, ist nicht abbildbar. | mittel |
| 13 | `CalendarView.vue:170-173` | Ein Klick auf eine Task im Kalender springt nach `/tasks`, ohne die Task zu öffnen oder hinzuscrollen. | Der Kontext geht verloren und der Nutzer muss die Task in der Liste suchen. | mittel |
| 14 | `HabitsView.vue:95-109` | Tage vor `start_date` und bei `TIMES_PER_WEEK` jeder nicht erledigte Tag werden als „verpasst“ gezeichnet. Ein heute angelegtes Habit zeigt 27 graue Fehltage. | Unehrliche, demotivierende Darstellung. Ein 3×-pro-Woche-Habit sieht trotz erfülltem Ziel wie ein Misserfolg aus. | mittel |
| 15 | PRD §3 (`preferred_time` in der Zeitleiste), `HabitsView.vue:176-193` | Habits mit Uhrzeit erscheinen nicht im Tagesplan. Der Anlegedialog kennt weder Uhrzeit noch Ziel, Einheit, Start- oder Enddatum. Habits lassen sich nach dem Anlegen nicht bearbeiten (nur deaktivieren oder löschen). | Die Doku verspricht mehr, als die App kann, und ein Tippfehler im Namen heißt löschen und neu anlegen. | mittel |
| 16 | Backend `ProjectService.Delete`; `0007_time_entries.sql:13` | Löschen eines Projekts klappt, wenn nur Task-Zeiteinträge daran hängen (Test: 204). Diese Einträge verlieren dann still ihre `project_id`, und die Tasks werden projektlos. Die Rückfrage sagt dazu nur „„Kairo“ löschen?“. | Die Projektstatistik der Vergangenheit verschwiebt sich nach „Sonstiges“, ohne dass der Nutzer es merkt. | mittel |
| 17 | `ProjectsView.vue:139` | Der Projektordner ist ein freies Textfeld („Absoluter Pfad“), ohne Auswahl und ohne Prüfung. | Ein Tippfehler bricht die Workspace-Erkennung kommentarlos, und genau darauf baut die Extension auf. | mittel |
| 18 | `TodayView.vue` / Today-Service | „Frei“ berechnet sich aus Arbeitsfenster minus Termine minus geplante Schätzungen und ignoriert die bereits vergangene Tageszeit. Tasks ohne Schätzung zählen 0 Minuten. | Um 20 Uhr zeigt die App immer noch „3:30 h frei“. Überplanung bleibt unsichtbar, solange man keine Schätzungen pflegt. | mittel |
| 19 | `view.js:16-22`, `view.css` `.tabs` | Fünf Text-Tabs in einem etwa 300 px breiten Panel. „Dokumente“ stößt am Rand an. `role="tablist"` ohne Pfeiltasten-Navigation und ohne `aria-controls`. | Eng, schlecht per Tastatur bedienbar, und der Platz geht an Tabs mit wenig Wert. | mittel |
| 20 | `design.css` `--tx-disabled` (#4a4a46 auf #181716 ≈ 2,0:1; Light #a9a599 auf #efede7 ≈ 2,1:1) | Platzhalter, die Anweisungen tragen („Aufgabe für heute hinzufügen“), sind kaum lesbar. | Verstößt gegen WCAG AA, und das Quick-Add übersieht man leicht. | mittel |
| 21 | `CalendarView.vue:260-261`, `HabitsView.vue:169`, `ProjectsView.vue:129` | Dialoge haben keine Fokusfalle, kein `aria-modal` und keine Fokusrückgabe. `Esc` wirkt nur, wenn der Fokus im Overlay liegt. Der Kalenderdialog hat kein `role="dialog"`. | Tastatur- und Screenreader-Nutzer verlieren sich hinter dem Overlay. | mittel |
| 22 | `TaskActions.vue:10-12` | Die Buttons heißen nur „Start“ und „Fertig“. Die Tab-Reihenfolge auf „Start“ liest sich: Pause, Fertig, Start, Fertig, Start, Fertig. | Für Screenreader ist das unbrauchbar. | mittel |
| 23 | `view.js:298` | Offline zeigt die Extension die rohe Meldung „fetch failed“ plus „Erneut versuchen“, ohne Hinweis, wie man das Backend startet. | Neue Nutzer bleiben ratlos. | mittel |
| 24 | `App.vue:46` + `design.css` | Die Sidebar-Links sind unterstrichen (Standard-`<a>`-Stil, sichtbar in allen Screenshots). | Der Eindruck „nicht fertig“ entsteht direkt in der Hauptnavigation. | niedrig |
| 25 | `CalendarView.vue:246-255` | Die SVGs für Zurück, Weiter und Plus haben keine Klasse `ic` und damit keinen Strich. Die Pfeile sind kaum sichtbar, das Plus fehlt ganz. | Die Kalendernavigation ist schwer zu finden. | niedrig |
| 26 | `TasksView.vue:140` | Es fehlt ein Leerzeichen: „Heute10:15“. | Kosmetisch. | niedrig |
| 27 | `TodayView.vue:220` | Überfällige Tasks zeigen das rohe ISO-Datum „2026-10-06“. | Wirkt unfertig, und „gestern“ wäre schneller erfasst. | niedrig |
| 28 | `TasksView.vue:21-24` | Der Filter „Alle“ zeigt nur offene Tasks. | Die Benennung ist irreführend. | niedrig |
| 29 | `TasksView.vue:83` | Ein erledigtes Häkchen zurücknehmen setzt immer `PLANNED`, auch bei einer Task, die vorher im Backlog lag. | Der Status wird verfälscht. | niedrig |
| 30 | `view.js:126, 147` | „Tagesziel 63 %“ ist erfasste Zeit geteilt durch geplante Zeit, und darin steckt auch Zeit für ungeplante Tasks. | Das ist kein Ziel, und über 100 % deckelt die Anzeige still. Die Kennzahl suggeriert Fortschritt, wo keiner ist. | niedrig |
| 31 | `CalendarView.vue:21-27` und `lib/projectColor.ts` | Zwei Implementierungen der Projektfarbe, nur fünf Farben per Hash. | Bei sechs oder mehr Projekten kollidieren Farben zufällig, Farbe allein unterscheidet Projekte nicht verlässlich. | niedrig |
| 32 | Extension | Die Extension reagiert nicht auf `onDidChangeWorkspaceFolders`. | Nach dem Hinzufügen eines Ordners zum Workspace bleibt das erkannte Projekt bis zum nächsten Ereignis veraltet. | niedrig |

---

## 4. Bereich für Bereich

### 4.1 WebUI · Start (Today)

**Ist:** Begrüßung, ein großer Satz zum nächsten Termin, eine Auslastungsleiste, fünf Kennzahlen.
Links Aufgaben (überfällig / mit Uhrzeit / ohne Uhrzeit / aktiv), Quick-Add, Zeiterfassung mit
editierbaren Zeiten und Habits zum Abhaken. Rechts ein Tagesplan 07–22 Uhr.

**Probleme:**
- Die Hierarchie ist falsch herum: Die größte Schrift gehört dem nächsten *Termin*, nicht der Arbeit
  (#6). „Guten Morgen“ belegt die zweitbeste Fläche, ohne Information zu tragen.
- Der Tagesplan ist reine Anzeige. Man kann nichts anklicken, ziehen oder verlängern, und
  überlappende Blöcke verdecken sich (#11). Habits mit Uhrzeit fehlen (#15).
- „Frei 3:30 h“ ist zu jeder Uhrzeit gleich (#18). Die Kennzahl „Geplant“ ignoriert Tasks ohne
  Schätzung.
- Überfällige Tasks lassen sich nur auf heute oder morgen schieben, nicht ins Backlog oder auf ein
  freies Datum, und abbrechen schon gar nicht.
- Erledigte Tasks verschwinden in einer Zeile „1 erledigt“. Das Erfolgserlebnis fehlt und
  Zurücknehmen geht nicht.
- Zeiterfassung: gut, dass Start und Ende korrigierbar sind. Aber „Projektzeit“ ohne Projektnamen
  (`TodayView.vue:247`), kein „+ Zeit nachtragen“ (#7) und keine Summe pro Task.

**Ungenutztes Potenzial:** Das ist der Ort für den „Tagesstart“ à la Sunsama: einmal am Morgen
durchgehen (Gestern offen → heute? Backlog → heute? Kapazität passt?). Hier könnte auch der
Tagesabschluss stattfinden (siehe 5.).

**Vorschläge:**
1. Fokusbereich neu ordnen. Läuft ein Timer, steht oben groß Task, Projekt und Zeit mit Pause und
   Fertig. Läuft keiner, steht dort „Als Nächstes: <nächste geplante Task>“ mit „Starten“ und
   „In VSCodium öffnen“. Der nächste Termin kommt als zweite Zeile darunter.
2. Den Tagesplan interaktiv machen. Die Aufgabenliste links als Quelle zum Ziehen nutzen (FullCalendar
   `Draggable` gibt es schon als Abhängigkeit), Ziehen und Verlängern im Plan erlauben, Überlappungen
   in Spalten zeigen. Habits mit Uhrzeit als dünne Marker einzeichnen.
3. „Frei“ als **verbleibende** freie Zeit ab jetzt berechnen, Tasks ohne Schätzung mit einem
   Standardwert (30 Min, wie schon im Plan gezeichnet) mitzählen und das sichtbar machen
   („2 Tasks ohne Schätzung“).
4. Überfällige Tasks: „gestern“ / „vor 3 Tagen“ statt ISO-Datum, dazu ein Menü
   Heute / Morgen / Backlog / Abbrechen.
5. Zeiterfassung: Projektname anzeigen, Button „Zeit nachtragen“ und Summe pro Task.

### 4.2 WebUI · Kalender

**Ist:** FullCalendar mit Monat, Woche und Tag. Termine und geplante Tasks erscheinen gemeinsam,
Tasks gestrichelt und in Projektfarbe. Ein Dialog legt Termine oder Tasks an. Wöchentliche
Wiederholung an Wochentagen mit Enddatum. Einzeltermine und Tasks lassen sich ziehen und verlängern.

**Probleme:**
- Löschen ohne Rückfrage, auch bei Serien (#2).
- Keine Ausnahmen für Serien (#12) und keine Ganztagstermine (Prüfungstage, Ferien, Abgaben).
- Ein Klick auf eine Task führt weg in die Liste (#13).
- Es gibt keine Quelle für ungeplante Tasks (#8). Im Kalender plant man also nur um, man plant nicht.
- Die Navigations-Icons sind praktisch unsichtbar (#25).
- Der Dialog unterscheidet „Termin“ und „Task“, die Navigation sagt „Aufgaben“. Im Dialog fehlen
  Beschreibung, URL und Projektverknüpfung, obwohl die PRD (§1) sie für Termine vorsieht.
- Kalender und Tasks rechnen in der Zeitzone des Browsers, Today in der des Backends. Bei Reisen
  oder einem anders konfigurierten Backend zeigen die beiden Ansichten unterschiedliche Uhrzeiten.

**Ungenutztes Potenzial:** Für Studierende ist der Kalender der Stundenplan. Der wertvollste Schritt
ist nicht ein schönerer Kalender, sondern **Termine mit Projekten verknüpfen**. Dann kann die Vorlesung
„AlgoDat“ um 08:30 im Editor sagen: „Vorlesung in 10 Min, Skript Kapitel 5 öffnen?“

**Vorschläge:**
1. Löschen mit Rückfrage und bei Serien die Auswahl „Nur dieser Termin“ (exdate) oder „Ganze Serie“.
   Dieselbe Auswahl beim Verschieben einer Serieninstanz.
2. Eine Seitenleiste „Ungeplant“ mit offenen Tasks ohne Datum, als Quelle zum Ziehen (wie bei
   Akiflow und Sunsama).
3. Klick auf eine Task öffnet einen Seitendialog mit denselben Feldern wie in der Taskliste, ohne die
   Ansicht zu wechseln.
4. Im Termindialog ein Projektfeld. Termine mit Projekt bekommen dessen Farbe und tauchen in der
   Extension auf.
5. ICS-Import einmalig, für den Stundenplan aus dem Uni-Portal (siehe 5.).

### 4.3 WebUI · Aufgaben (Tasks)

**Ist:** Eine flache Liste mit Filterchips (Alle, Heute, Diese Woche, Überfällig, Prio hoch,
Erledigt), Projektfilter, Quick-Add und einer aufklappbaren Detailzeile, die jedes Feld beim
Verlassen speichert.

**Probleme:**
- Kein Status-Feld, also kein Abbrechen und kein Zurück ins Backlog (#1).
- Keine Gruppierung und keine Sortierung (nach Projekt, Datum, Priorität). Bei 80 Tasks ist die Liste
  eine Wand.
- Keine Suche. Subtasks (`parent_task_id`) gibt es im Backend, aber nicht in der UI. Keine
  Ressourcen pro Task (#5).
- Das Quick-Add der WebUI versteht nichts. Das Quick-Add der Extension versteht „30 min Sport“. Das
  ist inkonsistent, und gerade die WebUI, die zum Planen da ist, kann weniger.
- Speichern beim Verlassen eines Feldes ohne jede Rückmeldung. Ob es geklappt hat, sieht man nur an
  einer ausbleibenden Fehlermeldung oben.
- Neben dem Prioritätstext steht zusätzlich ein Prioritätspunkt. Das ist doppelt und frisst Breite.
  Tasks mit mittlerer Priorität (der Standard) tragen trotzdem „Mittel“ in jeder Zeile, das ist Rauschen.
- Tastatur: Es gibt keinen Weg, per Tastatur durch die Liste zu laufen, eine Task abzuhaken oder zu
  öffnen (Things: Pfeiltasten, Leertaste, Cmd+K).

**Vorschläge:**
1. Status-Menü in der Detailzeile (Backlog / Geplant / Abgebrochen) und Filter „Abgebrochen“.
   „Alle“ in „Offen“ umbenennen.
2. Quick-Add mit einfacher Syntax, die Extension und WebUI teilen: `heute`, `morgen`, `mo`,
   `14:00`, `30m`, `#AlgoDat`, `!hoch`. Das ist Todoist-Grammatik light. Die Logik einmal in einer
   Datei schreiben und an beiden Stellen nutzen.
3. Gruppierung nach Projekt oder Datum umschaltbar. „Mittel“ nicht anzeigen, nur Abweichungen.
4. Ein Abschnitt „Ressourcen“ in der Detailzeile (Datei, Ordner, URL), siehe 4.5.
5. Tastenkürzel: `n` für eine neue Task, `j`/`k` zum Navigieren, `x` zum Abhaken, `Enter` zum Öffnen,
   `s` zum Starten, `t` für „auf heute“.
6. Nach dem Speichern eine dezente Bestätigung, zum Beispiel ein kurzer Haken am Feld.

### 4.4 WebUI · Gewohnheiten (Habits)

**Ist:** Chips „Heute abhaken“, pro Habit ein Raster der letzten 28 Tage, die aktuelle Serie,
„Letzte 7 Tage x/7“, Deaktivieren und Löschen. Ein Dialog zum Anlegen mit vier Rhythmen.

**Probleme:**
- Das Raster lügt bei neuen Habits und bei X-mal-pro-Woche (#14).
- „Letzte 7 Tage 0/7“ ist bei 3× pro Woche die falsche Bezugsgröße. Gemeint ist „0/3 diese Woche“,
  und das Backend liefert `week_progress` sogar schon.
- Nicht bearbeitbar. Uhrzeit, Ziel und Einheit fehlen (#15).
- Ein vergessener Tag lässt sich nicht nachtragen. Gestern abhaken geht in der UI nicht, obwohl die
  API ein Datum nimmt. Das ist der häufigste Habit-Fehlerfall überhaupt.
- „Wöchentlich (MO)“ zeigt rohe Kürzel, „SPECIFIC_WEEKDAYS“ eine Liste „MO, WE, FR“.

**Ungenutztes Potenzial und Frage nach dem Sinn:** Habits sind der am wenigsten mit dem
Alleinstellungsmerkmal verbundene Bereich. Habits wie „Jeden Tag 30 min AlgoDat“ oder „3× pro Woche an
Kairo“ könnten **aus Zeiteinträgen automatisch erfüllt** werden. Das wäre einzigartig: Das Habit hakt
sich ab, weil der Timer in VSCodium lief. Ohne diese Verbindung ist der Bereich ein schwächerer
Streaks-Klon.

**Vorschläge:**
1. Tage vor `start_date` als „neutral“ zeichnen, bei X-pro-Woche Wochen statt Tage bewerten
   (eine Kachel pro Woche mit x/3).
2. Klick auf eine Kachel im Raster schaltet die Erledigung für diesen Tag um (Nachtragen).
3. Bearbeiten-Dialog mit Uhrzeit, Ziel und Einheit, deutsche Wochentagsnamen.
4. Optional ein Habit mit Projekt plus Mindestminuten verknüpfen, das sich aus Zeiteinträgen
   automatisch erfüllt (passt zur Vision und ist kein Gamification-Kram).

### 4.5 WebUI · Projekte

**Ist:** Karten mit Farbpunkt, Prozent erledigter Tasks, Beschreibung, Status, Pfad und
Ressourcen-Chips. Ein Dialog zum Bearbeiten inklusive Ressourcen.

**Probleme:**
- Eine Projektkarte zeigt weder die Tasks noch die Zeit des Projekts, obwohl `/api/review` die Zeit
  liefert. Den Weg „Projekt → seine Tasks“ gibt es nicht. Man muss in die Taskliste wechseln und dort
  filtern.
- Datei- und Ordner-Ressourcen sind im Browser nicht anklickbar (`ProjectsView.vue:91-92`). Das ist
  technisch verständlich, aber ein Button „In VSCodium öffnen“ (über eine `vscodium://`-URI an die
  Extension) würde genau die Brücke bauen.
- Ressourcen lassen sich nur löschen und neu anlegen. Das Ressourcen-Formular verlangt vom Nutzer, die
  Typen URL, FILE und FOLDER zu kennen, statt sie aus der Eingabe abzuleiten.
- Der Prozentwert zählt alle Tasks seit Projektbeginn. Ein Projekt wie „Kairo“ mit 200 erledigten und
  10 offenen Tasks steht ewig bei 95 % und sagt nichts aus.
- Löschen erklärt nicht, was mit Tasks und Zeit passiert (#16).
- Freitext-Pfad (#17).

**Vorschläge:**
1. Die Projektkarte aufklappbar machen: offene Tasks (startbar), Zeit diese Woche und gesamt,
   nächster Termin, Ressourcen mit „In VSCodium öffnen“.
2. Den Ressourcen-Typ automatisch erkennen (`http…` ist eine URL, endet auf `/` oder hat keine
   Endung, ist es ein Ordner).
3. Statt Prozent: „3 offen · 4 h 20 diese Woche · zuletzt aktiv gestern“.
4. Die Löschen-Rückfrage nennt die Folgen („12 Tasks werden projektlos, 34 h Zeit verlieren die
   Zuordnung“) und bietet „Archivieren“ als Hauptaktion an.

### 4.6 WebUI · Review (nicht vorhanden)

**Ist:** `GET /api/review` existiert (Phase 7). Die WebUI hat keine Review-Ansicht. Habits und
Projekte zweckentfremden den Endpunkt mit `from=to=heute`, nur um Serien und Fortschritt zu holen
(`HabitsView.vue:38`, `ProjectsView.vue:33`). In der Extension gibt es einen eigenen Stats-Tab, der
die Auswertung im Client noch einmal selbst rechnet.

**Problem:** Rückblick ist eine der vier Säulen aus den Agent-Guidelines, aber er findet am falschen
Ort statt (im Editor-Seitenpanel) und mit Doppellogik.

**Vorschläge:**
1. Eine WebUI-Ansicht „Woche“ als Review: geplant vs. erfasst pro Tag, Zeit pro Projekt, erledigte
   Tasks, was überfällig wurde, Habit-Wochen. Dazu ein Feld „Notiz zur Woche“ (das ist kein
   Notizsystem, nur ein Satz).
2. Der Stats-Tab der Extension wird dafür auf eine Zeile reduziert (siehe 4.10).

### 4.7 Extension · Tab „Jetzt“

**Ist:** Oben die Karte des erkannten Projekts mit Ressourcen und offenen Tasks. Darunter der Timer
mit Pause und Fertig, drei Kennzahlen und „Tagesziel“.

**Bewertung:** Das ist **der beste Bildschirm von Kairo**. Er zeigt Projektkontext, Dateien zum Öffnen
und startbare Tasks an dem Ort, an dem gearbeitet wird. Genau so steht es in der Vision.

**Probleme:**
- Im Timer-Block ist die Zeit groß und die Task klein und grau (`timer-task`). Wichtiger ist, *woran*
  man arbeitet.
- Die Projektkarte steht vor dem Timer. Läuft ein Timer für ein anderes Projekt als den erkannten
  Workspace, gibt es keinen Hinweis („Timer läuft für Kairo, du bist in AlgoDat – wechseln?“). Das ist
  der typische Kontextwechsel-Fehler.
- „Tagesziel“ ist kein Ziel (#30).
- Es fehlen der nächste Termin („Vorlesung in 25 Min“) und eine Notiz oder Beschreibung zur aktuellen
  Task. Beides gehört genau hierher.
- Ist kein Projekt erkannt, gibt es nur den Hinweis „Starte eine Task unter Heute“. Das ist die
  Gelegenheit für „Diesen Ordner als Projekt anlegen / mit Projekt verknüpfen“.

**Vorschläge:**
1. Timer-Block: Tasktitel groß, Projekt und Zeit darunter. Bei Abweichung vom Workspace ein
   Banner „Timer läuft für ‚X‘“ mit [Wechseln zu ‚Y‘] [Ignorieren].
2. Eine Zeile „Als Nächstes: 12:30 Mittagessen · 14:00 Login reparieren“.
3. Keine Projektzuordnung, dann [Diesen Workspace mit Projekt verknüpfen ▾]. Das ist eine Aktion
   statt Tippen eines Pfades in der WebUI und der wichtigste Onboarding-Schritt.
4. „Tagesziel“ durch „Erfasst 1:35 von 2:30 geplant“ ersetzen.

### 4.8 Extension · Tab „Heute“

**Ist:** Alle für heute geplanten Tasks (offene oben, erledigte durchgestrichen), Abhaken, Start-Button
und Quick-Add mit Minuten-Syntax.

**Probleme:**
- Er doppelt sich weitgehend mit der Projektkarte in „Jetzt“. Bei einem Projekt-Workspace stehen
  dieselben Tasks zweimal.
- Überfällige Tasks (`today.overdue`) fehlen. In der WebUI stehen sie ganz oben, im Editor gibt es
  sie nicht.
- Keine Termine, also keine Zeitleiste.
- Das Quick-Add legt die Task im erkannten Projekt an. Das ist sinnvoll, aber unsichtbar (kein
  Hinweis „in AlgoDat“).
- Ein Start-Button für eine Task aus einem anderen Projekt startet nur den Timer und öffnet nicht
  deren Workspace (#4).

**Vorschläge:**
1. „Heute“ als kompakte Agenda: Termine und Tasks in zeitlicher Reihenfolge, Überfällige oben,
   Tasks des aktuellen Projekts hervorgehoben.
2. Den Start einer Task aus einem anderen Projekt mit der Frage „Workspace ‚Kairo‘ öffnen?“
   verbinden (dieses Fenster / neues Fenster / nein).
3. Im Quick-Add-Label das Zielprojekt nennen: „Neue Aufgabe für heute · AlgoDat“.
4. Dann „Jetzt“ und „Heute“ zusammenlegen (siehe 6.).

### 4.9 Extension · Tab „Projekte“

**Ist:** Gruppen Aktiv, Pausiert und Archiv. Pro Projekt ein Farbpunkt, x/y Tasks, ein
Fortschrittsbalken und ein Button „In neuem Fenster öffnen“.

**Probleme:** Reine Anzeige. Ein Klick auf ein Projekt tut nichts, man sieht weder dessen Tasks noch
dessen Ressourcen. Die Farbpunkte haben **andere Farben als in der WebUI** (`view.css` färbt nach
Status, die WebUI nach ID-Hash). Der Fortschritt in x/y ist wie in der WebUI langfristig sinnlos.

**Ungenutztes Potenzial:** Das ist der eingebaute **Projektwechsler** im Editor, VS Codes „Recent
Projects“ mit Kontext.

**Vorschläge:**
1. Klick auf ein Projekt klappt Ressourcen und offene Tasks auf (dieselbe Karte wie in „Jetzt“).
2. Prominente Hauptaktion „Öffnen“ (dieses Fenster / neues Fenster) plus Befehl
   `Kairo: Projekt öffnen…` als QuickPick, sortiert nach „zuletzt gearbeitet“ (aus den Zeiteinträgen).
3. Statt x/y: „3 offen · heute 0:45 · zuletzt gestern“.
4. Projektfarben aus derselben Quelle wie die WebUI.

### 4.10 Extension · Tab „Stats“

**Ist:** Balken für die Woche, Projektverteilung (Top 4 plus Sonstiges) und eine Heatmap über
12 Wochen. Alles im Client aus 84 Tagen Zeiteinträgen berechnet.

**Probleme:**
- Doppellogik zu `/api/review`. Zwei Auswertungen, die bei Grenzfällen (Mitternacht, Zeitzone,
  laufender Timer) auseinanderlaufen werden.
- Die Heatmap hat keine Legende, keine Wochen- und Monatsbeschriftung und Werte nur im
  Tooltip. Sie ist hübsch, aber bei der Arbeit nicht verwertbar.
- Ein Rückblick gehört nicht in das Seitenpanel, in dem man arbeitet. Er gehört zum Planen (WebUI).
  WakaTime zeigt im Editor auch nur eine Statusleisten-Zahl und das Dashboard im Browser.
- Farbpalette fest im Code (`#b180d7`), nicht aus dem Theme.

**Vorschläge:** Den Tab streichen. Stattdessen in „Jetzt“ eine Zeile „Diese Woche 12:40 · AlgoDat 5:10“
mit Link auf die Review-Ansicht der WebUI (4.6).

### 4.11 Extension · Tab „Dokumente“

**Ist:** Ein Dateibaum von `~/Documents` (einstellbar), mit Aufklappen, Öffnen im Editor oder in der
Standard-App und „Im Finder zeigen“.

**Bewertung:** Wie in deinem Beispiel: In dieser Form ist der Tab **überflüssig**. Er dupliziert den
Explorer von VSCodium (der beliebige Ordner per „Add Folder to Workspace“ zeigen kann) und den Finder.
Er weiß nichts von Projekten, Tasks oder Terminen. Dazu kommt: Er steht weder in der Doku noch in der
README, die Einstellung `kairo.documentsFolder` ist das einzige Indiz.

**Was er sein könnte (absteigend nach Wert):**
1. **Dokumente zum aktuellen Kontext:** Projekt-Ressourcen plus Ressourcen der laufenden Task plus
   Dateien, die zuletzt während Zeiteinträgen dieses Projekts offen waren. Ein Klick öffnet sie. Das
   ist die eigentliche Antwort auf „Wo muss ich dafür arbeiten?“.
2. **An Task anheften:** Rechtsklick im Explorer oder im Editor-Tab → „An aktuelle Task anheften“ bzw.
   „An Projekt anheften“. Daraus entsteht eine Ressource, ohne jemals einen Pfad zu tippen. Das löst
   auch #5.
3. **Zuletzt geöffnet pro Projekt:** Die Extension merkt sich lokal (nicht als Produktdaten, oder als
   Ressourcen mit Flag) die letzten zehn Dateien je Projekt. Morgens im AlgoDat-Workspace stehen dann
   das Skript und die letzte Übung oben.
4. **Uni-Material nach Fach:** Projekte mit `local_path` unter `~/Documents/Uni/…` als Fächer, darunter
   die PDFs nach Änderungsdatum. Das ist strukturierter als der rohe Baum.
5. **PDFs mit Lernfortschritt:** Pro PDF-Ressource „Seite 34/120“ oder „Kapitel 5 erledigt“ als
   Checkliste. Das ist Scope-nah, weil es Arbeit an Uni-Material sichtbar macht. Aber Vorsicht, es
   grenzt an ein Notizsystem. Nur als einfache Zahl oder Haken umsetzen.
6. **Verknüpfung mit Terminen:** Vorlesung „AlgoDat“ in 10 Minuten → Skript des Projekts oben
   anzeigen.
7. **Suche** über alle Ressourcen aller Projekte (QuickPick `Kairo: Ressource öffnen…`). Das ist
   schneller als jeder Baum.

**Empfehlung:** Den Baum entfernen und den Tab in „Kontext“ oder „Dateien“ umbauen mit (1), (2) und
(7). Zusammen ist das ein Tag Arbeit und ersetzt einen Tab ohne Wert durch einen mit Alleinstellung.

### 4.12 Extension übergreifend

- **Statusleiste (fehlt, #3):** `$(clock) 0:32 · Musterprüfung A` als Statusleisten-Eintrag. Ein
  Klick öffnet einen QuickPick mit Pause, Fertig, Task wechseln und „Kairo öffnen“. Ohne Timer:
  `$(play) Kairo` → QuickPick „Task starten…“. Das ist der einzige Ort, der im Editor *immer* sichtbar
  ist (vgl. WakaTime, Toggl Track for VS Code).
- **Befehle (fehlen):** `Kairo: Task starten…`, `Pausieren`, `Abschließen`, `Neue Task…`
  (mit Quick-Add-Syntax), `Projekt öffnen…`, `Ressource öffnen…`, `Datei an Task anheften`. Alle
  belegbar mit Tastenkürzeln.
- **URI-Handler (fehlt):** `vscodium://kairo-local.kairo/start?task=…` erlaubt der WebUI ein „In
  VSCodium starten“. Das ist die eigentliche Brücke Planen → Arbeiten.
- **Inaktivitäts-Rückfrage:** gut gedacht (rückdatiert, pausiert nie selbst). Zwei Risiken nach
  Code-Lektüre, nicht im echten Editor geprüft:
  - Wer in einer PDF-App oder auf Papier lernt, also das Kernszenario Uni, bekommt alle 10 Minuten
    eine Rückfrage.
  - Bei mehreren VSCodium-Fenstern fragt vermutlich jedes Fenster einzeln, denn jede Instanz hat ihren
    eigenen `watchIdle`.

  Vorschlag: Bei Tasks ohne Code-Bezug, etwa wenn das Projekt kein Repository ist, die Rückfrage
  abschalten oder „für diese Task nicht mehr fragen“ anbieten.
- **Fehler und Offline:** Backend-Meldungen durchreichen (#10). Offline den Hinweis
  „Backend starten: `kairo` im Terminal oder `kairo install` für Autostart“ zeigen (#23).
- **Rendering:** Jede Aktualisierung baut per `innerHTML` alle fünf Tabs neu auf, auch die
  unsichtbaren (`view.js:301-305`). Bei aufgeklappten Ordnern und Hover-Zuständen flackert das
  vermutlich. Das ist nicht kritisch, aber ein weiteres Argument für weniger Tabs.

---

## 5. Was man noch machen könnte (nach Wirkung sortiert)

| Idee | Wirkung | Passt zur Vision? |
|---|---|---|
| **„In VSCodium starten“** aus WebUI und Extension: Workspace öffnen, Task-Ressourcen öffnen, Timer starten, in einem Schritt (URI-Handler). | sehr hoch | **Ja, das ist der Kern.** |
| **Statusleiste und Befehlspalette** in der Extension. | sehr hoch | Ja |
| **Workspace ↔ Projekt verknüpfen** direkt aus VSCodium; außerdem beim Öffnen eines unbekannten Repos unter `~/code` fragen „Als Projekt anlegen?“. | hoch | Ja (Kontext liefern statt pflegen) |
| **Datei an Task oder Projekt anheften** (Kontextmenü im Explorer und im Editor-Tab). | hoch | Ja |
| **Tagesstart und Tagesabschluss** als geführter 2-Minuten-Ablauf in der WebUI: gestern offen → heute/Backlog, Kapazität prüfen. Abends: erfasst vs. geplant, Offenes verschieben, Zeitlücken nachtragen. | hoch | Ja (Planung + Rückblick) |
| **Backlog-Seitenleiste** zum Ziehen in Kalender und Tagesplan. | hoch | Ja (steht in der PRD) |
| **ICS-Import** (einmalig oder Abo nur lesend) für den Uni-Stundenplan. | hoch für die Zielgruppe | Ja, Phase 8 „Import“. Kein Zwei-Wege-Sync, das wäre Scope Creep. |
| **Termin ↔ Projekt** verknüpfen, dann „Vorlesung in 10 Min, Skript öffnen?“ im Editor. | mittel | Ja |
| **Git-Bezug:** Branch oder Commit-Message mit Task verknüpfen (`kairo/<id>`); beim Wechsel des Branches Task-Wechsel vorschlagen (nie automatisch starten). | mittel | Ja, mit Vorsicht (nur Vorschlag, gemäß „Workspace geöffnet ≠ gearbeitet“) |
| **Habit aus Zeiteinträgen erfüllt** (z. B. „30 min AlgoDat täglich“). | mittel | Ja, verbindet Habits mit echter Arbeit |
| **Export** (JSON und ICS) plus Restore-Befehl `kairo restore <backup>`. | mittel (Vertrauen) | Ja (Local-first, Daten gehören dem Nutzer) |
| **Raycast- oder Alfred-Skript** „Task starten“, „Heute“. Die API gibt es schon. | mittel | Grenzfall, aber billig |
| **Fokusmodus/Pomodoro** im Editor. | niedrig | Scope Creep: es gibt gute Tools, und es verwässert „echte Zeitstempel“ nicht, bringt aber nichts zur Brücke. |
| **KI-Tagesplanung** aus Freitext („Heute AlgoDat, Testing, Activitytracker“). | potenziell hoch | Langfristziel laut Handoff. Jetzt **Scope Creep**, solange die manuelle Planung keine Drag-Geste hat. |
| **Mobile Ansicht** (nur lesend: heute, Habits abhaken). | mittel | Nicht-MVP laut Doku. Über eine responsive WebUI im lokalen Netz wäre es fast gratis, verletzt aber das Sicherheitsmodell (nur localhost). Bewusst entscheiden. |
| Notizen pro Task/Projekt (Markdown). | mittel | **Scope Creep.** Lieber eine Markdown-Datei im Projektordner als Ressource anheften. |

---

## 6. Was man streichen oder vereinfachen sollte

1. **Den Extension-Tab „Stats“ streichen.** Eine Zeile Wochensumme in „Jetzt“ reicht. Der Rückblick
   kommt als Ansicht in die WebUI, auf Basis von `/api/review`. Das spart die Doppellogik in
   `view.js:192-254`.
2. **„Dokumente“ als Dateibaum streichen** und durch kontextbezogene Dateien ersetzen (4.11).
3. **„Jetzt“ und „Heute“ in der Extension zusammenlegen:** Timer oben, darunter die Agenda des Tages
   mit hervorgehobenem Projekt, darunter die Ressourcen. Dann bleiben die Tabs „Jetzt“, „Projekte“
   und „Dateien“, und sie passen in 300 px.
4. **Kennzahlen auf „Start“ reduzieren:** Fünf Zahlen in gleicher Größe sind zu viele. Behalten:
   „Frei ab jetzt“ und „Erfasst/Geplant“. „Offen heute“ steht schon an der Liste.
5. **Die Begrüßung „Guten Morgen“** hat keinen Informationswert und kostet die beste Fläche. Weg oder
   klein.
6. **Ein Styling-System:** `styles.css` (`--k-*`, steuert sich über `prefers-color-scheme`),
   `design.css` (`--bg-*`, steuert sich über `data-theme`) und das PrimeVue-Preset in `theme.ts`
   definieren dieselben Farben dreifach, teils widersprüchlich. PrimeVue wird nur für `ConfirmPopup`
   und `Message` genutzt. Beides lässt sich mit etwa 30 Zeilen eigenem Code ersetzen (es gibt schon
   eigene Dialoge). Das spart eine große Abhängigkeit und eine Fehlerquelle bei Dark und Light.
7. **Projektfarbe einmal definieren** (`lib/projectColor.ts`) und in Kalender, Extension und
   Projektkarte benutzen. Besser noch: Die Farbe als Feld am Projekt speichern und wählbar machen,
   statt sie per Hash zu würfeln.
8. **Prioritäten:** Vier Stufen plus Punkt plus Text pro Zeile ist viel für eine Ein-Personen-App.
   Nur „Hoch“ und „Dringend“ anzeigen (wie die Extension es schon tut), oder auf drei Stufen
   reduzieren.
9. **Task-Status vereinfachen (für die UI):** `PLANNED` vs. `BACKLOG` ergibt sich faktisch aus
   `planned_date`. In der UI genügt „offen / läuft / pausiert / erledigt / abgebrochen“. Den Status
   intern behalten, aber nicht separat pflegen lassen.

---

## 7. Priorisierte Empfehlung (Top 10, Quick Wins zuerst)

| # | Maßnahme | Aufwand | Wirkung |
|---|---|:---:|:---:|
| 1 | **Abbrechen / Status in der Taskliste** plus Filter „Abgebrochen“, „Alle“ → „Offen“ (behebt #1, #28). | S | M |
| 2 | **Löschen absichern:** Rückfrage beim Termin, Auswahl „Nur dieser / ganze Serie“ (Backend kann exdates), dazu ein **Undo-Toast** für Erledigen, Löschen und Verschieben (behebt #2, #9, #12). | S–M | L |
| 3 | **Extension-Fehler und Offline verständlich:** Backend-Meldung anzeigen, Startanleitung im Offline-Zustand (behebt #10, #23). | S | M |
| 4 | **„Zeit nachtragen“ und Korrektur für beliebige Tage** (Formular auf „Start“ plus Datumswahl in der Zeiterfassung) (behebt #7). | S | M |
| 5 | **„Diesen Workspace mit Projekt verknüpfen“** in der Extension, plus Ordnerauswahl statt Freitext (behebt #17, löst Onboarding). | S | L |
| 6 | **Statusleisten-Timer und Befehle** (Starten…, Pause, Fertig, Neue Task…, Projekt öffnen…, Ressource öffnen…) (behebt #3). | M | L |
| 7 | **„Start“ neu fokussieren:** Hero zeigt laufende oder nächste Task mit Starten-Button. „Frei ab jetzt“ berechnen, Überlappungen in Spalten, sichtbare Fehler beheben (#6, #11, #18, #24–27). | M | L |
| 8 | **Task-Ressourcen in WebUI und Extension** plus „Datei an Task anheften“ im Kontextmenü (behebt #5). Damit wird „Dokumente“ zum Kontext-Tab (4.11). | M | L |
| 9 | **„In VSCodium starten“:** URI-Handler in der Extension plus Button in der WebUI. Beim Start Workspace und Ressourcen öffnen (behebt #4). | M | L |
| 10 | **Backlog → Tag ziehen** im Kalender und im Tagesplan von „Start“ (behebt #8). | M | L |

Danach: Stats- und Dokumente-Baum streichen (6.1, 6.2), Habit-Raster ehrlich machen (#14), eine
Review-Ansicht in der WebUI (4.6), ICS-Import, ein `.vsix`-Paket und ein Release-Binary für das
Onboarding.

---

## 8. Offene Fragen an dich

1. **Wo lebt dein Stundenplan wirklich?** Wenn Uni-Termine weiter in Apple oder Google Calendar
   stehen, ist ein (nur lesender) ICS-Import Pflicht. Sonst pflegst du doppelt. Willst du Kairo als
   *einzigen* Kalender, oder als Planungsschicht über einem bestehenden?
2. **Sind Habits Kernbestandteil?** Sie sind der am wenigsten mit der Brücke verbundene Bereich. Soll
   ich sie als Nebenfunktion behandeln (minimal pflegen) oder mit Zeiteinträgen verbinden (4.4)?
3. **Wie soll Lernzeit außerhalb des Editors erfasst werden?** PDF lesen, Papier, Vorlesung. Bleibt der
   Timer dann einfach laufen (und die Inaktivitäts-Rückfrage stört), oder soll es pro Task oder
   Projekt einen Modus „nicht nach Inaktivität fragen“ geben?
4. **Darf die WebUI VSCodium steuern?** Ein Klick auf „Starten“ in der WebUI, der VSCodium öffnet und
   den Workspace wechselt, ist die stärkste Umsetzung der Vision. Er ist aber auch übergriffig. Willst
   du das als Standard, als Option oder gar nicht?
5. **Wer soll Kairo installieren können?** Nur du (dann sind Go, Node und F5 okay) oder auch
   Kommilitonen (dann braucht es ein Release-Binary, eine `.vsix` und einen Ersteinrichtungsablauf)?
   Davon hängt ab, wie viel Onboarding-Arbeit sinnvoll ist.
6. **Soll der Dokumente-Bereich global (Uni-Material nach Fach) oder strikt pro Projekt/Task sein?**
   Mein Vorschlag ist pro Kontext plus Suche, aber für reine Uni-Nutzung kann ein Fach-Browser
   wertvoller sein.
7. **Löschen oder Papierkorb?** Willst du Soft-Delete (Papierkorb, 30 Tage) für Tasks, Termine und
   Habits, oder genügt ein Undo-Toast direkt nach der Aktion?
8. **Wo gehört der Rückblick hin?** In die WebUI (mein Vorschlag) oder bewusst in den Editor, weil du
   ihn dort öfter siehst?
9. **Subtasks:** Das Backend kann sie, die UI nicht. Brauchst du sie (z. B. Kapitel einer Prüfung),
   oder soll das Feld weg?
10. **Mobile:** Willst du morgens auf dem Handy „Heute“ sehen oder Habits abhaken? Das würde das
    Sicherheitsmodell (nur localhost) berühren und gehört dann bewusst in die Roadmap oder bewusst
    auf die Nicht-Liste.
