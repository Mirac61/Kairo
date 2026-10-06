package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"kairo/internal/domain"
)

// TodayTasks liefert Tasks nach Filter.
type TodayTasks interface {
	List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error)
}

// TodayEvents liefert die konkreten Termine in einem Zeitfenster.
type TodayEvents interface {
	Occurrences(ctx context.Context, from, to time.Time) ([]domain.EventInstance, error)
}

// TodayHabits liefert die an einem Tag fälligen Habits.
type TodayHabits interface {
	DueOn(ctx context.Context, date string) ([]HabitDay, error)
}

// TodayTimes liefert Zeiteinträge nach Filter.
type TodayTimes interface {
	List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error)
}

// TodayService stellt den Tageskontext zusammen (nur lesend).
type TodayService struct {
	tasks  TodayTasks
	events TodayEvents
	habits TodayHabits
	times  TodayTimes
	loc    *time.Location
	now    func() time.Time
}

// NewTodayService erzeugt einen TodayService. loc ist die Zeitzone, in der
// ein Tag gerechnet wird; now ist die Uhr (nil = time.Now).
func NewTodayService(tasks TodayTasks, events TodayEvents, habits TodayHabits, times TodayTimes,
	loc *time.Location, now func() time.Time) *TodayService {
	if now == nil {
		now = time.Now
	}
	return &TodayService{tasks: tasks, events: events, habits: habits, times: times, loc: loc, now: now}
}

// Today ist der Kontext eines lokalen Tages.
type Today struct {
	Date     string // YYYY-MM-DD
	Timezone string
	// DayStart und DayEnd begrenzen den Tag [DayStart, DayEnd) in UTC. Der
	// Tag ist an Zeitumstellungen 23 oder 25 Stunden lang.
	DayStart time.Time
	DayEnd   time.Time
	// Events sind die Termine, die den Tag berühren, nach Beginn sortiert.
	Events []domain.EventInstance
	// Tasks sind die für den Tag geplanten Tasks (ohne CANCELLED): zuerst die
	// mit planned_start_at nach Uhrzeit, danach die ohne in Anlegereihenfolge.
	Tasks []domain.Task
	// ActiveTasks sind IN_PROGRESS-Tasks, die nicht für diesen Tag geplant sind.
	ActiveTasks []domain.Task
	Habits      []HabitDay
	// Running ist der laufende Timer, falls einer läuft (unabhängig vom Tag).
	Running *domain.TimeEntry
	// PlannedMinutes ist die Summe von estimated_minutes der Tasks.
	PlannedMinutes int
	// CalendarMinutes ist die belegte Zeit der Termine im Tag, Überlappungen
	// zählen einmal, Termine über Mitternacht nur mit dem Anteil im Tag.
	CalendarMinutes int
	// TrackedMinutes ist die erfasste Arbeitszeit der Einträge, die an diesem
	// Tag gestartet sind. Ein laufender Timer zählt bis jetzt.
	TrackedMinutes int
}

// Get stellt den Kontext für date (YYYY-MM-DD) zusammen. Leer heißt heute
// in der konfigurierten Zeitzone.
func (s *TodayService) Get(ctx context.Context, date string) (Today, error) {
	now := s.now()
	date = strings.TrimSpace(date)
	if date == "" {
		date = now.In(s.loc).Format("2006-01-02")
	}
	if err := validateDate("date", date); err != nil {
		return Today{}, err
	}
	day, _ := time.Parse("2006-01-02", date)
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, s.loc)
	end := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, s.loc)
	t := Today{Date: date, Timezone: s.loc.String(), DayStart: start.UTC(), DayEnd: end.UTC()}

	var err error
	if t.Events, err = s.events.Occurrences(ctx, start, end); err != nil {
		return Today{}, err
	}
	planned, err := s.tasks.List(ctx, domain.TaskFilter{PlannedDate: date})
	if err != nil {
		return Today{}, err
	}
	t.Tasks = []domain.Task{}
	for _, task := range planned {
		if task.Status != domain.TaskCancelled {
			t.Tasks = append(t.Tasks, task)
			t.PlannedMinutes += task.EstimatedMinutes
		}
	}
	sortPlanned(t.Tasks)
	active, err := s.tasks.List(ctx, domain.TaskFilter{Status: domain.TaskInProgress})
	if err != nil {
		return Today{}, err
	}
	t.ActiveTasks = []domain.Task{}
	for _, task := range active {
		if task.PlannedDate == nil || *task.PlannedDate != date {
			t.ActiveTasks = append(t.ActiveTasks, task)
		}
	}
	if t.Habits, err = s.habits.DueOn(ctx, date); err != nil {
		return Today{}, err
	}
	running, err := s.times.List(ctx, domain.TimeEntryFilter{Running: true})
	if err != nil {
		return Today{}, err
	}
	if len(running) > 0 {
		t.Running = &running[0]
	}
	entries, err := s.times.List(ctx, domain.TimeEntryFilter{From: &start, To: &end})
	if err != nil {
		return Today{}, fmt.Errorf("Zeiteinträge des Tages: %w", err)
	}
	for _, e := range entries {
		finish := now
		if e.EndedAt != nil {
			finish = *e.EndedAt
		}
		if finish.After(e.StartedAt) {
			t.TrackedMinutes += int(finish.Sub(e.StartedAt) / time.Minute)
		}
	}
	t.CalendarMinutes = busyMinutes(t.Events, start, end)
	return t, nil
}

// sortPlanned sortiert Tasks mit planned_start_at nach vorn (nach Uhrzeit),
// die übrigen behalten ihre Reihenfolge.
func sortPlanned(ts []domain.Task) {
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i].PlannedStartAt, ts[j].PlannedStartAt
		switch {
		case a != nil && b != nil:
			return a.Before(*b)
		default:
			return a != nil && b == nil
		}
	})
}

// busyMinutes ist die Vereinigung der Termine, auf [from, to) beschnitten, in Minuten.
func busyMinutes(events []domain.EventInstance, from, to time.Time) int {
	var total time.Duration
	cursor := from
	for _, e := range events { // nach Beginn sortiert
		s, f := e.Start, e.End
		if s.Before(cursor) {
			s = cursor
		}
		if f.After(to) {
			f = to
		}
		if !f.After(s) {
			continue
		}
		total += f.Sub(s)
		cursor = f
	}
	return int(total / time.Minute)
}
