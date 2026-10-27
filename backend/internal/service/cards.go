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
	Review(ctx context.Context, key string, next domain.CardState, g domain.CardGrade, at time.Time, prev *domain.CardState) error
	Undo(ctx context.Context) (string, error)
	History(ctx context.Context, since time.Time) ([]domain.CardLog, error)
	List(ctx context.Context) ([]domain.Card, error)
	Create(ctx context.Context, c domain.Card) error
	Update(ctx context.Context, c domain.Card) error
	Delete(ctx context.Context, id string) error
	DeckColors(ctx context.Context) (map[string]string, error)
	SetDeckColor(ctx context.Context, name, color string) error
	DeckLimits(ctx context.Context) (map[string]int, error)
	SetDeckLimit(ctx context.Context, name string, n int) error
	RenameDeck(ctx context.Context, from, to string) error
	DeleteDeck(ctx context.Context, name string) error
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

// CardView ist eine Karte mit Lernstand. New: noch nie gelernt; Learning: zuletzt „Nochmal“, noch nicht wieder gewusst;
// Due: heute dran (auch neue). Intervals: Tage bis zur nächsten Wiederholung je Bewertung (nochmal … leicht).
type CardView struct {
	domain.Card
	State     domain.CardState
	New       bool
	Learning  bool
	Due       bool
	Intervals [4]int
}

// DueCard ist eine heute fällige Karte.
type DueCard = CardView

// Deck ist ein Fach mit Farbe und Zahlen.
type Deck struct {
	Name                         string
	Color                        string
	Total, Due, NewDue, LearnDue int
	NewPerDay                    int
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
	day := s.today()
	today := day.Format("2006-01-02")
	out, seen := []CardView{}, map[string]bool{}
	add := func(c domain.Card) {
		if seen[c.Key] {
			return
		}
		seen[c.Key] = true
		st, known := states[c.Key]
		v := CardView{Card: c, State: st, New: !known, Learning: known && st.Reps == 0, Due: !known || st.Due <= today}
		for g := range v.Intervals {
			v.Intervals[g] = st.Next(domain.CardGrade(g), day).Interval
		}
		out = append(out, v)
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
	if err := walk(tree); err != nil {
		return nil, err
	}
	return out, s.limitNew(ctx, out)
}

// NewPerDay: so viele neue Karten je Fach und Tag sind fällig, solange das Fach kein eigenes Limit hat (wie bei Anki);
// der Rest kommt an den Folgetagen.
const NewPerDay = 20

func (s *CardService) limits(ctx context.Context) (func(deck string) int, error) {
	m, err := s.store.DeckLimits(ctx)
	return func(deck string) int {
		if n, ok := m[deck]; ok {
			return n
		}
		return NewPerDay
	}, err
}

// limitNew macht neue Karten über dem Tageslimit ihres Fachs nicht fällig. Heute schon begonnene neue Karten zählen mit.
func (s *CardService) limitNew(ctx context.Context, cards []CardView) error {
	d := s.now().In(s.loc)
	log, err := s.store.History(ctx, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, s.loc))
	if err != nil {
		return err
	}
	limit, err := s.limits(ctx)
	if err != nil {
		return err
	}
	deckOf := make(map[string]string, len(cards))
	for _, c := range cards {
		deckOf[c.Key] = c.Deck
	}
	used := map[string]int{}
	for _, l := range log {
		if l.First {
			used[deckOf[l.Key]]++
		}
	}
	for i := range cards {
		if c := &cards[i]; c.New && c.Due {
			c.Due = used[c.Deck] < limit(c.Deck)
			used[c.Deck]++
		}
	}
	return nil
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
	limit, err := s.limits(ctx)
	if err != nil {
		return nil, err
	}
	by := map[string]*Deck{}
	get := func(name string) *Deck {
		if by[name] == nil {
			by[name] = &Deck{Name: name, Color: colors[name], NewPerDay: limit(name)}
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
			if c.Learning {
				d.LearnDue++
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

// SetDeckLimit legt fest, wie viele neue Karten das Fach pro Tag bringt (0 bis 9999).
func (s *CardService) SetDeckLimit(ctx context.Context, name string, n int) error {
	name = strings.TrimSpace(name)
	if name == "" || n < 0 || n > 9999 {
		return fmt.Errorf("%w: Name und Limit 0–9999 nötig", domain.ErrInvalid)
	}
	return s.store.SetDeckLimit(ctx, name, n)
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
	prev, known := states[key]
	next := prev.Next(g, s.today())
	var p *domain.CardState
	if known {
		p = &prev
	}
	return next, s.store.Review(ctx, key, next, g, s.now(), p)
}

// Undo nimmt die letzte Bewertung zurück und liefert den Key der Karte (domain.ErrNotFound: nichts zurückzunehmen).
func (s *CardService) Undo(ctx context.Context) (string, error) { return s.store.Undo(ctx) }

// History liefert die Bewertungen der letzten days Tage für Statistik und „oft vergessen“.
func (s *CardService) History(ctx context.Context, days int) ([]domain.CardLog, error) {
	return s.store.History(ctx, s.today().AddDate(0, 0, -days))
}

// RenameDeck benennt ein Fach um; gibt es das neue schon, landen die Karten dort. Karten aus Notizen bleiben im Fach ihres Ordners.
func (s *CardService) RenameDeck(ctx context.Context, from, to string) error {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return fmt.Errorf("%w: alter und neuer Name nötig", domain.ErrInvalid)
	}
	if from == to {
		return nil
	}
	return s.store.RenameDeck(ctx, from, to)
}

// DeleteDeck löscht ein Fach mit seinen direkt angelegten Karten; Karten aus Notizen bleiben in der Notiz.
func (s *CardService) DeleteDeck(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: Name nötig", domain.ErrInvalid)
	}
	return s.store.DeleteDeck(ctx, name)
}
