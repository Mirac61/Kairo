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
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EventOccurrence ist ein konkreter Termin eines (ggf. wiederkehrenden) Events.
type EventOccurrence struct {
	Start time.Time
	End   time.Time
}
