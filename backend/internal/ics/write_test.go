package ics

import (
	"reflect"
	"strings"
	"testing"
)

func TestFormatRoundTrip(t *testing.T) {
	loc := berlin(t)
	in := Event{
		UID: "a@kairo", Summary: "Sitzung; lang, mit Ümläuten " + strings.Repeat("ä", 60), Description: "Zeile 1\nZeile 2",
		Start: utc("2026-10-05T08:00:00Z"), End: utc("2026-10-05T09:00:00Z"),
		Rule: "FREQ=WEEKLY;COUNT=3", Exdates: []string{"2026-10-12"},
	}
	todo := Todo{UID: "t@kairo", Summary: "Abgabe", Due: utc("2026-10-06T10:00:00Z"), Status: "NEEDS-ACTION"}
	out := Format([]Event{in}, []Todo{todo}, utc("2026-10-01T00:00:00Z"), loc)
	for _, l := range strings.Split(out, "\r\n") {
		if len(l) > 75 {
			t.Errorf("Zeile länger als 75 Oktette: %q", l)
		}
	}
	if !strings.Contains(out, "BEGIN:VTODO") || !strings.Contains(out, "DUE:20261006T100000Z") {
		t.Errorf("VTODO fehlt:\n%s", out)
	}
	evs, skipped, err := Parse(out, loc)
	if err != nil || len(skipped) != 0 || len(evs) != 1 {
		t.Fatalf("Parse = %v %v %v", evs, skipped, err)
	}
	if !reflect.DeepEqual(evs[0], in) {
		t.Errorf("Round-Trip = %+v, will %+v", evs[0], in)
	}
}
