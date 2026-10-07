// Package service enthält die Geschäftslogik.
package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kairo/internal/domain"
)

// ProjectStore ist der Speicher, den der ProjectService braucht.
type ProjectStore interface {
	Create(ctx context.Context, p domain.Project) error
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
	Update(ctx context.Context, p domain.Project) error
	Delete(ctx context.Context, id string) error
}

// ProjectService enthält die Regeln für Projekte.
type ProjectService struct {
	publisher
	store ProjectStore
	now   func() time.Time
}

// NewProjectService erzeugt einen ProjectService. now ist die Uhr (nil = time.Now).
func NewProjectService(store ProjectStore, now func() time.Time) *ProjectService {
	if now == nil {
		now = time.Now
	}
	return &ProjectService{store: store, now: now}
}

// CreateProjectInput sind die Felder beim Anlegen. Status ist standardmäßig ACTIVE, die
// Farbe die am wenigsten benutzte der Palette.
type CreateProjectInput struct {
	Name        string
	Description string
	LocalPath   *string
	Status      domain.ProjectStatus
	Color       string
}

// UpdateProjectInput ändert nur die gesetzten (non-nil) Felder.
// Ein leerer LocalPath entfernt die Verknüpfung zum Ordner.
type UpdateProjectInput struct {
	Name        *string
	Description *string
	LocalPath   *string
	Status      *domain.ProjectStatus
	Color       *string
}

func (s *ProjectService) Create(ctx context.Context, in CreateProjectInput) (domain.Project, error) {
	if in.Status == "" {
		in.Status = domain.ProjectActive
	}
	now := s.now().UTC()
	p := domain.Project{
		ID:          domain.NewID(),
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		LocalPath:   trimOrNil(in.LocalPath),
		Status:      in.Status,
		Color:       in.Color,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if p.Color == "" {
		var err error
		if p.Color, err = s.nextColor(ctx); err != nil {
			return domain.Project{}, err
		}
	}
	if err := validateProject(p); err != nil {
		return domain.Project{}, err
	}
	if p.LocalPath != nil {
		if err := checkLocalPath(*p.LocalPath); err != nil {
			return domain.Project{}, err
		}
	}
	if err := s.store.Create(ctx, p); err != nil {
		return domain.Project{}, err
	}
	s.emit(domain.Event{Type: domain.EventProjectCreated, ID: p.ID})
	return p, nil
}

func (s *ProjectService) Get(ctx context.Context, id string) (domain.Project, error) {
	return s.store.Get(ctx, id)
}

func (s *ProjectService) List(ctx context.Context) ([]domain.Project, error) {
	return s.store.List(ctx)
}

func (s *ProjectService) Update(ctx context.Context, id string, in UpdateProjectInput) (domain.Project, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.LocalPath != nil {
		old := p.LocalPath
		p.LocalPath = trimOrNil(in.LocalPath)
		// Nur Änderungen prüfen: ein inzwischen verschwundener Ordner soll das Bearbeiten nicht sperren.
		if p.LocalPath != nil && (old == nil || *old != *p.LocalPath) {
			if err := checkLocalPath(*p.LocalPath); err != nil {
				return domain.Project{}, err
			}
		}
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	if in.Color != nil {
		p.Color = *in.Color
	}
	if err := validateProject(p); err != nil {
		return domain.Project{}, err
	}
	p.UpdatedAt = s.now().UTC()
	if err := s.store.Update(ctx, p); err != nil {
		return domain.Project{}, err
	}
	s.emit(domain.Event{Type: domain.EventProjectUpdated, ID: p.ID})
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventProjectDeleted, ID: id})
	return nil
}

// nextColor liefert die am wenigsten benutzte Farbe der Palette (bei Gleichstand die erste).
func (s *ProjectService) nextColor(ctx context.Context) (string, error) {
	ps, err := s.store.List(ctx)
	if err != nil {
		return "", err
	}
	used := map[string]int{}
	for _, p := range ps {
		used[p.Color]++
	}
	best := domain.ProjectColors[0]
	for _, c := range domain.ProjectColors {
		if used[c] < used[best] {
			best = c
		}
	}
	return best, nil
}

// checkLocalPath prüft, dass der Pfad absolut ist (oder mit ~ beginnt) und existiert. Gespeichert
// wird er so, wie er eingegeben wurde; die Extension löst ~ selbst auf.
func checkLocalPath(p string) error {
	full := p
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("%w: local_path: Home-Verzeichnis unbekannt", domain.ErrInvalid)
		}
		full = filepath.Join(home, p[1:])
	}
	if !filepath.IsAbs(full) {
		return fmt.Errorf("%w: local_path muss ein absoluter Pfad sein (oder mit ~ beginnen)", domain.ErrInvalid)
	}
	if _, err := os.Stat(full); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: local_path: %q gibt es nicht", domain.ErrInvalid, p)
	} else if err != nil {
		return fmt.Errorf("%w: local_path: %q nicht lesbar: %v", domain.ErrInvalid, p, err)
	}
	return nil
}

func trimOrNil(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func validateProject(p domain.Project) error {
	if p.Name == "" {
		return fmt.Errorf("%w: name darf nicht leer sein", domain.ErrInvalid)
	}
	if !p.Status.Valid() {
		return fmt.Errorf("%w: unbekannter Status %q", domain.ErrInvalid, p.Status)
	}
	if !domain.ValidProjectColor(p.Color) {
		return fmt.Errorf("%w: unbekannte Farbe %q (erlaubt: %s)", domain.ErrInvalid, p.Color, strings.Join(domain.ProjectColors, ", "))
	}
	return nil
}
