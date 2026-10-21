package domain

import "time"

// JournalTaskImport markiert den einmaligen Aufgabenimport eines Journaltags.
type JournalTaskImport struct {
	JournalDate string
	TaskCount   int
	ImportedAt  time.Time
}
