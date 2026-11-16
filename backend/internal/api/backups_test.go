package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestBackups(t *testing.T) {
	dir := t.TempDir()
	create := func(_ context.Context, dest string) error {
		if _, err := os.Stat(dest); err == nil {
			return os.ErrExist
		}
		return os.WriteFile(dest, []byte("db"), 0o600)
	}
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Backups: &Backups{Dir: filepath.Join(dir, "backups"), Create: create},
	})
	list := func() []backupDTO {
		t.Helper()
		rec := call(h, "GET", "/api/backups", "")
		var got []backupDTO
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
			t.Fatalf("GET = %d %s", rec.Code, rec.Body)
		}
		return got
	}
	if got := list(); got == nil || len(got) != 0 {
		t.Fatalf("leerer Ordner: %v", got)
	}
	// Ältere Datei vorhanden: die neue steht in der Liste vorn.
	os.MkdirAll(filepath.Join(dir, "backups"), 0o700)
	os.WriteFile(filepath.Join(dir, "backups", "kairo-20000101-000000.db"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir, "backups", "fremd.txt"), []byte("x"), 0o600) // gehört nicht dazu

	rec := call(h, "POST", "/api/backups", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	got := list()
	if len(got) != 2 || got[1].Name != "kairo-20000101-000000.db" || got[0].Size != 2 {
		t.Fatalf("Liste = %+v", got)
	}
}
