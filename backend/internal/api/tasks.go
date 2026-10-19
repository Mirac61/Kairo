package api

import (
	"context"
	"fmt"
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
	Trash(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (domain.Task, error)
	ListTrashed(ctx context.Context) ([]domain.Task, error)
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
	RecurrenceRule   *string `json:"recurrence_rule"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	CompletedAt      *string `json:"completed_at"`
	DeletedAt        *string `json:"deleted_at,omitempty"` // nur im Papierkorb
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
		RecurrenceRule:   t.RecurrenceRule,
		CreatedAt:        t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        t.UpdatedAt.UTC().Format(time.RFC3339),
		CompletedAt:      formatOptTime(t.CompletedAt),
		DeletedAt:        formatOptTime(t.DeletedAt),
	}
}

type taskHandlers struct{ svc TaskService }

func (h taskHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tasks", h.list)
	mux.HandleFunc("POST /api/tasks", h.create)
	mux.HandleFunc("POST /api/tasks/bulk", h.bulk)
	mux.HandleFunc("GET /api/tasks/{id}", h.get)
	mux.HandleFunc("PATCH /api/tasks/{id}", h.update)
	mux.HandleFunc("DELETE /api/tasks/{id}", deleteHandler(h.svc.Trash, h.svc.Delete))
	mux.HandleFunc("POST /api/tasks/{id}/restore", restoreHandler(h.svc.Restore, toTaskDTO))
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
	writeList(w, ts, toTaskDTO)
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
		RecurrenceRule   *string `json:"recurrence_rule"`
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
		RecurrenceRule:   in.RecurrenceRule,
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
// due_at, planned_date, planned_start_at, project_id, parent_task_id und recurrence_rule
// entfernt "" den Wert.
func (h taskHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in taskUpdate
	if !decodeJSON(w, r, &in) {
		return
	}
	t, err := h.svc.Update(r.Context(), r.PathValue("id"), in.input())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskDTO(t))
}

// taskUpdate ist eine einzelne Feldänderung; für PATCH /api/tasks/{id} bleibt id leer,
// für POST /api/tasks/bulk trägt jedes Element seine id.
type taskUpdate struct {
	ID               string  `json:"id"`
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
	RecurrenceRule   *string `json:"recurrence_rule"`
}

func (u taskUpdate) input() service.UpdateTaskInput {
	upd := service.UpdateTaskInput{
		Title:            u.Title,
		Description:      u.Description,
		EstimatedMinutes: u.EstimatedMinutes,
		DueAt:            u.DueAt,
		PlannedDate:      u.PlannedDate,
		PlannedStartAt:   u.PlannedStartAt,
		ProjectID:        u.ProjectID,
		ParentTaskID:     u.ParentTaskID,
		RecurrenceRule:   u.RecurrenceRule,
	}
	if u.Status != nil {
		s := domain.TaskStatus(*u.Status)
		upd.Status = &s
	}
	if u.Priority != nil {
		p := domain.TaskPriority(*u.Priority)
		upd.Priority = &p
	}
	return upd
}

// bulk ändert mehrere Aufgaben in einem Aufruf (eine Anfrage statt n einzelner PATCH).
func (h taskHandlers) bulk(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Updates []taskUpdate `json:"updates"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "updates darf nicht leer sein"})
		return
	}
	updated := 0
	for _, u := range in.Updates {
		if _, err := h.svc.Update(r.Context(), u.ID, u.input()); err != nil {
			// Keine Transaktion über alle: Die ersten Änderungen bleiben, die Meldung sagt, wie viele.
			writeError(w, fmt.Errorf("%d von %d geändert, dann: %w", updated, len(in.Updates), err))
			return
		}
		updated++
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": updated})
}
