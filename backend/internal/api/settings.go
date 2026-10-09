package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"kairo/internal/config"
)

// Settings verbindet /api/settings mit config.json. Geänderte Werte gelten
// erst nach einem Neustart, den Restart auslöst.
type Settings struct {
	Running config.Config
	Getenv  func(string) string
	Restart func()
}

type settingsHandlers struct{ s *Settings }

func (h settingsHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", h.get)
	mux.HandleFunc("PUT /api/settings", h.put)
	mux.HandleFunc("POST /api/restart", h.restart)
}

// get liefert die Werte, die nach einem Neustart gelten, und welche davon
// eine Umgebungsvariable festlegt (die WebUI kann sie dann nicht ändern).
func (h settingsHandlers) get(w http.ResponseWriter, _ *http.Request) {
	next, err := config.Load(h.s.Getenv)
	if err != nil {
		writeError(w, err)
		return
	}
	env := map[string]string{}
	for field, key := range config.FileKeys {
		if h.s.Getenv(key) != "" {
			env[field] = key
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":       next.File(),
		"env":            env,
		"path":           next.ConfigPath,
		"restart_needed": next.File() != h.s.Running.File(),
	})
}

func (h settingsHandlers) put(w http.ResponseWriter, r *http.Request) {
	var f config.File
	if !decodeJSON(w, r, &f) {
		return
	}
	if f.NotesDir == "~" || strings.HasPrefix(f.NotesDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, err)
			return
		}
		f.NotesDir = filepath.Join(home, f.NotesDir[1:])
	}
	if err := f.Validate(); err != nil {
		var ce *config.Error
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeError(w, err)
		return
	}
	if err := config.WriteFile(h.s.Running.ConfigPath, f); err != nil {
		writeError(w, err)
		return
	}
	h.get(w, r)
}

func (h settingsHandlers) restart(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	h.s.Restart()
}
