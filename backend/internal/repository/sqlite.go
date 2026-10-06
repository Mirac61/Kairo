// Package repository kapselt den SQLite-Zugriff und die Migrationen.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kairo/migrations"

	_ "modernc.org/sqlite" // pure-Go-Treiber
)

// Open öffnet (und erstellt) die Datenbank und wendet alle Migrationen an.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("repository: Verzeichnis anlegen: %w", err)
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	// Transaktionen nehmen die Schreibsperre sofort. So warten zwei Timer-Wechsel
	// aufeinander (busy_timeout), statt beim Lock-Upgrade mit SQLITE_BUSY zu scheitern.
	q.Add("_txlock", "immediate")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("repository: öffnen: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("repository: ping: %w", err)
	}
	if err := Migrate(ctx, db, migrations.FS); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate wendet noch nicht angewendete *.sql-Dateien aus fsys in
// Dateinamen-Reihenfolge an, je Datei in einer eigenen Transaktion.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("repository: schema_migrations anlegen: %w", err)
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
			return fmt.Errorf("repository: Migrationsstand lesen: %w", err)
		}
		if n > 0 {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		if err := applyOne(ctx, db, version, string(body)); err != nil {
			return fmt.Errorf("repository: Migration %s: %w", version, err)
		}
	}
	return nil
}

func applyOne(ctx context.Context, db *sql.DB, version, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // nach Commit wirkungslos
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}
