package notes

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"kairo/internal/domain"
)

// History ist der versteckte Ordner mit älteren Fassungen: .history/<Pfad der Notiz>/<Zeit>.md.
// Wie .trash fehlt er in Baum, Suche, Index und ZIP-Export.
const History = ".history"

const (
	historyEvery = 5 * time.Minute // höchstens ein Schnappschuss in diesem Abstand
	historyKeep  = 20              // so viele bleiben je Notiz, die ältesten fallen weg
	snapFormat   = "20060102-150405"
)

var snapName = regexp.MustCompile(`^\d{8}-\d{6}\.md$`)

var now = time.Now // für Tests

// Snapshot ist eine ältere Fassung einer Notiz.
type Snapshot struct {
	Name string `json:"name"`
	Time string `json:"time"` // RFC 3339
	Size int64  `json:"size"`
}

// snapshotNames listet die Schnappschüsse von p aufsteigend (ältester zuerst); ein fehlender Ordner ergibt nil.
func (s *Store) snapshotNames(p string) []string {
	ents, err := fs.ReadDir(s.root.FS(), path.Join(History, p))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && snapName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names
}

// snapshot legt old (die Fassung vor dem Überschreiben) ab; der Aufrufer hält s.mu.
// Ohne force höchstens alle historyEvery. Fehler werden verschluckt: der Verlauf ist ein
// Extra und darf das Speichern nie blockieren.
// shortcut: Umbenennen oder Löschen einer Notiz lässt ihren Verlauf als Waise liegen, bei Bedarf mitverschieben.
func (s *Store) snapshot(p string, old []byte, force bool) {
	names := s.snapshotNames(p)
	name := now().Format(snapFormat) + ".md"
	if n := len(names); n > 0 {
		if names[n-1] == name {
			return
		}
		last, err := time.ParseInLocation(snapFormat, strings.TrimSuffix(names[n-1], ".md"), time.Local)
		if !force && err == nil && now().Sub(last) < historyEvery {
			return
		}
	}
	dir := path.Join(History, p)
	if s.root.MkdirAll(dir, 0o755) != nil || s.root.WriteFile(path.Join(dir, name), old, 0o644) != nil {
		return
	}
	names = append(names, name)
	for len(names) > historyKeep {
		_ = s.root.Remove(path.Join(dir, names[0]))
		names = names[1:]
	}
}

// History listet die älteren Fassungen von p, die neueste zuerst.
func (s *Store) History(p string) ([]Snapshot, error) {
	p, err := checkPath(p, "md")
	if err != nil {
		return nil, err
	}
	names := s.snapshotNames(p)
	out := make([]Snapshot, 0, len(names))
	for _, n := range slices.Backward(names) {
		fi, err := s.root.Stat(path.Join(History, p, n))
		if err != nil {
			continue
		}
		t, _ := time.ParseInLocation(snapFormat, strings.TrimSuffix(n, ".md"), time.Local)
		out = append(out, Snapshot{Name: n, Time: t.Format(time.RFC3339), Size: fi.Size()})
	}
	return out, nil
}

// ReadSnapshot liefert den Inhalt einer älteren Fassung.
func (s *Store) ReadSnapshot(p, name string) (string, error) {
	p, err := checkPath(p, "md")
	if err != nil {
		return "", err
	}
	if !snapName.MatchString(name) {
		return "", fmt.Errorf("%w: Fassung %q unbekannt", domain.ErrInvalid, name)
	}
	b, err := s.root.ReadFile(path.Join(History, p, name))
	if err != nil {
		return "", mapErr(err)
	}
	return string(b), nil
}

// Restore schreibt eine ältere Fassung zurück. Die aktuelle Fassung landet vorher im Verlauf,
// auch innerhalb der Sperrfrist; die Notiz darf dafür geändert oder gelöscht sein (force).
func (s *Store) Restore(p, name string) (content, version string, err error) {
	if content, err = s.ReadSnapshot(p, name); err != nil {
		return "", "", err
	}
	version, err = s.save(p, content, "", true, true)
	return content, version, err
}
