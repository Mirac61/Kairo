package api

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"kairo/internal/domain"

	"kairo/internal/notes"
)

type noteHandlers struct{ store *notes.Store }

func (h noteHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/notes", h.tree)
	mux.HandleFunc("GET /api/notes/{path...}", h.read)
	mux.HandleFunc("PUT /api/notes/{path...}", h.save)
	mux.HandleFunc("POST /api/notes/{path...}", h.create)
	mux.HandleFunc("PATCH /api/notes/{path...}", h.move)
	mux.HandleFunc("DELETE /api/notes/{path...}", h.delete)
	mux.HandleFunc("GET /api/files/{path...}", h.file)
	mux.HandleFunc("POST /api/files/{path...}", h.upload)
	mux.HandleFunc("PUT /api/files/{path...}", h.replace)
}

func (h noteHandlers) tree(w http.ResponseWriter, _ *http.Request) {
	nodes, err := h.store.Tree()
	if err != nil {
		writeError(w, err)
		return
	}
	if nodes == nil {
		nodes = []notes.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (h noteHandlers) read(w http.ResponseWriter, r *http.Request) {
	p := r.PathValue("path")
	content, mtime, err := h.store.Read(p)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": p, "content": content, "mtime": mtime})
}

// save: mtime ist die Version beim Laden; weicht die Datei davon ab, antwortet es mit 409, außer bei force.
func (h noteHandlers) save(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Content string `json:"content"`
		Mtime   string `json:"mtime"`
		Force   bool   `json:"force"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	mtime, err := h.store.Save(r.PathValue("path"), in.Content, in.Mtime, in.Force)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mtime": mtime})
}

// create legt eine leere Notiz an, mit {"dir": true} einen Ordner.
func (h noteHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Dir bool `json:"dir"`
	}
	if !decodeOptionalJSON(w, r, &in) {
		return
	}
	p := r.PathValue("path")
	if in.Dir {
		if err := h.store.Mkdir(p); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"path": p})
		return
	}
	mtime, err := h.store.CreateFile(p)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": p, "mtime": mtime})
}

// move benennt um oder verschiebt: {"to": "neuer/pfad.md"}; 409, wenn das Ziel schon existiert.
func (h noteHandlers) move(w http.ResponseWriter, r *http.Request) {
	var in struct {
		To string `json:"to"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := h.store.Move(r.PathValue("path"), in.To); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": in.To})
}

// delete verschiebt nach .trash/ im Notizordner und nennt den neuen Ort.
func (h noteHandlers) delete(w http.ResponseWriter, r *http.Request) {
	dest, err := h.store.Delete(r.PathValue("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"trash": dest})
}

// file liefert ein PDF oder Bild aus dem Notizordner (für <img> und den PDF-Viewer).
func (h noteHandlers) file(w http.ResponseWriter, r *http.Request) {
	f, fi, err := h.store.Open(r.PathValue("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache") // nach dem Bearbeiten nie die alte Fassung aus dem Cache
	w.Header().Set("X-Kairo-Mtime", notes.Version(fi))
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

const maxUploadBytes = 200 << 20 // Vorlesungsskripte können groß sein

// upload legt ein PDF oder Bild an (Rohbytes im Body, z. B. ein eingefügter Screenshot); 409, wenn es schon existiert.
func (h noteHandlers) upload(w http.ResponseWriter, r *http.Request) {
	p := r.PathValue("path")
	body, ok := checkedBody(w, r, p)
	if !ok {
		return
	}
	if err := h.store.Upload(p, body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": p})
}

// replace speichert ein bearbeitetes PDF oder Bild: ?mtime=<Version beim Laden>, ?force=1 überschreibt trotzdem.
func (h noteHandlers) replace(w http.ResponseWriter, r *http.Request) {
	p := r.PathValue("path")
	body, ok := checkedBody(w, r, p)
	if !ok {
		return
	}
	q := r.URL.Query()
	mtime, err := h.store.Replace(p, body, q.Get("mtime"), q.Get("force") == "1")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mtime": mtime})
}

// checkedBody begrenzt den Body und prüft, dass der Inhalt zur Endung passt,
// damit keine HTML-Datei als .png durchrutscht.
func checkedBody(w http.ResponseWriter, r *http.Request, p string) (*bufio.Reader, bool) {
	body := bufio.NewReader(http.MaxBytesReader(w, r.Body, maxUploadBytes))
	head, _ := body.Peek(512)
	want := map[string]string{"pdf": "application/pdf", "image": "image/"}[notes.Kind(p)]
	if want == "" || !strings.HasPrefix(http.DetectContentType(head), want) {
		writeError(w, fmt.Errorf("%w: Inhalt passt nicht zu %q (erlaubt: PDF und Bilder)", domain.ErrInvalid, p))
		return nil, false
	}
	return body, true
}
