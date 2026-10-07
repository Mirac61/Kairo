.PHONY: help install build check
.DEFAULT_GOAL := help

help:
	@echo "make install  npm ci in frontend/ und extension/ (ersetzt node_modules)"
	@echo "make build    WebUI bauen, dann backend/bin/kairo"
	@echo "make check    gofmt, vet, Tests, Typprüfung, Extension-Build"

install:
	cd frontend && npm ci
	cd extension && npm ci

# Die Reihenfolge ist wichtig: embed greift beim Kompilieren, also muss das Frontend vorher nach backend/internal/web/dist bauen.
build:
	cd frontend && npm run build
	cd backend && go build -o bin/kairo ./cmd/server

# Alles, was auch die CI prüft.
check:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...
	cd frontend && npm run type-check && npm test
	# Die Extension trägt eine Kopie des Schnelleingabe-Parsers der WebUI; beide müssen gleich bleiben.
	cmp frontend/src/lib/quickAdd.ts extension/src/quickAdd.ts
	cd extension && npm run compile && npm test
