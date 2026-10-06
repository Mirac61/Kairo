package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"kairo/internal/domain"
)

type memHabits struct {
	habits map[string]domain.Habit
	comps  []domain.HabitCompletion
}

func (s *memHabits) Create(_ context.Context, h domain.Habit) error { s.habits[h.ID] = h; return nil }
func (s *memHabits) Get(_ context.Context, id string) (domain.Habit, error) {
	h, ok := s.habits[id]
	if !ok {
		return domain.Habit{}, domain.ErrNotFound
	}
	return h, nil
}
func (s *memHabits) List(context.Context) ([]domain.Habit, error) {
	var out []domain.Habit
	for _, h := range s.habits {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *memHabits) Update(_ context.Context, h domain.Habit) error { s.habits[h.ID] = h; return nil }
func (s *memHabits) Delete(_ context.Context, id string) error      { delete(s.habits, id); return nil }
func (s *memHabits) CreateCompletion(_ context.Context, c domain.HabitCompletion) error {
	for _, x := range s.comps {
		if x.HabitID == c.HabitID && x.Date == c.Date {
			return domain.ErrConflict
		}
	}
	s.comps = append(s.comps, c)
	return nil
}
func (s *memHabits) DeleteCompletion(_ context.Context, habitID, date string) error {
	for i, x := range s.comps {
		if x.HabitID == habitID && x.Date == date {
			s.comps = append(s.comps[:i], s.comps[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}
func (s *memHabits) ListCompletions(_ context.Context, habitID, from, to string) ([]domain.HabitCompletion, error) {
	var out []domain.HabitCompletion
	for _, c := range s.comps {
		if (habitID == "" || c.HabitID == habitID) && (from == "" || c.Date >= from) && (to == "" || c.Date <= to) {
			out = append(out, c)
		}
	}
	return out, nil
}

// newHabitSvc startet am Mittwoch, 07.10.2026, 23:30 UTC. In Berlin ist das
// schon Donnerstag, der 08.10.
func newHabitSvc(t *testing.T) *HabitService {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("keine Zeitzonendaten:", err)
	}
	clock := time.Date(2026, 10, 7, 23, 30, 0, 0, time.UTC)
	return NewHabitService(&memHabits{habits: map[string]domain.Habit{}}, loc, func() time.Time { return clock })
}

func TestHabitCreateDefaultsAndValidation(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()

	h, err := svc.Create(ctx, CreateHabitInput{Name: " Lesen ", FrequencyType: domain.FreqWeekly, TargetValue: ptr(30), Unit: " min "})
	if err != nil {
		t.Fatal(err)
	}
	// "Heute" ist in Berlin der 08.10. (Donnerstag), daher weekday TH.
	if h.Name != "Lesen" || h.Unit != "min" || h.StartDate != "2026-10-08" || !h.Active || h.FrequencyConfig.Weekday != "TH" {
		t.Errorf("Create = %+v", h)
	}

	base := CreateHabitInput{Name: "x", FrequencyType: domain.FreqDaily}
	mod := func(f func(*CreateHabitInput)) CreateHabitInput { in := base; f(&in); return in }
	for name, in := range map[string]CreateHabitInput{
		"leerer Name":        mod(func(i *CreateHabitInput) { i.Name = " " }),
		"Typ fehlt":          mod(func(i *CreateHabitInput) { i.FrequencyType = "" }),
		"Config passt nicht": mod(func(i *CreateHabitInput) { i.FrequencyConfig.Times = 2 }),
		"Ziel 0":             mod(func(i *CreateHabitInput) { i.TargetValue = ptr(0) }),
		"Uhrzeit":            mod(func(i *CreateHabitInput) { i.PreferredTime = ptr("25:00") }),
		"Uhrzeit ohne 0":     mod(func(i *CreateHabitInput) { i.PreferredTime = ptr("7:30") }),
		"Start":              mod(func(i *CreateHabitInput) { i.StartDate = ptr("08.10.2026") }),
		"Ende vor Start":     mod(func(i *CreateHabitInput) { i.StartDate = ptr("2026-10-08"); i.EndDate = ptr("2026-10-01") }),
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestHabitUpdate(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()
	h, _ := svc.Create(ctx, CreateHabitInput{Name: "Sport", FrequencyType: domain.FreqTimesPerWeek,
		FrequencyConfig: domain.FrequencyConfig{Times: 3}, TargetValue: ptr(45), PreferredTime: ptr("18:00"), EndDate: ptr("2026-12-31")})

	got, err := svc.Update(ctx, h.ID, UpdateHabitInput{Name: ptr("Training")})
	if err != nil || got.Name != "Training" || got.FrequencyConfig.Times != 3 || *got.TargetValue != 45 {
		t.Fatalf("partielles Update: %+v, %v", got, err)
	}
	got, err = svc.Update(ctx, h.ID, UpdateHabitInput{TargetValue: ptr(0), PreferredTime: ptr(""), EndDate: ptr("")})
	if err != nil || got.TargetValue != nil || got.PreferredTime != nil || got.EndDate != nil {
		t.Errorf("Felder entfernen: %+v, %v", got, err)
	}
	// Typwechsel ohne Config verwirft die alte Config (TIMES_PER_WEEK -> DAILY ist dann gültig).
	typ := domain.FreqDaily
	got, err = svc.Update(ctx, h.ID, UpdateHabitInput{FrequencyType: &typ})
	if err != nil || got.FrequencyType != domain.FreqDaily || got.FrequencyConfig.Times != 0 {
		t.Errorf("Typwechsel: %+v, %v", got, err)
	}
	// Typwechsel auf einen Typ, der Details braucht, schlägt ohne Config fehl.
	typ = domain.FreqSpecificWeekdays
	if _, err := svc.Update(ctx, h.ID, UpdateHabitInput{FrequencyType: &typ}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Typwechsel ohne Config: %v", err)
	}
	if got, err = svc.Update(ctx, h.ID, UpdateHabitInput{FrequencyType: &typ,
		FrequencyConfig: &domain.FrequencyConfig{Weekdays: []string{"fr", "mo"}}}); err != nil ||
		len(got.FrequencyConfig.Weekdays) != 2 || got.FrequencyConfig.Weekdays[0] != "MO" {
		t.Errorf("Typwechsel mit Config: %+v, %v", got, err)
	}
	if _, err := svc.Update(ctx, h.ID, UpdateHabitInput{StartDate: ptr("")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("start_date leeren: %v", err)
	}
	if _, err := svc.Update(ctx, "missing", UpdateHabitInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannte ID: %v", err)
	}
}

func TestHabitComplete(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()
	h, _ := svc.Create(ctx, CreateHabitInput{Name: "Lesen", FrequencyType: domain.FreqDaily, StartDate: ptr("2026-10-05"), EndDate: ptr("2026-10-30")})

	c, err := svc.Complete(ctx, h.ID, CompleteHabitInput{Value: ptr(30), Note: "Kapitel 3"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Date != "2026-10-08" || *c.Value != 30 || c.ID == "" || c.CompletedAt.Format(time.RFC3339) != "2026-10-07T23:30:00Z" {
		t.Errorf("Complete = %+v", c)
	}
	if _, err := svc.Complete(ctx, h.ID, CompleteHabitInput{}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("doppelt: %v", err)
	}
	if _, err := svc.Complete(ctx, h.ID, CompleteHabitInput{Date: "2026-10-06"}); err != nil {
		t.Errorf("Vortag: %v", err)
	}
	for name, in := range map[string]CompleteHabitInput{
		"Zukunft":        {Date: "2026-10-09"},
		"vor Start":      {Date: "2026-10-04"},
		"kaputtes Datum": {Date: "gestern"},
		"negativer Wert": {Date: "2026-10-07", Value: ptr(-1)},
	} {
		if _, err := svc.Complete(ctx, h.ID, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	off := false
	_, _ = svc.Update(ctx, h.ID, UpdateHabitInput{Active: &off})
	if _, err := svc.Complete(ctx, h.ID, CompleteHabitInput{Date: "2026-10-07"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("inaktiv: %v", err)
	}
	if _, err := svc.Complete(ctx, "missing", CompleteHabitInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekanntes Habit: %v", err)
	}

	if err := svc.Uncomplete(ctx, h.ID, "2026-10-06"); err != nil {
		t.Errorf("Uncomplete: %v", err)
	}
	if err := svc.Uncomplete(ctx, h.ID, "2026-10-06"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Uncomplete: %v", err)
	}
	if err := svc.Uncomplete(ctx, h.ID, "x"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Uncomplete kaputtes Datum: %v", err)
	}
}

func TestHabitCompleteAfterEndDate(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()
	h, _ := svc.Create(ctx, CreateHabitInput{Name: "x", FrequencyType: domain.FreqDaily, StartDate: ptr("2026-10-01"), EndDate: ptr("2026-10-05")})
	if _, err := svc.Complete(ctx, h.ID, CompleteHabitInput{Date: "2026-10-06"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("nach end_date: %v", err)
	}
}

func TestHabitDueOn(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()
	start := ptr("2026-10-05")
	daily, _ := svc.Create(ctx, CreateHabitInput{Name: "a-daily", FrequencyType: domain.FreqDaily, StartDate: start})
	_, _ = svc.Create(ctx, CreateHabitInput{Name: "b-weekdays", FrequencyType: domain.FreqSpecificWeekdays,
		FrequencyConfig: domain.FrequencyConfig{Weekdays: []string{"MO"}}, StartDate: start})
	tpw, _ := svc.Create(ctx, CreateHabitInput{Name: "c-times", FrequencyType: domain.FreqTimesPerWeek,
		FrequencyConfig: domain.FrequencyConfig{Times: 2}, StartDate: start})
	off := false
	_, _ = svc.Create(ctx, CreateHabitInput{Name: "d-inactive", FrequencyType: domain.FreqDaily, StartDate: start, Active: &off})

	_, _ = svc.Complete(ctx, daily.ID, CompleteHabitInput{Date: "2026-10-06"})
	_, _ = svc.Complete(ctx, tpw.ID, CompleteHabitInput{Date: "2026-10-05"})
	_, _ = svc.Complete(ctx, tpw.ID, CompleteHabitInput{Date: "2026-10-06"})

	names := func(date string) (out []string, days []HabitDay) {
		days, err := svc.DueOn(ctx, date)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range days {
			out = append(out, d.Habit.Name)
		}
		return out, days
	}
	// Dienstag 06.10.: daily (erledigt) + times (Ziel heute erreicht, bleibt sichtbar), nicht Montags-Habit.
	got, days := names("2026-10-06")
	if len(got) != 2 || got[0] != "a-daily" || got[1] != "c-times" || !days[0].Occurrence.Done || days[1].Occurrence.Progress.Done != 2 {
		t.Errorf("06.10.: %v %+v", got, days)
	}
	// Mittwoch 07.10.: Wochenziel erreicht, times verschwindet; daily offen.
	got, days = names("2026-10-07")
	if len(got) != 1 || got[0] != "a-daily" || days[0].Occurrence.Done {
		t.Errorf("07.10.: %v %+v", got, days)
	}
	// Montag 12.10.: neue Woche, alle drei fällig, times 0/2.
	got, days = names("2026-10-12")
	if len(got) != 3 || days[2].Occurrence.Progress.Done != 0 {
		t.Errorf("12.10.: %v %+v", got, days)
	}
	if _, err := svc.DueOn(ctx, "heute"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("kaputtes Datum: %v", err)
	}
}

func TestHabitCompletionsList(t *testing.T) {
	svc := newHabitSvc(t)
	ctx := context.Background()
	h, _ := svc.Create(ctx, CreateHabitInput{Name: "x", FrequencyType: domain.FreqDaily, StartDate: ptr("2026-10-05")})
	_, _ = svc.Complete(ctx, h.ID, CompleteHabitInput{Date: "2026-10-06"})
	if cs, err := svc.Completions(ctx, h.ID, "2026-10-01", "2026-10-31"); err != nil || len(cs) != 1 {
		t.Errorf("Completions = %v, %v", cs, err)
	}
	if _, err := svc.Completions(ctx, h.ID, "x", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("kaputtes from: %v", err)
	}
	if _, err := svc.Completions(ctx, "missing", "", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekanntes Habit: %v", err)
	}
}
