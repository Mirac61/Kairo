package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"kairo/internal/domain"
)

// memTime ist Store und Transaktion in einem; Rollback kennt er nicht
// (das prüfen die Repository-Tests).
type memTime struct {
	tasks   map[string]domain.Task
	entries []domain.TimeEntry
}

func (m *memTime) Create(_ context.Context, e domain.TimeEntry) error {
	m.entries = append(m.entries, e)
	return nil
}
func (m *memTime) List(context.Context, domain.TimeEntryFilter) ([]domain.TimeEntry, error) {
	return m.entries, nil
}
func (m *memTime) Tx(_ context.Context, fn func(TimeTx) error) error { return fn(m) }

func (m *memTime) GetTask(_ context.Context, id string) (domain.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return t, nil
}
func (m *memTime) UpdateTask(_ context.Context, t domain.Task) error { m.tasks[t.ID] = t; return nil }
func (m *memTime) RunningEntry(context.Context) (*domain.TimeEntry, error) {
	for i := range m.entries {
		if m.entries[i].EndedAt == nil {
			e := m.entries[i]
			return &e, nil
		}
	}
	return nil, nil
}
func (m *memTime) CreateEntry(ctx context.Context, e domain.TimeEntry) error { return m.Create(ctx, e) }
func (m *memTime) CloseEntry(_ context.Context, id string, at time.Time) error {
	for i := range m.entries {
		if m.entries[i].ID == id && m.entries[i].EndedAt == nil {
			m.entries[i].EndedAt = &at
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *memTime) running() []domain.TimeEntry {
	var out []domain.TimeEntry
	for _, e := range m.entries {
		if e.EndedAt == nil {
			out = append(out, e)
		}
	}
	return out
}

func newTimeSvc(tasks map[string]domain.TaskStatus) (*TimeTrackingService, *memTime, *time.Time) {
	clock := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	m := &memTime{tasks: map[string]domain.Task{}}
	for id, status := range tasks {
		m.tasks[id] = domain.Task{ID: id, Title: id, Status: status, Priority: domain.PriorityMedium}
	}
	return NewTimeTrackingService(m, func() time.Time { return clock }), m, &clock
}

func TestTimeStartSetsInProgressAndOpensEntry(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPaused})
	res, err := svc.Start(context.Background(), "a", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Task.Status != domain.TaskInProgress || res.Task.CompletedAt != nil || !res.Task.UpdatedAt.Equal(*clock) {
		t.Errorf("Task = %+v", res.Task)
	}
	if res.Entry == nil || *res.Entry.TaskID != "a" || !res.Entry.StartedAt.Equal(*clock) ||
		res.Entry.EndedAt != nil || res.Entry.Source != domain.SourceManual {
		t.Errorf("Entry = %+v", res.Entry)
	}
	if len(m.entries) != 1 || m.tasks["a"].Status != domain.TaskInProgress {
		t.Errorf("gespeichert: %d Einträge, Task %s", len(m.entries), m.tasks["a"].Status)
	}

	svc2, _, _ := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned})
	res, _ = svc2.Start(context.Background(), "a", domain.SourceVSCodium)
	if res.Entry.Source != domain.SourceVSCodium {
		t.Errorf("Quelle = %s", res.Entry.Source)
	}
}

func TestTimeStartPausesOtherRunningTask(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned, "b": domain.TaskBacklog})
	ctx := context.Background()
	if _, err := svc.Start(ctx, "a", ""); err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(30 * time.Minute)
	res, err := svc.Start(ctx, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Task.ID != "b" || res.Task.Status != domain.TaskInProgress {
		t.Errorf("Task b = %+v", res.Task)
	}
	if a := m.tasks["a"]; a.Status != domain.TaskPaused || !a.UpdatedAt.Equal(*clock) {
		t.Errorf("Task a = %+v", a)
	}
	if len(m.entries) != 2 || m.entries[0].EndedAt == nil || !m.entries[0].EndedAt.Equal(*clock) {
		t.Errorf("Eintrag a nicht zur Startzeit von b beendet: %+v", m.entries)
	}
	if r := m.running(); len(r) != 1 || *r[0].TaskID != "b" {
		t.Errorf("laufend = %+v", r)
	}
}

func TestTimeStartIsIdempotentForRunningTask(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned})
	ctx := context.Background()
	first, _ := svc.Start(ctx, "a", "")
	*clock = clock.Add(time.Minute)
	again, err := svc.Start(ctx, "a", domain.SourceVSCodium)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.entries) != 1 || again.Entry.ID != first.Entry.ID || !again.Entry.StartedAt.Equal(first.Entry.StartedAt) {
		t.Errorf("zweiter Start änderte den Timer: %+v", m.entries)
	}
}

