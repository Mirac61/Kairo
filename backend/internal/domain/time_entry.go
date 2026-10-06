package domain

import "time"

// TimeSource sagt, woher ein Zeiteintrag stammt.
type TimeSource string

const (
	SourceManual    TimeSource = "MANUAL"
	SourceVSCodium  TimeSource = "VSCODIUM"
	SourceAutomatic TimeSource = "AUTOMATIC"
)

// Valid meldet, ob s eine bekannte Quelle ist.
func (s TimeSource) Valid() bool {
	switch s {
	case SourceManual, SourceVSCodium, SourceAutomatic:
		return true
	}
	return false
}

// TimeEntry ist ein Zeitintervall echter Arbeit. Ohne EndedAt läuft der Timer.
//
// Gespeichert wird entweder TaskID oder ProjectID. Beim Lesen ist ProjectID
// das wirksame Projekt (bei Task-Einträgen das Projekt der Task) und kann
// nil sein, wenn die Task zu keinem Projekt gehört.
type TimeEntry struct {
	ID        string
	TaskID    *string
	ProjectID *string
	StartedAt time.Time  // UTC
	EndedAt   *time.Time // UTC
	Source    TimeSource
}

// TimeEntryFilter schränkt die Liste der Zeiteinträge ein. Leere Felder
// filtern nicht. From und To begrenzen StartedAt (From inklusive, To exklusive).
type TimeEntryFilter struct {
	TaskID    string
	ProjectID string // wirksames Projekt, siehe TimeEntry
	From      *time.Time
	To        *time.Time
	Running   bool
}

// TimerResult ist das Ergebnis von Start, Pause und Abschluss einer Task:
// die geänderte Task und der betroffene Zeiteintrag (nil, wenn keiner
// betroffen war).
type TimerResult struct {
	Task  Task
	Entry *TimeEntry
}
