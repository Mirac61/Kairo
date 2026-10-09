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
	dir := filepath.Join(t.TempDir(), "notizen") // fehlt noch, Open legt ihn an
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
	_ = os.WriteFile(filepath.Join(dir, "bild.png"), nil, 0o644)
	_ = os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	forbidden := []string{"/api/notes/..%2Fx.md", "/api/notes/uni%2F..%2F..%2Fx.md", "/api/notes/bild.png", "/api/notes/.git/x.md"}
	// Symlinks braucht unter Windows Admin-Rechte; ohne sie fehlt nur dieser Teil.
	if os.Symlink(filepath.Join(outside, "geheim.md"), filepath.Join(dir, "link.md")) == nil &&
		os.Symlink(outside, filepath.Join(dir, "raus")) == nil {
		forbidden = append(forbidden, "/api/notes/link.md", "/api/notes/raus/geheim.md")
	}
	for _, p := range forbidden {
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

	// Baum: nur Ordner, .md, PDFs und Bilder, ohne Versteckte und ohne Symlinks nach außen.
	rec := call(h, "GET", "/api/notes", ``)
	var tree []notes.Node
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].Path != "uni" || tree[1].Path != "bild.png" || len(tree[0].Children) != 1 || tree[0].Children[0].Path != "uni/se.md" {
		t.Fatalf("Baum: %+v", tree)
	}
	if strings.Contains(rec.Body.String(), "kairo-tmp") {
		t.Fatal("temporäre Datei im Baum")
	}
}

func TestNoteFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := notes.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{Notes: store})
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 16)
	_ = os.WriteFile(filepath.Join(dir, "skript.pdf"), []byte("%PDF-1.4"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "film.mp4"), []byte("x"), 0o644)

	if rec := call(h, "POST", "/api/files/a.png", png); rec.Code != http.StatusCreated {
		t.Fatalf("Upload: %d %s", rec.Code, rec.Body)
	}
	for p, want := range map[string]int{
		"/api/files/a.png":      http.StatusConflict,   // nie überschreiben
		"/api/files/b.svg":      http.StatusBadRequest, // kein erlaubter Typ
		"/api/files/b.md":       http.StatusBadRequest,
		"/api/files/..%2Fb.png": http.StatusBadRequest,
	} {
		if rec := call(h, "POST", p, png); rec.Code != want {
			t.Errorf("POST %s: %d, erwartet %d", p, rec.Code, want)
		}
	}
	if rec := call(h, "POST", "/api/files/c.png", "<html>"); rec.Code != http.StatusBadRequest {
		t.Errorf("Nicht-Bild hochgeladen: %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/files/d.pdf", png); rec.Code != http.StatusBadRequest {
		t.Errorf("Bild als PDF hochgeladen: %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/files/Folien.pdf", "%PDF-1.7\n"); rec.Code != http.StatusCreated {
		t.Errorf("PDF hochladen: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.png")); err == nil {
		t.Error("abgelehntes Bild liegt trotzdem auf der Platte")
	}

	rec := call(h, "GET", "/api/files/a.png", ``)
	if rec.Code != http.StatusOK || rec.Body.String() != png || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("GET a.png: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := call(h, "GET", "/api/files/skript.pdf", ``); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("GET skript.pdf: %d %v", rec.Code, rec.Header())
	}
	for _, p := range []string{"/api/files/film.mp4", "/api/files/notiz.md"} {
		if rec := call(h, "GET", p, ``); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: %d, erwartet 400", p, rec.Code)
		}
	}

	// Bearbeitet speichern: passende Version ersetzt, die alte Fassung liegt danach in .trash/.
	v := call(h, "GET", "/api/files/a.png", ``).Header().Get("X-Kairo-Mtime")
	png2 := png + "neu"
	if rec := call(h, "PUT", "/api/files/a.png?mtime=alt", png2); rec.Code != http.StatusConflict {
		t.Errorf("veraltete Version: %d", rec.Code)
	}
	if rec := call(h, "PUT", "/api/files/a.png?mtime="+v, "<html>"); rec.Code != http.StatusBadRequest {
		t.Errorf("Nicht-Bild ersetzt: %d", rec.Code)
	}
	if rec := call(h, "PUT", "/api/files/a.png?mtime="+v, png2); rec.Code != http.StatusOK {
		t.Fatalf("ersetzen: %d %s", rec.Code, rec.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.png")); string(b) != png2 {
		t.Fatal("nicht ersetzt")
	}
	if old, _ := filepath.Glob(filepath.Join(dir, ".trash", "* a.png")); len(old) != 1 {
		t.Fatalf("alte Fassung fehlt im Papierkorb: %v", old)
	} else if b, _ := os.ReadFile(old[0]); string(b) != png {
		t.Fatal("Papierkorb enthält nicht die alte Fassung")
	}
	if rec := call(h, "PUT", "/api/files/a.png?mtime=alt&force=1", png); rec.Code != http.StatusOK {
		t.Errorf("force: %d", rec.Code)
	}
	if rec := call(h, "PUT", "/api/files/fehlt.png?force=1", png); rec.Code != http.StatusNotFound {
		t.Errorf("ersetzen ohne Datei: %d", rec.Code)
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, ".*kairo-tmp")); len(tmp) != 0 {
		t.Fatalf("temporäre Dateien übrig: %v", tmp)
	}

	// PDFs lassen sich umbenennen, behalten aber ihren Typ.
	if rec := call(h, "PATCH", "/api/notes/skript.pdf", `{"to":"skript.md"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("Typwechsel beim Umbenennen: %d", rec.Code)
	}
	if rec := call(h, "PATCH", "/api/notes/skript.pdf", `{"to":"Skript 1.pdf"}`); rec.Code != http.StatusOK {
		t.Errorf("PDF umbenennen: %d %s", rec.Code, rec.Body)
	}

	var tree []notes.Node
	_ = json.NewDecoder(call(h, "GET", "/api/notes", ``).Body).Decode(&tree)
	var names []string
	for _, n := range tree {
		names = append(names, n.Name)
	}
	if strings.Join(names, ",") != "a.png,Folien.pdf,Skript 1.pdf" {
		t.Fatalf("Baum: %v", names)
	}
}
