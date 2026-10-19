package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sqliteHeader = "SQLite format 3\x00"

func TestRestoreDB(t *testing.T) {
	dir := t.TempDir()
	db, src := filepath.Join(dir, "k.db"), filepath.Join(dir, "backup.db")
	write := func(path, s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	write(db, sqliteHeader+"alt")
	write(db+"-wal", "wal")
	write(src, sqliteHeader+"neu")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	// Läuft schon etwas auf dem Port, bleibt alles unberührt.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restoreDB(db, ln.Addr().String(), src, now, false); err == nil || !strings.Contains(err.Error(), "belegt") {
		t.Errorf("belegter Port: err = %v", err)
	}
	// Mit --force (force=true) läuft die Wiederherstellung trotz belegtem Port.
	forceDB := filepath.Join(dir, "force.db")
	write(forceDB, sqliteHeader+"alt")
	if _, err := restoreDB(forceDB, ln.Addr().String(), src, now, true); err != nil {
		t.Errorf("force ignoriert den belegten Port nicht: %v", err)
	}
	if read(forceDB) != sqliteHeader+"neu" {
		t.Error("force hat nicht wiederhergestellt")
	}
	ln.Close()

	write(filepath.Join(dir, "text.txt"), "kein SQLite")
	if _, err := restoreDB(db, "127.0.0.1:0", filepath.Join(dir, "text.txt"), now, false); err == nil {
		t.Error("Datei ohne SQLite-Header wurde akzeptiert")
	}
	if read(db) != sqliteHeader+"alt" {
		t.Fatal("Datenbank wurde trotz Fehler verändert")
	}

	aside, err := restoreDB(db, "127.0.0.1:0", src, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := db + ".before-restore-20261019-120000"; aside != want {
		t.Errorf("aside = %s, will %s", aside, want)
	}
	if read(db) != sqliteHeader+"neu" || read(aside) != sqliteHeader+"alt" || read(aside+"-wal") != "wal" {
		t.Error("Inhalt nach dem Wiederherstellen falsch")
	}
	if _, err := os.Stat(db + "-wal"); err == nil {
		t.Error("altes WAL liegt noch neben der wiederhergestellten Datenbank")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".restore-*")); len(left) != 0 {
		t.Errorf("Temp-Datei übrig: %v", left)
	}
}
