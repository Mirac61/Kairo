# Kairo Frontend

Vue 3 + TypeScript + Vite (WebUI zum Planen).

## Entwicklung

```sh
pnpm install
pnpm dev         # http://127.0.0.1:5173
```

Das Backend muss auf `127.0.0.1:8742` laufen; Vite leitet `/api` und `/ws`
dorthin weiter (kein CORS nötig).

## Produktion

```sh
pnpm build
```

Die Ausgabe landet in `../backend/internal/web/dist` und wird vom Go-Binary
per `embed` ausgeliefert. `pnpm type-check` prüft nur die Typen, `pnpm test` führt die Tests in `test/` aus (Node ab 22.18, ohne Testframework). `pnpm e2e` führt die Smoke-Tests in `e2e/` gegen das gebaute Binary aus (vorher `make build`).

Texte der Oberfläche stehen auf Deutsch im Code und gehen durch `t('…')` (Script) bzw. `$t('…')` (Template) aus `src/lib/i18n.ts`; Platzhalter als `{name}`, Mehrzahl mit `tn(n, '{n} Eintrag', '{n} Einträge')`. Jeder neue Text braucht einen Eintrag in `src/lib/en.ts`, sonst schlägt `test/i18n.test.ts` fehl. Mit `pnpm shot --lang en` lässt sich die englische Oberfläche ansehen.
