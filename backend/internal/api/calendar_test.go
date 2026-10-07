package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
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
	// Im Papierkorb bleibt die Verknüpfung (für restore); erst das endgültige Löschen löst sie.
	call(h, "DELETE", "/api/tasks/"+task.ID+"?permanent=true", "")
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

const importICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
	"BEGIN:VEVENT\r\nUID:a@uni\r\nSUMMARY:Arzt\r\nDTSTART:20261009T120000Z\r\nDTEND:20261009T130000Z\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:w@uni\r\nSUMMARY:AlgoDat\r\nLOCATION:HS 1\r\nDTSTART:20261007T083000Z\r\nDTEND:20261007T100000Z\r\n" +
	"RRULE:FREQ=WEEKLY;BYDAY=WE;UNTIL=20261031T000000Z\r\nEXDATE:20261014T083000Z\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:d@uni\r\nSUMMARY:Ferien\r\nDTSTART;VALUE=DATE:20261224\r\nDTEND;VALUE=DATE:20261227\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:m@uni\r\nSUMMARY:Monatlich\r\nDTSTART:20261001T080000Z\r\nDTEND:20261001T090000Z\r\nRRULE:FREQ=MONTHLY\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:w@uni\r\nSUMMARY:AlgoDat verschoben\r\nRECURRENCE-ID:20261021T083000Z\r\nDTSTART:20261022T083000Z\r\nDTEND:20261022T100000Z\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestCalendarImportICSIsIdempotent(t *testing.T) {
	h := newCalendarRouter(t)
	doImport := func(ics string) importResultDTO {
		t.Helper()
		rec := call(h, "POST", "/api/calendar/import", ics)
		var res importResultDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if rec.Code != 200 {
			t.Fatalf("Import = %d %s", rec.Code, rec.Body)
		}
		return res
	}
	listEvents := func() map[string]calendarEventDTO {
		var list []calendarEventDTO
		_ = json.Unmarshal(call(h, "GET", "/api/calendar/events", "").Body.Bytes(), &list)
		byTitle := map[string]calendarEventDTO{}
		for _, e := range list {
			byTitle[e.Title] = e
		}
		return byTitle
	}

	res := doImport(importICS)
	if res.Created != 4 || res.Updated != 0 || res.Skipped != 1 || res.UnsupportedRules != 1 || len(res.Notes) != 2 {
		t.Errorf("1. Import = %+v", res)
	}
	events := listEvents()
	if len(events) != 4 || events["Monatlich"].RecurrenceRule != nil || events["Ferien"].StartAt != "2026-12-24T00:00:00Z" {
		t.Errorf("Events = %+v", events)
	}

	// Projekt am Termin setzen; ein erneuter Import ändert Felder aus der ICS, lässt das Projekt aber stehen.
	var p projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"AlgoDat"}`).Body.Bytes(), &p)
	call(h, "PATCH", "/api/calendar/events/"+events["AlgoDat"].ID, `{"project_id":"`+p.ID+`"}`)

	res = doImport(strings.Replace(importICS, "SUMMARY:Arzt", "SUMMARY:Zahnarzt", 1))
	if res.Created != 0 || res.Updated != 4 || res.Skipped != 1 {
		t.Errorf("2. Import = %+v", res)
	}
	events = listEvents()
	if _, ok := events["Zahnarzt"]; len(events) != 4 || !ok {
		t.Errorf("Duplikate oder fehlendes Update: %+v", events)
	}
	if pid := events["AlgoDat"].ProjectID; pid == nil || *pid != p.ID {
		t.Errorf("Projekt nach Re-Import: %v", pid)
	}

	q := url.Values{"from": {"2026-10-05T00:00:00Z"}, "to": {"2026-10-26T00:00:00Z"}}.Encode()
	var occ []todayEventDTO
	_ = json.Unmarshal(call(h, "GET", "/api/calendar/occurrences?"+q, "").Body.Bytes(), &occ)
	var starts []string
	for _, o := range occ {
		if o.Title == "AlgoDat" {
			starts = append(starts, o.OccurrenceStart)
		}
	}
	if want := []string{"2026-10-07T08:30:00Z", "2026-10-21T08:30:00Z"}; !reflect.DeepEqual(starts, want) {
		t.Errorf("Serie mit EXDATE = %v, want %v", starts, want)
	}
}

func TestCalendarImportICSRejects(t *testing.T) {
	h := newCalendarRouter(t)
	if rec := call(h, "POST", "/api/calendar/import", "das ist kein Kalender"); rec.Code != 400 {
		t.Errorf("kein ICS = %d, erwartet 400", rec.Code)
	}
	big := "BEGIN:VCALENDAR\r\n" + strings.Repeat("X-PAD:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\n", maxICSBytes/60) + "END:VCALENDAR\r\n"
	if rec := call(h, "POST", "/api/calendar/import", big); rec.Code != 413 {
		t.Errorf("zu groß = %d, erwartet 413", rec.Code)
	}
	// Gleiche Absicherung wie die anderen Schreib-Endpunkte: ohne Origin oder Token kein Import.
	req := httptest.NewRequest("POST", "/api/calendar/import", strings.NewReader(importICS))
	req.Host = "127.0.0.1:8742"
	req.Header.Set("Content-Type", "text/calendar")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("ohne Origin/Token = %d, erwartet 403", rec.Code)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("mit Token = %d %s", rec.Code, rec.Body)
	}
}
