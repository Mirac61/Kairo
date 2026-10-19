package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// PrefsRepository speichert die Einstellungen der Oberfläche als ein JSON-Objekt.
type PrefsRepository struct{ db *sql.DB }

// NewPrefsRepository erzeugt das Repository.
func NewPrefsRepository(db *sql.DB) *PrefsRepository { return &PrefsRepository{db: db} }

// Get liefert das gespeicherte Objekt, ohne Eintrag {}.
func (r *PrefsRepository) Get(ctx context.Context) (json.RawMessage, error) {
	var data string
	err := r.db.QueryRowContext(ctx, `SELECT data FROM ui_prefs WHERE id = 1`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return json.RawMessage(`{}`), nil
	}
	if err != nil {
		return nil, fmt.Errorf("repository: Einstellungen lesen: %w", err)
	}
	return json.RawMessage(data), nil
}

// Put ersetzt das Objekt.
func (r *PrefsRepository) Put(ctx context.Context, data json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO ui_prefs (id, data) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET data = excluded.data`, string(data))
	if err != nil {
		return fmt.Errorf("repository: Einstellungen speichern: %w", err)
	}
	return nil
}
