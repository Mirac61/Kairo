//go:build windows

package update

import "os"

// swap ersetzt exe durch tmp. Windows lässt eine laufende .exe nicht überschreiben,
// aber umbenennen; der Rest heißt exe + ".alt" und wird beim nächsten Update gelöscht.
func swap(exe, tmp string) error {
	old := exe + ".alt"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Rename(old, exe) // lieber den alten Stand zurück als nichts
		return err
	}
	return nil
}
