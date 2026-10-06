// Package api enthält Router, Handler und Sicherheits-Middleware.
package api

import (
	"encoding/json"
	"net/http"

	"kairo/internal/realtime"
)

// Services bündelt die Geschäftslogik, die der Router an die Handler gibt.
// Nicht gesetzte Services registrieren ihre Routen nicht.
type Services struct {
	Projects  ProjectService
	Tasks     TaskService
	Calendar  CalendarService
	Habits    HabitService
	Resources ResourceService
	Time      TimeTrackingService
	Today     TodayService
	Review    ReviewService
	// Hub speist /ws mit Ereignissen. Ohne Hub gibt es keinen WebSocket.
	Hub *realtime.Hub
}

// NewRouter baut den HTTP-Handler. Die Security-Middleware läuft vor dem Routing.
func NewRouter(port int, token, version string, webUI http.Handler, svc Services) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health(version))
	if svc.Projects != nil {
		projectHandlers{svc.Projects}.register(mux)
	}
	if svc.Tasks != nil {
		taskHandlers{svc.Tasks}.register(mux)
	}
	if svc.Calendar != nil {
		calendarHandlers{svc.Calendar}.register(mux)
	}
	if svc.Habits != nil {
		habitHandlers{svc.Habits}.register(mux)
	}
	if svc.Resources != nil {
		resourceHandlers{svc.Resources}.register(mux)
	}
	if svc.Time != nil {
		timeHandlers{svc.Time}.register(mux)
	}
	if svc.Today != nil {
		todayHandlers{svc.Today}.register(mux)
	}
	if svc.Review != nil {
		reviewHandlers{svc.Review}.register(mux)
	}
	if svc.Hub != nil {
		mux.HandleFunc("GET /ws", handleWebSocket(svc.Hub))
	}
	mux.Handle("/", webUI)
	return Security(port, token, mux)
}

func health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	}
}
