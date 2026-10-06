package launchd

import (
	"encoding/xml"
	"io"
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
	if _, err := Install("/var/folders/x/T/go-build123/b001/exe/server", nil); err == nil {
		t.Error("Binary von go run wurde akzeptiert")
	}
}

func TestPaths(t *testing.T) {
	plist, log := Paths("/Users/x")
	if plist != "/Users/x/Library/LaunchAgents/app.kairo.backend.plist" || log != "/Users/x/Library/Logs/kairo.log" {
		t.Errorf("Paths = %s, %s", plist, log)
	}
}
