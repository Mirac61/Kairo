# Kairo

Kairo ist ein lokales Personal OS für Planung und Arbeit: WebUI = Planen, VSCodium = Arbeiten.

**Status:** Phase 0 (Gerüst) – Backend, WebUI und Extension starten und sprechen miteinander (`/api/health`). Fachlogik folgt ab Phase 1.

## Dokumentation

- [Vision](docs/00_VISION.md) – Idee, Ziele und Leitprinzipien
- [Produktanforderungen](docs/01_PRODUCT_REQUIREMENTS.md) – Funktionen und Anforderungen im Detail
- [Technische Architektur](docs/02_ARCHITECTURE.md) – Komponenten, Datenmodell und Struktur
- [Regeln für Agenten](docs/03_AGENT_GUIDELINES.md) – verbindliche Regeln für die Entwicklung
- [Einstieg für Agenten](docs/04_AGENT_HANDOFF.md) – Übergabe und Startpunkt für Coding-Agenten
- [Roadmap](docs/05_ROADMAP.md) – Phasen und Meilensteine

## Geplante Struktur

```
backend/     Go + SQLite
frontend/    Vue 3 + TypeScript + Vite
extension/   VSCodium-Extension
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
