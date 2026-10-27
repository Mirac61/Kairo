package service

import (
	"context"
	"errors"
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
	SetCompletionValue(ctx context.Context, c domain.HabitCompletion) error
	ListPauses(ctx context.Context) ([]domain.HabitPause, error)
	CreatePause(ctx context.Context, p domain.HabitPause) error
	DeletePause(ctx context.Context, id string) error
}

// HabitService enthält die Regeln für Habits und Completions.
type HabitService struct {
	publisher
	store   HabitStore
	loc     *time.Location
	now     func() time.Time
	tracked TimeLister // für Gewohnheiten mit Projekt; nil = aus
}

// TimeLister liefert Zeiteinträge (TimeEntryRepository).
type TimeLister interface {
	List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error)
}

// WithTracked schaltet ein, dass sich Gewohnheiten mit Projekt aus erfasster Zeit erfüllen (siehe SyncTracked).
func (s *HabitService) WithTracked(l TimeLister) *HabitService {
	s.tracked = l
	return s
}

// SyncTracked hakt am Tag von at jede aktive, fällige Gewohnheit mit Projekt ab, für die an diesem Tag
// mindestens DurationMinutes im Projekt erfasst sind. Läuft nach jeder Änderung der Zeiterfassung.
// shortcut: nimmt nichts zurück, wenn Zeit später gelöscht wird; Abhaken bleibt, bis man es von Hand löst.
func (s *HabitService) SyncTracked(ctx context.Context, at time.Time) error {
	if s.tracked == nil {
		return nil
	}
	local := at.In(s.loc)
	date := local.Format("2006-01-02")
	if date > s.today() {
		return nil
	}
	due, err := s.DueOn(ctx, date)
	if err != nil {
		return err
	}
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	to := from.AddDate(0, 0, 1)
	for _, d := range due {
		h := d.Habit
		if h.ProjectID == nil || d.Occurrence.Done {
			continue
		}
		entries, err := s.tracked.List(ctx, domain.TimeEntryFilter{ProjectID: *h.ProjectID, From: &from, To: &to})
		if err != nil {
			return err
		}
		var mins time.Duration
		for _, e := range entries {
			if e.EndedAt == nil {
				continue // läuft noch: zählt beim Stoppen
			}
			start, end := e.StartedAt, *e.EndedAt
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			mins += end.Sub(start)
		}
		if mins >= time.Duration(h.DurationMinutes)*time.Minute {
			if _, err := s.Complete(ctx, h.ID, CompleteHabitInput{Date: date, Note: "aus Zeiterfassung"}); err != nil && !errors.Is(err, domain.ErrConflict) {
				return err
			}
		}
	}
	return nil
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
	Icon            string
	Remind          bool
	PreferredTime   *string
	DurationMinutes int // 0 = Standard (30)
	ProjectID       *string
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
	Icon            *string
	Remind          *bool
	PreferredTime   *string
	DurationMinutes *int
	ProjectID       *string // "" entfernt das Projekt
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
		ID:              domain.NewID(),
		Name:            strings.TrimSpace(in.Name),
		Description:     in.Description,
		FrequencyType:   in.FrequencyType,
		TargetValue:     in.TargetValue,
		Unit:            strings.TrimSpace(in.Unit),
		Icon:            strings.TrimSpace(in.Icon),
		Remind:          in.Remind,
		PreferredTime:   trimOrNil(in.PreferredTime),
		DurationMinutes: in.DurationMinutes,
		ProjectID:       trimOrNil(in.ProjectID),
		StartDate:       s.today(),
		EndDate:         trimOrNil(in.EndDate),
		Active:          true,
		CreatedAt:       s.now().UTC(),
	}
	if h.DurationMinutes == 0 {
		h.DurationMinutes = 30
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
	h, err := s.store.Get(ctx, id)
	if err != nil {
		return h, err
	}
	hs, err := s.withPauses(ctx, []domain.Habit{h})
	if err != nil {
		return h, err
	}
	return hs[0], nil
}

// List liefert alle Habits samt ihren Pausen (Habit.Pauses).
func (s *HabitService) List(ctx context.Context) ([]domain.Habit, error) {
	hs, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.withPauses(ctx, hs)
}

// withPauses hängt an jedes Habit seine eigenen und die globalen Pausen.
func (s *HabitService) withPauses(ctx context.Context, hs []domain.Habit) ([]domain.Habit, error) {
	ps, err := s.store.ListPauses(ctx)
	if err != nil {
		return nil, err
	}
	for i := range hs {
		for _, p := range ps {
			if p.HabitID == nil || *p.HabitID == hs[i].ID {
				hs[i].Pauses = append(hs[i].Pauses, p)
			}
		}
	}
	return hs, nil
}

// Pauses liefert alle Pausen; Pause legt eine an (habitID nil = alle Habits), Unpause löscht sie.
func (s *HabitService) Pauses(ctx context.Context) ([]domain.HabitPause, error) {
	return s.store.ListPauses(ctx)
}

