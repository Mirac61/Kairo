// Package ics liest VEVENTs aus iCalendar-Texten (RFC 5545), soweit Kairo sie braucht.
package ics

import (
	"cmp"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Event ist ein VEVENT in Kairos Begriffen.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Start, End  time.Time // UTC; ganztägige Termine: Mitternacht in loc, End exklusiv
	Rule        string    // RRULE-Wert ohne "RRULE:", leer = Einzeltermin; nicht auf Unterstützung geprüft
	Exdates     []string  // ausgelassene Tage in loc, YYYY-MM-DD
	Note        string    // Hinweis zu diesem Termin, sonst leer
}

// Parse liest alle VEVENTs aus data. Floating-Zeiten und Tage ohne Zeitzone gelten in loc.
// skipped nennt je übersprungenem VEVENT den Grund. Fehler nur, wenn data kein iCalendar ist.
func Parse(data string, loc *time.Location) (events []Event, skipped []string, err error) {
	if !strings.Contains(strings.ToUpper(data), "BEGIN:VCALENDAR") {
		return nil, nil, errors.New("keine iCalendar-Datei (BEGIN:VCALENDAR fehlt)")
	}
	var (
		cur     []prop // Eigenschaften des laufenden VEVENT
		inEvent bool
		nested  int // Tiefe verschachtelter Komponenten im VEVENT (VALARM)
	)
	for _, line := range unfold(data) {
		p, ok := parseLine(line)
		switch {
		case !ok:
		case p.name == "BEGIN" && !inEvent && strings.EqualFold(p.value, "VEVENT"):
			inEvent, nested, cur = true, 0, nil
		case !inEvent:
		case p.name == "BEGIN":
			nested++
		case p.name == "END" && nested > 0:
			nested--
		case p.name == "END":
			inEvent = false
			if ev, err := buildEvent(cur, loc); err != nil {
				skipped = append(skipped, err.Error())
			} else {
				events = append(events, ev)
			}
		case nested == 0:
			cur = append(cur, p)
		}
	}
	return events, skipped, nil
}

type prop struct {
	name   string // groß geschrieben
	params map[string]string
	value  string
}

// unfold löst die Zeilenfaltung auf (Fortsetzungszeile beginnt mit Leerzeichen oder Tab).
func unfold(data string) []string {
	var lines []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
	}
	for _, l := range strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n") {
		if l != "" && (l[0] == ' ' || l[0] == '\t') {
			cur.WriteString(l[1:])
			continue
		}
		flush()
		cur.WriteString(strings.TrimSuffix(l, "\r"))
	}
	flush()
	return lines
}

// parseLine zerlegt "NAME;PARAM=V:wert". Der Doppelpunkt in Anführungszeichen trennt nicht.
func parseLine(l string) (prop, bool) {
	quoted, i := false, -1
	for j := 0; j < len(l) && i < 0; j++ {
		switch l[j] {
		case '"':
			quoted = !quoted
		case ':':
			if !quoted {
				i = j
			}
		}
	}
	if i < 0 {
		return prop{}, false
	}
	parts := strings.Split(l[:i], ";")
	p := prop{name: strings.ToUpper(strings.TrimSpace(parts[0])), params: map[string]string{}, value: l[i+1:]}
	for _, kv := range parts[1:] {
		k, v, _ := strings.Cut(kv, "=")
		p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
	}
	return p, true
}

var unescape = strings.NewReplacer(`\\`, `\`, `\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";")

// dt ist ein gelesener DATE oder DATE-TIME.
type dt struct {
	t     time.Time
	date  bool   // nur Datum (ganztägig), t ist Mitternacht in loc
	badTZ string // unbekannte TZID; t wurde in loc gelesen
}

