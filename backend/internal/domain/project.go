package domain

import (
	"slices"
	"time"
)

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

// ProjectColors ist die Palette für Projektfarben (Namen der Akzentfarben der WebUI), in der
// Reihenfolge, in der neue Projekte sie bekommen.
var ProjectColors = []string{"violet", "blue", "orange", "aqua", "pink", "yellow", "green", "red"}

// ValidProjectColor meldet, ob c in der Palette steht.
func ValidProjectColor(c string) bool { return slices.Contains(ProjectColors, c) }

// Project bündelt Tasks und Ressourcen und kann mit einem lokalen Ordner
// (VSCodium-Workspace) verbunden sein.
type Project struct {
	ID          string
	Name        string
	Description string
	LocalPath   *string
	Status      ProjectStatus
	Color       string // aus ProjectColors
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
