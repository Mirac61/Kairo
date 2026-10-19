// Package service enthält die Geschäftslogik.
package service

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kairo/internal/domain"
	"kairo/internal/github"
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
	GitHubRepo  *string
	Status      domain.ProjectStatus
	Color       string
}

// UpdateProjectInput ändert nur die gesetzten (non-nil) Felder.
// Ein leerer LocalPath oder GitHubRepo entfernt die Verknüpfung.
type UpdateProjectInput struct {
	Name        *string
	Description *string
	LocalPath   *string
	GitHubRepo  *string
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
		GitHubRepo:  trimOrNil(in.GitHubRepo),
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
	if err := normalizeRepo(p.GitHubRepo); err != nil {
		return domain.Project{}, err
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
		p.LocalPath = trimOrNil(in.LocalPath)
		if p.LocalPath != nil {
			if err := checkLocalPath(*p.LocalPath); err != nil {
				return domain.Project{}, err
			}
		}
	}
	if in.GitHubRepo != nil {
		p.GitHubRepo = trimOrNil(in.GitHubRepo)
		if err := normalizeRepo(p.GitHubRepo); err != nil {
			return domain.Project{}, err
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

// checkLocalPath prüft nur die Form: absolut oder mit ~ beginnend. Ob der Ordner existiert, weiß nur
// der Client (Server z. B. auf dem Pi, Ordner auf dem Mac); die Extension löst ~ selbst auf.
// "/" gilt auf jedem System, wie bei Ressourcen.
func checkLocalPath(p string) error {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "/") || filepath.IsAbs(p) {
		return nil
	}
	return fmt.Errorf("%w: local_path muss ein absoluter Pfad sein (oder mit ~ beginnen)", domain.ErrInvalid)
}

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// normalizeRepo kürzt eine GitHub-URL (https://…, git@…) auf „owner/name“ und prüft die Form.
func normalizeRepo(r *string) error {
	if r == nil {
		return nil
	}
	if short := github.RepoFromRemote(*r); short != "" {
		*r = short
	}
	*r = strings.TrimSuffix(*r, ".git")
	if !repoName.MatchString(*r) {
		return fmt.Errorf("%w: github_repo muss „owner/name“ oder eine GitHub-URL sein", domain.ErrInvalid)
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
