package service

import "testing"

func TestFormatScorecard(t *testing.T) {
	cur := Review{TrackedMinutes: 750, CompletedTasks: 4,
		Projects: []ReviewProject{{Name: "Kairo", TrackedMinutes: 370}, {Name: "Uni", TrackedMinutes: 380}, {Name: "Leer"}},
		Habits:   []ReviewHabit{{Name: "Sport", Done: 5, Streak: 12}, {Name: "Lesen", Done: 2}},
	}
	prev := Review{TrackedMinutes: 620, CompletedTasks: 7, Habits: []ReviewHabit{{Done: 7}}}
	title, body := FormatScorecard(cur, prev)
	if title != "Wochenbilanz: 12:30 h · 4 erledigt" {
		t.Errorf("title = %q", title)
	}
	want := "Zeit: 12:30 h (+2:10)\nErledigt: 4 Aufgaben (−3)\nGewohnheiten: 7× abgehakt\nBeste Serie: Sport (12)\nProjekte: Kairo 6:10 · Uni 6:20"
	if body != want {
		t.Errorf("body =\n%s\nwant\n%s", body, want)
	}
}
