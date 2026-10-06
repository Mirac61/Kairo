// Package launchd richtet das Backend als macOS-LaunchAgent ein (Autostart beim Login).
package launchd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Label ist die Kennung des LaunchAgents.
const Label = "app.kairo.backend"

// Plist erzeugt die Property-List des Agents. env sind die Umgebungsvariablen
// (KAIRO_*), die der Agent beim Start bekommt.
func Plist(binary, logPath string, env map[string]string) []byte {
	var b bytes.Buffer
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + Label + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + esc(binary) + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>` + esc(logPath) + `</string>
	<key>StandardErrorPath</key>
	<string>` + esc(logPath) + `</string>
`)
	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n", esc(k), esc(env[k]))
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

// KairoEnv sammelt die KAIRO_*-Variablen aus environ (Einträge "K=V").
func KairoEnv(environ []string) map[string]string {
	out := map[string]string{}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, "KAIRO_") && v != "" {
			out[k] = v
		}
	}
	return out
}

// Paths liefert den Pfad der Plist und der Log-Datei.
func Paths(home string) (plist, log string) {
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"),
		filepath.Join(home, "Library", "Logs", "kairo.log")
}

// Install schreibt die Plist und lädt den Agent. Er startet sofort und bei jedem Login.
func Install(binary string, environ []string) (plistPath string, err error) {
	if strings.Contains(binary, "go-build") {
		return "", fmt.Errorf("%s ist ein temporäres Binary von go run; zuerst bauen (go build -o ~/.local/bin/kairo ./cmd/server) und von dort aus installieren", binary)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	plistPath, logPath := Paths(home)
	for _, dir := range []string{filepath.Dir(plistPath), filepath.Dir(logPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(plistPath, Plist(binary, logPath, KairoEnv(environ)), 0o644); err != nil {
		return "", err
	}
	_ = launchctl("bootout", domain()+"/"+Label) // ein alter Agent darf fehlen
	if err := launchctl("bootstrap", domain(), plistPath); err != nil {
		return plistPath, err
	}
	return plistPath, nil
}

// Uninstall stoppt den Agent und löscht die Plist. Ist er nicht eingerichtet, ist das kein Fehler.
func Uninstall() (plistPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	plistPath, _ = Paths(home)
	_ = launchctl("bootout", domain()+"/"+Label)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return plistPath, err
	}
	return plistPath, nil
}

func domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func launchctl(args ...string) error {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
