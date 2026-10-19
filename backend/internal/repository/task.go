package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"kairo/internal/domain"
)

// TaskRepository speichert Tasks in SQLite.
type TaskRepository struct{ db *sql.DB }

// NewTaskRepository erzeugt ein TaskRepository.
func NewTaskRepository(db *sql.DB) *TaskRepository { return &TaskRepository{db: db} }

const taskColumns = `id, project_id, parent_task_id, title, description, status, priority,
	estimated_minutes, due_at, planned_date, planned_start_at, created_at, updated_at, completed_at, recurrence_rule`

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullStr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// scanTask liest taskColumns; extra sind weitere Zielvariablen für Spalten dahinter.
func scanTask(s scanner, extra ...any) (domain.Task, error) {
	var (
		t                                           domain.Task
		project, parent, due, planned, plannedStart sql.NullString
		completed, rule                             sql.NullString
		status, priority, created, updated          string
	)
	if err := s.Scan(append([]any{&t.ID, &project, &parent, &t.Title, &t.Description, &status, &priority,
		&t.EstimatedMinutes, &due, &planned, &plannedStart, &created, &updated, &completed, &rule}, extra...)...); err != nil {
		return domain.Task{}, err
	}
	t.Status, t.Priority = domain.TaskStatus(status), domain.TaskPriority(priority)
	t.ProjectID, t.ParentTaskID, t.PlannedDate, t.RecurrenceRule = nullStr(project), nullStr(parent), nullStr(planned), nullStr(rule)
	var err error
	if t.DueAt, err = parseNullTime(due); err != nil {
		return domain.Task{}, fmt.Errorf("due_at: %w", err)
	}
	if t.PlannedStartAt, err = parseNullTime(plannedStart); err != nil {
		return domain.Task{}, fmt.Errorf("planned_start_at: %w", err)
	}
	if t.CompletedAt, err = parseNullTime(completed); err != nil {
		return domain.Task{}, fmt.Errorf("completed_at: %w", err)
	}
	if t.CreatedAt, err = parseTime(created); err != nil {
		return domain.Task{}, fmt.Errorf("created_at: %w", err)
	}
	if t.UpdatedAt, err = parseTime(updated); err != nil {
		return domain.Task{}, fmt.Errorf("updated_at: %w", err)
	}
	return t, nil
}

// mapFK übersetzt eine verletzte Fremdschlüssel-Beziehung in domain.ErrInvalid.
func mapFK(err error) error {
	if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		return fmt.Errorf("%w: verknüpftes Projekt oder Task existiert nicht", domain.ErrInvalid)
	}
	return err
}

// Create legt eine Task an.
func (r *TaskRepository) Create(ctx context.Context, t domain.Task) error {
	return createTask(ctx, r.db, t)
}

