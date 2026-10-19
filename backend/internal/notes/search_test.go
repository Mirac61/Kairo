package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Uni/Algo.md", "# Algo\n\nDer AVL-Baum ist balanciert.\n")
	write("Uni/Baum-Notizen.md", "nichts\n")
	write(".obsidian/x.md", "AVL-Baum versteckt\n")
	write("lang.md", strings.Repeat("x", 300)+" Baum "+strings.Repeat("y", 300))
	write("bild.png", "Baum")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	hits, err := s.Search("  baum ", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Hit{}
	for _, h := range hits {
		got[h.Path] = h
	}
	if len(hits) != 3 || got["Uni/Algo.md"].Line != 3 || got["Uni/Algo.md"].Snippet != "Der AVL-Baum ist balanciert." {
		t.Fatalf("hits = %+v", hits)
	}
	if h := got["Uni/Baum-Notizen.md"]; h.Line != 1 || h.Snippet != "" { // nur der Dateiname passt
		t.Errorf("Dateiname = %+v", h)
	}
	if sn := got["lang.md"].Snippet; !strings.HasPrefix(sn, "…") || !strings.HasSuffix(sn, "…") || !strings.Contains(sn, "Baum") {
		t.Errorf("Ausschnitt = %q", sn)
	}
	if hits, _ := s.Search("baum", 1); len(hits) != 1 {
		t.Errorf("limit: %d", len(hits))
	}
	if hits, _ := s.Search(" ", 10); len(hits) != 0 {
		t.Errorf("leer: %+v", hits)
	}
}
