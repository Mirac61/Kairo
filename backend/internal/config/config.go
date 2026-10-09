// Package config liest die Kairo-Konfiguration aus config.json und der Umgebung.
// Umgebungsvariablen haben Vorrang vor der Datei.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BindHost ist die einzige erlaubte Bind-Adresse.
const BindHost = "127.0.0.1"

// Config enthält alle Laufzeiteinstellungen.
type Config struct {
	Port      int
	DBPath    string
	TokenPath string
	// ConfigPath ist die Datei mit den Einstellungen aus der WebUI.
	ConfigPath string
	// NotesDir ist der Wurzelordner der Notizen (Markdown-Dateien).
	NotesDir string
	Location *time.Location
	LogLevel slog.Level
	// WorkStart und WorkEnd begrenzen die Arbeitszeit eines Tages in Minuten
	// seit Mitternacht (Ortszeit); immer WorkStart < WorkEnd.
	WorkStart, WorkEnd int
}

// Addr liefert die Listen-Adresse (immer localhost).
func (c Config) Addr() string { return fmt.Sprintf("%s:%d", BindHost, c.Port) }

// Error beschreibt einen ungültigen Konfigurationswert.
type Error struct {
	Key, Value, Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("config: %s=%q ungültig: %s", e.Key, e.Value, e.Reason)
}

// File sind die Einstellungen in config.json, die die WebUI ändern darf.
// Leere Felder bedeuten Standardwert.
type File struct {
	NotesDir  string `json:"notesDir,omitempty"`
	WorkStart string `json:"workStart,omitempty"`
	WorkEnd   string `json:"workEnd,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
}

// FileKeys ordnet jedem Feld von File die Umgebungsvariable zu, die es überschreibt.
var FileKeys = map[string]string{
	"notesDir": "KAIRO_NOTES_DIR", "workStart": "KAIRO_WORK_START", "workEnd": "KAIRO_WORK_END", "timezone": "KAIRO_TIMEZONE",
}

func (f File) lookup(key string) string {
	return map[string]string{
		"KAIRO_NOTES_DIR": f.NotesDir, "KAIRO_WORK_START": f.WorkStart, "KAIRO_WORK_END": f.WorkEnd, "KAIRO_TIMEZONE": f.Timezone,
	}[key]
}

// Validate prüft die Werte wie Load und zusätzlich, dass der Notizordner
// existiert und beschreibbar ist.
func (f File) Validate() error {
	if _, err := resolve(f.lookup); err != nil {
		return err
	}
	if f.NotesDir == "" {
		return nil
	}
	if !filepath.IsAbs(f.NotesDir) {
		return &Error{"notesDir", f.NotesDir, "erwartet absoluten Pfad"}
	}
	probe, err := os.CreateTemp(f.NotesDir, ".kairo-check-*")
	if err != nil {
		return &Error{"notesDir", f.NotesDir, "Ordner fehlt oder ist nicht beschreibbar"}
	}
	probe.Close()
	return os.Remove(probe.Name())
}

// ReadFile liest config.json; fehlt die Datei, ist das Ergebnis leer.
func ReadFile(path string) (File, error) {
	var f File
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("config: %s: %w", path, err)
	}
	return f, nil
}

// WriteFile schreibt config.json und legt den Ordner bei Bedarf an.
func WriteFile(path string, f File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// File liefert die laufenden Werte in der Form von config.json.
// Die Systemzeitzone ist leer.
func (c Config) File() File {
	f := File{NotesDir: c.NotesDir, WorkStart: clock(c.WorkStart), WorkEnd: clock(c.WorkEnd)}
	if c.Location != time.Local {
		f.Timezone = c.Location.String()
	}
	return f
}

func clock(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

// Load liest config.json (Pfad aus KAIRO_CONFIG_PATH, sonst
// ~/.config/kairo/config.json) und danach die Umgebung über getenv (z. B. os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	path := getenv("KAIRO_CONFIG_PATH")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("config: Home-Verzeichnis unbekannt: %w", err)
		}
		path = filepath.Join(home, ".config", "kairo", "config.json")
	}
	f, err := ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg, err := resolve(func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return f.lookup(k)
	})
	cfg.ConfigPath = path
	return cfg, err
}

func resolve(getenv func(string) string) (Config, error) {
	var cfg Config
	var errs []error

	cfg.Port = 8742
	if v := getenv("KAIRO_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, &Error{"KAIRO_PORT", v, "erwartet Zahl 1-65535"})
		} else {
			cfg.Port = p
		}
	}

	home, homeErr := os.UserHomeDir()
	cfg.DBPath = getenv("KAIRO_DB_PATH")
	if cfg.DBPath == "" {
		if homeErr != nil {
			errs = append(errs, fmt.Errorf("config: Home-Verzeichnis unbekannt: %w", homeErr))
		}
		cfg.DBPath = filepath.Join(home, ".local", "share", "kairo", "kairo.db")
	}
	cfg.TokenPath = getenv("KAIRO_TOKEN_PATH")
	if cfg.TokenPath == "" {
		cfg.TokenPath = filepath.Join(home, ".config", "kairo", "token")
	}
	cfg.NotesDir = getenv("KAIRO_NOTES_DIR")
	if cfg.NotesDir == "" {
		cfg.NotesDir = filepath.Join(home, "Kairo")
	}

	cfg.Location = time.Local
	if v := getenv("KAIRO_TIMEZONE"); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			errs = append(errs, &Error{"KAIRO_TIMEZONE", v, "unbekannte IANA-Zeitzone"})
		} else {
			cfg.Location = loc
		}
	}

	cfg.WorkStart, cfg.WorkEnd = 9*60, 17*60
	if v := getenv("KAIRO_WORK_START"); v != "" {
		if m, ok := parseClock(v); ok {
			cfg.WorkStart = m
		} else {
			errs = append(errs, &Error{"KAIRO_WORK_START", v, "erwartet HH:MM"})
		}
	}
	if v := getenv("KAIRO_WORK_END"); v != "" {
		if m, ok := parseClock(v); ok {
			cfg.WorkEnd = m
		} else {
			errs = append(errs, &Error{"KAIRO_WORK_END", v, "erwartet HH:MM"})
		}
	}
	if cfg.WorkStart >= cfg.WorkEnd {
		errs = append(errs, &Error{"KAIRO_WORK_END", getenv("KAIRO_WORK_END"), "muss nach KAIRO_WORK_START liegen"})
	}

	if v := getenv("KAIRO_LOG_LEVEL"); v != "" {
		switch strings.ToLower(v) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			errs = append(errs, &Error{"KAIRO_LOG_LEVEL", v, "erwartet debug|info|warn|error"})
		}
	}

	return cfg, errors.Join(errs...)
}

// parseClock liest HH:MM (24 h) als Minuten seit Mitternacht.
func parseClock(v string) (int, bool) {
	t, err := time.Parse("15:04", v)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}
