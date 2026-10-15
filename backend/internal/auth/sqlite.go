package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// SQLiteRepository speichert Benutzer in SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository erzeugt ein neues SQLite-Repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Create legt einen neuen Benutzer an.
func (r *SQLiteRepository) Create(ctx context.Context, user User) error {
	const query = `INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Email, user.PasswordHash, user.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		return err
	}
	return nil
}

// FindByEmail sucht einen Benutzer anhand der E-Mail-Adresse.
func (r *SQLiteRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return r.findBy(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE lower(email) = ?`, strings.ToLower(email))
}

// FindByID sucht einen Benutzer anhand seiner ID.
func (r *SQLiteRepository) FindByID(ctx context.Context, id string) (User, error) {
	return r.findBy(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE id = ?`, id)
}

func (r *SQLiteRepository) findBy(ctx context.Context, query string, arg any) (User, error) {
	var u User
	var createdAt string
	err := r.db.QueryRowContext(ctx, query, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// Count zählt alle Benutzer.
func (r *SQLiteRepository) Count(ctx context.Context) (int, error) {
	var n int
	const query = `SELECT COUNT(*) FROM users`
	err := r.db.QueryRowContext(ctx, query).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// SQLiteSessionStore speichert Sessions in SQLite.
type SQLiteSessionStore struct {
	db *sql.DB
}

// NewSQLiteSessionStore erzeugt einen neuen SQLite-Session-Store.
func NewSQLiteSessionStore(db *sql.DB) *SQLiteSessionStore {
	return &SQLiteSessionStore{db: db}
}

// Create legt eine neue Session an.
func (s *SQLiteSessionStore) Create(ctx context.Context, session Session) error {
	const query = `INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, hex.EncodeToString(session.TokenHash), session.UserID, session.CreatedAt.Format(time.RFC3339Nano), session.ExpiresAt.Format(time.RFC3339Nano))
	return err
}

// Get liest eine nicht abgelaufene Session anhand des Token-Hashes.
func (s *SQLiteSessionStore) Get(ctx context.Context, tokenHash []byte) (Session, error) {
	const query = `SELECT user_id, created_at, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var createdAt, expiresAt string
	session := Session{TokenHash: tokenHash}
	err := s.db.QueryRowContext(ctx, query, hex.EncodeToString(tokenHash), now).Scan(&session.UserID, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var parseErr error
	session.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAt)
	if parseErr != nil {
		return Session{}, parseErr
	}
	session.ExpiresAt, parseErr = time.Parse(time.RFC3339Nano, expiresAt)
	if parseErr != nil {
		return Session{}, parseErr
	}
	return session, nil
}

// Touch verlängert eine Session.
func (s *SQLiteSessionStore) Touch(ctx context.Context, tokenHash []byte, expiresAt time.Time) error {
	const query = `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`
	_, err := s.db.ExecContext(ctx, query, expiresAt.Format(time.RFC3339Nano), hex.EncodeToString(tokenHash))
	return err
}

// Delete löscht eine Session.
func (s *SQLiteSessionStore) Delete(ctx context.Context, tokenHash []byte) error {
	const query = `DELETE FROM sessions WHERE token_hash = ?`
	_, err := s.db.ExecContext(ctx, query, hex.EncodeToString(tokenHash))
	return err
}

// DeleteExpiredByUser löscht abgelaufene Sessions eines Benutzers.
func (s *SQLiteSessionStore) DeleteExpiredByUser(ctx context.Context, userID string) error {
	const query = `DELETE FROM sessions WHERE user_id = ? AND expires_at <= ?`
	_, err := s.db.ExecContext(ctx, query, userID, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// isUniqueViolation prüft, ob ein SQLite-Fehler eine UNIQUE-Verletzung ist.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
