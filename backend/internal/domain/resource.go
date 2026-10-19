package domain

import "time"

// ResourceType ist die Art einer Ressource.
type ResourceType string

const (
	ResourceFile   ResourceType = "FILE"
	ResourceFolder ResourceType = "FOLDER"
	ResourceURL    ResourceType = "URL"
	ResourceNote   ResourceType = "NOTE" // Datei in Kairos Notizordner, Target relativ dazu
)

// Valid meldet, ob t ein bekannter Typ ist.
func (t ResourceType) Valid() bool {
	switch t {
	case ResourceFile, ResourceFolder, ResourceURL, ResourceNote:
		return true
	}
	return false
}

// Resource ist eine Referenz (kein Dateiinhalt) auf eine Datei, einen Ordner
// oder eine URL. Sie gehört genau einer Task oder genau einem Projekt.
type Resource struct {
	ID        string
	TaskID    *string
	ProjectID *string
	Type      ResourceType
	Target    string // Pfad (absolut oder mit ~), http(s)-URL bzw. bei NOTE Pfad im Notizordner
	Label     string
	CreatedAt time.Time
}

// ResourceFilter schränkt die Ressourcen-Liste ein. Leere Felder filtern nicht.
type ResourceFilter struct {
	TaskID    string
	ProjectID string
	Type      ResourceType
}
