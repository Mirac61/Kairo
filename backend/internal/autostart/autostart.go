// Package autostart startet das Backend bei jedem Login: macOS per LaunchAgent,
// Linux per systemd-User-Unit, Windows per Skript im Autostart-Ordner.
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
)

// Install richtet den Autostart für binary ein und startet das Backend sofort.
// Die gesetzten KAIRO_*-Variablen aus environ gelten auch beim Autostart.
// Rückgabe: die geschriebene Datei und wo das Log landet.
func Install(binary string, environ []string) (path, log string, err error) {
	if strings.Contains(binary, "go-build") {
		return "", "", fmt.Errorf("%s ist ein temporäres Binary von go run; zuerst bauen (go build -o ~/.local/bin/kairo ./cmd/server) und von dort aus installieren", binary)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	env := KairoEnv(environ)
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(home, binary, env)
	case "linux":
		return installSystemd(home, binary, env)
	case "windows":
		return installStartup(binary, env)
	}
	return "", "", fmt.Errorf("Autostart gibt es für %s nicht", runtime.GOOS)
}

// Uninstall entfernt den Autostart. Ist er nicht eingerichtet, ist das kein Fehler.
func Uninstall() (path string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd(home)
	case "linux":
		return uninstallSystemd(home)
	case "windows":
		return uninstallStartup()
	}
	return "", fmt.Errorf("Autostart gibt es für %s nicht", runtime.GOOS)
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

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
