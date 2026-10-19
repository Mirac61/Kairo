package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"kairo/internal/domain"
)

// PrefsStore hält die Einstellungen der Oberfläche (ein JSON-Objekt), damit Handy und Rechner gleich aussehen.
type PrefsStore interface {
	Get(ctx context.Context) (json.RawMessage, error)
	Put(ctx context.Context, data json.RawMessage) error
}

const maxPrefsBytes = 16 << 10

type prefsHandlers struct{ store PrefsStore }

func (h prefsHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/prefs", h.get)
	mux.HandleFunc("PUT /api/prefs", h.put)
}

func (h prefsHandlers) get(w http.ResponseWriter, r *http.Request) {
	data, err := h.store.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// put ersetzt das ganze Objekt. Die Schlüssel kennt nur die WebUI; der Server prüft nur Form und Größe.
func (h prefsHandlers) put(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if !decodeJSON(w, r, &in) {
		return
	}
	if in == nil {
		writeError(w, fmt.Errorf("%w: erwartet ein JSON-Objekt", domain.ErrInvalid))
		return
	}
	data, err := json.Marshal(in)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(data) > maxPrefsBytes {
		writeError(w, fmt.Errorf("%w: Einstellungen größer als 16 KB", domain.ErrInvalid))
		return
	}
	if err := h.store.Put(r.Context(), data); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
