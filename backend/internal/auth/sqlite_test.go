package auth

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/repository"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	db, err := repository.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close(); os.RemoveAll(dir) })
	return db
}

func TestSQLiteRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	u := User{ID: "u1", Email: "A@Example.COM", PasswordHash: "hash", CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, u); err != ErrEmailTaken {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	found, err := repo.FindByEmail(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if found.ID != u.ID {
		t.Fatalf("expected %s, got %s", u.ID, found.ID)
	}

	byID, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if byID.Email != u.Email {
		t.Fatalf("expected %s, got %s", u.Email, byID.Email)
	}

	if _, err := repo.FindByID(ctx, "missing"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected count 1, got %d", n)
	}
}

func TestSQLiteSessionStore(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLiteRepository(db)
	store := NewSQLiteSessionStore(db)
	ctx := context.Background()

	if err := repo.Create(ctx, User{ID: "u1", Email: "s@example.com", PasswordHash: "hash", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	token := "opaque-token"
	hash := hashToken(token)

	sess := Session{
		TokenHash: hash,
		UserID:    "u1",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := store.Create(ctx, sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, hash)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != sess.UserID {
		t.Fatalf("expected user %s, got %s", sess.UserID, got.UserID)
	}

	if err := store.Touch(ctx, hash, time.Now().UTC().Add(2*time.Hour)); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	if err := store.Delete(ctx, hash); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, hash); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}
