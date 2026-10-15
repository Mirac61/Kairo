package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultSessionTTL = 30 * 24 * time.Hour

const minPasswordLen = 8

// dummyHash ist ein Stand-in-Hash für Logins mit unbekannter E-Mail,
// damit die Antwortzeit nicht verrät, ob ein Konto existiert.
var dummyHash = sync.OnceValue(func() string {
	hash, _ := hashPassword("dummy")
	return hash
})

type userRepository interface {
	Create(ctx context.Context, user User) error
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	Count(ctx context.Context) (int, error)
}

// Service kapselt Registrierung, Login und Session-Verwaltung.
type Service struct {
	repo       userRepository
	sessions   SessionStore
	sessionTTL time.Duration
	hasher     *Hasher
}

// NewService erzeugt einen Service mit Standard-Parametern.
func NewService(repo userRepository, sessions SessionStore) *Service {
	hasher, err := NewHasher(defaultHashConcurrency())
	if err != nil {
		panic(fmt.Sprintf("auth: %v", err))
	}
	return NewServiceWithSessionTTL(repo, sessions, defaultSessionTTL, hasher)
}

// NewServiceWithSessionTTL erzeugt einen Service mit konfigurierbarer TTL.
func NewServiceWithSessionTTL(repo userRepository, sessions SessionStore, ttl time.Duration, hasher *Hasher) *Service {
	if sessions == nil {
		panic("auth: sessions store is nil")
	}
	if hasher == nil {
		panic("auth: hasher is nil")
	}
	return &Service{repo: repo, sessions: sessions, sessionTTL: ttl, hasher: hasher}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Register legt das erste Benutzerkonto an; danach ist die Registrierung zu.
// shortcut: Prüfen und Anlegen sind nicht atomar, zwei gleichzeitige Setups
// könnten beide durchkommen. Für ein Einzelnutzer-Setup im VPN egal.
func (s *Service) Register(ctx context.Context, email, password string) (User, error) {
	if len(password) < minPasswordLen {
		return User{}, ErrPasswordTooShort
	}
	if has, err := s.HasUsers(ctx); err != nil {
		return User{}, err
	} else if has {
		return User{}, ErrRegistrationClosed
	}
	email = normalizeEmail(email)
	hash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	user := User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Login prüft E-Mail/Passwort und erzeugt eine neue Session.
func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	user, err := s.repo.FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Gegen Dummy-Hash prüfen, damit die Zeit gleich bleibt.
			verifyErr := s.hasher.Verify(ctx, dummyHash(), password)
			if ctx.Err() != nil {
				return User{}, "", verifyErr
			}
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", fmt.Errorf("find user: %w", err)
	}

	if err := s.hasher.Verify(ctx, user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrMismatchedPassword) {
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", fmt.Errorf("verify password: %w", err)
	}

	_ = s.sessions.DeleteExpiredByUser(ctx, user.ID)

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return User{}, "", fmt.Errorf("new session token: %w", err)
	}
	now := time.Now().UTC()
	session := Session{
		TokenHash: tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return User{}, "", fmt.Errorf("create session: %w", err)
	}

	return user, token, nil
}

// Logout löscht eine Session.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, hashToken(token))
}

// Me liefert einen Benutzer anhand seiner ID.
func (s *Service) Me(ctx context.Context, id string) (User, error) {
	return s.repo.FindByID(ctx, id)
}

// HasUsers prüft, ob bereits mindestens ein Benutzer existiert.
func (s *Service) HasUsers(ctx context.Context) (bool, error) {
	n, err := s.repo.Count(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
