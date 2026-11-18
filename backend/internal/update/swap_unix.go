//go:build !windows

package update

import "os"

// swap ersetzt exe durch tmp. Unter Unix ist das Umbenennen über eine laufende
// Datei erlaubt; der laufende Prozess behält seinen alten Stand bis zum Neustart.
func swap(exe, tmp string) error { return os.Rename(tmp, exe) }