func TestTimeStartErrors(t *testing.T) {
	svc, m, _ := newTimeSvc(map[string]domain.TaskStatus{"done": domain.TaskCompleted, "off": domain.TaskCancelled, "ok": domain.TaskPlanned})
	ctx := context.Background()
	for id, want := range map[string]error{"done": domain.ErrConflict, "off": domain.ErrConflict, "gibt-es-nicht": domain.ErrNotFound} {
		if _, err := svc.Start(ctx, id, ""); !errors.Is(err, want) {
			t.Errorf("Start %s: %v, erwartet %v", id, err, want)
		}
	}
	if _, err := svc.Start(ctx, "ok", "ROBOTER"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("falsche Quelle: %v", err)
	}
	if len(m.entries) != 0 {
		t.Errorf("Fehlerfälle haben Einträge angelegt: %+v", m.entries)
	}
}

func TestTimePause(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned, "b": domain.TaskPlanned})
	ctx := context.Background()
	if _, err := svc.Pause(ctx, "a"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Pause ohne Timer: %v", err)
	}
	if _, err := svc.Start(ctx, "a", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pause(ctx, "b"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Pause einer anderen Task: %v", err)
	}
	if _, err := svc.Pause(ctx, "gibt-es-nicht"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Pause unbekannt: %v", err)
	}

	*clock = clock.Add(45 * time.Minute)
	res, err := svc.Pause(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if res.Task.Status != domain.TaskPaused || res.Entry == nil || res.Entry.EndedAt == nil || !res.Entry.EndedAt.Equal(*clock) {
		t.Errorf("Pause = %+v / %+v", res.Task, res.Entry)
	}
	if len(m.running()) != 0 {
		t.Error("nach Pause läuft noch ein Timer")
	}
	if _, err := svc.Pause(ctx, "a"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("zweite Pause: %v", err)
	}
}

func TestTimeComplete(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{
		"a": domain.TaskPlanned, "b": domain.TaskPlanned, "off": domain.TaskCancelled})
	ctx := context.Background()
	if _, err := svc.Start(ctx, "a", ""); err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(time.Hour)
	res, err := svc.Complete(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if res.Task.Status != domain.TaskCompleted || res.Task.CompletedAt == nil || !res.Task.CompletedAt.Equal(*clock) ||
		res.Entry == nil || !res.Entry.EndedAt.Equal(*clock) || len(m.running()) != 0 {
		t.Errorf("Complete = %+v / %+v", res.Task, res.Entry)
	}

	*clock = clock.Add(time.Hour)
	again, err := svc.Complete(ctx, "a")
	if err != nil || again.Entry != nil || !again.Task.CompletedAt.Equal(*res.Task.CompletedAt) {
		t.Errorf("zweites Complete = %+v, %v", again, err)
	}
	if _, err := svc.Complete(ctx, "off"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Complete einer abgebrochenen Task: %v", err)
	}

	// Task ohne Timer abschließen, während eine andere läuft: der fremde Timer bleibt.
	if _, err := svc.Start(ctx, "b", ""); err != nil {
		t.Fatal(err)
	}
	svc2, m2, _ := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskBacklog, "b": domain.TaskPlanned})
	_, _ = svc2.Start(ctx, "b", "")
	res, err = svc2.Complete(ctx, "a")
	if err != nil || res.Entry != nil || res.Task.Status != domain.TaskCompleted {
		t.Errorf("Complete ohne Timer = %+v, %v", res, err)
	}
	if r := m2.running(); len(r) != 1 || *r[0].TaskID != "b" {
		t.Errorf("fremder Timer verändert: %+v", r)
	}
}

func TestTimeClosingNeverEndsBeforeStart(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned})
	ctx := context.Background()
	_, _ = svc.Start(ctx, "a", "")
	started := *clock
	*clock = clock.Add(-time.Hour) // Uhr springt zurück
	if _, err := svc.Pause(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if end := m.entries[0].EndedAt; end == nil || end.Before(started) {
		t.Errorf("ended_at = %v vor started_at %v", end, started)
	}
}

