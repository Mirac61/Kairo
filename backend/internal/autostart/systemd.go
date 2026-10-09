package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Linux: systemd-User-Unit unter ~/.config/systemd/user. Log: journalctl --user -u kairo.

const unitName = "kairo.service"

// Unit erzeugt die systemd-Unit; Restart=always hält das Backend am Laufen (wie KeepAlive unter macOS).
func Unit(binary string, env map[string]string) []byte {
	// systemd liest Anführungszeichen und Backslashes in Werten; % leitet Platzhalter ein.
	q := func(s string) string {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(s) + `"`
	}
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=Kairo\n\n[Service]\nExecStart=" + q(binary) + "\n")
	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "Environment=%s\n", q(k+"="+env[k]))
	}
	b.WriteString("Restart=always\n\n[Install]\nWantedBy=default.target\n")
	return []byte(b.String())
}

func unitPath(home string) string { return filepath.Join(home, ".config", "systemd", "user", unitName) }

func installSystemd(home, binary string, env map[string]string) (string, string, error) {
	path := unitPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, Unit(binary, env), 0o644); err != nil {
		return "", "", err
	}
	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return path, "", err
	}
	return path, "journalctl --user -u kairo", run("systemctl", "--user", "enable", "--now", unitName)
}

func uninstallSystemd(home string) (string, error) {
	path := unitPath(home)
	_ = run("systemctl", "--user", "disable", "--now", unitName)
	if err := removeIfExists(path); err != nil {
		return path, err
	}
	_ = run("systemctl", "--user", "daemon-reload")
	return path, nil
}
