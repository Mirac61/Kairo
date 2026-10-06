package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

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
