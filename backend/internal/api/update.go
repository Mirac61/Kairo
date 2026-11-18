package api

import (
	"net/http"

	"kairo/internal/update"
)

// Updates verbindet /api/update mit GitHub: GET prüft die neueste Fassung,
// POST installiert sie und startet den Server neu, damit sie läuft. Auf einem
// Raspberry Pi ist beides aus (Disabled), dort aktualisiert ./deploy.sh.
type Updates struct {
	Client   *update.Client
	Current  string
	Disabled bool
	Restart  func()
}

type updateHandlers struct{ u *Updates }

func (h updateHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/update", h.get)
	mux.HandleFunc("POST /api/update", h.install)
}

func (h updateHandlers) get(w http.ResponseWriter, r *http.Request) {
	if h.u.Disabled {
		writeJSON(w, http.StatusOK, update.Status{Current: h.u.Current, Disabled: true})
		return
	}
	st, err := h.u.Client.Status(r.Context(), h.u.Current)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// install antwortet mit 202; der Neustart beendet den Server, dispatch startet
// dieselbe Datei – inzwischen die neue Fassung.
func (h updateHandlers) install(w http.ResponseWriter, r *http.Request) {
	if h.u.Disabled {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Selbst-Updates sind aus; auf dem Raspberry Pi aktualisiert ./deploy.sh",
		})
		return
	}
	rel, err := h.u.Client.Latest(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if !update.Newer(rel.Version, h.u.Current) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Kairo ist bereits aktuell."})
		return
	}
	if _, err := h.u.Client.Install(r.Context(), rel); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	h.u.Restart()
}
