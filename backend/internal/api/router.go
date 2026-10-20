// Package api enthält Router, Handler und Sicherheits-Middleware.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"kairo/internal/auth"
	"kairo/internal/notes"
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
	Goals     GoalService
	Notes     *notes.Store
	Settings  *Settings
	Prefs     PrefsStore
	// Location bestimmt die Ortszeit im ICS-Export; ohne Angabe gilt UTC.
	Location *time.Location
	// GitHub liefert Issues; zusammen mit Projects, Tasks und Resources gibt es /api/github.
	GitHub IssueSource
	// Hub speist /ws mit Ereignissen. Ohne Hub gibt es keinen WebSocket.
	Hub *realtime.Hub
}

// NewRouter baut den HTTP-Handler. Die Security-Middleware läuft vor dem Routing.
func NewRouter(port int, token, version string, webUI http.Handler, svc Services, authH *auth.Handler, authMW func(http.Handler) http.Handler, authEnabled bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health(version))
	if authH != nil {
		mux.HandleFunc("GET /api/auth/setup-required", authH.SetupRequiredHandler)
		mux.HandleFunc("POST /api/auth/register", authH.RegisterHandler)
		mux.HandleFunc("POST /api/auth/login", authH.LoginHandler)
		mux.HandleFunc("POST /api/auth/logout", authH.LogoutHandler)
		mux.HandleFunc("GET /api/auth/me", authH.MeHandler)
	}
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
	if svc.Tasks != nil && svc.Calendar != nil && svc.Habits != nil {
		trashHandlers{svc.Tasks, svc.Calendar, svc.Habits}.register(mux)
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
	if svc.Goals != nil {
		goalHandlers{svc.Goals}.register(mux)
	}
	if svc.Notes != nil {
		noteHandlers{svc.Notes}.register(mux)
	}
	if svc.GitHub != nil && svc.Projects != nil && svc.Tasks != nil && svc.Resources != nil {
		githubHandlers{svc.GitHub, svc.Projects, svc.Tasks, svc.Resources}.register(mux)
	}
	if svc.Projects != nil && svc.Tasks != nil && svc.Calendar != nil && svc.Habits != nil && svc.Time != nil && svc.Resources != nil {
		exportHandlers{svc, svc.Location}.register(mux)
	}
	if svc.Settings != nil {
		settingsHandlers{svc.Settings}.register(mux)
	}
	if svc.Prefs != nil {
		prefsHandlers{svc.Prefs}.register(mux)
	}
	if svc.Hub != nil {
		mux.HandleFunc("GET /ws", handleWebSocket(svc.Hub))
	}
	mux.Handle("/", webUI)

	handler := http.Handler(mux)
	if authMW != nil {
		handler = authMW(handler)
	}
	return Security(port, token, authEnabled, handler)
}

func health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	}
}
