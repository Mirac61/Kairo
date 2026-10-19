package notes

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
