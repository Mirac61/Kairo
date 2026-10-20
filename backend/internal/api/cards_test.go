package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/notes"
	"kairo/internal/repository"
	"kairo/internal/service"
)

func TestCardsFlow(t *testing.T) {
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "uni"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "uni", "bio.md"), []byte("- ATP :: Energieträger\n- DNA :: Erbgut\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "kopie.md"), []byte("ATP :: Energieträger\n"), 0o644)
	ns, err := notes.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ns.Close() })
	now := func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Cards: service.NewCardService(ns, repository.NewCardRepository(db), time.UTC, now),
	})

	due := func() (out []dueCardDTO) {
		_ = json.Unmarshal(call(h, "GET", "/api/cards/due", "").Body.Bytes(), &out)
		return out
	}
	cs := due()
	if len(cs) != 2 || !cs[0].New || cs[0].Question != "ATP" || cs[0].NotePath != "uni/bio.md" {
		t.Fatalf("due = %+v", cs)
	}
	rec := call(h, "POST", "/api/cards/"+cs[0].Key+"/review", `{"grade":2}`)
	if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) || rec.Body.String() != `{"due":"2026-10-22","interval_days":1}`+"\n" {
		t.Fatalf("review = %d %s", rec.Code, rec.Body)
	}
	if cs := due(); len(cs) != 1 || cs[0].Question != "DNA" {
		t.Errorf("due nach review = %+v", cs)
	}
	for _, body := range []string{`{}`, `{"grade":4}`} {
		if rec := call(h, "POST", "/api/cards/x/review", body); rec.Code != 400 {
			t.Errorf("review %s = %d", body, rec.Code)
		}
	}
}
