package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

func newTimeRepos(t *testing.T) (*TimeEntryRepository, *TaskRepository, *ProjectRepository, *sql.DB) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewTimeEntryRepository(db), NewTaskRepository(db), NewProjectRepository(db), db
}

var t0 = time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

func newEntry(id string, start time.Duration, length time.Duration) domain.TimeEntry {
	s := t0.Add(start)
	e := domain.TimeEntry{ID: id, StartedAt: s, Source: domain.SourceManual}
	if length > 0 {
		end := s.Add(length)
		e.EndedAt = &end
	}
	return e
}

func TestTimeEntryRepositoryRoundTripAndFilters(t *testing.T) {
	ctx := context.Background()
	entries, tasks, projects, _ := newTimeRepos(t)
	if err := projects.Create(ctx, domain.Project{ID: "p1", Name: "P", Status: domain.ProjectActive, CreatedAt: t0, UpdatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	inProject := newTask("t1")
	inProject.ProjectID = ptr("p1")
	if err := tasks.Create(ctx, inProject); err != nil {
		t.Fatal(err)
	}
	if err := tasks.Create(ctx, newTask("t2")); err != nil {
		t.Fatal(err)
	}

	a := newEntry("a", 0, time.Hour)
	a.TaskID = ptr("t1")
	// Stempel ohne Sekundenbruchteil bzw. mit: "…:05Z" liegt nach "…:05.5Z", wenn man Text vergleicht.
	b := newEntry("b", 5*time.Hour, time.Hour)
	b.TaskID, b.Source = ptr("t2"), domain.SourceVSCodium
	c := newEntry("c", 5*time.Hour+500*time.Millisecond, time.Hour)
	c.ProjectID = ptr("p1")
	running := newEntry("d", 8*time.Hour, 0)
	running.TaskID = ptr("t1")
	for _, e := range []domain.TimeEntry{a, b, c, running} {
		if err := entries.Create(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	all, err := entries.List(ctx, domain.TimeEntryFilter{})
	if err != nil || len(all) != 4 {
		t.Fatalf("List = %+v, %v", all, err)
	}
	if all[0].ID != "d" || all[1].ID != "c" || all[2].ID != "b" || all[3].ID != "a" {
		t.Errorf("Reihenfolge = %s %s %s %s, erwartet neueste zuerst d c b a", all[0].ID, all[1].ID, all[2].ID, all[3].ID)
	}
	got := all[3]
	if got.TaskID == nil || *got.TaskID != "t1" || got.ProjectID == nil || *got.ProjectID != "p1" ||
		!got.StartedAt.Equal(a.StartedAt) || got.EndedAt == nil || !got.EndedAt.Equal(*a.EndedAt) || got.Source != domain.SourceManual {
		t.Errorf("Round-Trip (wirksames Projekt aus der Task) = %+v", got)
	}
	if all[0].EndedAt != nil || all[2].Source != domain.SourceVSCodium || all[2].ProjectID != nil {
		t.Errorf("laufender/ohne Projekt = %+v / %+v", all[0], all[2])
	}

	from, to := t0.Add(4*time.Hour), t0.Add(8*time.Hour)
	for name, tc := range map[string]struct {
		f    domain.TimeEntryFilter
		want []string
	}{
		"task":        {domain.TimeEntryFilter{TaskID: "t1"}, []string{"d", "a"}},
		"projekt":     {domain.TimeEntryFilter{ProjectID: "p1"}, []string{"d", "c", "a"}},
		"laufend":     {domain.TimeEntryFilter{Running: true}, []string{"d"}},
		"ab from":     {domain.TimeEntryFilter{From: &from}, []string{"d", "c", "b"}},
		"vor to":      {domain.TimeEntryFilter{To: &to}, []string{"c", "b", "a"}},
		"from und to": {domain.TimeEntryFilter{From: &from, To: &to}, []string{"c", "b"}},
		"kombi":       {domain.TimeEntryFilter{TaskID: "t1", To: &to}, []string{"a"}},
	} {
		got, err := entries.List(ctx, tc.f)
		if err != nil || len(got) != len(tc.want) {
			t.Errorf("%s: %d Einträge, err %v, erwartet %v", name, len(got), err, tc.want)
			continue
		}
		for i, id := range tc.want {
			if got[i].ID != id {
				t.Errorf("%s: Position %d = %s, erwartet %s", name, i, got[i].ID, id)
			}
		}
	}
}

func TestTimeEntryRepositoryConstraints(t *testing.T) {
	ctx := context.Background()
	entries, tasks, _, db := newTimeRepos(t)
	if err := tasks.Create(ctx, newTask("t1")); err != nil {
		t.Fatal(err)
	}

	if err := entries.Create(ctx, newEntry("a", 0, time.Hour)); err == nil {
		t.Error("ohne Besitzer: CHECK hat nicht gegriffen")
	}
	// Das Repository speichert bei Task-Einträgen kein Projekt, also direkt per SQL prüfen.
	if _, err := db.ExecContext(ctx, `INSERT INTO time_entries (id, task_id, project_id, started_at, source)
		VALUES ('x', 't1', 'p1', '2026-10-06T09:00:00.000Z', 'MANUAL')`); err == nil {
		t.Error("zwei Besitzer: CHECK hat nicht gegriffen")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO time_entries (id, task_id, started_at, ended_at, source)
		VALUES ('y', 't1', '2026-10-06T09:00:00.000Z', '2026-10-06T08:59:59.999Z', 'MANUAL')`); err == nil {
		t.Error("Ende vor Start: CHECK hat nicht gegriffen")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO time_entries (id, task_id, started_at, source)
		VALUES ('z', 't1', '2026-10-06T09:00:00.000Z', 'NEU')`); err == nil {
		t.Error("falsche Quelle: CHECK hat nicht gegriffen")
	}

	missing := newEntry("m", 0, time.Hour)
	missing.TaskID = ptr("gibt-es-nicht")
	if err := entries.Create(ctx, missing); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unbekannte Task: %v", err)
	}
}

func TestTimeEntryRepositoryAllowsOnlyOneRunning(t *testing.T) {
	ctx := context.Background()
	entries, tasks, _, _ := newTimeRepos(t)
	for _, id := range []string{"t1", "t2"} {
		if err := tasks.Create(ctx, newTask(id)); err != nil {
			t.Fatal(err)
		}
	}
	first, second, closed := newEntry("a", 0, 0), newEntry("b", time.Hour, 0), newEntry("c", 2*time.Hour, time.Hour)
	first.TaskID, second.TaskID, closed.TaskID = ptr("t1"), ptr("t2"), ptr("t2")
	if err := entries.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := entries.Create(ctx, second); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("zweiter laufender Timer: %v, erwartet ErrConflict", err)
	}
	if err := entries.Create(ctx, closed); err != nil {
		t.Errorf("beendeter Eintrag neben laufendem Timer: %v", err)
	}
}

func TestTimeEntriesCascadeWithOwner(t *testing.T) {
	ctx := context.Background()
	entries, tasks, projects, db := newTimeRepos(t)
	if err := projects.Create(ctx, domain.Project{ID: "p1", Name: "P", Status: domain.ProjectActive, CreatedAt: t0, UpdatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	task := newTask("t1")
	task.ProjectID = ptr("p1")
	if err := tasks.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	onTask, onProject := newEntry("a", 0, time.Hour), newEntry("b", 2*time.Hour, time.Hour)
	onTask.TaskID, onProject.ProjectID = ptr("t1"), ptr("p1")
	for _, e := range []domain.TimeEntry{onTask, onProject} {
		if err := entries.Create(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	if err := tasks.Delete(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM time_entries`); n != 1 {
		t.Errorf("nach Task-Löschen: %d Einträge, erwartet 1", n)
	}
	if err := projects.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM time_entries`); n != 0 {
		t.Errorf("nach Projekt-Löschen: %d Einträge, erwartet 0", n)
	}
}

func TestTimeEntryTxCommitAndRollback(t *testing.T) {
	ctx := context.Background()
	entries, tasks, _, _ := newTimeRepos(t)
	if err := tasks.Create(ctx, newTask("t1")); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")

	err := entries.Tx(ctx, func(tx service.TimeTx) error {
		task, err := tx.GetTask(ctx, "t1")
		if err != nil {
			return err
		}
		task.Status, task.UpdatedAt = domain.TaskInProgress, t0
		if err := tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		e := newEntry("a", 0, 0)
		e.TaskID = ptr("t1")
		if err := tx.CreateEntry(ctx, e); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx-Fehler = %v", err)
	}
	if got, _ := tasks.Get(ctx, "t1"); got.Status != domain.TaskBacklog {
		t.Errorf("Task nach Rollback = %s", got.Status)
	}
	if all, _ := entries.List(ctx, domain.TimeEntryFilter{}); len(all) != 0 {
		t.Errorf("Eintrag nach Rollback: %+v", all)
	}

	err = entries.Tx(ctx, func(tx service.TimeTx) error {
		e := newEntry("a", 0, 0)
		e.TaskID = ptr("t1")
		if err := tx.CreateEntry(ctx, e); err != nil {
			return err
		}
		running, err := tx.RunningEntry(ctx)
		if err != nil || running == nil || running.ID != "a" {
			t.Errorf("RunningEntry = %+v, %v", running, err)
		}
		return tx.CloseEntry(ctx, "a", t0.Add(time.Hour))
	})
	if err != nil {
		t.Fatal(err)
	}
	all, _ := entries.List(ctx, domain.TimeEntryFilter{})
	if len(all) != 1 || all[0].EndedAt == nil || !all[0].EndedAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("nach Commit = %+v", all)
	}
	err = entries.Tx(ctx, func(tx service.TimeTx) error {
		if running, err := tx.RunningEntry(ctx); err != nil || running != nil {
			t.Errorf("RunningEntry ohne Timer = %+v, %v", running, err)
		}
		return tx.CloseEntry(ctx, "a", t0)
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("bereits beendeten Eintrag erneut beenden: %v", err)
	}
}
