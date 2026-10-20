package reminder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kairo/internal/domain"
)

type fakeStore struct {
	tasks       []domain.Task
	due         []domain.Task
	dueCalls    int
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

func (f *fakeStore) DueTasks(_ context.Context, _, _ time.Time) ([]domain.Task, error) {
	f.dueCalls++
	return f.due, nil
}

func (f *fakeStore) Occurrences(_ context.Context, lo, hi time.Time) ([]domain.EventInstance, error) {
	f.eventsCalls++
	f.lo, f.hi = lo, hi
	return f.events, nil
}

func TestSend(t *testing.T) {
	var method, path string
	var msg map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		msg = nil
		_ = json.NewDecoder(r.Body).Decode(&msg)
	}))
	defer srv.Close()

	berlin, _ := time.LoadLocation("Europe/Berlin")
	r := New(nil, srv.URL, "kairo", 10*time.Minute, time.Minute, berlin, nil)
	at := time.Date(2026, 10, 19, 14, 0, 0, 0, berlin)
	if err := r.send("VL 3", at, "Hörsaal 1"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/" || msg["topic"] != "kairo" || msg["message"] != "Start um 14:00 · Hörsaal 1" || msg["title"] != "VL 3" {
		t.Fatalf("send = %s %s %v", method, path, msg)
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

func TestDueDigest(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	day := time.Date(2026, 10, 20, 0, 0, 0, 0, berlin)
	store := &fakeStore{due: []domain.Task{
		{ID: "a", Title: "Blatt 2", DueAt: ptr(day.Add(-time.Minute))},
		{ID: "b", Title: "Abgabe Blatt 3", DueAt: ptr(day.Add(23*time.Hour + 59*time.Minute))},
	}}
	var msgs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		msgs = append(msgs, m)
	}))
	defer srv.Close()

	r := New(store, srv.URL, "kairo", 10*time.Minute, time.Minute, berlin, nil).WithDigest(8 * time.Hour)
	ctx := context.Background()
	for _, at := range []time.Duration{7*time.Hour + 59*time.Minute, 8 * time.Hour, 8*time.Hour + time.Minute, 9 * time.Hour} {
		now := day.Add(at)
		r.now = func() time.Time { return now }
		r.tick(ctx)
	}
	if len(msgs) != 1 {
		t.Fatalf("%d Meldungen, erwartet genau eine um 08:00: %v", len(msgs), msgs)
	}
	if msgs[0]["title"] != "Heute fällig: 1 · überfällig: 1" || msgs[0]["message"] != "Abgabe Blatt 3\nÜberfällig: Blatt 2" {
		t.Fatalf("Meldung %v", msgs[0])
	}
	// Später am Tag gestartet: keine Meldung mehr; ausgeschaltet: keine Abfrage.
	late := New(store, srv.URL, "kairo", 10*time.Minute, time.Minute, berlin, nil).WithDigest(8 * time.Hour)
	late.now = func() time.Time { return day.Add(10 * time.Hour) }
	late.tick(ctx)
	calls := store.dueCalls
	off := New(store, srv.URL, "kairo", 10*time.Minute, time.Minute, berlin, nil)
	off.now = func() time.Time { return day.Add(8 * time.Hour) }
	off.tick(ctx)
	if len(msgs) != 1 || store.dueCalls != calls {
		t.Fatalf("Meldungen %d, Abfragen %d -> %d", len(msgs), calls, store.dueCalls)
	}
}

func TestJobs(t *testing.T) {
	sunday := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	var titles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		titles = append(titles, m["title"].(string)+"|"+m["message"].(string))
	}))
	defer srv.Close()

	journalDone := false
	r := New(&fakeStore{}, srv.URL, "kairo", 10*time.Minute, time.Minute, time.UTC, nil).
		WithDigest(8*time.Hour).
		WithDigestLine(func(context.Context) (string, error) { return "Ziel: Fit", nil }).
		WithJob(time.Sunday, 19*time.Hour, func(_ context.Context, day time.Time) (string, string, error) {
			return "Woche", day.Format("2006-01-02"), nil
		}).
		WithJob(Daily, 21*time.Hour, func(context.Context, time.Time) (string, string, error) {
			if journalDone {
				return "", "", nil
			}
			return "Journal", "", nil
		}).
		WithJob(Daily, -1, func(context.Context, time.Time) (string, string, error) { panic("ausgeschaltet") })
	ctx := context.Background()
	tick := func(d time.Time) {
		r.now = func() time.Time { return d }
		r.tick(ctx)
	}
	// Sonntag: Sammelmeldung nur mit Ziel, Wochenbilanz, Journal; jede nur einmal.
	for _, h := range []time.Duration{8 * time.Hour, 8*time.Hour + time.Minute, 19 * time.Hour, 19*time.Hour + 30*time.Minute, 21 * time.Hour} {
		tick(sunday.Add(h))
	}
	// Montag: keine Wochenbilanz, Journal schon geschrieben.
	journalDone = true
	for _, h := range []time.Duration{19 * time.Hour, 21 * time.Hour} {
		tick(sunday.AddDate(0, 0, 1).Add(h))
	}
	want := []string{"Heute nichts fällig|Ziel: Fit", "Woche|2026-10-25", "Journal|"}
	if len(titles) != len(want) {
		t.Fatalf("Meldungen = %q, erwartet %q", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("Meldung %d = %q, erwartet %q", i, titles[i], want[i])
		}
	}
}
