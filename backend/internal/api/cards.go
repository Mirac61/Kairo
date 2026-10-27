package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// CardService ist die Geschäftslogik, die die Karteikarten-Handler brauchen.
type CardService interface {
	All(ctx context.Context) ([]service.CardView, error)
	Due(ctx context.Context) ([]service.DueCard, error)
	Decks(ctx context.Context) ([]service.Deck, error)
	Create(ctx context.Context, in service.CardInput) (domain.Card, error)
	Update(ctx context.Context, id string, in service.CardInput) (domain.Card, error)
	Delete(ctx context.Context, id string) error
	SetDeckColor(ctx context.Context, name, color string) error
	SetDeckLimit(ctx context.Context, name string, n int) error
	Review(ctx context.Context, key string, g domain.CardGrade) (domain.CardState, error)
	Undo(ctx context.Context) (string, error)
	History(ctx context.Context, days int) ([]domain.CardLog, error)
	RenameDeck(ctx context.Context, from, to string) error
	DeleteDeck(ctx context.Context, name string) error
}

type cardDTO struct {
	Key       string  `json:"key"`
	ID        *string `json:"id"` // nur bei direkt angelegten Karten
	Deck      string  `json:"deck"`
	Question  string  `json:"question"`
	Answer    string  `json:"answer"`
	NotePath  *string `json:"note_path"`
	New       bool    `json:"new"`
	Learning  bool    `json:"learning"`
	Due       bool    `json:"due"`
	DueDate   *string `json:"due_date"`
	Interval  int     `json:"interval_days"`
	Intervals [4]int  `json:"next_intervals"` // Tage bis zur nächsten Wiederholung je Bewertung 0–3
	CreatedAt *string `json:"created_at"`
}

func toCardDTO(c service.CardView) cardDTO {
	out := cardDTO{Key: c.Key, Deck: c.Deck, Question: c.Question, Answer: c.Answer, New: c.New, Learning: c.Learning, Due: c.Due,
		Interval: c.State.Interval, Intervals: c.Intervals}
	if !c.CreatedAt.IsZero() {
		at := c.CreatedAt.Format(time.RFC3339)
		out.CreatedAt = &at
	}
	if c.ID != "" {
		out.ID = &c.ID
	}
	if c.NotePath != "" {
		out.NotePath = &c.NotePath
	}
	if !c.New {
		out.DueDate = &c.State.Due
	}
	return out
}

type deckDTO struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	Total  int    `json:"total"`
	Due    int    `json:"due"`
	NewDue int    `json:"new_due"`
	Learn  int    `json:"learn_due"`
	Limit  int    `json:"new_per_day"`
}

type cardHandlers struct{ svc CardService }

func (h cardHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cards", h.list(h.svc.All))
	mux.HandleFunc("GET /api/cards/due", h.list(h.svc.Due))
	mux.HandleFunc("POST /api/cards", h.create)
	mux.HandleFunc("PUT /api/cards/{id}", h.update)
	mux.HandleFunc("DELETE /api/cards/{id}", deleteHandler(h.svc.Delete, h.svc.Delete))
	mux.HandleFunc("POST /api/cards/{key}/review", h.review)
	mux.HandleFunc("GET /api/cards/decks", h.decks)
	mux.HandleFunc("PUT /api/cards/decks/{name}", h.setDeck)
	mux.HandleFunc("PATCH /api/cards/decks/{name}", h.renameDeck)
	mux.HandleFunc("DELETE /api/cards/decks/{name}", h.deleteDeck)
	mux.HandleFunc("POST /api/cards/undo", h.undo)
	mux.HandleFunc("GET /api/cards/log", h.log)
}

// renameDeck: {"name": "Neuer Name"}.
func (h cardHandlers) renameDeck(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.RenameDeck(r.Context(), r.PathValue("name"), in.Name); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h cardHandlers) deleteDeck(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteDeck(r.Context(), r.PathValue("name")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// undo nimmt die letzte Bewertung zurück und liefert {"key": …}; 404, wenn es keine gibt.
func (h cardHandlers) undo(w http.ResponseWriter, r *http.Request) {
	key, err := h.svc.Undo(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}

type cardLogDTO struct {
	Key   string `json:"key"`
	Grade int    `json:"grade"`
	At    string `json:"at"`
}

// log liefert die Bewertungen des letzten Jahres, älteste zuerst.
func (h cardHandlers) log(w http.ResponseWriter, r *http.Request) {
	ls, err := h.svc.History(r.Context(), 366)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, ls, func(l domain.CardLog) cardLogDTO { return cardLogDTO{l.Key, int(l.Grade), l.At.Format(time.RFC3339)} })
}

func (h cardHandlers) list(get func(context.Context) ([]service.CardView, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := get(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeList(w, cs, toCardDTO)
	}
}

func (h cardHandlers) decks(w http.ResponseWriter, r *http.Request) {
	ds, err := h.svc.Decks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, ds, func(d service.Deck) deckDTO {
		return deckDTO{d.Name, d.Color, d.Total, d.Due, d.NewDue, d.LearnDue, d.NewPerDay}
	})
}

type cardInputDTO struct {
	Deck     string `json:"deck"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func (h cardHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in cardInputDTO
	if !decodeJSON(w, r, &in) {
		return
	}
	c, err := h.svc.Create(r.Context(), service.CardInput(in))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCardDTO(service.CardView{Card: c, New: true, Due: true}))
}

func (h cardHandlers) update(w http.ResponseWriter, r *http.Request) {
	var in cardInputDTO
	if !decodeJSON(w, r, &in) {
		return
	}
	c, err := h.svc.Update(r.Context(), r.PathValue("id"), service.CardInput(in))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCardDTO(service.CardView{Card: c}))
}

// setDeck legt Farbe und/oder Limit neuer Karten pro Tag eines Fachs fest: {"color": "violet", "new_per_day": 20}.
func (h cardHandlers) setDeck(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Color *string `json:"color"`
		Limit *int    `json:"new_per_day"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	var err error
	switch {
	case in.Color == nil && in.Limit == nil:
		err = fmt.Errorf("%w: color oder new_per_day nötig", domain.ErrInvalid)
	case in.Color != nil:
		err = h.svc.SetDeckColor(r.Context(), r.PathValue("name"), *in.Color)
	}
	if err == nil && in.Limit != nil {
		err = h.svc.SetDeckLimit(r.Context(), r.PathValue("name"), *in.Limit)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
