package notes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kairo/internal/domain"
)

func TestBacklinks(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Uni/SE.md":      "# SE\n[[SE]] verweist auf sich selbst",
		"a.md":           "Intro\nSiehe [[se]] dazu.\nNoch [[SE]] einmal",
		"b.md":           "Mit Alias [[SE|Software Engineering]] und mehr",
		"Uni/c.md":       "Überschrift [[Uni/SE#Ziele]] im Ordnerlink",
		"d.md":           "![[SE.md]] eingebettet",
		"e.md":           "[[SEE]] [[Andere Notiz]] [SE] [[ SE x]] kein Treffer",
		".trash/alt.md":  "[[SE]]",
		".obsidian/x.md": "[[SE]]",
		"f.txt":          "[[SE]]",
		"lang.md":        "[[SE]] " + strings.Repeat("x", 300),
	}
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	got, err := s.Backlinks("Uni/SE.md")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, b := range got {
		paths = append(paths, b.Path)
	}
	if want := []string{"a.md", "b.md", "d.md", "lang.md", "Uni/c.md"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("Pfade %v, erwartet %v", paths, want)
	}
	if got[0].Title != "a" || got[0].Snippet != "Siehe [[se]] dazu." {
		t.Errorf("Titel/Snippet: %+v", got[0])
	}
	if n := len([]rune(got[3].Snippet)); n != 161 {
		t.Errorf("Snippet nicht gekürzt: %d Zeichen", n)
	}

	if got, err = s.Backlinks("a.md"); err != nil || len(got) != 0 {
		t.Errorf("a.md: %v, %v", got, err)
	}
	for _, bad := range []string{"../x.md", "/etc/passwd", ".trash/alt.md", "a.txt", ""} {
		if _, err := s.Backlinks(bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: Fehler %v, erwartet ErrInvalid", bad, err)
		}
	}
}

// Gleichnamige Notizen in verschiedenen Ordnern: [[Name]] zählt für den kürzesten Pfad,
// [[Ordner/Name]] nur für diesen Pfad.
func TestBacklinksDisambiguation(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Uni/SE.md":    "# Uni\n",
		"Privat/SE.md": "# Privat\n",
		"x.md":         "[[SE]] und [[Uni/SE]]",
		"y.md":         "[[Privat/SE]]",
		"Uni/z.md":     "[[SE]] neben der Uni-Notiz",
	}
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	paths := func(p string) []string {
		got, err := s.Backlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, b := range got {
			out = append(out, b.Path)
		}
		return out
	}
	// [[SE]] (in x.md) löst zum kürzesten Pfad Uni/SE.md; [[Uni/SE]] ebenfalls. Uni/z.md hat [[SE]] neben der Notiz.
	if want := []string{"Uni/z.md", "x.md"}; !reflect.DeepEqual(paths("Uni/SE.md"), want) {
		t.Fatalf("Uni/SE.md: %v, erwartet %v", paths("Uni/SE.md"), want)
	}
	// Privat/SE.md bekommt nur den expliziten Pfad-Link, nicht das bloße [[SE]].
	if want := []string{"y.md"}; !reflect.DeepEqual(paths("Privat/SE.md"), want) {
		t.Fatalf("Privat/SE.md: %v, erwartet %v", paths("Privat/SE.md"), want)
	}
}

// Nach dem Speichern wird der Index verworfen und neu aufgebaut (kein veralteter Rückverweis).
func TestBacklinksCacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	for p, c := range map[string]string{"a.md": "[[b]]", "b.md": "", "c.md": ""} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if got, _ := s.Backlinks("b.md"); len(got) != 1 || got[0].Path != "a.md" {
		t.Fatalf("vor dem Speichern: %+v", got)
	}
	// a.md verweist jetzt auf c; der Index muss das nach dem Save widerspiegeln.
	if _, err := s.Save("a.md", "[[c]]", "", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Backlinks("b.md"); len(got) != 0 {
		t.Fatalf("b.md nach Save: %+v", got)
	}
	if got, _ := s.Backlinks("c.md"); len(got) != 1 || got[0].Path != "a.md" {
		t.Fatalf("c.md nach Save: %+v", got)
	}
}

// Umbenennen zieht [[Links]] in anderen Notizen mit (Name und Pfad, Suffix bleibt).
func TestMoveRewritesLinks(t *testing.T) {
	dir := t.TempDir()
	for p, c := range map[string]string{
		"a.md":      "Siehe [[SE]], [[Uni/SE]] und [[SE#Ziele|mehr]].\n",
		"Uni/SE.md": "# SE\n",
		"b.md":      "[[Andere]] bleibt unberührt.\n",
	} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Move("Uni/SE.md", "Uni/SoftwareEngineering.md"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if got, want := string(b), "Siehe [[SoftwareEngineering]], [[Uni/SoftwareEngineering]] und [[SoftwareEngineering#Ziele|mehr]].\n"; got != want {
		t.Fatalf("a.md = %q, erwartet %q", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.md")); string(b) != "[[Andere]] bleibt unberührt.\n" {
		t.Errorf("b.md wurde verändert: %q", b)
	}
}

func TestHistory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()

	v, _ := s.CreateFile("a.md")
	save := func(c string) {
		t.Helper()
		if v, err = s.Save("a.md", c, v, false); err != nil {
			t.Fatal(err)
		}
	}
	snaps := func() []Snapshot {
		t.Helper()
		l, err := s.History("a.md")
		if err != nil {
			t.Fatal(err)
		}
		return l
	}

	save("eins") // vorher leer: Schnappschuss von ""
	save("zwei") // 0 s später: Sperrfrist
	save("zwei") // unverändert
	if n := len(snaps()); n != 1 {
		t.Fatalf("nach drei Saves in der Sperrfrist: %d Schnappschüsse, want 1", n)
	}
	for i := range 25 { // alle 6 Minuten ein Stand, mehr als historyKeep
		clock = clock.Add(6 * time.Minute)
		save(fmt.Sprintf("v%d", i))
	}
	l := snaps()
	if len(l) != historyKeep {
		t.Fatalf("%d Schnappschüsse, want %d", len(l), historyKeep)
	}
	if got, _ := s.ReadSnapshot("a.md", l[0].Name); got != "v23" {
		t.Errorf("neuester Schnappschuss = %q, want v23", got)
	}

	clock = clock.Add(time.Second) // gleiche Sekunde wie der letzte Save wird übersprungen
	// Restore: ältere Fassung zurück, aktuelle (v24) landet trotz Sperrfrist im Verlauf.
	got, _, err := s.Restore("a.md", l[1].Name)
	if err != nil || got != "v22" {
		t.Fatalf("Restore = %q, %v; want v22", got, err)
	}
	if cur, _, _ := s.Read("a.md"); cur != "v22" {
		t.Errorf("Notiz = %q, want v22", cur)
	}
	if top, _ := s.ReadSnapshot("a.md", snaps()[0].Name); top != "v24" {
		t.Errorf("Restore hat v24 nicht gesichert: %q", top)
	}

	if _, err := s.ReadSnapshot("a.md", "../../x.md"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Pfad im Namen: %v, want ErrInvalid", err)
	}
	if tree, _ := s.Tree(); len(tree) != 1 {
		t.Errorf("Baum zeigt %d Einträge, want 1 (.history versteckt)", len(tree))
	}
}
