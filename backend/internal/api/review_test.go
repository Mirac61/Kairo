package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/repository"
	"kairo/internal/service"
)

func TestReviewEndpoint(t *testing.T) {
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	projects := service.NewProjectService(repository.NewProjectRepository(db), clock)
	tasks := service.NewTaskService(repository.NewTaskRepository(db), clock)
	cal := service.NewCalendarService(repository.NewCalendarRepository(db), time.UTC, clock)
	habits := service.NewHabitService(repository.NewHabitRepository(db), time.UTC, clock)
	timeSvc := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), clock)
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: projects, Tasks: tasks.WithTimerStopper(timeSvc), Calendar: cal, Habits: habits, Time: timeSvc,
		Review: service.NewReviewService(tasks, projects, habits, cal, timeSvc, time.UTC, clock),
	})

	var p struct{ ID string }
	if err := json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"Kairo"}`).Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	call(h, "POST", "/api/calendar/events", `{"title":"Vorlesung","start_at":"2026-10-06T08:00:00Z","end_at":"2026-10-06T10:00:00Z"}`)
	a := createTask(t, h, `{"title":"A","project_id":"`+p.ID+`"}`)
	createTask(t, h, `{"title":"B","project_id":"`+p.ID+`"}`)
	call(h, "POST", "/api/habits", `{"name":"Sport","frequency_type":"DAILY","start_date":"2026-10-01"}`)
	call(h, "POST", "/api/habits/"+habitID(t, h)+"/completions", `{"date":"2026-10-06"}`)
	timer(t, h, "/api/tasks/"+a.ID+"/start", "", 200)
	now = now.Add(45 * time.Minute)
	timer(t, h, "/api/tasks/"+a.ID+"/complete", "", 200)

	rec := call(h, "GET", "/api/review?from=2026-10-05&to=2026-10-07", "")
	if rec.Code != 200 {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	var got reviewDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TrackedMinutes != 45 || got.CompletedTasks != 1 || len(got.Days) != 3 || got.Days[1].CalendarMinutes != 120 || got.Days[2].TrackedMinutes != 45 {
		t.Errorf("Review = %+v", got)
	}
	if len(got.Projects) != 1 || got.Projects[0].TotalTasks != 2 || got.Projects[0].DoneTasks != 1 || got.Projects[0].TrackedMinutes != 45 {
		t.Errorf("Projects = %+v", got.Projects)
	}
	if len(got.Habits) != 1 || got.Habits[0].Done != 1 || got.Habits[0].Streak != 1 {
		t.Errorf("Habits = %+v", got.Habits)
	}
	if rec := call(h, "GET", "/api/review?from=2026-10-09&to=2026-10-07", ""); rec.Code != 400 {
		t.Errorf("umgekehrter Bereich = %d, want 400", rec.Code)
	}
}

func habitID(t *testing.T, h http.Handler) string {
	t.Helper()
	var hs []struct{ ID string }
	if err := json.Unmarshal(call(h, "GET", "/api/habits", "").Body.Bytes(), &hs); err != nil || len(hs) == 0 {
		t.Fatal("kein Habit", err)
	}
	return hs[0].ID
}
