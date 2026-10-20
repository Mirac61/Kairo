package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// GoalService ist die Geschäftslogik, die die Ziel-Handler brauchen.
type GoalService interface {
	Create(ctx context.Context, in service.CreateGoalInput) (service.GoalView, error)
	Update(ctx context.Context, id string, in service.UpdateGoalInput) (service.GoalView, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]service.GoalView, error)
	Today(ctx context.Context) (*service.GoalView, error)
}

type goalProgressDTO struct {
	Current int `json:"current"`
	Target  int `json:"target"`
	Percent int `json:"percent"`
}

type goalDTO struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Why       string           `json:"why"`
	ImagePath *string          `json:"image_path"`
	Kind      string           `json:"kind"`
	ProjectID *string          `json:"project_id"`
	HabitID   *string          `json:"habit_id"`
	Target    int              `json:"target"`
	StartDate string           `json:"start_date"`
	DueDate   *string          `json:"due_date"`
	Archived  bool             `json:"archived"`
	ParentID  *string          `json:"parent_id"`
	CreatedAt string           `json:"created_at"`
	Progress  *goalProgressDTO `json:"progress"`
}

func toGoalDTO(v service.GoalView) goalDTO {
	out := goalDTO{
		ID: v.ID, Title: v.Title, Why: v.Why, ImagePath: v.ImagePath, Kind: string(v.Kind),
		ProjectID: v.ProjectID, HabitID: v.HabitID, Target: v.Target, StartDate: v.StartDate,
		DueDate: v.DueDate, Archived: v.Archived, ParentID: v.ParentID, CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339),
	}
	if v.Progress != nil {
		p := goalProgressDTO(*v.Progress)
		out.Progress = &p
	}
	return out
}

type goalHandlers struct{ svc GoalService }

func (h goalHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/goals", h.list)
	mux.HandleFunc("GET /api/goals/today", h.today)
	mux.HandleFunc("POST /api/goals", h.create)
	mux.HandleFunc("PATCH /api/goals/{id}", h.update)
	// Ziele haben keinen Papierkorb; permanent=true ändert nichts.
	mux.HandleFunc("DELETE /api/goals/{id}", deleteHandler(h.svc.Delete, h.svc.Delete))
}

func (h goalHandlers) list(w http.ResponseWriter, r *http.Request) {
	gs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, gs, toGoalDTO)
}

// today liefert das Ziel des Tages oder null, wenn es keine aktiven Ziele gibt.
func (h goalHandlers) today(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.Today(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if g == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, toGoalDTO(*g))
}

func (h goalHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string  `json:"title"`
		Why       string  `json:"why"`
		ImagePath *string `json:"image_path"`
		Kind      string  `json:"kind"`
		ProjectID *string `json:"project_id"`
		HabitID   *string `json:"habit_id"`
		Target    int     `json:"target"`
		StartDate *string `json:"start_date"`
		DueDate   *string `json:"due_date"`
		ParentID  *string `json:"parent_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	g, err := h.svc.Create(r.Context(), service.CreateGoalInput{
		Title: in.Title, Why: in.Why, ImagePath: in.ImagePath, Kind: domain.GoalKind(in.Kind),
		ProjectID: in.ProjectID, HabitID: in.HabitID, Target: in.Target, StartDate: in.StartDate, DueDate: in.DueDate, ParentID: in.ParentID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGoalDTO(g))
}

// update: nicht gesendete Felder bleiben; "" entfernt image_path, project_id, habit_id und due_date.
func (h goalHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     *string `json:"title"`
		Why       *string `json:"why"`
		ImagePath *string `json:"image_path"`
		Kind      *string `json:"kind"`
		ProjectID *string `json:"project_id"`
		HabitID   *string `json:"habit_id"`
		Target    *int    `json:"target"`
		StartDate *string `json:"start_date"`
		DueDate   *string `json:"due_date"`
		Archived  *bool   `json:"archived"`
		ParentID  *string `json:"parent_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	upd := service.UpdateGoalInput{
		Title: in.Title, Why: in.Why, ImagePath: in.ImagePath, ProjectID: in.ProjectID, HabitID: in.HabitID,
		Target: in.Target, StartDate: in.StartDate, DueDate: in.DueDate, Archived: in.Archived, ParentID: in.ParentID,
	}
	if in.Kind != nil {
		k := domain.GoalKind(*in.Kind)
		upd.Kind = &k
	}
	g, err := h.svc.Update(r.Context(), r.PathValue("id"), upd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGoalDTO(g))
}
