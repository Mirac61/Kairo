package config

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// env liefert m als Umgebung; ohne KAIRO_CONFIG_PATH zeigt sie auf eine fehlende
// Datei, damit Tests nie die echte config.json lesen.
func env(m map[string]string) func(string) string {
	return func(k string) string {
		if k == "KAIRO_CONFIG_PATH" && m[k] == "" {
			return "/nonexistent/kairo/config.json"
		}
		return m[k]
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8742 || cfg.LogLevel != slog.LevelInfo || cfg.Location != time.Local {
		t.Errorf("unerwartete Defaults: %+v", cfg)
	}
	if !strings.HasSuffix(cfg.DBPath, filepath.Join(".local", "share", "kairo", "kairo.db")) {
		t.Errorf("DBPath = %s", cfg.DBPath)
	}
	if !strings.HasSuffix(cfg.TokenPath, filepath.Join(".config", "kairo", "token")) {
		t.Errorf("TokenPath = %s", cfg.TokenPath)
	}
	if filepath.Base(cfg.NotesDir) != "Kairo" {
		t.Errorf("NotesDir = %s", cfg.NotesDir)
	}
	if cfg.Addr() != "127.0.0.1:8742" {
		t.Errorf("Addr = %s", cfg.Addr())
	}
}

func TestLoadValues(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"KAIRO_PORT": "9000", "KAIRO_DB_PATH": "/tmp/x.db", "KAIRO_TOKEN_PATH": "/tmp/t", "KAIRO_NOTES_DIR": "/tmp/n",
		"KAIRO_TIMEZONE": "Europe/Berlin", "KAIRO_LOG_LEVEL": "DEBUG",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 || cfg.DBPath != "/tmp/x.db" || cfg.TokenPath != "/tmp/t" || cfg.NotesDir != "/tmp/n" ||
		cfg.Location.String() != "Europe/Berlin" || cfg.LogLevel != slog.LevelDebug {
		t.Errorf("unerwartet: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	for _, key := range []string{"KAIRO_PORT", "KAIRO_TIMEZONE", "KAIRO_LOG_LEVEL"} {
		for _, val := range []string{"nope", "0", "70000"} {
			if key != "KAIRO_PORT" && val != "nope" {
				continue
			}
			_, err := Load(env(map[string]string{key: val}))
			var ce *Error
			if !errors.As(err, &ce) || ce.Key != key {
				t.Errorf("%s=%s: err = %v", key, val, err)
			}
		}
	}
}

func TestLoadWorkWindow(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil || cfg.WorkStart != 540 || cfg.WorkEnd != 1020 {
		t.Fatalf("Default 09:00-17:00 erwartet: %+v, %v", cfg, err)
	}
	cfg, err = Load(env(map[string]string{"KAIRO_WORK_START": "08:30", "KAIRO_WORK_END": "16:00"}))
	if err != nil || cfg.WorkStart != 510 || cfg.WorkEnd != 960 {
		t.Fatalf("08:30-16:00 erwartet: %+v, %v", cfg, err)
	}
	for _, bad := range []map[string]string{
		{"KAIRO_WORK_START": "9"},
		{"KAIRO_WORK_END": "25:00"},
		{"KAIRO_WORK_START": "18:00"}, // Ende 17:00 liegt davor
	} {
		if _, err := Load(env(bad)); err == nil {
			t.Errorf("%v sollte ungültig sein", bad)
		}
	}
}

func TestLoadFileUnderEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kairo", "config.json")
	if err := WriteFile(path, File{NotesDir: dir, WorkStart: "08:00", Timezone: "Europe/Berlin"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(env(map[string]string{"KAIRO_CONFIG_PATH": path, "KAIRO_WORK_START": "07:00"}))
	if err != nil {
		t.Fatal(err)
	}
	// Datei gilt, die Umgebung schlägt sie.
	if cfg.NotesDir != dir || cfg.WorkStart != 420 || cfg.Location.String() != "Europe/Berlin" || cfg.ConfigPath != path {
		t.Errorf("unerwartet: %+v", cfg)
	}
	if got := cfg.File(); got != (File{NotesDir: dir, WorkStart: "07:00", WorkEnd: "17:00", Timezone: "Europe/Berlin"}) {
		t.Errorf("File() = %+v", got)
	}
}

func TestFileValidate(t *testing.T) {
	dir := t.TempDir()
	if err := (File{NotesDir: dir, WorkStart: "08:00", WorkEnd: "12:00"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []File{
		{NotesDir: filepath.Join(dir, "fehlt")},
		{NotesDir: "relativ"},
		{WorkStart: "18:00"},
		{Timezone: "Mars/Olympus"},
	} {
		if f.Validate() == nil {
			t.Errorf("%+v sollte ungültig sein", f)
		}
	}
}
