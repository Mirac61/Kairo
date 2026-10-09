// Package notes verwaltet Notizen als normale Markdown-Dateien in einem Wurzelordner.
// Die Dateien bleiben die Quelle; Kairo speichert nichts davon in SQLite, damit sie
// parallel in VSCodium bearbeitet werden können.
package notes

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"kairo/internal/domain"
)

// Store greift über os.Root zu: Pfade mit ".." und Symlinks, die aus dem
// Wurzelordner hinauszeigen, lehnt schon das Betriebssystem-API ab.
type Store struct {
	root *os.Root
	mu   sync.Mutex // macht Prüfen und Schreiben in Save atomar (zwei Tabs speichern gleichzeitig)
}

// Node ist ein Eintrag im Dateibaum. Path ist relativ zum Wurzelordner, mit "/".
type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Dir      bool   `json:"dir"`
	Children []Node `json:"children,omitempty"`
}

// Open legt dir an, falls es fehlt, und öffnet es als Wurzelordner.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

// Close gibt den Wurzelordner frei.
func (s *Store) Close() error { return s.root.Close() }

// Tree listet Ordner, Notizen (.md), PDFs und Bilder; versteckte Einträge (.git, .obsidian, …)
// fehlen. Symlinks auf Ordner werden nicht verfolgt (keine Schleifen).
func (s *Store) Tree() ([]Node, error) { return s.tree(".") }

func (s *Store) tree(dir string) ([]Node, error) {
	entries, err := fs.ReadDir(s.root.FS(), dir)
	if err != nil {
		return nil, mapErr(err)
	}
	var dirs, files []Node
	for _, e := range entries {
		p := path.Join(dir, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			children, err := s.tree(p)
			if err != nil {
				return nil, err
			}
			dirs = append(dirs, Node{Name: e.Name(), Path: p, Dir: true, Children: children})
		case Kind(p) != "" && s.isFile(p):
			files = append(files, Node{Name: e.Name(), Path: p})
		}
	}
	byName := func(a, b Node) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) }
	slices.SortFunc(dirs, byName)
	slices.SortFunc(files, byName)
	return append(dirs, files...), nil
}

// isFile: regulär oder ein Symlink, der innerhalb des Wurzelordners auf eine Datei zeigt.
func (s *Store) isFile(p string) bool {
	fi, err := s.root.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// Read liefert Inhalt und Version (mtime) einer Notiz.
func (s *Store) Read(p string) (content, version string, err error) {
	if p, err = checkPath(p, "md"); err != nil {
		return "", "", err
	}
	fi, err := s.root.Stat(p)
	if err != nil {
		return "", "", mapErr(err)
	}
	if !fi.Mode().IsRegular() {
		return "", "", fmt.Errorf("%w: %s ist keine Datei", domain.ErrInvalid, p)
	}
	b, err := s.root.ReadFile(p)
	if err != nil {
		return "", "", mapErr(err)
	}
	return string(b), Version(fi), nil
}

// Save schreibt eine Notiz. Hat sich die Datei seit dem Laden geändert (andere
// Version, auch: inzwischen gelöscht), gibt es domain.ErrConflict, außer bei force.
// Geschrieben wird über eine temporäre Datei und Rename, damit ein Abbruch nie
// eine halbe Notiz hinterlässt (ein Symlink wird dabei durch eine echte Datei ersetzt).
func (s *Store) Save(p, content, version string, force bool) (string, error) {
	p, err := checkPath(p, "md")
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := fs.FileMode(0o644)
	switch fi, err := s.root.Stat(p); {
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", mapErr(err)
	case !force && (err != nil || Version(fi) != version):
		return "", fmt.Errorf("%w: %s wurde außerhalb von Kairo geändert", domain.ErrConflict, p)
	case err == nil:
		perm = fi.Mode().Perm()
	}
	tmp := path.Join(path.Dir(p), "."+path.Base(p)+".kairo-tmp")
	if err := s.root.WriteFile(tmp, []byte(content), perm); err != nil {
		return "", mapErr(err)
	}
	if err := s.root.Rename(tmp, p); err != nil {
		_ = s.root.Remove(tmp)
		return "", mapErr(err)
	}
	fi, err := s.root.Stat(p)
	if err != nil {
		return "", mapErr(err)
	}
	return Version(fi), nil
}

// CreateFile legt eine leere Notiz an; existiert sie schon: domain.ErrConflict.
func (s *Store) CreateFile(p string) (string, error) {
	p, err := checkPath(p, "md")
	if err != nil {
		return "", err
	}
	f, err := s.root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", mapErr(err)
	}
	fi, err := f.Stat()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", mapErr(err)
	}
	return Version(fi), nil
}

