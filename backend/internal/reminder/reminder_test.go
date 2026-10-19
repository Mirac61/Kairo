package reminder

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kairo/internal/domain"
)

type fakeStore struct {
	tasks       []domain.Task
	events      []domain.EventInstance
	lo, hi      time.Time
	tasksCalls  int
	eventsCalls int
}

func (f *fakeStore) UpcomingTasks(_ context.Context, lo, hi time.Time) ([]domain.Task, error) {
	f.tasksCalls++
	f.lo, f.hi = lo, hi
	return f.tasks, nil
}

func (f *fakeStore) Occurrences(_ context.Context, lo, hi time.Time) ([]domain.EventInstance, error) {
	f.eventsCalls++
	f.lo, f.hi = lo, hi
	return f.events, nil
}

func TestSend(t *testing.T) {
	var method, path, body, title string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		title = r.Header.Get("Title")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()

	berlin, _ := time.LoadLocation("Europe/Berlin")
	r := New(nil, srv.URL, "kairo", 10*time.Minute, time.Minute, berlin, nil)
	at := time.Date(2026, 10, 19, 14, 0, 0, 0, berlin)
	if err := r.send("VL 3", at, "Hörsaal 1"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/kairo" || body != "Start um 14:00 · Hörsaal 1" || title != "VL 3" {
		t.Fatalf("send = %s %s body=%q title=%q", method, path, body, title)
	}
	off := New(nil, "", "", 10*time.Minute, time.Minute, berlin, nil)
	if off.Enabled() {
		t.Fatal("off sollte als deaktiviert gelten")
	}
	if err := off.send("x", at, ""); err != nil {
		t.Fatalf("send ohne Konfiguration: %v", err)
	}
}

func TestTickWindowAndDedup(t *testing.T) {
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{
		tasks:  []domain.Task{{ID: "t1", Title: "Sport", PlannedStartAt: ptr(now.Add(5 * time.Minute))}},
		events: []domain.EventInstance{{Event: domain.CalendarEvent{ID: "e1", Title: "VL"}, Start: now.Add(8 * time.Minute)}},
	}
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { posts++ }))
	defer srv.Close()

	r := New(store, srv.URL, "kairo", 10*time.Minute, time.Minute, time.UTC, nil)
	r.now = func() time.Time { return now }

	ctx := context.Background()
	r.tick(ctx)
	r.tick(ctx) // zweiter Tick: nichts Neues (Dedup)
	if posts != 2 {
		t.Fatalf("posts = %d, erwartet 2", posts)
	}
	// Fenster: [now - 1m, now + 10m], also (lo, hi] für Aufgaben bzw. [lo, hi) für Termine.
	if !store.lo.Equal(now.Add(-time.Minute)) || !store.hi.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("Fenster = %v .. %v", store.lo, store.hi)
	}
}

func ptr[T any](v T) *T { return &v }
