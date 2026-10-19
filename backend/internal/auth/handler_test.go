package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Nach 5 Fehlversuchen ist die Adresse eine Minute gesperrt, auch für das richtige Passwort.
func TestLoginLockout(t *testing.T) {
	svc, _, _ := newTestService(t)
	if _, err := svc.Register(context.Background(), "user@example.com", "secret123"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	h := NewHandler(svc, false)
	h.limit.now = func() time.Time { return now }
	login := func(addr, mail, pw string) int {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"`+mail+`","password":"`+pw+`"}`))
		r.RemoteAddr = addr
		w := httptest.NewRecorder()
		h.LoginHandler(w, r)
		return w.Code
	}

	for i := range maxLoginFails {
		if c := login("192.0.2.20:1000", "user@example.com", "falsch"); c != http.StatusUnauthorized {
			t.Fatalf("Versuch %d = %d", i+1, c)
		}
	}
	if c := login("192.0.2.20:2000", "user@example.com", "secret123"); c != http.StatusTooManyRequests {
		t.Errorf("gesperrte Adresse = %d, erwartet 429", c)
	}
	// Das Konto ist mitgesperrt, auch von einer anderen Adresse.
	if c := login("198.51.100.5:1000", "USER@example.com", "secret123"); c != http.StatusTooManyRequests {
		t.Errorf("gesperrtes Konto = %d, erwartet 429", c)
	}
	now = now.Add(lockFor + time.Second)
	if c := login("192.0.2.20:1000", "user@example.com", "secret123"); c != http.StatusNoContent {
		t.Errorf("nach Ablauf = %d, erwartet 204", c)
	}
}
