package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kairo/internal/repository"
	"kairo/internal/service"
)

type trashDTO struct {
	Tasks  []taskDTO          `json:"tasks"`
	Events []calendarEventDTO `json:"events"`
	Habits []habitDTO         `json:"habits"`
}

// newTrashRouter baut alle Services gegen echtes SQLite; *now ist die verstellbare Uhr.
func newTrashRouter(t *testing.T, now *time.Time) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	clock := func() time.Time { return *now }
	tasks := service.NewTaskService(repository.NewTaskRepository(db), clock)
	cal := service.NewCalendarService(repository.NewCalendarRepository(db), time.UTC, clock)
	habits := service.NewHabitService(repository.NewHabitRepository(db), time.UTC, clock)
	timeSvc := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), clock)
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Tasks:    tasks.WithTimerStopper(timeSvc),
		Calendar: cal,
		Habits:   habits,
		Time:     timeSvc,
		Today:    service.NewTodayService(tasks, cal, habits, timeSvc, time.UTC, clock),
	})
}

func getTrash(t *testing.T, h http.Handler) trashDTO {
	t.Helper()
	rec := call(h, "GET", "/api/trash", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/trash = %d %s", rec.Code, rec.Body)
	}
	var out trashDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func wantCode(t *testing.T, h http.Handler, method, path string, want int) {
	t.Helper()
	body := ""
	if method == "PATCH" {
		body = "{}"
	}
	if rec := call(h, method, path, body); rec.Code != want {
		t.Errorf("%s %s = %d %s, erwartet %d", method, path, rec.Code, rec.Body, want)
	}
}

func TestTrashTaskWithSubtasks(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	h := newTrashRouter(t, &now)
	p := createTask(t, h, `{"title":"Eltern","planned_date":"2026-10-07"}`)
	c := createTask(t, h, `{"title":"Kind","parent_task_id":"`+p.ID+`"}`)
	g := createTask(t, h, `{"title":"Enkel","parent_task_id":"`+c.ID+`"}`)
	early := createTask(t, h, `{"title":"früher gelöscht","parent_task_id":"`+p.ID+`"}`)

	wantCode(t, h, "DELETE", "/api/tasks/"+early.ID, 204)
	now = now.Add(time.Minute)
	wantCode(t, h, "DELETE", "/api/tasks/"+p.ID, 204)

	for _, id := range []string{p.ID, c.ID, g.ID, early.ID} {
		wantCode(t, h, "GET", "/api/tasks/"+id, 404)
	}
	wantCode(t, h, "PATCH", "/api/tasks/"+p.ID, 404)
	wantCode(t, h, "POST", "/api/tasks/"+p.ID+"/start", 404)
	wantCode(t, h, "POST", "/api/tasks/"+p.ID+"/complete", 404)
	var list []taskDTO
	_ = json.Unmarshal(call(h, "GET", "/api/tasks", "").Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("Liste enthält %d gelöschte Tasks", len(list))
	}
	var today todayDTO
	_ = json.Unmarshal(call(h, "GET", "/api/today?date=2026-10-07", "").Body.Bytes(), &today)
	if len(today.Tasks) != 0 {
		t.Errorf("/api/today zeigt gelöschte Task: %+v", today.Tasks)
	}

	// Zuletzt gelöscht zuerst; Kind und Enkel stehen nicht einzeln darin.
	tr := getTrash(t, h)
	if len(tr.Tasks) != 2 || tr.Tasks[0].ID != p.ID || tr.Tasks[1].ID != early.ID || tr.Tasks[0].DeletedAt == nil {
		t.Fatalf("Papierkorb = %+v", tr.Tasks)
	}

	rec := call(h, "POST", "/api/tasks/"+p.ID+"/restore", "")
	var back taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &back)
	if rec.Code != 200 || back.ID != p.ID || back.DeletedAt != nil {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body)
	}
	wantCode(t, h, "GET", "/api/tasks/"+c.ID, 200)
	wantCode(t, h, "GET", "/api/tasks/"+g.ID, 200)
	wantCode(t, h, "GET", "/api/tasks/"+early.ID, 404) // eigener Zeitstempel: bleibt im Papierkorb
	wantCode(t, h, "POST", "/api/tasks/"+p.ID+"/restore", 200)
	wantCode(t, h, "POST", "/api/tasks/nope/restore", 404)
	if tr := getTrash(t, h); len(tr.Tasks) != 1 || tr.Tasks[0].ID != early.ID {
		t.Errorf("Papierkorb nach restore = %+v", tr.Tasks)
	}
}

