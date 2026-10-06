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
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Habits: service.NewHabitService(repository.NewHabitRepository(db), time.UTC, nil),
	})
}

func TestHabitsCRUD(t *testing.T) {
	h := newHabitsRouter(t)

	rec := call(h, "POST", "/api/habits", `{"name":"Sport","frequency_type":"TIMES_PER_WEEK",
		"frequency_config":{"times":3},"preferred_time":"18:00","start_date":"2026-01-05"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var x habitDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &x)
	if x.FrequencyType != "TIMES_PER_WEEK" || x.FrequencyConfig.Times != 3 || !x.Active || *x.PreferredTime != "18:00" {
		t.Errorf("POST-Antwort = %+v", x)
	}

	rec = call(h, "PATCH", "/api/habits/"+x.ID, `{"frequency_type":"SPECIFIC_WEEKDAYS",
		"frequency_config":{"weekdays":["fr","mo"]},"preferred_time":"","active":false}`)
	if rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &x)
	if x.FrequencyType != "SPECIFIC_WEEKDAYS" || len(x.FrequencyConfig.Weekdays) != 2 || x.FrequencyConfig.Weekdays[0] != "MO" ||
		x.PreferredTime != nil || x.Active || x.Name != "Sport" {
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
