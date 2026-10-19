package notes

import (
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

// Hit ist ein Treffer der Volltextsuche: die erste passende Zeile einer Notiz.
type Hit struct {
	Path    string `json:"path"`
	Line    int    `json:"line"` // ab 1
	Snippet string `json:"snippet"`
}

const (
	maxSearchBytes = 2 << 20 // größere .md-Dateien sind eher Exporte als Notizen
	snippetRunes   = 120
)

// Search sucht q (ohne Groß/Klein) in allen Notizen (.md, ohne versteckte Ordner) und liefert höchstens limit
// Treffer, sortiert nach Pfad. Ein leeres q liefert nichts.
// shortcut: liest bei jeder Suche alle Dateien; reicht für einige tausend Notizen, sonst einen Index (SQLite FTS5) bauen.
func (s *Store) Search(q string, limit int) ([]Hit, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	hits := []Hit{}
	if q == "" {
		return hits, nil
	}
	err := fs.WalkDir(s.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unlesbarer Ordner: überspringen statt die ganze Suche abzubrechen
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || Kind(p) != "md" || len(hits) >= limit {
			return nil
		}
		if fi, err := d.Info(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxSearchBytes {
			return nil
		}
		b, err := s.root.ReadFile(p)
		if err != nil {
			return nil
		}
		if h, ok := firstHit(p, string(b), q); ok {
			hits = append(hits, h)
		}
		return nil
	})
	return hits, err
}

// firstHit findet die erste Zeile mit q oder, wenn nur der Dateiname passt, die erste Zeile überhaupt.
func firstHit(p, content, q string) (Hit, bool) {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			return Hit{Path: p, Line: i + 1, Snippet: snippet(line, q)}, true
		}
	}
	if strings.Contains(strings.ToLower(path.Base(p)), q) {
		return Hit{Path: p, Line: 1}, true
	}
	return Hit{}, false
}

// snippet kürzt die Zeile auf ein Fenster um den Treffer.
func snippet(line, q string) string {
	line = strings.TrimSpace(line)
	if utf8.RuneCountInString(line) <= snippetRunes {
		return line
	}
	r := []rune(line)
	at := utf8.RuneCountInString(line[:max(0, strings.Index(strings.ToLower(line), q))])
	if at > len(r) { // ToLower kann Längen verschieben; dann vom Anfang
		at = 0
	}
	from := max(0, at-snippetRunes/3)
	to := min(len(r), from+snippetRunes)
	out := string(r[from:to])
	if from > 0 {
		out = "…" + out
	}
	if to < len(r) {
		out += "…"
	}
	return out
}
