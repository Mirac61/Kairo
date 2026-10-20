package repository

import (
	"context"
	"database/sql"
	"fmt"

	"kairo/internal/domain"
)

// CardRepository speichert den Lernstand der Karteikarten.
type CardRepository struct{ db *sql.DB }

// NewCardRepository erzeugt ein CardRepository.
func NewCardRepository(db *sql.DB) *CardRepository { return &CardRepository{db: db} }

// States liefert den Lernstand aller schon gelernten Karten, nach Key.
func (r *CardRepository) States(ctx context.Context) (map[string]domain.CardState, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, due, interval_days, ease, reps FROM card_reviews`)
	if err != nil {
		return nil, fmt.Errorf("repository: Lernstand lesen: %w", err)
	}
	defer rows.Close()
	out := map[string]domain.CardState{}
	for rows.Next() {
		var key string
		var c domain.CardState
		if err := rows.Scan(&key, &c.Due, &c.Interval, &c.Ease, &c.Reps); err != nil {
			return nil, fmt.Errorf("repository: Lernstand lesen: %w", err)
		}
		out[key] = c
	}
	return out, rows.Err()
}

// Save legt den Lernstand einer Karte an oder überschreibt ihn.
func (r *CardRepository) Save(ctx context.Context, key string, c domain.CardState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO card_reviews (key, due, interval_days, ease, reps, reviewed_at) VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT (key) DO UPDATE SET due = excluded.due, interval_days = excluded.interval_days, ease = excluded.ease,
			reps = excluded.reps, reviewed_at = excluded.reviewed_at`,
		key, c.Due, c.Interval, c.Ease, c.Reps)
	if err != nil {
		return fmt.Errorf("repository: Lernstand speichern: %w", err)
	}
	return nil
}
