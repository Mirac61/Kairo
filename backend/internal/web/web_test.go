package web

import (
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
