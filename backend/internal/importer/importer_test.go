package importer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"kairo/internal/repository"
)

// Roundtrip: eine Export-Datei einlesen und die Objekte mit ihren IDs anlegen.
func TestImport(t *testing.T) {
	ctx := context.Background()
	db, err := repository.Open(ctx, filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	in := strings.NewReader(`{
		"exported_at": "2026-10-19T18:00:00Z",
		"projects": [{"id":"p1","name":"Studium","description":"","local_path":null,"github_repo":null,"status":"ACTIVE","color":"blue","created_at":"2026-10-01T08:00:00Z","updated_at":"2026-10-01T08:00:00Z"}],
		"tasks": [
			{"id":"t1","title":"Mutter","description":"","status":"PLANNED","priority":"HIGH","estimated_minutes":60,"due_at":null,"planned_date":"2026-10-20","planned_start_at":"2026-10-20T09:00:00Z","project_id":"p1","parent_task_id":null,"recurrence_rule":null,"created_at":"2026-10-01T08:00:00Z","updated_at":"2026-10-01T08:00:00Z","completed_at":null},
			{"id":"t2","title":"Kind","description":"","status":"BACKLOG","priority":"MEDIUM","estimated_minutes":30,"due_at":null,"planned_date":null,"planned_start_at":null,"project_id":"p1","parent_task_id":"t1","recurrence_rule":null,"created_at":"2026-10-02T08:00:00Z","updated_at":"2026-10-02T08:00:00Z","completed_at":null}
		],
		"habits": [{"id":"h1","name":"Lesen","description":"","frequency_type":"DAILY","frequency_config":{},"target_value":null,"unit":"","preferred_time":null,"duration_minutes":30,"start_date":"2026-10-01","end_date":null,"active":true,"created_at":"2026-10-01T08:00:00Z"}],
		"habit_completions": [{"id":"c1","habit_id":"h1","date":"2026-10-19","value":null,"completed_at":"2026-10-19T20:00:00Z","note":""}],
		"calendar_events": [{"id":"e1","title":"VL","description":"","start_at":"2026-10-20T10:00:00Z","end_at":"2026-10-20T11:30:00Z","location":"Raum 1","url":"","project_id":"p1","task_id":null,"recurrence_rule":null,"recurrence_exdates":[],"created_at":"2026-10-01T08:00:00Z","updated_at":"2026-10-01T08:00:00Z"}],
		"time_entries": [{"id":"x1","task_id":"t1","project_id":"p1","started_at":"2026-10-19T09:00:00Z","ended_at":"2026-10-19T10:00:00Z","source":"MANUAL"}],
		"resources": [{"id":"r1","task_id":"t1","project_id":null,"type":"URL","target":"https://example.org","label":"Skript","created_at":"2026-10-01T08:00:00Z"}]
	}`)

	res, err := Import(ctx, db, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Projects != 1 || res.Tasks != 2 || res.Habits != 1 || res.HabitCompletions != 1 || res.CalendarEvents != 1 || res.TimeEntries != 1 || res.Resources != 1 {
		t.Fatalf("Result = %+v", res)
	}

	// IDs erhalten, Verknüpfungen auflösbar.
	task, err := repository.NewTaskRepository(db).Get(ctx, "t1")
	if err != nil || task.ProjectID == nil || *task.ProjectID != "p1" {
		t.Fatalf("Task t1 = %+v, err=%v", task, err)
	}
	sub, err := repository.NewTaskRepository(db).Get(ctx, "t2")
	if err != nil || sub.ParentTaskID == nil || *sub.ParentTaskID != "t1" {
		t.Fatalf("Task t2 = %+v, err=%v", sub, err)
	}
}

// Eine kaputte Datei meldet einen Fehler, statt halb zu importieren.
func TestImportRejectsBadJSON(t *testing.T) {
	ctx := context.Background()
	db, err := repository.Open(ctx, filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Import(ctx, db, strings.NewReader(`{`)); err == nil {
		t.Fatal("kaputtes JSON wurde akzeptiert")
	}
}
