package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// ProjectService ist die Geschäftslogik, die die Project-Handler brauchen.
type ProjectService interface {
	Create(ctx context.Context, in service.CreateProjectInput) (domain.Project, error)
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
	Update(ctx context.Context, id string, in service.UpdateProjectInput) (domain.Project, error)
	Delete(ctx context.Context, id string) error
}

type projectDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	LocalPath   *string `json:"local_path"`
	GitHubRepo  *string `json:"github_repo"`
	Status      string  `json:"status"`
	Color       string  `json:"color"`
	Icon        string  `json:"icon"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func toProjectDTO(p domain.Project) projectDTO {
	return projectDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		LocalPath:   p.LocalPath,
		GitHubRepo:  p.GitHubRepo,
		Status:      string(p.Status),
		Color:       p.Color,
		Icon:        p.Icon,
		CreatedAt:   p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type projectHandlers struct{ svc ProjectService }

func (h projectHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", h.list)
	mux.HandleFunc("POST /api/projects", h.create)
	mux.HandleFunc("GET /api/projects/{id}", h.get)
	mux.HandleFunc("PATCH /api/projects/{id}", h.update)
	mux.HandleFunc("DELETE /api/projects/{id}", h.delete)
}

func (h projectHandlers) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, ps, toProjectDTO)
}

func (h projectHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		LocalPath   *string `json:"local_path"`
		GitHubRepo  *string `json:"github_repo"`
		Status      string  `json:"status"`
		Color       string  `json:"color"`
		Icon        string  `json:"icon"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	p, err := h.svc.Create(r.Context(), service.CreateProjectInput{
		Name:        in.Name,
		Description: in.Description,
		LocalPath:   in.LocalPath,
		GitHubRepo:  in.GitHubRepo,
		Status:      domain.ProjectStatus(in.Status),
		Color:       in.Color,
		Icon:        in.Icon,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProjectDTO(p))
}

func (h projectHandlers) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProjectDTO(p))
}

// update: nicht gesendete Felder (oder null) bleiben unverändert;
// local_path bzw. github_repo "" entfernt die Verknüpfung.
func (h projectHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		LocalPath   *string `json:"local_path"`
		GitHubRepo  *string `json:"github_repo"`
		Status      *string `json:"status"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	upd := service.UpdateProjectInput{Name: in.Name, Description: in.Description, LocalPath: in.LocalPath, GitHubRepo: in.GitHubRepo, Color: in.Color, Icon: in.Icon}
	if in.Status != nil {
		s := domain.ProjectStatus(*in.Status)
		upd.Status = &s
	}
	p, err := h.svc.Update(r.Context(), r.PathValue("id"), upd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProjectDTO(p))
}

func (h projectHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
