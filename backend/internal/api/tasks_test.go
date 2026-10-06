package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

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
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:    service.NewTaskService(repository.NewTaskRepository(db), nil),
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
