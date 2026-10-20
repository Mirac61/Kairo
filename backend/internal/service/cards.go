package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
	"kairo/internal/notes"
)

// CardNotes ist der Notizordner, aus dem die Karten kommen (notes.Store).
type CardNotes interface {
	Tree() ([]notes.Node, error)
	Read(p string) (content, version string, err error)
}

// CardStore speichert den Lernstand.
type CardStore interface {
	States(ctx context.Context) (map[string]domain.CardState, error)
	Save(ctx context.Context, key string, c domain.CardState) error
}

// CardService: Karteikarten aus den Notizen mit Wiederholung nach SM-2.
type CardService struct {
	notes CardNotes
	store CardStore
	loc   *time.Location
	now   func() time.Time
}

func NewCardService(n CardNotes, store CardStore, loc *time.Location, now func() time.Time) *CardService {
	if loc == nil {
		loc = time.Local
	}
	if now == nil {
		now = time.Now
	}
	return &CardService{notes: n, store: store, loc: loc, now: now}
}

// DueCard ist eine heute fällige Karte; New: noch nie gelernt.
type DueCard struct {
	domain.Card
	New bool
}

func (s *CardService) today() time.Time {
	d := s.now().In(s.loc)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// Due liefert alle Karten, die heute fällig oder neu sind, in Reihenfolge der Notizen.
// shortcut: liest bei jedem Aufruf alle Notizen; bei einem großen Ordner einen Index wie für die Rückverweise bauen.
func (s *CardService) Due(ctx context.Context) ([]DueCard, error) {
	tree, err := s.notes.Tree()
	if err != nil {
		return nil, err
	}
	states, err := s.store.States(ctx)
	if err != nil {
		return nil, err
	}
	today := s.today().Format("2006-01-02")
	out, seen := []DueCard{}, map[string]bool{}
	var walk func([]notes.Node) error
	walk = func(ns []notes.Node) error {
		for _, n := range ns {
			if n.Dir {
				if err := walk(n.Children); err != nil {
					return err
				}
				continue
			}
			if !strings.HasSuffix(strings.ToLower(n.Path), ".md") {
				continue
			}
			content, _, err := s.notes.Read(n.Path)
			if err != nil {
				return fmt.Errorf("Karten aus %s: %w", n.Path, err)
			}
			for _, c := range domain.ParseCards(n.Path, content) {
				st, known := states[c.Key]
				// Gleiche Frage in zwei Notizen teilt sich den Lernstand und kommt nur einmal.
				if seen[c.Key] || known && st.Due > today {
					continue
				}
				seen[c.Key] = true
				out = append(out, DueCard{Card: c, New: !known})
			}
		}
		return nil
	}
	return out, walk(tree)
}

// Review speichert eine Antwort und liefert den neuen Lernstand.
func (s *CardService) Review(ctx context.Context, key string, g domain.CardGrade) (domain.CardState, error) {
	if key == "" || g < domain.GradeAgain || g > domain.GradeEasy {
		return domain.CardState{}, fmt.Errorf("%w: key und grade 0–3 nötig", domain.ErrInvalid)
	}
	states, err := s.store.States(ctx)
	if err != nil {
		return domain.CardState{}, err
	}
	next := states[key].Next(g, s.today())
	return next, s.store.Save(ctx, key, next)
}
