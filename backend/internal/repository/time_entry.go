package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
	"kairo/internal/service"
)

// TimeEntryRepository speichert Zeiteinträge in SQLite.
type TimeEntryRepository struct{ db *sql.DB }

// NewTimeEntryRepository erzeugt ein TimeEntryRepository.
func NewTimeEntryRepository(db *sql.DB) *TimeEntryRepository { return &TimeEntryRepository{db: db} }

// entryTimeLayout hat feste Breite (Millisekunden, UTC), damit der
// Textvergleich in SQL der Zeitreihenfolge entspricht. Das Format bleibt
// gültiges RFC 3339 und lässt sich mit parseTime lesen.
const entryTimeLayout = "2006-01-02T15:04:05.000Z"

func formatEntryTime(t time.Time) string { return t.UTC().Format(entryTimeLayout) }

func nullEntryTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatEntryTime(*t)
}

// timeEntrySelect liest das wirksame Projekt: bei Task-Einträgen das der Task.
const timeEntrySelect = `SELECT te.id, te.task_id, COALESCE(te.project_id, t.project_id), te.started_at, te.ended_at, te.source
	FROM time_entries te LEFT JOIN tasks t ON t.id = te.task_id`

func scanTimeEntry(s scanner) (domain.TimeEntry, error) {
	var (
		e                    domain.TimeEntry
		task, project, ended sql.NullString
		started, source      string
	)
	if err := s.Scan(&e.ID, &task, &project, &started, &ended, &source); err != nil {
		return domain.TimeEntry{}, err
	}
	e.TaskID, e.ProjectID, e.Source = nullStr(task), nullStr(project), domain.TimeSource(source)
	var err error
	if e.StartedAt, err = parseTime(started); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("started_at: %w", err)
	}
	if e.EndedAt, err = parseNullTime(ended); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("ended_at: %w", err)
	}
	return e, nil
}

// mapEntryErr übersetzt verletzte Constraints in Domain-Fehler.
func mapEntryErr(err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: es läuft bereits ein Timer", domain.ErrConflict)
	}
	return mapFK(err)
}

func insertEntry(ctx context.Context, q dbtx, e domain.TimeEntry) error {
	// Bei Task-Einträgen ergibt sich das Projekt aus der Task und wird nicht gespeichert.
	project := e.ProjectID
	if e.TaskID != nil {
		project = nil
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO time_entries (id, task_id, project_id, started_at, ended_at, source) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.TaskID, project, formatEntryTime(e.StartedAt), nullEntryTime(e.EndedAt), string(e.Source))
	if err != nil {
		return fmt.Errorf("repository: Zeiteintrag anlegen: %w", mapEntryErr(err))
	}
	return nil
}

// Create legt einen Zeiteintrag an. Ein zweiter Eintrag ohne EndedAt
// liefert domain.ErrConflict.
func (r *TimeEntryRepository) Create(ctx context.Context, e domain.TimeEntry) error {
	return insertEntry(ctx, r.db, e)
}

// List liefert Zeiteinträge nach Filter, neueste zuerst.
func (r *TimeEntryRepository) List(ctx context.Context, f domain.TimeEntryFilter) ([]domain.TimeEntry, error) {
	var (
		where []string
		args  []any
	)
	if f.TaskID != "" {
		where, args = append(where, "te.task_id = ?"), append(args, f.TaskID)
	}
	if f.ProjectID != "" {
		where, args = append(where, "COALESCE(te.project_id, t.project_id) = ?"), append(args, f.ProjectID)
	}
	if f.From != nil {
		where, args = append(where, "te.started_at >= ?"), append(args, formatEntryTime(*f.From))
	}
	if f.To != nil {
		where, args = append(where, "te.started_at < ?"), append(args, formatEntryTime(*f.To))
	}
	if f.Running {
		where = append(where, "te.ended_at IS NULL")
	}
	q := timeEntrySelect
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY te.started_at DESC, te.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: Zeiteinträge lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.TimeEntry{}
	for rows.Next() {
		e, err := scanTimeEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Zeiteintrag lesen: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Tx führt fn in einer Transaktion aus. Gibt fn einen Fehler zurück, wird
// zurückgerollt.
func (r *TimeEntryRepository) Tx(ctx context.Context, fn func(service.TimeTx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: Transaktion starten: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // nach Commit wirkungslos
	if err := fn(timeTx{tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: Transaktion abschließen: %w", mapEntryErr(err))
	}
	return nil
}

type timeTx struct{ q dbtx }

func (t timeTx) GetTask(ctx context.Context, id string) (domain.Task, error) {
	return getTask(ctx, t.q, id)
}

func (t timeTx) UpdateTask(ctx context.Context, task domain.Task) error {
	return updateTask(ctx, t.q, task)
}

func (t timeTx) RunningEntry(ctx context.Context) (*domain.TimeEntry, error) {
	e, err := scanTimeEntry(t.q.QueryRowContext(ctx, timeEntrySelect+` WHERE te.ended_at IS NULL`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("repository: laufenden Timer lesen: %w", err)
	}
	return &e, nil
}

func (t timeTx) CreateEntry(ctx context.Context, e domain.TimeEntry) error {
	return insertEntry(ctx, t.q, e)
}

func (t timeTx) CloseEntry(ctx context.Context, id string, endedAt time.Time) error {
	res, err := t.q.ExecContext(ctx,
		`UPDATE time_entries SET ended_at = ? WHERE id = ? AND ended_at IS NULL`, formatEntryTime(endedAt), id)
	if err != nil {
		return fmt.Errorf("repository: Zeiteintrag beenden: %w", err)
	}
	return requireAffected(res)
}
