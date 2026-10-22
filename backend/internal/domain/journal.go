package domain

import "time"

// JournalTaskImport markiert den Aufgabenimport eines Journaltags.
// Titles sind die importierten Aufgabentitel (auch aus dem Papierkorb), TaskCount ihre Anzahl.
type JournalTaskImport struct {
	JournalDate string
	TaskCount   int
	ImportedAt  time.Time
	Titles      []string
}
