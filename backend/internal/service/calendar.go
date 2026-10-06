package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"kairo/internal/domain"
)

// CalendarStore ist der Speicher, den der CalendarService braucht.
type CalendarStore interface {
	Create(ctx context.Context, e domain.CalendarEvent) error
	Get(ctx context.Context, id string) (domain.CalendarEvent, error)
	List(ctx context.Context) ([]domain.CalendarEvent, error)
	Update(ctx context.Context, e domain.CalendarEvent) error
	Delete(ctx context.Context, id string) error
}

// CalendarService enthält die Regeln für Kalender-Events.
type CalendarService struct {
	publisher
	store CalendarStore
	loc   *time.Location
	now   func() time.Time
}

// NewCalendarService erzeugt einen CalendarService. loc ist die Zeitzone für
// Wiederholungen und Ausnahmetage (nil = time.Local), now die Uhr (nil = time.Now).
func NewCalendarService(store CalendarStore, loc *time.Location, now func() time.Time) *CalendarService {
	if loc == nil {
		loc = time.Local
	}
	if now == nil {
		now = time.Now
	}
	return &CalendarService{store: store, loc: loc, now: now}
}

// CreateEventInput sind die Felder beim Anlegen. StartAt und EndAt sind
// RFC 3339 und Pflicht; Sekundenbruchteile werden abgeschnitten.
type CreateEventInput struct {
	Title             string
	Description       string
	StartAt           string
	EndAt             string
	Location          string
	URL               string
	ProjectID         *string
	TaskID            *string
	RecurrenceRule    *string
	RecurrenceExdates []string
}

// UpdateEventInput ändert nur die gesetzten (non-nil) Felder. Bei
// ProjectID, TaskID und RecurrenceRule entfernt ein leerer String den Wert;
// ohne Regel werden auch die Ausnahmetage gelöscht. RecurrenceExdates
// ersetzt die ganze Liste.
type UpdateEventInput struct {
	Title             *string
	Description       *string
	StartAt           *string
	EndAt             *string
	Location          *string
	URL               *string
	ProjectID         *string
	TaskID            *string
	RecurrenceRule    *string
	RecurrenceExdates *[]string
}

func (s *CalendarService) Create(ctx context.Context, in CreateEventInput) (domain.CalendarEvent, error) {
	now := s.now().UTC()
	e := domain.CalendarEvent{
		ID:                domain.NewID(),
		Title:             strings.TrimSpace(in.Title),
		Description:       in.Description,
		Location:          in.Location,
		URL:               strings.TrimSpace(in.URL),
		ProjectID:         trimOrNil(in.ProjectID),
		TaskID:            trimOrNil(in.TaskID),
		RecurrenceRule:    cleanRule(in.RecurrenceRule),
		RecurrenceExdates: in.RecurrenceExdates,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	var err error
	if e.StartAt, err = parseEventTime("start_at", in.StartAt); err != nil {
		return domain.CalendarEvent{}, err
	}
	if e.EndAt, err = parseEventTime("end_at", in.EndAt); err != nil {
		return domain.CalendarEvent{}, err
	}
	if err := s.validate(&e); err != nil {
		return domain.CalendarEvent{}, err
	}
	if err := s.store.Create(ctx, e); err != nil {
		return domain.CalendarEvent{}, err
	}
	s.emit(domain.Event{Type: domain.EventCalendarEventCreated, ID: e.ID})
	return e, nil
}

func (s *CalendarService) Get(ctx context.Context, id string) (domain.CalendarEvent, error) {
	return s.store.Get(ctx, id)
}

// List liefert alle Events. Mit from und to (beide oder keins) nur die, von
// denen mindestens ein Termin das Fenster [from, to) berührt.
func (s *CalendarService) List(ctx context.Context, from, to *time.Time) ([]domain.CalendarEvent, error) {
	if (from == nil) != (to == nil) {
		return nil, fmt.Errorf("%w: from und to nur zusammen", domain.ErrInvalid)
	}
	if from != nil && !to.After(*from) {
		return nil, fmt.Errorf("%w: to muss nach from liegen", domain.ErrInvalid)
	}
	all, err := s.store.List(ctx)
	if err != nil || from == nil {
		return all, err
	}
	out := []domain.CalendarEvent{}
	for _, e := range all {
		occ, err := s.occurrences(e, *from, *to)
		if err != nil {
			return nil, err
		}
		if len(occ) > 0 {
			out = append(out, e)
		}
	}
	return out, nil
}

// Occurrences liefert alle konkreten Termine, die das Fenster [from, to)
// berühren, nach Beginn sortiert. Wiederkehrende Events erscheinen einmal je Termin.
func (s *CalendarService) Occurrences(ctx context.Context, from, to time.Time) ([]domain.EventInstance, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("%w: to muss nach from liegen", domain.ErrInvalid)
	}
	all, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []domain.EventInstance{}
	for _, e := range all {
		occ, err := s.occurrences(e, from, to)
		if err != nil {
			return nil, err
		}
		for _, o := range occ {
			out = append(out, domain.EventInstance{Event: e, Start: o.Start, End: o.End})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

func (s *CalendarService) Update(ctx context.Context, id string, in UpdateEventInput) (domain.CalendarEvent, error) {
	e, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.CalendarEvent{}, err
	}
	if in.Title != nil {
		e.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		e.Description = *in.Description
	}
	if in.Location != nil {
		e.Location = *in.Location
	}
	if in.URL != nil {
		e.URL = strings.TrimSpace(*in.URL)
	}
	if in.ProjectID != nil {
		e.ProjectID = trimOrNil(in.ProjectID)
	}
	if in.TaskID != nil {
		e.TaskID = trimOrNil(in.TaskID)
	}
	if in.StartAt != nil {
		if e.StartAt, err = parseEventTime("start_at", *in.StartAt); err != nil {
			return domain.CalendarEvent{}, err
		}
	}
	if in.EndAt != nil {
		if e.EndAt, err = parseEventTime("end_at", *in.EndAt); err != nil {
			return domain.CalendarEvent{}, err
		}
	}
	if in.RecurrenceRule != nil {
		e.RecurrenceRule = cleanRule(in.RecurrenceRule)
		if e.RecurrenceRule == nil {
			e.RecurrenceExdates = nil
		}
	}
	if in.RecurrenceExdates != nil {
		e.RecurrenceExdates = *in.RecurrenceExdates
	}
	if err := s.validate(&e); err != nil {
		return domain.CalendarEvent{}, err
	}
	e.UpdatedAt = s.now().UTC()
	if err := s.store.Update(ctx, e); err != nil {
		return domain.CalendarEvent{}, err
	}
	s.emit(domain.Event{Type: domain.EventCalendarEventUpdated, ID: e.ID})
	return e, nil
}

func (s *CalendarService) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventCalendarEventDeleted, ID: id})
	return nil
}

