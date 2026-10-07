package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"kairo/internal/domain"
)

// Papierkorb: Tasks, Termine und Habits tragen deleted_at (NULL = aktiv; Zeitpunkte mit fester Breite wie bei
// Zeiteinträgen, damit die Textsortierung stimmt). Get, List und Update
// sehen nur aktive Zeilen; Delete (hart) trifft auch Zeilen im Papierkorb.

// softDelete setzt deleted_at einer aktiven Zeile; domain.ErrNotFound, wenn es keine gibt.
func softDelete(ctx context.Context, db dbtx, table, id string, at time.Time) error {
	res, err := db.ExecContext(ctx, `UPDATE `+table+` SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, formatEntryTime(at), id)
	if err != nil {
		return fmt.Errorf("repository: in den Papierkorb legen: %w", err)
	}
	return requireAffected(res)
}

// softRestore holt eine Zeile zurück; eine aktive bleibt unverändert, eine fehlende ist domain.ErrNotFound.
func softRestore(ctx context.Context, db dbtx, table, id string) error {
	res, err := db.ExecContext(ctx, `UPDATE `+table+` SET deleted_at = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: wiederherstellen: %w", err)
	}
	return requireAffected(res)
}

// listTrashed führt query aus (Spalten wie bei scan, danach deleted_at) und setzt DeletedAt.
func listTrashed[T any](ctx context.Context, db dbtx, query string, scan func(scanner, ...any) (T, error), setDeleted func(*T, *time.Time)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("repository: Papierkorb lesen: %w", err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var del string
		v, err := scan(rows, &del)
		if err != nil {
			return nil, fmt.Errorf("repository: Papierkorb lesen: %w", err)
		}
		at, err := parseTime(del)
		if err != nil {
			return nil, fmt.Errorf("repository: deleted_at: %w", err)
		}
		setDeleted(&v, &at)
		out = append(out, v)
	}
	return out, rows.Err()
}

// Trash legt einen Termin in den Papierkorb.
func (r *CalendarRepository) Trash(ctx context.Context, id string, at time.Time) error {
	return softDelete(ctx, r.db, "calendar_events", id, at)
}

// Restore holt einen Termin zurück (idempotent) und liefert ihn.
func (r *CalendarRepository) Restore(ctx context.Context, id string) (domain.CalendarEvent, error) {
	if err := softRestore(ctx, r.db, "calendar_events", id); err != nil {
		return domain.CalendarEvent{}, err
	}
	return r.Get(ctx, id)
}

// ListTrashed liefert die Termine im Papierkorb, zuletzt gelöschte zuerst.
func (r *CalendarRepository) ListTrashed(ctx context.Context) ([]domain.CalendarEvent, error) {
	return listTrashed(ctx, r.db, `SELECT `+calendarColumns+`, deleted_at FROM calendar_events
		WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC, id`, scanEvent,
		func(e *domain.CalendarEvent, t *time.Time) { e.DeletedAt = t })
}

// Trash legt ein Habit in den Papierkorb.
func (r *HabitRepository) Trash(ctx context.Context, id string, at time.Time) error {
	return softDelete(ctx, r.db, "habits", id, at)
}

// Restore holt ein Habit zurück (idempotent) und liefert es.
func (r *HabitRepository) Restore(ctx context.Context, id string) (domain.Habit, error) {
	if err := softRestore(ctx, r.db, "habits", id); err != nil {
		return domain.Habit{}, err
	}
	return r.Get(ctx, id)
}

// ListTrashed liefert die Habits im Papierkorb, zuletzt gelöschte zuerst.
func (r *HabitRepository) ListTrashed(ctx context.Context) ([]domain.Habit, error) {
	return listTrashed(ctx, r.db, `SELECT `+habitColumns+`, deleted_at FROM habits
		WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC, id`, scanHabit,
		func(h *domain.Habit, t *time.Time) { h.DeletedAt = t })
}

// Trash legt eine Task samt Teilaufgaben mit gleichem Zeitstempel in den Papierkorb. Läuft ein
// Timer auf einer davon, liefert es domain.ErrConflict. Bereits gelöschte Teilaufgaben behalten
// ihren Zeitstempel.
func (r *TaskRepository) Trash(ctx context.Context, id string, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `WITH RECURSIVE sub(id) AS (
		SELECT id FROM tasks WHERE id = ? AND deleted_at IS NULL
		UNION ALL SELECT t.id FROM tasks t JOIN sub ON t.parent_task_id = sub.id WHERE t.deleted_at IS NULL
	) UPDATE tasks SET deleted_at = ? WHERE id IN (SELECT id FROM sub)
		AND NOT EXISTS (SELECT 1 FROM time_entries WHERE ended_at IS NULL AND task_id IN (SELECT id FROM sub))`,
		id, formatEntryTime(at))
	if err != nil {
		return fmt.Errorf("repository: Task in den Papierkorb legen: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n > 0 {
		return err
	}
	// Nichts geändert: entweder gibt es die Task nicht (mehr), oder ein Timer läuft.
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	return fmt.Errorf("%w: auf der Task oder einer Teilaufgabe läuft ein Timer, erst pausieren", domain.ErrConflict)
}

// Restore holt eine Task samt der mit ihr gelöschten Teilaufgaben zurück (idempotent) und liefert sie.
func (r *TaskRepository) Restore(ctx context.Context, id string) (domain.Task, error) {
	var at sql.NullString
	if err := r.db.QueryRowContext(ctx, `SELECT deleted_at FROM tasks WHERE id = ?`, id).Scan(&at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("repository: Task lesen: %w", err)
	}
	if at.Valid {
		if _, err := r.db.ExecContext(ctx, `WITH RECURSIVE sub(id) AS (
			SELECT ? UNION ALL SELECT t.id FROM tasks t JOIN sub ON t.parent_task_id = sub.id
		) UPDATE tasks SET deleted_at = NULL WHERE id IN (SELECT id FROM sub) AND deleted_at = ?`, id, at.String); err != nil {
			return domain.Task{}, fmt.Errorf("repository: Task wiederherstellen: %w", err)
		}
	}
	return r.Get(ctx, id)
}

// ListTrashed liefert die Tasks im Papierkorb, zuletzt gelöschte zuerst. Teilaufgaben, die
// zusammen mit ihrer Elterntask gelöscht wurden, stehen nicht einzeln darin.
func (r *TaskRepository) ListTrashed(ctx context.Context) ([]domain.Task, error) {
	return listTrashed(ctx, r.db, `SELECT `+taskColumns+`, deleted_at FROM tasks t WHERE deleted_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM tasks p WHERE p.id = t.parent_task_id AND p.deleted_at = t.deleted_at)
		ORDER BY deleted_at DESC, id`, scanTask,
		func(t *domain.Task, at *time.Time) { t.DeletedAt = at })
}
