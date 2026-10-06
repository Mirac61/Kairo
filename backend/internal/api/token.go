package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LoadOrCreateToken liest das lokale API-Token oder erzeugt es beim ersten Start
// (32 Zufallsbytes hex, Verzeichnis 0700, Datei 0600).
func LoadOrCreateToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return "", fmt.Errorf("api: Token-Datei %s ist leer", path)
		}
		return tok, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("api: Token lesen: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("api: Token-Verzeichnis: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("api: Zufall: %w", err)
	}
	tok := hex.EncodeToString(raw)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("api: Token schreiben: %w", err)
	}
	if _, err := f.WriteString(tok + "\n"); err != nil {
		f.Close()
		return "", fmt.Errorf("api: Token schreiben: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return tok, nil
}
