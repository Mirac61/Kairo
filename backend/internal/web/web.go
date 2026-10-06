// Package web liefert die eingebettete WebUI aus.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

const notBuilt = "WebUI nicht gebaut — im Ordner frontend/ `npm run build` ausführen.\n"

// Handler liefert die eingebettete WebUI.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // dist ist zur Compile-Zeit eingebettet
	}
	return newHandler(sub)
}

func newHandler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/ws" || strings.HasPrefix(r.URL.Path, "/ws/") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(files, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(notBuilt))
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "."
		}
		if info, err := fs.Stat(files, name); err == nil && (name == "." || !info.IsDir()) {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA-Fallback
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
