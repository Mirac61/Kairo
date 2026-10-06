// Package domain enthält die reinen Geschäftsobjekte, ohne HTTP- oder DB-Logik.
package domain

import "errors"

var (
	// ErrNotFound: das angefragte Objekt existiert nicht.
	ErrNotFound = errors.New("nicht gefunden")
	// ErrConflict: die Aktion widerspricht dem aktuellen Zustand (z. B. zweite
	// Completion am selben Tag, Löschen einer Task mit Zeiteinträgen).
	ErrConflict = errors.New("Konflikt")
	// ErrInvalid: die Eingabe verletzt eine Regel. Die Fehlermeldung nennt den Grund.
	ErrInvalid = errors.New("ungültige Eingabe")
)
