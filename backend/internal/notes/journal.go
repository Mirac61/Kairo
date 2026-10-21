package notes

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// JournalDates liefert gültige Datumsnamen der Dateien unter Journal/, aufsteigend.
// Fehlende Journale werden wie ein leeres Journal behandelt.
func (s *Store) JournalDates() ([]string, error) {
	entries, err := fs.ReadDir(s.root.FS(), "Journal")
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || !s.isFile(path.Join("Journal", entry.Name())) {
			continue
		}
		date := strings.TrimSuffix(entry.Name(), ".md")
		if _, err := time.Parse("2006-01-02", date); err == nil {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	return dates, nil
}
