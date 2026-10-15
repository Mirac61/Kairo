package auth

import (
	"context"
	"net/http"
	"time"
)

const maxRenewInterval = 15 * time.Minute

// contextKey ist ein privater Typ für Context-Schlüssel.
type contextKey int

const userIDKey contextKey = iota

// UserIDFromContext liest die Benutzer-ID aus dem Context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// RequireAuth prüft die Session und schreibt die User-ID in den Request-Context.
func RequireAuth(sessions SessionStore, ttl time.Duration, cookieSecure bool) func(http.Handler) http.Handler {
	renewInterval := min(maxRenewInterval, ttl/2)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			tokenHash := hashToken(cookie.Value)
			session, err := sessions.Get(r.Context(), tokenHash)
			if err != nil {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			now := time.Now().UTC()
			lastTouch := session.ExpiresAt.Add(-ttl)
			if lastTouch.Before(now.Add(-renewInterval)) {
				newExpiresAt := now.Add(ttl)
				if err := sessions.Touch(r.Context(), tokenHash, newExpiresAt); err == nil {
					setSessionCookie(w, cookie.Value, int(ttl.Seconds()), cookieSecure)
				}
			}

			ctx := context.WithValue(r.Context(), userIDKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth prüft die Session, falls vorhanden, verlangt sie aber nicht.
func OptionalAuth(sessions SessionStore, ttl time.Duration, cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			tokenHash := hashToken(cookie.Value)
			session, err := sessions.Get(r.Context(), tokenHash)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
