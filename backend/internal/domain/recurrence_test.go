package domain

import (
	"errors"
	"testing"
	"time"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("keine Zeitzonendaten:", err)
	}
	return loc
}

func days(ts []time.Time, loc *time.Location) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.In(loc).Format("2006-01-02")
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseRuleErrors(t *testing.T) {
	for _, s := range []string{
		"", "FREQ", "FREQ=MONTHLY", "INTERVAL=2", "FREQ=DAILY;INTERVAL=0", "FREQ=DAILY;COUNT=x",
		"FREQ=DAILY;COUNT=2;UNTIL=20261231", "FREQ=DAILY;BYDAY=MO", "FREQ=WEEKLY;BYDAY=1MO",
		"FREQ=WEEKLY;BYDAY=XX", "FREQ=DAILY;FREQ=DAILY", "FREQ=DAILY;BYMONTH=1", "FREQ=DAILY;UNTIL=morgen",
	} {
		if _, err := ParseRule(s, time.UTC); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v", s, err)
		}
	}
}

func TestDailyIntervalAndCount(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 10, 5, 8, 30, 0, 0, loc)
	r, err := ParseRule("FREQ=DAILY;INTERVAL=2;COUNT=3", loc)
	if err != nil {
		t.Fatal(err)
	}
	got := days(r.Starts(start, start, start.AddDate(0, 1, 0)), loc)
	if want := []string{"2026-10-05", "2026-10-07", "2026-10-09"}; !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// COUNT zählt ab dem Beginn, auch wenn das Fenster später liegt.
	got = days(r.Starts(start, start.AddDate(0, 0, 2), start.AddDate(0, 1, 0)), loc)
	if want := []string{"2026-10-07", "2026-10-09"}; !equal(got, want) {
		t.Errorf("Fenster: got %v, want %v", got, want)
	}
}

func TestWeeklyByDay(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, loc) // Mittwoch
	r, _ := ParseRule("FREQ=WEEKLY;BYDAY=WE,MO,FR;COUNT=5", loc)
	got := days(r.Starts(start, start, start.AddDate(1, 0, 0)), loc)
	// Mo der Startwoche (05.10.) liegt vor dem Start und zählt nicht.
	if want := []string{"2026-10-07", "2026-10-09", "2026-10-12", "2026-10-14", "2026-10-16"}; !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	r, _ = ParseRule("FREQ=WEEKLY;INTERVAL=2", loc) // Wochentag des Starts
	got = days(r.Starts(start, start, start.AddDate(0, 1, 0)), loc)
	if want := []string{"2026-10-07", "2026-10-21", "2026-11-04"}; !equal(got, want) {
		t.Errorf("INTERVAL: got %v, want %v", got, want)
	}
}

func TestUntilIsInclusiveEndOfDay(t *testing.T) {
	loc := berlin(t)
	start := time.Date(2026, 10, 5, 22, 0, 0, 0, loc)
	r, _ := ParseRule("FREQ=DAILY;UNTIL=20261007", loc)
	got := days(r.Starts(start, start, start.AddDate(0, 1, 0)), loc)
	if want := []string{"2026-10-05", "2026-10-06", "2026-10-07"}; !equal(got, want) {
		t.Errorf("Datum: got %v, want %v", got, want)
	}
	r, _ = ParseRule("FREQ=DAILY;UNTIL=20261007T200000Z", loc) // 22:00 Berlin = 20:00Z
	got = days(r.Starts(start, start, start.AddDate(0, 1, 0)), loc)
	if want := []string{"2026-10-05", "2026-10-06", "2026-10-07"}; !equal(got, want) {
		t.Errorf("Zeitpunkt: got %v, want %v", got, want)
	}
}

func TestLocalTimeSurvivesDST(t *testing.T) {
	loc := berlin(t)
	start := time.Date(2026, 10, 24, 8, 30, 0, 0, loc) // Zeitumstellung am 25.10.
	r, _ := ParseRule("FREQ=DAILY;COUNT=3", loc)
	got := r.Starts(start, start, start.AddDate(0, 0, 10))
	if len(got) != 3 {
		t.Fatalf("%d Termine", len(got))
	}
	for _, s := range got {
		if l := s.In(loc); l.Hour() != 8 || l.Minute() != 30 {
			t.Errorf("Ortszeit verrutscht: %s", l)
		}
	}
	if got[0].UTC().Hour() == got[2].UTC().Hour() {
		t.Error("UTC-Stunde hätte sich über die Umstellung ändern müssen")
	}
}

func TestSkipAheadMatchesFullRun(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 1, 7, 9, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 400)
	for _, rule := range []string{"FREQ=DAILY", "FREQ=DAILY;INTERVAL=3", "FREQ=WEEKLY;BYDAY=MO,WE", "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,SU"} {
		r, _ := ParseRule(rule, loc)
		full := r.Starts(start, start, end)
		from := start.AddDate(0, 0, 200)
		var want []time.Time
		for _, s := range full {
			if !s.Before(from) {
				want = append(want, s)
			}
		}
		got := r.Starts(start, from, end)
		if len(got) != len(want) {
			t.Errorf("%s: %d statt %d Termine", rule, len(got), len(want))
			continue
		}
		for i := range got {
			if !got[i].Equal(want[i]) {
				t.Errorf("%s: [%d] %s statt %s", rule, i, got[i], want[i])
			}
		}
	}
}
