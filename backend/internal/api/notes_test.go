package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kairo/internal/notes"
)

func TestNotes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "life-os") // fehlt noch, Open legt ihn an
	store, err := notes.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Notes: store})

	expect := func(rec interface{ Result() *http.Response }, want int) map[string]string {
		t.Helper()
		res := rec.Result()
		var body map[string]string
		_ = json.NewDecoder(res.Body).Decode(&body)
		if res.StatusCode != want {
			t.Fatalf("Status %d, erwartet %d: %v", res.StatusCode, want, body)
		}
		return body
	}

	expect(call(h, "POST", "/api/notes/uni", `{"dir":true}`), http.StatusCreated)
	expect(call(h, "POST", "/api/notes/uni", `{"dir":true}`), http.StatusConflict)
	created := expect(call(h, "POST", "/api/notes/uni/se.md", ``), http.StatusCreated)
	expect(call(h, "POST", "/api/notes/uni/se.md", ``), http.StatusConflict)

	// Speichern mit der Version vom Anlegen klappt, danach gilt die neue Version.
	saved := expect(call(h, "PUT", "/api/notes/uni/se.md", `{"content":"# SE","mtime":"`+created["mtime"]+`"}`), http.StatusOK)
	got := expect(call(h, "GET", "/api/notes/uni/se.md", ``), http.StatusOK)
	if got["content"] != "# SE" || got["mtime"] != saved["mtime"] {
		t.Fatalf("gelesen: %v", got)
	}

	// Extern geändert (wie in VSCodium): veraltete Version → 409, mit force → 200.
	file := filepath.Join(dir, "uni", "se.md")
	if err := os.WriteFile(file, []byte("extern"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(file, time.Now(), time.Now().Add(time.Second))
	expect(call(h, "PUT", "/api/notes/uni/se.md", `{"content":"kairo","mtime":"`+saved["mtime"]+`"}`), http.StatusConflict)
	if b, _ := os.ReadFile(file); string(b) != "extern" {
		t.Fatalf("trotz Konflikt überschrieben: %q", b)
	}
	expect(call(h, "PUT", "/api/notes/uni/se.md", `{"content":"kairo","mtime":"","force":true}`), http.StatusOK)

	// Alles außerhalb des Wurzelordners, Nicht-Markdown und Versteckte sind tabu.
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(outside, "geheim.md"), []byte("x"), 0o644)
	_ = os.Symlink(filepath.Join(outside, "geheim.md"), filepath.Join(dir, "link.md"))
	_ = os.Symlink(outside, filepath.Join(dir, "raus"))
	_ = os.WriteFile(filepath.Join(dir, "bild.png"), nil, 0o644)
	_ = os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	for _, p := range []string{"/api/notes/..%2Fx.md", "/api/notes/uni%2F..%2F..%2Fx.md", "/api/notes/link.md", "/api/notes/raus/geheim.md", "/api/notes/bild.png", "/api/notes/.git/x.md"} {
		if rec := call(h, "GET", p, ``); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: Status %d, erwartet 400", p, rec.Code)
		}
		if rec := call(h, "PUT", p, `{"content":"x","force":true}`); rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %s: Status %d, erwartet 400", p, rec.Code)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(outside, "geheim.md")); string(b) != "x" {
		t.Fatal("Datei außerhalb verändert")
	}

	// Umbenennen und Verschieben; ein vorhandenes Ziel wird nicht überschrieben.
	expect(call(h, "POST", "/api/notes/uni/db.md", ``), http.StatusCreated)
	expect(call(h, "PATCH", "/api/notes/uni/db.md", `{"to":"uni/se.md"}`), http.StatusConflict)
	expect(call(h, "PATCH", "/api/notes/uni/db.md", `{"to":"uni/db.txt"}`), http.StatusBadRequest)
	expect(call(h, "PATCH", "/api/notes/uni/db.md", `{"to":"../db.md"}`), http.StatusBadRequest)
	expect(call(h, "PATCH", "/api/notes/uni", `{"to":"uni/x"}`), http.StatusBadRequest)
	expect(call(h, "PATCH", "/api/notes/uni/db.md", `{"to":"Datenbanken.md"}`), http.StatusOK)
	expect(call(h, "PATCH", "/api/notes/Datenbanken.md", `{"to":"datenbanken.md"}`), http.StatusOK) // nur Groß/klein
	if b, _ := os.ReadFile(file); string(b) != "kairo" {
		t.Fatalf("Ziel verändert: %q", b)
	}
	// Löschen landet in .trash/ und verschwindet aus dem Baum.
	trashed := expect(call(h, "DELETE", "/api/notes/datenbanken.md", ``), http.StatusOK)
	if _, err := os.Stat(filepath.Join(dir, trashed["trash"])); err != nil || !strings.HasPrefix(trashed["trash"], ".trash/") {
		t.Fatalf("nicht im Papierkorb: %v %v", trashed, err)
	}
	expect(call(h, "DELETE", "/api/notes/datenbanken.md", ``), http.StatusNotFound)
	expect(call(h, "DELETE", "/api/notes/link.md", ``), http.StatusOK) // nur der Link, nicht das Ziel
	if b, _ := os.ReadFile(filepath.Join(outside, "geheim.md")); string(b) != "x" {
		t.Fatal("Ziel des Links gelöscht")
	}

	// Baum: nur Ordner und .md, ohne Versteckte und ohne Symlinks nach außen.
	rec := call(h, "GET", "/api/notes", ``)
	var tree []notes.Node
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Path != "uni" || len(tree[0].Children) != 1 || tree[0].Children[0].Path != "uni/se.md" {
		t.Fatalf("Baum: %+v", tree)
	}
	if strings.Contains(rec.Body.String(), "kairo-tmp") {
		t.Fatal("temporäre Datei im Baum")
	}
}
