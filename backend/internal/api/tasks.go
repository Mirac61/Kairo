package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// TaskService ist die Geschäftslogik, die die Task-Handler brauchen.
type TaskService interface {
	Create(ctx context.Context, in service.CreateTaskInput) (domain.Task, error)
	Get(ctx context.Context, id string) (domain.Task, error)
	List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error)
	Update(ctx context.Context, id string, in service.UpdateTaskInput) (domain.Task, error)
	Delete(ctx context.Context, id string) error
}

type taskDTO struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority"`
	EstimatedMinutes int     `json:"estimated_minutes"`
	DueAt            *string `json:"due_at"`
	PlannedDate      *string `json:"planned_date"`
	PlannedStartAt   *string `json:"planned_start_at"`
	ProjectID        *string `json:"project_id"`
	ParentTaskID     *string `json:"parent_task_id"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	CompletedAt      *string `json:"completed_at"`
}

func formatOptTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toTaskDTO(t domain.Task) taskDTO {
	return taskDTO{
		ID:               t.ID,
		Title:            t.Title,
		Description:      t.Description,
		Status:           string(t.Status),
		Priority:         string(t.Priority),
		EstimatedMinutes: t.EstimatedMinutes,
		DueAt:            formatOptTime(t.DueAt),
		PlannedDate:      t.PlannedDate,
		PlannedStartAt:   formatOptTime(t.PlannedStartAt),
		ProjectID:        t.ProjectID,
		ParentTaskID:     t.ParentTaskID,
		CreatedAt:        t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        t.UpdatedAt.UTC().Format(time.RFC3339),
		CompletedAt:      formatOptTime(t.CompletedAt),
	}
}

type taskHandlers struct{ svc TaskService }

func (h taskHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tasks", h.list)
	mux.HandleFunc("POST /api/tasks", h.create)
	mux.HandleFunc("GET /api/tasks/{id}", h.get)
	mux.HandleFunc("PATCH /api/tasks/{id}", h.update)
	mux.HandleFunc("DELETE /api/tasks/{id}", h.delete)
}

// list unterstützt die Filter ?status=, ?project_id= und ?planned_date=.
func (h taskHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ts, err := h.svc.List(r.Context(), domain.TaskFilter{
		Status:      domain.TaskStatus(q.Get("status")),
		ProjectID:   q.Get("project_id"),
		PlannedDate: q.Get("planned_date"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]taskDTO, len(ts))
	for i, t := range ts {
		out[i] = toTaskDTO(t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h taskHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title            string  `json:"title"`
		Description      string  `json:"description"`
		Status           string  `json:"status"`
		Priority         string  `json:"priority"`
		EstimatedMinutes int     `json:"estimated_minutes"`
		DueAt            *string `json:"due_at"`
		PlannedDate      *string `json:"planned_date"`
		PlannedStartAt   *string `json:"planned_start_at"`
		ProjectID        *string `json:"project_id"`
		ParentTaskID     *string `json:"parent_task_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	t, err := h.svc.Create(r.Context(), service.CreateTaskInput{
		Title:            in.Title,
		Description:      in.Description,
		Status:           domain.TaskStatus(in.Status),
		Priority:         domain.TaskPriority(in.Priority),
		EstimatedMinutes: in.EstimatedMinutes,
		DueAt:            in.DueAt,
		PlannedDate:      in.PlannedDate,
		PlannedStartAt:   in.PlannedStartAt,
		ProjectID:        in.ProjectID,
		ParentTaskID:     in.ParentTaskID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTaskDTO(t))
}

func (h taskHandlers) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskDTO(t))
}

// update: nicht gesendete Felder (oder null) bleiben unverändert; bei
// due_at, planned_date, planned_start_at, project_id und parent_task_id
// entfernt "" den Wert.
func (h taskHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title            *string `json:"title"`
		Description      *string `json:"description"`
		Status           *string `json:"status"`
		Priority         *string `json:"priority"`
		EstimatedMinutes *int    `json:"estimated_minutes"`
		DueAt            *string `json:"due_at"`
		PlannedDate      *string `json:"planned_date"`
		PlannedStartAt   *string `json:"planned_start_at"`
		ProjectID        *string `json:"project_id"`
		ParentTaskID     *string `json:"parent_task_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	upd := service.UpdateTaskInput{
		Title:            in.Title,
		Description:      in.Description,
		EstimatedMinutes: in.EstimatedMinutes,
		DueAt:            in.DueAt,
		PlannedDate:      in.PlannedDate,
		PlannedStartAt:   in.PlannedStartAt,
		ProjectID:        in.ProjectID,
		ParentTaskID:     in.ParentTaskID,
	}
	if in.Status != nil {
		s := domain.TaskStatus(*in.Status)
		upd.Status = &s
	}
	if in.Priority != nil {
		p := domain.TaskPriority(*in.Priority)
		upd.Priority = &p
	}
	t, err := h.svc.Update(r.Context(), r.PathValue("id"), upd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskDTO(t))
}

func (h taskHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
