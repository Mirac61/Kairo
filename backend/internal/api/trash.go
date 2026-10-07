package api

import (
	"context"
	"net/http"
)

// deleteHandler: DELETE legt in den Papierkorb, mit ?permanent=true löscht es endgültig.
func deleteHandler(trash, purge func(ctx context.Context, id string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		del := trash
		if r.URL.Query().Get("permanent") == "true" {
			del = purge
		}
		if err := del(r.Context(), r.PathValue("id")); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// restoreHandler: POST .../restore holt aus dem Papierkorb zurück (idempotent) und liefert das Objekt.
func restoreHandler[T, D any](restore func(ctx context.Context, id string) (T, error), dto func(T) D) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := restore(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, dto(v))
	}
}

type trashHandlers struct {
	tasks  TaskService
	events CalendarService
	habits HabitService
}

func (h trashHandlers) register(mux *http.ServeMux) { mux.HandleFunc("GET /api/trash", h.list) }

// list liefert den Papierkorb, jeweils zuletzt gelöschte zuerst.
func (h trashHandlers) list(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.tasks.ListTrashed(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	events, err := h.events.ListTrashed(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	habits, err := h.habits.ListTrashed(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := struct {
		Tasks  []taskDTO          `json:"tasks"`
		Events []calendarEventDTO `json:"events"`
		Habits []habitDTO         `json:"habits"`
	}{[]taskDTO{}, []calendarEventDTO{}, []habitDTO{}}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, toTaskDTO(t))
	}
	for _, e := range events {
		out.Events = append(out.Events, toCalendarEventDTO(e))
	}
	for _, x := range habits {
		out.Habits = append(out.Habits, toHabitDTO(x))
	}
	writeJSON(w, http.StatusOK, out)
}
