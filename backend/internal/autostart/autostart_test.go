package autostart

import (
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistIsValidXMLWithEnvAndEscaping(t *testing.T) {
	p := string(Plist("/Users/x/My & Bin/kairo", "/Users/x/Library/Logs/kairo.log",
		map[string]string{"KAIRO_PORT": "9000", "KAIRO_DB_PATH": "/a<b>/k.db"}))
	dec := xml.NewDecoder(strings.NewReader(p))
	dec.Strict = false
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("kein gültiges XML: %v\n%s", err, p)
		}
	}
	for _, want := range []string{
		"<string>" + Label + "</string>", "My &amp; Bin", "/a&lt;b&gt;/k.db",
		"<key>RunAtLoad</key>\n\t<true/>", "<key>KeepAlive</key>", "<key>KAIRO_PORT</key>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("Plist enthält %q nicht:\n%s", want, p)
		}
	}
	if strings.Index(p, "KAIRO_DB_PATH") > strings.Index(p, "KAIRO_PORT") {
		t.Error("Umgebungsvariablen nicht sortiert")
	}
}

func TestPlistWithoutEnvHasNoEnvironmentVariables(t *testing.T) {
	if p := string(Plist("/bin/kairo", "/log", nil)); strings.Contains(p, "EnvironmentVariables") {
		t.Errorf("leere Umgebung erzeugt EnvironmentVariables:\n%s", p)
	}
}

func TestKairoEnvOnlyTakesSetKairoVariables(t *testing.T) {
	got := KairoEnv([]string{"PATH=/bin", "KAIRO_PORT=9000", "KAIRO_TIMEZONE=", "KAIRO_X=a=b", "KAIRO"})
	if len(got) != 2 || got["KAIRO_PORT"] != "9000" || got["KAIRO_X"] != "a=b" {
		t.Errorf("KairoEnv = %v", got)
	}
}

func TestInstallRefusesGoRunBinary(t *testing.T) {
	if _, _, err := Install("/var/folders/x/T/go-build123/b001/exe/server", nil); err == nil {
		t.Error("Binary von go run wurde akzeptiert")
	}
}

func TestPaths(t *testing.T) {
	plist, log := launchdPaths("/Users/x")
	if plist != filepath.FromSlash("/Users/x/Library/LaunchAgents/app.kairo.backend.plist") || log != filepath.FromSlash("/Users/x/Library/Logs/kairo.log") {
		t.Errorf("Paths = %s, %s", plist, log)
	}
}

func TestUnitQuotesAndEnv(t *testing.T) {
	u := string(Unit(`/home/x/my bin/kairo`, map[string]string{"KAIRO_PORT": "9000", "KAIRO_DB_PATH": `/a"b/50%.db`}))
	for _, want := range []string{
		`ExecStart="/home/x/my bin/kairo"`,
		`Environment="KAIRO_DB_PATH=/a\"b/50%%.db"` + "\nEnvironment=\"KAIRO_PORT=9000\"",
		"Restart=always", "WantedBy=default.target",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("fehlt %q in\n%s", want, u)
		}
	}
}

func TestScriptQuotesPaths(t *testing.T) {
	s := string(Script(`C:\Program Files\kairo.exe`, `C:\Users\x\kairo.log`, map[string]string{"KAIRO_PORT": "9000"}))
	for _, want := range []string{
		`sh.Environment("Process")("KAIRO_PORT") = "9000"`,
		`sh.Run "cmd /c """"C:\Program Files\kairo.exe"" >> ""C:\Users\x\kairo.log"" 2>&1""", 0, False`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("fehlt %q in\n%s", want, s)
		}
	}
}
