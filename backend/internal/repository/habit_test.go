package repository

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kairo/internal/domain"
)

func newHabitRepo(t *testing.T) *HabitRepository {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewHabitRepository(db)
}

func newHabit(id string) domain.Habit {
	return domain.Habit{ID: id, Name: id, FrequencyType: domain.FreqDaily, StartDate: "2026-10-05",
		Active: true, CreatedAt: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)}
}

func TestHabitRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newHabitRepo(t)

	h := newHabit("h1")
	h.FrequencyType = domain.FreqSpecificWeekdays
	h.FrequencyConfig = domain.FrequencyConfig{Weekdays: []string{"MO", "WE"}}
	h.TargetValue, h.Unit, h.PreferredTime, h.EndDate = ptr(30), "min", ptr("07:30"), ptr("2026-12-31")
	if err := r.Create(ctx, h); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, h) {
		t.Errorf("Get = %+v\nwant %+v", got, h)
	}

	h.Active, h.TargetValue, h.PreferredTime, h.EndDate = false, nil, nil, nil
	h.FrequencyType, h.FrequencyConfig = domain.FreqTimesPerWeek, domain.FrequencyConfig{Times: 3}
	if err := r.Update(ctx, h); err != nil {
		t.Fatal(err)
	}
	got, _ = r.Get(ctx, "h1")
	if !reflect.DeepEqual(got, h) {
		t.Errorf("nach Update = %+v\nwant %+v", got, h)
	}

	if err := r.Delete(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(ctx, "h1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get nach Delete: %v", err)
	}
	if err := r.Update(ctx, h); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update ohne Zeile: %v", err)
	}
}

func TestHabitTableRejectsInvalidRows(t *testing.T) {
	ctx := context.Background()
	r := newHabitRepo(t)
	for name, mod := range map[string]func(*domain.Habit){
		"leerer Name":    func(h *domain.Habit) { h.Name = " " },
		"falscher Typ":   func(h *domain.Habit) { h.FrequencyType = "MONTHLY" },
		"Ziel 0":         func(h *domain.Habit) { h.TargetValue = ptr(0) },
		"Uhrzeit":        func(h *domain.Habit) { h.PreferredTime = ptr("7:30") },
		"Ende vor Start": func(h *domain.Habit) { h.EndDate = ptr("2026-10-01") },
	} {
		h := newHabit("x")
		mod(&h)
		if err := r.Create(ctx, h); err == nil {
			t.Errorf("%s wurde akzeptiert", name)
		}
	}
}

func TestHabitCompletions(t *testing.T) {
	ctx := context.Background()
	r := newHabitRepo(t)
	for _, id := range []string{"a", "b"} {
		if err := r.Create(ctx, newHabit(id)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC)
	add := func(id, habit, date string) error {
		return r.CreateCompletion(ctx, domain.HabitCompletion{ID: id, HabitID: habit, Date: date, CompletedAt: now, Note: "n"})
	}
	for i, c := range [][2]string{{"a", "2026-10-07"}, {"a", "2026-10-05"}, {"b", "2026-10-06"}, {"a", "2026-10-12"}} {
		if err := add(string(rune('0'+i)), c[0], c[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := add("dup", "a", "2026-10-07"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("zweite Completion am Tag: %v", err)
	}
	if err := add("x", "b", "2026-10-07"); err != nil {
		t.Errorf("anderes Habit, gleicher Tag: %v", err)
	}

	for name, tc := range map[string]struct {
		habit, from, to string
		want            int
	}{
		"alle":          {"", "", "", 5},
		"Habit a":       {"a", "", "", 3},
		"Woche, alle":   {"", "2026-10-05", "2026-10-11", 4},
		"Grenzen inkl.": {"a", "2026-10-05", "2026-10-07", 2},
		"leer":          {"a", "2026-11-01", "", 0},
	} {
		got, err := r.ListCompletions(ctx, tc.habit, tc.from, tc.to)
		if err != nil || len(got) != tc.want {
			t.Errorf("%s: %d, erwartet %d (err %v)", name, len(got), tc.want, err)
		}
	}
	got, _ := r.ListCompletions(ctx, "a", "", "")
	if got[0].Date != "2026-10-05" || got[0].Note != "n" || !got[0].CompletedAt.Equal(now) || got[0].Value != nil {
		t.Errorf("Sortierung/Felder: %+v", got[0])
	}

	if err := r.DeleteCompletion(ctx, "a", "2026-10-07"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteCompletion(ctx, "a", "2026-10-07"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweites Delete: %v", err)
	}
	// Nach dem Löschen ist der Tag wieder frei.
	if err := add("again", "a", "2026-10-07"); err != nil {
		t.Errorf("erneut abhaken: %v", err)
	}
	// Habit löschen entfernt seine Completions.
	if err := r.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if left, _ := r.ListCompletions(ctx, "a", "", ""); len(left) != 0 {
		t.Errorf("%d Completions blieben übrig", len(left))
	}
	if err := add("orphan", "a", "2026-10-08"); err == nil {
		t.Error("Completion für gelöschtes Habit wurde akzeptiert")
	}
}
