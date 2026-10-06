package config

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
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
	if cfg.Addr() != "127.0.0.1:8742" {
		t.Errorf("Addr = %s", cfg.Addr())
	}
}

func TestLoadValues(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"KAIRO_PORT": "9000", "KAIRO_DB_PATH": "/tmp/x.db", "KAIRO_TOKEN_PATH": "/tmp/t",
		"KAIRO_TIMEZONE": "Europe/Berlin", "KAIRO_LOG_LEVEL": "DEBUG",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 || cfg.DBPath != "/tmp/x.db" || cfg.TokenPath != "/tmp/t" ||
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
