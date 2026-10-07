package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// HabitService ist die Geschäftslogik, die die Habit-Handler brauchen.
type HabitService interface {
	Create(ctx context.Context, in service.CreateHabitInput) (domain.Habit, error)
	Get(ctx context.Context, id string) (domain.Habit, error)
	List(ctx context.Context) ([]domain.Habit, error)
	Update(ctx context.Context, id string, in service.UpdateHabitInput) (domain.Habit, error)
	Delete(ctx context.Context, id string) error
	Trash(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (domain.Habit, error)
	ListTrashed(ctx context.Context) ([]domain.Habit, error)
	Complete(ctx context.Context, habitID string, in service.CompleteHabitInput) (domain.HabitCompletion, error)
	Uncomplete(ctx context.Context, habitID, date string) error
	Completions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error)
}

type habitDTO struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	FrequencyType   string                 `json:"frequency_type"`
	FrequencyConfig domain.FrequencyConfig `json:"frequency_config"`
	TargetValue     *int                   `json:"target_value"`
	Unit            string                 `json:"unit"`
	PreferredTime   *string                `json:"preferred_time"`
	StartDate       string                 `json:"start_date"`
	EndDate         *string                `json:"end_date"`
	Active          bool                   `json:"active"`
	CreatedAt       string                 `json:"created_at"`
	DeletedAt       *string                `json:"deleted_at,omitempty"` // nur im Papierkorb
}

func toHabitDTO(h domain.Habit) habitDTO {
	return habitDTO{
		ID:              h.ID,
		Name:            h.Name,
		Description:     h.Description,
		FrequencyType:   string(h.FrequencyType),
		FrequencyConfig: h.FrequencyConfig,
		TargetValue:     h.TargetValue,
		Unit:            h.Unit,
		PreferredTime:   h.PreferredTime,
		StartDate:       h.StartDate,
		EndDate:         h.EndDate,
		Active:          h.Active,
		CreatedAt:       h.CreatedAt.UTC().Format(time.RFC3339),
		DeletedAt:       formatOptTime(h.DeletedAt),
	}
}

type habitCompletionDTO struct {
	ID          string `json:"id"`
	HabitID     string `json:"habit_id"`
	Date        string `json:"date"`
	Value       *int   `json:"value"`
	CompletedAt string `json:"completed_at"`
	Note        string `json:"note"`
}

func toHabitCompletionDTO(c domain.HabitCompletion) habitCompletionDTO {
	return habitCompletionDTO{
		ID:          c.ID,
		HabitID:     c.HabitID,
		Date:        c.Date,
		Value:       c.Value,
		CompletedAt: c.CompletedAt.UTC().Format(time.RFC3339),
		Note:        c.Note,
	}
}

type habitHandlers struct{ svc HabitService }

func (h habitHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/habits", h.list)
	mux.HandleFunc("POST /api/habits", h.create)
	mux.HandleFunc("GET /api/habits/{id}", h.get)
	mux.HandleFunc("PATCH /api/habits/{id}", h.update)
	mux.HandleFunc("DELETE /api/habits/{id}", deleteHandler(h.svc.Trash, h.svc.Delete))
	mux.HandleFunc("POST /api/habits/{id}/restore", restoreHandler(h.svc.Restore, toHabitDTO))
	mux.HandleFunc("GET /api/habits/{id}/completions", h.listCompletions)
	mux.HandleFunc("POST /api/habits/{id}/completions", h.complete)
	mux.HandleFunc("DELETE /api/habits/{id}/completions/{date}", h.uncomplete)
}

func (h habitHandlers) list(w http.ResponseWriter, r *http.Request) {
	hs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]habitDTO, len(hs))
	for i, x := range hs {
		out[i] = toHabitDTO(x)
	}
	writeJSON(w, http.StatusOK, out)
}

// create: start_date ist standardmäßig heute, active standardmäßig true.
func (h habitHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name            string                 `json:"name"`
		Description     string                 `json:"description"`
		FrequencyType   string                 `json:"frequency_type"`
		FrequencyConfig domain.FrequencyConfig `json:"frequency_config"`
		TargetValue     *int                   `json:"target_value"`
		Unit            string                 `json:"unit"`
		PreferredTime   *string                `json:"preferred_time"`
		StartDate       *string                `json:"start_date"`
		EndDate         *string                `json:"end_date"`
		Active          *bool                  `json:"active"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	x, err := h.svc.Create(r.Context(), service.CreateHabitInput{
		Name:            in.Name,
		Description:     in.Description,
		FrequencyType:   domain.FrequencyType(in.FrequencyType),
		FrequencyConfig: in.FrequencyConfig,
		TargetValue:     in.TargetValue,
		Unit:            in.Unit,
		PreferredTime:   in.PreferredTime,
		StartDate:       in.StartDate,
		EndDate:         in.EndDate,
		Active:          in.Active,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toHabitDTO(x))
}

func (h habitHandlers) get(w http.ResponseWriter, r *http.Request) {
	x, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toHabitDTO(x))
}

// update: nicht gesendete Felder (oder null) bleiben unverändert;
// target_value 0, preferred_time "" und end_date "" entfernen den Wert.
// Ein neuer frequency_type ohne frequency_config setzt die Details zurück.
func (h habitHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name            *string                 `json:"name"`
		Description     *string                 `json:"description"`
		FrequencyType   *string                 `json:"frequency_type"`
		FrequencyConfig *domain.FrequencyConfig `json:"frequency_config"`
		TargetValue     *int                    `json:"target_value"`
		Unit            *string                 `json:"unit"`
		PreferredTime   *string                 `json:"preferred_time"`
		StartDate       *string                 `json:"start_date"`
		EndDate         *string                 `json:"end_date"`
		Active          *bool                   `json:"active"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	upd := service.UpdateHabitInput{
		Name:            in.Name,
		Description:     in.Description,
		FrequencyConfig: in.FrequencyConfig,
		TargetValue:     in.TargetValue,
		Unit:            in.Unit,
		PreferredTime:   in.PreferredTime,
		StartDate:       in.StartDate,
		EndDate:         in.EndDate,
		Active:          in.Active,
	}
	if in.FrequencyType != nil {
		t := domain.FrequencyType(*in.FrequencyType)
		upd.FrequencyType = &t
	}
	x, err := h.svc.Update(r.Context(), r.PathValue("id"), upd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toHabitDTO(x))
}

// listCompletions unterstützt ?from= und ?to= (YYYY-MM-DD, inklusive).
func (h habitHandlers) listCompletions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cs, err := h.svc.Completions(r.Context(), r.PathValue("id"), q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]habitCompletionDTO, len(cs))
	for i, c := range cs {
		out[i] = toHabitCompletionDTO(c)
	}
	writeJSON(w, http.StatusOK, out)
}

// complete hakt das Habit ab. date ist optional (Standard: heute); ein
// zweites Abhaken am selben Tag ergibt 409.
func (h habitHandlers) complete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Date  string `json:"date"`
		Value *int   `json:"value"`
		Note  string `json:"note"`
	}
	// Ein leerer Body bedeutet "heute, ohne Wert".
	if r.ContentLength != 0 && !decodeJSON(w, r, &in) {
		return
	}
	c, err := h.svc.Complete(r.Context(), r.PathValue("id"), service.CompleteHabitInput(in))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toHabitCompletionDTO(c))
}

func (h habitHandlers) uncomplete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Uncomplete(r.Context(), r.PathValue("id"), r.PathValue("date")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
