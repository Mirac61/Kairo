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

	dir := t.TempDir()
	rec := call(h, "POST", "/api/projects", `{"name":"AlgoDat","local_path":"`+dir+`"}`)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var p projectDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != "ACTIVE" || p.LocalPath == nil || *p.LocalPath != dir {
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

func TestProjectsColorsAndPath(t *testing.T) {
	h := newProjectsRouter(t)
	create := func(body string) projectDTO {
		t.Helper()
		var p projectDTO
		rec := call(h, "POST", "/api/projects", body)
		if rec.Code != 201 {
			t.Fatalf("POST %s = %d %s", body, rec.Code, rec.Body)
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		return p
	}

	// Ohne Angabe bekommt jedes neue Projekt die am wenigsten benutzte Farbe: erst alle acht verschieden.
	seen := map[string]bool{}
	var first projectDTO
	for i := 0; i < 8; i++ {
		p := create(`{"name":"P"}`)
		if seen[p.Color] || p.Color == "" {
			t.Fatalf("Farbe %q doppelt oder leer nach %d Projekten", p.Color, i)
		}
		seen[p.Color] = true
		if i == 0 {
			first = p
		}
	}
	if p := create(`{"name":"neuntes"}`); p.Color != first.Color {
		t.Errorf("9. Projekt = %q, erwartet Neustart der Palette bei %q", p.Color, first.Color)
	}
	if p := create(`{"name":"gewählt","color":"red"}`); p.Color != "red" {
		t.Errorf("gewählte Farbe = %q", p.Color)
	}

	rec := call(h, "PATCH", "/api/projects/"+first.ID, `{"color":"aqua"}`)
	var p projectDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if rec.Code != 200 || p.Color != "aqua" {
		t.Errorf("PATCH color = %d %s", rec.Code, rec.Body)
	}
	for name, c := range map[string]struct{ method, path, body string }{
		"Farbe unbekannt":    {"PATCH", "/api/projects/" + first.ID, `{"color":"magenta"}`},
		"Farbe beim Anlegen": {"POST", "/api/projects", `{"name":"x","color":"magenta"}`},
		"Pfad fehlt":         {"POST", "/api/projects", `{"name":"x","local_path":"/gibt/es/nicht"}`},
		"Pfad relativ":       {"PATCH", "/api/projects/" + first.ID, `{"local_path":"ordner"}`},
	} {
		if rec := call(h, c.method, c.path, c.body); rec.Code != 400 {
			t.Errorf("%s = %d %s, erwartet 400", name, rec.Code, rec.Body)
		}
	}
}
