package domain

import (
	"fmt"
	"strings"
	"time"
)

// GoalKind sagt, woraus der Fortschritt eines Ziels berechnet wird.
type GoalKind string

const (
	GoalNone         GoalKind = "NONE"          // ohne Fortschritt, nur Bild und Warum
	GoalProjectTasks GoalKind = "PROJECT_TASKS" // erledigte / alle Tasks des Projekts
	GoalProjectHours GoalKind = "PROJECT_HOURS" // erfasste Stunden im Projekt seit StartDate, Target in Stunden
	GoalHabitCount   GoalKind = "HABIT_COUNT"   // Completions der Gewohnheit seit StartDate, Target als Anzahl
)

// Goal ist ein großes Ziel auf dem Motivationsboard.
type Goal struct {
	ID        string
	Title     string
	Why       string
	ImagePath *string // Pfad im Notizordner (über /api/files)
	Kind      GoalKind
	ProjectID *string
	HabitID   *string
	Target    int
	StartDate string // YYYY-MM-DD, ab hier zählen Stunden und Completions
	DueDate   *string
	Archived  bool
	ParentID  *string // gesetzt: Teilziel eines Langzeitziels
	CreatedAt time.Time
}

// Validate prüft das Ziel und entfernt Verweise, die zur Art nicht passen.
func (g *Goal) Validate() error {
	g.Title = strings.TrimSpace(g.Title)
	if g.Title == "" {
		return fmt.Errorf("%w: title fehlt", ErrInvalid)
	}
	if g.Kind == "" {
		g.Kind = GoalNone
	}
	switch g.Kind {
	case GoalNone:
		g.ProjectID, g.HabitID, g.Target = nil, nil, 0
	case GoalProjectTasks, GoalProjectHours:
		g.HabitID = nil
		if g.ProjectID == nil {
			return fmt.Errorf("%w: project_id fehlt", ErrInvalid)
		}
	case GoalHabitCount:
		g.ProjectID = nil
		if g.HabitID == nil {
			return fmt.Errorf("%w: habit_id fehlt", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unbekannte kind %q", ErrInvalid, g.Kind)
	}
	if g.Kind == GoalProjectTasks {
		g.Target = 0
	}
	if (g.Kind == GoalProjectHours || g.Kind == GoalHabitCount) && g.Target <= 0 {
		return fmt.Errorf("%w: target muss größer als 0 sein", ErrInvalid)
	}
	for k, v := range map[string]*string{"start_date": &g.StartDate, "due_date": g.DueDate} {
		if v == nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", *v); err != nil {
			return fmt.Errorf("%w: %s muss YYYY-MM-DD sein", ErrInvalid, k)
		}
	}
	return nil
}
