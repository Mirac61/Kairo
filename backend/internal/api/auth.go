package api

import (
	"crypto/subtle"
	"net/http"
	"time"

	"kairo/internal/auth"
)

// AuthMiddleware schützt /api und /ws mit Session-Cookies, wenn Auth aktiviert ist.
// Requests mit korrektem API-Token (`Authorization: Bearer …`) gelten ebenfalls als
// authentifiziert; damit funktioniert die Extension auch bei aktiver Anmeldung,
// auch wenn sie nur über localhost (z. B. per VPN-Port-Forward) angebunden ist.
// /api/auth/login, /api/auth/register und /api/auth/setup-required bleiben öffentlich.
// Nicht-API-Routen (z. B. die WebUI) bleiben ungeschützt, damit die Login-Seite geladen werden kann.
func AuthMiddleware(enabled bool, token string, sessions auth.SessionStore, ttl time.Duration, cookieSecure bool) func(http.Handler) http.Handler {
	if !enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	tokenBytes := []byte(token)
	require := auth.RequireAuth(sessions, ttl, cookieSecure)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Health bleibt offen: Statusanzeige und Extension prüfen damit nur die Erreichbarkeit.
			if underAPIOrWS(r.URL.Path) && !isPublicAuthRoute(r.URL.Path) && r.URL.Path != "/api/health" {
				if got, ok := bearer(r); ok && subtle.ConstantTimeCompare([]byte(got), tokenBytes) == 1 {
					next.ServeHTTP(w, r)
					return
				}
				require(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isPublicAuthRoute sagt, ob ein Pfad zu den öffentlichen Auth-Endpunkten gehört.
func isPublicAuthRoute(p string) bool {
	switch p {
	case "/api/auth/login", "/api/auth/register", "/api/auth/setup-required", "/api/auth/logout":
		return true
	}
	return false
}
