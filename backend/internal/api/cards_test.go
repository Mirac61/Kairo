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
	if len(decks) != 2 || decks[0] != (deckDTO{"Mathe", "green", 1, 1, 1, 0, service.NewPerDay}) || decks[1].Name != "uni" || decks[1].Total != 2 || decks[1].Due != 1 {
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
	// Nochmal: Karte ist wieder heute dran und zählt als „lernen“; Rückgängig stellt den alten Stand her.
	if rec := call(h, "POST", "/api/cards/"+cs[0].Key+"/review", `{"grade":0}`); rec.Code != 200 {
		t.Fatalf("nochmal = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/cards/decks", "").Body.Bytes(), &decks)
	if decks[1].Name != "uni" || decks[1].Due != 2 || decks[1].Learn != 1 {
		t.Errorf("decks nach nochmal = %+v", decks)
	}
	var log []cardLogDTO
	_ = json.Unmarshal(call(h, "GET", "/api/cards/log", "").Body.Bytes(), &log)
	if len(log) != 2 || log[0].Grade != 2 || log[1].Grade != 0 || log[1].Key != cs[0].Key {
		t.Errorf("log = %+v", log)
	}
	if rec := call(h, "POST", "/api/cards/undo", ""); rec.Code != 200 || rec.Body.String() != `{"key":"`+cs[0].Key+`"}`+"\n" {
		t.Fatalf("undo = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/cards", "").Body.Bytes(), &all)
	if all[0].Learning || all[0].Due || *all[0].DueDate != "2026-10-22" || all[0].Intervals != [4]int{0, 1, 3, 3} {
		t.Errorf("nach undo = %+v", all[0])
	}
	call(h, "POST", "/api/cards/undo", "")
	if rec := call(h, "POST", "/api/cards/undo", ""); rec.Code != 404 {
		t.Errorf("undo ohne Verlauf = %d", rec.Code)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/cards", "").Body.Bytes(), &all)
	if !all[0].New {
		t.Errorf("nach zweitem undo wieder neu: %+v", all[0])
	}

	// Fach umbenennen (zusammenlegen) und löschen; Notizkarten bleiben im Ordner-Fach.
	call(h, "POST", "/api/cards", `{"deck":"Bio","question":"Zelle","answer":"kleinste Einheit"}`)
	call(h, "PUT", "/api/cards/decks/Bio", `{"color":"pink"}`)
	if rec := call(h, "PATCH", "/api/cards/decks/Bio", `{"name":"uni"}`); rec.Code != 204 {
		t.Fatalf("rename = %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/cards/decks", "").Body.Bytes(), &decks)
	if len(decks) != 2 || decks[1].Name != "uni" || decks[1].Total != 3 || decks[1].Color != "pink" {
		t.Errorf("decks nach rename = %+v", decks)
	}
	if rec := call(h, "PATCH", "/api/cards/decks/uni", `{"name":" "}`); rec.Code != 400 {
		t.Errorf("rename leer = %d", rec.Code)
	}
	if rec := call(h, "DELETE", "/api/cards/decks/uni", ""); rec.Code != 204 {
		t.Errorf("delete deck = %d", rec.Code)
	}
	_ = json.Unmarshal(call(h, "GET", "/api/cards/decks", "").Body.Bytes(), &decks)
	if len(decks) != 2 || decks[1].Name != "uni" || decks[1].Total != 2 {
		t.Errorf("decks nach delete = %+v", decks)
	}

	// Tageslimit: von 21 neuen Karten sind 20 fällig; eine heute begonnene zählt mit.
	for i := range service.NewPerDay + 1 {
		call(h, "POST", "/api/cards", `{"deck":"Vokabeln","question":"Wort `+string(rune('A'+i))+`","answer":"x"}`)
	}
	vok := func() deckDTO {
		_ = json.Unmarshal(call(h, "GET", "/api/cards/decks", "").Body.Bytes(), &decks)
		return decks[2]
	}
	if d := vok(); d.Name != "Vokabeln" || d.Total != 21 || d.Due != 20 || d.NewDue != 20 {
		t.Errorf("Tageslimit = %+v", d)
	}
	var first cardDTO
	_ = json.Unmarshal(call(h, "GET", "/api/cards/due", "").Body.Bytes(), &cs)
	for _, c := range cs {
		if c.Deck == "Vokabeln" {
			first = c
			break
		}
	}
	call(h, "POST", "/api/cards/"+first.Key+"/review", `{"grade":2}`)
	if d := vok(); d.Due != 19 {
		t.Errorf("Tageslimit nach einer neuen = %+v", d)
	}
	if rec := call(h, "PUT", "/api/cards/decks/Vokabeln", `{"new_per_day":5}`); rec.Code != http.StatusNoContent {
		t.Fatalf("Limit setzen = %d %s", rec.Code, rec.Body)
	}
	if d := vok(); d.Due != 4 || d.Limit != 5 || d.Color == "" {
		t.Errorf("eigenes Tageslimit = %+v", d)
	}
	if rec := call(h, "PUT", "/api/cards/decks/Vokabeln", `{"new_per_day":-1}`); rec.Code != http.StatusBadRequest {
		t.Errorf("negatives Limit = %d", rec.Code)
	}

	for _, body := range []string{`{}`, `{"grade":4}`} {
		if rec := call(h, "POST", "/api/cards/x/review", body); rec.Code != 400 {
			t.Errorf("review %s = %d", body, rec.Code)
		}
	}
}
