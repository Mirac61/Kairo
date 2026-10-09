package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"kairo/internal/config"
)

func TestSettings(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{"KAIRO_CONFIG_PATH": filepath.Join(dir, "config.json"), "KAIRO_TIMEZONE": "UTC"}
	getenv := func(k string) string { return env[k] }
	running, err := config.Load(getenv)
	if err != nil {
		t.Fatal(err)
	}
	restarted := false
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Settings: &Settings{Running: running, Getenv: getenv, Restart: func() { restarted = true }},
	})

	var got struct {
		Settings      config.File       `json:"settings"`
		Env           map[string]string `json:"env"`
		RestartNeeded bool              `json:"restart_needed"`
	}
	read := func(code int, body string) {
		t.Helper()
		if code != http.StatusOK {
			t.Fatalf("Status %d: %s", code, body)
		}
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatal(err)
		}
	}

	rec := call(h, "GET", "/api/settings", "")
	read(rec.Code, rec.Body.String())
	if got.RestartNeeded || got.Env["timezone"] != "KAIRO_TIMEZONE" || got.Settings.WorkStart != "09:00" {
		t.Fatalf("GET = %+v", got)
	}

	// Ein fehlender Notizordner wird abgelehnt und nichts geschrieben.
	if rec := call(h, "PUT", "/api/settings", `{"notesDir":`+strconv.Quote(filepath.Join(dir, "fehlt"))+`}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT fehlender Ordner = %d", rec.Code)
	}
	if rec := call(h, "PUT", "/api/settings", `{"workStart":"18:00"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT 18:00 = %d", rec.Code)
	}

	rec = call(h, "PUT", "/api/settings", `{"notesDir":`+strconv.Quote(dir)+`,"workStart":"08:00","workEnd":"16:30"}`)
	read(rec.Code, rec.Body.String())
	if !got.RestartNeeded || got.Settings.NotesDir != dir || got.Settings.WorkEnd != "16:30" || got.Settings.Timezone != "UTC" {
		t.Fatalf("PUT = %+v", got)
	}

	if rec := call(h, "POST", "/api/restart", ""); rec.Code != http.StatusAccepted || !restarted {
		t.Fatalf("restart = %d, %v", rec.Code, restarted)
	}
}