// occurrences liefert die Termine von e, die das Fenster [from, to) berühren,
// ohne ausgelassene Tage.
func (s *CalendarService) occurrences(e domain.CalendarEvent, from, to time.Time) ([]domain.EventOccurrence, error) {
	if e.RecurrenceRule == nil {
		if e.StartAt.Before(to) && e.EndAt.After(from) {
			return []domain.EventOccurrence{{Start: e.StartAt, End: e.EndAt}}, nil
		}
		return nil, nil
	}
	rule, err := domain.ParseRule(*e.RecurrenceRule, s.loc)
	if err != nil {
		return nil, fmt.Errorf("service: Event %s: %w", e.ID, err)
	}
	skip := map[string]bool{}
	for _, d := range e.RecurrenceExdates {
		skip[d] = true
	}
	dur := e.EndAt.Sub(e.StartAt)
	var out []domain.EventOccurrence
	// Ein Termin berührt das Fenster, wenn er vor to beginnt und nach from endet.
	for _, st := range rule.Starts(e.StartAt.In(s.loc), from.Add(-dur), to) {
		if skip[st.In(s.loc).Format("2006-01-02")] || !st.Add(dur).After(from) {
			continue
		}
		out = append(out, domain.EventOccurrence{Start: st.UTC(), End: st.Add(dur).UTC()})
	}
	return out, nil
}

func (s *CalendarService) validate(e *domain.CalendarEvent) error {
	switch {
	case e.Title == "":
		return fmt.Errorf("%w: title darf nicht leer sein", domain.ErrInvalid)
	case !e.EndAt.After(e.StartAt):
		return fmt.Errorf("%w: end_at muss nach start_at liegen", domain.ErrInvalid)
	}
	if e.URL != "" {
		u, err := url.Parse(e.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: url muss mit http:// oder https:// beginnen", domain.ErrInvalid)
		}
	}
	if e.RecurrenceRule == nil {
		if len(e.RecurrenceExdates) > 0 {
			return fmt.Errorf("%w: recurrence_exdates braucht recurrence_rule", domain.ErrInvalid)
		}
		e.RecurrenceExdates = []string{}
		return nil
	}
	if _, err := domain.ParseRule(*e.RecurrenceRule, s.loc); err != nil {
		return err
	}
	seen := map[string]bool{}
	days := []string{}
	for _, d := range e.RecurrenceExdates {
		if err := validateDate("recurrence_exdates", d); err != nil {
			return err
		}
		if !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	sort.Strings(days)
	e.RecurrenceExdates = days
	return nil
}

// cleanRule trimmt die Regel und entfernt ein optionales "RRULE:"-Präfix.
func cleanRule(v *string) *string {
	r := trimOrNil(v)
	if r == nil {
		return nil
	}
	c := strings.TrimSpace(strings.TrimPrefix(*r, "RRULE:"))
	if c == "" {
		return nil
	}
	return &c
}

func parseEventTime(field, v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s muss ein RFC-3339-Zeitpunkt sein", domain.ErrInvalid, field)
	}
	return t.UTC().Truncate(time.Second), nil
}
