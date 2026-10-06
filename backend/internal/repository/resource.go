package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"kairo/internal/domain"
)

// ResourceRepository speichert Ressourcen in SQLite.
type ResourceRepository struct{ db *sql.DB }

// NewResourceRepository erzeugt ein ResourceRepository.
func NewResourceRepository(db *sql.DB) *ResourceRepository { return &ResourceRepository{db: db} }

const resourceColumns = `id, task_id, project_id, type, target, label, created_at`

func scanResource(s scanner) (domain.Resource, error) {
	var (
		r             domain.Resource
		task, project sql.NullString
		typ, created  string
	)
	if err := s.Scan(&r.ID, &task, &project, &typ, &r.Target, &r.Label, &created); err != nil {
		return domain.Resource{}, err
	}
	r.TaskID, r.ProjectID, r.Type = nullStr(task), nullStr(project), domain.ResourceType(typ)
	var err error
	if r.CreatedAt, err = parseTime(created); err != nil {
		return domain.Resource{}, fmt.Errorf("created_at: %w", err)
	}
	return r, nil
}

// Create legt eine Ressource an.
func (r *ResourceRepository) Create(ctx context.Context, res domain.Resource) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO resources (`+resourceColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		res.ID, res.TaskID, res.ProjectID, string(res.Type), res.Target, res.Label, formatTime(res.CreatedAt))
	if err != nil {
		return fmt.Errorf("repository: Ressource anlegen: %w", mapFK(err))
	}
	return nil
}

// List liefert Ressourcen nach Filter in Anlegereihenfolge.
func (r *ResourceRepository) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, error) {
	var (
		where []string
		args  []any
	)
	if f.TaskID != "" {
		where, args = append(where, "task_id = ?"), append(args, f.TaskID)
	}
	if f.ProjectID != "" {
		where, args = append(where, "project_id = ?"), append(args, f.ProjectID)
	}
	if f.Type != "" {
		where, args = append(where, "type = ?"), append(args, string(f.Type))
	}
	q := `SELECT ` + resourceColumns + ` FROM resources`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY rowid`, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: Ressourcen lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Resource{}
	for rows.Next() {
		res, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Ressource lesen: %w", err)
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

// Delete löscht eine Ressource.
func (r *ResourceRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Ressource löschen: %w", err)
	}
	return requireAffected(res)
}
