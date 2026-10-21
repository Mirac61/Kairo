package notes

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJournalDates(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dates, err := store.JournalDates()
	if err != nil || len(dates) != 0 {
		t.Fatalf("fehlender Ordner: dates=%v, err=%v", dates, err)
	}

	dir := filepath.Join(root, "Journal")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"2026-10-21.md", "2026-10-20.md", "2026-02-30.md", "Notiz.md", "2026-10-19.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "2026-10-18.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	dates, err = store.JournalDates()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2026-10-20", "2026-10-21"}; !reflect.DeepEqual(dates, want) {
		t.Errorf("JournalDates = %v, erwartet %v", dates, want)
	}
}
