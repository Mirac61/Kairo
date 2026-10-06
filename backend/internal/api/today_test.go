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

func newTodayRouter(t *testing.T, now time.Time) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	clock := func() time.Time { return now }
	loc := time.UTC
	tasks := service.NewTaskService(repository.NewTaskRepository(db), clock)
	cal := service.NewCalendarService(repository.NewCalendarRepository(db), loc, clock)
	habits := service.NewHabitService(repository.NewHabitRepository(db), loc, clock)
	timeSvc := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), clock)
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Tasks:    tasks.WithTimerStopper(timeSvc),
		Calendar: cal,
		Habits:   habits,
		Time:     timeSvc,
		Today:    service.NewTodayService(tasks, cal, habits, timeSvc, loc, clock),
	})
}

func TestTodayEndpoint(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) // Mittwoch
	h := newTodayRouter(t, now)

	call(h, "POST", "/api/calendar/events", `{"title":"Vorlesung","start_at":"2026-10-07T08:30:00Z","end_at":"2026-10-07T10:00:00Z"}`)
	call(h, "POST", "/api/calendar/events", `{"title":"Serie","start_at":"2026-09-30T13:00:00Z","end_at":"2026-09-30T14:00:00Z",
		"recurrence_rule":"FREQ=WEEKLY;BYDAY=WE"}`)
	call(h, "POST", "/api/calendar/events", `{"title":"Morgen","start_at":"2026-10-08T08:00:00Z","end_at":"2026-10-08T09:00:00Z"}`)
	a := createTask(t, h, `{"title":"Musterprüfung","estimated_minutes":90,"planned_date":"2026-10-07","planned_start_at":"2026-10-07T10:15:00Z"}`)
	createTask(t, h, `{"title":"andere Woche","planned_date":"2026-10-14"}`)
	b := createTask(t, h, `{"title":"läuft woanders","status":"PLANNED"}`)
	call(h, "POST", "/api/habits", `{"name":"Sport","frequency_type":"DAILY","preferred_time":"18:00","start_date":"2026-10-01"}`)
	call(h, "POST", "/api/habits", `{"name":"Lesen","frequency_type":"SPECIFIC_WEEKDAYS","frequency_config":{"weekdays":["MON"]},"start_date":"2026-10-01"}`)
	timer(t, h, "/api/tasks/"+b.ID+"/start", "", 200)

	rec := call(h, "GET", "/api/today?date=2026-10-07", "")
	if rec.Code != 200 {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	var got todayDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Date != "2026-10-07" || got.Timezone != "UTC" || got.DayStart != "2026-10-07T00:00:00Z" || got.DayEnd != "2026-10-08T00:00:00Z" {
		t.Errorf("Kopf = %+v", got)
	}
	if len(got.Events) != 2 || got.Events[0].Title != "Vorlesung" || got.Events[1].Title != "Serie" ||
		got.Events[1].OccurrenceStart != "2026-10-07T13:00:00Z" || got.Events[1].StartAt != "2026-09-30T13:00:00Z" {
		t.Errorf("Events = %+v", got.Events)
	}
	if len(got.Tasks) != 1 || got.Tasks[0].ID != a.ID || got.PlannedMinutes != 90 {
		t.Errorf("Tasks = %+v, geplant %d", got.Tasks, got.PlannedMinutes)
	}
	if len(got.ActiveTasks) != 1 || got.ActiveTasks[0].ID != b.ID || got.Running == nil || *got.Running.TaskID != b.ID {
		t.Errorf("ActiveTasks = %+v, Running = %+v", got.ActiveTasks, got.Running)
	}
	if len(got.Habits) != 1 || got.Habits[0].Name != "Sport" || got.Habits[0].Done {
		t.Errorf("Habits = %+v", got.Habits)
	}
	if got.CalendarMinutes != 90+60 || got.TrackedMinutes != 0 {
		t.Errorf("Minuten: Kalender %d, erfasst %d", got.CalendarMinutes, got.TrackedMinutes)
	}

	// Ohne date gilt der Tag der Uhr; abgehakte Habits melden done.
	call(h, "POST", "/api/habits/"+got.Habits[0].ID+"/completions", `{"date":"2026-10-07"}`)
	rec = call(h, "GET", "/api/today", "")
	got = todayDTO{}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Date != "2026-10-07" || len(got.Habits) != 1 || !got.Habits[0].Done {
		t.Errorf("heute = %s, Habits %+v", got.Date, got.Habits)
	}
}

func TestTodayEmptyDayAndErrors(t *testing.T) {
	h := newTodayRouter(t, time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	rec := call(h, "GET", "/api/today?date=2030-01-01", "")
	if rec.Code != 200 {
		t.Fatalf("GET = %d", rec.Code)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"events", "tasks", "active_tasks", "habits"} {
		if l, ok := raw[k].([]any); !ok || len(l) != 0 {
			t.Errorf("%s = %v, erwartet []", k, raw[k])
		}
	}
	if raw["running_time_entry"] != nil {
		t.Errorf("running_time_entry = %v", raw["running_time_entry"])
	}
	if rec := call(h, "GET", "/api/today?date=morgen", ""); rec.Code != 400 {
		t.Errorf("falsches Datum = %d", rec.Code)
	}
}
