package repository

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, name, status, created_at, updated_at) VALUES ('p','Kairo','ACTIVE','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "sub", "b.db")
	if err := Backup(ctx, db, dest); err != nil {
		t.Fatal(err)
	}
	cp, err := Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	var name string
	if err := cp.QueryRowContext(ctx, `SELECT name FROM projects WHERE id='p'`).Scan(&name); err != nil || name != "Kairo" {
		t.Errorf("Kopie: name=%q err=%v", name, err)
	}
	if err := Backup(ctx, db, dest); err == nil {
		t.Error("zweites Backup überschreibt vorhandene Datei")
	}
}
