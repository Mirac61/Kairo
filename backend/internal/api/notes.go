package api

import (
	"net/http"

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
