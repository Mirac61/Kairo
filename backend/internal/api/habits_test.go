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

func newHabitsRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Habits: service.NewHabitService(repository.NewHabitRepository(db), time.UTC, nil),
	})
}

func TestHabitsCRUD(t *testing.T) {
	h := newHabitsRouter(t)

	rec := call(h, "POST", "/api/habits", `{"name":"Sport","frequency_type":"TIMES_PER_WEEK",
		"frequency_config":{"times":3},"preferred_time":"18:00","start_date":"2026-01-05","icon":"dumbbell"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var x habitDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &x)
	if x.FrequencyType != "TIMES_PER_WEEK" || x.FrequencyConfig.Times != 3 || !x.Active || *x.PreferredTime != "18:00" || x.DurationMinutes != 30 || x.Icon != "dumbbell" {
		t.Errorf("POST-Antwort = %+v", x)
	}

	var due []habitDueDTO
	rec = call(h, "GET", "/api/habits/due?from=2026-01-05&to=2026-01-06", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &due)
	if rec.Code != 200 || len(due) != 2 || due[1].Date != "2026-01-06" || *due[1].PreferredTime != "18:00" || due[0].DurationMinutes != 30 {
		t.Errorf("GET due = %d %s", rec.Code, rec.Body)
	}
	if rec := call(h, "GET", "/api/habits/due?from=2026-01-05", ""); rec.Code != 400 {
		t.Errorf("GET due ohne to = %d", rec.Code)
	}

	rec = call(h, "PATCH", "/api/habits/"+x.ID, `{"frequency_type":"SPECIFIC_WEEKDAYS",
		"frequency_config":{"weekdays":["fr","mo"]},"preferred_time":"","duration_minutes":45,"active":false,"icon":"bike"}`)
	if rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &x)
	if x.FrequencyType != "SPECIFIC_WEEKDAYS" || len(x.FrequencyConfig.Weekdays) != 2 || x.FrequencyConfig.Weekdays[0] != "MO" ||
		x.PreferredTime != nil || x.Active || x.Name != "Sport" || x.DurationMinutes != 45 || x.Icon != "bike" {
		t.Errorf("PATCH-Antwort = %+v", x)
	}

	var list []habitDTO
	_ = json.Unmarshal(call(h, "GET", "/api/habits", "").Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("GET list = %d", len(list))
	}
	if rec := call(h, "DELETE", "/api/habits/"+x.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "GET", "/api/habits/"+x.ID, ""); rec.Code != 404 {
		t.Errorf("GET nach DELETE = %d", rec.Code)
	}
}

func TestHabitCompletionsFlow(t *testing.T) {
	h := newHabitsRouter(t)
	var x habitDTO
	_ = json.Unmarshal(call(h, "POST", "/api/habits", `{"name":"Lesen","frequency_type":"DAILY","target_value":30,"unit":"min","start_date":"2026-01-01"}`).Body.Bytes(), &x)
	path := "/api/habits/" + x.ID + "/completions"

	rec := call(h, "POST", path, `{"date":"2026-03-02","value":25,"note":"Kapitel 3"}`)
	if rec.Code != 201 {
		t.Fatalf("POST completion = %d %s", rec.Code, rec.Body)
	}
	var c habitCompletionDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	if c.Date != "2026-03-02" || *c.Value != 25 || c.Note != "Kapitel 3" || c.HabitID != x.ID {
		t.Errorf("Completion = %+v", c)
	}
	if rec := call(h, "POST", path, `{"date":"2026-03-02"}`); rec.Code != 409 {
		t.Errorf("zweite Completion = %d, erwartet 409", rec.Code)
	}
	if rec := call(h, "POST", path, ""); rec.Code != 201 { // leerer Body = heute
		t.Errorf("Completion ohne Body = %d %s", rec.Code, rec.Body)
	}

	var list []habitCompletionDTO
	_ = json.Unmarshal(call(h, "GET", path+"?from=2026-03-01&to=2026-03-31", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].Date != "2026-03-02" {
		t.Errorf("Liste mit Fenster = %+v", list)
	}

	if rec := call(h, "DELETE", path+"/2026-03-02", ""); rec.Code != 204 {
		t.Errorf("DELETE completion = %d", rec.Code)
	}
	if rec := call(h, "DELETE", path+"/2026-03-02", ""); rec.Code != 404 {
		t.Errorf("zweites DELETE = %d", rec.Code)
	}
	if rec := call(h, "POST", path, `{"date":"2026-03-02"}`); rec.Code != 201 {
		t.Errorf("erneut abhaken = %d", rec.Code)
	}
}

func TestHabitsErrors(t *testing.T) {
	h := newHabitsRouter(t)
	var x habitDTO
	_ = json.Unmarshal(call(h, "POST", "/api/habits", `{"name":"x","frequency_type":"DAILY","start_date":"2026-01-01"}`).Body.Bytes(), &x)
	for name, tc := range map[string]struct {
		method, path, body string
		want               int
	}{
		"leerer Name":             {"POST", "/api/habits", `{"name":"","frequency_type":"DAILY"}`, 400},
		"Typ fehlt":               {"POST", "/api/habits", `{"name":"x"}`, 400},
		"falscher Typ":            {"POST", "/api/habits", `{"name":"x","frequency_type":"MONTHLY"}`, 400},
		"times fehlt":             {"POST", "/api/habits", `{"name":"x","frequency_type":"TIMES_PER_WEEK"}`, 400},
		"unbekanntes Feld":        {"POST", "/api/habits", `{"name":"x","frequency_type":"DAILY","foo":1}`, 400},
		"unbekanntes Projekt":     {"POST", "/api/habits", `{"name":"x","frequency_type":"DAILY","project_id":"nope"}`, 400},
		"unbekanntes Config-Feld": {"POST", "/api/habits", `{"name":"x","frequency_type":"DAILY","frequency_config":{"foo":1}}`, 400},
		"GET unbekannt":           {"GET", "/api/habits/nope", "", 404},
		"PATCH unbekannt":         {"PATCH", "/api/habits/nope", `{}`, 404},
		"DELETE unbekannt":        {"DELETE", "/api/habits/nope", "", 404},
		"Completion unbekannt":    {"POST", "/api/habits/nope/completions", `{}`, 404},
		"Completion Zukunft":      {"POST", "/api/habits/" + x.ID + "/completions", `{"date":"2999-01-01"}`, 400},
		"Completion Datum":        {"POST", "/api/habits/" + x.ID + "/completions", `{"date":"gestern"}`, 400},
		"Completions Filter":      {"GET", "/api/habits/" + x.ID + "/completions?from=x", "", 400},
		"Uncomplete Datum":        {"DELETE", "/api/habits/" + x.ID + "/completions/x", "", 400},
	} {
		if rec := call(h, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s: %d, erwartet %d (%s)", name, rec.Code, tc.want, rec.Body)
		}
	}
}

func TestHabitValueAndPauses(t *testing.T) {
	h := newHabitsRouter(t)
	var x habitDTO
	_ = json.Unmarshal(call(h, "POST", "/api/habits", `{"name":"Wasser","frequency_type":"DAILY","target_value":8,"remind":true,"preferred_time":"09:00","start_date":"2026-01-01"}`).Body.Bytes(), &x)
	if !x.Remind {
		t.Errorf("remind = %+v", x)
	}
	path := "/api/habits/" + x.ID + "/completions"
	call(h, "POST", path, `{"date":"2026-03-02","value":3}`)
	if rec := call(h, "POST", path, `{"date":"2026-03-02","value":8}`); rec.Code != 201 {
		t.Fatalf("Menge ändern = %d %s", rec.Code, rec.Body)
	}
	var list []habitCompletionDTO
	_ = json.Unmarshal(call(h, "GET", path, "").Body.Bytes(), &list)
	if len(list) != 1 || *list[0].Value != 8 {
		t.Errorf("Menge = %+v", list)
	}

	rec := call(h, "POST", "/api/habits/pauses", `{"habit_id":"`+x.ID+`","start_date":"2026-03-03"}`)
	var p habitPauseDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if rec.Code != 201 || p.EndDate != "2026-03-03" || *p.HabitID != x.ID {
		t.Fatalf("Pause = %d %s", rec.Code, rec.Body)
	}
	var due []habitDueDTO
	_ = json.Unmarshal(call(h, "GET", "/api/habits/due?from=2026-03-02&to=2026-03-04", "").Body.Bytes(), &due)
	if len(due) != 2 || due[0].Date != "2026-03-02" || !due[0].Done || due[1].Date != "2026-03-04" {
		t.Errorf("due mit Pause = %+v", due)
	}
	var ps []habitPauseDTO
	_ = json.Unmarshal(call(h, "GET", "/api/habits/pauses", "").Body.Bytes(), &ps)
	if len(ps) != 1 {
		t.Errorf("Pausen = %+v", ps)
	}
	if rec := call(h, "POST", "/api/habits/pauses", `{"habit_id":"gibtsnicht","start_date":"2026-03-03"}`); rec.Code != 404 {
		t.Errorf("Pause für unbekanntes Habit = %d", rec.Code)
	}
	if rec := call(h, "DELETE", "/api/habits/pauses/"+p.ID, ""); rec.Code != 204 {
		t.Errorf("Pause löschen = %d", rec.Code)
	}
	if rec := call(h, "DELETE", "/api/habits/pauses/"+p.ID, ""); rec.Code != 404 {
		t.Errorf("Pause zweimal löschen = %d", rec.Code)
	}
}
