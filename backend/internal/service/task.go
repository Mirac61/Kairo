package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"kairo/internal/domain"
)

// TaskStore ist der Speicher, den der TaskService braucht.
type TaskStore interface {
	Create(ctx context.Context, t domain.Task) error
	Get(ctx context.Context, id string) (domain.Task, error)
	List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error)
	Update(ctx context.Context, t domain.Task) error
	Delete(ctx context.Context, id string) error
	Trash(ctx context.Context, id string, at time.Time) error
	Restore(ctx context.Context, id string) (domain.Task, error)
	ListTrashed(ctx context.Context) ([]domain.Task, error)
}

// maxTaskDepth begrenzt die Suche nach Zyklen in der Subtask-Kette.
const maxTaskDepth = 100

// TimerStopper beendet den laufenden Timer einer Task.
type TimerStopper interface {
	StopTimer(ctx context.Context, taskID string) error
}

// TaskService enthält die Regeln für Tasks.
type TaskService struct {
	publisher
	store  TaskStore
	now    func() time.Time
	timers TimerStopper
	loc    *time.Location // Ortszeit für Wiederholungen
}

// NewTaskService erzeugt einen TaskService. now ist die Uhr (nil = time.Now).
func NewTaskService(store TaskStore, now func() time.Time) *TaskService {
	if now == nil {
		now = time.Now
	}
	return &TaskService{store: store, now: now, loc: time.Local}
}

// WithLocation setzt die Zeitzone, in der Wiederholungen gerechnet werden (Standard: time.Local).
func (s *TaskService) WithLocation(loc *time.Location) *TaskService {
	s.loc = loc
	return s
}

// WithTimerStopper sorgt dafür, dass Update den Timer einer Task beendet,
// sobald sie IN_PROGRESS verlässt. Ohne das könnte ein Timer auf einer
// abgeschlossenen Task weiterlaufen.
func (s *TaskService) WithTimerStopper(ts TimerStopper) *TaskService {
	s.timers = ts
	return s
}

// CreateTaskInput sind die Felder beim Anlegen. Status ist standardmäßig
// BACKLOG, Priorität MEDIUM. DueAt und PlannedStartAt sind RFC 3339.
type CreateTaskInput struct {
	Title            string
	Description      string
	Status           domain.TaskStatus
	Priority         domain.TaskPriority
	EstimatedMinutes int
	DueAt            *string
	PlannedDate      *string
	PlannedStartAt   *string
	ProjectID        *string
	ParentTaskID     *string
	RecurrenceRule   *string
}

// UpdateTaskInput ändert nur die gesetzten (non-nil) Felder. Bei den
// optionalen Feldern (DueAt, PlannedDate, PlannedStartAt, ProjectID,
// ParentTaskID, RecurrenceRule) entfernt ein leerer String den Wert.
type UpdateTaskInput struct {
	Title            *string
	Description      *string
	Status           *domain.TaskStatus
	Priority         *domain.TaskPriority
	EstimatedMinutes *int
	DueAt            *string
	PlannedDate      *string
	PlannedStartAt   *string
	ProjectID        *string
	ParentTaskID     *string
	RecurrenceRule   *string
}

func (s *TaskService) Create(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
	if in.Status == "" {
		in.Status = domain.TaskBacklog
	}
	if in.Priority == "" {
		in.Priority = domain.PriorityMedium
	}
	now := s.now().UTC()
	t := domain.Task{
		ID:               domain.NewID(),
		Title:            strings.TrimSpace(in.Title),
		Description:      in.Description,
		Status:           in.Status,
		Priority:         in.Priority,
		EstimatedMinutes: in.EstimatedMinutes,
		PlannedDate:      trimOrNil(in.PlannedDate),
		ProjectID:        trimOrNil(in.ProjectID),
		ParentTaskID:     trimOrNil(in.ParentTaskID),
		RecurrenceRule:   trimOrNil(in.RecurrenceRule),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	var err error
	if t.DueAt, err = parseOptTime("due_at", in.DueAt); err != nil {
		return domain.Task{}, err
	}
	if t.PlannedStartAt, err = parseOptTime("planned_start_at", in.PlannedStartAt); err != nil {
		return domain.Task{}, err
	}
	if t.Status == domain.TaskCompleted {
		t.CompletedAt = &now
	}
	if err := s.validate(t); err != nil {
		return domain.Task{}, err
	}
	if err := s.store.Create(ctx, t); err != nil {
		return domain.Task{}, err
	}
	s.emit(domain.Event{Type: domain.EventTaskCreated, ID: t.ID})
	return t, nil
}

func (s *TaskService) Get(ctx context.Context, id string) (domain.Task, error) {
	return s.store.Get(ctx, id)
}

func (s *TaskService) List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error) {
	if f.Status != "" && !f.Status.Valid() {
		return nil, fmt.Errorf("%w: unbekannter Status %q", domain.ErrInvalid, f.Status)
	}
	if f.PlannedDate != "" {
		if err := validateDate("planned_date", f.PlannedDate); err != nil {
			return nil, err
		}
	}
	return s.store.List(ctx, f)
}

