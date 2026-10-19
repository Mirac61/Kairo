// Package importer stellt Daten aus einer Export-Datei (GET /api/export.json) wieder her.
// Es legt alle Objekte mit ihren ursprünglichen IDs an, in der Reihenfolge, in der die
// Fremdschlüssel erfüllbar sind (Projekte → Tasks → Habits → Completions → Termine →
// Zeiteinträge → Ressourcen). Für einen leeren Bestand gedacht; vorhandene IDs führen zu
// einem Fehler statt zu stillen Überschreibungen.
package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"kairo/internal/domain"
	"kairo/internal/repository"
)

// Result zählt die angelegten Objekte.
type Result struct {
	Projects         int `json:"projects"`
	Tasks            int `json:"tasks"`
	Habits           int `json:"habits"`
	HabitCompletions int `json:"habit_completions"`
	CalendarEvents   int `json:"calendar_events"`
	TimeEntries      int `json:"time_entries"`
	Resources        int `json:"resources"`
}

// data spiegelt export.json.
type data struct {
	ExportedAt       string            `json:"exported_at"`
	Projects         []project         `json:"projects"`
	Tasks            []task            `json:"tasks"`
	Habits           []habit           `json:"habits"`
	HabitCompletions []habitCompletion `json:"habit_completions"`
	CalendarEvents   []calendarEvent   `json:"calendar_events"`
	TimeEntries      []timeEntry       `json:"time_entries"`
	Resources        []resource        `json:"resources"`
}

