# 06 – Design-Review: Kairo Redesign v3

Stand: 7. Oktober 2026. Grundlage sind Screenshots des Mockups „Kairo Redesign v3“ (Start, Aufgaben, Kalender, Gewohnheiten, Projekte, Woche, Papierkorb, Dialoge, Extension) bei 68–70 % Zoom. Bewertet wurden statische Zustände, nicht die laufende Vue-App.

**Bewertung:** Design-Reife 8/10, Testbereitschaft 6/10. Als Grundlage zum Testen reicht der Stand, als Abnahme nicht.

Legende: 🔴 vor dem Testen klären · 🟡 mittelfristig · 🟢 Feinschliff

---

## Priorität 1: vor dem Testen

### 🔴 Fehlende Zustände
- Leerzustände (keine Aufgaben, keine Termine, kein Projekt, leerer Papierkorb)
- Ladezustand und Fehlerzustand
- Offline und Sync: aktuell nur „Verbunden (dev)“, obwohl „Sync-Konflikte lösen“ im Produkt selbst eine Aufgabe ist
- Aufgabe öffnen/bearbeiten (Detailansicht)
- Formulare: „Neuer Eintrag“ (Kalender), „Neues Projekt“, „Neue Gewohnheit“
- Kalender: Monats- und Tagesansicht
- Ohne diese Zustände testet man nur den Happy Path.

### 🔴 Kalender Woche: Titel nicht lesbar
- Titel werden auf etwa 8 Zeichen gekürzt („Vorlesung S…“ und „Vorlesung D…“ sind nicht unterscheidbar).
- Die Spalte „Ungeplant“ nimmt Breite weg.
- Vorschlag: „Ungeplant“ einklappbar machen, bei Hover/Fokus ein Popover mit vollem Titel zeigen, Mindestspaltenbreite setzen.

### 🔴 Testdaten angleichen
- activitytracker hat „3 offen“ (Projekte, Extension), in der Aufgabenliste steht aber nur eine Aufgabe.
- Die überfällige „Sync-Konflikte im Offline-Modus lösen“ (Mo 5.10.) fehlt im Kalender-Ganztag; „Lebenslauf“ (gestern) wird dort gezeigt.
- Seed-Daten prüfen, ein Tester meldet das sonst als Bug.

---

## Priorität 2: Usability

### 🟡 „Woche“ hat drei Bedeutungen
- Sidebar-Eintrag (eigentlich der Rückblick), Umschalter im Kalender und Link „Woche ›“ im Tagesplan.
- Die Extension nennt die Seite „Rückblick“.
- Vorschlag: Sidebar-Eintrag in „Rückblick“ umbenennen.

### 🟡 Aktionen nur bei Hover
- Betrifft Aufgaben, Projekte, Gewohnheiten, Papierkorb.
- Per Tastatur gelöst (`:focus-within`), per Touch nicht.
- Entscheiden: „nur Desktop“ dokumentieren oder auf Touch dauerhaft anzeigen.

### 🟡 Dialog „Termin löschen?“
- Der Button heißt immer „Termin löschen“, auch wenn „Ganze Serie“ gewählt ist.
- Label an die Auswahl koppeln („Ganze Serie löschen“), passend zur Regel „das Wort nennt die Folge“.

### 🟡 Gewohnheiten
- „Serie 3 Tage“ ist bei Sport (Mo/Mi/Fr) mehrdeutig (Kalendertage oder Einheiten). Vorschlag: „3 in Folge“ oder eigene Formulierung für Wochentags-Gewohnheiten.
- „7 Tage 6/7“ und „Serie“ wirken doppelt.
- „Nicht geplant“ (gestrichelt) ist kaum von „Offen“ zu unterscheiden.

### 🟢 Start
- „frei 5:06“ ist ohne Kontext unklar: Tooltip ergänzen.
- Der laufende Task steht viermal auf der Seite (Timer-Karte, Heute-Liste, Tagesplan, Zeiterfassung). Wahrscheinlich Absicht, im Test prüfen, ob es stört.

