# Kairo

Kairo ist ein lokales Personal OS für Planung und Arbeit: WebUI = Planen, VSCodium = Arbeiten.

**Status:** Planungsphase – bisher existiert nur Dokumentation, noch kein Code.

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
