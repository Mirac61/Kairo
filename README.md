# Kairo

Kairo ist ein lokales Personal OS für Planung und Arbeit: WebUI = Planen, VSCodium = Arbeiten.

**Status:** Der MVP (Phasen 0–5 der [Roadmap](docs/05_ROADMAP.md)) ist fertig: Kalender mit Serien, Tasks, Habits, Projekte, Ressourcen, Zeiterfassung mit Korrektur, Today-Ansicht mit überfälligen Tasks, Echtzeit-Updates und die VSCodium-Extension mit Workspace-Erkennung. Dazu kommen die Rückfrage bei Inaktivität (Phase 6), `GET /api/review` (Phase 7) und Backups (`kairo backup`, automatisch beim Serverstart).

## Dokumentation

- [Vision](docs/00_VISION.md) – Idee, Ziele und Leitprinzipien
- [Produktanforderungen](docs/01_PRODUCT_REQUIREMENTS.md) – Funktionen und Anforderungen im Detail
- [Technische Architektur](docs/02_ARCHITECTURE.md) – Komponenten, Datenmodell und Struktur
- [Regeln für Agenten](docs/03_AGENT_GUIDELINES.md) – verbindliche Regeln für die Entwicklung
- [Einstieg für Agenten](docs/04_AGENT_HANDOFF.md) – Übergabe und Startpunkt für Coding-Agenten
- [Roadmap](docs/05_ROADMAP.md) – Phasen und Meilensteine

## Struktur

```
backend/     Go + SQLite
frontend/    Vue 3 + TypeScript + Vite
extension/   VSCodium-Extension
tools/       Hilfsskripte (Menüleiste)
docs/        Projektdokumentation
```

## Entwicklung

Entwicklungsmodus (zwei Terminals):

```
cd backend  && go run ./cmd/server      # http://127.0.0.1:8742
cd frontend && npm install && npm run dev   # http://127.0.0.1:5173, leitet /api ans Backend
```

Ein einzelnes Binary mit eingebetteter WebUI (die Reihenfolge ist wichtig, weil `embed` beim Kompilieren greift):

```
cd frontend && npm run build            # schreibt nach backend/internal/web/dist
cd backend  && go build -o kairo ./cmd/server
```

Die Extension steht in `extension/` (siehe dortige README, Start per F5 in VSCodium).
Details zu Konfiguration und Sicherheit: `backend/README.md`.

Timer in der macOS-Menüleiste: `brew install --cask swiftbar`, dann `tools/menubar/kairo.10s.sh` in den SwiftBar-Plugin-Ordner verlinken. Das Skript zeigt den laufenden Timer und bietet Pause, Fertig und das Starten heutiger Tasks an (Backend-URL und Token per `KAIRO_URL` und `KAIRO_TOKEN_PATH`).
