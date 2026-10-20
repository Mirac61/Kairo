package api

import (
	"context"
	"net/http"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// CardService ist die Geschäftslogik, die die Karteikarten-Handler brauchen.
type CardService interface {
	Due(ctx context.Context) ([]service.DueCard, error)
	Review(ctx context.Context, key string, g domain.CardGrade) (domain.CardState, error)
}

type dueCardDTO struct {
	Key      string `json:"key"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	NotePath string `json:"note_path"`
	New      bool   `json:"new"`
}

type cardHandlers struct{ svc CardService }

func (h cardHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cards/due", h.due)
	mux.HandleFunc("POST /api/cards/{key}/review", h.review)
}

func (h cardHandlers) due(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.Due(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, cs, func(c service.DueCard) dueCardDTO {
		return dueCardDTO{c.Key, c.Question, c.Answer, c.NotePath, c.New}
	})
}

// review nimmt {"grade": 0–3} (nochmal, schwer, gut, leicht) und liefert den nächsten Termin.
func (h cardHandlers) review(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Grade *int `json:"grade"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	g := -1
	if in.Grade != nil {
		g = *in.Grade
	}
	st, err := h.svc.Review(r.Context(), r.PathValue("key"), domain.CardGrade(g))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"due": st.Due, "interval_days": st.Interval})
}
