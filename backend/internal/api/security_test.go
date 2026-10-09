package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testToken = "s3cret"

func TestSecurity(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := Security(8742, testToken, ok)

	tests := []struct {
		name    string
		method  string
		path    string
		host    string
		headers map[string]string
		want    int
	}{
		{"GET host 127", "GET", "/api/x", "127.0.0.1:8742", nil, 204},
		{"GET host localhost", "GET", "/api/x", "localhost:8742", nil, 204},
		{"GET bad host", "GET", "/api/x", "evil.example", nil, 403},
		{"GET wrong port", "GET", "/api/x", "127.0.0.1:1", nil, 403},
		{"GET no port", "GET", "/", "127.0.0.1", nil, 403},
		{"GET foreign origin ok (safe)", "GET", "/api/x", "127.0.0.1:8742", map[string]string{"Origin": "http://evil.example"}, 204},
		{"HEAD safe", "HEAD", "/", "localhost:8742", nil, 204},
		{"OPTIONS safe", "OPTIONS", "/api/x", "localhost:8742", nil, 204},
		{"POST no origin/token", "POST", "/api/x", "127.0.0.1:8742", nil, 403},
		{"POST own origin 127", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Origin": "http://127.0.0.1:8742"}, 204},
		{"POST own origin localhost", "POST", "/api/x", "localhost:8742", map[string]string{"Origin": "http://localhost:8742"}, 204},
		{"POST foreign origin", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Origin": "http://evil.example"}, 403},
		{"POST foreign origin + good token", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Origin": "http://evil.example", "Authorization": "Bearer " + testToken}, 204},
		{"POST good token", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Authorization": "Bearer " + testToken}, 204},
		{"POST bad token", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Authorization": "Bearer nope"}, 401},
		{"POST non-bearer auth", "POST", "/api/x", "127.0.0.1:8742", map[string]string{"Authorization": "Basic abc"}, 403},
		{"POST good token bad host", "POST", "/api/x", "evil.example", map[string]string{"Authorization": "Bearer " + testToken}, 403},
		{"DELETE no auth", "DELETE", "/api/x", "127.0.0.1:8742", nil, 403},
		{"PUT good token", "PUT", "/api/x", "127.0.0.1:8742", map[string]string{"Authorization": "Bearer " + testToken}, 204},
		{"WS upgrade no origin", "GET", "/ws", "127.0.0.1:8742", map[string]string{"Upgrade": "websocket"}, 403},
		{"WS upgrade foreign origin", "GET", "/ws", "127.0.0.1:8742", map[string]string{"Upgrade": "websocket", "Origin": "http://evil.example"}, 403},
		{"WS upgrade own origin", "GET", "/ws", "127.0.0.1:8742", map[string]string{"Upgrade": "websocket", "Origin": "http://localhost:8742"}, 204},
		{"WS upgrade token", "GET", "/api/ws", "127.0.0.1:8742", map[string]string{"Upgrade": "websocket", "Authorization": "Bearer " + testToken}, 204},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Host = tt.host
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("Status = %d, want %d", rec.Code, tt.want)
			}
			for k := range rec.Header() {
				if strings.HasPrefix(strings.ToLower(k), "access-control-") {
					t.Errorf("CORS-Header gesendet: %s", k)
				}
			}
		})
	}
}

func TestHealth(t *testing.T) {
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{})
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Host = "127.0.0.1:8742"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok","version":"dev"}` {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestLoadOrCreateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "token")
	tok, err := LoadOrCreateToken(path)
	if err != nil || len(tok) != 64 {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	// Windows kennt keine Unix-Dateirechte; dort schützt das Nutzerprofil die Datei.
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("Datei-Modus %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Dir(path)); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Errorf("Dir-Modus %v", fi.Mode().Perm())
	}
	again, err := LoadOrCreateToken(path)
	if err != nil || again != tok {
		t.Errorf("zweiter Lauf: %q %v", again, err)
	}
	if err := os.WriteFile(path, []byte("  abc \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadOrCreateToken(path); got != "abc" {
		t.Errorf("trim: %q", got)
	}
}
