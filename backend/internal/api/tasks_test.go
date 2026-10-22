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

func newTasksRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:    service.NewTaskService(repository.NewTaskRepository(db), nil).WithLocation(loc),
	})
}

func TestTasksCRUDAndFilters(t *testing.T) {
	h := newTasksRouter(t)

	rec := call(h, "POST", "/api/tasks", `{"title":"Musterprüfung A","estimated_minutes":90,"priority":"HIGH",
		"planned_date":"2026-10-07","planned_start_at":"2026-10-07T08:15:00Z"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var task taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &task)
	if task.Status != "BACKLOG" || task.Priority != "HIGH" || task.EstimatedMinutes != 90 || task.CompletedAt != nil {
		t.Errorf("POST-Antwort = %+v", task)
	}
	call(h, "POST", "/api/tasks", `{"title":"andere"}`)

	var list []taskDTO
	rec = call(h, "GET", "/api/tasks?planned_date=2026-10-07", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list) != 1 || list[0].ID != task.ID {
		t.Errorf("Filter planned_date = %d %s", rec.Code, rec.Body)
	}
	rec = call(h, "GET", "/api/tasks", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Errorf("GET list = %d Tasks", len(list))
	}

	rec = call(h, "PATCH", "/api/tasks/"+task.ID, `{"status":"COMPLETED","planned_start_at":""}`)
	if rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &task)
	if task.Status != "COMPLETED" || task.CompletedAt == nil || task.PlannedStartAt != nil || task.Title != "Musterprüfung A" {
		t.Errorf("PATCH-Antwort = %+v", task)
	}

	if rec := call(h, "DELETE", "/api/tasks/"+task.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "GET", "/api/tasks/"+task.ID, ""); rec.Code != 404 {
		t.Errorf("GET nach DELETE = %d", rec.Code)
	}
}

func TestTasksWithProjectAndSubtasks(t *testing.T) {
	h := newTasksRouter(t)
	var p projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"P"}`).Body.Bytes(), &p)

	rec := call(h, "POST", "/api/tasks", `{"title":"t","project_id":"`+p.ID+`"}`)
	if rec.Code != 201 {
		t.Fatalf("POST mit Projekt = %d %s", rec.Code, rec.Body)
	}
	var parent taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &parent)
	if rec := call(h, "POST", "/api/tasks", `{"title":"sub","parent_task_id":"`+parent.ID+`"}`); rec.Code != 201 {
		t.Errorf("POST Subtask = %d %s", rec.Code, rec.Body)
	}

	var list []taskDTO
	_ = json.Unmarshal(call(h, "GET", "/api/tasks?project_id="+p.ID, "").Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("Filter project_id: %d Tasks", len(list))
	}
	call(h, "DELETE", "/api/projects/"+p.ID, "")
	rec = call(h, "GET", "/api/tasks/"+parent.ID, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &parent)
	if rec.Code != 200 || parent.ProjectID != nil {
		t.Errorf("Task nach Projekt-Delete: %d %+v", rec.Code, parent)
	}
}

func TestTasksErrors(t *testing.T) {
	h := newTasksRouter(t)
	for name, tc := range map[string]struct {
		method, path, body string
		want               int
	}{
		"leerer Titel":        {"POST", "/api/tasks", `{"title":""}`, 400},
		"unbekanntes Feld":    {"POST", "/api/tasks", `{"title":"x","foo":1}`, 400},
		"falscher Status":     {"POST", "/api/tasks", `{"title":"x","status":"NOPE"}`, 400},
		"falsches Datum":      {"POST", "/api/tasks", `{"title":"x","planned_date":"heute"}`, 400},
		"unbekanntes Projekt": {"POST", "/api/tasks", `{"title":"x","project_id":"nope"}`, 400},
		"unbekannte Eltern":   {"POST", "/api/tasks", `{"title":"x","parent_task_id":"nope"}`, 400},
		"Filter Status":       {"GET", "/api/tasks?status=NOPE", "", 400},
		"GET unbekannt":       {"GET", "/api/tasks/nope", "", 404},
		"PATCH unbekannt":     {"PATCH", "/api/tasks/nope", `{}`, 404},
		"DELETE unbekannt":    {"DELETE", "/api/tasks/nope", "", 404},
	} {
		if rec := call(h, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s: %d, erwartet %d (%s)", name, rec.Code, tc.want, rec.Body)
		}
	}
}

