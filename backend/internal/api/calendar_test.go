package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/repository"
	"kairo/internal/service"
)

func newCalendarRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:    service.NewTaskService(repository.NewTaskRepository(db), nil),
		Calendar: service.NewCalendarService(repository.NewCalendarRepository(db), time.UTC, nil),
	})
}

func TestCalendarCRUDAndWindow(t *testing.T) {
	h := newCalendarRouter(t)

	rec := call(h, "POST", "/api/calendar/events", `{"title":"AlgoDat Vorlesung","location":"HS 1",
		"start_at":"2026-10-07T08:30:00Z","end_at":"2026-10-07T10:00:00Z",
		"recurrence_rule":"FREQ=WEEKLY;BYDAY=WE","recurrence_exdates":["2026-10-14"]}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var e calendarEventDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.RecurrenceRule == nil || *e.RecurrenceRule != "FREQ=WEEKLY;BYDAY=WE" || len(e.RecurrenceExdates) != 1 || e.Location != "HS 1" {
		t.Errorf("POST-Antwort = %+v", e)
	}
	call(h, "POST", "/api/calendar/events", `{"title":"Arzt","start_at":"2026-10-09T12:00:00Z","end_at":"2026-10-09T13:00:00Z"}`)

	window := func(from, to string) int {
		q := url.Values{"from": {from}, "to": {to}}.Encode()
		rec := call(h, "GET", "/api/calendar/events?"+q, "")
		var list []calendarEventDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &list)
		if rec.Code != 200 {
			t.Fatalf("GET Fenster = %d %s", rec.Code, rec.Body)
		}
		return len(list)
	}
	if n := window("2026-10-21T00:00:00Z", "2026-10-22T00:00:00Z"); n != 1 {
		t.Errorf("Serie in Fenster: %d", n)
	}
	if n := window("2026-10-14T00:00:00Z", "2026-10-15T00:00:00Z"); n != 0 {
		t.Errorf("ausgelassener Tag: %d", n)
	}
	var all []calendarEventDTO
	_ = json.Unmarshal(call(h, "GET", "/api/calendar/events", "").Body.Bytes(), &all)
	if len(all) != 2 {
		t.Errorf("ohne Fenster: %d", len(all))
	}

	rec = call(h, "PATCH", "/api/calendar/events/"+e.ID, `{"recurrence_rule":"","title":"Einmalig"}`)
	if rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.RecurrenceRule != nil || len(e.RecurrenceExdates) != 0 || e.Title != "Einmalig" {
		t.Errorf("PATCH-Antwort = %+v", e)
	}

	if rec := call(h, "DELETE", "/api/calendar/events/"+e.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "GET", "/api/calendar/events/"+e.ID, ""); rec.Code != 404 {
		t.Errorf("GET nach DELETE = %d", rec.Code)
	}
}

func TestCalendarLinksToTask(t *testing.T) {
	h := newCalendarRouter(t)
	var task taskDTO
	_ = json.Unmarshal(call(h, "POST", "/api/tasks", `{"title":"t"}`).Body.Bytes(), &task)

	rec := call(h, "POST", "/api/calendar/events", `{"title":"e","task_id":"`+task.ID+`",
		"start_at":"2026-10-07T08:00:00Z","end_at":"2026-10-07T09:00:00Z"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var e calendarEventDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	call(h, "DELETE", "/api/tasks/"+task.ID, "")
	rec = call(h, "GET", "/api/calendar/events/"+e.ID, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != 200 || e.TaskID != nil {
		t.Errorf("Event nach Task-Delete: %d %+v", rec.Code, e)
	}
}

func TestCalendarErrors(t *testing.T) {
	h := newCalendarRouter(t)
	const times = `"start_at":"2026-10-07T08:00:00Z","end_at":"2026-10-07T09:00:00Z"`
	for name, tc := range map[string]struct {
		method, path, body string
		want               int
	}{
		"leerer Titel":     {"POST", "/api/calendar/events", `{"title":"",` + times + `}`, 400},
		"ohne Zeiten":      {"POST", "/api/calendar/events", `{"title":"x"}`, 400},
		"Ende vor Start":   {"POST", "/api/calendar/events", `{"title":"x","start_at":"2026-10-07T09:00:00Z","end_at":"2026-10-07T08:00:00Z"}`, 400},
		"falsche Regel":    {"POST", "/api/calendar/events", `{"title":"x",` + times + `,"recurrence_rule":"FREQ=YEARLY"}`, 400},
		"unbekannte Task":  {"POST", "/api/calendar/events", `{"title":"x",` + times + `,"task_id":"nope"}`, 400},
		"unbekanntes Feld": {"POST", "/api/calendar/events", `{"title":"x",` + times + `,"foo":1}`, 400},
		"nur from":         {"GET", "/api/calendar/events?from=2026-10-07T00:00:00Z", "", 400},
		"kaputtes from":    {"GET", "/api/calendar/events?from=heute&to=morgen", "", 400},
		"GET unbekannt":    {"GET", "/api/calendar/events/nope", "", 404},
		"PATCH unbekannt":  {"PATCH", "/api/calendar/events/nope", `{}`, 404},
		"DELETE unbekannt": {"DELETE", "/api/calendar/events/nope", "", 404},
	} {
		if rec := call(h, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s: %d, erwartet %d (%s)", name, rec.Code, tc.want, rec.Body)
		}
	}
}

func TestCalendarOccurrences(t *testing.T) {
	h := newCalendarRouter(t)
	call(h, "POST", "/api/calendar/events", `{"title":"Serie","start_at":"2026-10-07T08:30:00Z","end_at":"2026-10-07T10:00:00Z",
		"recurrence_rule":"FREQ=WEEKLY;BYDAY=WE"}`)

	q := url.Values{"from": {"2026-10-05T00:00:00Z"}, "to": {"2026-10-26T00:00:00Z"}}.Encode()
	rec := call(h, "GET", "/api/calendar/occurrences?"+q, "")
	var list []todayEventDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list) != 3 || list[1].OccurrenceStart != "2026-10-14T08:30:00Z" {
		t.Errorf("3 Vorkommen erwartet: %d %s", rec.Code, rec.Body)
	}
	if rec := call(h, "GET", "/api/calendar/occurrences?from=2026-10-05T00:00:00Z", ""); rec.Code != 400 {
		t.Errorf("ohne to = %d, erwartet 400", rec.Code)
	}
}