func TestTrashTaskTimerAndPermanent(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	h := newTrashRouter(t, &now)
	p := createTask(t, h, `{"title":"Eltern"}`)
	c := createTask(t, h, `{"title":"Kind","parent_task_id":"`+p.ID+`"}`)

	timer(t, h, "/api/tasks/"+c.ID+"/start", "", 200)
	for _, id := range []string{p.ID, c.ID} {
		rec := call(h, "DELETE", "/api/tasks/"+id, "")
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), "pausieren") {
			t.Errorf("DELETE %s mit laufendem Timer = %d %s", id, rec.Code, rec.Body)
		}
	}
	wantCode(t, h, "GET", "/api/tasks/"+p.ID, 200)

	now = now.Add(30 * time.Minute)
	timer(t, h, "/api/tasks/"+c.ID+"/pause", "", 200)
	wantCode(t, h, "DELETE", "/api/tasks/"+p.ID, 204)

	// Die erfasste Zeit zählt weiter, auch wenn die Task im Papierkorb liegt.
	var today todayDTO
	_ = json.Unmarshal(call(h, "GET", "/api/today?date=2026-10-07", "").Body.Bytes(), &today)
	if today.TrackedMinutes != 30 || len(listEntries(t, h, "")) != 1 {
		t.Errorf("erfasst %d Min, %d Einträge", today.TrackedMinutes, len(listEntries(t, h, "")))
	}

	// Endgültig löschen schützt Zeiteinträge weiter.
	rec := call(h, "DELETE", "/api/tasks/"+p.ID+"?permanent=true", "")
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "abbrechen") {
		t.Errorf("permanent mit Zeiteinträgen = %d %s", rec.Code, rec.Body)
	}

	x := createTask(t, h, `{"title":"weg"}`)
	wantCode(t, h, "DELETE", "/api/tasks/"+x.ID, 204)
	wantCode(t, h, "DELETE", "/api/tasks/"+x.ID+"?permanent=true", 204)
	wantCode(t, h, "POST", "/api/tasks/"+x.ID+"/restore", 404)
	y := createTask(t, h, `{"title":"direkt endgültig"}`)
	wantCode(t, h, "DELETE", "/api/tasks/"+y.ID+"?permanent=true", 204)
	if tr := getTrash(t, h); len(tr.Tasks) != 1 || tr.Tasks[0].ID != p.ID {
		t.Errorf("Papierkorb = %+v", tr.Tasks)
	}
}

