package domain

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID erzeugt eine zufällige ID (128 Bit, hex).
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand schlägt praktisch nie fehl
	}
	return hex.EncodeToString(b[:])
}
