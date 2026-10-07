package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kairo/internal/domain"
)

// HabitRepository speichert Habits und ihre Completions in SQLite.
type HabitRepository struct{ db *sql.DB }

// NewHabitRepository erzeugt ein HabitRepository.
func NewHabitRepository(db *sql.DB) *HabitRepository { return &HabitRepository{db: db} }

const habitColumns = `id, name, description, frequency_type, frequency_config, target_value, unit,
	preferred_time, start_date, end_date, active, created_at`

const completionColumns = `id, habit_id, date, value, completed_at, note`

func nullInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

// scanHabit liest habitColumns; extra sind weitere Zielvariablen für Spalten dahinter.
func scanHabit(s scanner, extra ...any) (domain.Habit, error) {
	var (
		h                     domain.Habit
		target                sql.NullInt64
		preferred, end        sql.NullString
		freq, config, created string
		active                int
	)
	if err := s.Scan(append([]any{&h.ID, &h.Name, &h.Description, &freq, &config, &target, &h.Unit,
		&preferred, &h.StartDate, &end, &active, &created}, extra...)...); err != nil {
		return domain.Habit{}, err
	}
	h.FrequencyType = domain.FrequencyType(freq)
	if err := json.Unmarshal([]byte(config), &h.FrequencyConfig); err != nil {
		return domain.Habit{}, fmt.Errorf("frequency_config: %w", err)
	}
	if target.Valid {
		v := int(target.Int64)
		h.TargetValue = &v
	}
	h.PreferredTime, h.EndDate, h.Active = nullStr(preferred), nullStr(end), active == 1
	var err error
	if h.CreatedAt, err = parseTime(created); err != nil {
		return domain.Habit{}, fmt.Errorf("created_at: %w", err)
	}
	return h, nil
}

func scanCompletion(s scanner) (domain.HabitCompletion, error) {
	var (
		c         domain.HabitCompletion
		value     sql.NullInt64
		completed string
	)
	if err := s.Scan(&c.ID, &c.HabitID, &c.Date, &value, &completed, &c.Note); err != nil {
		return domain.HabitCompletion{}, err
	}
	if value.Valid {
		v := int(value.Int64)
		c.Value = &v
	}
	var err error
	if c.CompletedAt, err = parseTime(completed); err != nil {
		return domain.HabitCompletion{}, fmt.Errorf("completed_at: %w", err)
	}
	return c, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Create legt ein Habit an.
func (r *HabitRepository) Create(ctx context.Context, h domain.Habit) error {
	cfg, err := json.Marshal(h.FrequencyConfig)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO habits (`+habitColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.Description, string(h.FrequencyType), string(cfg), nullInt(h.TargetValue), h.Unit,
		h.PreferredTime, h.StartDate, h.EndDate, boolInt(h.Active), formatTime(h.CreatedAt))
	if err != nil {
		return fmt.Errorf("repository: Habit anlegen: %w", err)
	}
	return nil
}

// Get liefert ein Habit oder domain.ErrNotFound.
func (r *HabitRepository) Get(ctx context.Context, id string) (domain.Habit, error) {
	h, err := scanHabit(r.db.QueryRowContext(ctx, `SELECT `+habitColumns+` FROM habits WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Habit{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Habit{}, fmt.Errorf("repository: Habit lesen: %w", err)
	}
	return h, nil
}

// List liefert alle Habits, nach Name sortiert.
func (r *HabitRepository) List(ctx context.Context) ([]domain.Habit, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+habitColumns+` FROM habits WHERE deleted_at IS NULL ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Habits lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Habit{}
	for rows.Next() {
		h, err := scanHabit(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Habit lesen: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Update überschreibt die veränderlichen Felder eines Habits.
func (r *HabitRepository) Update(ctx context.Context, h domain.Habit) error {
	cfg, err := json.Marshal(h.FrequencyConfig)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE habits SET name = ?, description = ?, frequency_type = ?, frequency_config = ?, target_value = ?,
			unit = ?, preferred_time = ?, start_date = ?, end_date = ?, active = ? WHERE id = ? AND deleted_at IS NULL`,
		h.Name, h.Description, string(h.FrequencyType), string(cfg), nullInt(h.TargetValue),
		h.Unit, h.PreferredTime, h.StartDate, h.EndDate, boolInt(h.Active), h.ID)
	if err != nil {
		return fmt.Errorf("repository: Habit ändern: %w", err)
	}
	return requireAffected(res)
}

// Delete löscht ein Habit (auch aus dem Papierkorb) endgültig samt Completions.
func (r *HabitRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM habits WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Habit löschen: %w", err)
	}
	return requireAffected(res)
}

// CreateCompletion speichert eine Completion; eine zweite am selben Tag
// ergibt domain.ErrConflict.
func (r *HabitRepository) CreateCompletion(ctx context.Context, c domain.HabitCompletion) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO habit_completions (`+completionColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.HabitID, c.Date, nullInt(c.Value), formatTime(c.CompletedAt), c.Note)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: für diesen Tag ist das Habit schon abgehakt", domain.ErrConflict)
		}
		return fmt.Errorf("repository: Completion anlegen: %w", err)
	}
	return nil
}

// DeleteCompletion löscht die Completion eines Habits an einem Tag.
func (r *HabitRepository) DeleteCompletion(ctx context.Context, habitID, date string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM habit_completions WHERE habit_id = ? AND date = ?`, habitID, date)
	if err != nil {
		return fmt.Errorf("repository: Completion löschen: %w", err)
	}
	return requireAffected(res)
}

// ListCompletions liefert Completions mit from <= date <= to (beide
// inklusive, leer = unbegrenzt) nach Tag sortiert. habitID leer = alle Habits.
func (r *HabitRepository) ListCompletions(ctx context.Context, habitID, from, to string) ([]domain.HabitCompletion, error) {
	var (
		where []string
		args  []any
	)
	if habitID != "" {
		where, args = append(where, "habit_id = ?"), append(args, habitID)
	}
	if from != "" {
		where, args = append(where, "date >= ?"), append(args, from)
	}
	if to != "" {
		where, args = append(where, "date <= ?"), append(args, to)
	}
	q := `SELECT ` + completionColumns + ` FROM habit_completions`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY date, habit_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: Completions lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.HabitCompletion{}
	for rows.Next() {
		c, err := scanCompletion(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: Completion lesen: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
