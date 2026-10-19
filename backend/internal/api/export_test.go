package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kairo/internal/notes"
	"kairo/internal/repository"
	"kairo/internal/service"
)

func newExportRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ns, err := notes.Open(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ns.Close() })
	return NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects:  service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:     service.NewTaskService(repository.NewTaskRepository(db), nil),
		Calendar:  service.NewCalendarService(repository.NewCalendarRepository(db), time.UTC, nil),
		Habits:    service.NewHabitService(repository.NewHabitRepository(db), time.UTC, nil),
		Resources: service.NewResourceService(repository.NewResourceRepository(db), nil),
		Time:      service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), nil),
		Notes:     ns,
		Location:  time.UTC,
	})
}

func TestExport(t *testing.T) {
	h := newExportRouter(t)
	call(h, "POST", "/api/projects", `{"name":"Beispielprojekt"}`)
	call(h, "POST", "/api/tasks", `{"title":"=Abgabe, wichtig","due_at":"2026-10-07T10:00:00Z"}`)
	call(h, "POST", "/api/tasks", `{"title":"ohne Frist"}`)
	call(h, "POST", "/api/calendar/events", `{"title":"Sitzung","start_at":"2026-10-07T08:00:00Z","end_at":"2026-10-07T09:00:00Z"}`)

	rec := call(h, "GET", "/api/export.json", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("export.json = %d %v", rec.Code, rec.Header())
	}
	var out struct {
		Projects       []projectDTO       `json:"projects"`
		Tasks          []taskDTO          `json:"tasks"`
		CalendarEvents []calendarEventDTO `json:"calendar_events"`
		Habits         []habitDTO         `json:"habits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Projects) != 1 || len(out.Tasks) != 2 || len(out.CalendarEvents) != 1 || out.Habits == nil {
		t.Errorf("export.json = %s", rec.Body)
	}

	rec = call(h, "GET", "/api/export.ics", "")
	body := rec.Body.String()
	if rec.Code != 200 || strings.Count(body, "BEGIN:VEVENT") != 1 || strings.Count(body, "BEGIN:VTODO") != 1 || !strings.Contains(body, "DUE:20261007T100000Z") {
		t.Errorf("export.ics = %d %s", rec.Code, body)
	}

	rec = call(h, "GET", "/api/export.csv", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"'=Abgabe, wichtig"`) || !strings.HasSuffix(rec.Header().Get("Content-Disposition"), `.csv"`) {
		t.Errorf("export.csv = %d %s", rec.Code, rec.Body)
	}

	// Notizen als ZIP: der neue Notizordner ist leer, also eine leere, aber gültige ZIP-Datei.
	rec = call(h, "GET", "/api/export.zip", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export.zip = %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if _, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len())); err != nil {
		t.Errorf("export.zip ist kein gültiges ZIP: %v", err)
	}
}
