package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memUserRepo struct {
	mu    sync.Mutex
	users map[string]User
	byID  map[string]User
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{users: make(map[string]User), byID: make(map[string]User)}
}

func (r *memUserRepo) Create(_ context.Context, u User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[u.Email]; ok {
		return ErrEmailTaken
	}
	r.users[u.Email] = u
	r.byID[u.ID] = u
	return nil
}

func (r *memUserRepo) FindByEmail(_ context.Context, email string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (r *memUserRepo) FindByID(_ context.Context, id string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (r *memUserRepo) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.users), nil
}

type memSessionStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func newMemSessionStore() *memSessionStore {
	return &memSessionStore{sessions: make(map[string]Session)}
}

func (s *memSessionStore) Create(_ context.Context, sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[string(sess.TokenHash)] = sess
	return nil
}

func (s *memSessionStore) Get(_ context.Context, tokenHash []byte) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[string(tokenHash)]
	if !ok || sess.ExpiresAt.Before(time.Now().UTC()) {
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}

func (s *memSessionStore) Touch(_ context.Context, tokenHash []byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[string(tokenHash)]
	if !ok {
		return nil
	}
	sess.ExpiresAt = expiresAt
	s.sessions[string(tokenHash)] = sess
	return nil
}

func (s *memSessionStore) Delete(_ context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, string(tokenHash))
	return nil
}

func (s *memSessionStore) DeleteExpiredByUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for k, v := range s.sessions {
		if v.UserID == userID && v.ExpiresAt.Before(now) {
			delete(s.sessions, k)
		}
	}
	return nil
}

func newTestService(t *testing.T) (*Service, *memUserRepo, *memSessionStore) {
	t.Helper()
	h, err := NewHasher(1)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	users := newMemUserRepo()
	sessions := newMemSessionStore()
	return NewServiceWithSessionTTL(users, sessions, time.Hour, h), users, sessions
}

func TestServiceRegisterAndLogin(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "user@example.com", "kurz"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
	if _, err := svc.Register(ctx, "user@example.com", "secret123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := svc.Register(ctx, "evil@example.com", "secret123"); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("zweites Konto: expected ErrRegistrationClosed, got %v", err)
	}

	_, token, err := svc.Login(ctx, "user@example.com", "secret123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected token")
	}

	if _, _, err := svc.Login(ctx, "user@example.com", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	if _, _, err := svc.Login(ctx, "other@example.com", "secret123"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
}

func TestServiceLogout(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, "u@example.com", "secret123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, token, err := svc.Login(ctx, "u@example.com", "secret123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestServiceHasUsers(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	has, err := svc.HasUsers(ctx)
	if err != nil {
		t.Fatalf("HasUsers: %v", err)
	}
	if has {
		t.Fatal("expected no users")
	}
	if _, err := svc.Register(ctx, "first@example.com", "secret123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	has, err = svc.HasUsers(ctx)
	if err != nil {
		t.Fatalf("HasUsers: %v", err)
	}
	if !has {
		t.Fatal("expected users")
	}
}
