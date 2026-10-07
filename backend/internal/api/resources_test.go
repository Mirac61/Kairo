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

func newResourcesRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects:  service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:     service.NewTaskService(repository.NewTaskRepository(db), nil),
		Resources: service.NewResourceService(repository.NewResourceRepository(db), nil),
	})
}

func TestResourcesCRUDAndFilters(t *testing.T) {
	h := newResourcesRouter(t)
	var task taskDTO
	_ = json.Unmarshal(call(h, "POST", "/api/tasks", `{"title":"AlgoDat"}`).Body.Bytes(), &task)
	var project projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"Uni"}`).Body.Bytes(), &project)

	rec := call(h, "POST", "/api/resources", `{"task_id":"`+task.ID+`","type":"FILE",
		"target":"~/Documents/Uni/AuD_Musterprf_A.pdf","label":"Musterprüfung A"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var res resourceDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.ID == "" || res.TaskID == nil || *res.TaskID != task.ID || res.ProjectID != nil ||
		res.Type != "FILE" || res.Label != "Musterprüfung A" || res.CreatedAt == "" {
		t.Errorf("POST-Antwort = %+v", res)
	}
	call(h, "POST", "/api/resources", `{"project_id":"`+project.ID+`","type":"URL","target":"https://moodle.example.org"}`)

	var list []resourceDTO
	for query, want := range map[string]int{
		"":                          2,
		"?task_id=" + task.ID:       1,
		"?project_id=" + project.ID: 1,
		"?type=URL":                 1,
		"?task_id=nix":              0,
	} {
		rec = call(h, "GET", "/api/resources"+query, "")
		list = nil
		_ = json.Unmarshal(rec.Body.Bytes(), &list)
		if rec.Code != 200 || len(list) != want {
			t.Errorf("GET %q = %d, %d Ressourcen, erwartet %d", query, rec.Code, len(list), want)
		}
	}

	if rec := call(h, "DELETE", "/api/resources/"+res.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "DELETE", "/api/resources/"+res.ID, ""); rec.Code != 404 {
		t.Errorf("zweites DELETE = %d", rec.Code)
	}
}

func TestResourcesErrors(t *testing.T) {
	h := newResourcesRouter(t)
	for name, body := range map[string]string{
		"ohne Besitzer":    `{"type":"URL","target":"https://x.org"}`,
		"Typ NOTE":         `{"task_id":"t","type":"NOTE","target":"text"}`,
		"relativer Pfad":   `{"task_id":"t","type":"FILE","target":"a.pdf"}`,
		"unbekannte Task":  `{"task_id":"gibt-es-nicht","type":"URL","target":"https://x.org"}`,
		"unbekanntes Feld": `{"task_id":"t","type":"URL","target":"https://x.org","foo":1}`,
		"kaputtes JSON":    `{`,
	} {
		if rec := call(h, "POST", "/api/resources", body); rec.Code != 400 {
			t.Errorf("%s: POST = %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := call(h, "GET", "/api/resources?type=NOTE", ""); rec.Code != 400 {
		t.Errorf("GET mit falschem Typ = %d", rec.Code)
	}
}

func TestResourceTypeOptional(t *testing.T) {
	h := newResourcesRouter(t)
	var project projectDTO
	_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"Uni"}`).Body.Bytes(), &project)
	dir := t.TempDir()
	for target, want := range map[string]string{
		"https://moodle.example.org": "URL",
		dir:                          "FOLDER",
		dir + "/gibt-es-nicht.pdf":   "FILE",
	} {
		rec := call(h, "POST", "/api/resources", `{"project_id":"`+project.ID+`","target":"`+target+`"}`)
		var res resourceDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if rec.Code != 201 || res.Type != want || res.Target != target {
			t.Errorf("%q: %d %+v, erwartet Typ %s", target, rec.Code, res, want)
		}
	}
	// Ein gesetzter Typ gewinnt; leeres Ziel bleibt ein Fehler.
	rec := call(h, "POST", "/api/resources", `{"project_id":"`+project.ID+`","type":"FILE","target":"`+dir+`"}`)
	var res resourceDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 201 || res.Type != "FILE" {
		t.Errorf("expliziter Typ: %d %+v", rec.Code, res)
	}
	if rec := call(h, "POST", "/api/resources", `{"project_id":"`+project.ID+`","target":" "}`); rec.Code != 400 {
		t.Errorf("leeres Ziel = %d, erwartet 400", rec.Code)
	}
}
