package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
)

func newProjectRepo(t *testing.T) *ProjectRepository {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewProjectRepository(db)
}

func TestProjectRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newProjectRepo(t)
	path := "~/code/personal/Kairo"
	now := time.Date(2026, 10, 6, 12, 0, 0, 123, time.UTC)
	p := domain.Project{ID: "p1", Name: "Kairo", Description: "d", LocalPath: &path,
		Status: domain.ProjectActive, Color: "blue", CreatedAt: now, UpdatedAt: now}
	if err := r.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Kairo" || got.Color != "blue" || got.LocalPath == nil || *got.LocalPath != path || !got.CreatedAt.Equal(now) {
		t.Errorf("Get = %+v", got)
	}

	p.LocalPath, p.Status = nil, domain.ProjectPaused
	if err := r.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, _ = r.Get(ctx, "p1")
	if got.LocalPath != nil || got.Status != domain.ProjectPaused {
		t.Errorf("nach Update = %+v", got)
	}

	if err := r.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, "p1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get nach Delete: %v", err)
	}
	if err := r.Delete(ctx, "p1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Delete: %v", err)
	}
	if err := r.Update(ctx, p); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update ohne Zeile: %v", err)
	}
}

func TestProjectRepositoryListSortedByName(t *testing.T) {
	ctx := context.Background()
	r := newProjectRepo(t)
	now := time.Now()
	for _, n := range []string{"beta", "Alpha", "gamma"} {
		if err := r.Create(ctx, domain.Project{ID: n, Name: n, Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	ps, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 || ps[0].Name != "Alpha" || ps[1].Name != "beta" || ps[2].Name != "gamma" {
		t.Errorf("List = %+v", ps)
	}
}

func TestProjectTableRejectsInvalidRows(t *testing.T) {
	r := newProjectRepo(t)
	now := time.Now()
	if err := r.Create(context.Background(), domain.Project{ID: "x", Name: " ", Status: domain.ProjectActive, CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Error("leerer Name wurde akzeptiert")
	}
	if err := r.Create(context.Background(), domain.Project{ID: "y", Name: "n", Status: "BOGUS", CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Error("ungültiger Status wurde akzeptiert")
	}
}
