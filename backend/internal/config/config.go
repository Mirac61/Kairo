// Package config liest die Kairo-Konfiguration aus der Umgebung.
package config

import (
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
	Location  *time.Location
	LogLevel  slog.Level
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

// Load liest die Konfiguration über getenv (z. B. os.Getenv).
func Load(getenv func(string) string) (Config, error) {
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
