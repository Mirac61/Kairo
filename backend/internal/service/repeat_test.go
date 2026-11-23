package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
	"kairo/internal/repository"
	"kairo/internal/service"
)

func TestPlanRepetitions(t *testing.T) {
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
	// 00:30 Ortszeit am Tag der Zeitumstellung, in UTC noch der Vortag.
	clock := time.Date(2026, 10, 25, 0, 30, 0, 0, loc)
	now := func() time.Time { return clock }
	tasks := service.NewTaskService(repository.NewTaskRepository(db), now).WithLocation(loc)
	proj, err := service.NewProjectService(repository.NewProjectRepository(db), now).
		Create(ctx, service.CreateProjectInput{Name: "Analysis"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := tasks.Create(ctx, service.CreateTaskInput{Title: "5 Min erklären: Mathe 2", ProjectID: &proj.ID})
	if err != nil {
		t.Fatal(err)
	}

	got, err := tasks.PlanRepetitions(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-26", "2026-10-28", "2026-11-01", "2026-11-08"}
	if len(got) != len(want) {
		t.Fatalf("%d Wiederholungen, want %d", len(got), len(want))
	}
	for i, r := range got {
		if *r.PlannedDate != want[i] || r.Title != "Wiederholen: Mathe 2" || r.ProjectID == nil ||
			*r.ProjectID != proj.ID || r.Status != domain.TaskPlanned {
			t.Errorf("Wiederholung %d = %+v", i, r)
		}
	}

	if again, err := tasks.PlanRepetitions(ctx, src.ID); err != nil || len(again) != 0 {
		t.Errorf("zweiter Aufruf am selben Tag = %d, %v; want 0", len(again), err)
	}
	if _, err := tasks.PlanRepetitions(ctx, got[0].ID); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Wiederholung wiederholen = %v, want ErrInvalid", err)
	}
	clock = clock.AddDate(0, 0, 5)
	if later, err := tasks.PlanRepetitions(ctx, src.ID); err != nil || len(later) != 4 {
		t.Errorf("neue Runde an späterem Tag = %d, %v; want 4", len(later), err)
	}
}
