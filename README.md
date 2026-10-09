# Kairo

**Plan your day in the browser. Do the work in your editor. Kairo keeps both in sync.**

Kairo is a planner for tasks, calendar events, habits and notes. It runs on your own computer (macOS, Windows or Linux): no account, no cloud, no subscription. Your data is one SQLite file you can copy anywhere.

![Kairo start page](docs/screenshots/start.png)

## Why Kairo

- **One place for your day.** Calendar, tasks and habits sit side by side, so you see at a glance what's on and what's next.
- **Built for working in your editor.** The VSCodium/VS Code extension knows which project you have open and shows its tasks. Start a timer right there.
- **Honest time tracking.** Timers only run when you start them. When you go idle, Kairo asks before it counts the time.
- **Notes are just files.** Markdown in a normal folder, so you can edit them in Kairo or in your editor. You can also annotate PDFs and images.
- **Fast to type.** `Gym 30m @tomorrow #Uni !high` creates a 30-minute, high-priority task for tomorrow in project "Uni".
- **English or German.** Switch the language in the settings at any time.

## Get started in two minutes

1. **Download** the file for your system from the [latest release](https://github.com/Mirac61/Kairo/releases):

   | System | File |
   |---|---|
   | macOS (Apple Silicon) | `kairo_…_darwin_arm64.tar.gz` |
   | macOS (Intel) | `kairo_…_darwin_amd64.tar.gz` |
   | Windows | `kairo_…_windows_amd64.zip` |
   | Linux | `kairo_…_linux_amd64.tar.gz` (or `arm64`) |

2. **Unpack it and start Kairo.**
   - **macOS / Linux:** in a terminal, run `tar xzf kairo_*.tar.gz` and then `./kairo`. If macOS blocks it, run `xattr -d com.apple.quarantine kairo` once.
   - **Windows:** unzip, then double-click `kairo.exe`. If SmartScreen warns you, click "More info" → "Run anyway".
3. **Open <http://127.0.0.1:8742>** in your browser. Kairo starts in your browser's language; you can change it under settings.

Optional: install `kairo-*.vsix` from the same release in VSCodium or VS Code (Extensions → "Install from VSIX…").

**Start Kairo automatically at login.** Put the program in a fixed place and run `kairo install` from there. Remove it with `kairo uninstall`.

| System | How it starts | Log |
|---|---|---|
| macOS | LaunchAgent | `~/Library/Logs/kairo.log` |
| Linux | systemd user service | `journalctl --user -u kairo` |
| Windows | script in your Startup folder (no window) | `%LOCALAPPDATA%\kairo\kairo.log` |

## What's inside

![Calendar](docs/screenshots/kalender.png)

| View | What it does |
|---|---|
| **Start** | Running timer, what's next, today's plan with drag and drop |
| **Calendar** | Events and planned tasks in one week, next to the time you tracked. Imports `.ics` files |
| **Tasks** | Quick-add syntax, keyboard shortcuts, subtasks, priorities |
| **Habits** | Daily or weekly habits with a 28-day grid |
| **Projects** | Colors, links and files, time per project |
| **Notes** | Markdown, PDFs and images. Draw on PDFs, split view, templates |
| **Week** | A short review of your week |
| **Trash** | Everything you delete can be restored |

Almost every action can be undone right away.

![Tasks](docs/screenshots/aufgaben.png)

## Your data

- Everything stays on your machine. Kairo only listens on `127.0.0.1`.
- Database: `~/.local/share/kairo/kairo.db` (on Windows inside your user folder). Kairo makes a backup at every start; `kairo backup` makes one on demand.
- Notes: `~/Kairo` by default. You can change it in the app under settings.

---

## For developers

**Build from source.** You need Go (version from `backend/go.mod`), Node.js 22.18+ and pnpm.

```sh
make install   # install dependencies
make build     # build web UI + backend
./backend/bin/kairo
```

**Develop.**

```sh
cd backend  && go run ./cmd/server   # API on :8742
cd frontend && pnpm dev              # UI on :5173 with hot reload
make check                           # everything CI runs
make e2e                             # browser smoke tests
```

**How it fits together.**

```
Web UI (Vue 3)  ─┐
Extension       ─┼─ HTTP + WebSocket ─ Backend (Go) ─ SQLite
Menu bar plugin ─┘
```

The Go backend is the only place that stores data. The web UI is built into the binary. More detail:

- [`backend/README.md`](backend/README.md): configuration and environment variables
- [`frontend/README.md`](frontend/README.md) and [`extension/README.md`](extension/README.md)
- [`docs/`](docs/): vision, architecture and roadmap (in German)
- Releases are built from tags with `.goreleaser.yaml`

MIT licensed.