func TestJournalTaskImportIsPartialAndDueNextLocalDay(t *testing.T) {
	h := newTasksRouter(t)
	body := `{"titles":["Abgabe vorbereiten","Kapitel lesen"]}`
	rec := call(h, "POST", "/api/journal/2026-10-21/tasks", body)
	var imported journalTaskImportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated || imported.JournalDate != "2026-10-21" || imported.TaskCount != 2 || len(imported.Titles) != 2 {
		t.Fatalf("Import = %d %+v %s", rec.Code, imported, rec.Body)
	}

	var list []taskDTO
	if rec := call(h, "GET", "/api/tasks", ""); rec.Code != http.StatusOK {
		t.Fatalf("GET Tasks = %d %s", rec.Code, rec.Body)
	} else if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Berlin")
	if len(list) != 2 {
		t.Fatalf("Tasks = %d, erwartet 2", len(list))
	}
	for _, task := range list {
		if task.DueAt == nil {
			t.Errorf("Fälligkeit fehlt: %+v", task)
			continue
		}
		due, err := time.Parse(time.RFC3339, *task.DueAt)
		if err != nil || task.SourceJournalDate == nil || *task.SourceJournalDate != "2026-10-21" ||
			due.In(loc).Format("2006-01-02 15:04") != "2026-10-22 23:59" {
			t.Errorf("Task-Ursprung/Fälligkeit = %+v, due=%v, err=%v", task, due, err)
		}
	}

	// Gleicher Titel in anderer Schreibweise: nichts Neues.
	rec = call(h, "POST", "/api/journal/2026-10-21/tasks", `{"titles":["ABGABE vorbereiten"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("Wiederholung = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &imported)
	if imported.TaskCount != 2 || len(imported.Titles) != 2 {
		t.Errorf("Wiederholung meldet %+v statt 2", imported)
	}

	// Neuer Titel: wird zusätzlich angelegt, der Marker zählt hoch.
	rec = call(h, "POST", "/api/journal/2026-10-21/tasks", `{"titles":["Andere Task"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Teilimport = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &imported)
	if imported.TaskCount != 3 || len(imported.Titles) != 3 {
		t.Errorf("Teilimport meldet %+v statt 3", imported)
	}

	rec = call(h, "GET", "/api/journal/task-imports", "")
	var imports []journalTaskImportDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &imports)
	if rec.Code != http.StatusOK || len(imports) != 1 || imports[0].TaskCount != 3 || len(imports[0].Titles) != 3 {
		t.Errorf("Importstatus = %d %+v", rec.Code, imports)
	}
}

func TestJournalTaskImportRejectsInvalidInput(t *testing.T) {
	h := newTasksRouter(t)
	for _, tc := range []struct{ path, body string }{
		{"/api/journal/not-a-date/tasks", `{"titles":["Task"]}`},
		{"/api/journal/2026-10-21/tasks", `{"titles":[]}`},
		{"/api/journal/2026-10-21/tasks", `{"titles":[" "]}`},
	} {
		if rec := call(h, "POST", tc.path, tc.body); rec.Code != http.StatusBadRequest {
			t.Errorf("POST %s: %d %s", tc.path, rec.Code, rec.Body)
		}
	}
	if rec := call(h, "GET", "/api/journal/task-imports", ""); rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Errorf("fehlerhafte Anfragen hinterließen Marker: %d %s", rec.Code, rec.Body)
	}
}

// Wiederholung über SQLite: Regel kommt zurück, Erledigen legt die nächste an.
func TestTasksRecurrence(t *testing.T) {
	h := newTasksRouter(t)
	rec := call(h, "POST", "/api/tasks", `{"title":"Müll","planned_date":"2026-10-09","recurrence_rule":"FREQ=WEEKLY"}`)
	var task taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &task)
	if rec.Code != 201 || task.RecurrenceRule == nil || *task.RecurrenceRule != "FREQ=WEEKLY" {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	call(h, "PATCH", "/api/tasks/"+task.ID, `{"status":"COMPLETED"}`)
	var list []taskDTO
	_ = json.Unmarshal(call(h, "GET", "/api/tasks?status=PLANNED", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID == task.ID || list[0].RecurrenceRule == nil || *list[0].PlannedDate <= "2026-10-09" {
		t.Errorf("nächste = %+v", list)
	}
	if rec := call(h, "PATCH", "/api/tasks/"+task.ID, `{"recurrence_rule":"FREQ=HOURLY"}`); rec.Code != 400 {
		t.Errorf("ungültige Regel = %d", rec.Code)
	}
}

// Sammel-Änderung: eine Anfrage ändert mehrere Aufgaben.
func TestTasksBulk(t *testing.T) {
	h := newTasksRouter(t)
	var a, b taskDTO
	_ = json.Unmarshal(call(h, "POST", "/api/tasks", `{"title":"a"}`).Body.Bytes(), &a)
	_ = json.Unmarshal(call(h, "POST", "/api/tasks", `{"title":"b"}`).Body.Bytes(), &b)

	body := `{"updates":[{"id":"` + a.ID + `","planned_date":"2026-10-07"},{"id":"` + b.ID + `","priority":"HIGH"}]}`
	rec := call(h, "POST", "/api/tasks/bulk", body)
	var res struct {
		Updated int `json:"updated"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 200 || res.Updated != 2 {
		t.Fatalf("bulk = %d %s", rec.Code, rec.Body)
	}
	var list []taskDTO
	_ = json.Unmarshal(call(h, "GET", "/api/tasks", "").Body.Bytes(), &list)
	if list[0].PlannedDate == nil || *list[0].PlannedDate != "2026-10-07" || list[1].Priority != "HIGH" {
		t.Errorf("nach bulk = %+v", list)
	}
	if rec := call(h, "POST", "/api/tasks/bulk", `{"updates":[]}`); rec.Code != 400 {
		t.Errorf("leere updates = %d", rec.Code)
	}
	// Bricht es mittendrin ab, sagt die Meldung, wie viele schon geändert sind.
	rec = call(h, "POST", "/api/tasks/bulk", `{"updates":[{"id":"`+a.ID+`","priority":"LOW"},{"id":"fehlt","priority":"LOW"}]}`)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "1 von 2 geändert") {
		t.Errorf("teilweise = %d %s", rec.Code, rec.Body)
	}
}
