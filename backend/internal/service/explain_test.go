package service_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
	"kairo/internal/repository"
	"kairo/internal/service"
)

func TestExplainChecksSync(t *testing.T) {
	ctx := context.Background()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("keine Zeitzonendatenbank:", err)
	}
	db, err := repository.Open(ctx, filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	taskRepo := repository.NewTaskRepository(db)
	clock := time.Date(2026, 11, 23, 9, 0, 0, 0, loc) // Montag
	now := func() time.Time { return clock }
	tasks := service.NewTaskService(taskRepo, now).WithLocation(loc)
	cal := service.NewCalendarService(repository.NewCalendarRepository(db), loc, now)
	weekly := "FREQ=WEEKLY"
	lecture, err := cal.Create(ctx, service.CreateEventInput{Title: "Mathe 2", StartAt: "2026-11-23T09:15:00Z",
		EndAt: "2026-11-23T10:45:00Z", RecurrenceRule: &weekly, ExplainCheck: true}) // 10:15 Ortszeit
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cal.Create(ctx, service.CreateEventInput{Title: "Sprechstunde", StartAt: "2026-11-23T07:00:00Z",
		EndAt: "2026-11-23T07:30:00Z"}); err != nil {
		t.Fatal(err)
	}
	ex := service.NewExplainChecks(cal, tasks, loc, now)
	open := func() []domain.Task {
		t.Helper()
		ts, err := tasks.List(ctx, domain.TaskFilter{})
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	sync := func(at time.Time) {
		t.Helper()
		clock = at
		if err := ex.Sync(ctx); err != nil {
			t.Fatal(err)
		}
	}

	sync(time.Date(2026, 11, 23, 9, 0, 0, 0, loc))
	if n := len(open()); n != 0 {
		t.Fatalf("vor Beginn: %d Tasks, want 0", n)
	}

	sync(time.Date(2026, 11, 23, 10, 20, 0, 0, loc))
	sync(time.Date(2026, 11, 23, 18, 0, 0, 0, loc)) // zweiter Lauf legt nichts doppelt an
	ts := open()
	if len(ts) != 1 {
		t.Fatalf("nach Beginn: %d Tasks, want 1 (Sprechstunde ohne Haken zählt nicht)", len(ts))
	}
	got := ts[0]
	if got.Title != "5 Min erklären: Mathe 2" || *got.PlannedDate != "2026-11-23" || got.EstimatedMinutes != 5 ||
		got.PlannedStartAt != nil || got.Status != domain.TaskPlanned {
		t.Fatalf("Task = %+v", got)
	}
	if want := time.Date(2026, 11, 23, 23, 59, 0, 0, loc); !got.DueAt.Equal(want) {
		t.Fatalf("due_at = %v, want %v", got.DueAt, want)
	}
	if want := "explain:" + lecture.ID + "@2026-11-23"; *got.SourceKey != want {
		t.Fatalf("source_key = %q, want %q", *got.SourceKey, want)
	}

	// Am nächsten Tag verfällt der unerledigte Check, es entsteht kein neuer.
	sync(time.Date(2026, 11, 24, 8, 0, 0, 0, loc))
	if ts := open(); len(ts) != 1 || ts[0].Status != domain.TaskCancelled {
		t.Fatalf("nächster Tag: %+v, want 1 abgebrochene Task", ts)
	}
	// Wieder geöffnet (nachholen) bleibt er offen.
	planned := domain.TaskPlanned
	if _, err := tasks.Update(ctx, got.ID, service.UpdateTaskInput{Status: &planned}); err != nil {
		t.Fatal(err)
	}
	sync(time.Date(2026, 11, 24, 8, 5, 0, 0, loc))
	if ts := open(); ts[0].Status != domain.TaskPlanned {
		t.Fatalf("wieder geöffneter Check wurde %s", ts[0].Status)
	}

	// Nächste Vorlesung der Serie: neuer Check; gelöscht entsteht er nicht wieder.
	sync(time.Date(2026, 11, 30, 10, 20, 0, 0, loc))
	var next domain.Task
	for _, x := range open() {
		if *x.PlannedDate == "2026-11-30" {
			next = x
		}
	}
	if next.ID == "" {
		t.Fatal("kein Check für die zweite Vorlesung")
	}
	if err := tasks.Trash(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	sync(time.Date(2026, 11, 30, 11, 0, 0, 0, loc))
	if n := len(open()); n != 1 {
		t.Fatalf("nach Löschen: %d aktive Tasks, want 1 (nur der verfallene)", n)
	}

	// Ein erledigter Check verfällt nicht.
	clock = time.Date(2026, 12, 7, 10, 20, 0, 0, loc)
	if err := ex.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	done := domain.TaskCompleted
	for _, x := range open() {
		if *x.PlannedDate == "2026-12-07" {
			if _, err := tasks.Update(ctx, x.ID, service.UpdateTaskInput{Status: &done}); err != nil {
				t.Fatal(err)
			}
		}
	}
	sync(time.Date(2026, 12, 8, 8, 0, 0, 0, loc))
	for _, x := range open() {
		if *x.PlannedDate == "2026-12-07" && x.Status != domain.TaskCompleted {
			t.Fatalf("erledigter Check wurde %s", x.Status)
		}
	}
}