func (s *TaskService) Update(ctx context.Context, id string, in UpdateTaskInput) (domain.Task, error) {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	now := s.now().UTC()
	wasInProgress := t.Status == domain.TaskInProgress
	if in.Title != nil {
		t.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Priority != nil {
		t.Priority = *in.Priority
	}
	if in.EstimatedMinutes != nil {
		t.EstimatedMinutes = *in.EstimatedMinutes
	}
	if in.PlannedDate != nil {
		t.PlannedDate = trimOrNil(in.PlannedDate)
	}
	if in.ProjectID != nil {
		t.ProjectID = trimOrNil(in.ProjectID)
	}
	if in.RecurrenceRule != nil {
		t.RecurrenceRule = trimOrNil(in.RecurrenceRule)
	}
	parentChanged := in.ParentTaskID != nil
	if parentChanged {
		t.ParentTaskID = trimOrNil(in.ParentTaskID)
	}
	if in.DueAt != nil {
		if t.DueAt, err = parseOptTime("due_at", in.DueAt); err != nil {
			return domain.Task{}, err
		}
	}
	if in.PlannedStartAt != nil {
		if t.PlannedStartAt, err = parseOptTime("planned_start_at", in.PlannedStartAt); err != nil {
			return domain.Task{}, err
		}
	}
	completing := in.Status != nil && *in.Status == domain.TaskCompleted && t.Status != domain.TaskCompleted
	if in.Status != nil && *in.Status != t.Status {
		t.Status = *in.Status
		t.CompletedAt = nil
		if t.Status == domain.TaskCompleted {
			t.CompletedAt = &now
		}
	}
	if err := s.validate(t); err != nil {
		return domain.Task{}, err
	}
	if parentChanged && t.ParentTaskID != nil {
		if err := s.checkNoCycle(ctx, t.ID, *t.ParentTaskID); err != nil {
			return domain.Task{}, err
		}
	}
	t.UpdatedAt = now
	if wasInProgress && t.Status != domain.TaskInProgress && s.timers != nil {
		// Zuerst den Timer, dann die Task: Schlägt Update fehl, ist nur der Timer beendet.
		if err := s.timers.StopTimer(ctx, t.ID); err != nil {
			return domain.Task{}, err
		}
	}
	if completing && t.RecurrenceRule != nil {
		// Erst die nächste anlegen: Scheitert das, bleibt die Task offen und die Regel geht nicht verloren.
		if err := s.createNext(ctx, t); err != nil {
			return domain.Task{}, err
		}
		t.RecurrenceRule = nil // sonst entstünde nach Wiedereröffnen und erneutem Erledigen eine zweite
	}
	if err := s.store.Update(ctx, t); err != nil {
		return domain.Task{}, err
	}
	s.emit(domain.Event{Type: domain.EventTaskUpdated, ID: t.ID})
	return t, nil
}

// createNext legt die nächste Wiederholung von t an (siehe nextRecurrence).
func (s *TaskService) createNext(ctx context.Context, t domain.Task) error {
	next, err := nextRecurrence(t, s.loc, s.now())
	if err != nil || next == nil {
		return err
	}
	if err := s.store.Create(ctx, *next); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventTaskCreated, ID: next.ID})
	return nil
}

// nextRecurrence liefert die nächste Wiederholung von t: gleiche Felder, Tag (und Uhrzeiten) auf den nächsten
// Termin nach max(Tag von t, heute) verschoben. Ohne Tag zählt heute. Endet die Regel (UNTIL), kommt nil.
// shortcut: Teilaufgaben wandern nicht mit; erst nachziehen, wenn wiederkehrende Tasks mit Checklisten gebraucht werden.
func nextRecurrence(t domain.Task, loc *time.Location, clock time.Time) (*domain.Task, error) {
	rule, err := domain.ParseRule(*t.RecurrenceRule, loc)
	if err != nil {
		return nil, err
	}
	now := clock.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	anchor := today
	switch {
	case t.PlannedDate != nil:
		anchor, _ = time.ParseInLocation("2006-01-02", *t.PlannedDate, loc)
	case t.DueAt != nil:
		d := t.DueAt.In(loc)
		anchor = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	}
	from := anchor
	if today.After(from) {
		from = today
	}
	from = from.AddDate(0, 0, 1)
	starts := rule.Starts(anchor, from, from.AddDate(0, 0, 7*rule.Interval+7))
	if len(starts) == 0 {
		return nil, nil
	}
	days := int(math.Round(starts[0].Sub(anchor).Hours() / 24))
	shift := func(p *time.Time) *time.Time {
		if p == nil {
			return nil
		}
		v := p.In(loc).AddDate(0, 0, days).UTC()
		return &v
	}
	day := starts[0].Format("2006-01-02")
	at := clock.UTC()
	next := domain.Task{
		ID:               domain.NewID(),
		Title:            t.Title,
		Description:      t.Description,
		Status:           domain.TaskBacklog,
		Priority:         t.Priority,
		EstimatedMinutes: t.EstimatedMinutes,
		DueAt:            shift(t.DueAt),
		PlannedStartAt:   shift(t.PlannedStartAt),
		ProjectID:        t.ProjectID,
		ParentTaskID:     t.ParentTaskID,
		RecurrenceRule:   t.RecurrenceRule,
		CreatedAt:        at,
		UpdatedAt:        at,
	}
	if t.PlannedDate != nil || t.DueAt == nil {
		next.PlannedDate, next.Status = &day, domain.TaskPlanned
	}
	return &next, nil
}

