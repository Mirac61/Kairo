package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
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

// CardStore speichert Lernstand, direkt angelegte Karten und Fachfarben.
type CardStore interface {
	States(ctx context.Context) (map[string]domain.CardState, error)
	Save(ctx context.Context, key string, c domain.CardState) error
	List(ctx context.Context) ([]domain.Card, error)
	Create(ctx context.Context, c domain.Card) error
	Update(ctx context.Context, c domain.Card) error
	Delete(ctx context.Context, id string) error
	DeckColors(ctx context.Context) (map[string]string, error)
	SetDeckColor(ctx context.Context, name, color string) error
}

// CardService: Karteikarten aus den Notizen und direkt angelegte, nach Fächern, mit Wiederholung nach SM-2.
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

// CardView ist eine Karte mit Lernstand. New: noch nie gelernt; Due: heute dran (auch neue).
type CardView struct {
	domain.Card
	State domain.CardState
	New   bool
	Due   bool
}

// DueCard ist eine heute fällige Karte.
type DueCard = CardView

// Deck ist ein Fach mit Farbe und Zahlen.
type Deck struct {
	Name               string
	Color              string
	Total, Due, NewDue int
}

func (s *CardService) today() time.Time {
	d := s.now().In(s.loc)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// All liefert alle Karten: erst die direkt angelegten, dann die aus Notizen in Reihenfolge des Ordners.
// Gleiche Frage zweimal: nur die erste zählt (sie teilen sich den Lernstand).
// shortcut: liest bei jedem Aufruf alle Notizen; bei einem großen Ordner einen Index wie für die Rückverweise bauen.
func (s *CardService) All(ctx context.Context) ([]CardView, error) {
	states, err := s.store.States(ctx)
	if err != nil {
		return nil, err
	}
	own, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := s.notes.Tree()
	if err != nil {
		return nil, err
	}
	today := s.today().Format("2006-01-02")
	out, seen := []CardView{}, map[string]bool{}
	add := func(c domain.Card) {
		if seen[c.Key] {
			return
		}
		seen[c.Key] = true
		st, known := states[c.Key]
		out = append(out, CardView{Card: c, State: st, New: !known, Due: !known || st.Due <= today})
	}
	for _, c := range own {
		add(c)
	}
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
				add(c)
			}
		}
		return nil
	}
	return out, walk(tree)
}

// Due liefert die heute fälligen Karten.
func (s *CardService) Due(ctx context.Context) ([]DueCard, error) {
	all, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, c := range all {
		if c.Due {
			out = append(out, c)
		}
	}
	return out, nil
}

// Decks fasst die Karten nach Fach zusammen; Fächer ohne Karten, aber mit Farbe, zählen mit.
func (s *CardService) Decks(ctx context.Context) ([]Deck, error) {
	all, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	colors, err := s.store.DeckColors(ctx)
	if err != nil {
		return nil, err
	}
	by := map[string]*Deck{}
	get := func(name string) *Deck {
		if by[name] == nil {
			by[name] = &Deck{Name: name, Color: colors[name]}
		}
		return by[name]
	}
	for name := range colors {
		get(name)
	}
	for _, c := range all {
		d := get(c.Deck)
		d.Total++
		if c.Due {
			d.Due++
			if c.New {
				d.NewDue++
			}
		}
	}
	out := make([]Deck, 0, len(by))
	for _, d := range by {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	for i := range out {
		if out[i].Color == "" {
			// Ohne gewählte Farbe: fest aus dem Namen, damit sie sich nicht verschiebt, wenn Fächer dazukommen.
			h := fnv.New32a()
			h.Write([]byte(out[i].Name))
			out[i].Color = domain.ProjectColors[h.Sum32()%uint32(len(domain.ProjectColors))]
		}
	}
	return out, nil
}

// CardInput: Fach, Frage und Antwort einer direkt angelegten Karte.
type CardInput struct {
	Deck, Question, Answer string
}

func (in CardInput) card(id string) (domain.Card, error) {
	c := domain.Card{ID: id, Deck: strings.TrimSpace(in.Deck), Question: strings.TrimSpace(in.Question), Answer: strings.TrimSpace(in.Answer)}
	if c.Deck == "" || c.Question == "" || c.Answer == "" {
		return c, fmt.Errorf("%w: deck, question und answer nötig", domain.ErrInvalid)
	}
	c.Key = domain.CardKey(c.Question)
	return c, nil
}

func (s *CardService) Create(ctx context.Context, in CardInput) (domain.Card, error) {
	c, err := in.card(domain.NewID())
	if err != nil {
		return c, err
	}
	return c, s.store.Create(ctx, c)
}

// Update ändert eine direkt angelegte Karte. Eine neue Frage beginnt mit neuem Lernstand.
func (s *CardService) Update(ctx context.Context, id string, in CardInput) (domain.Card, error) {
	c, err := in.card(id)
	if err != nil {
		return c, err
	}
	return c, s.store.Update(ctx, c)
}

func (s *CardService) Delete(ctx context.Context, id string) error { return s.store.Delete(ctx, id) }

// SetDeckColor wählt die Farbe eines Fachs (aus domain.ProjectColors); so entsteht auch ein leeres Fach.
func (s *CardService) SetDeckColor(ctx context.Context, name, color string) error {
	name = strings.TrimSpace(name)
	if name == "" || !domain.ValidProjectColor(color) {
		return fmt.Errorf("%w: Name und Farbe aus der Palette nötig", domain.ErrInvalid)
	}
	return s.store.SetDeckColor(ctx, name, color)
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
