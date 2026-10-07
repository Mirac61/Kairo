package service

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"kairo/internal/domain"
)

type memEvents struct {
	m map[string]domain.CalendarEvent
}

func (s *memEvents) Create(_ context.Context, e domain.CalendarEvent) error {
	s.m[e.ID] = e
	return nil
}
func (s *memEvents) Get(_ context.Context, id string) (domain.CalendarEvent, error) {
	e, ok := s.m[id]
	if !ok {
		return domain.CalendarEvent{}, domain.ErrNotFound
	}
	return e, nil
}
func (s *memEvents) List(context.Context) ([]domain.CalendarEvent, error) {
	var out []domain.CalendarEvent
	for _, e := range s.m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}
func (s *memEvents) Update(_ context.Context, e domain.CalendarEvent) error {
	s.m[e.ID] = e
	return nil
}
func (s *memEvents) Delete(_ context.Context, id string) error { delete(s.m, id); return nil }

// Papierkorb: gegen echtes SQLite getestet (api/trash_test.go).
func (s *memEvents) Trash(context.Context, string, time.Time) error { return nil }
func (s *memEvents) Restore(context.Context, string) (domain.CalendarEvent, error) {
	return domain.CalendarEvent{}, nil
}
func (s *memEvents) ListTrashed(context.Context) ([]domain.CalendarEvent, error) { return nil, nil }

func newCalSvc(t *testing.T) *CalendarService {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("keine Zeitzonendaten:", err)
	}
	return NewCalendarService(&memEvents{m: map[string]domain.CalendarEvent{}}, loc, nil)
}

