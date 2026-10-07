# Kairo – VSCodium-Extension

Phase 4: Ansicht „Kontext“ mit Backend-Status, erkanntem Projekt, aktueller Task
und den Tasks von heute (Start, Pause, Abschließen per Inline-Button).
Die Extension speichert keine Produktdaten; das Backend ist die Source of Truth.

## Funktionsweise

- **Projekt-Erkennung:** Der längste `local_path` eines Projekts, der einem
  Workspace-Ordner gleicht oder ihn umfasst (ganze Pfadsegmente).
- **Live:** `/ws` ist ab dem Start offen, auch bei verborgener Ansicht (die
  Statusleiste braucht den Stand). Jedes Ereignis, Verbinden und Trennen lädt
  Projekte und `/api/today` neu. Ohne Verbindung zeigt die Ansicht „Backend:
  offline“; die Extension verbindet alle 2 s neu. Ein Ordnerwechsel im
  Workspace erkennt das Projekt neu.
- **Statusleiste:** `$(clock) 0:32 · Task` bei laufendem Timer (Laufzeit aus dem
  Startzeitstempel des Backends), sonst `$(play) Kairo`. Ein Klick öffnet ein
  Menü (Pause, Fertig, Task wechseln, Kairo öffnen).
- **Tabs:** „Jetzt“ (Projekt mit Ressourcen und offenen Tasks, Timer, Kennzahlen,
  Zeile „Diese Woche“ mit Link „Rückblick“, der `<kairo.backendUrl>/review` im
  Browser öffnet), „Heute“ und „Projekte“. Die Tabs „Stats“ und „Dokumente“ gibt
  es nicht mehr; Rückblick und Statistik stehen in der WebUI, die Einstellung
  `kairo.documentsFolder` entfällt.
- **Befehle:** Kairo: Task starten …, Timer pausieren, Task abschließen, Neue
  Task …, Projekt öffnen …, Ressource öffnen …, Diesen Ordner mit Projekt
  verknüpfen. „Ressource öffnen …“ durchsucht die Ressourcen aller Projekte und
  Tasks (die des erkannten Projekts stehen oben); Dateien öffnen im Editor oder
  in der Standard-App, Ordner im Finder, URLs im Browser. Ein Timer startet nur
  auf ausdrückliche Aktion, nie weil ein Workspace geöffnet wurde.
- **URI-Handler** (`vscodium://kairo-local.kairo/…`), damit die WebUI VSCodium
  steuern kann:
  - `/start?task=<id>` startet den Timer der Task (ausdrückliche Aktion durch den
    Klick in der WebUI), wechselt zum Projektordner (`local_path`; schon offen:
    nichts tun, sonst neues Fenster, wenn bereits ein Ordner offen ist, sonst
    dasselbe Fenster) und öffnet danach die Ressourcen der Task. Nach einem
    Ordnerwechsel merkt sich die Extension die Task kurz (`pendingOpen`,
    60 s) und öffnet die Ressourcen beim Aktivieren im neuen Fenster; das startet
    nie einen Timer.
  - `/open?project=<id>` öffnet nur den Projektordner, ohne Timer.
  - Fehler (unbekannte Task oder Projekt, Backend aus) erscheinen mit der
    Backend-Meldung.
- **Schreibende Aufrufe** (Start/Pause/Abschließen, Quelle `VSCODIUM`) und
  `/ws` senden das Token aus `~/.config/kairo/token`; ohne Token lehnt das
  Backend sie ab.
- Laufzeit-Abhängigkeit `ws`: VSCodium bringt kein globales `WebSocket` mit.
  Für ein `.vsix` muss `node_modules` mit verpackt werden.

## Test

    npm test

## Bauen

    npm install
    npm run compile

## Starten

Den Ordner `extension/` in VSCodium öffnen und F5 drücken
(„Run Extension“). Das Backend sollte unter `kairo.backendUrl`
(Standard `http://127.0.0.1:8742`) laufen, sonst zeigt die Ansicht „Backend: offline“.

## Verpacken (später)

    npx @vscode/vsce package

Die entstehende `.vsix` lässt sich dann manuell in VSCodium installieren.
