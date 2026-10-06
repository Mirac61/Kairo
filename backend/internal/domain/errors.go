// Package domain enthält die reinen Geschäftsobjekte, ohne HTTP- oder DB-Logik.
package domain

import "errors"

var (
	// ErrNotFound: das angefragte Objekt existiert nicht.
	ErrNotFound = errors.New("nicht gefunden")
	// ErrConflict: das Objekt existiert bereits (z. B. zweite Completion am selben Tag).
	ErrConflict = errors.New("existiert bereits")
	// ErrInvalid: die Eingabe verletzt eine Regel. Die Fehlermeldung nennt den Grund.
	ErrInvalid = errors.New("ungültige Eingabe")
)
