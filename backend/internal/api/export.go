package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kairo/internal/domain"
	"kairo/internal/ics"
)

// exportHandlers liefern die Daten als Download. Notizen sind nicht dabei:
// das sind normale Dateien im Notizordner. Der Papierkorb bleibt außen vor.
type exportHandlers struct {
	svc Services
	loc *time.Location
}

func (h exportHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/export.json", h.json)
	mux.HandleFunc("GET /api/export.ics", h.ics)
	mux.HandleFunc("GET /api/export.csv", h.csv)
}

// attachment setzt Content-Type und Dateinamen, damit der Browser speichert statt anzeigt.
func attachment(w http.ResponseWriter, contentType, ext string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="kairo-`+time.Now().Format("2006-01-02")+"."+ext+`"`)
}

func (h exportHandlers) json(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := struct {
		ExportedAt       string               `json:"exported_at"`
		Projects         []projectDTO         `json:"projects"`
		Tasks            []taskDTO            `json:"tasks"`
		Habits           []habitDTO           `json:"habits"`
		HabitCompletions []habitCompletionDTO `json:"habit_completions"`
		CalendarEvents   []calendarEventDTO   `json:"calendar_events"`
		TimeEntries      []timeEntryDTO       `json:"time_entries"`
		Resources        []resourceDTO        `json:"resources"`
	}{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Projects:   []projectDTO{}, Tasks: []taskDTO{}, Habits: []habitDTO{}, HabitCompletions: []habitCompletionDTO{},
		CalendarEvents: []calendarEventDTO{}, TimeEntries: []timeEntryDTO{}, Resources: []resourceDTO{},
	}
	projects, err := h.svc.Projects.List(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, err := h.svc.Tasks.List(ctx, domain.TaskFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	habits, err := h.svc.Habits.List(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	events, err := h.svc.Calendar.List(ctx, nil, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	entries, err := h.svc.Time.List(ctx, domain.TimeEntryFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	resources, err := h.svc.Resources.List(ctx, domain.ResourceFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	for _, p := range projects {
		out.Projects = append(out.Projects, toProjectDTO(p))
	}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, toTaskDTO(t))
	}
	for _, x := range habits {
		out.Habits = append(out.Habits, toHabitDTO(x))
		cs, err := h.svc.Habits.Completions(ctx, x.ID, "", "")
		if err != nil {
			writeError(w, err)
			return
		}
		for _, c := range cs {
			out.HabitCompletions = append(out.HabitCompletions, toHabitCompletionDTO(c))
		}
	}
	for _, e := range events {
		out.CalendarEvents = append(out.CalendarEvents, toCalendarEventDTO(e))
	}
	for _, e := range entries {
		out.TimeEntries = append(out.TimeEntries, toTimeEntryDTO(e))
	}
	for _, x := range resources {
		out.Resources = append(out.Resources, toResourceDTO(x))
	}
	attachment(w, "application/json", "json")
	writeJSON(w, http.StatusOK, out)
}

// ics exportiert Termine als VEVENT und Aufgaben mit Fälligkeit als VTODO.
func (h exportHandlers) ics(w http.ResponseWriter, r *http.Request) {
	events, err := h.svc.Calendar.List(r.Context(), nil, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, err := h.svc.Tasks.List(r.Context(), domain.TaskFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	var evs []ics.Event
	for _, e := range events {
		uid := e.ID + "@kairo"
		if e.ExternalUID != nil {
			uid = *e.ExternalUID
		}
		ev := ics.Event{UID: uid, Summary: e.Title, Description: e.Description, Location: e.Location, Start: e.StartAt, End: e.EndAt, Exdates: e.RecurrenceExdates}
		if e.RecurrenceRule != nil {
			ev.Rule = *e.RecurrenceRule
		}
		evs = append(evs, ev)
	}
	status := map[domain.TaskStatus]string{domain.TaskCompleted: "COMPLETED", domain.TaskCancelled: "CANCELLED", domain.TaskInProgress: "IN-PROCESS"}
	var todos []ics.Todo
	for _, t := range tasks {
		if t.DueAt == nil {
			continue
		}
		st := status[t.Status]
		if st == "" {
			st = "NEEDS-ACTION"
		}
		todos = append(todos, ics.Todo{UID: t.ID + "@kairo", Summary: t.Title, Description: t.Description, Due: *t.DueAt, Status: st})
	}
	loc := h.loc
	if loc == nil {
		loc = time.UTC
	}
	attachment(w, "text/calendar; charset=utf-8", "ics")
	_, _ = w.Write([]byte(ics.Format(evs, todos, time.Now(), loc)))
}

// csv exportiert die Aufgaben, eine Zeile je Task.
func (h exportHandlers) csv(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.svc.Tasks.List(r.Context(), domain.TaskFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	projects, err := h.svc.Projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	names := map[string]string{}
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	attachment(w, "text/csv; charset=utf-8", "csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "title", "status", "priority", "project", "due_at", "planned_date", "estimated_minutes", "completed_at", "description"})
	for _, t := range tasks {
		dto := toTaskDTO(t)
		project := ""
		if t.ProjectID != nil {
			project = names[*t.ProjectID]
		}
		_ = cw.Write([]string{
			t.ID, safeCell(t.Title), dto.Status, dto.Priority, safeCell(project), deref(dto.DueAt), deref(dto.PlannedDate),
			strconv.Itoa(t.EstimatedMinutes), deref(dto.CompletedAt), safeCell(t.Description),
		})
	}
	cw.Flush()
}

// safeCell entschärft Zellen, die Tabellenprogramme als Formel auswerten würden.
func safeCell(s string) string {
	if strings.ContainsAny(s[:min(1, len(s))], "=+-@\t\r") {
		return "'" + s
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