// Delete löscht endgültig; die Zeiteinträge bleiben dabei geschützt (domain.ErrConflict).
func (s *TaskService) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventTaskDeleted, ID: id})
	return nil
}

// Trash legt die Task samt Teilaufgaben in den Papierkorb; mit laufendem Timer domain.ErrConflict.
func (s *TaskService) Trash(ctx context.Context, id string) error {
	if err := s.store.Trash(ctx, id, s.now()); err != nil {
		return err
	}
	s.emit(domain.Event{Type: domain.EventTaskDeleted, ID: id})
	return nil
}

func (s *TaskService) Restore(ctx context.Context, id string) (domain.Task, error) {
	t, err := s.store.Restore(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	s.emit(domain.Event{Type: domain.EventTaskCreated, ID: id})
	return t, nil
}

func (s *TaskService) ListTrashed(ctx context.Context) ([]domain.Task, error) {
	return s.store.ListTrashed(ctx)
}

// checkNoCycle stellt sicher, dass id nicht unter seinen eigenen Nachfahren hängt.
func (s *TaskService) checkNoCycle(ctx context.Context, id, parentID string) error {
	for cur, i := parentID, 0; i < maxTaskDepth; i++ {
		if cur == id {
			return fmt.Errorf("%w: Subtask-Zyklus", domain.ErrInvalid)
		}
		p, err := s.store.Get(ctx, cur)
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: übergeordnete Task existiert nicht", domain.ErrInvalid)
		}
		if err != nil {
			return err
		}
		if p.ParentTaskID == nil {
			return nil
		}
		cur = *p.ParentTaskID
	}
	return fmt.Errorf("%w: Subtask-Kette zu tief", domain.ErrInvalid)
}

func parseOptTime(field string, v *string) (*time.Time, error) {
	s := trimOrNil(v)
	if s == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s ist kein RFC-3339-Zeitpunkt", domain.ErrInvalid, field)
	}
	t = t.UTC()
	return &t, nil
}

func validateDate(field, v string) error {
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return fmt.Errorf("%w: %s muss YYYY-MM-DD sein", domain.ErrInvalid, field)
	}
	return nil
}

// validate prüft die Felder und die Wiederholungsregel. COUNT ergibt bei Tasks keinen Sinn, weil jede
// Wiederholung neu ab ihrem eigenen Tag zählt.
func (s *TaskService) validate(t domain.Task) error {
	if err := validateTask(t); err != nil {
		return err
	}
	if t.RecurrenceRule == nil {
		return nil
	}
	rule, err := domain.ParseRule(*t.RecurrenceRule, s.loc)
	if err != nil {
		return err
	}
	if rule.Count > 0 {
		return fmt.Errorf("%w: recurrence_rule: COUNT geht bei Tasks nicht, stattdessen UNTIL", domain.ErrInvalid)
	}
	return nil
}

func validateTask(t domain.Task) error {
	switch {
	case t.Title == "":
		return fmt.Errorf("%w: title darf nicht leer sein", domain.ErrInvalid)
	case !t.Status.Valid():
		return fmt.Errorf("%w: unbekannter Status %q", domain.ErrInvalid, t.Status)
	case !t.Priority.Valid():
		return fmt.Errorf("%w: unbekannte Priorität %q", domain.ErrInvalid, t.Priority)
	case t.EstimatedMinutes < 0:
		return fmt.Errorf("%w: estimated_minutes darf nicht negativ sein", domain.ErrInvalid)
	case t.PlannedStartAt != nil && t.PlannedDate == nil:
		return fmt.Errorf("%w: planned_start_at braucht planned_date", domain.ErrInvalid)
	case t.ParentTaskID != nil && *t.ParentTaskID == t.ID:
		return fmt.Errorf("%w: Task kann nicht ihre eigene Eltern-Task sein", domain.ErrInvalid)
	}
	if t.PlannedDate != nil {
		return validateDate("planned_date", *t.PlannedDate)
	}
	return nil
}
