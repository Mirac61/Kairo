package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"kairo/internal/domain"
)

type memTasks struct {
	m       map[string]domain.Task
	imports map[string]domain.JournalTaskImport
}

func (s *memTasks) Create(_ context.Context, t domain.Task) error { s.m[t.ID] = t; return nil }
func (s *memTasks) ImportFromJournal(_ context.Context, date string, tasks []domain.Task) (domain.JournalTaskImport, bool, error) {
	if imported, ok := s.imports[date]; ok {
		return imported, false, nil
	}
	if s.imports == nil {
		s.imports = map[string]domain.JournalTaskImport{}
	}
	imported := domain.JournalTaskImport{JournalDate: date, TaskCount: len(tasks), ImportedAt: tasks[0].CreatedAt}
	for _, task := range tasks {
		s.m[task.ID] = task
	}
	s.imports[date] = imported
	return imported, true, nil
}
func (s *memTasks) ListJournalTaskImports(context.Context) ([]domain.JournalTaskImport, error) {
	out := make([]domain.JournalTaskImport, 0, len(s.imports))
	for _, item := range s.imports {
		out = append(out, item)
	}
	return out, nil
}
func (s *memTasks) Get(_ context.Context, id string) (domain.Task, error) {
	t, ok := s.m[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return t, nil
}
func (s *memTasks) List(context.Context, domain.TaskFilter) ([]domain.Task, error) { return nil, nil }
func (s *memTasks) Update(_ context.Context, t domain.Task) error                  { s.m[t.ID] = t; return nil }
func (s *memTasks) UpdateAndCreate(_ context.Context, t, next domain.Task) error {
	s.m[t.ID], s.m[next.ID] = t, next
	return nil
}
func (s *memTasks) Delete(_ context.Context, id string) error { delete(s.m, id); return nil }

// Papierkorb: gegen echtes SQLite getestet (api/trash_test.go).
func (s *memTasks) Trash(context.Context, string, time.Time) error       { return nil }
func (s *memTasks) Restore(context.Context, string) (domain.Task, error) { return domain.Task{}, nil }
func (s *memTasks) ListTrashed(context.Context) ([]domain.Task, error)   { return nil, nil }

func newTaskSvc() (*TaskService, *time.Time) {
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	return NewTaskService(&memTasks{m: map[string]domain.Task{}}, func() time.Time { return clock }), &clock
}

func TestTaskCreateDefaultsAndValidation(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()

	task, err := svc.Create(ctx, CreateTaskInput{Title: " Lernen ", PlannedDate: ptr("2026-10-07"),
		PlannedStartAt: ptr("2026-10-07T10:15:00+02:00")})
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Lernen" || task.Status != domain.TaskBacklog || task.Priority != domain.PriorityMedium {
		t.Errorf("Defaults = %+v", task)
	}
	if got := task.PlannedStartAt.Format(time.RFC3339); got != "2026-10-07T08:15:00Z" {
		t.Errorf("planned_start_at nicht in UTC: %s", got)
	}

	for name, in := range map[string]CreateTaskInput{
		"leerer Titel":       {Title: " "},
		"falscher Status":    {Title: "x", Status: "NOPE"},
		"falsche Priorität":  {Title: "x", Priority: "NOPE"},
		"negative Minuten":   {Title: "x", EstimatedMinutes: -1},
		"falsches Datum":     {Title: "x", PlannedDate: ptr("07.10.2026")},
		"Start ohne Datum":   {Title: "x", PlannedStartAt: ptr("2026-10-07T10:00:00Z")},
		"falscher Zeitpunkt": {Title: "x", DueAt: ptr("morgen")},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestTaskImportFromJournalUsesLocalDayAndIsIdempotent(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	store := &memTasks{m: map[string]domain.Task{}, imports: map[string]domain.JournalTaskImport{}}
	clock := time.Date(2026, 10, 24, 12, 0, 0, 0, berlin)
	svc := NewTaskService(store, func() time.Time { return clock }).WithLocation(berlin)

	imported, created, err := svc.ImportFromJournal(context.Background(), "2026-10-24", []string{"  Abgabe vorbereiten  ", "Lesen"})
	if err != nil || !created || imported.TaskCount != 2 {
		t.Fatalf("Import = %+v, created=%v, err=%v", imported, created, err)
	}
	for _, task := range store.m {
		if task.SourceJournalDate == nil || *task.SourceJournalDate != "2026-10-24" || task.DueAt == nil ||
			task.DueAt.In(berlin).Format("2006-01-02 15:04") != "2026-10-25 23:59" {
			t.Errorf("Task ohne Journal-Ursprung oder falsche Fälligkeit: %+v", task)
		}
	}

	imported, created, err = svc.ImportFromJournal(context.Background(), "2026-10-24", []string{"Andere Task"})
	if err != nil || created || imported.TaskCount != 2 || len(store.m) != 2 {
		t.Fatalf("zweiter Import = %+v, created=%v, Tasks=%d, err=%v", imported, created, len(store.m), err)
	}
}

func TestTaskImportFromJournalRejectsInvalidInput(t *testing.T) {
	svc, _ := newTaskSvc()
	for name, tc := range map[string]struct {
		date   string
		titles []string
	}{
		"Datum":         {"gestern", []string{"Task"}},
		"keine Auswahl": {"2026-10-07", nil},
		"leerer Titel":  {"2026-10-07", []string{"  "}},
	} {
		if _, _, err := svc.ImportFromJournal(context.Background(), tc.date, tc.titles); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestTaskUpdateCompletedAtFollowsStatus(t *testing.T) {
	svc, clock := newTaskSvc()
	ctx := context.Background()
	task, _ := svc.Create(ctx, CreateTaskInput{Title: "x", Description: "d"})

	*clock = clock.Add(time.Hour)
	done := domain.TaskCompleted
	got, err := svc.Update(ctx, task.ID, UpdateTaskInput{Status: &done})
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(*clock) || got.Description != "d" {
		t.Errorf("COMPLETED: %+v", got)
	}

	*clock = clock.Add(time.Hour)
	got, _ = svc.Update(ctx, task.ID, UpdateTaskInput{Title: ptr("y")})
	if got.CompletedAt == nil || got.CompletedAt.Equal(*clock) {
		t.Errorf("completed_at ohne Statuswechsel verändert: %+v", got)
	}

	open := domain.TaskPlanned
	got, _ = svc.Update(ctx, task.ID, UpdateTaskInput{Status: &open})
	if got.CompletedAt != nil {
		t.Error("completed_at nach Wiedereröffnen nicht gelöscht")
	}
}

func TestTaskUpdateClearsOptionalFields(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()
	task, _ := svc.Create(ctx, CreateTaskInput{Title: "x", PlannedDate: ptr("2026-10-07"),
		PlannedStartAt: ptr("2026-10-07T10:00:00Z"), ProjectID: ptr("p")})

	got, err := svc.Update(ctx, task.ID, UpdateTaskInput{PlannedDate: ptr(""), PlannedStartAt: ptr(""), ProjectID: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.PlannedDate != nil || got.PlannedStartAt != nil || got.ProjectID != nil {
		t.Errorf("nicht entfernt: %+v", got)
	}
	// Datum entfernen, Startzeit behalten verletzt die Regel.
	task, _ = svc.Create(ctx, CreateTaskInput{Title: "y", PlannedDate: ptr("2026-10-07"), PlannedStartAt: ptr("2026-10-07T10:00:00Z")})
	if _, err := svc.Update(ctx, task.ID, UpdateTaskInput{PlannedDate: ptr("")}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Datum ohne Startzeit entfernen: %v", err)
	}
}

func TestTaskParentCycles(t *testing.T) {
	svc, _ := newTaskSvc()
	ctx := context.Background()
	a, _ := svc.Create(ctx, CreateTaskInput{Title: "a"})
	b, _ := svc.Create(ctx, CreateTaskInput{Title: "b", ParentTaskID: &a.ID})
	c, _ := svc.Create(ctx, CreateTaskInput{Title: "c", ParentTaskID: &b.ID})

	for name, tc := range map[string]struct{ id, parent string }{
		"selbst":    {a.ID, a.ID},
		"Kind":      {a.ID, b.ID},
		"Enkel":     {a.ID, c.ID},
		"unbekannt": {a.ID, "missing"},
	} {
		if _, err := svc.Update(ctx, tc.id, UpdateTaskInput{ParentTaskID: ptr(tc.parent)}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := svc.Update(ctx, c.ID, UpdateTaskInput{ParentTaskID: ptr(a.ID)}); err != nil {
		t.Errorf("gültiges Umhängen: %v", err)
	}
}

func TestTaskListRejectsBadFilters(t *testing.T) {
	svc, _ := newTaskSvc()
	if _, err := svc.List(context.Background(), domain.TaskFilter{Status: "NOPE"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Status: %v", err)
	}
	if _, err := svc.List(context.Background(), domain.TaskFilter{PlannedDate: "heute"}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("Datum: %v", err)
	}
}

func TestTaskRecurrence(t *testing.T) {
	ctx := context.Background()
	berlin, _ := time.LoadLocation("Europe/Berlin")
	store := &memTasks{m: map[string]domain.Task{}}
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, berlin) // Dienstag
	svc := NewTaskService(store, func() time.Time { return clock }).WithLocation(berlin)
	done := domain.TaskCompleted
	others := func(id string) []domain.Task {
		var out []domain.Task
		for _, x := range store.m {
			if x.ID != id && x.Status != domain.TaskCompleted {
				out = append(out, x)
			}
		}
		return out
	}

	// Jeden Freitag, geplant Fr 9.10. 14:00, am Dienstag schon erledigt: nächste am Fr 16.10. 14:00.
	fr, err := svc.Create(ctx, CreateTaskInput{Title: "Wochenrückblick", PlannedDate: ptr("2026-10-09"),
		PlannedStartAt: ptr("2026-10-09T14:00:00+02:00"), RecurrenceRule: ptr("FREQ=WEEKLY;BYDAY=FR")})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Update(ctx, fr.ID, UpdateTaskInput{Status: &done})
	if err != nil {
		t.Fatal(err)
	}
	if got.RecurrenceRule != nil {
		t.Errorf("erledigte Task behält die Regel")
	}
	next := others(fr.ID)
	if len(next) != 1 || *next[0].PlannedDate != "2026-10-16" || next[0].Status != domain.TaskPlanned ||
		next[0].PlannedStartAt.In(berlin).Format("15:04") != "14:00" || *next[0].RecurrenceRule != "FREQ=WEEKLY;BYDAY=FR" {
		t.Fatalf("nächste = %+v", next)
	}
	delete(store.m, next[0].ID)

	// Monatlich am Monatsletzten, geplant 30.9., erledigt am 6.10.: nächste am 31.10.
	mo, _ := svc.Create(ctx, CreateTaskInput{Title: "Miete", PlannedDate: ptr("2026-09-30"), RecurrenceRule: ptr("FREQ=MONTHLY;BYMONTHDAY=-1")})
	if _, err := svc.Update(ctx, mo.ID, UpdateTaskInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	if next := others(mo.ID); len(next) != 1 || *next[0].PlannedDate != "2026-10-31" {
		t.Fatalf("monatlich: %+v", next)
	} else {
		delete(store.m, next[0].ID)
	}

	// Täglich, überfällig seit Freitag: nächste ist morgen, nicht der Samstag in der Vergangenheit.
	d, _ := svc.Create(ctx, CreateTaskInput{Title: "Lesen", PlannedDate: ptr("2026-10-02"), RecurrenceRule: ptr("FREQ=DAILY")})
	if _, err := svc.Update(ctx, d.ID, UpdateTaskInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	if n := others(d.ID); len(n) != 1 || *n[0].PlannedDate != "2026-10-07" {
		t.Fatalf("täglich nächste = %+v", n)
	}
	for id := range store.m {
		delete(store.m, id)
	}

	// Abgelaufenes UNTIL: keine neue Task. COUNT und Unsinn werden abgelehnt.
	u, _ := svc.Create(ctx, CreateTaskInput{Title: "x", RecurrenceRule: ptr("FREQ=DAILY;UNTIL=20261006")})
	if _, err := svc.Update(ctx, u.ID, UpdateTaskInput{Status: &done}); err != nil || len(others(u.ID)) != 0 {
		t.Errorf("UNTIL: err=%v, andere=%v", err, others(u.ID))
	}
	for _, r := range []string{"FREQ=DAILY;COUNT=3", "FREQ=HOURLY"} {
		if _, err := svc.Create(ctx, CreateTaskInput{Title: "x", RecurrenceRule: ptr(r)}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", r, err)
		}
	}
}
