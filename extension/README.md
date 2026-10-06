# Kairo – VSCodium-Extension

Phase 0: Gerüst, Activity Bar und Ansicht „Kontext“ (Backend-Status).
Die Extension speichert keine Produktdaten; das Backend ist die Source of Truth.

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
