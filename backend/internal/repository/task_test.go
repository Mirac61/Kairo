package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
)

func newTaskRepos(t *testing.T) (*TaskRepository, *ProjectRepository) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewTaskRepository(db), NewProjectRepository(db)
}

func ptr[T any](v T) *T { return &v }

func newTask(id string) domain.Task {
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	return domain.Task{ID: id, Title: id, Status: domain.TaskBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now}
}

func TestTaskRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	tasks, projects := newTaskRepos(t)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	if err := projects.Create(ctx, domain.Project{ID: "p1", Name: "P", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	task := newTask("t1")
	task.ProjectID = ptr("p1")
	task.PlannedDate = ptr("2026-10-07")
	task.PlannedStartAt = ptr(now.Add(24 * time.Hour))
	task.DueAt = ptr(now.Add(48 * time.Hour))
	task.EstimatedMinutes = 90
	if err := tasks.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	got, err := tasks.Get(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID == nil || *got.ProjectID != "p1" || *got.PlannedDate != "2026-10-07" ||
		!got.PlannedStartAt.Equal(*task.PlannedStartAt) || !got.DueAt.Equal(*task.DueAt) ||
		got.EstimatedMinutes != 90 || got.CompletedAt != nil {
		t.Errorf("Get = %+v", got)
	}

	task.Status, task.CompletedAt, task.PlannedStartAt = domain.TaskCompleted, &now, nil
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	got, _ = tasks.Get(ctx, "t1")
	if got.Status != domain.TaskCompleted || got.CompletedAt == nil || got.PlannedStartAt != nil {
		t.Errorf("nach Update = %+v", got)
	}

	if err := tasks.Delete(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Get(ctx, "t1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get nach Delete: %v", err)
	}
}

func TestTaskRepositoryFilters(t *testing.T) {
	ctx := context.Background()
	tasks, _ := newTaskRepos(t)
	a, b, c := newTask("a"), newTask("b"), newTask("c")
	a.PlannedDate = ptr("2026-10-07")
	b.PlannedDate, b.Status = ptr("2026-10-07"), domain.TaskPlanned
	for _, x := range []domain.Task{a, b, c} {
		if err := tasks.Create(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	for name, tc := range map[string]struct {
		f    domain.TaskFilter
		want int
	}{
		"alle":         {domain.TaskFilter{}, 3},
		"Datum":        {domain.TaskFilter{PlannedDate: "2026-10-07"}, 2},
		"Status":       {domain.TaskFilter{Status: domain.TaskPlanned}, 1},
		"Datum+Status": {domain.TaskFilter{PlannedDate: "2026-10-07", Status: domain.TaskBacklog}, 1},
		"leer":         {domain.TaskFilter{ProjectID: "nope"}, 0},
	} {
		got, err := tasks.List(ctx, tc.f)
		if err != nil || len(got) != tc.want {
			t.Errorf("%s: %d Tasks, erwartet %d (err %v)", name, len(got), tc.want, err)
		}
	}
}

func TestTaskForeignKeys(t *testing.T) {
	ctx := context.Background()
	tasks, projects := newTaskRepos(t)

	bad := newTask("x")
	bad.ProjectID = ptr("missing")
	if err := tasks.Create(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unbekanntes Projekt: %v", err)
	}

	now := time.Now()
	_ = projects.Create(ctx, domain.Project{ID: "p", Name: "P", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now})
	parent, child := newTask("parent"), newTask("child")
	parent.ProjectID, child.ProjectID, child.ParentTaskID = ptr("p"), ptr("p"), ptr("parent")
	_ = tasks.Create(ctx, parent)
	_ = tasks.Create(ctx, child)

	// Projekt löschen: Tasks bleiben, project_id wird NULL.
	if err := projects.Delete(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	got, err := tasks.Get(ctx, "child")
	if err != nil || got.ProjectID != nil {
		t.Errorf("nach Projekt-Delete: %+v, %v", got, err)
	}
	// Eltern-Task löschen: Subtasks verschwinden mit.
	if err := tasks.Delete(ctx, "parent"); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Get(ctx, "child"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Subtask blieb übrig: %v", err)
	}
}

func TestTaskTableRejectsStartWithoutDate(t *testing.T) {
	tasks, _ := newTaskRepos(t)
	x := newTask("x")
	x.PlannedStartAt = ptr(time.Now())
	if err := tasks.Create(context.Background(), x); err == nil {
		t.Error("planned_start_at ohne planned_date wurde akzeptiert")
	}
}
