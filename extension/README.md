# Kairo – VSCodium-Extension

Phase 4: Ansicht „Kontext“ mit Backend-Status, erkanntem Projekt, aktueller Task
und den Tasks von heute (Start, Pause, Abschließen per Inline-Button).
Die Extension speichert keine Produktdaten; das Backend ist die Source of Truth.

## Funktionsweise

- **Projekt-Erkennung:** Der längste `local_path` eines Projekts, der einem
  Workspace-Ordner gleicht oder ihn umfasst (ganze Pfadsegmente).
- **Live:** `/ws` ist nur offen, solange die Ansicht sichtbar ist. Jedes
  Ereignis, Verbinden und Trennen lädt Projekte und `/api/today` neu. Ohne
  Verbindung zeigt die Ansicht „Backend: offline“; die Extension verbindet
  alle 2 s neu.
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
