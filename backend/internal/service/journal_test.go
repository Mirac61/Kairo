package service

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type journalDates []string

func (j journalDates) JournalDates() ([]string, error) { return j, nil }

func TestJournalSummaryStreakAcrossMonthsAndGaps(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		now    time.Time
		dates  []string
		date   string
		total  int
		streak int
	}{
		{
			name:  "Monatsgrenze",
			now:   time.Date(2026, 11, 2, 12, 0, 0, 0, berlin),
			dates: []string{"2026-10-31", "2026-11-01", "2026-11-02"},
			date:  "2026-11-02", total: 3, streak: 3,
		},
		{
			name:  "Lücke beendet die Folge",
			now:   time.Date(2026, 11, 2, 12, 0, 0, 0, berlin),
			dates: []string{"2026-10-29", "2026-10-31", "2026-11-01"},
			date:  "2026-11-02", total: 3, streak: 2,
		},
		{
			name:  "heute fehlt, gestern zählt",
			now:   time.Date(2026, 11, 2, 12, 0, 0, 0, berlin),
			dates: []string{"2026-10-31", "2026-11-01"},
			date:  "2026-11-02", total: 2, streak: 2,
		},
		{
			name:  "Berlin bestimmt den Kalendertag",
			now:   time.Date(2026, 10, 21, 22, 30, 0, 0, time.UTC),
			dates: []string{"2026-10-22"},
			date:  "2026-10-22", total: 1, streak: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewJournalService(journalDates(tc.dates), func() time.Time { return tc.now }, berlin)
			got, err := svc.Summary(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := JournalSummary{Date: tc.date, Total: tc.total, Streak: tc.streak}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Summary = %+v, erwartet %+v", got, want)
			}
		})
	}
}
