package autostart

import (
	"os"
	"path/filepath"
	"strings"
)

// Windows: VBScript im Autostart-Ordner des Nutzers. wscript startet das Backend
// ohne Konsolenfenster und leitet die Ausgabe in eine Log-Datei.

// Script erzeugt das VBScript. env sind die KAIRO_*-Variablen für den Prozess.
func Script(binary, logPath string, env map[string]string) []byte {
	lit := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	var b strings.Builder
	b.WriteString("Set sh = CreateObject(\"WScript.Shell\")\r\n")
	for _, k := range sortedKeys(env) {
		b.WriteString("sh.Environment(\"Process\")(" + lit(k) + ") = " + lit(env[k]) + "\r\n")
	}
	// cmd /c entfernt die äußeren Anführungszeichen, die inneren schützen Leerzeichen in Pfaden.
	cmd := `cmd /c ""` + binary + `" >> "` + logPath + `" 2>&1"`
	b.WriteString("sh.Run " + lit(cmd) + ", 0, False\r\n")
	return []byte(b.String())
}

func startupPaths() (script, log string, err error) {
	appData, err := os.UserConfigDir() // %AppData%
	if err != nil {
		return "", "", err
	}
	local, err := os.UserCacheDir() // %LocalAppData%
	if err != nil {
		return "", "", err
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Kairo.vbs"),
		filepath.Join(local, "kairo", "kairo.log"), nil
}

func installStartup(binary string, env map[string]string) (string, string, error) {
	script, logPath, err := startupPaths()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(script, Script(binary, logPath, env), 0o644); err != nil {
		return "", "", err
	}
	return script, logPath, run("wscript", script)
}

// uninstallStartup löscht nur das Skript; ein laufendes Backend läuft bis zum Abmelden weiter.
func uninstallStartup() (string, error) {
	script, _, err := startupPaths()
	if err != nil {
		return "", err
	}
	return script, removeIfExists(script)
}
