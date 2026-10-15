package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
)

// Security schützt den Server (siehe docs/02_ARCHITECTURE.md,
// "Lokale Sicherheit"). Es werden nie CORS-Header gesendet.
// Wenn authEnabled true ist, wird der Host-Check aufgehoben; API/WS-Requests
// ohne Bearer-Token werden durchgelassen, wenn ihr Origin zum Host passt, damit
// die Auth-Middleware das Session-Cookie prüfen kann. SameSite ignoriert den
// Port, ohne Origin-Check könnte also ein anderer Dienst auf derselben IP
// Anfragen mit dem Cookie schicken.
func Security(port int, token string, authEnabled bool, next http.Handler) http.Handler {
	p := strconv.Itoa(port)
	hosts := map[string]bool{"127.0.0.1:" + p: true, "localhost:" + p: true}
	origins := map[string]bool{"http://127.0.0.1:" + p: true, "http://localhost:" + p: true}
	tokenBytes := []byte(token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authEnabled {
			if !hosts[r.Host] {
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
		}
		if needsOriginOrToken(r) && !isPublicAuthRoute(r.URL.Path) {
			if !authEnabled && origins[r.Header.Get("Origin")] {
				// eigener Origin
			} else if got, ok := bearer(r); ok {
				if subtle.ConstantTimeCompare([]byte(got), tokenBytes) != 1 {
					http.Error(w, "invalid token", http.StatusUnauthorized)
					return
				}
			} else if authEnabled && underAPIOrWS(r.URL.Path) && sameOrigin(r) {
				// API/WS dürfen bei aktiviertem Auth auch per Session-Cookie
				// authentifiziert werden; das prüft die Auth-Middleware.
			} else {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin sagt, ob der Origin-Header genau auf den angefragten Host zeigt.
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return o == "http://"+r.Host || o == "https://"+r.Host
}

func needsOriginOrToken(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return isWebSocketUpgrade(r) && underAPIOrWS(r.URL.Path)
	}
	return true
}

func underAPIOrWS(p string) bool {
	for _, prefix := range []string{"/api", "/ws"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
