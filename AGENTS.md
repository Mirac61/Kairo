# AGENTS.md

Anweisungen für Coding-Agenten in diesem Repository.

## Vorgehen

1. Lies zuerst `docs/04_AGENT_HANDOFF.md`.
2. Lies danach `docs/00_VISION.md` bis `docs/03_AGENT_GUIDELINES.md`.
3. Halte dich an `docs/03_AGENT_GUIDELINES.md`.

## Harte Regeln

- Das Backend MUSS in Go geschrieben sein.
- Backend + SQLite sind die Source of Truth.
- Schemaänderungen nur über Migrationen.
- Zeittracking nur mit echten Zeitstempeln.
- Kein automatischer Timer, nur weil ein Workspace geöffnet wurde.
- Arbeite in kleinen vertikalen Schritten.
- Vor dem Abschluss `make check` ausführen (gofmt, vet, Tests, Typprüfung, Extension-Build).
- UI-Änderungen im Frontend ansehen, nicht nur bauen: `npm run dev`, dann `npm run shot -- --view week|work|day|month` (Kalender) bzw. `--route tasks|habits|projects|review|trash` [--theme light] [--size 1280x800] [--click '<Selektor>'] in `frontend/` und das PNG in `frontend/.shots/` lesen (Mock-Daten, siehe `scripts/shot.mjs`).
