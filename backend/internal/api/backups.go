package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kairo/internal/domain"
)

// Backups verbindet /api/backups mit dem Backup-Ordner. Create schreibt einen
// Snapshot nach dest (repository.Backup) und überschreibt nie eine Datei.
// Einspielen bleibt bewusst im Terminal (kairo restore), nicht über die API.
type Backups struct {
	Dir    string
	Create func(ctx context.Context, dest string) error
}

type backupDTO struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

type backupHandlers struct{ b *Backups }

func (h backupHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/backups", h.list)
	mux.HandleFunc("POST /api/backups", h.create)
}

func (h backupHandlers) list(w http.ResponseWriter, _ *http.Request) {
	files, err := filepath.Glob(filepath.Join(h.b.Dir, "kairo-*.db"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]backupDTO, 0, len(files))
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			out = append(out, dtoOf(fi))
		}
	}
	// Der Name enthält die Zeit, also sortiert der Name auch nach Alter (neueste zuerst).
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	writeJSON(w, http.StatusOK, out)
}

func (h backupHandlers) create(w http.ResponseWriter, r *http.Request) {
	dest := filepath.Join(h.b.Dir, "kairo-"+time.Now().Format("20060102-150405")+".db")
	if err := h.b.Create(r.Context(), dest); err != nil {
		if strings.Contains(err.Error(), "existiert bereits") { // zweimal in derselben Sekunde
			err = domain.ErrConflict
		}
		writeError(w, err)
		return
	}
	fi, err := os.Stat(dest)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dtoOf(fi))
}

func dtoOf(fi os.FileInfo) backupDTO {
	return backupDTO{Name: fi.Name(), Size: fi.Size(), CreatedAt: fi.ModTime().UTC().Format(time.RFC3339)}
}
