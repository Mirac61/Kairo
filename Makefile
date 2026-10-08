.PHONY: help install build check run link

BIN := backend/bin/kairo
BINDIR ?= $(HOME)/.local/bin
OPEN ?= open
URL = http://127.0.0.1:$(or $(KAIRO_PORT),8742)
# Was ein neues Binary nötig macht: Go-Code und alles, was in die WebUI einfließt.
SRC := $(shell find backend frontend/src frontend/public frontend/index.html frontend/pnpm-lock.yaml frontend/vite.config.ts -type f -not -path 'backend/bin/*' -not -path 'backend/internal/web/dist/*' 2>/dev/null)
.DEFAULT_GOAL := help

help:
	@echo "make install  pnpm install in frontend/ und extension/ (nach Lockfile)"
	@echo "make build    WebUI bauen, dann backend/bin/kairo"
	@echo "make run      bei Bedarf bauen, Backend starten, WebUI im Browser öffnen"
	@echo "make link     Befehl 'kairo' nach $(BINDIR) legen (startet 'make run')"
	@echo "make check    gofmt, vet, Tests, Typprüfung, Extension-Build"

install:
	cd frontend && pnpm install --frozen-lockfile
	cd extension && pnpm install --frozen-lockfile

# Die Reihenfolge ist wichtig: embed greift beim Kompilieren, also muss das Frontend vorher nach backend/internal/web/dist bauen.
build:
	cd frontend && pnpm build
	cd backend && go build -o bin/kairo ./cmd/server

# Baut nur neu, wenn sich Quellen geändert haben.
$(BIN): $(SRC)
	$(MAKE) build

# Läuft schon ein Server auf dem Port, wird nur der Browser geöffnet.
run: $(BIN)
	@if curl -sf -o /dev/null $(URL); then $(OPEN) $(URL); exit; fi; \
	(for i in $$(seq 50); do curl -sf -o /dev/null $(URL) && { $(OPEN) $(URL); break; }; sleep 0.2; done) & \
	exec $(BIN)

# 'kairo' ohne Argument startet 'make run', mit Argument (install, backup, ...) geht es ans Binary.
# Nach einem Verschieben des Repos neu ausführen.
link:
	mkdir -p $(BINDIR)
	printf '#!/bin/sh\n[ $$# -gt 0 ] && exec "%s/$(BIN)" "$$@"\nexec make -C "%s" run\n' "$(CURDIR)" "$(CURDIR)" > $(BINDIR)/kairo
	chmod +x $(BINDIR)/kairo

# Alles, was auch die CI prüft.
check:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...
	cd frontend && pnpm type-check && pnpm test
	# Die Extension trägt eine Kopie des Schnelleingabe-Parsers der WebUI; beide müssen gleich bleiben.
	cmp frontend/src/lib/quickAdd.ts extension/src/quickAdd.ts
	cd extension && pnpm compile && pnpm test
