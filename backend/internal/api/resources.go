package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// ResourceService ist die Geschäftslogik, die die Ressourcen-Handler brauchen.
type ResourceService interface {
	Create(ctx context.Context, in service.CreateResourceInput) (domain.Resource, error)
	List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, error)
	Delete(ctx context.Context, id string) error
}

type resourceDTO struct {
	ID        string  `json:"id"`
	TaskID    *string `json:"task_id"`
	ProjectID *string `json:"project_id"`
	Type      string  `json:"type"`
	Target    string  `json:"target"`
	Label     string  `json:"label"`
	CreatedAt string  `json:"created_at"`
}

func toResourceDTO(r domain.Resource) resourceDTO {
	return resourceDTO{
		ID:        r.ID,
		TaskID:    r.TaskID,
		ProjectID: r.ProjectID,
		Type:      string(r.Type),
		Target:    r.Target,
		Label:     r.Label,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type resourceHandlers struct{ svc ResourceService }

func (h resourceHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/resources", h.list)
	mux.HandleFunc("POST /api/resources", h.create)
	mux.HandleFunc("DELETE /api/resources/{id}", h.delete)
}

// list unterstützt die Filter ?task_id=, ?project_id= und ?type=.
func (h resourceHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rs, err := h.svc.List(r.Context(), domain.ResourceFilter{
		TaskID:    q.Get("task_id"),
		ProjectID: q.Get("project_id"),
		Type:      domain.ResourceType(q.Get("type")),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, rs, toResourceDTO)
}

// create: type ist optional (URL, FOLDER, FILE); ohne ihn leitet der Service ihn aus target ab.
func (h resourceHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TaskID    *string `json:"task_id"`
		ProjectID *string `json:"project_id"`
		Type      string  `json:"type"`
		Target    string  `json:"target"`
		Label     string  `json:"label"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	res, err := h.svc.Create(r.Context(), service.CreateResourceInput{
		TaskID:    in.TaskID,
		ProjectID: in.ProjectID,
		Type:      domain.ResourceType(in.Type),
		Target:    in.Target,
		Label:     in.Label,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResourceDTO(res))
}

func (h resourceHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
