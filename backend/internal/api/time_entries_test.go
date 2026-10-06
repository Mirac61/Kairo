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

func newTimeRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	timeSvc := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), nil)
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:    service.NewTaskService(repository.NewTaskRepository(db), nil).WithTimerStopper(timeSvc),
		Time:     timeSvc,
	})
}

func createTask(t *testing.T, h http.Handler, body string) taskDTO {
	t.Helper()
	rec := call(h, "POST", "/api/tasks", body)
	if rec.Code != 201 {
		t.Fatalf("Task anlegen = %d %s", rec.Code, rec.Body)
	}
	var task taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &task)
	return task
}

func timer(t *testing.T, h http.Handler, path, body string, wantCode int) timerDTO {
	t.Helper()
	rec := call(h, "POST", path, body)
	if rec.Code != wantCode {
		t.Fatalf("POST %s = %d %s, erwartet %d", path, rec.Code, rec.Body, wantCode)
	}
	var out timerDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func listEntries(t *testing.T, h http.Handler, query string) []timeEntryDTO {
	t.Helper()
	rec := call(h, "GET", "/api/time-entries"+query, "")
	if rec.Code != 200 {
		t.Fatalf("GET %s = %d %s", query, rec.Code, rec.Body)
	}
	var out []timeEntryDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func taskStatus(t *testing.T, h http.Handler, id string) string {
	t.Helper()
	var task taskDTO
	_ = json.Unmarshal(call(h, "GET", "/api/tasks/"+id, "").Body.Bytes(), &task)
	return task.Status
}

func TestTimerStartPauseComplete(t *testing.T) {
	h := newTimeRouter(t)
	var project projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"Uni"}`).Body.Bytes(), &project)
	a := createTask(t, h, `{"title":"A","project_id":"`+project.ID+`"}`)
	b := createTask(t, h, `{"title":"B"}`)

	// Start ohne Body: Standardquelle MANUAL, wirksames Projekt kommt von der Task.
	res := timer(t, h, "/api/tasks/"+a.ID+"/start", "", 200)
	if res.Task.Status != "IN_PROGRESS" || res.TimeEntry == nil || res.TimeEntry.EndedAt != nil ||
		res.TimeEntry.Source != "MANUAL" || *res.TimeEntry.TaskID != a.ID ||
		res.TimeEntry.ProjectID == nil || *res.TimeEntry.ProjectID != project.ID {
		t.Errorf("Start a = %+v / %+v", res.Task, res.TimeEntry)
	}
	if running := listEntries(t, h, "?running=true"); len(running) != 1 || running[0].ID != res.TimeEntry.ID {
		t.Errorf("running = %+v", running)
	}

	// Start von b pausiert a.
	res = timer(t, h, "/api/tasks/"+b.ID+"/start", `{"source":"VSCODIUM"}`, 200)
	if res.Task.ID != b.ID || res.TimeEntry.Source != "VSCODIUM" {
		t.Errorf("Start b = %+v / %+v", res.Task, res.TimeEntry)
	}
	if got := taskStatus(t, h, a.ID); got != "PAUSED" {
		t.Errorf("Task a nach Start von b = %s", got)
	}
	if running := listEntries(t, h, "?running=true"); len(running) != 1 || *running[0].TaskID != b.ID {
		t.Errorf("running = %+v", running)
	}
	if byTask := listEntries(t, h, "?task_id="+a.ID); len(byTask) != 1 || byTask[0].EndedAt == nil {
		t.Errorf("Eintrag a nicht beendet: %+v", byTask)
	}

	// Pause b, zweite Pause ist ein Konflikt.
	res = timer(t, h, "/api/tasks/"+b.ID+"/pause", "", 200)
	if res.Task.Status != "PAUSED" || res.TimeEntry == nil || res.TimeEntry.EndedAt == nil {
		t.Errorf("Pause b = %+v / %+v", res.Task, res.TimeEntry)
	}
	timer(t, h, "/api/tasks/"+b.ID+"/pause", "", 409)
	if running := listEntries(t, h, "?running=true"); len(running) != 0 {
		t.Errorf("nach Pause läuft noch: %+v", running)
	}

	// Complete: ohne laufenden Timer nur Status, mit Timer wird er beendet.
	timer(t, h, "/api/tasks/"+a.ID+"/start", "", 200)
	res = timer(t, h, "/api/tasks/"+a.ID+"/complete", "", 200)
	if res.Task.Status != "COMPLETED" || res.Task.CompletedAt == nil || res.TimeEntry == nil || res.TimeEntry.EndedAt == nil {
		t.Errorf("Complete a = %+v / %+v", res.Task, res.TimeEntry)
	}
	res = timer(t, h, "/api/tasks/"+b.ID+"/complete", "", 200)
	if res.Task.Status != "COMPLETED" || res.TimeEntry != nil {
		t.Errorf("Complete b ohne Timer = %+v / %+v", res.Task, res.TimeEntry)
	}
	if running := listEntries(t, h, "?running=true"); len(running) != 0 {
		t.Errorf("nach Complete läuft noch: %+v", running)
	}
	if all := listEntries(t, h, ""); len(all) != 3 {
		t.Errorf("%d Einträge, erwartet 3", len(all))
	}
}

func TestTimerErrors(t *testing.T) {
	h := newTimeRouter(t)
	done := createTask(t, h, `{"title":"fertig","status":"COMPLETED"}`)
	off := createTask(t, h, `{"title":"abgebrochen","status":"CANCELLED"}`)
	ok := createTask(t, h, `{"title":"offen"}`)

	timer(t, h, "/api/tasks/"+done.ID+"/start", "", 409)
	timer(t, h, "/api/tasks/"+off.ID+"/start", "", 409)
	timer(t, h, "/api/tasks/"+off.ID+"/complete", "", 409)
	for _, action := range []string{"start", "pause", "complete"} {
		timer(t, h, "/api/tasks/gibt-es-nicht/"+action, "", 404)
	}
	timer(t, h, "/api/tasks/"+ok.ID+"/start", `{"source":"ROBOTER"}`, 400)
	timer(t, h, "/api/tasks/"+ok.ID+"/start", `{"foo":1}`, 400)
	timer(t, h, "/api/tasks/"+ok.ID+"/start", `{`, 400)
	if all := listEntries(t, h, ""); len(all) != 0 {
		t.Errorf("Fehlerfälle haben Einträge angelegt: %+v", all)
	}
	// Zweimal starten ändert nichts (idempotent).
	first := timer(t, h, "/api/tasks/"+ok.ID+"/start", "", 200)
	second := timer(t, h, "/api/tasks/"+ok.ID+"/start", "", 200)
	if first.TimeEntry.ID != second.TimeEntry.ID || len(listEntries(t, h, "")) != 1 {
		t.Errorf("zweiter Start legte einen neuen Eintrag an")
	}
}

func TestPatchingRunningTaskStopsTimer(t *testing.T) {
	h := newTimeRouter(t)
	task := createTask(t, h, `{"title":"A"}`)
	timer(t, h, "/api/tasks/"+task.ID+"/start", "", 200)

	if rec := call(h, "PATCH", "/api/tasks/"+task.ID, `{"title":"umbenannt"}`); rec.Code != 200 {
		t.Fatalf("PATCH Titel = %d", rec.Code)
	}
	if running := listEntries(t, h, "?running=true"); len(running) != 1 {
		t.Fatalf("Umbenennen beendete den Timer: %+v", running)
	}
	if rec := call(h, "PATCH", "/api/tasks/"+task.ID, `{"status":"COMPLETED"}`); rec.Code != 200 {
		t.Fatalf("PATCH Status = %d %s", rec.Code, rec.Body)
	}
	all := listEntries(t, h, "")
	if len(all) != 1 || all[0].EndedAt == nil {
		t.Errorf("Timer läuft auf abgeschlossener Task weiter: %+v", all)
	}
}

func TestDeletingTaskRemovesItsEntries(t *testing.T) {
	h := newTimeRouter(t)
	task := createTask(t, h, `{"title":"A"}`)
	timer(t, h, "/api/tasks/"+task.ID+"/start", "", 200)
	if rec := call(h, "DELETE", "/api/tasks/"+task.ID, ""); rec.Code != 204 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	if all := listEntries(t, h, ""); len(all) != 0 {
		t.Errorf("Einträge der gelöschten Task: %+v", all)
	}
	// Der Timer ist frei: eine neue Task lässt sich starten.
	other := createTask(t, h, `{"title":"B"}`)
	timer(t, h, "/api/tasks/"+other.ID+"/start", "", 200)
}

func TestTimeEntriesManualCreateAndFilters(t *testing.T) {
	h := newTimeRouter(t)
	var project projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"Uni"}`).Body.Bytes(), &project)
	task := createTask(t, h, `{"title":"A","project_id":"`+project.ID+`"}`)

	rec := call(h, "POST", "/api/time-entries", `{"task_id":"`+task.ID+`",
		"started_at":"2025-03-01T09:00:00Z","ended_at":"2025-03-01T10:30:00Z"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var e timeEntryDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.ID == "" || e.Source != "MANUAL" || e.StartedAt != "2025-03-01T09:00:00Z" ||
		e.EndedAt == nil || *e.EndedAt != "2025-03-01T10:30:00Z" || *e.TaskID != task.ID {
		t.Errorf("POST-Antwort = %+v", e)
	}
	if rec := call(h, "POST", "/api/time-entries", `{"project_id":"`+project.ID+`","source":"AUTOMATIC",
		"started_at":"2025-03-02T09:00:00Z","ended_at":"2025-03-02T09:45:00Z"}`); rec.Code != 201 {
		t.Fatalf("POST Projekt = %d %s", rec.Code, rec.Body)
	}

	for query, want := range map[string]int{
		"":                           2,
		"?task_id=" + task.ID:        1,
		"?project_id=" + project.ID:  2, // Task-Eintrag zählt über das Projekt der Task
		"?from=2025-03-02T00:00:00Z": 1,
		"?to=2025-03-02T00:00:00Z":   1,
		"?from=2025-03-01T00:00:00Z&to=2025-03-03T00:00:00Z": 2,
		"?from=2025-03-01T09:00:00Z&to=2025-03-01T09:00:00Z": 0,
		"?running=false": 2,
		"?running=true":  0,
	} {
		if got := listEntries(t, h, query); len(got) != want {
			t.Errorf("GET %q = %d Einträge, erwartet %d", query, len(got), want)
		}
	}
	if got := listEntries(t, h, ""); got[0].StartedAt != "2025-03-02T09:00:00Z" {
		t.Errorf("nicht neueste zuerst: %+v", got)
	}
}

func TestTimeEntriesErrors(t *testing.T) {
	h := newTimeRouter(t)
	task := createTask(t, h, `{"title":"A"}`)
	const when = `"started_at":"2025-03-01T09:00:00Z","ended_at":"2025-03-01T10:00:00Z"`
	for name, body := range map[string]string{
		"ohne Besitzer":       `{` + when + `}`,
		"zwei Besitzer":       `{"task_id":"` + task.ID + `","project_id":"p",` + when + `}`,
		"ohne Ende":           `{"task_id":"` + task.ID + `","started_at":"2025-03-01T09:00:00Z"}`,
		"Ende vor Start":      `{"task_id":"` + task.ID + `","started_at":"2025-03-01T10:00:00Z","ended_at":"2025-03-01T09:00:00Z"}`,
		"Ende in Zukunft":     `{"task_id":"` + task.ID + `","started_at":"2025-03-01T10:00:00Z","ended_at":"2999-01-01T00:00:00Z"}`,
		"falsche Quelle":      `{"task_id":"` + task.ID + `","source":"ROBOTER",` + when + `}`,
		"unbekannte Task":     `{"task_id":"gibt-es-nicht",` + when + `}`,
		"unbekanntes Projekt": `{"project_id":"gibt-es-nicht",` + when + `}`,
		"unbekanntes Feld":    `{"task_id":"` + task.ID + `","foo":1,` + when + `}`,
		"kaputtes JSON":       `{`,
	} {
		if rec := call(h, "POST", "/api/time-entries", body); rec.Code != 400 {
			t.Errorf("%s: POST = %d %s", name, rec.Code, rec.Body)
		}
	}
	for _, query := range []string{"?from=gestern", "?to=heute", "?running=vielleicht"} {
		if rec := call(h, "GET", "/api/time-entries"+query, ""); rec.Code != 400 {
			t.Errorf("GET %s = %d", query, rec.Code)
		}
	}
	if all := listEntries(t, h, ""); len(all) != 0 {
		t.Errorf("Fehlerfälle haben Einträge angelegt: %+v", all)
	}
}