func parseDT(p prop, loc *time.Location) (dt, error) {
	v := strings.TrimSpace(p.value)
	switch {
	case strings.EqualFold(p.params["VALUE"], "DATE") || len(v) == 8:
		t, err := time.ParseInLocation("20060102", v, loc)
		return dt{t: t, date: true}, err
	case strings.HasSuffix(v, "Z"):
		t, err := time.Parse("20060102T150405Z", v)
		return dt{t: t}, err
	}
	zone, bad := loc, ""
	if id := p.params["TZID"]; id != "" {
		if z, err := time.LoadLocation(id); err == nil {
			zone = z
		} else {
			bad = id
		}
	}
	t, err := time.ParseInLocation("20060102T150405", v, zone)
	return dt{t: t, badTZ: bad}, err
}

func buildEvent(props []prop, loc *time.Location) (Event, error) {
	var (
		ev                           Event
		dtstart, dtend               *prop
		exdates                      []prop
		cancelled, overridesInstance bool
	)
	for i, p := range props {
		switch p.name {
		case "UID":
			ev.UID = strings.TrimSpace(p.value)
		case "SUMMARY":
			ev.Summary = strings.TrimSpace(unescape.Replace(p.value))
		case "DESCRIPTION":
			ev.Description = unescape.Replace(p.value)
		case "LOCATION":
			ev.Location = strings.TrimSpace(strings.ReplaceAll(unescape.Replace(p.value), "\n", ", "))
		case "DTSTART":
			dtstart = &props[i]
		case "DTEND":
			dtend = &props[i]
		case "RRULE":
			ev.Rule = dropDefaultWKST(p.value)
		case "EXDATE":
			exdates = append(exdates, p)
		case "STATUS":
			cancelled = strings.EqualFold(strings.TrimSpace(p.value), "CANCELLED")
		case "RECURRENCE-ID":
			overridesInstance = true
		}
	}
	name := ev.Summary
	if name == "" {
		name = ev.UID
	}
	switch {
	case ev.UID == "":
		return Event{}, fmt.Errorf("Termin %q ohne UID übersprungen", name)
	case overridesInstance:
		return Event{}, fmt.Errorf("%q: geänderte einzelne Wiederholung (RECURRENCE-ID) wird nicht unterstützt", name)
	case cancelled:
		return Event{}, fmt.Errorf("%q: abgesagt (STATUS:CANCELLED)", name)
	case dtstart == nil:
		return Event{}, fmt.Errorf("%q: DTSTART fehlt", name)
	}
	start, err := parseDT(*dtstart, loc)
	if err != nil {
		return Event{}, fmt.Errorf("%q: DTSTART %q ist ungültig", name, dtstart.value)
	}
	end := dt{t: start.t.AddDate(0, 0, 1), date: true}
	switch {
	case dtend != nil:
		if end, err = parseDT(*dtend, loc); err != nil {
			return Event{}, fmt.Errorf("%q: DTEND %q ist ungültig", name, dtend.value)
		}
	case !start.date:
		return Event{}, fmt.Errorf("%q: DTEND fehlt", name)
	}
	if !end.t.After(start.t) {
		return Event{}, fmt.Errorf("%q: Ende liegt nicht nach dem Beginn", name)
	}
	ev.Start, ev.End = start.t.UTC(), end.t.UTC()
	if bad := cmp.Or(start.badTZ, end.badTZ); bad != "" {
		ev.Note = fmt.Sprintf("%q: Zeitzone %q unbekannt, Serverzeitzone verwendet", name, bad)
	}
	for _, p := range exdates {
		for _, v := range strings.Split(p.value, ",") {
			q := p
			q.value = v
			if d, err := parseDT(q, loc); err == nil {
				ev.Exdates = append(ev.Exdates, d.t.In(loc).Format("2006-01-02"))
			}
		}
	}
	return ev, nil
}

// dropDefaultWKST entfernt WKST=MO, den Vorgabewert (Kairos Regeln rechnen Wochen ab Montag).
func dropDefaultWKST(rule string) string {
	var keep []string
	for _, part := range strings.Split(strings.TrimSpace(rule), ";") {
		if !strings.EqualFold(part, "WKST=MO") {
			keep = append(keep, part)
		}
	}
	return strings.Join(keep, ";")
}
