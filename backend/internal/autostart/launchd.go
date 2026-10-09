package autostart

// macOS: LaunchAgent unter ~/Library/LaunchAgents.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// launchdPaths liefert den Pfad der Plist und der Log-Datei.
func launchdPaths(home string) (plist, log string) {
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"),
		filepath.Join(home, "Library", "Logs", "kairo.log")
}

// installLaunchd schreibt die Plist und lädt den Agent. Er startet sofort und bei jedem Login.
func installLaunchd(home, binary string, env map[string]string) (plistPath, logPath string, err error) {
	plistPath, logPath = launchdPaths(home)
	for _, dir := range []string{filepath.Dir(plistPath), filepath.Dir(logPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", "", err
		}
	}
	if err := os.WriteFile(plistPath, Plist(binary, logPath, env), 0o644); err != nil {
		return "", "", err
	}
	_ = run("launchctl", "bootout", domain()+"/"+Label) // ein alter Agent darf fehlen
	return plistPath, logPath, run("launchctl", "bootstrap", domain(), plistPath)
}

// uninstallLaunchd stoppt den Agent und löscht die Plist.
func uninstallLaunchd(home string) (string, error) {
	plistPath, _ := launchdPaths(home)
	_ = run("launchctl", "bootout", domain()+"/"+Label)
	return plistPath, removeIfExists(plistPath)
}

func domain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }
