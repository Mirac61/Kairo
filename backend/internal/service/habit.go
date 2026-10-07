package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
)

// HabitStore ist der Speicher, den der HabitService braucht.
type HabitStore interface {
	Create(ctx context.Context, h domain.Habit) error
	Get(ctx context.Context, id string) (domain.Habit, error)
	List(ctx context.Context) ([]domain.Habit, error)
	Update(ctx context.Context, h domain.Habit) error
	Delete(ctx context.Context, id string) error
	Trash(ctx context.Context, id string, at time.Time) error
	Restore(ctx context.Context, id string) (domain.Habit, error)
	ListTrashed(ctx context.Context) ([]domain.Habit, error)
	CreateCompletion(ctx context.Context, c domain.HabitCompletion) error
	DeleteCompletion(ctx context.Context, habitID, date string) error
	ListCompletions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error)
}

// HabitService enthält die Regeln für Habits und Completions.
type HabitService struct {
	publisher
	store HabitStore
	loc   *time.Location
	now   func() time.Time
}

// NewHabitService erzeugt einen HabitService. loc ist die Zeitzone für "heute"
// (nil = time.Local), now die Uhr (nil = time.Now).
func NewHabitService(store HabitStore, loc *time.Location, now func() time.Time) *HabitService {
	if loc == nil {
		loc = time.Local
	}
	if now == nil {
		now = time.Now
	}
	return &HabitService{store: store, loc: loc, now: now}
}

// CreateHabitInput sind die Felder beim Anlegen. StartDate ist standardmäßig
// heute, Active standardmäßig true.
type CreateHabitInput struct {
	Name            string
	Description     string
	FrequencyType   domain.FrequencyType
	FrequencyConfig domain.FrequencyConfig
	TargetValue     *int
	Unit            string
	PreferredTime   *string
	StartDate       *string
	EndDate         *string
	Active          *bool
}

// UpdateHabitInput ändert nur die gesetzten (non-nil) Felder. TargetValue 0,
// PreferredTime "" und EndDate "" entfernen den Wert. Wird FrequencyType ohne
// FrequencyConfig gesetzt, startet die Konfiguration leer (Standardwerte).
type UpdateHabitInput struct {
	Name            *string
	Description     *string
	FrequencyType   *domain.FrequencyType
	FrequencyConfig *domain.FrequencyConfig
	TargetValue     *int
	Unit            *string
	PreferredTime   *string
	StartDate       *string
	EndDate         *string
	Active          *bool
}

// CompleteHabitInput beschreibt eine Completion. Date ist ein lokaler Tag
// (leer = heute).
type CompleteHabitInput struct {
	Date  string
	Value *int
	Note  string
}

// HabitDay ist ein an einem Tag fälliges Habit mit seiner berechneten Occurrence.
type HabitDay struct {
	Habit      domain.Habit
	Occurrence domain.HabitOccurrence
}

func (s *HabitService) today() string { return s.now().In(s.loc).Format("2006-01-02") }

func (s *HabitService) Create(ctx context.Context, in CreateHabitInput) (domain.Habit, error) {
	h := domain.Habit{
		ID:            domain.NewID(),
		Name:          strings.TrimSpace(in.Name),
		Description:   in.Description,
		FrequencyType: in.FrequencyType,
		TargetValue:   in.TargetValue,
		Unit:          strings.TrimSpace(in.Unit),
		PreferredTime: trimOrNil(in.PreferredTime),
		StartDate:     s.today(),
		EndDate:       trimOrNil(in.EndDate),
		Active:        true,
		CreatedAt:     s.now().UTC(),
	}
	if sd := trimOrNil(in.StartDate); sd != nil {
		h.StartDate = *sd
	}
	if in.Active != nil {
		h.Active = *in.Active
	}
	if err := normalizeHabit(&h, in.FrequencyConfig); err != nil {
		return domain.Habit{}, err
	}
	if err := s.store.Create(ctx, h); err != nil {
		return domain.Habit{}, err
	}
	s.emit(domain.Event{Type: domain.EventHabitCreated, ID: h.ID})
	return h, nil
}

func (s *HabitService) Get(ctx context.Context, id string) (domain.Habit, error) {
	return s.store.Get(ctx, id)
}

func (s *HabitService) List(ctx context.Context) ([]domain.Habit, error) {
	return s.store.List(ctx)
}

func (s *HabitService) Update(ctx context.Context, id string, in UpdateHabitInput) (domain.Habit, error) {
	h, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Habit{}, err
	}
	cfg := h.FrequencyConfig
	if in.Name != nil {
		h.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		h.Description = *in.Description
	}
	if in.FrequencyType != nil && *in.FrequencyType != h.FrequencyType {
		h.FrequencyType, cfg = *in.FrequencyType, domain.FrequencyConfig{}
	}
	if in.FrequencyConfig != nil {
		cfg = *in.FrequencyConfig
	}
	if in.TargetValue != nil {
		h.TargetValue = in.TargetValue
		if *in.TargetValue == 0 {
			h.TargetValue = nil
		}
	}
	if in.Unit != nil {
		h.Unit = strings.TrimSpace(*in.Unit)
	}
	if in.PreferredTime != nil {
		h.PreferredTime = trimOrNil(in.PreferredTime)
	}
	if in.StartDate != nil {
		h.StartDate = strings.TrimSpace(*in.StartDate)
	}
	if in.EndDate != nil {
		h.EndDate = trimOrNil(in.EndDate)
	}
	if in.Active != nil {
		h.Active = *in.Active
	}
	if err := normalizeHabit(&h, cfg); err != nil {
		return domain.Habit{}, err
	}
	if err := s.store.Update(ctx, h); err != nil {
		return domain.Habit{}, err
	}
	s.emit(domain.Event{Type: domain.EventHabitUpdated, ID: h.ID})
	return h, nil
}

// Delete löscht endgültig, samt Completions.
func (s *HabitService) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventHabitDeleted, ID: id})
	return nil
}

// Trash legt das Habit in den Papierkorb; die Completions bleiben für die Wiederherstellung erhalten.
func (s *HabitService) Trash(ctx context.Context, id string) error {
	if err := s.store.Trash(ctx, id, s.now()); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventHabitDeleted, ID: id})
	return nil
}

func (s *HabitService) Restore(ctx context.Context, id string) (domain.Habit, error) {
	h, err := s.store.Restore(ctx, id)
	if err != nil {
		return domain.Habit{}, err
	}
	s.emit(domain.Event{Type: domain.EventHabitCreated, ID: id})
	return h, nil
}

func (s *HabitService) ListTrashed(ctx context.Context) ([]domain.Habit, error) {
	return s.store.ListTrashed(ctx)
}

// Complete hakt ein Habit an einem Tag ab. Der Tag muss zwischen start_date
// und end_date liegen und darf nicht in der Zukunft liegen; das Habit muss
// aktiv sein. Er muss nicht fällig sein, man darf auch "extra" abhaken.
func (s *HabitService) Complete(ctx context.Context, habitID string, in CompleteHabitInput) (domain.HabitCompletion, error) {
	h, err := s.store.Get(ctx, habitID)
	if err != nil {
		return domain.HabitCompletion{}, err
	}
	date := strings.TrimSpace(in.Date)
	if date == "" {
		date = s.today()
	}
	if err := validateDate("date", date); err != nil {
		return domain.HabitCompletion{}, err
	}
	switch {
	case !h.Active:
		return domain.HabitCompletion{}, fmt.Errorf("%w: Habit ist nicht aktiv", domain.ErrInvalid)
	case date > s.today():
		return domain.HabitCompletion{}, fmt.Errorf("%w: date liegt in der Zukunft", domain.ErrInvalid)
	case date < h.StartDate:
		return domain.HabitCompletion{}, fmt.Errorf("%w: date liegt vor start_date", domain.ErrInvalid)
	case h.EndDate != nil && date > *h.EndDate:
		return domain.HabitCompletion{}, fmt.Errorf("%w: date liegt nach end_date", domain.ErrInvalid)
	case in.Value != nil && *in.Value < 0:
		return domain.HabitCompletion{}, fmt.Errorf("%w: value darf nicht negativ sein", domain.ErrInvalid)
	}
	c := domain.HabitCompletion{
		ID:          domain.NewID(),
		HabitID:     habitID,
		Date:        date,
		Value:       in.Value,
		CompletedAt: s.now().UTC(),
		Note:        in.Note,
	}
	if err := s.store.CreateCompletion(ctx, c); err != nil {
		return domain.HabitCompletion{}, err
	}
	s.emit(domain.Event{Type: domain.EventHabitCompleted, ID: habitID})
	return c, nil
}

// Uncomplete entfernt die Completion eines Habits an einem Tag.
func (s *HabitService) Uncomplete(ctx context.Context, habitID, date string) error {
	if err := validateDate("date", date); err != nil {
		return err
	}
	if err := s.store.DeleteCompletion(ctx, habitID, date); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventHabitUncompleted, ID: habitID})
	return nil
}

// Completions liefert die Completions eines Habits, optional auf [from, to]
// (lokale Tage, inklusive) eingeschränkt.
func (s *HabitService) Completions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error) {
	for field, v := range map[string]string{"from": from, "to": to} {
		if v != "" {
			if err := validateDate(field, v); err != nil {
				return nil, err
			}
		}
	}
	if _, err := s.store.Get(ctx, habitID); err != nil {
		return nil, err
	}
	return s.store.ListCompletions(ctx, habitID, from, to)
}

// DueOn liefert alle Habits, die am Tag date fällig sind, mit berechneter
// Occurrence (Done, Wochenfortschritt), nach Name sortiert.
func (s *HabitService) DueOn(ctx context.Context, date string) ([]HabitDay, error) {
	monday, sunday, err := domain.WeekRange(date)
	if err != nil {
		return nil, err
	}
	habits, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	cs, err := s.store.ListCompletions(ctx, "", monday, sunday)
	if err != nil {
		return nil, err
	}
	done := map[string]map[string]bool{}
	for _, c := range cs {
		if done[c.HabitID] == nil {
			done[c.HabitID] = map[string]bool{}
		}
		done[c.HabitID][c.Date] = true
	}
	out := []HabitDay{}
	for _, h := range habits {
		if occ, due := h.OccurrenceOn(date, done[h.ID]); due {
			out = append(out, HabitDay{Habit: h, Occurrence: occ})
		}
	}
	return out, nil
}

// normalizeHabit prüft h und setzt die kanonische FrequencyConfig.
func normalizeHabit(h *domain.Habit, cfg domain.FrequencyConfig) error {
	switch {
	case h.Name == "":
		return fmt.Errorf("%w: name darf nicht leer sein", domain.ErrInvalid)
	case h.TargetValue != nil && *h.TargetValue < 1:
		return fmt.Errorf("%w: target_value muss mindestens 1 sein", domain.ErrInvalid)
	}
	if err := validateDate("start_date", h.StartDate); err != nil {
		return err
	}
	if h.EndDate != nil {
		if err := validateDate("end_date", *h.EndDate); err != nil {
			return err
		}
		if *h.EndDate < h.StartDate {
			return fmt.Errorf("%w: end_date liegt vor start_date", domain.ErrInvalid)
		}
	}
	if h.PreferredTime != nil {
		if t, err := time.Parse("15:04", *h.PreferredTime); err != nil || t.Format("15:04") != *h.PreferredTime {
			return fmt.Errorf("%w: preferred_time muss HH:MM sein", domain.ErrInvalid)
		}
	}
	norm, err := domain.NormalizeConfig(h.FrequencyType, cfg, h.StartDate)
	if err != nil {
		return err
	}
	h.FrequencyConfig = norm
	return nil
}
