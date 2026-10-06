package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// keepBackups ist die Zahl der Backups, die AutoBackup behält.
const keepBackups = 14

// AutoBackup sichert die Datenbank nach dir/kairo-YYYYMMDD-HHMMSS.db, wenn eine
// Migration aussteht oder das neueste Backup dort älter als 24 h ist (oder es
// keins gibt). Danach bleiben nur die neuesten keepBackups Backups übrig.
func AutoBackup(ctx context.Context, db *sql.DB, dir string, pending bool, now time.Time) error {
	old, err := filepath.Glob(filepath.Join(dir, "kairo-*.db")) // sortiert, also nach Zeit
	if err != nil {
		return err
	}
	if !pending && len(old) > 0 {
		if fi, err := os.Stat(old[len(old)-1]); err == nil && now.Sub(fi.ModTime()) < 24*time.Hour {
			return nil
		}
	}
	dest := filepath.Join(dir, "kairo-"+now.Format("20060102-150405")+".db")
	if err := Backup(ctx, db, dest); err != nil {
		return err
	}
	all := append(old, dest)
	for _, f := range all[:max(0, len(all)-keepBackups)] {
		if err := os.Remove(f); err != nil {
			return fmt.Errorf("repository: altes Backup löschen: %w", err)
		}
	}
	return nil
}

// Backup schreibt einen konsistenten Snapshot der Datenbank nach dest. Das geht
// auch, während der Server läuft. Eine vorhandene Datei wird nie überschrieben.
func Backup(ctx context.Context, db *sql.DB, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("repository: %s existiert bereits", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("repository: Verzeichnis anlegen: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("repository: Backup: %w", err)
	}
	return os.Chmod(dest, 0o600)
}
