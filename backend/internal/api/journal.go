package api

import (
	"context"
	"net/http"

	"kairo/internal/service"
)

type JournalSummaryService interface {
	Summary(ctx context.Context) (service.JournalSummary, error)
}

type journalSummaryDTO struct {
	Date   string `json:"date"`
	Total  int    `json:"total"`
	Streak int    `json:"streak"`
}

type journalHandlers struct{ svc JournalSummaryService }

func (h journalHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/journal/summary", h.summary)
}

func (h journalHandlers) summary(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Summary(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, journalSummaryDTO{Date: result.Date, Total: result.Total, Streak: result.Streak})
}