func createTask(ctx context.Context, q dbtx, t domain.Task) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO tasks (`+taskColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.ParentTaskID, t.Title, t.Description, string(t.Status), string(t.Priority),
		t.EstimatedMinutes, nullTime(t.DueAt), t.PlannedDate, nullTime(t.PlannedStartAt),
		formatTime(t.CreatedAt), formatTime(t.UpdatedAt), nullTime(t.CompletedAt), t.RecurrenceRule)
	if err != nil {
		return fmt.Errorf("repository: Task anlegen: %w", mapFK(err))
	}
	return nil
}

// dbtx ist die gemeinsame Teilmenge von *sql.DB und *sql.Tx, damit dieselben
// Abfragen auch innerhalb einer Transaktion laufen.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Get liefert eine Task oder domain.ErrNotFound.
func (r *TaskRepository) Get(ctx context.Context, id string) (domain.Task, error) {
	return getTask(ctx, r.db, id)
}

func getTask(ctx context.Context, q dbtx, id string) (domain.Task, error) {
	t, err := scanTask(q.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("repository: Task lesen: %w", err)
	}
	return t, nil
}

// List liefert Tasks nach Filter, älteste zuerst.
func (r *TaskRepository) List(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error) {
	var (
		where = []string{"deleted_at IS NULL"}
		args  []any
	)
	if f.Status != "" {
		where, args = append(where, "status = ?"), append(args, string(f.Status))
	}
	if f.ProjectID != "" {
		where, args = append(where, "project_id = ?"), append(args, f.ProjectID)
	}
	if f.PlannedDate != "" {
		where, args = append(where, "planned_date = ?"), append(args, f.PlannedDate)
	}
	if f.PlannedBefore != "" {
		where, args = append(where, "planned_date < ?"), append(args, f.PlannedBefore)
	}
	q := `SELECT ` + taskColumns + ` FROM tasks`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: Tasks lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Task lesen: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateAndCreate schreibt t und legt next in einer Transaktion an.
func (r *TaskRepository) UpdateAndCreate(ctx context.Context, t, next domain.Task) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: Transaktion: %w", err)
	}
	defer tx.Rollback()
	if err := updateTask(ctx, tx, t); err != nil {
		return err
	}
	if err := createTask(ctx, tx, next); err != nil {
		return err
	}
	return tx.Commit()
}

// Update überschreibt die veränderlichen Felder einer Task.
func (r *TaskRepository) Update(ctx context.Context, t domain.Task) error {
	return updateTask(ctx, r.db, t)
}

// UpcomingTasks liefert offene Tasks mit geplantem Beginn im Fenster (lo, hi]
// (beide UTC), aufsteigend. Für die Erinnerung: nur Tasks mit Uhrzeit.
func (r *TaskRepository) UpcomingTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM tasks
		WHERE deleted_at IS NULL AND planned_start_at IS NOT NULL
			AND status NOT IN ('COMPLETED', 'CANCELLED')
			AND planned_start_at > ? AND planned_start_at <= ?
		ORDER BY planned_start_at, id`, formatTime(lo), formatTime(hi))
	if err != nil {
		return nil, fmt.Errorf("repository: kommende Tasks lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Task lesen: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DueTasks liefert offene Tasks mit Fälligkeit im Fenster (lo, hi] (beide UTC), aufsteigend.
// Für die tägliche Meldung „heute fällig“.
func (r *TaskRepository) DueTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM tasks
		WHERE deleted_at IS NULL AND due_at IS NOT NULL
			AND status NOT IN ('COMPLETED', 'CANCELLED')
			AND due_at > ? AND due_at <= ?
		ORDER BY due_at, id`, formatTime(lo), formatTime(hi))
	if err != nil {
		return nil, fmt.Errorf("repository: fällige Tasks lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Task lesen: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func updateTask(ctx context.Context, q dbtx, t domain.Task) error {
	res, err := q.ExecContext(ctx,
		`UPDATE tasks SET project_id = ?, parent_task_id = ?, title = ?, description = ?, status = ?,
			priority = ?, estimated_minutes = ?, due_at = ?, planned_date = ?, planned_start_at = ?,
			updated_at = ?, completed_at = ?, recurrence_rule = ? WHERE id = ? AND deleted_at IS NULL`,
		t.ProjectID, t.ParentTaskID, t.Title, t.Description, string(t.Status), string(t.Priority),
		t.EstimatedMinutes, nullTime(t.DueAt), t.PlannedDate, nullTime(t.PlannedStartAt),
		formatTime(t.UpdatedAt), nullTime(t.CompletedAt), t.RecurrenceRule, t.ID)
	if err != nil {
		return fmt.Errorf("repository: Task ändern: %w", mapFK(err))
	}
	return requireAffected(res)
}

// Delete löscht eine Task (auch aus dem Papierkorb) endgültig samt Subtasks. Hat eine davon Zeiteinträge, liefert
// es domain.ErrConflict, weil ON DELETE CASCADE sie sonst mitlöschen würde.
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	var tracked bool
	if err := r.db.QueryRowContext(ctx, `WITH RECURSIVE sub(id) AS (
		SELECT ? UNION ALL SELECT t.id FROM tasks t JOIN sub ON t.parent_task_id = sub.id
	) SELECT EXISTS (SELECT 1 FROM time_entries WHERE task_id IN (SELECT id FROM sub))`, id).Scan(&tracked); err != nil {
		return fmt.Errorf("repository: Zeiteinträge prüfen: %w", err)
	}
	if tracked {
		return fmt.Errorf("%w: Task hat Zeiteinträge, stattdessen abbrechen", domain.ErrConflict)
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Task löschen: %w", err)
	}
	return requireAffected(res)
}
