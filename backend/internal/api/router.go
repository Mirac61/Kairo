// Package api enthält Router, Handler und Sicherheits-Middleware.
package api

import (
	"encoding/json"
	"net/http"
)

// NewRouter baut den HTTP-Handler. Die Security-Middleware läuft vor dem Routing.
func NewRouter(port int, token, version string, webUI http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health(version))
	mux.Handle("/", webUI)
	return Security(port, token, mux)
}

func health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	}
}
