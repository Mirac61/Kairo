package auth

import "errors"

// Öffentliche Fehler, die an den Client gehen dürfen.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSessionNotFound    = errors.New("session expired or invalid")
	ErrUserNotFound       = errors.New("user not found")
	// ErrRegistrationClosed: Es gibt schon ein Konto, /setup ist nur für das erste.
	ErrRegistrationClosed = errors.New("registration closed")
	ErrPasswordTooShort   = errors.New("password must have at least 8 characters")
)

// Interne Fehler, die nicht an den Client gehen.
var (
	ErrMismatchedPassword = errors.New("auth: password does not match hash")
	ErrInvalidHash        = errors.New("auth: hash is not in PHC format")
)

// IsMismatched sagt, ob ein Fehler ein nicht passendes Passwort bedeutet.
func IsMismatched(err error) bool { return errors.Is(err, ErrMismatchedPassword) }

// IsInvalidHash sagt, ob ein Fehler ein ungültiges Hash-Format bedeutet.
func IsInvalidHash(err error) bool { return errors.Is(err, ErrInvalidHash) }
