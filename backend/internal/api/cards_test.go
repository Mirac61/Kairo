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

	due := func() (out []cardDTO) {
		_ = json.Unmarshal(call(h, "GET", "/api/cards/due", "").Body.Bytes(), &out)
		return out
	}
	cs := due()
	if len(cs) != 2 || !cs[0].New || cs[0].Question != "ATP" || *cs[0].NotePath != "uni/bio.md" || cs[0].Deck != "uni" {
		t.Fatalf("due = %+v", cs)
	}
	rec := call(h, "POST", "/api/cards/"+cs[0].Key+"/review", `{"grade":2}`)
	if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) || rec.Body.String() != `{"due":"2026-10-22","interval_days":1}`+"\n" {
		t.Fatalf("review = %d %s", rec.Code, rec.Body)
	}
	if cs := due(); len(cs) != 1 || cs[0].Question != "DNA" {
		t.Errorf("due nach review = %+v", cs)
	}
	// Direkt angelegte Karte in neuem Fach; Fächer zählen Notiz- und eigene Karten.
	rec = call(h, "POST", "/api/cards", `{"deck":"Mathe","question":"2+2","answer":"4"}`)
	var own cardDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &own)
	if rec.Code != 201 || own.ID == nil || own.Deck != "Mathe" || !own.Due {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	if rec := call(h, "PUT", "/api/cards/decks/Mathe", `{"color":"green"}`); rec.Code != 204 {
		t.Errorf("Farbe = %d %s", rec.Code, rec.Body)
	}
	var decks []deckDTO
	_ = json.Unmarshal(call(h, "GET", "/api/cards/decks", "").Body.Bytes(), &decks)
	if len(decks) != 2 || decks[0] != (deckDTO{"Mathe", "green", 1, 1, 1}) || decks[1].Name != "uni" || decks[1].Total != 2 || decks[1].Due != 1 {
		t.Errorf("decks = %+v", decks)
	}
	if rec := call(h, "PUT", "/api/cards/"+*own.ID, `{"deck":"Mathe","question":"2+3","answer":"5"}`); rec.Code != 200 {
		t.Errorf("PUT = %d %s", rec.Code, rec.Body)
	}
	if rec := call(h, "DELETE", "/api/cards/"+*own.ID, ""); rec.Code != 204 {
		t.Errorf("DELETE = %d", rec.Code)
	}
	var all []cardDTO
	_ = json.Unmarshal(call(h, "GET", "/api/cards", "").Body.Bytes(), &all)
	if len(all) != 2 || all[0].DueDate == nil || *all[0].DueDate != "2026-10-22" {
		t.Errorf("all = %+v", all)
	}
	for _, body := range []string{`{}`, `{"grade":4}`} {
		if rec := call(h, "POST", "/api/cards/x/review", body); rec.Code != 400 {
			t.Errorf("review %s = %d", body, rec.Code)
		}
	}
}
