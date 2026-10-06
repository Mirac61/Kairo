package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"kairo/internal/domain"
)

// maxReviewDays begrenzt das Auswertungsfenster.
const maxReviewDays = 366

type reviewTasks interface {
	List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error)
}

type reviewProjects interface {
	List(ctx context.Context) ([]domain.Project, error)
}

type reviewHabits interface {
	List(ctx context.Context) ([]domain.Habit, error)
	Completions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error)
}

// ReviewService wertet vergangene Zeit aus (nur lesend, setzt auf den anderen Services auf).
type ReviewService struct {
	tasks    reviewTasks
	projects reviewProjects
	habits   reviewHabits
	events   TodayEvents
	times    TodayTimes
	loc      *time.Location
	now      func() time.Time
}

func NewReviewService(tasks reviewTasks, projects reviewProjects, habits reviewHabits, events TodayEvents,
	times TodayTimes, loc *time.Location, now func() time.Time) *ReviewService {
	if now == nil {
		now = time.Now
	}
	return &ReviewService{tasks, projects, habits, events, times, loc, now}
}

type ReviewDay struct {
	Date            string
	TrackedMinutes  int
	CalendarMinutes int
	CompletedTasks  int
}

// ReviewProject: Zeit und erledigte Tasks im Fenster, Fortschritt über alle Tasks
// des Projekts (ohne CANCELLED). ProjectID "" ist "ohne Projekt".
type ReviewProject struct {
	ProjectID      string
	Name           string
	TrackedMinutes int
	CompletedTasks int
	DoneTasks      int
	TotalTasks     int
}

type ReviewHabit struct {
	HabitID string
	Name    string
	Done    int // Completions im Fenster
	Streak  int // bis heute, siehe domain.Habit.Streak
}

type Review struct {
	From, To       string // YYYY-MM-DD, beide inklusive
	Timezone       string
	TrackedMinutes int
	CompletedTasks int
	Days           []ReviewDay
	Projects       []ReviewProject
	Habits         []ReviewHabit
}

// Get wertet [from, to] aus. Leer: die letzten 7 Tage bis heute.
func (s *ReviewService) Get(ctx context.Context, from, to string) (Review, error) {
	now := s.now()
	today := now.In(s.loc).Format("2006-01-02")
	if to == "" {
		to = today
	}
	if from == "" {
		d, err := time.Parse("2006-01-02", to)
		if err != nil {
			return Review{}, validateDate("to", to)
		}
		from = d.AddDate(0, 0, -6).Format("2006-01-02")
	}
	for k, v := range map[string]string{"from": from, "to": to} {
		if err := validateDate(k, v); err != nil {
			return Review{}, err
		}
	}
	first, _ := time.Parse("2006-01-02", from)
	last, _ := time.Parse("2006-01-02", to)
	if last.Before(first) || last.Sub(first) >= maxReviewDays*24*time.Hour {
		return Review{}, fmt.Errorf("%w: from..to muss 1 bis %d Tage umfassen", domain.ErrInvalid, maxReviewDays)
	}
	local := func(d time.Time, add int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day()+add, 0, 0, 0, 0, s.loc)
	}
	start, end := local(first, 0), local(last, 1)
	r := Review{From: from, To: to, Timezone: s.loc.String(), Days: []ReviewDay{}, Projects: []ReviewProject{}, Habits: []ReviewHabit{}}
	idx := map[string]int{}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		idx[d.Format("2006-01-02")] = len(r.Days)
		r.Days = append(r.Days, ReviewDay{Date: d.Format("2006-01-02")})
	}

	events, err := s.events.Occurrences(ctx, start, end)
	if err != nil {
		return Review{}, err
	}
	for i := range r.Days {
		d := local(first, i)
		r.Days[i].CalendarMinutes = busyMinutes(events, d, local(first, i+1))
	}

	projects, err := s.projects.List(ctx)
	if err != nil {
		return Review{}, err
	}
	byProject := map[string]*ReviewProject{}
	get := func(id string) *ReviewProject {
		if p, ok := byProject[id]; ok {
			return p
		}
		p := &ReviewProject{ProjectID: id, Name: "Ohne Projekt"}
		byProject[id] = p
		return p
	}
	for _, p := range projects {
		get(p.ID).Name = p.Name
	}

	tasks, err := s.tasks.List(ctx, domain.TaskFilter{})
	if err != nil {
		return Review{}, err
	}
	for _, t := range tasks {
		if t.Status == domain.TaskCancelled {
			continue
		}
		p := get(deref(t.ProjectID))
		p.TotalTasks++
		if t.Status != domain.TaskCompleted {
			continue
		}
		p.DoneTasks++
		if t.CompletedAt != nil && !t.CompletedAt.Before(start) && t.CompletedAt.Before(end) {
			p.CompletedTasks++
			r.CompletedTasks++
			r.Days[idx[t.CompletedAt.In(s.loc).Format("2006-01-02")]].CompletedTasks++
		}
	}

	entries, err := s.times.List(ctx, domain.TimeEntryFilter{From: &start, To: &end})
	if err != nil {
		return Review{}, fmt.Errorf("Zeiteinträge: %w", err)
	}
	for _, e := range entries {
		finish := now
		if e.EndedAt != nil {
			finish = *e.EndedAt
		}
		if !finish.After(e.StartedAt) {
			continue
		}
		m := int(finish.Sub(e.StartedAt) / time.Minute)
		r.TrackedMinutes += m
		r.Days[idx[e.StartedAt.In(s.loc).Format("2006-01-02")]].TrackedMinutes += m
		get(deref(e.ProjectID)).TrackedMinutes += m
	}
	for _, p := range byProject {
		// "Ohne Projekt" nur zeigen, wenn es etwas zu zeigen gibt.
		if p.ProjectID != "" || p.TotalTasks+p.TrackedMinutes > 0 {
			r.Projects = append(r.Projects, *p)
		}
	}
	sort.Slice(r.Projects, func(i, j int) bool {
		a, b := r.Projects[i], r.Projects[j]
		if a.TrackedMinutes != b.TrackedMinutes {
			return a.TrackedMinutes > b.TrackedMinutes
		}
		return a.Name < b.Name
	})

	habits, err := s.habits.List(ctx)
	if err != nil {
		return Review{}, err
	}
	for _, h := range habits {
		if !h.Active {
			continue
		}
		cs, err := s.habits.Completions(ctx, h.ID, "", "")
		if err != nil {
			return Review{}, err
		}
		done, inRange := map[string]bool{}, 0
		for _, c := range cs {
			done[c.Date] = true
			if c.Date >= from && c.Date <= to {
				inRange++
			}
		}
		r.Habits = append(r.Habits, ReviewHabit{HabitID: h.ID, Name: h.Name, Done: inRange, Streak: h.Streak(today, done)})
	}
	sort.Slice(r.Habits, func(i, j int) bool {
		a, b := r.Habits[i], r.Habits[j]
		return a.Streak > b.Streak || a.Streak == b.Streak && a.Name < b.Name
	})
	return r, nil
}
