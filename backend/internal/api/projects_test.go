package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"kairo/internal/repository"
	"kairo/internal/service"
)

func newProjectsRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.NewProjectService(repository.NewProjectRepository(db), nil)
	return NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Projects: svc})
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8742"
	req.Header.Set("Origin", "http://127.0.0.1:8742")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestProjectsCRUD(t *testing.T) {
	h := newProjectsRouter(t)

	rec := call(h, "POST", "/api/projects", `{"name":"AlgoDat","local_path":"~/Documents/Uni/AlgoDat"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var p projectDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != "ACTIVE" || p.LocalPath == nil || *p.LocalPath != "~/Documents/Uni/AlgoDat" {
		t.Errorf("POST-Antwort = %+v", p)
	}

	if rec := call(h, "GET", "/api/projects/"+p.ID, ""); rec.Code != 200 {
		t.Errorf("GET = %d", rec.Code)
	}
	rec = call(h, "PATCH", "/api/projects/"+p.ID, `{"status":"PAUSED","local_path":""}`)
	if rec.Code != 200 {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Status != "PAUSED" || p.LocalPath != nil || p.Name != "AlgoDat" {
		t.Errorf("PATCH-Antwort = %+v", p)
	}

	rec = call(h, "GET", "/api/projects", "")
	var list []projectDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list) != 1 {
		t.Errorf("GET list = %d %s", rec.Code, rec.Body)
	}

	if rec := call(h, "DELETE", "/api/projects/"+p.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	if rec := call(h, "GET", "/api/projects/"+p.ID, ""); rec.Code != 404 {
		t.Errorf("GET nach DELETE = %d", rec.Code)
	}
}

func TestProjectsErrors(t *testing.T) {
	h := newProjectsRouter(t)
	for name, tc := range map[string]struct {
		method, path, body string
		want               int
	}{
		"leerer Name":      {"POST", "/api/projects", `{"name":""}`, 400},
		"unbekanntes Feld": {"POST", "/api/projects", `{"name":"x","foo":1}`, 400},
		"kaputtes JSON":    {"POST", "/api/projects", `{`, 400},
		"falscher Status":  {"POST", "/api/projects", `{"name":"x","status":"NOPE"}`, 400},
		"GET unbekannt":    {"GET", "/api/projects/nope", "", 404},
		"PATCH unbekannt":  {"PATCH", "/api/projects/nope", `{}`, 404},
		"DELETE unbekannt": {"DELETE", "/api/projects/nope", "", 404},
	} {
		if rec := call(h, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s: %d, erwartet %d (%s)", name, rec.Code, tc.want, rec.Body)
		}
	}
}
