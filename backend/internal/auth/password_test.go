package auth

import (
	"context"
	"testing"
)

func TestHasherHashAndVerify(t *testing.T) {
	h, err := NewHasher(1)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	ctx := context.Background()
	hash, err := h.Hash(ctx, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if err := h.Verify(ctx, hash, "correct horse battery staple"); err != nil {
		t.Fatalf("Verify correct password: %v", err)
	}
	if err := h.Verify(ctx, hash, "wrong password"); !IsMismatched(err) {
		t.Fatalf("expected mismatched password error, got %v", err)
	}
}

func TestHasherInvalidHash(t *testing.T) {
	h, err := NewHasher(1)
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	if err := h.Verify(context.Background(), "not-a-hash", "password"); !IsInvalidHash(err) {
		t.Fatalf("expected invalid hash error, got %v", err)
	}
}

func TestHasherConcurrencyValidation(t *testing.T) {
	if _, err := NewHasher(0); err == nil {
		t.Fatal("expected error for concurrency 0")
	}
}
