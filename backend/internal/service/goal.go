package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
)

// GoalStore ist der Speicher, den der GoalService braucht.
type GoalStore interface {
	Create(ctx context.Context, g domain.Goal) error
	Get(ctx context.Context, id string) (domain.Goal, error)
	List(ctx context.Context) ([]domain.Goal, error)
	Update(ctx context.Context, g domain.Goal) error
	Delete(ctx context.Context, id string) error
}

// GoalCompletions liefert die Completions einer Gewohnheit (HabitRepository).
type GoalCompletions interface {
	ListCompletions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error)
}

// GoalService verwaltet Ziele und berechnet ihren Fortschritt aus vorhandenen Daten.
type GoalService struct {
	publisher
	store  GoalStore
	tasks  reviewTasks
	times  TimeLister
	habits GoalCompletions
	loc    *time.Location
	now    func() time.Time
}

func NewGoalService(store GoalStore, tasks reviewTasks, times TimeLister, habits GoalCompletions, loc *time.Location, now func() time.Time) *GoalService {
	if loc == nil {
		loc = time.Local
	}
	if now == nil {
		now = time.Now
	}
	return &GoalService{store: store, tasks: tasks, times: times, habits: habits, loc: loc, now: now}
}

// GoalProgress: Current von Target, bei Stunden in ganzen Stunden. Percent ist auf 100 begrenzt.
type GoalProgress struct {
	Current int
	Target  int
	Percent int
}

// GoalView ist ein Ziel mit Fortschritt (nil bei GoalNone oder gelöschtem Projekt/Gewohnheit).
type GoalView struct {
	domain.Goal
	Progress *GoalProgress
}

// CreateGoalInput: StartDate ist standardmäßig heute.
type CreateGoalInput struct {
	Title     string
	Why       string
	ImagePath *string
	Kind      domain.GoalKind
	ProjectID *string
	HabitID   *string
	Target    int
	StartDate *string
	DueDate   *string
}

// UpdateGoalInput ändert nur gesetzte Felder; "" entfernt ImagePath, ProjectID, HabitID und DueDate.
type UpdateGoalInput struct {
	Title     *string
	Why       *string
	ImagePath *string
	Kind      *domain.GoalKind
	ProjectID *string
	HabitID   *string
	Target    *int
	StartDate *string
	DueDate   *string
	Archived  *bool
}

func (s *GoalService) Create(ctx context.Context, in CreateGoalInput) (GoalView, error) {
	g := domain.Goal{
		ID:        domain.NewID(),
		Title:     in.Title,
		Why:       strings.TrimSpace(in.Why),
		ImagePath: trimOrNil(in.ImagePath),
		Kind:      in.Kind,
		ProjectID: trimOrNil(in.ProjectID),
		HabitID:   trimOrNil(in.HabitID),
		Target:    in.Target,
		StartDate: s.now().In(s.loc).Format("2006-01-02"),
		DueDate:   trimOrNil(in.DueDate),
		CreatedAt: s.now().UTC(),
	}
	if sd := trimOrNil(in.StartDate); sd != nil {
		g.StartDate = *sd
	}
	if err := g.Validate(); err != nil {
		return GoalView{}, err
	}
	if err := s.store.Create(ctx, g); err != nil {
		return GoalView{}, err
	}
	s.emit(domain.Event{Type: domain.EventGoalCreated, ID: g.ID})
	return s.view(ctx, g)
}