type project struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	LocalPath   *string `json:"local_path"`
	GitHubRepo  *string `json:"github_repo"`
	Status      string  `json:"status"`
	Color       string  `json:"color"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type task struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority"`
	EstimatedMinutes int     `json:"estimated_minutes"`
	DueAt            *string `json:"due_at"`
	PlannedDate      *string `json:"planned_date"`
	PlannedStartAt   *string `json:"planned_start_at"`
	ProjectID        *string `json:"project_id"`
	ParentTaskID     *string `json:"parent_task_id"`
	RecurrenceRule   *string `json:"recurrence_rule"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	CompletedAt      *string `json:"completed_at"`
}

type habit struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	FrequencyType   string                 `json:"frequency_type"`
	FrequencyConfig domain.FrequencyConfig `json:"frequency_config"`
	TargetValue     *int                   `json:"target_value"`
	Unit            string                 `json:"unit"`
	PreferredTime   *string                `json:"preferred_time"`
	DurationMinutes int                    `json:"duration_minutes"`
	ProjectID       *string                `json:"project_id"`
	StartDate       string                 `json:"start_date"`
	EndDate         *string                `json:"end_date"`
	Active          bool                   `json:"active"`
	CreatedAt       string                 `json:"created_at"`
}

type habitCompletion struct {
	ID          string `json:"id"`
	HabitID     string `json:"habit_id"`
	Date        string `json:"date"`
	Value       *int   `json:"value"`
	CompletedAt string `json:"completed_at"`
	Note        string `json:"note"`
}

type calendarEvent struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	StartAt           string   `json:"start_at"`
	EndAt             string   `json:"end_at"`
	Location          string   `json:"location"`
	URL               string   `json:"url"`
	ProjectID         *string  `json:"project_id"`
	TaskID            *string  `json:"task_id"`
	RecurrenceRule    *string  `json:"recurrence_rule"`
	RecurrenceExdates []string `json:"recurrence_exdates"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type timeEntry struct {
	ID        string  `json:"id"`
	TaskID    *string `json:"task_id"`
	ProjectID *string `json:"project_id"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Source    string  `json:"source"`
}

type resource struct {
	ID        string  `json:"id"`
	TaskID    *string `json:"task_id"`
	ProjectID *string `json:"project_id"`
	Type      string  `json:"type"`
	Target    string  `json:"target"`
	Label     string  `json:"label"`
	CreatedAt string  `json:"created_at"`
}

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

func optTime(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := parseTime(*s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Import liest eine Export-Datei und legt die Objekte in db an.
func Import(ctx context.Context, db *sql.DB, r io.Reader) (Result, error) {
	var in data
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&in); err != nil {
		return Result{}, fmt.Errorf("export.json lesen: %w", err)
	}
	var out Result

	projects := repository.NewProjectRepository(db)
	for _, p := range in.Projects {
		created, err := parseTime(p.CreatedAt)
		if err != nil {
			return out, fmt.Errorf("Projekt %s: created_at: %w", p.ID, err)
		}
		updated, err := parseTime(p.UpdatedAt)
		if err != nil {
			return out, fmt.Errorf("Projekt %s: updated_at: %w", p.ID, err)
		}
		if err := projects.Create(ctx, domain.Project{
			ID: p.ID, Name: p.Name, Description: p.Description, LocalPath: p.LocalPath, GitHubRepo: p.GitHubRepo,
			Status: domain.ProjectStatus(p.Status), Color: p.Color, CreatedAt: created, UpdatedAt: updated,
		}); err != nil {
			return out, fmt.Errorf("Projekt %s: %w", p.ID, err)
		}
		out.Projects++
	}

	tasks := repository.NewTaskRepository(db)
	// Eltern vor Kindern (parent_task_id): in Runden anlegen, bis alles drin ist.
	remaining := make([]domain.Task, 0, len(in.Tasks))
	for _, t := range in.Tasks {
		created, err := parseTime(t.CreatedAt)
		if err != nil {
			return out, fmt.Errorf("Task %s: created_at: %w", t.ID, err)
		}
		updated, err := parseTime(t.UpdatedAt)
		if err != nil {
			return out, fmt.Errorf("Task %s: updated_at: %w", t.ID, err)
		}
		completed, err := optTime(t.CompletedAt)
		if err != nil {
			return out, fmt.Errorf("Task %s: completed_at: %w", t.ID, err)
		}
		due, err := optTime(t.DueAt)
		if err != nil {
			return out, fmt.Errorf("Task %s: due_at: %w", t.ID, err)
		}
		start, err := optTime(t.PlannedStartAt)
		if err != nil {
			return out, fmt.Errorf("Task %s: planned_start_at: %w", t.ID, err)
		}
		remaining = append(remaining, domain.Task{
			ID: t.ID, ProjectID: t.ProjectID, ParentTaskID: t.ParentTaskID, Title: t.Title, Description: t.Description,
			Status: domain.TaskStatus(t.Status), Priority: domain.TaskPriority(t.Priority), EstimatedMinutes: t.EstimatedMinutes,
			DueAt: due, PlannedDate: t.PlannedDate, PlannedStartAt: start, RecurrenceRule: t.RecurrenceRule,
			CreatedAt: created, UpdatedAt: updated, CompletedAt: completed,
		})
	}
	createdTasks := map[string]bool{}
	for len(remaining) > 0 {
		progress := false
		var next []domain.Task
		for _, t := range remaining {
			if t.ParentTaskID != nil && !createdTasks[*t.ParentTaskID] {
				next = append(next, t)
				continue
			}
			if err := tasks.Create(ctx, t); err != nil {
				return out, fmt.Errorf("Task %s: %w", t.ID, err)
			}
			createdTasks[t.ID] = true
			out.Tasks++
			progress = true
		}
		if !progress {
			return out, fmt.Errorf("Task-Verknüpfungen lassen sich nicht auflösen (Zyklus oder fehlende Eltern)")
		}
		remaining = next
	}

	habits := repository.NewHabitRepository(db)
	for _, h := range in.Habits {
		created, err := parseTime(h.CreatedAt)
		if err != nil {
			return out, fmt.Errorf("Habit %s: created_at: %w", h.ID, err)
		}
		if err := habits.Create(ctx, domain.Habit{
			ID: h.ID, Name: h.Name, Description: h.Description, FrequencyType: domain.FrequencyType(h.FrequencyType),
			FrequencyConfig: h.FrequencyConfig, TargetValue: h.TargetValue, Unit: h.Unit, PreferredTime: h.PreferredTime,
			DurationMinutes: h.DurationMinutes, ProjectID: h.ProjectID, StartDate: h.StartDate, EndDate: h.EndDate, Active: h.Active, CreatedAt: created,
		}); err != nil {
			return out, fmt.Errorf("Habit %s: %w", h.ID, err)
		}
		out.Habits++
	}

	for _, c := range in.HabitCompletions {
		completed, err := parseTime(c.CompletedAt)
		if err != nil {
			return out, fmt.Errorf("HabitCompletion %s: %w", c.ID, err)
		}
		if err := habits.CreateCompletion(ctx, domain.HabitCompletion{
			ID: c.ID, HabitID: c.HabitID, Date: c.Date, Value: c.Value, CompletedAt: completed, Note: c.Note,
		}); err != nil {
			return out, fmt.Errorf("HabitCompletion %s: %w", c.ID, err)
		}
		out.HabitCompletions++
	}

	calendar := repository.NewCalendarRepository(db)
	for _, e := range in.CalendarEvents {
		start, err := parseTime(e.StartAt)
		if err != nil {
			return out, fmt.Errorf("Termin %s: start_at: %w", e.ID, err)
		}
		end, err := parseTime(e.EndAt)
		if err != nil {
			return out, fmt.Errorf("Termin %s: end_at: %w", e.ID, err)
		}
		created, err := parseTime(e.CreatedAt)
		if err != nil {
			return out, fmt.Errorf("Termin %s: created_at: %w", e.ID, err)
		}
		updated, err := parseTime(e.UpdatedAt)
		if err != nil {
			return out, fmt.Errorf("Termin %s: updated_at: %w", e.ID, err)
		}
		if err := calendar.Create(ctx, domain.CalendarEvent{
			ID: e.ID, Title: e.Title, Description: e.Description, StartAt: start, EndAt: end, Location: e.Location, URL: e.URL,
			ProjectID: e.ProjectID, TaskID: e.TaskID, RecurrenceRule: e.RecurrenceRule, RecurrenceExdates: e.RecurrenceExdates,
			CreatedAt: created, UpdatedAt: updated,
		}); err != nil {
			return out, fmt.Errorf("Termin %s: %w", e.ID, err)
		}
		out.CalendarEvents++
	}

	timeRepo := repository.NewTimeEntryRepository(db)
	for _, e := range in.TimeEntries {
		start, err := parseTime(e.StartedAt)
		if err != nil {
			return out, fmt.Errorf("Zeiteintrag %s: started_at: %w", e.ID, err)
		}
		end, err := optTime(e.EndedAt)
		if err != nil {
			return out, fmt.Errorf("Zeiteintrag %s: ended_at: %w", e.ID, err)
		}
		if err := timeRepo.Create(ctx, domain.TimeEntry{
			ID: e.ID, TaskID: e.TaskID, ProjectID: e.ProjectID, StartedAt: start, EndedAt: end, Source: domain.TimeSource(e.Source),
		}); err != nil {
			return out, fmt.Errorf("Zeiteintrag %s: %w", e.ID, err)
		}
		out.TimeEntries++
	}

	resources := repository.NewResourceRepository(db)
	for _, r := range in.Resources {
		created, err := parseTime(r.CreatedAt)
		if err != nil {
			return out, fmt.Errorf("Ressource %s: created_at: %w", r.ID, err)
		}
		if err := resources.Create(ctx, domain.Resource{
			ID: r.ID, TaskID: r.TaskID, ProjectID: r.ProjectID, Type: domain.ResourceType(r.Type), Target: r.Target, Label: r.Label, CreatedAt: created,
		}); err != nil {
			return out, fmt.Errorf("Ressource %s: %w", r.ID, err)
		}
		out.Resources++
	}

	return out, nil
}
