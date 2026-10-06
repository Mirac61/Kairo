package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
)

// TimeStore ist der Speicher, den der TimeTrackingService braucht.
type TimeStore interface {
	// Create legt einen einzelnen Eintrag an. Läuft schon ein Timer und der
	// neue Eintrag hat kein EndedAt, liefert es domain.ErrConflict.
	Create(ctx context.Context, e domain.TimeEntry) error
	List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error)
	// Tx führt fn atomar aus: Fehler von fn machen alle Änderungen rückgängig.
	Tx(ctx context.Context, fn func(TimeTx) error) error
}

// TimeTx sind die Operationen innerhalb einer Transaktion von TimeStore.Tx.
type TimeTx interface {
	GetTask(ctx context.Context, id string) (domain.Task, error)
	UpdateTask(ctx context.Context, t domain.Task) error
	// RunningEntry liefert den laufenden Timer oder nil.
	RunningEntry(ctx context.Context) (*domain.TimeEntry, error)
	CreateEntry(ctx context.Context, e domain.TimeEntry) error
	CloseEntry(ctx context.Context, id string, endedAt time.Time) error
}

// TimeTrackingService enthält die Timer-Regeln: höchstens ein laufender
// Timer, Zeit nur aus echten Zeitstempeln, kein automatischer Start.
type TimeTrackingService struct {
	publisher
	store TimeStore
	now   func() time.Time
}

// NewTimeTrackingService erzeugt einen TimeTrackingService. now ist die Uhr (nil = time.Now).
func NewTimeTrackingService(store TimeStore, now func() time.Time) *TimeTrackingService {
	if now == nil {
		now = time.Now
	}
	return &TimeTrackingService{store: store, now: now}
}

// Start startet den Timer einer Task und setzt sie auf IN_PROGRESS. Läuft
// ein Timer für eine andere Task, wird dieser beendet und seine Task
// PAUSED. Läuft die Task schon, ändert sich nichts. Abgeschlossene oder
// abgebrochene Tasks lassen sich nicht starten (domain.ErrConflict).
// source ist standardmäßig MANUAL.
func (s *TimeTrackingService) Start(ctx context.Context, taskID string, source domain.TimeSource) (domain.TimerResult, error) {
	if source == "" {
		source = domain.SourceManual
	}
	if !source.Valid() {
		return domain.TimerResult{}, fmt.Errorf("%w: unbekannte Quelle %q", domain.ErrInvalid, source)
	}
	var (
		res domain.TimerResult
		evs []domain.Event
	)
	err := s.store.Tx(ctx, func(tx TimeTx) error {
		evs = evs[:0]
		now := s.now().UTC()
		task, err := tx.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		if task.Status == domain.TaskCompleted || task.Status == domain.TaskCancelled {
			return fmt.Errorf("%w: Task ist %s und lässt sich nicht starten", domain.ErrConflict, task.Status)
		}
		running, err := tx.RunningEntry(ctx)
		if err != nil {
			return err
		}
		if running != nil {
			if running.TaskID != nil && *running.TaskID == taskID {
				res = domain.TimerResult{Task: task, Entry: running}
				evs = nil
				return nil
			}
			if err := pauseRunning(ctx, tx, *running, now); err != nil {
				return err
			}
			evs = append(evs, domain.Event{Type: domain.EventTimerStopped, ID: running.ID, TaskID: deref(running.TaskID)})
			if running.TaskID != nil {
				evs = append(evs, domain.Event{Type: domain.EventTaskPaused, ID: *running.TaskID})
			}
		}
		task.Status, task.CompletedAt, task.UpdatedAt = domain.TaskInProgress, nil, now
		if err := tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		entry := domain.TimeEntry{ID: domain.NewID(), TaskID: &task.ID, ProjectID: task.ProjectID, StartedAt: now, Source: source}
		if err := tx.CreateEntry(ctx, entry); err != nil {
			return err
		}
		res = domain.TimerResult{Task: task, Entry: &entry}
		evs = append(evs,
			domain.Event{Type: domain.EventTaskStarted, ID: task.ID},
			domain.Event{Type: domain.EventTimerStarted, ID: entry.ID, TaskID: task.ID})
		return nil
	})
	if err == nil {
		s.emit(evs...)
	}
	return res, err
}

