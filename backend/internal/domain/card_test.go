package domain

import (
	"testing"
	"time"
)

func TestParseCards(t *testing.T) {
	cs := ParseCards("uni/bio.md", "# Bio\n- Mitochondrium :: Kraftwerk der Zelle\nText ohne Karte\n```\nx :: y\n```\nleer :: \nATP :: Energieträger")
	if len(cs) != 2 || cs[0].Question != "Mitochondrium" || cs[0].Answer != "Kraftwerk der Zelle" || cs[1].Question != "ATP" || cs[1].NotePath != "uni/bio.md" {
		t.Fatalf("ParseCards = %+v", cs)
	}
	if again := ParseCards("x.md", "Mitochondrium :: anders"); again[0].Key != cs[0].Key {
		t.Error("gleiche Frage muss den gleichen Key haben")
	}
}

func TestCardNext(t *testing.T) {
	day := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	var c CardState
	steps := []struct {
		g        CardGrade
		interval int
		due      string
	}{
		{GradeGood, 1, "2026-10-22"},
		{GradeGood, 3, "2026-10-24"},
		{GradeGood, 8, "2026-10-29"}, // 3 × 2,5 = 7,5
		{GradeAgain, 0, "2026-10-21"},
		{GradeEasy, 4, "2026-10-25"},
		{GradeHard, 5, "2026-10-26"},
	}
	for i, s := range steps {
		c = c.Next(s.g, day)
		if c.Interval != s.interval || c.Due != s.due {
			t.Fatalf("Schritt %d: %+v, erwartet Intervall %d, fällig %s", i, c, s.interval, s.due)
		}
	}
	for range 20 {
		c = c.Next(GradeAgain, day)
	}
	if c.Ease != 1.3 {
		t.Errorf("Ease = %v, Untergrenze 1,3", c.Ease)
	}
}
