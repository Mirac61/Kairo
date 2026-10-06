package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"kairo/internal/domain"
)

// ResourceStore ist der Speicher, den der ResourceService braucht.
type ResourceStore interface {
	Create(ctx context.Context, r domain.Resource) error
	List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, error)
	Delete(ctx context.Context, id string) error
}

// ResourceService enthält die Regeln für Ressourcen.
type ResourceService struct {
	store ResourceStore
	now   func() time.Time
}

// NewResourceService erzeugt einen ResourceService. now ist die Uhr (nil = time.Now).
func NewResourceService(store ResourceStore, now func() time.Time) *ResourceService {
	if now == nil {
		now = time.Now
	}
	return &ResourceService{store: store, now: now}
}

// CreateResourceInput sind die Felder beim Anlegen. Genau eines von TaskID
// und ProjectID muss gesetzt sein.
type CreateResourceInput struct {
	TaskID    *string
	ProjectID *string
	Type      domain.ResourceType
	Target    string
	Label     string
}

func (s *ResourceService) Create(ctx context.Context, in CreateResourceInput) (domain.Resource, error) {
	r := domain.Resource{
		ID:        domain.NewID(),
		TaskID:    trimOrNil(in.TaskID),
		ProjectID: trimOrNil(in.ProjectID),
		Type:      in.Type,
		Target:    strings.TrimSpace(in.Target),
		Label:     strings.TrimSpace(in.Label),
		CreatedAt: s.now().UTC(),
	}
	if err := validateResource(r); err != nil {
		return domain.Resource{}, err
	}
	if err := s.store.Create(ctx, r); err != nil {
		return domain.Resource{}, err
	}
	return r, nil
}

func (s *ResourceService) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, error) {
	if f.Type != "" && !f.Type.Valid() {
		return nil, fmt.Errorf("%w: unbekannter Typ %q", domain.ErrInvalid, f.Type)
	}
	return s.store.List(ctx, f)
}

func (s *ResourceService) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func validateResource(r domain.Resource) error {
	switch {
	case (r.TaskID == nil) == (r.ProjectID == nil):
		return fmt.Errorf("%w: genau eines von task_id und project_id angeben", domain.ErrInvalid)
	case !r.Type.Valid():
		return fmt.Errorf("%w: unbekannter Typ %q", domain.ErrInvalid, r.Type)
	case r.Target == "":
		return fmt.Errorf("%w: target darf nicht leer sein", domain.ErrInvalid)
	}
	if r.Type == domain.ResourceURL {
		u, err := url.Parse(r.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: target muss eine http(s)-URL sein", domain.ErrInvalid)
		}
		return nil
	}
	// Pfade sind Referenzen für Clients mit eigenem Arbeitsverzeichnis, also nie relativ.
	if !strings.HasPrefix(r.Target, "/") && r.Target != "~" && !strings.HasPrefix(r.Target, "~/") {
		return fmt.Errorf("%w: target muss ein absoluter Pfad sein (/… oder ~/…)", domain.ErrInvalid)
	}
	return nil
}
