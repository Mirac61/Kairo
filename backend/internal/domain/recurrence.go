package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxPeriods begrenzt die Schleife in Rule.Starts gegen Endlosläufe.
const maxPeriods = 100000

var weekdayCodes = map[string]time.Weekday{
	"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday,
	"FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday,
}

// Rule ist eine wiederkehrende Regel (RRULE-Teilmenge aus RFC 5545):
// FREQ=DAILY|WEEKLY, INTERVAL, BYDAY (nur WEEKLY, ohne Zahlenpräfix),
// COUNT oder UNTIL (nicht beides).
type Rule struct {
	Weekly   bool
	Interval int
	ByDay    []time.Weekday // aufsteigend ab Montag, leer = Wochentag des Starts
	Count    int            // 0 = unbegrenzt
	Until    *time.Time     // inklusive Obergrenze für den Beginn einer Wiederholung
}

func invalidRule(format string, args ...any) error {
	return fmt.Errorf("%w: recurrence_rule: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// ParseRule parst eine Regel wie "FREQ=WEEKLY;BYDAY=MO,WE;COUNT=10".
// Ein UNTIL ohne Uhrzeit gilt bis zum Ende dieses Tages in loc.
func ParseRule(s string, loc *time.Location) (Rule, error) {
	r := Rule{Interval: 1}
	seen := map[string]bool{}
	var haveFreq bool
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(part, "=")
		k = strings.ToUpper(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if !ok || v == "" {
			return Rule{}, invalidRule("Teil %q ist nicht KEY=VALUE", part)
		}
		if seen[k] {
			return Rule{}, invalidRule("%s kommt doppelt vor", k)
		}
		seen[k] = true
		switch k {
		case "FREQ":
			switch strings.ToUpper(v) {
			case "DAILY":
			case "WEEKLY":
				r.Weekly = true
			default:
				return Rule{}, invalidRule("FREQ %q wird nicht unterstützt (DAILY, WEEKLY)", v)
			}
			haveFreq = true
		case "INTERVAL":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return Rule{}, invalidRule("INTERVAL muss eine Zahl ≥ 1 sein")
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return Rule{}, invalidRule("COUNT muss eine Zahl ≥ 1 sein")
			}
			r.Count = n
		case "UNTIL":
			u, err := parseUntil(v, loc)
			if err != nil {
				return Rule{}, err
			}
			r.Until = &u
		case "BYDAY":
			days := map[time.Weekday]bool{}
			for _, code := range strings.Split(v, ",") {
				d, ok := weekdayCodes[strings.ToUpper(strings.TrimSpace(code))]
				if !ok {
					return Rule{}, invalidRule("BYDAY-Wert %q ist ungültig (MO..SU)", code)
				}
				days[d] = true
			}
			for d := range days {
				r.ByDay = append(r.ByDay, d)
			}
			sort.Slice(r.ByDay, func(i, j int) bool { return mondayIndex(r.ByDay[i]) < mondayIndex(r.ByDay[j]) })
		default:
			return Rule{}, invalidRule("%s wird nicht unterstützt", k)
		}
	}
	switch {
	case !haveFreq:
		return Rule{}, invalidRule("FREQ fehlt")
	case r.Count > 0 && r.Until != nil:
		return Rule{}, invalidRule("COUNT und UNTIL schließen sich aus")
	case len(r.ByDay) > 0 && !r.Weekly:
		return Rule{}, invalidRule("BYDAY geht nur mit FREQ=WEEKLY")
	}
	return r, nil
}

func parseUntil(v string, loc *time.Location) (time.Time, error) {
	if t, err := time.ParseInLocation("20060102", v, loc); err == nil {
		y, m, d := t.Date()
		return time.Date(y, m, d, 23, 59, 59, 0, loc), nil
	}
	if t, err := time.Parse("20060102T150405Z", v); err == nil {
		return t, nil
	}
	return time.Time{}, invalidRule("UNTIL %q muss YYYYMMDD oder YYYYMMDDTHHMMSSZ sein", v)
}

func mondayIndex(d time.Weekday) int { return (int(d) + 6) % 7 }

// Starts liefert die Beginnzeiten der Wiederholungen mit from <= t < to,
// aufsteigend. dtstart ist die erste Wiederholung; seine Location bestimmt
// die Ortszeit (die Uhrzeit bleibt über Zeitumstellungen gleich). Ausnahmen
// (EXDATE) zählen hier noch mit, wie in RFC 5545 bei COUNT.
func (r Rule) Starts(dtstart, from, to time.Time) []time.Time {
	loc := dtstart.Location()
	y, m, d := dtstart.Date()
	h, mi, s := dtstart.Clock()
	at := func(dayOffset int) time.Time { return time.Date(y, m, d+dayOffset, h, mi, s, 0, loc) }

	var out []time.Time
	n := 0
	emit := func(t time.Time) bool { // false = fertig
		if r.Until != nil && t.After(*r.Until) {
			return false
		}
		if r.Count > 0 && n >= r.Count {
			return false
		}
		n++
		if !t.Before(to) {
			return false
		}
		if !t.Before(from) {
			out = append(out, t)
		}
		return true
	}

	// Ohne COUNT muss nicht von vorn gezählt werden: bis kurz vor from springen.
	skipDays := 0
	if r.Count == 0 && from.After(dtstart) {
		skipDays = int(from.Sub(dtstart) / (24 * time.Hour))
	}

	if !r.Weekly {
		for k := max(0, skipDays/r.Interval-1); k < maxPeriods; k++ {
			if !emit(at(k * r.Interval)) {
				break
			}
		}
		return out
	}

	days := r.ByDay
	if len(days) == 0 {
		days = []time.Weekday{dtstart.Weekday()}
	}
	monday := -mondayIndex(dtstart.Weekday()) // Tagesoffset zum Montag der Startwoche
	for w := max(0, skipDays/(7*r.Interval)-1); w < maxPeriods; w++ {
		base := monday + w*7*r.Interval
		for _, wd := range days {
			off := base + mondayIndex(wd)
			if off < 0 {
				continue // vor dem Start in der ersten Woche
			}
			if !emit(at(off)) {
				return out
			}
		}
	}
	return out
}
