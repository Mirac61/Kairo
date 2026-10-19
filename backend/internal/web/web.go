// Package web liefert die eingebettete WebUI aus.
package web

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed all:dist
var distFS embed.FS

const notBuilt = "WebUI nicht gebaut — im Ordner frontend/ `npm run build` ausführen.\n"

// Go kennt .webmanifest nicht; als text/plain ignorieren manche Browser das PWA-Manifest.
func init() { _ = mime.AddExtensionType(".webmanifest", "application/manifest+json") }

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
	var gz sync.Map // Dateiname -> gzip-Bytes, einmal gepackt und dann im Speicher
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
			// Vite hängt einen Hash an die Dateinamen in assets/: der Inhalt ändert sich nie, das Handy darf ihn behalten.
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			if serveGzip(w, r, files, name, &gz) {
				return
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA-Fallback
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

var packbar = map[string]bool{".js": true, ".mjs": true, ".css": true, ".html": true, ".svg": true, ".json": true, ".webmanifest": true}

// serveGzip liefert Text-Dateien gepackt aus (JS schrumpft auf etwa ein Drittel). Range-Anfragen gehen
// weiter an den FileServer, weil sich Byte-Bereiche auf die ungepackte Datei beziehen.
func serveGzip(w http.ResponseWriter, r *http.Request, files fs.FS, name string, cache *sync.Map) bool {
	if !packbar[path.Ext(name)] || r.Header.Get("Range") != "" || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	v, ok := cache.Load(name)
	if !ok {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return false
		}
		var b bytes.Buffer
		zw := gzip.NewWriter(&b)
		_, _ = zw.Write(data)
		_ = zw.Close()
		v, _ = cache.LoadOrStore(name, b.Bytes())
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(v.([]byte)))
	return true
}
