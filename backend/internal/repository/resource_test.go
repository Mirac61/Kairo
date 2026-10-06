package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
)

func newResourceRepos(t *testing.T) (*ResourceRepository, *TaskRepository, *ProjectRepository, *sql.DB) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewResourceRepository(db), NewTaskRepository(db), NewProjectRepository(db), db
}

func newResource(id string) domain.Resource {
	return domain.Resource{ID: id, Type: domain.ResourceURL, Target: "https://example.org/" + id,
		CreatedAt: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)}
}

func TestResourceRepositoryRoundTripAndFilters(t *testing.T) {
	ctx := context.Background()
	res, tasks, projects, _ := newResourceRepos(t)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	if err := projects.Create(ctx, domain.Project{ID: "p1", Name: "P", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := tasks.Create(ctx, newTask("t1")); err != nil {
		t.Fatal(err)
	}

	a, b, c := newResource("a"), newResource("b"), newResource("c")
	a.TaskID, a.Label = ptr("t1"), "Moodle"
	b.TaskID, b.Type, b.Target = ptr("t1"), domain.ResourceFile, "~/Documents/x.pdf"
	c.ProjectID = ptr("p1")
	for _, r := range []domain.Resource{a, b, c} {
		if err := res.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	all, err := res.List(ctx, domain.ResourceFilter{})
	if err != nil || len(all) != 3 || all[0].ID != "a" || all[2].ID != "c" {
		t.Fatalf("List = %+v, %v", all, err)
	}
	if all[0].Label != "Moodle" || all[0].TaskID == nil || *all[0].TaskID != "t1" || all[0].ProjectID != nil ||
		!all[0].CreatedAt.Equal(now) {
		t.Errorf("Round-Trip = %+v", all[0])
	}
	for name, tc := range map[string]struct {
		f    domain.ResourceFilter
		want int
	}{
		"task":    {domain.ResourceFilter{TaskID: "t1"}, 2},
		"project": {domain.ResourceFilter{ProjectID: "p1"}, 1},
		"typ":     {domain.ResourceFilter{Type: domain.ResourceFile}, 1},
		"kombi":   {domain.ResourceFilter{TaskID: "t1", Type: domain.ResourceURL}, 1},
		"leer":    {domain.ResourceFilter{TaskID: "gibt-es-nicht"}, 0},
	} {
		got, err := res.List(ctx, tc.f)
		if err != nil || len(got) != tc.want {
			t.Errorf("%s: %d Ressourcen, err %v, erwartet %d", name, len(got), err, tc.want)
		}
	}

	if err := res.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := res.Delete(ctx, "a"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Delete: %v", err)
	}
}

func TestResourceRepositoryConstraints(t *testing.T) {
	ctx := context.Background()
	res, tasks, _, _ := newResourceRepos(t)
	if err := tasks.Create(ctx, newTask("t1")); err != nil {
		t.Fatal(err)
	}

	noOwner := newResource("x")
	both := newResource("y")
	both.TaskID, both.ProjectID = ptr("t1"), ptr("p1")
	badType := newResource("z")
	badType.TaskID, badType.Type = ptr("t1"), "NOTE"
	emptyTarget := newResource("w")
	emptyTarget.TaskID, emptyTarget.Target = ptr("t1"), "  "
	for name, r := range map[string]domain.Resource{
		"ohne Besitzer": noOwner, "mit zwei Besitzern": both, "falscher Typ": badType, "leeres Ziel": emptyTarget,
	} {
		if err := res.Create(ctx, r); err == nil {
			t.Errorf("%s: CHECK hat nicht gegriffen", name)
		}
	}

	missing := newResource("m")
	missing.TaskID = ptr("gibt-es-nicht")
	if err := res.Create(ctx, missing); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unbekannte Task: %v", err)
	}
}

func TestResourcesCascadeWithOwner(t *testing.T) {
	ctx := context.Background()
	res, tasks, projects, db := newResourceRepos(t)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	if err := projects.Create(ctx, domain.Project{ID: "p1", Name: "P", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	task := newTask("t1")
	task.ProjectID = ptr("p1")
	if err := tasks.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	onTask, onProject := newResource("a"), newResource("b")
	onTask.TaskID, onProject.ProjectID = ptr("t1"), ptr("p1")
	for _, r := range []domain.Resource{onTask, onProject} {
		if err := res.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	if err := tasks.Delete(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM resources`); n != 1 {
		t.Errorf("nach Task-Löschen: %d Ressourcen, erwartet 1", n)
	}
	if err := projects.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM resources`); n != 0 {
		t.Errorf("nach Projekt-Löschen: %d Ressourcen, erwartet 0", n)
	}
}