func TestTrashEventsAndICSReimport(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	h := newTrashRouter(t, &now)
	var e calendarEventDTO
	rec := call(h, "POST", "/api/calendar/events", `{"title":"Serie","start_at":"2026-10-07T08:00:00Z","end_at":"2026-10-07T09:00:00Z","recurrence_rule":"FREQ=WEEKLY;BYDAY=WE"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	occurrences := func() int {
		var list []map[string]any
		_ = json.Unmarshal(call(h, "GET", "/api/calendar/occurrences?from=2026-10-01T00:00:00Z&to=2026-11-01T00:00:00Z", "").Body.Bytes(), &list)
		return len(list)
	}
	if occurrences() != 4 {
		t.Fatalf("vorher %d Termine", occurrences())
	}

	wantCode(t, h, "DELETE", "/api/calendar/events/"+e.ID, 204)
	wantCode(t, h, "GET", "/api/calendar/events/"+e.ID, 404)
	wantCode(t, h, "PATCH", "/api/calendar/events/"+e.ID, 404)
	if occurrences() != 0 {
		t.Errorf("Papierkorb-Serie erscheint noch: %d Termine", occurrences())
	}
	if tr := getTrash(t, h); len(tr.Events) != 1 || tr.Events[0].ID != e.ID || tr.Events[0].DeletedAt == nil {
		t.Fatalf("Papierkorb = %+v", tr.Events)
	}
	wantCode(t, h, "POST", "/api/calendar/events/"+e.ID+"/restore", 200)
	wantCode(t, h, "POST", "/api/calendar/events/"+e.ID+"/restore", 200)
	wantCode(t, h, "POST", "/api/calendar/events/nope/restore", 404)
	if occurrences() != 4 || len(getTrash(t, h).Events) != 0 {
		t.Errorf("nach restore %d Termine", occurrences())
	}
	wantCode(t, h, "DELETE", "/api/calendar/events/"+e.ID+"?permanent=true", 204)
	wantCode(t, h, "POST", "/api/calendar/events/"+e.ID+"/restore", 404)

	// ICS: Ein Termin im Papierkorb wird beim erneuten Import weder belebt noch dupliziert.
	doImport := func() importResultDTO {
		rec := call(h, "POST", "/api/calendar/import", importICS)
		var res importResultDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if rec.Code != 200 {
			t.Fatalf("Import = %d %s", rec.Code, rec.Body)
		}
		return res
	}
	doImport()
	var list []calendarEventDTO
	_ = json.Unmarshal(call(h, "GET", "/api/calendar/events", "").Body.Bytes(), &list)
	var arzt calendarEventDTO
	for _, x := range list {
		if x.Title == "Arzt" {
			arzt = x
		}
	}
	wantCode(t, h, "DELETE", "/api/calendar/events/"+arzt.ID, 204)
	res := doImport()
	if res.Created != 0 || res.Updated != 3 || res.Skipped != 2 || !strings.Contains(strings.Join(res.Notes, "|"), `"Arzt": im Papierkorb`) {
		t.Errorf("Re-Import = %+v", res)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/calendar/events", "").Body.Bytes(), &list)
	if tr := getTrash(t, h); len(list) != 3 || len(tr.Events) != 1 || tr.Events[0].ID != arzt.ID {
		t.Errorf("%d aktive Events, Papierkorb %+v", len(list), tr.Events)
	}
	wantCode(t, h, "POST", "/api/calendar/events/"+arzt.ID+"/restore", 200)
	if res := doImport(); res.Created != 0 || res.Updated != 4 {
		t.Errorf("Import nach restore = %+v", res)
	}
}

func TestTrashHabit(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	h := newTrashRouter(t, &now)
	var x habitDTO
	rec := call(h, "POST", "/api/habits", `{"name":"Sport","frequency_type":"DAILY","start_date":"2026-10-01"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &x)
	if rec := call(h, "POST", "/api/habits/"+x.ID+"/completions", `{"date":"2026-10-07"}`); rec.Code != 201 {
		t.Fatalf("Completion = %d %s", rec.Code, rec.Body)
	}
	habitsToday := func() []todayHabitDTO {
		var today todayDTO
		_ = json.Unmarshal(call(h, "GET", "/api/today?date=2026-10-07", "").Body.Bytes(), &today)
		return today.Habits
	}

	wantCode(t, h, "DELETE", "/api/habits/"+x.ID, 204)
	wantCode(t, h, "GET", "/api/habits/"+x.ID, 404)
	wantCode(t, h, "PATCH", "/api/habits/"+x.ID, 404)
	if rec := call(h, "POST", "/api/habits/"+x.ID+"/completions", `{"date":"2026-10-06"}`); rec.Code != 404 {
		t.Errorf("Completion auf Papierkorb-Habit = %d", rec.Code)
	}
	var list []habitDTO
	_ = json.Unmarshal(call(h, "GET", "/api/habits", "").Body.Bytes(), &list)
	if len(list) != 0 || len(habitsToday()) != 0 {
		t.Errorf("gelöschtes Habit sichtbar: Liste %d, heute %d", len(list), len(habitsToday()))
	}
	if tr := getTrash(t, h); len(tr.Habits) != 1 || tr.Habits[0].ID != x.ID || tr.Habits[0].DeletedAt == nil {
		t.Fatalf("Papierkorb = %+v", tr.Habits)
	}

	wantCode(t, h, "POST", "/api/habits/"+x.ID+"/restore", 200)
	if hs := habitsToday(); len(hs) != 1 || !hs[0].Done {
		t.Errorf("nach restore: %+v (Completion muss erhalten bleiben)", hs)
	}
	wantCode(t, h, "DELETE", "/api/habits/"+x.ID+"?permanent=true", 204)
	wantCode(t, h, "POST", "/api/habits/"+x.ID+"/restore", 404)
}
