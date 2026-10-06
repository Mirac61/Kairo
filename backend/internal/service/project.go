// Package service enthält die Geschäftslogik.
package service

import (
	"context"
	"fmt"
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

// CreateProjectInput sind die Felder beim Anlegen. Status ist standardmäßig ACTIVE.
type CreateProjectInput struct {
	Name        string
	Description string
	LocalPath   *string
	Status      domain.ProjectStatus
}

// UpdateProjectInput ändert nur die gesetzten (non-nil) Felder.
// Ein leerer LocalPath entfernt die Verknüpfung zum Ordner.
type UpdateProjectInput struct {
	Name        *string
	Description *string
	LocalPath   *string
	Status      *domain.ProjectStatus
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
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := validateProject(p); err != nil {
		return domain.Project{}, err
	}
	if err := s.store.Create(ctx, p); err != nil {
		return domain.Project{}, err
	}
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
		p.LocalPath = trimOrNil(in.LocalPath)
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	if err := validateProject(p); err != nil {
		return domain.Project{}, err
	}
	p.UpdatedAt = s.now().UTC()
	if err := s.store.Update(ctx, p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
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
	return nil
}
