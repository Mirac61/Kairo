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

type memResources struct{ m map[string]domain.Resource }

func (s *memResources) Create(_ context.Context, r domain.Resource) error { s.m[r.ID] = r; return nil }
func (s *memResources) List(context.Context, domain.ResourceFilter) ([]domain.Resource, error) {
	return nil, nil
}
func (s *memResources) Delete(_ context.Context, id string) error { delete(s.m, id); return nil }

func newResourceSvc() *ResourceService {
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	return NewResourceService(&memResources{m: map[string]domain.Resource{}}, func() time.Time { return clock })
}

func TestResourceCreateNormalizes(t *testing.T) {
	svc := newResourceSvc()
	r, err := svc.Create(context.Background(), CreateResourceInput{
		TaskID: ptr(" t1 "), ProjectID: ptr(" "), Type: domain.ResourceFile,
		Target: " ~/Documents/AuD.pdf ", Label: " Musterprüfung "})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == "" || *r.TaskID != "t1" || r.ProjectID != nil || r.Target != "~/Documents/AuD.pdf" ||
		r.Label != "Musterprüfung" || r.CreatedAt.IsZero() {
		t.Errorf("Resource = %+v", r)
	}
}

func TestResourceCreateValidation(t *testing.T) {
	svc := newResourceSvc()
	ctx := context.Background()
	for name, in := range map[string]CreateResourceInput{
		"ohne Besitzer":           {Type: domain.ResourceURL, Target: "https://x.org"},
		"zwei Besitzer":           {TaskID: ptr("t"), ProjectID: ptr("p"), Type: domain.ResourceURL, Target: "https://x.org"},
		"falscher Typ":            {TaskID: ptr("t"), Type: "NOTE", Target: "text"},
		"leeres Ziel ohne Typ":    {TaskID: ptr("t"), Target: " "},
		"relativer Pfad ohne Typ": {TaskID: ptr("t"), Target: "docs/a.pdf"},
		"leeres Ziel":             {TaskID: ptr("t"), Type: domain.ResourceFile, Target: " "},
		"URL ohne Schema":         {TaskID: ptr("t"), Type: domain.ResourceURL, Target: "moodle.example.org"},
		"URL mit file://":         {TaskID: ptr("t"), Type: domain.ResourceURL, Target: "file:///etc/passwd"},
		"URL ohne Host":           {TaskID: ptr("t"), Type: domain.ResourceURL, Target: "https://"},
		"relativer Pfad":          {TaskID: ptr("t"), Type: domain.ResourceFile, Target: "docs/a.pdf"},
		"relativer Ordner":        {ProjectID: ptr("p"), Type: domain.ResourceFolder, Target: "../x"},
		"Tilde ohne Slash":        {ProjectID: ptr("p"), Type: domain.ResourceFolder, Target: "~user/x"},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	for name, in := range map[string]CreateResourceInput{
		"absolute Datei": {TaskID: ptr("t"), Type: domain.ResourceFile, Target: "/Users/x/a.pdf"},
		"Home":           {ProjectID: ptr("p"), Type: domain.ResourceFolder, Target: "~"},
		"Home-Ordner":    {ProjectID: ptr("p"), Type: domain.ResourceFolder, Target: "~/code"},
		"http-URL":       {TaskID: ptr("t"), Type: domain.ResourceURL, Target: "http://localhost:3000/x?y=1"},
	} {
		if _, err := svc.Create(ctx, in); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResourceListRejectsUnknownType(t *testing.T) {
	if _, err := newResourceSvc().List(context.Background(), domain.ResourceFilter{Type: "NOTE"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestResourceTypeInferred(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir unter Windows
	dir := filepath.Join(home, "Uni")
	file := filepath.Join(dir, "skript.pdf")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newResourceSvc()
	for target, want := range map[string]domain.ResourceType{
		"https://moodle.example.org/kurs?id=1": domain.ResourceURL,
		"HTTP://localhost:3000":                domain.ResourceURL,
		dir:                                    domain.ResourceFolder,
		"~/Uni":                                domain.ResourceFolder,
		"~":                                    domain.ResourceFolder,
		file:                                   domain.ResourceFile,
		"~/Uni/skript.pdf":                     domain.ResourceFile,
		"~/Uni/gibt-es-nicht.pdf":              domain.ResourceFile, // Tippfehler ist kein Fehler
	} {
		r, err := svc.Create(context.Background(), CreateResourceInput{ProjectID: ptr("p"), Target: target})
		if err != nil || r.Type != want {
			t.Errorf("%q: Typ %q, err %v, erwartet %q", target, r.Type, err, want)
		} else if r.Target != target {
			t.Errorf("%q: gespeichert wurde %q", target, r.Target)
		}
	}
	// Ein gesetzter Typ gewinnt, auch gegen die Ableitung.
	r, err := svc.Create(context.Background(), CreateResourceInput{ProjectID: ptr("p"), Type: domain.ResourceFile, Target: dir})
	if err != nil || r.Type != domain.ResourceFile {
		t.Errorf("expliziter Typ: %q, %v", r.Type, err)
	}
}
