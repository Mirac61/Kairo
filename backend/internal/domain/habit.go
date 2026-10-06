package domain

import (
	"fmt"
	"sort"
	"time"
)

// FrequencyType bestimmt, an welchen Tagen ein Habit fällig ist.
type FrequencyType string

const (
	FreqDaily            FrequencyType = "DAILY"
	FreqWeekly           FrequencyType = "WEEKLY"
	FreqSpecificWeekdays FrequencyType = "SPECIFIC_WEEKDAYS"
	FreqTimesPerWeek     FrequencyType = "TIMES_PER_WEEK"
)

// Valid meldet, ob t ein bekannter Typ ist.
func (t FrequencyType) Valid() bool {
	switch t {
	case FreqDaily, FreqWeekly, FreqSpecificWeekdays, FreqTimesPerWeek:
		return true
	}
	return false
}

// FrequencyConfig sind die Details zum FrequencyType (als JSON gespeichert).
// DAILY: leer. WEEKLY: Weekday (MO..SU). SPECIFIC_WEEKDAYS: Weekdays.
// TIMES_PER_WEEK: Times (1..7). Weitere Typen kommen mit eigenen Feldern dazu.
type FrequencyConfig struct {
	Weekday  string   `json:"weekday,omitempty"`
	Weekdays []string `json:"weekdays,omitempty"`
	Times    int      `json:"times,omitempty"`
}

var weekdayOrder = []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"}

func weekdayCode(d time.Weekday) string { return weekdayOrder[mondayIndex(d)] }

// ParseDate parst einen lokalen Tag (YYYY-MM-DD).
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.UTC)
}

