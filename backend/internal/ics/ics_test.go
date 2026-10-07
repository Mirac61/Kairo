package ics

import (
	"reflect"
	"strings"
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

func calendar(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func vevent(lines ...string) string {
	return "BEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseSingleEvent(t *testing.T) {
	data := calendar(vevent(
		"UID:a1@uni",
		"SUMMARY:Arzt\\, Dr. Müller",
		"LOCATION:Praxis\\nHauptstr. 1",
		"DTSTART;TZID=Europe/Berlin:20261009T140000",
		"DTEND;TZID=Europe/Berlin:20261009T150000",
		"DESCRIPTION:Zeile eins\\nZeile",
		" zwei",
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"DESCRIPTION:Erinnerung",
		"TRIGGER:-PT15M",
		"END:VALARM",
	))
	evs, skipped, err := Parse(data, berlin(t))
	if err != nil || len(skipped) != 0 || len(evs) != 1 {
		t.Fatalf("Parse = %+v, %v, %v", evs, skipped, err)
	}
	want := Event{
		UID: "a1@uni", Summary: "Arzt, Dr. Müller", Location: "Praxis, Hauptstr. 1",
		Description: "Zeile eins\nZeilezwei", // gefaltet, VALARM-Beschreibung ignoriert
		Start:       utc("2026-10-09T12:00:00Z"), End: utc("2026-10-09T13:00:00Z"),
	}
	if !reflect.DeepEqual(evs[0], want) {
		t.Errorf("Event = %+v\nwant    %+v", evs[0], want)
	}
}

func TestParseWeeklyWithExdates(t *testing.T) {
	data := calendar(vevent(
		"UID:w1",
		"SUMMARY:AlgoDat",
		"DTSTART;TZID=Europe/Berlin:20261005T083000",
		"DTEND;TZID=Europe/Berlin:20261005T100000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20270130T225959Z;WKST=MO",
		"EXDATE;TZID=Europe/Berlin:20261012T083000,20261014T083000",
		"EXDATE;VALUE=DATE:20261019",
		"EXDATE:20261026T073000Z", // 08:30 Berlin (MEZ nach der Zeitumstellung)
	))
	evs, _, err := Parse(data, berlin(t))
	if err != nil || len(evs) != 1 {
		t.Fatalf("Parse = %+v, %v", evs, err)
	}
	e := evs[0]
	if e.Rule != "FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20270130T225959Z" {
		t.Errorf("Rule = %q", e.Rule)
	}
	if want := []string{"2026-10-12", "2026-10-14", "2026-10-19", "2026-10-26"}; !reflect.DeepEqual(e.Exdates, want) {
		t.Errorf("Exdates = %v, want %v", e.Exdates, want)
	}
	if !e.Start.Equal(utc("2026-10-05T06:30:00Z")) || !e.End.Equal(utc("2026-10-05T08:00:00Z")) {
		t.Errorf("Zeiten = %v – %v", e.Start, e.End)
	}
}

func TestParseAllDay(t *testing.T) {
	data := calendar(
		vevent("UID:d1", "SUMMARY:Ferien", "DTSTART;VALUE=DATE:20261224", "DTEND;VALUE=DATE:20261227"),
		vevent("UID:d2", "SUMMARY:Prüfungstag", "DTSTART;VALUE=DATE:20270201"), // ohne DTEND: ein Tag
	)
	evs, skipped, err := Parse(data, berlin(t))
	if err != nil || len(skipped) != 0 || len(evs) != 2 {
		t.Fatalf("Parse = %+v, %v, %v", evs, skipped, err)
	}
	// Mitternacht Berlin (MEZ) = 23:00 UTC des Vortags; Ende exklusiv.
	if !evs[0].Start.Equal(utc("2026-12-23T23:00:00Z")) || !evs[0].End.Equal(utc("2026-12-26T23:00:00Z")) {
		t.Errorf("Ferien = %v – %v", evs[0].Start, evs[0].End)
	}
	if !evs[1].Start.Equal(utc("2027-01-31T23:00:00Z")) || !evs[1].End.Equal(utc("2027-02-01T23:00:00Z")) {
		t.Errorf("Prüfungstag = %v – %v", evs[1].Start, evs[1].End)
	}
}

func TestParseTimeZones(t *testing.T) {
	data := calendar(
		vevent("UID:z1", "DTSTART;TZID=America/New_York:20261105T090000", "DTEND;TZID=America/New_York:20261105T100000"),
		vevent("UID:z2", "DTSTART:20261105T090000Z", "DTEND:20261105T100000Z"),
		vevent("UID:z3", "DTSTART:20261105T090000", "DTEND:20261105T100000"), // schwebend: Serverzone
		vevent("UID:z4", "SUMMARY:Windows", "DTSTART;TZID=W. Europe Standard Time:20261105T090000", "DTEND;TZID=W. Europe Standard Time:20261105T100000"),
	)
	evs, _, err := Parse(data, berlin(t))
	if err != nil || len(evs) != 4 {
		t.Fatalf("Parse = %+v, %v", evs, err)
	}
	for i, want := range []string{"2026-11-05T14:00:00Z", "2026-11-05T09:00:00Z", "2026-11-05T08:00:00Z", "2026-11-05T08:00:00Z"} {
		if !evs[i].Start.Equal(utc(want)) {
			t.Errorf("Event %d Start = %v, want %s", i, evs[i].Start, want)
		}
	}
	if evs[0].Note != "" || !strings.Contains(evs[3].Note, "W. Europe Standard Time") {
		t.Errorf("Notes = %q, %q", evs[0].Note, evs[3].Note)
	}
}

func TestParseSkipsAndErrors(t *testing.T) {
	data := calendar(
		vevent("SUMMARY:ohne UID", "DTSTART:20261105T090000Z", "DTEND:20261105T100000Z"),
		vevent("UID:r", "SUMMARY:Verschoben", "RECURRENCE-ID:20261105T090000Z", "DTSTART:20261106T090000Z", "DTEND:20261106T100000Z"),
		vevent("UID:c", "SUMMARY:Abgesagt", "STATUS:CANCELLED", "DTSTART:20261105T090000Z", "DTEND:20261105T100000Z"),
		vevent("UID:n", "SUMMARY:Ohne Start", "DTEND:20261105T100000Z"),
		vevent("UID:e", "SUMMARY:Ohne Ende", "DTSTART:20261105T090000Z"),
		vevent("UID:b", "SUMMARY:Rückwärts", "DTSTART:20261105T100000Z", "DTEND:20261105T090000Z"),
		vevent("UID:x", "SUMMARY:Kaputt", "DTSTART:morgen", "DTEND:20261105T090000Z"),
		vevent("UID:ok", "SUMMARY:Gut", "DTSTART:20261105T090000Z", "DTEND:20261105T100000Z"),
	)
	evs, skipped, err := Parse(data, time.UTC)
	if err != nil || len(evs) != 1 || evs[0].UID != "ok" || len(skipped) != 7 {
		t.Errorf("Parse = %+v, %q, %v", evs, skipped, err)
	}
	if _, _, err := Parse("kein Kalender", time.UTC); err == nil {
		t.Error("Text ohne VCALENDAR wurde akzeptiert")
	}
}
