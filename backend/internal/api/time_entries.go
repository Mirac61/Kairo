package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// TimeTrackingService ist die Geschäftslogik, die die Zeittracking-Handler brauchen.
type TimeTrackingService interface {
	Start(ctx context.Context, taskID string, source domain.TimeSource) (domain.TimerResult, error)
	Pause(ctx context.Context, taskID string, endedAt *string) (domain.TimerResult, error)
	Complete(ctx context.Context, taskID string) (domain.TimerResult, error)
	Create(ctx context.Context, in service.CreateTimeEntryInput) (domain.TimeEntry, error)
	List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error)
	Update(ctx context.Context, id string, in service.UpdateTimeEntryInput) (domain.TimeEntry, error)
	Delete(ctx context.Context, id string) error
}

type timeEntryDTO struct {
	ID        string  `json:"id"`
	TaskID    *string `json:"task_id"`
	ProjectID *string `json:"project_id"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Source    string  `json:"source"`
}

func toTimeEntryDTO(e domain.TimeEntry) timeEntryDTO {
	return timeEntryDTO{
		ID:        e.ID,
		TaskID:    e.TaskID,
		ProjectID: e.ProjectID,
		StartedAt: e.StartedAt.UTC().Format(time.RFC3339),
		EndedAt:   formatOptTime(e.EndedAt),
		Source:    string(e.Source),
	}
}

// timerDTO ist die Antwort von start, pause und complete.
type timerDTO struct {
	Task      taskDTO       `json:"task"`
	TimeEntry *timeEntryDTO `json:"time_entry"`
}

func toTimerDTO(r domain.TimerResult) timerDTO {
	out := timerDTO{Task: toTaskDTO(r.Task)}
	if r.Entry != nil {
		e := toTimeEntryDTO(*r.Entry)
		out.TimeEntry = &e
	}
	return out
}

type timeHandlers struct{ svc TimeTrackingService }

func (h timeHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/time-entries", h.list)
	mux.HandleFunc("POST /api/time-entries", h.create)
	mux.HandleFunc("PATCH /api/time-entries/{id}", h.update)
	mux.HandleFunc("DELETE /api/time-entries/{id}", h.delete)
	mux.HandleFunc("POST /api/tasks/{id}/start", h.start)
	mux.HandleFunc("POST /api/tasks/{id}/pause", h.pause)
	mux.HandleFunc("POST /api/tasks/{id}/complete", h.complete)
}

// list unterstützt die Filter ?task_id=, ?project_id= (wirksames Projekt),
// ?from= und ?to= (RFC 3339, begrenzen started_at: from inklusive, to
// exklusiv; ein "+" im Offset als %2B kodieren) sowie ?running=true (nur der laufende Timer).
func (h timeHandlers) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.TimeEntryFilter{TaskID: q.Get("task_id"), ProjectID: q.Get("project_id")}
	var err error
	if f.From, err = queryTime(r, "from"); err != nil {
		writeError(w, err)
		return
	}
	if f.To, err = queryTime(r, "to"); err != nil {
		writeError(w, err)
		return
	}
	switch q.Get("running") {
	case "", "false":
	case "true":
		f.Running = true
	default:
		writeError(w, fmt.Errorf("%w: running muss true oder false sein", domain.ErrInvalid))
		return
	}
	es, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, es, toTimeEntryDTO)
}

// create erfasst einen beendeten Eintrag nachträglich (started_at und
// ended_at sind Pflicht). Einen laufenden Timer startet POST /api/tasks/{id}/start.
func (h timeHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TaskID    *string `json:"task_id"`
		ProjectID *string `json:"project_id"`
		StartedAt string  `json:"started_at"`
		EndedAt   string  `json:"ended_at"`
		Source    string  `json:"source"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	e, err := h.svc.Create(r.Context(), service.CreateTimeEntryInput{
		TaskID:    in.TaskID,
		ProjectID: in.ProjectID,
		StartedAt: in.StartedAt,
		EndedAt:   in.EndedAt,
		Source:    domain.TimeSource(in.Source),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTimeEntryDTO(e))
}

// update korrigiert {"started_at", "ended_at"} (RFC 3339, beide optional).
func (h timeHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StartedAt *string `json:"started_at"`
		EndedAt   *string `json:"ended_at"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	e, err := h.svc.Update(r.Context(), r.PathValue("id"), service.UpdateTimeEntryInput{StartedAt: in.StartedAt, EndedAt: in.EndedAt})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTimeEntryDTO(e))
}

func (h timeHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// start akzeptiert optional {"source": "MANUAL|VSCODIUM|AUTOMATIC"} (Standard MANUAL).
func (h timeHandlers) start(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Source string `json:"source"`
	}
	if !decodeOptionalJSON(w, r, &in) {
		return
	}
	res, err := h.svc.Start(r.Context(), r.PathValue("id"), domain.TimeSource(in.Source))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTimerDTO(res))
}

// pause akzeptiert optional {"ended_at": RFC 3339} (Standard: jetzt).
func (h timeHandlers) pause(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EndedAt *string `json:"ended_at"`
	}
	if !decodeOptionalJSON(w, r, &in) {
		return
	}
	res, err := h.svc.Pause(r.Context(), r.PathValue("id"), in.EndedAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTimerDTO(res))
}

func (h timeHandlers) complete(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Complete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTimerDTO(res))
}
