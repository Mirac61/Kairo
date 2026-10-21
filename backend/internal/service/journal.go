package service

import (
	"context"
	"time"
)

type JournalDateStore interface {
	JournalDates() ([]string, error)
}

// JournalSummary enthält die Eintragszahl und die aktuelle Folge täglicher Einträge.
type JournalSummary struct {
	Date   string
	Total  int
	Streak int
}

type JournalService struct {
	store JournalDateStore
	now   func() time.Time
	loc   *time.Location
}

func NewJournalService(store JournalDateStore, now func() time.Time, loc *time.Location) *JournalService {
	if now == nil {
		now = time.Now
	}
	if loc == nil {
		loc = time.Local
	}
	return &JournalService{store: store, now: now, loc: loc}
}

// Summary berechnet die Streak in der konfigurierten lokalen Zeitzone.
// Fehlt der heutige Eintrag, zählt die Folge ab gestern.
func (s *JournalService) Summary(ctx context.Context) (JournalSummary, error) {
	if err := ctx.Err(); err != nil {
		return JournalSummary{}, err
	}
	dates, err := s.store.JournalDates()
	if err != nil {
		return JournalSummary{}, err
	}
	now := s.now().In(s.loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	date := day.Format("2006-01-02")
	have := make(map[string]struct{}, len(dates))
	for _, d := range dates {
		have[d] = struct{}{}
	}
	if _, ok := have[date]; !ok {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for {
		if _, ok := have[day.Format("2006-01-02")]; !ok {
			break
		}
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return JournalSummary{Date: date, Total: len(dates), Streak: streak}, nil
}
