package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, p string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
	return rec
}

func TestNotBuilt(t *testing.T) {
	rec := get(newHandler(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "WebUI nicht gebaut") {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestServeAndFallback(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":    {Data: []byte("INDEX")},
		"assets/app.js": {Data: []byte("JS")},
	})
	tests := []struct {
		path string
		code int
		body string
	}{
		{"/", 200, "INDEX"},
		{"/assets/app.js", 200, "JS"},
		{"/today", 200, "INDEX"},
		{"/assets", 200, "INDEX"},
		{"/api/nope", 404, ""},
	}
	for _, tt := range tests {
		rec := get(h, tt.path)
		if rec.Code != tt.code || (tt.body != "" && !strings.Contains(rec.Body.String(), tt.body)) {
			t.Errorf("%s: %d %q", tt.path, rec.Code, rec.Body.String())
		}
	}
}

func TestGzipAndCache(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":    {Data: []byte("INDEX")},
		"assets/app.js": {Data: []byte(strings.Repeat("console.log(1);", 100))},
		"icon.png":      {Data: []byte("PNG")},
	})
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("headers %v", rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := io.ReadAll(zr); string(body) != strings.Repeat("console.log(1);", 100) {
		t.Errorf("entpackt %q", body)
	}
	// ohne Accept-Encoding und für Bilder ungepackt, Bilder außerhalb von assets/ ohne Dauer-Cache
	if rec := get(h, "/assets/app.js"); rec.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(rec.Body.String(), "console") {
		t.Errorf("ohne gzip: %v", rec.Header())
	}
	req = httptest.NewRequest("GET", "/icon.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Cache-Control") != "" || rec.Body.String() != "PNG" {
		t.Errorf("png: %v %q", rec.Header(), rec.Body.String())
	}
}