func (s *GoalService) Update(ctx context.Context, id string, in UpdateGoalInput) (GoalView, error) {
	g, err := s.store.Get(ctx, id)
	if err != nil {
		return GoalView{}, err
	}
	if in.Title != nil {
		g.Title = *in.Title
	}
	if in.Why != nil {
		g.Why = strings.TrimSpace(*in.Why)
	}
	if in.ImagePath != nil {
		g.ImagePath = trimOrNil(in.ImagePath)
	}
	if in.Kind != nil {
		g.Kind = *in.Kind
	}
	if in.ProjectID != nil {
		g.ProjectID = trimOrNil(in.ProjectID)
	}
	if in.HabitID != nil {
		g.HabitID = trimOrNil(in.HabitID)
	}
	if in.Target != nil {
		g.Target = *in.Target
	}
	if in.StartDate != nil {
		g.StartDate = strings.TrimSpace(*in.StartDate)
	}
	if in.DueDate != nil {
		g.DueDate = trimOrNil(in.DueDate)
	}
	if in.Archived != nil {
		g.Archived = *in.Archived
	}
	if err := g.Validate(); err != nil {
		return GoalView{}, err
	}
	if err := s.store.Update(ctx, g); err != nil {
		return GoalView{}, err
	}
	s.emit(domain.Event{Type: domain.EventGoalUpdated, ID: g.ID})
	return s.view(ctx, g)
}

func (s *GoalService) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventGoalDeleted, ID: id})
	return nil
}

// List liefert alle Ziele mit Fortschritt.
func (s *GoalService) List(ctx context.Context) ([]GoalView, error) {
	gs, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]GoalView, 0, len(gs))
	for _, g := range gs {
		v, err := s.view(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Today wählt reihum ein aktives Ziel für heute (nach Tag im Jahr); nil, wenn es keins gibt.
func (s *GoalService) Today(ctx context.Context) (*GoalView, error) {
	gs, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	active := gs[:0]
	for _, g := range gs {
		if !g.Archived {
			active = append(active, g)
		}
	}
	if len(active) == 0 {
		return nil, nil
	}
	v, err := s.view(ctx, active[s.now().In(s.loc).YearDay()%len(active)])
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *GoalService) view(ctx context.Context, g domain.Goal) (GoalView, error) {
	v := GoalView{Goal: g}
	var cur, target int
	switch {
	case g.Kind == domain.GoalProjectTasks && g.ProjectID != nil:
		ts, err := s.tasks.List(ctx, domain.TaskFilter{ProjectID: *g.ProjectID})
		if err != nil {
			return v, err
		}
		for _, t := range ts {
			if t.Status == domain.TaskCancelled {
				continue
			}
			target++
			if t.Status == domain.TaskCompleted {
				cur++
			}
		}
	case g.Kind == domain.GoalProjectHours && g.ProjectID != nil:
		d, _ := time.Parse("2006-01-02", g.StartDate)
		from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, s.loc)
		es, err := s.times.List(ctx, domain.TimeEntryFilter{ProjectID: *g.ProjectID, From: &from})
		if err != nil {
			return v, err
		}
		now := s.now()
		for _, e := range es {
			finish := now
			if e.EndedAt != nil {
				finish = *e.EndedAt
			}
			if finish.After(e.StartedAt) {
				cur += int(finish.Sub(e.StartedAt) / time.Minute)
			}
		}
		// In Minuten rechnen, damit Percent nicht auf ganze Stunden springt.
		target = g.Target * 60
	case g.Kind == domain.GoalHabitCount && g.HabitID != nil:
		cs, err := s.habits.ListCompletions(ctx, *g.HabitID, g.StartDate, "")
		if err != nil {
			return v, err
		}
		cur, target = len(cs), g.Target
	default:
		return v, nil
	}
	p := &GoalProgress{Current: cur, Target: target}
	if target > 0 {
		p.Percent = min(100, cur*100/target)
	}
	if g.Kind == domain.GoalProjectHours {
		p.Current, p.Target = cur/60, g.Target
	}
	v.Progress = p
	return v, nil
}

// Line ist das Ziel als eine Zeile für ntfy, z. B. „Ziel: Fit – 40 % (20 von 50-mal)“.
func (v GoalView) Line() string {
	s := "Ziel: " + v.Title
	if p := v.Progress; p != nil {
		unit := map[domain.GoalKind]string{domain.GoalProjectTasks: " Aufgaben", domain.GoalProjectHours: " Std.", domain.GoalHabitCount: "-mal"}[v.Kind]
		s += fmt.Sprintf(" – %d %% (%d von %d%s)", p.Percent, p.Current, p.Target, unit)
	}
	if v.Why != "" {
		s += "\n„" + v.Why + "“"
	}
	return s
}