func TestTimeStopTimer(t *testing.T) {
	svc, m, clock := newTimeSvc(map[string]domain.TaskStatus{"a": domain.TaskPlanned, "b": domain.TaskPlanned})
	ctx := context.Background()
	if err := svc.StopTimer(ctx, "a"); err != nil {
		t.Errorf("StopTimer ohne Timer: %v", err)
	}
	_, _ = svc.Start(ctx, "a", "")
	if err := svc.StopTimer(ctx, "b"); err != nil || len(m.running()) != 1 {
		t.Errorf("StopTimer einer anderen Task: %v, laufend %d", err, len(m.running()))
	}
	*clock = clock.Add(time.Minute)
	if err := svc.StopTimer(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if len(m.running()) != 0 || m.tasks["a"].Status != domain.TaskInProgress {
		t.Errorf("laufend %d, Status %s (StopTimer darf den Status nicht ändern)", len(m.running()), m.tasks["a"].Status)
	}
}

func TestTimeCreateManualEntry(t *testing.T) {
	svc, m, _ := newTimeSvc(nil)
	ctx := context.Background()
	e, err := svc.Create(ctx, CreateTimeEntryInput{TaskID: ptr(" t1 "),
		StartedAt: "2026-10-06T10:00:00+02:00", EndedAt: "2026-10-06T08:30:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" || *e.TaskID != "t1" || e.ProjectID != nil || e.Source != domain.SourceManual ||
		e.StartedAt.Format(time.RFC3339) != "2026-10-06T08:00:00Z" || e.EndedAt == nil || len(m.entries) != 1 {
		t.Errorf("Entry = %+v", e)
	}
	if _, err := svc.Create(ctx, CreateTimeEntryInput{ProjectID: ptr("p1"), Source: domain.SourceAutomatic,
		StartedAt: "2026-10-06T07:00:00Z", EndedAt: "2026-10-06T08:00:00Z"}); err != nil {
		t.Errorf("Projekt-Eintrag: %v", err)
	}
}

func TestTimeCreateValidation(t *testing.T) {
	svc, m, _ := newTimeSvc(nil) // Uhr: 2026-10-06T09:00:00Z
	ctx := context.Background()
	const s, e = "2026-10-06T07:00:00Z", "2026-10-06T08:00:00Z"
	for name, in := range map[string]CreateTimeEntryInput{
		"ohne Besitzer":   {StartedAt: s, EndedAt: e},
		"zwei Besitzer":   {TaskID: ptr("t"), ProjectID: ptr("p"), StartedAt: s, EndedAt: e},
		"ohne Start":      {TaskID: ptr("t"), EndedAt: e},
		"ohne Ende":       {TaskID: ptr("t"), StartedAt: s},
		"Start kaputt":    {TaskID: ptr("t"), StartedAt: "gestern", EndedAt: e},
		"Ende kaputt":     {TaskID: ptr("t"), StartedAt: s, EndedAt: "heute"},
		"Ende vor Start":  {TaskID: ptr("t"), StartedAt: e, EndedAt: s},
		"Länge null":      {TaskID: ptr("t"), StartedAt: s, EndedAt: s},
		"Ende in Zukunft": {TaskID: ptr("t"), StartedAt: s, EndedAt: "2026-10-06T09:00:01Z"},
		"falsche Quelle":  {TaskID: ptr("t"), StartedAt: s, EndedAt: e, Source: "ROBOTER"},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if len(m.entries) != 0 {
		t.Errorf("Fehlerfälle haben Einträge angelegt: %+v", m.entries)
	}
}

type fakeStopper struct{ stopped []string }

func (f *fakeStopper) StopTimer(_ context.Context, id string) error {
	f.stopped = append(f.stopped, id)
	return nil
}

func TestTaskUpdateStopsTimerWhenLeavingInProgress(t *testing.T) {
	svc, _ := newTaskSvc()
	stopper := &fakeStopper{}
	svc.WithTimerStopper(stopper)
	ctx := context.Background()
	task, _ := svc.Create(ctx, CreateTaskInput{Title: "x", Status: domain.TaskInProgress})
	other, _ := svc.Create(ctx, CreateTaskInput{Title: "y", Status: domain.TaskPlanned})

	if _, err := svc.Update(ctx, task.ID, UpdateTaskInput{Title: ptr("neu")}); err != nil {
		t.Fatal(err)
	}
	same := domain.TaskInProgress
	if _, err := svc.Update(ctx, task.ID, UpdateTaskInput{Status: &same}); err != nil {
		t.Fatal(err)
	}
	toProgress := domain.TaskInProgress
	if _, err := svc.Update(ctx, other.ID, UpdateTaskInput{Status: &toProgress}); err != nil {
		t.Fatal(err)
	}
	if len(stopper.stopped) != 0 {
		t.Fatalf("Timer ohne Statusverlassen beendet: %v", stopper.stopped)
	}

	done := domain.TaskCompleted
	if _, err := svc.Update(ctx, task.ID, UpdateTaskInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
	if len(stopper.stopped) != 1 || stopper.stopped[0] != task.ID {
		t.Errorf("stopped = %v", stopper.stopped)
	}
	planned := domain.TaskPlanned
	if _, err := svc.Update(ctx, other.ID, UpdateTaskInput{Status: &planned}); err != nil {
		t.Fatal(err)
	}
	if len(stopper.stopped) != 2 {
		t.Errorf("IN_PROGRESS → PLANNED beendete den Timer nicht: %v", stopper.stopped)
	}
}

func TestTaskUpdateWithoutStopperStillWorks(t *testing.T) {
	svc, _ := newTaskSvc()
	task, _ := svc.Create(context.Background(), CreateTaskInput{Title: "x", Status: domain.TaskInProgress})
	done := domain.TaskCompleted
	if _, err := svc.Update(context.Background(), task.ID, UpdateTaskInput{Status: &done}); err != nil {
		t.Fatal(err)
	}
}
