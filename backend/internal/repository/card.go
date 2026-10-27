package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// Review speichert den neuen Lernstand einer Karte und schreibt die Bewertung samt altem Stand (nil: neue Karte) in den Verlauf.
func (r *CardRepository) Review(ctx context.Context, key string, next domain.CardState, g domain.CardGrade, at time.Time, prev *domain.CardState) error {
	var prevJSON *string
	if prev != nil {
		b, err := json.Marshal(prev)
		if err != nil {
			return err
		}
		s := string(b)
		prevJSON = &s
	}
	return r.tx(ctx, func(tx *sql.Tx) error {
		if err := saveState(ctx, tx, key, next); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO card_log (key, grade, reviewed_at, prev) VALUES (?, ?, ?, ?)`, key, int(g), formatTime(at), prevJSON)
		return err
	})
}

// Undo nimmt die letzte Bewertung zurück: alter Lernstand wieder her (oder keiner bei einer neuen Karte), Eintrag aus dem Verlauf.
// Liefert den Key der Karte; domain.ErrNotFound, wenn der Verlauf leer ist.
func (r *CardRepository) Undo(ctx context.Context) (string, error) {
	var key string
	err := r.tx(ctx, func(tx *sql.Tx) error {
		var id int64
		var prev sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT id, key, prev FROM card_log ORDER BY id DESC LIMIT 1`).Scan(&id, &key, &prev)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if prev.Valid {
			var st domain.CardState
			if err := json.Unmarshal([]byte(prev.String), &st); err != nil {
				return err
			}
			err = saveState(ctx, tx, key, st)
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM card_reviews WHERE key = ?`, key)
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM card_log WHERE id = ?`, id)
		return err
	})
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		err = fmt.Errorf("repository: Bewertung zurücknehmen: %w", err)
	}
	return key, err
}

// History liefert die Bewertungen seit since, älteste zuerst.
func (r *CardRepository) History(ctx context.Context, since time.Time) ([]domain.CardLog, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, grade, reviewed_at, prev IS NULL FROM card_log WHERE reviewed_at >= ? ORDER BY id`, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("repository: Lernverlauf lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.CardLog{}
	for rows.Next() {
		var l domain.CardLog
		var at string
		if err := rows.Scan(&l.Key, &l.Grade, &at, &l.First); err != nil {
			return nil, err
		}
		if l.At, err = parseTime(at); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func saveState(ctx context.Context, q dbtx, key string, c domain.CardState) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO card_reviews (key, due, interval_days, ease, reps, reviewed_at) VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT (key) DO UPDATE SET due = excluded.due, interval_days = excluded.interval_days, ease = excluded.ease,
			reps = excluded.reps, reviewed_at = excluded.reviewed_at`,
		key, c.Due, c.Interval, c.Ease, c.Reps)
	if err != nil {
		return fmt.Errorf("repository: Lernstand speichern: %w", err)
	}
	return nil
}

func (r *CardRepository) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: Transaktion starten: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // nach Commit wirkungslos
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// List liefert die Karten, die direkt in Kairo angelegt wurden, älteste zuerst.
func (r *CardRepository) List(ctx context.Context) ([]domain.Card, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, deck, question, answer, created_at FROM cards ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("repository: Karten lesen: %w", err)
	}
	defer rows.Close()
	out := []domain.Card{}
	for rows.Next() {
		var c domain.Card
		var created string
		if err := rows.Scan(&c.ID, &c.Deck, &c.Question, &c.Answer, &created); err != nil {
			return nil, fmt.Errorf("repository: Karte lesen: %w", err)
		}
		if c.CreatedAt, err = parseTime(created); err != nil {
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

// DeckLimits liefert die gewählten Limits für neue Karten pro Tag je Fach.
func (r *CardRepository) DeckLimits(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, new_per_day FROM card_decks WHERE new_per_day IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("repository: Fächer lesen: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var n string
		var l int
		if err := rows.Scan(&n, &l); err != nil {
			return nil, err
		}
		out[n] = l
	}
	return out, rows.Err()
}

// SetDeckLimit speichert das Limit für neue Karten pro Tag eines Fachs.
func (r *CardRepository) SetDeckLimit(ctx context.Context, name string, n int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO card_decks (name, color, new_per_day) VALUES (?, '', ?) ON CONFLICT (name) DO UPDATE SET new_per_day = excluded.new_per_day`, name, n)
	if err != nil {
		return fmt.Errorf("repository: Fach speichern: %w", err)
	}
	return nil
}

// RenameDeck benennt ein Fach um (die direkt angelegten Karten und die Farbe). Gibt es das Ziel schon, werden beide zusammengelegt.
func (r *CardRepository) RenameDeck(ctx context.Context, from, to string) error {
	err := r.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE cards SET deck = ? WHERE deck = ?`, to, from); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE OR IGNORE card_decks SET name = ? WHERE name = ?`, to, from); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM card_decks WHERE name = ?`, from)
		return err
	})
	if err != nil {
		return fmt.Errorf("repository: Fach umbenennen: %w", err)
	}
	return nil
}

// DeleteDeck löscht ein Fach mit seinen direkt angelegten Karten.
func (r *CardRepository) DeleteDeck(ctx context.Context, name string) error {
	err := r.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE deck = ?`, name); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM card_decks WHERE name = ?`, name)
		return err
	})
	if err != nil {
		return fmt.Errorf("repository: Fach löschen: %w", err)
	}
	return nil
}
