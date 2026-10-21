package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kairo/internal/notes"
	"kairo/internal/service"
)

func TestJournalSummaryEndpoint(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Journal"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-10-21", "2026-10-22", "invalid"} {
		if err := os.WriteFile(filepath.Join(root, "Journal", date+".md"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := notes.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	now := time.Date(2026, 10, 21, 22, 30, 0, 0, time.UTC)
	svc := service.NewJournalService(store, func() time.Time { return now }, loc)
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Journal: svc})

	rec := call(h, "GET", "/api/journal/summary", "")
	var got journalSummaryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := journalSummaryDTO{Date: "2026-10-22", Total: 2, Streak: 2}
	if rec.Code != http.StatusOK || got != want {
		t.Errorf("GET summary = %d %+v, erwartet %+v", rec.Code, got, want)
	}
}

func TestJournalSummaryWithNoJournal(t *testing.T) {
	store, err := notes.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	svc := service.NewJournalService(store, func() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }, time.UTC)
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Journal: svc})

	rec := call(h, "GET", "/api/journal/summary", "")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"date":"2026-10-22","total":0,"streak":0}`+"\n" {
		t.Errorf("GET summary ohne Journal = %d %s", rec.Code, rec.Body)
	}
}
