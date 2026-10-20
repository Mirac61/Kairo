package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
	"kairo/internal/repository"
	"kairo/internal/service"
)

func TestGoalsFlow(t *testing.T) {
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := func() time.Time { return time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC) }
	habitRepo, taskRepo, times := repository.NewHabitRepository(db), repository.NewTaskRepository(db), repository.NewTimeEntryRepository(db)
	tasks := service.NewTaskService(taskRepo, now)
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Habits:   service.NewHabitService(habitRepo, time.UTC, now),
		Projects: service.NewProjectService(repository.NewProjectRepository(db), now),
		Tasks:    tasks,
		Goals:    service.NewGoalService(repository.NewGoalRepository(db), tasks, times, habitRepo, time.UTC, now),
	})
	id := func(path, body string) string {
		rec := call(h, "POST", path, body)
		if rec.Code != 201 {
			t.Fatalf("POST %s = %d %s", path, rec.Code, rec.Body)
		}
		var x struct{ ID string }
		_ = json.Unmarshal(rec.Body.Bytes(), &x)
		return x.ID
	}

	if rec := call(h, "GET", "/api/goals/today", ""); rec.Code != 200 || rec.Body.String() != "null\n" {
		t.Errorf("today ohne Ziele = %d %q", rec.Code, rec.Body)
	}
	if rec := call(h, "POST", "/api/goals", `{"title":"Fit","kind":"HABIT_COUNT"}`); rec.Code != 400 {
		t.Errorf("HABIT_COUNT ohne habit_id = %d", rec.Code)
	}

	habit := id("/api/habits", `{"name":"Sport","frequency_type":"DAILY","start_date":"2026-01-01"}`)
	for _, d := range []string{"2026-01-03", "2026-01-08", "2026-01-09"} {
		call(h, "POST", "/api/habits/"+habit+"/completions", `{"date":"`+d+`"}`)
	}
	g := id("/api/goals", `{"title":"Fit","why":"Für mich","kind":"HABIT_COUNT","habit_id":"`+habit+`","target":4,"start_date":"2026-01-05"}`)

	project := id("/api/projects", `{"name":"Kairo"}`)
	t1 := id("/api/tasks", `{"title":"a","project_id":"`+project+`"}`)
	id("/api/tasks", `{"title":"b","project_id":"`+project+`"}`)
	call(h, "PATCH", "/api/tasks/"+t1, `{"status":"COMPLETED"}`)
	rel := id("/api/goals", `{"title":"Release","kind":"PROJECT_TASKS","project_id":"`+project+`","target":9}`)

	// 90 Minuten vor dem Start zählen nicht, 3 h danach schon (Timer läuft noch: bis jetzt).
	for _, e := range []struct{ start, end time.Time }{
		{time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC), time.Date(2026, 1, 4, 11, 30, 0, 0, time.UTC)},
		{time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC), time.Time{}},
	} {
		te := domain.TimeEntry{ID: domain.NewID(), ProjectID: &project, StartedAt: e.start, Source: domain.SourceManual}
		if !e.end.IsZero() {
			te.EndedAt = &e.end
		}
		if err := times.Create(context.Background(), te); err != nil {
			t.Fatal(err)
		}
	}
	hours := id("/api/goals", `{"title":"Stunden","kind":"PROJECT_HOURS","project_id":"`+project+`","target":10,"start_date":"2026-01-05"}`)
	var hv goalDTO
	_ = json.Unmarshal(call(h, "PATCH", "/api/goals/"+hours, `{"archived":true}`).Body.Bytes(), &hv)
	if p := hv.Progress; p == nil || p.Current != 3 || p.Target != 10 || p.Percent != 30 {
		t.Errorf("PROJECT_HOURS = %+v", p)
	}
	call(h, "DELETE", "/api/goals/"+hours, "")

	var list []goalDTO
	_ = json.Unmarshal(call(h, "GET", "/api/goals", "").Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("GET list = %+v", list)
	}
	// Gleiches created_at: die Reihenfolge hängt an der ID.
	if list[0].ID != g {
		list[0], list[1] = list[1], list[0]
	}
	if p := list[0].Progress; p == nil || p.Current != 2 || p.Target != 4 || p.Percent != 50 || list[0].Why != "Für mich" {
		t.Errorf("HABIT_COUNT = %+v %+v", list[0], p)
	}
	if p := list[1].Progress; p == nil || p.Current != 1 || p.Target != 2 || p.Percent != 50 || list[1].Target != 0 {
		t.Errorf("PROJECT_TASKS = %+v %+v", list[1], p)
	}

	// Archivierte Ziele kommen nicht als Ziel des Tages.
	if rec := call(h, "PATCH", "/api/goals/"+rel, `{"archived":true,"image_path":"Ziele/x.jpg"}`); rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	var today goalDTO
	_ = json.Unmarshal(call(h, "GET", "/api/goals/today", "").Body.Bytes(), &today)
	if today.ID != g {
		t.Errorf("today = %+v, want %s", today, g)
	}
	if rec := call(h, "DELETE", "/api/goals/"+g, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "PATCH", "/api/goals/"+g, `{"title":"x"}`); rec.Code != 404 {
		t.Errorf("PATCH nach DELETE = %d", rec.Code)
	}
}