// Pause beendet den laufenden Timer der Task und setzt sie auf PAUSED.
// Läuft für die Task kein Timer, liefert es domain.ErrConflict.
func (s *TimeTrackingService) Pause(ctx context.Context, taskID string) (domain.TimerResult, error) {
	var res domain.TimerResult
	err := s.store.Tx(ctx, func(tx TimeTx) error {
		now := s.now().UTC()
		task, err := tx.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		running, err := tx.RunningEntry(ctx)
		if err != nil {
			return err
		}
		if !runsFor(running, taskID) {
			return fmt.Errorf("%w: für die Task läuft kein Timer", domain.ErrConflict)
		}
		closed, err := closeEntry(ctx, tx, *running, now)
		if err != nil {
			return err
		}
		task.Status, task.UpdatedAt = domain.TaskPaused, now
		if err := tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		res = domain.TimerResult{Task: task, Entry: &closed}
		return nil
	})
	if err == nil {
		s.emit(
			domain.Event{Type: domain.EventTimerStopped, ID: res.Entry.ID, TaskID: taskID},
			domain.Event{Type: domain.EventTaskPaused, ID: taskID})
	}
	return res, err
}

// Complete beendet einen laufenden Timer der Task und setzt sie auf
// COMPLETED. Eine schon abgeschlossene Task bleibt unverändert; eine
// abgebrochene liefert domain.ErrConflict.
func (s *TimeTrackingService) Complete(ctx context.Context, taskID string) (domain.TimerResult, error) {
	var (
		res       domain.TimerResult
		completed bool
		changed   bool
	)
	err := s.store.Tx(ctx, func(tx TimeTx) error {
		now := s.now().UTC()
		task, err := tx.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		if task.Status == domain.TaskCancelled {
			return fmt.Errorf("%w: Task ist abgebrochen und lässt sich nicht abschließen", domain.ErrConflict)
		}
		running, err := tx.RunningEntry(ctx)
		if err != nil {
			return err
		}
		if runsFor(running, taskID) {
			closed, err := closeEntry(ctx, tx, *running, now)
			if err != nil {
				return err
			}
			res.Entry = &closed
		}
		if task.Status != domain.TaskCompleted {
			changed = true
			task.Status, task.CompletedAt, task.UpdatedAt = domain.TaskCompleted, &now, now
			if err := tx.UpdateTask(ctx, task); err != nil {
				return err
			}
		}
		res.Task = task
		completed = task.Status == domain.TaskCompleted && changed
		return nil
	})
	if err == nil {
		if res.Entry != nil {
			s.emit(domain.Event{Type: domain.EventTimerStopped, ID: res.Entry.ID, TaskID: taskID})
		}
		if completed {
			s.emit(domain.Event{Type: domain.EventTaskCompleted, ID: taskID})
		}
	}
	return res, err
}

// StopTimer beendet den laufenden Timer der Task, falls einer läuft. Den
// Status der Task ändert es nicht; das tut der Aufrufer (z. B. TaskService
// beim Ändern des Status).
func (s *TimeTrackingService) StopTimer(ctx context.Context, taskID string) error {
	var stopped *domain.TimeEntry
	err := s.store.Tx(ctx, func(tx TimeTx) error {
		stopped = nil
		running, err := tx.RunningEntry(ctx)
		if err != nil || !runsFor(running, taskID) {
			return err
		}
		closed, err := closeEntry(ctx, tx, *running, s.now().UTC())
		stopped = &closed
		return err
	})
	if err == nil && stopped != nil {
		s.emit(domain.Event{Type: domain.EventTimerStopped, ID: stopped.ID, TaskID: taskID})
	}
	return err
}

