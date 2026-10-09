# Kairo Backend

Go-Backend (stdlib `net/http`, SQLite via `modernc.org/sqlite`, ohne cgo).

- Starten: `go run ./cmd/server` (Version: `-ldflags "-X main.version=1.0"`)
- Autostart (macOS: LaunchAgent, Linux: systemd-User-Unit, Windows: Autostart-Ordner): `go build -o ~/.local/bin/kairo ./cmd/server`, dann `kairo install` (entfernen: `kairo uninstall`)
- Testen: `go test ./...` (auch `go vet ./...`, `gofmt -l .`)
- WebUI: `frontend/` baut nach `internal/web/dist` und wird per `embed` ausgeliefert.

Notizordner, Arbeitszeit und Zeitzone stellt man in der WebUI unter
Einstellungen ein. Sie landen in `~/.config/kairo/config.json` und gelten nach
einem Neustart (Knopf auf der Seite). Eine gesetzte Umgebungsvariable hat
Vorrang vor der Datei; die WebUI sperrt das Feld dann.

| Variable | Standard | Bedeutung |
|---|---|---|
| `KAIRO_PORT` | `8742` | Port (Bind immer `127.0.0.1`) |
| `KAIRO_DB_PATH` | `~/.local/share/kairo/kairo.db` | SQLite-Datei |
| `KAIRO_TIMEZONE` | Systemzone | IANA-Zeitzone, z. B. `Europe/Berlin` (auch in `config.json`) |
| `KAIRO_WORK_START`, `KAIRO_WORK_END` | `09:00`, `17:00` | Arbeitszeitfenster `HH:MM` (auch in `config.json`) |
| `KAIRO_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `KAIRO_NOTES_DIR` | `~/Kairo` | Wurzelordner der Notizen (Markdown-Dateien), wird beim Start angelegt (auch in `config.json`) |
| `KAIRO_TOKEN_PATH` | `~/.config/kairo/token` | Token-Datei (v. a. für Tests) |
| `KAIRO_CONFIG_PATH` | `~/.config/kairo/config.json` | Einstellungsdatei (v. a. für Tests) |

Sicherheitsmodell:

- Nur `127.0.0.1`; `Host` muss `127.0.0.1:<port>` oder `localhost:<port>` sein (DNS-Rebinding).
- Ändernde Requests und WebSockets unter `/api`, `/ws` brauchen eigenen `Origin` oder `Authorization: Bearer <token>`.
- Token wird beim ersten Start erzeugt (Datei `0600`, Ordner `0700`); es werden nie CORS-Header gesendet.
