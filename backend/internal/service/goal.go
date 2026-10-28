package service

import (
	"context"
	"errors"
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
// History ist der Fortschritt von StartDate bis heute, gleichmäßig verteilt; nil ohne Fortschritt.
type GoalView struct {
	domain.Goal
	Progress *GoalProgress
	History  []GoalPoint
}

// GoalPoint ist ein Punkt im Verlauf: so viel Prozent waren am Ende von Date erreicht.
type GoalPoint struct {
	Date    string
	Percent int
}

// goalEvent: n Einheiten Fortschritt zum Zeitpunkt at (eine Aufgabe, Minuten, eine Completion, ein Teilziel).
type goalEvent struct {
	at time.Time
	n  int
}

// historyPoints: so viele Stützstellen hat ein Verlauf höchstens.
const historyPoints = 24

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
	ParentID  *string
}

// UpdateGoalInput ändert nur gesetzte Felder; "" entfernt ImagePath, ProjectID, HabitID, DueDate und ParentID.
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
	ParentID  *string
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
		ParentID:  trimOrNil(in.ParentID),
		CreatedAt: s.now().UTC(),
	}
	if sd := trimOrNil(in.StartDate); sd != nil {
		g.StartDate = *sd
	}
	if err := s.validate(ctx, &g); err != nil {
		return GoalView{}, err
	}
	if err := s.store.Create(ctx, g); err != nil {
		return GoalView{}, err
	}
	s.emit(domain.Event{Type: domain.EventGoalCreated, ID: g.ID})
	return s.get(ctx, g.ID)
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
	if in.Archived != nil && *in.Archived != g.Archived {
		g.Archived = *in.Archived
		g.ArchivedAt = nil
		if g.Archived {
			now := s.now()
			g.ArchivedAt = &now
		}
	}
	if in.ParentID != nil {
		g.ParentID = trimOrNil(in.ParentID)
	}
	if err := s.validate(ctx, &g); err != nil {
		return GoalView{}, err
	}
	if err := s.store.Update(ctx, g); err != nil {
		return GoalView{}, err
	}
	s.emit(domain.Event{Type: domain.EventGoalUpdated, ID: g.ID})
	return s.get(ctx, g.ID)
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
	// Ein Langzeitziel ohne eigene Messung zeigt den Schnitt seiner Teilziele (erreichte zählen voll).
	for i := range out {
		if out[i].Progress != nil || out[i].ParentID != nil {
			continue
		}
		sum, n := 0, 0
		for _, c := range out {
			if c.ParentID == nil || *c.ParentID != out[i].ID {
				continue
			}
			switch {
			case c.Archived:
				sum += 100
			case c.Progress != nil:
				sum += c.Progress.Percent
			}
			n++
		}
		if n > 0 {
			done := 0
			// Erreichte zählen ab ihrem Datum voll, Teilfortschritt offener erst heute: ihr Verlauf liegt nicht vor.
			var evs []goalEvent
			for _, c := range out {
				if c.ParentID == nil || *c.ParentID != out[i].ID {
					continue
				}
				if c.Archived || c.Progress != nil && c.Progress.Percent >= 100 {
					done++
				}
				switch {
				case c.Archived && c.ArchivedAt != nil:
					evs = append(evs, goalEvent{*c.ArchivedAt, 100})
				case c.Archived:
					evs = append(evs, goalEvent{time.Time{}, 100})
				case c.Progress != nil:
					evs = append(evs, goalEvent{s.now(), c.Progress.Percent})
				}
			}
			out[i].Progress = &GoalProgress{Current: done, Target: n, Percent: sum / n}
			out[i].History = s.history(out[i].StartDate, evs, n*100)
		}
	}
	return out, nil
}

// get liefert ein Ziel so, wie es in List steht (mit Fortschritt aus Teilzielen).
func (s *GoalService) get(ctx context.Context, id string) (GoalView, error) {
	vs, err := s.List(ctx)
	if err != nil {
		return GoalView{}, err
	}
	for _, v := range vs {
		if v.ID == id {
			return v, nil
		}
	}
	return GoalView{}, domain.ErrNotFound
}

// validate prüft das Ziel und den Verweis aufs Langzeitziel: es gibt nur eine Ebene.
func (s *GoalService) validate(ctx context.Context, g *domain.Goal) error {
	if err := g.Validate(); err != nil {
		return err
	}
	if g.ParentID == nil {
		return nil
	}
	if *g.ParentID == g.ID {
		return fmt.Errorf("%w: ein Ziel kann nicht sein eigenes Teilziel sein", domain.ErrInvalid)
	}
	p, err := s.store.Get(ctx, *g.ParentID)
	if errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("%w: Langzeitziel existiert nicht", domain.ErrInvalid)
	}
	if err != nil {
		return err
	}
	if p.ParentID != nil {
		return fmt.Errorf("%w: Teilziele können keine eigenen Teilziele haben", domain.ErrInvalid)
	}
	gs, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range gs {
		if c.ParentID != nil && *c.ParentID == g.ID {
			return fmt.Errorf("%w: ein Langzeitziel mit Teilzielen kann nicht selbst Teilziel werden", domain.ErrInvalid)
		}
	}
	return nil
}

// Today wählt reihum ein aktives Ziel für heute (nach Tag im Jahr); nil, wenn es keins gibt.
func (s *GoalService) Today(ctx context.Context) (*GoalView, error) {
	gs, err := s.List(ctx)
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
	v := active[s.now().In(s.loc).YearDay()%len(active)]
	return &v, nil
}

func (s *GoalService) view(ctx context.Context, g domain.Goal) (GoalView, error) {
	v := GoalView{Goal: g}
	var (
		cur, target int
		evs         []goalEvent
	)
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
				if t.CompletedAt != nil {
					evs = append(evs, goalEvent{*t.CompletedAt, 1})
				} else {
					evs = append(evs, goalEvent{time.Time{}, 1})
				}
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
				m := int(finish.Sub(e.StartedAt) / time.Minute)
				cur += m
				evs = append(evs, goalEvent{e.StartedAt, m})
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
		for _, c := range cs {
			d, _ := time.ParseInLocation("2006-01-02", c.Date, s.loc)
			evs = append(evs, goalEvent{d, 1})
		}
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
	v.History = s.history(g.StartDate, evs, target)
	return v, nil
}

// history tastet den Fortschritt von start (YYYY-MM-DD) bis jetzt an höchstens historyPoints Stellen ab.
// Events vor start zählen ab dem ersten Punkt.
func (s *GoalService) history(start string, evs []goalEvent, target int) []GoalPoint {
	from, err := time.ParseInLocation("2006-01-02", start, s.loc)
	now := s.now().In(s.loc)
	if err != nil || target <= 0 || !now.After(from) {
		return nil
	}
	span := now.Sub(from)
	n := min(historyPoints, int(span.Hours()/24)+2)
	out := make([]GoalPoint, n)
	for i := range out {
		t := from.Add(time.Duration(float64(span) * float64(i) / float64(n-1)))
		sum := 0
		for _, e := range evs {
			if !e.at.After(t) {
				sum += e.n
			}
		}
		out[i] = GoalPoint{Date: t.Format("2006-01-02"), Percent: min(100, sum*100/target)}
	}
	return out
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
