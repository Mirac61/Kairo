package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"kairo/internal/domain"
)

// ProjectRepository speichert Projekte in SQLite.
type ProjectRepository struct{ db *sql.DB }

// NewProjectRepository erzeugt ein ProjectRepository.
func NewProjectRepository(db *sql.DB) *ProjectRepository { return &ProjectRepository{db: db} }

const projectColumns = `id, name, description, local_path, status, created_at, updated_at`

// formatTime speichert Zeitpunkte in UTC als RFC 3339.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

type scanner interface{ Scan(dest ...any) error }

func scanProject(s scanner) (domain.Project, error) {
	var (
		p                    domain.Project
		path                 sql.NullString
		status, created, upd string
	)
	if err := s.Scan(&p.ID, &p.Name, &p.Description, &path, &status, &created, &upd); err != nil {
		return domain.Project{}, err
	}
	if path.Valid {
		p.LocalPath = &path.String
	}
	p.Status = domain.ProjectStatus(status)
	var err error
	if p.CreatedAt, err = parseTime(created); err != nil {
		return domain.Project{}, fmt.Errorf("created_at: %w", err)
	}
	if p.UpdatedAt, err = parseTime(upd); err != nil {
		return domain.Project{}, fmt.Errorf("updated_at: %w", err)
	}
	return p, nil
}

// Create legt ein Projekt an.
func (r *ProjectRepository) Create(ctx context.Context, p domain.Project) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (`+projectColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.LocalPath, string(p.Status), formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("repository: Projekt anlegen: %w", err)
	}
	return nil
}

// Get liefert ein Projekt oder domain.ErrNotFound.
func (r *ProjectRepository) Get(ctx context.Context, id string) (domain.Project, error) {
	p, err := scanProject(r.db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("repository: Projekt lesen: %w", err)
	}
	return p, nil
}

// List liefert alle Projekte, nach Name sortiert.
func (r *ProjectRepository) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+projectColumns+` FROM projects ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Projekte lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Projekt lesen: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update überschreibt die veränderlichen Felder eines Projekts.
func (r *ProjectRepository) Update(ctx context.Context, p domain.Project) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, local_path = ?, status = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.LocalPath, string(p.Status), formatTime(p.UpdatedAt), p.ID)
	if err != nil {
		return fmt.Errorf("repository: Projekt ändern: %w", err)
	}
	return requireAffected(res)
}

// Delete löscht ein Projekt.
func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Projekt löschen: %w", err)
	}
	return requireAffected(res)
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
