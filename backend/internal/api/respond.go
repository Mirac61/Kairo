package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"kairo/internal/domain"
)

const maxBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError übersetzt Domain-Fehler in HTTP-Status. Interne Fehler werden
// geloggt, aber nicht an den Client durchgereicht.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		slog.Error("interner Fehler", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "interner Fehler"})
	}
}

// decodeJSON liest einen JSON-Body und lehnt unbekannte Felder ab.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeBody(w, r, dst, false)
}

// decodeOptionalJSON wie decodeJSON, akzeptiert aber auch einen leeren Body.
func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeBody(w, r, dst, true)
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !(allowEmpty && errors.Is(err, io.EOF)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ungültiger JSON-Body: " + err.Error()})
		return false
	}
	return true
}
