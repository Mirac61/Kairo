package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"kairo/internal/domain"
)

// CalendarRepository speichert Kalender-Events in SQLite.
type CalendarRepository struct{ db *sql.DB }

// NewCalendarRepository erzeugt ein CalendarRepository.
func NewCalendarRepository(db *sql.DB) *CalendarRepository { return &CalendarRepository{db: db} }

const calendarColumns = `id, title, description, start_at, end_at, location, url, project_id, task_id,
	recurrence_rule, recurrence_exdates, created_at, updated_at`

func scanEvent(s scanner) (domain.CalendarEvent, error) {
	var (
		e                                 domain.CalendarEvent
		project, task, rule               sql.NullString
		start, end, exdates, created, upd string
	)
	if err := s.Scan(&e.ID, &e.Title, &e.Description, &start, &end, &e.Location, &e.URL,
		&project, &task, &rule, &exdates, &created, &upd); err != nil {
		return domain.CalendarEvent{}, err
	}
	e.ProjectID, e.TaskID, e.RecurrenceRule = nullStr(project), nullStr(task), nullStr(rule)
	if err := json.Unmarshal([]byte(exdates), &e.RecurrenceExdates); err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("recurrence_exdates: %w", err)
	}
	var err error
	if e.StartAt, err = parseTime(start); err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("start_at: %w", err)
	}
	if e.EndAt, err = parseTime(end); err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("end_at: %w", err)
	}
	if e.CreatedAt, err = parseTime(created); err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("created_at: %w", err)
	}
	if e.UpdatedAt, err = parseTime(upd); err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("updated_at: %w", err)
	}
	return e, nil
}

func encodeExdates(d []string) (string, error) {
	if d == nil {
		d = []string{}
	}
	b, err := json.Marshal(d)
	return string(b), err
}

// Create legt ein Event an.
func (r *CalendarRepository) Create(ctx context.Context, e domain.CalendarEvent) error {
	ex, err := encodeExdates(e.RecurrenceExdates)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO calendar_events (`+calendarColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.Description, formatTime(e.StartAt), formatTime(e.EndAt), e.Location, e.URL,
		e.ProjectID, e.TaskID, e.RecurrenceRule, ex, formatTime(e.CreatedAt), formatTime(e.UpdatedAt))
	if err != nil {
		return fmt.Errorf("repository: Event anlegen: %w", mapFK(err))
	}
	return nil
}

// Get liefert ein Event oder domain.ErrNotFound.
func (r *CalendarRepository) Get(ctx context.Context, id string) (domain.CalendarEvent, error) {
	e, err := scanEvent(r.db.QueryRowContext(ctx, `SELECT `+calendarColumns+` FROM calendar_events WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CalendarEvent{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CalendarEvent{}, fmt.Errorf("repository: Event lesen: %w", err)
	}
	return e, nil
}

// List liefert alle Events nach Beginn sortiert.
func (r *CalendarRepository) List(ctx context.Context) ([]domain.CalendarEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+calendarColumns+` FROM calendar_events ORDER BY start_at, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Events lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.CalendarEvent{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Event lesen: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Update überschreibt die veränderlichen Felder eines Events.
func (r *CalendarRepository) Update(ctx context.Context, e domain.CalendarEvent) error {
	ex, err := encodeExdates(e.RecurrenceExdates)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE calendar_events SET title = ?, description = ?, start_at = ?, end_at = ?, location = ?, url = ?,
			project_id = ?, task_id = ?, recurrence_rule = ?, recurrence_exdates = ?, updated_at = ? WHERE id = ?`,
		e.Title, e.Description, formatTime(e.StartAt), formatTime(e.EndAt), e.Location, e.URL,
		e.ProjectID, e.TaskID, e.RecurrenceRule, ex, formatTime(e.UpdatedAt), e.ID)
	if err != nil {
		return fmt.Errorf("repository: Event ändern: %w", mapFK(err))
	}
	return requireAffected(res)
}

// Delete löscht ein Event.
func (r *CalendarRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM calendar_events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Event löschen: %w", err)
	}
	return requireAffected(res)
}
