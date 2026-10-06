package repository

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kairo/internal/domain"
)

func newCalendarRepos(t *testing.T) (*CalendarRepository, *TaskRepository, *ProjectRepository) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewCalendarRepository(db), NewTaskRepository(db), NewProjectRepository(db)
}

func newEvent(id string) domain.CalendarEvent {
	start := time.Date(2026, 10, 7, 6, 30, 0, 0, time.UTC)
	return domain.CalendarEvent{ID: id, Title: id, StartAt: start, EndAt: start.Add(90 * time.Minute),
		CreatedAt: start, UpdatedAt: start}
}

func TestCalendarRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	cal, _, _ := newCalendarRepos(t)

	e := newEvent("e1")
	e.Location, e.URL = "HS 1", "https://example.org"
	e.RecurrenceRule = ptr("FREQ=WEEKLY;BYDAY=WE")
	e.RecurrenceExdates = []string{"2026-10-14", "2026-10-21"}
	if err := cal.Create(ctx, e); err != nil {
		t.Fatal(err)
	}
	got, err := cal.Get(ctx, "e1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Location != "HS 1" || *got.RecurrenceRule != "FREQ=WEEKLY;BYDAY=WE" ||
		!reflect.DeepEqual(got.RecurrenceExdates, e.RecurrenceExdates) || !got.EndAt.Equal(e.EndAt) {
		t.Errorf("Get = %+v", got)
	}

	e.RecurrenceRule, e.RecurrenceExdates = nil, nil
	if err := cal.Update(ctx, e); err != nil {
		t.Fatal(err)
	}
	got, _ = cal.Get(ctx, "e1")
	if got.RecurrenceRule != nil || len(got.RecurrenceExdates) != 0 {
		t.Errorf("nach Update = %+v", got)
	}

	if err := cal.Delete(ctx, "e1"); err != nil {
		t.Fatal(err)
	}
	if _, err := cal.Get(ctx, "e1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get nach Delete: %v", err)
	}
	if err := cal.Update(ctx, e); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update ohne Zeile: %v", err)
	}
}

func TestCalendarRepositoryListOrderedByStart(t *testing.T) {
	ctx := context.Background()
	cal, _, _ := newCalendarRepos(t)
	for i, id := range []string{"c", "a", "b"} {
		e := newEvent(id)
		e.StartAt = e.StartAt.Add(time.Duration([]int{2, 0, 1}[i]) * time.Hour)
		e.EndAt = e.StartAt.Add(time.Hour)
		if err := cal.Create(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	es, err := cal.List(ctx)
	if err != nil || len(es) != 3 || es[0].ID != "a" || es[1].ID != "b" || es[2].ID != "c" {
		t.Errorf("List = %+v, %v", es, err)
	}
}

func TestCalendarForeignKeys(t *testing.T) {
	ctx := context.Background()
	cal, tasks, projects := newCalendarRepos(t)

	bad := newEvent("x")
	bad.TaskID = ptr("missing")
	if err := cal.Create(ctx, bad); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unbekannte Task: %v", err)
	}

	now := time.Now()
	_ = projects.Create(ctx, domain.Project{ID: "p", Name: "P", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now})
	task := newTask("t")
	_ = tasks.Create(ctx, task)
	e := newEvent("e")
	e.ProjectID, e.TaskID = ptr("p"), ptr("t")
	if err := cal.Create(ctx, e); err != nil {
		t.Fatal(err)
	}
	// Verknüpfte Objekte löschen: Event bleibt, Verknüpfung wird NULL.
	_ = tasks.Delete(ctx, "t")
	_ = projects.Delete(ctx, "p")
	got, err := cal.Get(ctx, "e")
	if err != nil || got.ProjectID != nil || got.TaskID != nil {
		t.Errorf("nach Delete: %+v, %v", got, err)
	}
}
