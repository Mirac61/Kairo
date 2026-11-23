package service

import (
	"context"
	"log/slog"
	"time"

	"kairo/internal/domain"
)

// explainPrefix kennzeichnet die SourceKey der Erklär-Checks: explain:<event_id>@<YYYY-MM-DD>.
const explainPrefix = "explain:"

// ExplainEvents liefert die konkreten Termine im Fenster [from, to).
type ExplainEvents interface {
	Occurrences(ctx context.Context, from, to time.Time) ([]domain.EventInstance, error)
}

// ExplainChecks legt nach dem Beginn eines Termins mit ExplainCheck eine Task „5 Min erklären“
// für denselben Tag an (ohne Uhrzeit, kein Zeitblock) und lässt unerledigte aus früheren Tagen
// verfallen. Erzeugt wird nur für heute und erst ab Beginn, damit verschobene oder gelöschte
// Termine nichts aufzuräumen brauchen.
type ExplainChecks struct {
	events ExplainEvents
	tasks  *TaskService
	loc    *time.Location
	now    func() time.Time
}

// NewExplainChecks erzeugt den Erklär-Check. now ist die Uhr (nil = time.Now).
func NewExplainChecks(events ExplainEvents, tasks *TaskService, loc *time.Location, now func() time.Time) *ExplainChecks {
	if now == nil {
		now = time.Now
	}
	return &ExplainChecks{events: events, tasks: tasks, loc: loc, now: now}
}

// Sync ist idempotent: mehrfach am Tag aufgerufen entsteht je Termin und Tag höchstens eine Task.
func (s *ExplainChecks) Sync(ctx context.Context) error {
	now := s.now().In(s.loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	date := dayStart.Format("2006-01-02")
	if err := s.expire(ctx, date); err != nil {
		return err
	}
	occ, err := s.events.Occurrences(ctx, dayStart.UTC(), now.UTC().Add(time.Second))
	if err != nil {
		return err
	}
	due := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, s.loc).UTC()
	at := now.UTC()
	for _, o := range occ {
		// Occurrences liefert auch Termine, die gestern begannen und heute noch laufen.
		if !o.Event.ExplainCheck || o.Start.Before(dayStart) || o.Start.After(now) {
			continue
		}
		key := explainPrefix + o.Event.ID + "@" + date
		if _, err := s.tasks.CreateGenerated(ctx, domain.Task{
			ID:               domain.NewID(),
			Title:            "5 Min erklären: " + o.Event.Title,
			Status:           domain.TaskPlanned,
			Priority:         domain.PriorityMedium,
			EstimatedMinutes: 5,
			DueAt:            &due,
			PlannedDate:      &date,
			ProjectID:        o.Event.ProjectID,
			SourceKey:        &key,
			CreatedAt:        at,
			UpdatedAt:        at,
		}); err != nil {
			return err
		}
	}
	return nil
}

// expire bricht offene, nie angefasste Erklär-Checks ab, die für einen Tag vor date geplant sind.
// Der Abend ist vorbei; so stapeln sie sich nicht als Überfälliges. Wer einen Check verschiebt,
// startet oder wieder öffnet, hat ihn angefasst (updated_at > created_at) und behält ihn.
func (s *ExplainChecks) expire(ctx context.Context, date string) error {
	return s.tasks.CancelUntouched(ctx, explainPrefix, date)
}

// Run ruft Sync sofort und danach alle interval auf, bis ctx endet. Fehler werden nur geloggt.
func (s *ExplainChecks) Run(ctx context.Context, interval time.Duration, log *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.Sync(ctx); err != nil && ctx.Err() == nil {
			log.Error("Erklär-Check", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
