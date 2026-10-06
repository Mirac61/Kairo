package domain

import "time"

// ProjectStatus ist der Lebenszyklus eines Projekts.
type ProjectStatus string

const (
	ProjectActive    ProjectStatus = "ACTIVE"
	ProjectPaused    ProjectStatus = "PAUSED"
	ProjectCompleted ProjectStatus = "COMPLETED"
	ProjectArchived  ProjectStatus = "ARCHIVED"
)

// Valid meldet, ob s ein bekannter Status ist.
func (s ProjectStatus) Valid() bool {
	switch s {
	case ProjectActive, ProjectPaused, ProjectCompleted, ProjectArchived:
		return true
	}
	return false
}

// Project bündelt Tasks und Ressourcen und kann mit einem lokalen Ordner
// (VSCodium-Workspace) verbunden sein.
type Project struct {
	ID          string
	Name        string
	Description string
	LocalPath   *string
	Status      ProjectStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