func tm(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestEventCreateValidation(t *testing.T) {
	svc := newCalSvc(t)
	ctx := context.Background()
	ok := CreateEventInput{Title: "Vorlesung", StartAt: "2026-10-07T08:30:00+02:00", EndAt: "2026-10-07T10:00:00+02:00"}

	e, err := svc.Create(ctx, ok)
	if err != nil {
		t.Fatal(err)
	}
	if e.StartAt.Format(time.RFC3339) != "2026-10-07T06:30:00Z" || e.RecurrenceExdates == nil {
		t.Errorf("Create = %+v", e)
	}

	mod := func(f func(*CreateEventInput)) CreateEventInput { in := ok; f(&in); return in }
	for name, in := range map[string]CreateEventInput{
		"leerer Titel":        mod(func(i *CreateEventInput) { i.Title = " " }),
		"Start fehlt":         mod(func(i *CreateEventInput) { i.StartAt = "" }),
		"Ende vor Start":      mod(func(i *CreateEventInput) { i.EndAt = i.StartAt }),
		"falsche URL":         mod(func(i *CreateEventInput) { i.URL = "ftp://x" }),
		"falsche Regel":       mod(func(i *CreateEventInput) { i.RecurrenceRule = ptr("FREQ=YEARLY") }),
		"Ausnahme ohne Regel": mod(func(i *CreateEventInput) { i.RecurrenceExdates = []string{"2026-10-14"} }),
		"falsche Ausnahme": mod(func(i *CreateEventInput) {
			i.RecurrenceRule = ptr("FREQ=DAILY")
			i.RecurrenceExdates = []string{"14.10."}
		}),
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestEventRuleNormalizedAndExdatesSorted(t *testing.T) {
	svc := newCalSvc(t)
	e, err := svc.Create(context.Background(), CreateEventInput{
		Title: "x", StartAt: "2026-10-07T08:30:00+02:00", EndAt: "2026-10-07T10:00:00+02:00",
		RecurrenceRule: ptr(" RRULE:FREQ=DAILY "), RecurrenceExdates: []string{"2026-10-20", "2026-10-09", "2026-10-20"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *e.RecurrenceRule != "FREQ=DAILY" || !reflect.DeepEqual(e.RecurrenceExdates, []string{"2026-10-09", "2026-10-20"}) {
		t.Errorf("Create = %v %v", *e.RecurrenceRule, e.RecurrenceExdates)
	}
}

func TestEventUpdateRuleRemovalClearsExdates(t *testing.T) {
	svc := newCalSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, CreateEventInput{Title: "x", StartAt: "2026-10-07T08:30:00Z", EndAt: "2026-10-07T09:30:00Z",
		RecurrenceRule: ptr("FREQ=DAILY"), RecurrenceExdates: []string{"2026-10-09"}})

	got, err := svc.Update(ctx, e.ID, UpdateEventInput{Title: ptr("y")})
	if err != nil || got.Title != "y" || len(got.RecurrenceExdates) != 1 {
		t.Fatalf("partielles Update: %+v, %v", got, err)
	}
	got, err = svc.Update(ctx, e.ID, UpdateEventInput{RecurrenceRule: ptr("")})
	if err != nil || got.RecurrenceRule != nil || len(got.RecurrenceExdates) != 0 {
		t.Errorf("Regel entfernen: %+v, %v", got, err)
	}
	if _, err := svc.Update(ctx, e.ID, UpdateEventInput{EndAt: ptr("2026-10-07T08:00:00Z")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Ende vor Start: %v", err)
	}
	if _, err := svc.Update(ctx, "missing", UpdateEventInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannte ID: %v", err)
	}
}

func TestEventListWindow(t *testing.T) {
	svc := newCalSvc(t)
	ctx := context.Background()
	// Mi 07.10. 08:30–10:00 Berlin, wöchentlich, 14.10. ausgelassen
	_, _ = svc.Create(ctx, CreateEventInput{Title: "weekly", StartAt: "2026-10-07T08:30:00+02:00",
		EndAt: "2026-10-07T10:00:00+02:00", RecurrenceRule: ptr("FREQ=WEEKLY"), RecurrenceExdates: []string{"2026-10-14"}})
	_, _ = svc.Create(ctx, CreateEventInput{Title: "single", StartAt: "2026-10-09T12:00:00+02:00", EndAt: "2026-10-09T13:00:00+02:00"})
	// 23:00–02:00 über Mitternacht
	_, _ = svc.Create(ctx, CreateEventInput{Title: "night", StartAt: "2026-10-09T23:00:00+02:00", EndAt: "2026-10-10T02:00:00+02:00"})

	ids := func(from, to string) []string {
		es, err := svc.List(ctx, tm(from), tm(to))
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, e := range es {
			out = append(out, e.Title)
		}
		sort.Strings(out)
		return out
	}
	for name, tc := range map[string]struct {
		from, to string
		want     []string
	}{
		"Tag mit Serie":           {"2026-10-07T00:00:00+02:00", "2026-10-08T00:00:00+02:00", []string{"weekly"}},
		"ausgelassener Tag":       {"2026-10-14T00:00:00+02:00", "2026-10-15T00:00:00+02:00", nil},
		"nächste Woche":           {"2026-10-21T00:00:00+02:00", "2026-10-22T00:00:00+02:00", []string{"weekly"}},
		"Einzeltermin":            {"2026-10-09T00:00:00+02:00", "2026-10-10T00:00:00+02:00", []string{"night", "single"}},
		"Termin über Mitternacht": {"2026-10-10T00:00:00+02:00", "2026-10-11T00:00:00+02:00", []string{"night"}},
		"Ende exklusiv":           {"2026-10-07T10:00:00+02:00", "2026-10-07T11:00:00+02:00", nil},
		"Start exklusiv":          {"2026-10-09T11:00:00+02:00", "2026-10-09T12:00:00+02:00", nil},
	} {
		got := ids(tc.from, tc.to)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, erwartet %v", name, got, tc.want)
		}
	}

	all, _ := svc.List(ctx, nil, nil)
	if len(all) != 3 {
		t.Errorf("ohne Fenster: %d Events", len(all))
	}
	if _, err := svc.List(ctx, tm("2026-10-07T00:00:00Z"), nil); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("nur from: %v", err)
	}
	if _, err := svc.List(ctx, tm("2026-10-08T00:00:00Z"), tm("2026-10-07T00:00:00Z")); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("to vor from: %v", err)
	}
}

func TestEventOccurrenceKeepsLocalTimeAcrossDST(t *testing.T) {
	svc := newCalSvc(t)
	e, _ := svc.Create(context.Background(), CreateEventInput{Title: "x", StartAt: "2026-10-24T08:30:00+02:00",
		EndAt: "2026-10-24T09:30:00+02:00", RecurrenceRule: ptr("FREQ=DAILY")})
	occ, err := svc.occurrences(e, *tm("2026-10-26T00:00:00+01:00"), *tm("2026-10-27T00:00:00+01:00"))
	if err != nil || len(occ) != 1 {
		t.Fatalf("occ = %v, %v", occ, err)
	}
	if got := occ[0].Start.In(svc.loc).Format("15:04"); got != "08:30" {
		t.Errorf("Ortszeit = %s", got)
	}
	if occ[0].End.Sub(occ[0].Start) != time.Hour {
		t.Errorf("Dauer = %s", occ[0].End.Sub(occ[0].Start))
	}
}

func TestImportICSEmitsEventsAndNamesUntitled(t *testing.T) {
	svc := newCalSvc(t)
	pub := &recPublisher{}
	svc.SetPublisher(pub)
	ics := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:u1\r\nDTSTART:20261009T120000Z\r\nDTEND:20261009T130000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	ctx := context.Background()

	res, err := svc.ImportICS(ctx, ics)
	if err != nil || res.Created != 1 || len(res.Notes) != 0 || res.Notes == nil {
		t.Fatalf("1. Import = %+v, %v", res, err)
	}
	if res, err = svc.ImportICS(ctx, ics); err != nil || res.Created != 0 || res.Updated != 1 {
		t.Fatalf("2. Import = %+v, %v", res, err)
	}
	all, _ := svc.List(ctx, nil, nil)
	if len(all) != 1 || all[0].Title != "(ohne Titel)" || *all[0].ExternalUID != "u1" {
		t.Errorf("Events = %+v", all)
	}
	if want := []domain.EventType{domain.EventCalendarEventCreated, domain.EventCalendarEventUpdated}; !reflect.DeepEqual(pub.got, want) {
		t.Errorf("Ereignisse = %v, want %v", pub.got, want)
	}
	if _, err := svc.ImportICS(ctx, "nope"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("kein ICS: %v", err)
	}
}