func (s *HabitService) Pause(ctx context.Context, habitID *string, start, end string) (domain.HabitPause, error) {
	p := domain.HabitPause{ID: domain.NewID(), HabitID: trimOrNil(habitID), StartDate: strings.TrimSpace(start), EndDate: strings.TrimSpace(end)}
	if p.EndDate == "" {
		p.EndDate = p.StartDate
	}
	if err := validateDate("start_date", p.StartDate); err != nil {
		return p, err
	}
	if err := validateDate("end_date", p.EndDate); err != nil {
		return p, err
	}
	if p.EndDate < p.StartDate {
		return p, fmt.Errorf("%w: end_date liegt vor start_date", domain.ErrInvalid)
	}
	if p.HabitID != nil {
		if _, err := s.store.Get(ctx, *p.HabitID); err != nil {
			return p, err
		}
	}
	if err := s.store.CreatePause(ctx, p); err != nil {
		return p, err
	}
	s.emit(domain.Event{Type: domain.EventHabitUpdated, ID: deref(p.HabitID)})
	return p, nil
}

func (s *HabitService) Unpause(ctx context.Context, id string) error {
	if err := s.store.DeletePause(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventHabitUpdated})
	return nil
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
	if in.Icon != nil {
		h.Icon = strings.TrimSpace(*in.Icon)
	}
	if in.Remind != nil {
		h.Remind = *in.Remind
	}
	if in.PreferredTime != nil {
		h.PreferredTime = trimOrNil(in.PreferredTime)
	}
	if in.DurationMinutes != nil {
		h.DurationMinutes = *in.DurationMinutes
	}
	if in.ProjectID != nil {
		h.ProjectID = trimOrNil(in.ProjectID)
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
	// Mit Wert (Mengen-Habits) überschreibt ein zweiter Aufruf die Menge des Tages; ohne Wert gilt: einmal pro Tag.
	save := s.store.CreateCompletion
	if in.Value != nil {
		save = s.store.SetCompletionValue
	}
	if err := save(ctx, c); err != nil {
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
	return s.DueBetween(ctx, date, date)
}

// maxDueDays begrenzt den Bereich von DueBetween (sechs Wochen Monatsansicht).
const maxDueDays = 42

// DueBetween liefert die fälligen Habits für jeden Tag von from bis to
// (inklusive), nach Tag und innerhalb eines Tages nach Name sortiert.
func (s *HabitService) DueBetween(ctx context.Context, from, to string) ([]HabitDay, error) {
	start, err := domain.ParseDate(from)
	if err != nil {
		return nil, fmt.Errorf("%w: from muss YYYY-MM-DD sein", domain.ErrInvalid)
	}
	end, err := domain.ParseDate(to)
	if err != nil {
		return nil, fmt.Errorf("%w: to muss YYYY-MM-DD sein", domain.ErrInvalid)
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 || days > maxDueDays {
		return nil, fmt.Errorf("%w: Bereich muss 1 bis %d Tage umfassen", domain.ErrInvalid, maxDueDays)
	}
	// TIMES_PER_WEEK braucht die ganzen Wochen am Rand.
	monday, _, _ := domain.WeekRange(from)
	_, sunday, _ := domain.WeekRange(to)
	habits, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	cs, err := s.store.ListCompletions(ctx, "", monday, sunday)
	if err != nil {
		return nil, err
	}
	done := map[string]map[string]bool{}
	for _, h := range habits {
		done[h.ID] = h.DoneDays(cs)
	}
	out := []HabitDay{}
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		for _, h := range habits {
			if occ, due := h.OccurrenceOn(date, done[h.ID]); due {
				out = append(out, HabitDay{Habit: h, Occurrence: occ})
			}
		}
	}
	return out, nil
}

// HabitReminder ist eine offene Gewohnheit mit Erinnerung zu ihrer Wunschzeit At.
type HabitReminder struct {
	Habit domain.Habit
	Date  string
	At    time.Time
}

// Reminders liefert die Gewohnheiten mit Remind, die im Fenster (lo, hi] ihre Wunschzeit haben
// und an dem Tag fällig, aber noch nicht erledigt sind (ausgesetzte Tage sind nicht fällig).
func (s *HabitService) Reminders(ctx context.Context, lo, hi time.Time) ([]HabitReminder, error) {
	from, to := lo.In(s.loc).Format("2006-01-02"), hi.In(s.loc).Format("2006-01-02")
	days, err := s.DueBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var out []HabitReminder
	for _, d := range days {
		h := d.Habit
		if !h.Remind || h.PreferredTime == nil || d.Occurrence.Done {
			continue
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", d.Occurrence.Date+" "+*h.PreferredTime, s.loc)
		if err != nil || !at.After(lo) || at.After(hi) {
			continue
		}
		out = append(out, HabitReminder{Habit: h, Date: d.Occurrence.Date, At: at})
	}
	return out, nil
}

// normalizeHabit prüft h und setzt die kanonische FrequencyConfig.
func normalizeHabit(h *domain.Habit, cfg domain.FrequencyConfig) error {
	switch {
	case h.Name == "":
		return fmt.Errorf("%w: name darf nicht leer sein", domain.ErrInvalid)
	case len(h.Icon) > 32:
		return fmt.Errorf("%w: icon ist zu lang", domain.ErrInvalid)
	case h.TargetValue != nil && *h.TargetValue < 1:
		return fmt.Errorf("%w: target_value muss mindestens 1 sein", domain.ErrInvalid)
	case h.DurationMinutes < 1 || h.DurationMinutes > 24*60:
		return fmt.Errorf("%w: duration_minutes muss zwischen 1 und 1440 liegen", domain.ErrInvalid)
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
