package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOpenMigratesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "k.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM schema_migrations`); n < 1 {
		t.Fatalf("keine Migration angewendet: %d", n)
	}
	var fk int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, err %v", fk, err)
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %s, err %v", mode, err)
	}
	before := count(t, db, `SELECT COUNT(*) FROM schema_migrations`)
	db.Close()

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if after := count(t, db, `SELECT COUNT(*) FROM schema_migrations`); after != before {
		t.Errorf("zweiter Lauf änderte Migrationen: %d -> %d", before, after)
	}
}

func TestMigrateBadMigrationRollsBack(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fsys := fstest.MapFS{
		"0100_ok.sql":    {Data: []byte(`CREATE TABLE a (id INTEGER);`)},
		"0101_bad.sql":   {Data: []byte(`CREATE TABLE b (id INTEGER); INSERT INTO missing VALUES (1);`)},
		"0102_never.sql": {Data: []byte(`CREATE TABLE c (id INTEGER);`)},
	}
	if err := Migrate(ctx, db, fsys); err == nil {
		t.Fatal("Fehler erwartet")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE name='a'`); n != 1 {
		t.Error("0100 sollte angewendet sein")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE name IN ('b','c')`); n != 0 {
		t.Error("b/c dürfen nicht existieren (Rollback bzw. Abbruch)")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM schema_migrations WHERE version IN ('0101_bad','0102_never')`); n != 0 {
		t.Error("fehlerhafte Migration darf nicht vermerkt sein")
	}
}
