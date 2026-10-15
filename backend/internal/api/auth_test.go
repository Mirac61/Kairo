package api

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kairo/internal/auth"
)

// stubSessions ist ein Session-Store für Tests: nur der Hash von validCookie gilt als Session.
type stubSessions struct {
	validCookie string
	userID      string
}

func (s stubSessions) Create(context.Context, auth.Session) error { return nil }

func (s stubSessions) Get(_ context.Context, tokenHash []byte) (auth.Session, error) {
	want := sha256.Sum256([]byte(s.validCookie))
	if string(tokenHash) != string(want[:]) {
		return auth.Session{}, errors.New("unbekannte session")
	}
	return auth.Session{UserID: s.userID, ExpiresAt: time.Now().UTC().Add(time.Hour)}, nil
}

func (s stubSessions) Touch(context.Context, []byte, time.Time) error { return nil }
func (s stubSessions) Delete(context.Context, []byte) error           { return nil }
func (s stubSessions) DeleteExpiredByUser(context.Context, string) error {
	return nil
}

func TestAuthMiddleware(t *testing.T) {
	const validCookie = "session-cookie-value"
	sessions := stubSessions{validCookie: validCookie, userID: "user-1"}
	mw := AuthMiddleware(true, testToken, sessions, time.Hour, false)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	protected := mw(ok)

	tests := []struct {
		name    string
		path    string
		headers map[string]string
		cookie  string
		want    int
	}{
		{"kein cookie kein token", "/api/x", nil, "", 401},
		{"gültiger bearer token", "/api/x", map[string]string{"Authorization": "Bearer " + testToken}, "", 204},
		{"falscher bearer token", "/api/x", map[string]string{"Authorization": "Bearer nope"}, "", 401},
		{"gültiges session-cookie", "/api/x", nil, validCookie, 204},
		{"bearer token am websocket", "/ws", map[string]string{"Authorization": "Bearer " + testToken}, "", 204},
		{"öffentlicher login-pfad", "/api/auth/login", nil, "", 204},
		{"health ohne anmeldung", "/api/health", nil, "", 204},
		{"webui ohne auth", "/", nil, "", 204},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		if tc.cookie != "" {
			req.AddCookie(&http.Cookie{Name: "session", Value: tc.cookie})
		}
		res := httptest.NewRecorder()
		protected.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Errorf("%s: code = %d, want %d", tc.name, res.Code, tc.want)
		}
	}

	// Ohne aktivierte Auth lässt die Middleware alles durch.
	open := AuthMiddleware(false, testToken, sessions, time.Hour, false)(ok)
	res := httptest.NewRecorder()
	open.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if res.Code != http.StatusNoContent {
		t.Errorf("auth deaktiviert: code = %d, want 204", res.Code)
	}
}
