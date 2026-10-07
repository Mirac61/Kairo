package domain

import "time"

// CalendarEvent ist ein fester Termin. Er ist nicht automatisch eine Task.
// Mit RecurrenceRule steht StartAt/EndAt für die erste Wiederholung.
type CalendarEvent struct {
	ID                string
	Title             string
	Description       string
	StartAt           time.Time // UTC
	EndAt             time.Time // UTC, nach StartAt
	Location          string
	URL               string
	ProjectID         *string
	TaskID            *string
	RecurrenceRule    *string  // RRULE-Teilmenge, siehe ParseRule
	RecurrenceExdates []string // ausgelassene lokale Tage, YYYY-MM-DD, aufsteigend
	ExternalUID       *string  // UID aus einem ICS-Import, sonst nil
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time // UTC; nur in der Papierkorb-Liste gesetzt
}

// EventOccurrence ist ein konkreter Termin eines (ggf. wiederkehrenden) Events.
type EventOccurrence struct {
	Start time.Time
	End   time.Time
}

// EventInstance ist ein konkreter Termin zusammen mit seinem Event.
type EventInstance struct {
	Event CalendarEvent
	Start time.Time // UTC
	End   time.Time // UTC
}
