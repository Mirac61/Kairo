package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/domain"
)

type memStore struct{ m map[string]domain.Project }

func (s *memStore) Create(_ context.Context, p domain.Project) error { s.m[p.ID] = p; return nil }
func (s *memStore) Get(_ context.Context, id string) (domain.Project, error) {
	p, ok := s.m[id]
	if !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	return p, nil
}
func (s *memStore) List(context.Context) ([]domain.Project, error) { return nil, nil }
func (s *memStore) Update(_ context.Context, p domain.Project) error {
	if _, ok := s.m[p.ID]; !ok {
		return domain.ErrNotFound
	}
	s.m[p.ID] = p
	return nil
}
func (s *memStore) Delete(_ context.Context, id string) error { delete(s.m, id); return nil }

func newSvc() (*ProjectService, *time.Time) {
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	return NewProjectService(&memStore{m: map[string]domain.Project{}}, func() time.Time { return clock }), &clock
}

func ptr[T any](v T) *T { return &v }

func TestProjectCreateDefaultsAndValidation(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()

	p, err := svc.Create(ctx, CreateProjectInput{Name: "  AlgoDat  ", LocalPath: ptr("  ")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "AlgoDat" || p.Status != domain.ProjectActive || p.LocalPath != nil || p.ID == "" {
		t.Errorf("Create = %+v", p)
	}

	for name, in := range map[string]CreateProjectInput{
		"leerer Name":     {Name: "   "},
		"falscher Status": {Name: "x", Status: "NOPE"},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestProjectUpdateIsPartial(t *testing.T) {
	svc, clock := newSvc()
	ctx := context.Background()
	p, _ := svc.Create(ctx, CreateProjectInput{Name: "Kairo", Description: "d", LocalPath: ptr(t.TempDir())})

	*clock = clock.Add(time.Hour)
	got, err := svc.Update(ctx, p.ID, UpdateProjectInput{Status: ptr(domain.ProjectPaused)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Kairo" || got.Description != "d" || got.LocalPath == nil || got.Status != domain.ProjectPaused {
		t.Errorf("unveränderte Felder verloren: %+v", got)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Error("updated_at nicht fortgeschrieben")
	}

	got, _ = svc.Update(ctx, p.ID, UpdateProjectInput{LocalPath: ptr("")})
	if got.LocalPath != nil {
		t.Error("leerer local_path entfernt Verknüpfung nicht")
	}
	if _, err := svc.Update(ctx, p.ID, UpdateProjectInput{Name: ptr(" ")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("leerer Name: %v", err)
	}
	if _, err := svc.Update(ctx, "missing", UpdateProjectInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannte ID: %v", err)
	}
}

func TestProjectLocalPathMustExist(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir unter Windows
	if err := os.Mkdir(filepath.Join(home, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{"relativ": "proj", "fehlt": filepath.Join(home, "gibt-es-nicht"), "~ fehlt": "~/gibt-es-nicht"} {
		if _, err := svc.Create(ctx, CreateProjectInput{Name: "x", LocalPath: ptr(path)}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// ~ wird für die Prüfung aufgelöst, gespeichert wird die Eingabe.
	p, err := svc.Create(ctx, CreateProjectInput{Name: "x", LocalPath: ptr(" ~/proj ")})
	if err != nil || p.LocalPath == nil || *p.LocalPath != "~/proj" {
		t.Fatalf("Create mit ~ = %+v, %v", p, err)
	}

	// Geprüft wird nur eine Änderung: ein verschwundener Ordner sperrt das Bearbeiten nicht.
	if err := os.Remove(filepath.Join(home, "proj")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, p.ID, UpdateProjectInput{Name: ptr("y"), LocalPath: ptr("~/proj")}); err != nil {
		t.Errorf("unveränderter Pfad: %v", err)
	}
	if _, err := svc.Update(ctx, p.ID, UpdateProjectInput{LocalPath: ptr("~/anders")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("geänderter, fehlender Pfad: %v", err)
	}
}

func TestProjectColorValidation(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	if _, err := svc.Create(ctx, CreateProjectInput{Name: "x", Color: "pink-ish"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("unbekannte Farbe beim Anlegen: %v", err)
	}
	p, err := svc.Create(ctx, CreateProjectInput{Name: "x", Color: "red"})
	if err != nil || p.Color != "red" {
		t.Fatalf("Create = %+v, %v", p, err)
	}
	if _, err := svc.Update(ctx, p.ID, UpdateProjectInput{Color: ptr("")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("leere Farbe: %v", err)
	}
	if got, err := svc.Update(ctx, p.ID, UpdateProjectInput{Color: ptr("blue")}); err != nil || got.Color != "blue" {
		t.Errorf("Farbe ändern = %+v, %v", got, err)
	}
}
