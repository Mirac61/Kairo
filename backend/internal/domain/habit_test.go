package domain

import (
	"errors"
	"reflect"
	"testing"
)

func set(days ...string) map[string]bool {
	m := map[string]bool{}
	for _, d := range days {
		m[d] = true
	}
	return m
}

func TestNormalizeConfig(t *testing.T) {
	ok := []struct {
		name  string
		typ   FrequencyType
		in    FrequencyConfig
		start string
		want  FrequencyConfig
	}{
		{"daily", FreqDaily, FrequencyConfig{}, "2026-10-05", FrequencyConfig{}},
		{"weekly Default = Wochentag des Starts", FreqWeekly, FrequencyConfig{}, "2026-10-07", FrequencyConfig{Weekday: "WE"}},
		{"weekly klein", FreqWeekly, FrequencyConfig{Weekday: "fr"}, "2026-10-07", FrequencyConfig{Weekday: "FR"}},
		{"weekdays sortiert, ohne Doppelte", FreqSpecificWeekdays, FrequencyConfig{Weekdays: []string{"su", "MO", "WE", "mo"}}, "2026-10-07",
			FrequencyConfig{Weekdays: []string{"MO", "WE", "SU"}}},
		{"times", FreqTimesPerWeek, FrequencyConfig{Times: 3}, "2026-10-07", FrequencyConfig{Times: 3}},
	}
	for _, tc := range ok {
		got, err := NormalizeConfig(tc.typ, tc.in, tc.start)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %+v, %v", tc.name, got, err)
		}
	}
	bad := map[string]struct {
		typ FrequencyType
		in  FrequencyConfig
	}{
		"daily mit times":     {FreqDaily, FrequencyConfig{Times: 1}},
		"weekly falscher Tag": {FreqWeekly, FrequencyConfig{Weekday: "XX"}},
		"weekly mit times":    {FreqWeekly, FrequencyConfig{Times: 2}},
		"weekdays leer":       {FreqSpecificWeekdays, FrequencyConfig{}},
		"weekdays falsch":     {FreqSpecificWeekdays, FrequencyConfig{Weekdays: []string{"MO", "Q"}}},
		"times 0":             {FreqTimesPerWeek, FrequencyConfig{}},
		"times 8":             {FreqTimesPerWeek, FrequencyConfig{Times: 8}},
		"times mit weekday":   {FreqTimesPerWeek, FrequencyConfig{Times: 2, Weekday: "MO"}},
		"unbekannter Typ":     {"MONTHLY", FrequencyConfig{}},
	}
	for name, tc := range bad {
		if _, err := NormalizeConfig(tc.typ, tc.in, "2026-10-07"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestOccurrenceOnFixedDays(t *testing.T) {
	h := Habit{ID: "h", Active: true, StartDate: "2026-10-05"}
	// 05.10.2026 ist ein Montag.
	cases := []struct {
		name string
		typ  FrequencyType
		cfg  FrequencyConfig
		date string
		want bool
	}{
		{"daily", FreqDaily, FrequencyConfig{}, "2026-10-06", true},
		{"weekly trifft", FreqWeekly, FrequencyConfig{Weekday: "WE"}, "2026-10-07", true},
		{"weekly trifft nicht", FreqWeekly, FrequencyConfig{Weekday: "WE"}, "2026-10-08", false},
		{"weekdays trifft", FreqSpecificWeekdays, FrequencyConfig{Weekdays: []string{"MO", "SU"}}, "2026-10-11", true},
		{"weekdays trifft nicht", FreqSpecificWeekdays, FrequencyConfig{Weekdays: []string{"MO", "SU"}}, "2026-10-10", false},
		{"vor Start", FreqDaily, FrequencyConfig{}, "2026-10-04", false},
	}
	for _, tc := range cases {
		h.FrequencyType, h.FrequencyConfig = tc.typ, tc.cfg
		occ, due := h.OccurrenceOn(tc.date, set("2026-10-06"))
		if due != tc.want {
			t.Errorf("%s: fällig = %v", tc.name, due)
		}
		if due && occ.Progress != nil {
			t.Errorf("%s: Progress nur bei TIMES_PER_WEEK", tc.name)
		}
	}
}

func TestOccurrenceOnBoundsAndInactive(t *testing.T) {
	end := "2026-10-09"
	h := Habit{ID: "h", Active: true, FrequencyType: FreqDaily, StartDate: "2026-10-05", EndDate: &end}
	for date, want := range map[string]bool{"2026-10-05": true, "2026-10-09": true, "2026-10-10": false} {
		if _, due := h.OccurrenceOn(date, nil); due != want {
			t.Errorf("%s: fällig = %v", date, due)
		}
	}
	if occ, due := h.OccurrenceOn("2026-10-06", set("2026-10-06")); !due || !occ.Done {
		t.Errorf("Done = %+v", occ)
	}
	h.Active = false
	if _, due := h.OccurrenceOn("2026-10-06", nil); due {
		t.Error("inaktiv ist fällig")
	}
	if _, due := h.OccurrenceOn("kaputt", nil); due {
		t.Error("ungültiges Datum ist fällig")
	}
}

func TestTimesPerWeekProgress(t *testing.T) {
	h := Habit{ID: "h", Active: true, FrequencyType: FreqTimesPerWeek, FrequencyConfig: FrequencyConfig{Times: 3}, StartDate: "2026-09-01"}
	// Woche Mo 05.10. – So 11.10.; 04.10. (Vorwoche) zählt nicht mit.
	done := set("2026-10-04", "2026-10-05")

	occ, due := h.OccurrenceOn("2026-10-07", done)
	if !due || occ.Done || occ.Progress == nil || *occ.Progress != (WeekProgress{Done: 1, Target: 3}) {
		t.Errorf("1/3: %+v due=%v", occ, due)
	}
	done = set("2026-10-05", "2026-10-07", "2026-10-09")
	if _, due := h.OccurrenceOn("2026-10-10", done); due {
		t.Error("Wochenziel erreicht, trotzdem fällig")
	}
	// Der Tag, an dem das Ziel erreicht wurde, bleibt als erledigt sichtbar.
	if occ, due := h.OccurrenceOn("2026-10-09", done); !due || !occ.Done || occ.Progress.Done != 3 {
		t.Errorf("Zieltag: %+v due=%v", occ, due)
	}
	// Nächste Woche beginnt wieder bei 0.
	if occ, due := h.OccurrenceOn("2026-10-12", done); !due || occ.Progress.Done != 0 {
		t.Errorf("neue Woche: %+v due=%v", occ, due)
	}
}

func TestWeekRange(t *testing.T) {
	for date, want := range map[string][2]string{
		"2026-10-05": {"2026-10-05", "2026-10-11"}, // Montag
		"2026-10-11": {"2026-10-05", "2026-10-11"}, // Sonntag
		"2026-10-07": {"2026-10-05", "2026-10-11"},
	} {
		m, s, err := WeekRange(date)
		if err != nil || m != want[0] || s != want[1] {
			t.Errorf("%s: %s %s %v", date, m, s, err)
		}
	}
	if _, _, err := WeekRange("x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("ungültig: %v", err)
	}
}
