package ics

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Todo ist ein VTODO in Kairos Begriffen.
type Todo struct {
	UID, Summary, Description string
	Due                       time.Time // UTC
	Status                    string    // NEEDS-ACTION, IN-PROCESS, COMPLETED oder CANCELLED
}

var escape = strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", "")

func stamp(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

// fold bricht eine Zeile nach höchstens 75 Oktetten um, nie mitten in einem Zeichen.
func fold(line string) string {
	var b strings.Builder
	n, limit := 0, 75
	for _, r := range line {
		if n+utf8.RuneLen(r) > limit {
			b.WriteString("\r\n ")
			n, limit = 0, 74
		}
		b.WriteRune(r)
		n += utf8.RuneLen(r)
	}
	return b.String()
}

// Format schreibt Termine und Aufgaben als iCalendar-Text. Ausgelassene Tage
// werden mit der Uhrzeit des Termins in loc zu EXDATE.
func Format(events []Event, todos []Todo, now time.Time, loc *time.Location) string {
	var lines []string
	add := func(s ...string) { lines = append(lines, s...) }
	prop := func(name, v string) {
		if v != "" {
			add(name + ":" + escape.Replace(v))
		}
	}
	add("BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Kairo//Export//DE", "CALSCALE:GREGORIAN")
	for _, e := range events {
		add("BEGIN:VEVENT", "UID:"+e.UID, "DTSTAMP:"+stamp(now), "DTSTART:"+stamp(e.Start), "DTEND:"+stamp(e.End))
		prop("SUMMARY", e.Summary)
		prop("DESCRIPTION", e.Description)
		prop("LOCATION", e.Location)
		if e.Rule != "" {
			add("RRULE:" + e.Rule)
		}
		s := e.Start.In(loc)
		for _, d := range e.Exdates {
			if day, err := time.ParseInLocation("2006-01-02", d, loc); err == nil {
				add("EXDATE:" + stamp(time.Date(day.Year(), day.Month(), day.Day(), s.Hour(), s.Minute(), s.Second(), 0, loc)))
			}
		}
		add("END:VEVENT")
	}
	for _, t := range todos {
		add("BEGIN:VTODO", "UID:"+t.UID, "DTSTAMP:"+stamp(now), "DUE:"+stamp(t.Due), "STATUS:"+t.Status)
		prop("SUMMARY", t.Summary)
		prop("DESCRIPTION", t.Description)
		add("END:VTODO")
	}
	add("END:VCALENDAR")
	for i, l := range lines {
		lines[i] = fold(l)
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}
