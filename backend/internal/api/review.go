package api

import (
	"context"
	"net/http"

	"kairo/internal/service"
)

// ReviewService ist die Geschäftslogik, die der Review-Handler braucht.
type ReviewService interface {
	Get(ctx context.Context, from, to string) (service.Review, error)
}

type reviewDayDTO struct {
	Date            string `json:"date"`
	TrackedMinutes  int    `json:"tracked_minutes"`
	CalendarMinutes int    `json:"calendar_minutes"`
	CompletedTasks  int    `json:"completed_tasks"`
}

type reviewProjectDTO struct {
	ProjectID      *string `json:"project_id"`
	Name           string  `json:"name"`
	TrackedMinutes int     `json:"tracked_minutes"`
	CompletedTasks int     `json:"completed_tasks"`
	DoneTasks      int     `json:"done_tasks"`
	TotalTasks     int     `json:"total_tasks"`
}

type reviewHabitDTO struct {
	HabitID string `json:"habit_id"`
	Name    string `json:"name"`
	Done    int    `json:"done"`
	Streak  int    `json:"streak"`
}

type reviewDTO struct {
	From           string             `json:"from"`
	To             string             `json:"to"`
	Timezone       string             `json:"timezone"`
	TrackedMinutes int                `json:"tracked_minutes"`
	CompletedTasks int                `json:"completed_tasks"`
	Days           []reviewDayDTO     `json:"days"`
	Projects       []reviewProjectDTO `json:"projects"`
	Habits         []reviewHabitDTO   `json:"habits"`
}

func toReviewDTO(r service.Review) reviewDTO {
	out := reviewDTO{
		From: r.From, To: r.To, Timezone: r.Timezone, TrackedMinutes: r.TrackedMinutes, CompletedTasks: r.CompletedTasks,
		Days: make([]reviewDayDTO, len(r.Days)), Projects: make([]reviewProjectDTO, len(r.Projects)), Habits: make([]reviewHabitDTO, len(r.Habits)),
	}
	for i, d := range r.Days {
		out.Days[i] = reviewDayDTO(d)
	}
	for i, p := range r.Projects {
		out.Projects[i] = reviewProjectDTO{Name: p.Name, TrackedMinutes: p.TrackedMinutes, CompletedTasks: p.CompletedTasks, DoneTasks: p.DoneTasks, TotalTasks: p.TotalTasks}
		if p.ProjectID != "" {
			id := p.ProjectID
			out.Projects[i].ProjectID = &id
		}
	}
	for i, h := range r.Habits {
		out.Habits[i] = reviewHabitDTO(h)
	}
	return out
}

type reviewHandlers struct{ svc ReviewService }

func (h reviewHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/review", h.get)
}

// get wertet ?from=YYYY-MM-DD&to=YYYY-MM-DD aus (beide inklusive; Standard: letzte 7 Tage).
func (h reviewHandlers) get(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rv, err := h.svc.Get(r.Context(), q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReviewDTO(rv))
}
