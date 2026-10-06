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
	estimated_minutes, due_at, planned_date, planned_start_at, created_at, updated_at, completed_at`

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

func scanTask(s scanner) (domain.Task, error) {
	var (
		t                                           domain.Task
		project, parent, due, planned, plannedStart sql.NullString
		completed                                   sql.NullString
		status, priority, created, updated          string
	)
	if err := s.Scan(&t.ID, &project, &parent, &t.Title, &t.Description, &status, &priority,
		&t.EstimatedMinutes, &due, &planned, &plannedStart, &created, &updated, &completed); err != nil {
		return domain.Task{}, err
	}
	t.Status, t.Priority = domain.TaskStatus(status), domain.TaskPriority(priority)
	t.ProjectID, t.ParentTaskID, t.PlannedDate = nullStr(project), nullStr(parent), nullStr(planned)
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
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (`+taskColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.ParentTaskID, t.Title, t.Description, string(t.Status), string(t.Priority),
		t.EstimatedMinutes, nullTime(t.DueAt), t.PlannedDate, nullTime(t.PlannedStartAt),
		formatTime(t.CreatedAt), formatTime(t.UpdatedAt), nullTime(t.CompletedAt))
	if err != nil {
		return fmt.Errorf("repository: Task anlegen: %w", mapFK(err))
	}
	return nil
}

// Get liefert eine Task oder domain.ErrNotFound.
func (r *TaskRepository) Get(ctx context.Context, id string) (domain.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
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
		where []string
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

// Update überschreibt die veränderlichen Felder einer Task.
func (r *TaskRepository) Update(ctx context.Context, t domain.Task) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET project_id = ?, parent_task_id = ?, title = ?, description = ?, status = ?,
			priority = ?, estimated_minutes = ?, due_at = ?, planned_date = ?, planned_start_at = ?,
			updated_at = ?, completed_at = ? WHERE id = ?`,
		t.ProjectID, t.ParentTaskID, t.Title, t.Description, string(t.Status), string(t.Priority),
		t.EstimatedMinutes, nullTime(t.DueAt), t.PlannedDate, nullTime(t.PlannedStartAt),
		formatTime(t.UpdatedAt), nullTime(t.CompletedAt), t.ID)
	if err != nil {
		return fmt.Errorf("repository: Task ändern: %w", mapFK(err))
	}
	return requireAffected(res)
}

// Delete löscht eine Task samt Subtasks.
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Task löschen: %w", err)
	}
	return requireAffected(res)
}
