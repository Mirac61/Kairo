package domain

import "time"

// TaskStatus ist der Bearbeitungsstand einer Task.
type TaskStatus string

const (
	TaskBacklog    TaskStatus = "BACKLOG"
	TaskPlanned    TaskStatus = "PLANNED"
	TaskInProgress TaskStatus = "IN_PROGRESS"
	TaskPaused     TaskStatus = "PAUSED"
	TaskCompleted  TaskStatus = "COMPLETED"
	TaskCancelled  TaskStatus = "CANCELLED"
)

// Valid meldet, ob s ein bekannter Status ist.
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskBacklog, TaskPlanned, TaskInProgress, TaskPaused, TaskCompleted, TaskCancelled:
		return true
	}
	return false
}

// TaskPriority ist die Wichtigkeit einer Task.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "LOW"
	PriorityMedium TaskPriority = "MEDIUM"
	PriorityHigh   TaskPriority = "HIGH"
	PriorityUrgent TaskPriority = "URGENT"
)

// Valid meldet, ob p eine bekannte Priorität ist.
func (p TaskPriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Task ist konkrete Arbeit, optional innerhalb eines Projekts.
type Task struct {
	ID               string
	Title            string
	Description      string
	Status           TaskStatus
	Priority         TaskPriority
	EstimatedMinutes int
	DueAt            *time.Time // UTC
	PlannedDate      *string    // lokaler Tag, YYYY-MM-DD
	PlannedStartAt   *time.Time // UTC, nur zusammen mit PlannedDate
	ProjectID        *string
	ParentTaskID     *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
}

// TaskFilter schränkt die Task-Liste ein. Leere Felder filtern nicht.
type TaskFilter struct {
	Status      TaskStatus
	ProjectID   string
	PlannedDate string
}
