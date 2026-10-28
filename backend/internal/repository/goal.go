package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"kairo/internal/domain"
)

// GoalRepository speichert die Ziele des Motivationsboards in SQLite.
type GoalRepository struct{ db *sql.DB }

// NewGoalRepository erzeugt ein GoalRepository.
func NewGoalRepository(db *sql.DB) *GoalRepository { return &GoalRepository{db: db} }

const goalColumns = `id, title, why, image_path, kind, project_id, habit_id, target, start_date, due_date, archived, created_at, parent_id, archived_at`

func scanGoal(s scanner) (domain.Goal, error) {
	var (
		g                               domain.Goal
		image, proj, habit, due, parent sql.NullString
		archivedAt                      sql.NullString
		kind, created                   string
		archived                        int
	)
	if err := s.Scan(&g.ID, &g.Title, &g.Why, &image, &kind, &proj, &habit, &g.Target, &g.StartDate, &due, &archived, &created, &parent, &archivedAt); err != nil {
		return domain.Goal{}, err
	}
	g.Kind, g.Archived = domain.GoalKind(kind), archived == 1
	g.ImagePath, g.ProjectID, g.HabitID, g.DueDate = nullStr(image), nullStr(proj), nullStr(habit), nullStr(due)
	g.ParentID = nullStr(parent)
	var err error
	if g.CreatedAt, err = parseTime(created); err != nil {
		return domain.Goal{}, fmt.Errorf("created_at: %w", err)
	}
	if g.ArchivedAt, err = parseNullTime(archivedAt); err != nil {
		return domain.Goal{}, fmt.Errorf("archived_at: %w", err)
	}
	return g, nil
}

// Create legt ein Ziel an.
func (r *GoalRepository) Create(ctx context.Context, g domain.Goal) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO goals (`+goalColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Title, g.Why, g.ImagePath, string(g.Kind), g.ProjectID, g.HabitID, g.Target, g.StartDate, g.DueDate,
		boolInt(g.Archived), formatTime(g.CreatedAt), g.ParentID, nullTime(g.ArchivedAt))
	if err != nil {
		return fmt.Errorf("repository: Ziel anlegen: %w", mapFK(err))
	}
	return nil
}

// Get liefert ein Ziel oder domain.ErrNotFound.
func (r *GoalRepository) Get(ctx context.Context, id string) (domain.Goal, error) {
	g, err := scanGoal(r.db.QueryRowContext(ctx, `SELECT `+goalColumns+` FROM goals WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Goal{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Goal{}, fmt.Errorf("repository: Ziel lesen: %w", err)
	}
	return g, nil
}

// List liefert alle Ziele, älteste zuerst.
func (r *GoalRepository) List(ctx context.Context) ([]domain.Goal, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+goalColumns+` FROM goals ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Ziele lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Goal{}
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Ziel lesen: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Update überschreibt die veränderlichen Felder eines Ziels.
func (r *GoalRepository) Update(ctx context.Context, g domain.Goal) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE goals SET title = ?, why = ?, image_path = ?, kind = ?, project_id = ?, habit_id = ?, target = ?,
			start_date = ?, due_date = ?, archived = ?, parent_id = ?, archived_at = ? WHERE id = ?`,
		g.Title, g.Why, g.ImagePath, string(g.Kind), g.ProjectID, g.HabitID, g.Target, g.StartDate, g.DueDate,
		boolInt(g.Archived), g.ParentID, nullTime(g.ArchivedAt), g.ID)
	if err != nil {
		return fmt.Errorf("repository: Ziel ändern: %w", mapFK(err))
	}
	return requireAffected(res)
}

// Delete löscht ein Ziel endgültig.
func (r *GoalRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM goals WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Ziel löschen: %w", err)
	}
	return requireAffected(res)
}
