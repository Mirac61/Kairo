package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"kairo/internal/domain"
)

type memTasks struct{ m map[string]domain.Task }

func (s *memTasks) Create(_ context.Context, t domain.Task) error { s.m[t.ID] = t; return nil }
func (s *memTasks) Get(_ context.Context, id string) (domain.Task, error) {
	t, ok := s.m[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return t, nil
}
func (s *memTasks) List(context.Context, domain.TaskFilter) ([]domain.Task, error) { return nil, nil }
func (s *memTasks) Update(_ context.Context, t domain.Task) error                  { s.m[t.ID] = t; return nil }
func (s *memTasks) Delete(_ context.Context, id string) error                      { delete(s.m, id); return nil }

// Papierkorb: gegen echtes SQLite getestet (api/trash_test.go).
func (s *memTasks) Trash(context.Context, string, time.Time) error       { return nil }
func (s *memTasks) Restore(context.Context, string) (domain.Task, error) { return domain.Task{}, nil }
func (s *memTasks) ListTrashed(context.Context) ([]domain.Task, error)   { return nil, nil }

func newTaskSvc() (*TaskService, *time.Time) {
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	return NewTaskService(&memTasks{m: map[string]domain.Task{}}, func() time.Time { return clock }), &clock
}

func TestTaskCreateDefaultsAndValidation(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()

	task, err := svc.Create(ctx, CreateTaskInput{Title: " Lernen ", PlannedDate: ptr("2026-10-07"),
		PlannedStartAt: ptr("2026-10-07T10:15:00+02:00")})
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Lernen" || task.Status != domain.TaskBacklog || task.Priority != domain.PriorityMedium {
		t.Errorf("Defaults = %+v", task)
	}
	if got := task.PlannedStartAt.Format(time.RFC3339); got != "2026-10-07T08:15:00Z" {
		t.Errorf("planned_start_at nicht in UTC: %s", got)
	}

	for name, in := range map[string]CreateTaskInput{
		"leerer Titel":       {Title: " "},
		"falscher Status":    {Title: "x", Status: "NOPE"},
		"falsche Priorität":  {Title: "x", Priority: "NOPE"},
		"negative Minuten":   {Title: "x", EstimatedMinutes: -1},
		"falsches Datum":     {Title: "x", PlannedDate: ptr("07.10.2026")},
		"Start ohne Datum":   {Title: "x", PlannedStartAt: ptr("2026-10-07T10:00:00Z")},
		"falscher Zeitpunkt": {Title: "x", DueAt: ptr("morgen")},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestTaskUpdateCompletedAtFollowsStatus(t *testing.T) {
	svc, clock := newTaskSvc()
	ctx := context.Background()
	task, _ := svc.Create(ctx, CreateTaskInput{Title: "x", Description: "d"})

	*clock = clock.Add(time.Hour)
	done := domain.TaskCompleted
	got, err := svc.Update(ctx, task.ID, UpdateTaskInput{Status: &done})
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(*clock) || got.Description != "d" {
		t.Errorf("COMPLETED: %+v", got)
	}

	*clock = clock.Add(time.Hour)
	got, _ = svc.Update(ctx, task.ID, UpdateTaskInput{Title: ptr("y")})
	if got.CompletedAt == nil || got.CompletedAt.Equal(*clock) {
		t.Errorf("completed_at ohne Statuswechsel verändert: %+v", got)
	}

	open := domain.TaskPlanned
	got, _ = svc.Update(ctx, task.ID, UpdateTaskInput{Status: &open})
	if got.CompletedAt != nil {
		t.Error("completed_at nach Wiedereröffnen nicht gelöscht")
	}
}

func TestTaskUpdateClearsOptionalFields(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()
	task, _ := svc.Create(ctx, CreateTaskInput{Title: "x", PlannedDate: ptr("2026-10-07"),
		PlannedStartAt: ptr("2026-10-07T10:00:00Z"), ProjectID: ptr("p")})

	got, err := svc.Update(ctx, task.ID, UpdateTaskInput{PlannedDate: ptr(""), PlannedStartAt: ptr(""), ProjectID: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.PlannedDate != nil || got.PlannedStartAt != nil || got.ProjectID != nil {
		t.Errorf("nicht entfernt: %+v", got)
	}
	// Datum entfernen, Startzeit behalten verletzt die Regel.
	task, _ = svc.Create(ctx, CreateTaskInput{Title: "y", PlannedDate: ptr("2026-10-07"), PlannedStartAt: ptr("2026-10-07T10:00:00Z")})
	if _, err := svc.Update(ctx, task.ID, UpdateTaskInput{PlannedDate: ptr("")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Datum ohne Startzeit entfernen: %v", err)
	}
}

func TestTaskParentCycles(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()
	a, _ := svc.Create(ctx, CreateTaskInput{Title: "a"})
	b, _ := svc.Create(ctx, CreateTaskInput{Title: "b", ParentTaskID: &a.ID})
	c, _ := svc.Create(ctx, CreateTaskInput{Title: "c", ParentTaskID: &b.ID})

	for name, tc := range map[string]struct{ id, parent string }{
		"selbst":    {a.ID, a.ID},
		"Kind":      {a.ID, b.ID},
		"Enkel":     {a.ID, c.ID},
		"unbekannt": {a.ID, "missing"},
	} {
		if _, err := svc.Update(ctx, tc.id, UpdateTaskInput{ParentTaskID: ptr(tc.parent)}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := svc.Update(ctx, c.ID, UpdateTaskInput{ParentTaskID: ptr(a.ID)}); err != nil {
		t.Errorf("gültiges Umhängen: %v", err)
	}
}

func TestTaskListRejectsBadFilters(t *testing.T) {
	svc, _ := newTaskSvc()
	if _, err := svc.List(context.Background(), domain.TaskFilter{Status: "NOPE"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Status: %v", err)
	}
	if _, err := svc.List(context.Background(), domain.TaskFilter{PlannedDate: "heute"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Datum: %v", err)
	}
}