### 🟢 Papierkorb und Rückblick
- Papierkorb: „Leeren“ und Hinweis zur Aufbewahrungsfrist fehlen.
- Rückblick: „Erledigt 14“ zeigt nur 4 Einträge, „Alle anzeigen“ fehlt.

---

## Priorität 3: Visuelles und Konsistenz

### 🟡 Aufgabenliste
- Badges „Dringend/Hoch/Niedrig“ stehen direkt hinter dem Titel und wandern mit dessen Länge. Besser in feste Spalte.
- Es gibt keine Spaltenköpfe, 2/5, 1:30 und 0:45 sind nur erratbar. Kopfzeile oder Tooltips ergänzen.
- Datumsformat ist uneinheitlich: „Fr 9.10.“, „Sa 10.10.“, „fällig 12.10.“, „Heute 14:30“. Ein Schema festlegen.
- Der Tooltip „Fällig Mo 5.10.“ verdeckt das Datum der Zeile darüber. Unterhalb oder seitlich platzieren.

### 🟡 Projekte
- Beschreibungen werden mitten im Satz gekürzt, obwohl rechts viel Platz frei ist. Spaltenbreiten anpassen.
- „dotfiles“ hat weder Bruch noch Balken, ein „–“ würde den Rhythmus halten.

### 🟢 Sonstiges
- Gewohnheit „lernen“ ist klein geschrieben, „Sport“ und „Lesen“ groß. Angleichen.
- Dark Mode: reines Schwarz, Sidebar und Inhalt kaum getrennt. Ein leichter Grauwert (z. B. #0c0c0d) gibt Tiefe.
- Der Pause-Button ist in der Extension blau, im Web weiß. Wirkt gewollt (VSCodium-Theme), im Spec festhalten.

---

## Barrierefreiheit prüfen

- **Kontrast messen** (Lighthouse/axe, nicht schätzen): Uhrzeiten in Kalenderblöcken, „Als Nächstes“, durchgestrichene Termine, gestrichelte „Nicht geplant“-Felder. Sie wirken unter 4,5:1 bzw. 3:1.
- **Schriftgröße:** ca. 10–12 px. Bei 100 % Zoom prüfen, die Screenshots waren bei 68–70 %.
- **Touch-Ziele:** Habit-Quadrate (ca. 10–14 px) und Zeilen-Icons liegen unter den 24×24 px aus WCAG 2.2 (2.5.8). Für Desktop vertretbar, bewusst entscheiden.
- Bereits gut gelöst: nicht nur Farbe als Träger (schraffierte Balken, Icons, Textlabels), sichtbare Fokusringe, Fokusfalle im Dialog.

---

## Testplan für die echte App

- [ ] Komplett mit der Tastatur bedienen
- [ ] Bei 100 % Zoom und bei 1280 px Breite prüfen
- [ ] Sehr lange Titel, 0 Einträge und über 100 Einträge
- [ ] Dark und Light vergleichen
- [ ] Kontrast messen
- [ ] Prüfen, ob die Vue-App dem Mockup entspricht (Hover-Aktionen, Fokusreihenfolge, Toast mit Rückgängig, Dialog-Fokus)

---

## Was beibehalten werden soll

- Farbdisziplin: Orange = „jetzt“, Rot = „überfällig“, sonst Graustufen. Jede Seite nennt ihre „Farbtöne“ im Spec.
- Rückgängig-Toast statt „Sicher?“-Dialog, echte Bestätigung nur beim endgültigen Löschen.
- Tastenkürzel-Leiste, Schnelleingabe mit `n`, Triage-Aktionen bei Überfälligem („→ Heute / → Morgen“).
- Extension-Zustände (Timer läuft, kein Timer, Ordner ohne Projekt) als echte Zustände gezeigt.
- Habit-Raster ohne Grün: das Muster trägt die Information, auch bei Farbfehlsichtigkeit lesbar.
- Daten sind über die Screens konsistent (Wochensumme 5:42, Timer-Zeit 10:42 + 0:42 = 11:24, Extension-Zähler passt zur Projekttabelle).
