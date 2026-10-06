package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"kairo/internal/domain"
)

type fakeTodayTasks struct{ all []domain.Task }

func (f fakeTodayTasks) List(_ context.Context, fl domain.TaskFilter) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f.all {
		if fl.PlannedDate != "" && (t.PlannedDate == nil || *t.PlannedDate != fl.PlannedDate) {
			continue
		}
		if fl.Status != "" && t.Status != fl.Status {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

type fakeTodayEvents struct {
	got [2]time.Time
	out []domain.EventInstance
}

func (f *fakeTodayEvents) Occurrences(_ context.Context, from, to time.Time) ([]domain.EventInstance, error) {
	f.got = [2]time.Time{from, to}
	return f.out, nil
}

type fakeTodayHabits struct{ date string }

func (f *fakeTodayHabits) DueOn(_ context.Context, date string) ([]HabitDay, error) {
	f.date = date
	return []HabitDay{{Habit: domain.Habit{ID: "h"}}}, nil
}

type fakeTodayTimes struct{ running, day []domain.TimeEntry }

func (f fakeTodayTimes) List(_ context.Context, fl domain.TimeEntryFilter) ([]domain.TimeEntry, error) {
	if fl.Running {
		return f.running, nil
	}
	return f.day, nil
}

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("keine Zeitzonendatenbank:", err)
	}
	return loc
}

func TestTodayAssemblesDay(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, loc)
	at := func(h, m int) *time.Time { v := time.Date(2026, 10, 7, h, m, 0, 0, loc).UTC(); return &v }
	d := "2026-10-07"
	tasks := fakeTodayTasks{[]domain.Task{
		{ID: "ohne", Status: domain.TaskBacklog, PlannedDate: &d, EstimatedMinutes: 30},
		{ID: "spaet", Status: domain.TaskPlanned, PlannedDate: &d, PlannedStartAt: at(14, 0), EstimatedMinutes: 60},
		{ID: "frueh", Status: domain.TaskPlanned, PlannedDate: &d, PlannedStartAt: at(10, 15), EstimatedMinutes: 90},
		{ID: "abgebrochen", Status: domain.TaskCancelled, PlannedDate: &d, EstimatedMinutes: 500},
		{ID: "laeuft-heute", Status: domain.TaskInProgress, PlannedDate: &d, EstimatedMinutes: 10},
		{ID: "laeuft-sonst", Status: domain.TaskInProgress},
	}}
	ev := &fakeTodayEvents{out: []domain.EventInstance{
		{Start: *at(8, 30), End: *at(10, 0)},
		{Start: *at(9, 30), End: *at(11, 0)}, // überlappt: Vereinigung 8:30-11:00 = 150
		{Start: *at(12, 30), End: *at(13, 0)},
	}}
	habits := &fakeTodayHabits{}
	run := domain.TimeEntry{ID: "r", StartedAt: *at(11, 30)}
	done := *at(9, 0)
	times := fakeTodayTimes{
		running: []domain.TimeEntry{run},
		day: []domain.TimeEntry{
			{ID: "a", StartedAt: *at(8, 0), EndedAt: &done}, // 60
			run, // läuft seit 11:30 bis now 12:00 = 30
		},
	}
	svc := NewTodayService(tasks, ev, habits, times, loc, func() time.Time { return now })

	got, err := svc.Get(context.Background(), "2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	ids := func(ts []domain.Task) (s []string) {
		for _, t := range ts {
			s = append(s, t.ID)
		}
		return
	}
	// frühe zuerst nach Uhrzeit, dann ohne Uhrzeit in Anlegereihenfolge
	if want := []string{"frueh", "spaet", "ohne", "laeuft-heute"}; !equal(ids(got.Tasks), want) {
		t.Errorf("Tasks = %v, erwartet %v", ids(got.Tasks), want)
	}
	if !equal(ids(got.ActiveTasks), []string{"laeuft-sonst"}) {
		t.Errorf("ActiveTasks = %v", ids(got.ActiveTasks))
	}
	if got.PlannedMinutes != 30+60+90+10 {
		t.Errorf("PlannedMinutes = %d (abgebrochene Task zählt nicht)", got.PlannedMinutes)
	}
	if got.CalendarMinutes != 150+30 {
		t.Errorf("CalendarMinutes = %d", got.CalendarMinutes)
	}
	if got.TrackedMinutes != 90 {
		t.Errorf("TrackedMinutes = %d", got.TrackedMinutes)
	}
	if got.Running == nil || got.Running.ID != "r" || habits.date != "2026-10-07" || len(got.Habits) != 1 {
		t.Errorf("Running/Habits = %+v / %s / %d", got.Running, habits.date, len(got.Habits))
	}
	wantFrom := time.Date(2026, 10, 7, 0, 0, 0, 0, loc)
	if !ev.got[0].Equal(wantFrom) || !ev.got[1].Equal(wantFrom.AddDate(0, 0, 1)) || got.Timezone != "Europe/Berlin" {
		t.Errorf("Fenster = %v, Zone %s", ev.got, got.Timezone)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTodayDefaultsToLocalDate(t *testing.T) {
	loc := berlin(t)
	// 23:30 UTC am 6.10. ist in Berlin schon der 7.10.
	now := time.Date(2026, 10, 6, 23, 30, 0, 0, time.UTC)
	habits := &fakeTodayHabits{}
	svc := NewTodayService(fakeTodayTasks{}, &fakeTodayEvents{}, habits, fakeTodayTimes{}, loc, func() time.Time { return now })
	got, err := svc.Get(context.Background(), "")
	if err != nil || got.Date != "2026-10-07" || habits.date != "2026-10-07" {
		t.Errorf("Date = %q, err %v", got.Date, err)
	}
	if got.Events == nil && got.Tasks == nil {
		t.Error("Listen sollten leer statt nil sein")
	}
}

func TestTodayDSTDayLength(t *testing.T) {
	loc := berlin(t)
	svc := NewTodayService(fakeTodayTasks{}, &fakeTodayEvents{}, &fakeTodayHabits{}, fakeTodayTimes{}, loc, nil)
	for date, hours := range map[string]time.Duration{"2026-03-29": 23, "2026-10-25": 25, "2026-10-07": 24} {
		got, err := svc.Get(context.Background(), date)
		if err != nil || got.DayEnd.Sub(got.DayStart) != hours*time.Hour {
			t.Errorf("%s: Länge %v, err %v, erwartet %dh", date, got.DayEnd.Sub(got.DayStart), err, hours)
		}
	}
}

func TestTodayInvalidDate(t *testing.T) {
	svc := NewTodayService(fakeTodayTasks{}, &fakeTodayEvents{}, &fakeTodayHabits{}, fakeTodayTimes{}, time.UTC, nil)
	for _, d := range []string{"07.10.2026", "2026-13-01", "morgen"} {
		if _, err := svc.Get(context.Background(), d); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: %v", d, err)
		}
	}
}

func TestBusyMinutesClipsToDay(t *testing.T) {
	from := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	evs := []domain.EventInstance{
		{Start: from.Add(-2 * time.Hour), End: from.Add(time.Hour)}, // 60 im Tag
		{Start: to.Add(-time.Hour), End: to.Add(3 * time.Hour)},     // 60 im Tag
		{Start: from.Add(5 * time.Hour), End: from.Add(5 * time.Hour)},
	}
	if got := busyMinutes(evs, from, to); got != 120 {
		t.Errorf("busyMinutes = %d", got)
	}
}