// CreateTimeEntryInput beschreibt einen nachträglich erfassten, schon
// beendeten Zeiteintrag. Genau eines von TaskID und ProjectID muss gesetzt
// sein. StartedAt und EndedAt sind RFC 3339.
type CreateTimeEntryInput struct {
	TaskID    *string
	ProjectID *string
	StartedAt string
	EndedAt   string
	Source    domain.TimeSource // Standard: MANUAL
}

// Create erfasst einen beendeten Zeiteintrag nachträglich. Laufende Timer
// entstehen nur über Start.
func (s *TimeTrackingService) Create(ctx context.Context, in CreateTimeEntryInput) (domain.TimeEntry, error) {
	if in.Source == "" {
		in.Source = domain.SourceManual
	}
	e := domain.TimeEntry{
		ID:        domain.NewID(),
		TaskID:    trimOrNil(in.TaskID),
		ProjectID: trimOrNil(in.ProjectID),
		Source:    in.Source,
	}
	started, err := parseOptTime("started_at", &in.StartedAt)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	ended, err := parseOptTime("ended_at", &in.EndedAt)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	switch {
	case started == nil:
		return domain.TimeEntry{}, fmt.Errorf("%w: started_at ist Pflicht", domain.ErrInvalid)
	case ended == nil:
		return domain.TimeEntry{}, fmt.Errorf("%w: ended_at ist Pflicht (einen laufenden Timer startet POST /api/tasks/{id}/start)", domain.ErrInvalid)
	case (e.TaskID == nil) == (e.ProjectID == nil):
		return domain.TimeEntry{}, fmt.Errorf("%w: genau eines von task_id und project_id angeben", domain.ErrInvalid)
	case !e.Source.Valid():
		return domain.TimeEntry{}, fmt.Errorf("%w: unbekannte Quelle %q", domain.ErrInvalid, e.Source)
	case !ended.After(*started):
		return domain.TimeEntry{}, fmt.Errorf("%w: ended_at muss nach started_at liegen", domain.ErrInvalid)
	case ended.After(s.now()):
		return domain.TimeEntry{}, fmt.Errorf("%w: ended_at darf nicht in der Zukunft liegen", domain.ErrInvalid)
	}
	e.StartedAt, e.EndedAt = *started, ended
	if err := s.store.Create(ctx, e); err != nil {
		return domain.TimeEntry{}, err
	}
	return e, nil
}

func (s *TimeTrackingService) List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error) {
	f.TaskID, f.ProjectID = strings.TrimSpace(f.TaskID), strings.TrimSpace(f.ProjectID)
	return s.store.List(ctx, f)
}

func runsFor(running *domain.TimeEntry, taskID string) bool {
	return running != nil && running.TaskID != nil && *running.TaskID == taskID
}

// closeEntry beendet e zum Zeitpunkt at (nie vor dem Start, falls die Uhr zurückspringt).
func closeEntry(ctx context.Context, tx TimeTx, e domain.TimeEntry, at time.Time) (domain.TimeEntry, error) {
	if at.Before(e.StartedAt) {
		at = e.StartedAt
	}
	if err := tx.CloseEntry(ctx, e.ID, at); err != nil {
		return domain.TimeEntry{}, err
	}
	e.EndedAt = &at
	return e, nil
}

// pauseRunning beendet den laufenden Timer und pausiert seine Task.
func pauseRunning(ctx context.Context, tx TimeTx, running domain.TimeEntry, now time.Time) error {
	if _, err := closeEntry(ctx, tx, running, now); err != nil {
		return err
	}
	if running.TaskID == nil {
		return nil
	}
	task, err := tx.GetTask(ctx, *running.TaskID)
	if err != nil {
		return err
	}
	if task.Status != domain.TaskInProgress {
		return nil
	}
	task.Status, task.UpdatedAt = domain.TaskPaused, now
	return tx.UpdateTask(ctx, task)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