// Mkdir legt einen Ordner an; der übergeordnete Ordner muss existieren.
func (s *Store) Mkdir(p string) error {
	p, err := checkPath(p)
	if err != nil {
		return err
	}
	return mapErr(s.root.Mkdir(p, 0o755))
}

// Move benennt eine Notiz oder einen Ordner um bzw. verschiebt sie; ein vorhandenes Ziel wird nie überschrieben.
func (s *Store) Move(from, to string) error {
	from, err := checkPath(from)
	if err != nil {
		return err
	}
	fi, err := s.root.Lstat(from)
	if err != nil {
		return mapErr(err)
	}
	kinds := []string{Kind(from)} // eine Datei behält ihren Typ
	if fi.IsDir() {
		kinds = nil
	}
	if to, err = checkPath(to, kinds...); err != nil {
		return err
	}
	if to == from {
		return nil
	}
	if fi.IsDir() && strings.HasPrefix(to, from+"/") {
		return fmt.Errorf("%w: ein Ordner kann nicht in sich selbst liegen", domain.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Rename ersetzt ein vorhandenes Ziel stillschweigend. Ausnahme: das Ziel ist die Quelle selbst
	// (nur Groß-/Kleinschreibung geändert, auf macOS ohne Unterscheidung der Schreibweise).
	if ti, err := s.root.Lstat(to); err == nil && !os.SameFile(fi, ti) {
		return fmt.Errorf("%w: %s existiert schon", domain.ErrConflict, to)
	}
	return mapErr(s.root.Rename(from, to))
}

// Open öffnet ein PDF oder Bild zum Ausliefern.
func (s *Store) Open(p string) (*os.File, fs.FileInfo, error) {
	p, err := checkPath(p, "pdf", "image")
	if err != nil {
		return nil, nil, err
	}
	f, err := s.root.Open(p)
	if err != nil {
		return nil, nil, mapErr(err)
	}
	fi, err := f.Stat()
	if err == nil && !fi.Mode().IsRegular() {
		err = fmt.Errorf("%w: %s ist keine Datei", domain.ErrInvalid, p)
	}
	if err != nil {
		f.Close()
		return nil, nil, mapErr(err)
	}
	return f, fi, nil
}

// Upload legt ein PDF oder Bild an; existiert es schon: domain.ErrConflict.
// Bricht das Schreiben ab, wird die halbe Datei wieder gelöscht.
func (s *Store) Upload(p string, r io.Reader) error {
	p, err := checkPath(p, "pdf", "image")
	if err != nil {
		return err
	}
	f, err := s.root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return mapErr(err)
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = s.root.Remove(p)
		return mapErr(err)
	}
	return nil
}

// Replace ersetzt ein PDF oder Bild (nach dem Bearbeiten in Kairo). Die alte Fassung
// landet in .trash/, damit sich Zeichnungen rückgängig machen lassen. Wie bei Save gibt
// es domain.ErrConflict, wenn sich die Datei seit dem Laden geändert hat, außer bei force.
func (s *Store) Replace(p string, r io.Reader, version string, force bool) (string, error) {
	p, err := checkPath(p, "pdf", "image")
	if err != nil {
		return "", err
	}
	tmp := path.Join(path.Dir(p), "."+path.Base(p)+".kairo-tmp")
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", mapErr(err)
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = s.root.Remove(tmp)
		return "", mapErr(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fi, err := s.root.Stat(p)
	switch {
	case err != nil:
		err = mapErr(err)
	case !force && Version(fi) != version:
		err = fmt.Errorf("%w: %s wurde außerhalb von Kairo geändert", domain.ErrConflict, p)
	default:
		_ = s.root.Chmod(tmp, fi.Mode().Perm())
		if _, err = s.trash(p); err == nil {
			err = mapErr(s.root.Rename(tmp, p))
		}
	}
	if err != nil {
		_ = s.root.Remove(tmp)
		return "", err
	}
	if fi, err = s.root.Stat(p); err != nil {
		return "", mapErr(err)
	}
	return Version(fi), nil
}

// Trash ist der Ordner für gelöschte Notizen; als versteckter Ordner fehlt er im Baum.
const Trash = ".trash"

// Delete verschiebt eine Notiz oder einen Ordner (mit Inhalt) nach .trash/, mit Zeitstempel im Namen.
// Wiederherstellen geht von Hand im Finder oder in VSCodium.
func (s *Store) Delete(p string) (string, error) {
	p, err := checkPath(p)
	if err != nil {
		return "", err
	}
	if _, err := s.root.Lstat(p); err != nil {
		return "", mapErr(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trash(p)
}

// trash verschiebt p nach .trash/; der Aufrufer hält s.mu.
func (s *Store) trash(p string) (string, error) {
	if err := s.root.Mkdir(Trash, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", mapErr(err)
	}
	dest := path.Join(Trash, time.Now().Format("20060102-150405")+" "+path.Base(p))
	for i := 2; ; i++ { // zweimal dasselbe in einer Sekunde gelöscht
		if _, err := s.root.Lstat(dest); errors.Is(err, fs.ErrNotExist) {
			break
		}
		dest = path.Join(Trash, fmt.Sprintf("%s %d %s", time.Now().Format("20060102-150405"), i, path.Base(p)))
	}
	return dest, mapErr(s.root.Rename(p, dest))
}

// checkPath lässt nur relative Pfade ohne ".." und ohne versteckte Teile zu;
// sind kinds angegeben, muss Kind(p) einer davon sein.
func checkPath(p string, kinds ...string) (string, error) {
	if !filepath.IsLocal(p) || p == "." || path.Clean(p) != p {
		return "", fmt.Errorf("%w: Pfad %q nicht erlaubt", domain.ErrInvalid, p)
	}
	for part := range strings.SplitSeq(p, "/") {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%w: versteckte Pfade (%q) nicht erlaubt", domain.ErrInvalid, p)
		}
	}
	if kinds != nil && !slices.Contains(kinds, Kind(p)) {
		return "", fmt.Errorf("%w: Dateityp von %q nicht erlaubt", domain.ErrInvalid, p)
	}
	return p, nil
}

// Kind ist der Dateityp, den Kairo im Notizordner zeigt: "md", "pdf", "image"
// oder "" (alles andere bleibt unsichtbar). SVG fehlt, weil es Skript enthalten kann.
func Kind(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".md":
		return "md"
	case ".pdf":
		return "pdf"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	}
	return ""
}

// Version ist die mtime in Nanosekunden, als String (passt sonst nicht exakt in JS-Zahlen).
func Version(fi fs.FileInfo) string { return strconv.FormatInt(fi.ModTime().UnixNano(), 10) }

// mapErr übersetzt Dateisystemfehler in Domain-Fehler. Alles Übrige, vor allem
// "path escapes from parent" von os.Root (Symlink nach außen), gilt als ungültige Eingabe.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf("%w: existiert schon", domain.ErrConflict)
	default:
		return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
}
