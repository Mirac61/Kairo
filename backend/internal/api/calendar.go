package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// CalendarService ist die Geschäftslogik, die die Kalender-Handler brauchen.
type CalendarService interface {
	Create(ctx context.Context, in service.CreateEventInput) (domain.CalendarEvent, error)
	Get(ctx context.Context, id string) (domain.CalendarEvent, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.CalendarEvent, error)
	Update(ctx context.Context, id string, in service.UpdateEventInput) (domain.CalendarEvent, error)
	Delete(ctx context.Context, id string) error
	Occurrences(ctx context.Context, from, to time.Time) ([]domain.EventInstance, error)
	ImportICS(ctx context.Context, data string) (service.ImportResult, error)
}

type calendarEventDTO struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	StartAt           string   `json:"start_at"`
	EndAt             string   `json:"end_at"`
	Location          string   `json:"location"`
	URL               string   `json:"url"`
	ProjectID         *string  `json:"project_id"`
	TaskID            *string  `json:"task_id"`
	RecurrenceRule    *string  `json:"recurrence_rule"`
	RecurrenceExdates []string `json:"recurrence_exdates"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func toCalendarEventDTO(e domain.CalendarEvent) calendarEventDTO {
	ex := e.RecurrenceExdates
	if ex == nil {
		ex = []string{}
	}
	return calendarEventDTO{
		ID:                e.ID,
		Title:             e.Title,
		Description:       e.Description,
		StartAt:           e.StartAt.UTC().Format(time.RFC3339),
		EndAt:             e.EndAt.UTC().Format(time.RFC3339),
		Location:          e.Location,
		URL:               e.URL,
		ProjectID:         e.ProjectID,
		TaskID:            e.TaskID,
		RecurrenceRule:    e.RecurrenceRule,
		RecurrenceExdates: ex,
		CreatedAt:         e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type calendarHandlers struct{ svc CalendarService }

func (h calendarHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/calendar/events", h.list)
	mux.HandleFunc("POST /api/calendar/events", h.create)
	mux.HandleFunc("GET /api/calendar/occurrences", h.occurrences)
	mux.HandleFunc("POST /api/calendar/import", h.importICS)
	mux.HandleFunc("GET /api/calendar/events/{id}", h.get)
	mux.HandleFunc("PATCH /api/calendar/events/{id}", h.update)
	mux.HandleFunc("DELETE /api/calendar/events/{id}", h.delete)
}

// queryTime liest einen optionalen RFC-3339-Parameter. Ein "+" im Offset
// muss als %2B kodiert sein.
func queryTime(r *http.Request, name string) (*time.Time, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, fmt.Errorf("%w: %s muss ein RFC-3339-Zeitpunkt sein", domain.ErrInvalid, name)
	}
	return &t, nil
}

// list unterstützt ?from= und ?to= (RFC 3339, nur zusammen): dann kommen nur
// Events, die das Fenster berühren. Wiederkehrende Events erscheinen einmal
// als Serie, nicht als einzelne Termine.
func (h calendarHandlers) list(w http.ResponseWriter, r *http.Request) {
	from, err := queryTime(r, "from")
	if err != nil {
		writeError(w, err)
		return
	}
	to, err := queryTime(r, "to")
	if err != nil {
		writeError(w, err)
		return
	}
	es, err := h.svc.List(r.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]calendarEventDTO, len(es))
	for i, e := range es {
		out[i] = toCalendarEventDTO(e)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h calendarHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title             string   `json:"title"`
		Description       string   `json:"description"`
		StartAt           string   `json:"start_at"`
		EndAt             string   `json:"end_at"`
		Location          string   `json:"location"`
		URL               string   `json:"url"`
		ProjectID         *string  `json:"project_id"`
		TaskID            *string  `json:"task_id"`
		RecurrenceRule    *string  `json:"recurrence_rule"`
		RecurrenceExdates []string `json:"recurrence_exdates"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	e, err := h.svc.Create(r.Context(), service.CreateEventInput(in))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCalendarEventDTO(e))
}

func (h calendarHandlers) get(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCalendarEventDTO(e))
}

// update: nicht gesendete Felder (oder null) bleiben unverändert; bei
// project_id, task_id und recurrence_rule entfernt "" den Wert.
// recurrence_exdates ersetzt die ganze Liste.
func (h calendarHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title             *string   `json:"title"`
		Description       *string   `json:"description"`
		StartAt           *string   `json:"start_at"`
		EndAt             *string   `json:"end_at"`
		Location          *string   `json:"location"`
		URL               *string   `json:"url"`
		ProjectID         *string   `json:"project_id"`
		TaskID            *string   `json:"task_id"`
		RecurrenceRule    *string   `json:"recurrence_rule"`
		RecurrenceExdates *[]string `json:"recurrence_exdates"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	e, err := h.svc.Update(r.Context(), r.PathValue("id"), service.UpdateEventInput(in))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCalendarEventDTO(e))
}

func (h calendarHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// occurrences liefert die konkreten Termine im Fenster ?from=&to= (beide
// Pflicht, RFC 3339), Serien aufgelöst; Format wie events in GET /api/today.
func (h calendarHandlers) occurrences(w http.ResponseWriter, r *http.Request) {
	from, err := queryTime(r, "from")
	if err == nil {
		var to *time.Time
		if to, err = queryTime(r, "to"); err == nil && (from == nil || to == nil) {
			err = fmt.Errorf("%w: from und to sind Pflicht", domain.ErrInvalid)
		} else if err == nil {
			var es []domain.EventInstance
			if es, err = h.svc.Occurrences(r.Context(), *from, *to); err == nil {
				out := make([]todayEventDTO, len(es))
				for i, e := range es {
					out[i] = toTodayEventDTO(e)
				}
				writeJSON(w, http.StatusOK, out)
				return
			}
		}
	}
	writeError(w, err)
}

// maxICSBytes begrenzt den Body von POST /api/calendar/import (ein Semester-Stundenplan ist wenige KiB groß).
const maxICSBytes = 5 << 20

type importResultDTO struct {
	Created          int      `json:"created"`
	Updated          int      `json:"updated"`
	Skipped          int      `json:"skipped"`
	UnsupportedRules int      `json:"unsupported_rules"`
	Notes            []string `json:"notes"`
}

// importICS nimmt den rohen ICS-Text (Content-Type text/calendar) entgegen.
// Termine mit bekannter UID werden aktualisiert, die anderen angelegt.
func (h calendarHandlers) importICS(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxICSBytes))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "ICS-Datei ist größer als 5 MiB"})
		return
	}
	if err != nil {
		writeError(w, fmt.Errorf("%w: Body nicht lesbar", domain.ErrInvalid))
		return
	}
	res, err := h.svc.ImportICS(r.Context(), string(body))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, importResultDTO(res))
}
