# Kairo Backend

Go-Backend (stdlib `net/http`, SQLite via `modernc.org/sqlite`, ohne cgo).

- Starten: `go run ./cmd/server` (Version: `-ldflags "-X main.version=1.0"`)
- Autostart (macOS): `go build -o ~/.local/bin/kairo ./cmd/server`, dann `kairo install` (entfernen: `kairo uninstall`)
- Testen: `go test ./...` (auch `go vet ./...`, `gofmt -l .`)
- WebUI: `frontend/` baut nach `internal/web/dist` und wird per `embed` ausgeliefert.

| Variable | Standard | Bedeutung |
|---|---|---|
| `KAIRO_PORT` | `8742` | Port (Bind immer `127.0.0.1`) |
| `KAIRO_DB_PATH` | `~/.local/share/kairo/kairo.db` | SQLite-Datei |
| `KAIRO_TIMEZONE` | Systemzone | IANA-Zeitzone, z. B. `Europe/Berlin` |
| `KAIRO_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `KAIRO_NOTES_DIR` | `~/life-os` | Wurzelordner der Notizen (Markdown-Dateien), wird beim Start angelegt |
| `KAIRO_TOKEN_PATH` | `~/.config/kairo/token` | Token-Datei (v. a. für Tests) |

Sicherheitsmodell:

- Nur `127.0.0.1`; `Host` muss `127.0.0.1:<port>` oder `localhost:<port>` sein (DNS-Rebinding).
- Ändernde Requests und WebSockets unter `/api`, `/ws` brauchen eigenen `Origin` oder `Authorization: Bearer <token>`.
- Token wird beim ersten Start erzeugt (Datei `0600`, Ordner `0700`); es werden nie CORS-Header gesendet.
