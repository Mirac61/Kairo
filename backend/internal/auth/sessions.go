package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// Session ist eine serverseitige Sitzung.
type Session struct {
	TokenHash []byte
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionStore speichert und liest Sessions.
type SessionStore interface {
	Create(ctx context.Context, s Session) error
	Get(ctx context.Context, tokenHash []byte) (Session, error)
	Touch(ctx context.Context, tokenHash []byte, expiresAt time.Time) error
	Delete(ctx context.Context, tokenHash []byte) error
	DeleteExpiredByUser(ctx context.Context, userID string) error
}

// newSessionToken erzeugt ein neues Opaque-Token und dessen SHA-256-Hash.
func newSessionToken() (string, []byte, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", nil, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(token)
	hash := sha256.Sum256([]byte(encoded))
	return encoded, hash[:], nil
}

// hashToken liefert den SHA-256-Hash eines Tokens.
func hashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
