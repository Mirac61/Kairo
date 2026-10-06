package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAutoBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bdir := filepath.Join(dir, "backups")
	backups := func() []string {
		t.Helper()
		files, err := filepath.Glob(filepath.Join(bdir, "kairo-*.db"))
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.Local)

	// Kein Backup vorhanden: eins anlegen. Gleich danach: keins mehr.
	if err := AutoBackup(ctx, db, bdir, false, now); err != nil {
		t.Fatal(err)
	}
	if err := AutoBackup(ctx, db, bdir, false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := len(backups()); n != 1 {
		t.Fatalf("%d Backups, erwartet 1", n)
	}
	// Ausstehende Migration: immer sichern.
	if err := AutoBackup(ctx, db, bdir, true, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := len(backups()); n != 2 {
		t.Fatalf("%d Backups, erwartet 2", n)
	}

	// 2 + 20 alte Backups und ein neues: Es bleiben die neuesten 14.
	for i := range 20 {
		f := filepath.Join(bdir, fmt.Sprintf("kairo-202509%02d-120000.db", i+1))
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range backups() {
		old := now.Add(-48 * time.Hour)
		if err := os.Chtimes(f, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := AutoBackup(ctx, db, bdir, false, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	files := backups()
	if len(files) != keepBackups {
		t.Fatalf("%d Backups, erwartet %d", len(files), keepBackups)
	}
	if want := filepath.Join(bdir, "kairo-20250910-120000.db"); files[0] != want {
		t.Errorf("ältestes übriges Backup %s, erwartet %s", files[0], want)
	}
}

func TestOpenWithBackupSkipsNewDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path, bdir := filepath.Join(dir, "k.db"), filepath.Join(dir, "backups")
	db, err := OpenWithBackup(ctx, path, bdir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if files, _ := filepath.Glob(filepath.Join(bdir, "*.db")); len(files) != 0 {
		t.Fatalf("Backup einer neuen Datenbank: %v", files)
	}
	db, err = OpenWithBackup(ctx, path, bdir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if files, _ := filepath.Glob(filepath.Join(bdir, "*.db")); len(files) != 1 {
		t.Fatalf("Backups beim zweiten Start: %v", files)
	}
}