// NormalizeConfig prüft c gegen t und liefert die kanonische Form (Wochentage
// groß, ohne Doppelte, ab Montag sortiert). Bei WEEKLY ohne Weekday gilt der
// Wochentag von startDate.
func NormalizeConfig(t FrequencyType, c FrequencyConfig, startDate string) (FrequencyConfig, error) {
	bad := func(format string, args ...any) (FrequencyConfig, error) {
		return FrequencyConfig{}, fmt.Errorf("%w: frequency_config: %s", ErrInvalid, fmt.Sprintf(format, args...))
	}
	known := func(code string) bool {
		for _, w := range weekdayOrder {
			if w == code {
				return true
			}
		}
		return false
	}
	switch t {
	case FreqDaily:
		if c.Weekday != "" || len(c.Weekdays) > 0 || c.Times != 0 {
			return bad("DAILY hat keine Einstellungen")
		}
		return FrequencyConfig{}, nil
	case FreqWeekly:
		if len(c.Weekdays) > 0 || c.Times != 0 {
			return bad("WEEKLY kennt nur weekday")
		}
		if c.Weekday == "" {
			d, err := ParseDate(startDate)
			if err != nil {
				return bad("weekday fehlt und start_date ist ungültig")
			}
			return FrequencyConfig{Weekday: weekdayCode(d.Weekday())}, nil
		}
		code := upper(c.Weekday)
		if !known(code) {
			return bad("weekday %q ist ungültig (MO..SU)", c.Weekday)
		}
		return FrequencyConfig{Weekday: code}, nil
	case FreqSpecificWeekdays:
		if c.Weekday != "" || c.Times != 0 {
			return bad("SPECIFIC_WEEKDAYS kennt nur weekdays")
		}
		if len(c.Weekdays) == 0 {
			return bad("weekdays darf nicht leer sein")
		}
		seen := map[string]bool{}
		var out []string
		for _, w := range c.Weekdays {
			code := upper(w)
			if !known(code) {
				return bad("weekdays-Wert %q ist ungültig (MO..SU)", w)
			}
			if !seen[code] {
				seen[code] = true
				out = append(out, code)
			}
		}
		sort.Slice(out, func(i, j int) bool { return indexOf(out[i]) < indexOf(out[j]) })
		return FrequencyConfig{Weekdays: out}, nil
	case FreqTimesPerWeek:
		if c.Weekday != "" || len(c.Weekdays) > 0 {
			return bad("TIMES_PER_WEEK kennt nur times")
		}
		if c.Times < 1 || c.Times > 7 {
			return bad("times muss zwischen 1 und 7 liegen")
		}
		return FrequencyConfig{Times: c.Times}, nil
	}
	return bad("unbekannter Typ %q", t)
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

func indexOf(code string) int {
	for i, w := range weekdayOrder {
		if w == code {
			return i
		}
	}
	return -1
}

// Habit ist eine wiederkehrende Verhaltensregel. Fällige Tage werden aus
// der Regel berechnet (OccurrenceOn), gespeichert werden nur Completions.
type Habit struct {
	ID              string
	Name            string
	Description     string
	FrequencyType   FrequencyType
	FrequencyConfig FrequencyConfig
	TargetValue     *int
	Unit            string
	PreferredTime   *string // lokale Uhrzeit HH:MM
	StartDate       string  // lokaler Tag
	EndDate         *string // lokaler Tag, inklusive
	Active          bool
	CreatedAt       time.Time
}

// HabitCompletion ist das Abhaken eines Habits an einem lokalen Tag.
type HabitCompletion struct {
	ID          string
	HabitID     string
	Date        string
	Value       *int
	CompletedAt time.Time
	Note        string
}

// WeekProgress ist der Wochenfortschritt eines TIMES_PER_WEEK-Habits.
type WeekProgress struct {
	Done   int
	Target int
}

// HabitOccurrence ist ein berechneter fälliger Tag eines Habits.
type HabitOccurrence struct {
	HabitID  string
	Date     string
	Done     bool          // an diesem Tag abgehakt
	Progress *WeekProgress // nur bei TIMES_PER_WEEK
}

// OccurrenceOn berechnet, ob h am Tag date fällig ist. done enthält die
// Completion-Tage des Habits, bei TIMES_PER_WEEK mindestens die ganze
// Woche (Montag bis Sonntag) von date. TIMES_PER_WEEK ist täglich fällig,
// bis das Wochenziel erreicht ist; ein an date abgehakter Tag bleibt sichtbar.
func (h Habit) OccurrenceOn(date string, done map[string]bool) (HabitOccurrence, bool) {
	day, err := ParseDate(date)
	if err != nil || !h.Active || date < h.StartDate || (h.EndDate != nil && date > *h.EndDate) {
		return HabitOccurrence{}, false
	}
	occ := HabitOccurrence{HabitID: h.ID, Date: date, Done: done[date]}
	switch h.FrequencyType {
	case FreqDaily:
		return occ, true
	case FreqWeekly:
		return occ, h.FrequencyConfig.Weekday == weekdayCode(day.Weekday())
	case FreqSpecificWeekdays:
		code := weekdayCode(day.Weekday())
		for _, w := range h.FrequencyConfig.Weekdays {
			if w == code {
				return occ, true
			}
		}
		return occ, false
	case FreqTimesPerWeek:
		monday := day.AddDate(0, 0, -mondayIndex(day.Weekday()))
		n := 0
		for i := 0; i < 7; i++ {
			if done[monday.AddDate(0, 0, i).Format("2006-01-02")] {
				n++
			}
		}
		occ.Progress = &WeekProgress{Done: n, Target: h.FrequencyConfig.Times}
		return occ, n < h.FrequencyConfig.Times || occ.Done
	}
	return HabitOccurrence{}, false
}

// WeekRange liefert Montag und Sonntag der Woche von date.
func WeekRange(date string) (monday, sunday string, err error) {
	day, err := ParseDate(date)
	if err != nil {
		return "", "", fmt.Errorf("%w: Datum muss YYYY-MM-DD sein", ErrInvalid)
	}
	m := day.AddDate(0, 0, -mondayIndex(day.Weekday()))
	return m.Format("2006-01-02"), m.AddDate(0, 0, 6).Format("2006-01-02"), nil
}
