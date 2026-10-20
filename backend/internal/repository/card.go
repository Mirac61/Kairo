package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

// List liefert die Karten, die direkt in Kairo angelegt wurden, älteste zuerst.
func (r *CardRepository) List(ctx context.Context) ([]domain.Card, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, deck, question, answer FROM cards ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Karten lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Card{}
	for rows.Next() {
		var c domain.Card
		if err := rows.Scan(&c.ID, &c.Deck, &c.Question, &c.Answer); err != nil {
			return nil, fmt.Errorf("repository: Karte lesen: %w", err)
		}
		c.Key = domain.CardKey(c.Question)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create legt eine Karte an.
func (r *CardRepository) Create(ctx context.Context, c domain.Card) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO cards (id, deck, question, answer, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Deck, c.Question, c.Answer, formatTime(time.Now()))
	if err != nil {
		return fmt.Errorf("repository: Karte anlegen: %w", err)
	}
	return nil
}

// Update ändert Fach, Frage und Antwort einer Karte.
func (r *CardRepository) Update(ctx context.Context, c domain.Card) error {
	res, err := r.db.ExecContext(ctx, `UPDATE cards SET deck = ?, question = ?, answer = ? WHERE id = ?`, c.Deck, c.Question, c.Answer, c.ID)
	if err != nil {
		return fmt.Errorf("repository: Karte ändern: %w", err)
	}
	return requireAffected(res)
}

// Delete löscht eine Karte.
func (r *CardRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM cards WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("repository: Karte löschen: %w", err)
	}
	return requireAffected(res)
}

// DeckColors liefert die gewählten Farben der Fächer.
func (r *CardRepository) DeckColors(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, color FROM card_decks`)
	if err != nil {
		return nil, fmt.Errorf("repository: Fächer lesen: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var n, c string
		if err := rows.Scan(&n, &c); err != nil {
			return nil, err
		}
		out[n] = c
	}
	return out, rows.Err()
}

// SetDeckColor speichert die Farbe eines Fachs.
func (r *CardRepository) SetDeckColor(ctx context.Context, name, color string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO card_decks (name, color) VALUES (?, ?) ON CONFLICT (name) DO UPDATE SET color = excluded.color`, name, color)
	if err != nil {
		return fmt.Errorf("repository: Fach speichern: %w", err)
	}
	return nil
}
