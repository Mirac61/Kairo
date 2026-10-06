# Kairo Frontend

Vue 3 + TypeScript + Vite (WebUI zum Planen).

## Entwicklung

```sh
npm install
npm run dev      # http://127.0.0.1:5173
```

Das Backend muss auf `127.0.0.1:8742` laufen; Vite leitet `/api` und `/ws`
dorthin weiter (kein CORS nötig).

## Produktion

```sh
npm run build
```

Die Ausgabe landet in `../backend/internal/web/dist` und wird vom Go-Binary
per `embed` ausgeliefert. `npm run type-check` prüft nur die Typen.
