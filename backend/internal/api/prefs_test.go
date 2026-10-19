package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"kairo/internal/repository"
)

func TestPrefs(t *testing.T) {
	db, err := repository.Open(context.Background(), t.TempDir()+"/k.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h := NewTestRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Prefs: repository.NewPrefsRepository(db)})

	if rec := call(h, "GET", "/api/prefs", ""); rec.Code != 200 || rec.Body.String() != "{}" {
		t.Fatalf("leer = %d %s", rec.Code, rec.Body)
	}
	if rec := call(h, "PUT", "/api/prefs", `{"theme":"light","calDays":7}`); rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(call(h, "GET", "/api/prefs", "").Body.Bytes(), &got)
	if got["theme"] != "light" || got["calDays"] != 7.0 {
		t.Fatalf("gelesen = %v", got)
	}
	for _, body := range []string{`[1]`, `null`, `"x"`, `{"x":"` + strings.Repeat("a", 17<<10) + `"}`} {
		if rec := call(h, "PUT", "/api/prefs", body); rec.Code != 400 {
			t.Errorf("%.20q = %d", body, rec.Code)
		}
	}
}
