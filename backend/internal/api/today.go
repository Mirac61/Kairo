package api

import (
	"context"
	"net/http"
	"time"

	"kairo/internal/service"
)

// TodayService ist die Geschäftslogik, die der Today-Handler braucht.
type TodayService interface {
	Get(ctx context.Context, date string) (service.Today, error)
}

type todayEventDTO struct {
	calendarEventDTO
	// OccurrenceStart und OccurrenceEnd sind der konkrete Termin an diesem
	// Tag; start_at und end_at gehören bei Serien zur ersten Wiederholung.
	OccurrenceStart string `json:"occurrence_start"`
	OccurrenceEnd   string `json:"occurrence_end"`
}

type weekProgressDTO struct {
	Done   int `json:"done"`
	Target int `json:"target"`
}

type todayHabitDTO struct {
	habitDTO
	Done         bool             `json:"done"`
	WeekProgress *weekProgressDTO `json:"week_progress"`
}

type todayDTO struct {
	Date            string          `json:"date"`
	Timezone        string          `json:"timezone"`
	DayStart        string          `json:"day_start"`
	DayEnd          string          `json:"day_end"`
	Events          []todayEventDTO `json:"events"`
	Tasks           []taskDTO       `json:"tasks"`
	ActiveTasks     []taskDTO       `json:"active_tasks"`
	Habits          []todayHabitDTO `json:"habits"`
	Running         *timeEntryDTO   `json:"running_time_entry"`
	PlannedMinutes  int             `json:"planned_minutes"`
	CalendarMinutes int             `json:"calendar_minutes"`
	TrackedMinutes  int             `json:"tracked_minutes"`
}

func toTodayDTO(t service.Today) todayDTO {
	out := todayDTO{
		Date:            t.Date,
		Timezone:        t.Timezone,
		DayStart:        t.DayStart.UTC().Format(time.RFC3339),
		DayEnd:          t.DayEnd.UTC().Format(time.RFC3339),
		Events:          make([]todayEventDTO, len(t.Events)),
		Tasks:           make([]taskDTO, len(t.Tasks)),
		ActiveTasks:     make([]taskDTO, len(t.ActiveTasks)),
		Habits:          make([]todayHabitDTO, len(t.Habits)),
		PlannedMinutes:  t.PlannedMinutes,
		CalendarMinutes: t.CalendarMinutes,
		TrackedMinutes:  t.TrackedMinutes,
	}
	for i, e := range t.Events {
		out.Events[i] = todayEventDTO{
			calendarEventDTO: toCalendarEventDTO(e.Event),
			OccurrenceStart:  e.Start.UTC().Format(time.RFC3339),
			OccurrenceEnd:    e.End.UTC().Format(time.RFC3339),
		}
	}
	for i, task := range t.Tasks {
		out.Tasks[i] = toTaskDTO(task)
	}
	for i, task := range t.ActiveTasks {
		out.ActiveTasks[i] = toTaskDTO(task)
	}
	for i, h := range t.Habits {
		d := todayHabitDTO{habitDTO: toHabitDTO(h.Habit), Done: h.Occurrence.Done}
		if p := h.Occurrence.Progress; p != nil {
			d.WeekProgress = &weekProgressDTO{Done: p.Done, Target: p.Target}
		}
		out.Habits[i] = d
	}
	if t.Running != nil {
		e := toTimeEntryDTO(*t.Running)
		out.Running = &e
	}
	return out
}

type todayHandlers struct{ svc TodayService }

func (h todayHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/today", h.get)
}

// get liefert den Tageskontext für ?date=YYYY-MM-DD (Standard: heute in der
// konfigurierten Zeitzone).
func (h todayHandlers) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(r.Context(), r.URL.Query().Get("date"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTodayDTO(t))
}
