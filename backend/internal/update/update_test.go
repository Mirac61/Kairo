package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.2.0", "0.1.0", true},
		{"v0.2.0", "0.1.9", true},
		{"0.1.10", "0.1.9", true},
		{"1.0.0", "0.9.9", true},
		{"0.2.0-rc1", "0.1.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.2.0", false},
		{"v1.2.3", "1.2.3", false},
		{"0.2.0", "dev", false}, // eigener Build, nichts zu holen
		{"dev", "0.1.0", false}, // keine Fassung
		{"0.2", "0.1.0", false}, // unvollständig
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, erwartet %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("0.2.0", "darwin", "arm64"); got != "kairo_0.2.0_darwin_arm64.tar.gz" {
		t.Errorf("AssetName = %q", got)
	}
	if got := AssetName("v0.2.0", "windows", "amd64"); got != "kairo_0.2.0_windows_amd64.zip" {
		t.Errorf("AssetName = %q", got)
	}
}

func TestPiModel(t *testing.T) {
	if !isPiModel("Raspberry Pi 5 Model B Rev 1.0\x00") {
		t.Error("Pi-Modell nicht erkannt")
	}
	if isPiModel("Apple MacBook Pro (Mac14,7)") {
		t.Error("MacBook als Pi erkannt")
	}
}

func TestDisabled(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "KAIRO_UPDATE" {
				return v
			}
			return ""
		}
	}
	if !Disabled(env("off")) || !Disabled(env("false")) {
		t.Error("KAIRO_UPDATE=off schaltet Selbst-Updates nicht ab")
	}
	if Disabled(env("on")) {
		t.Error("KAIRO_UPDATE=on lässt Selbst-Updates aus")
	}
}

// releaseServer liefert /repos/<Repo>/releases/latest und die Dateien aus assets,
// dazu checksums.txt. badSum schreibt absichtlich falsche Prüfsummen.
func releaseServer(t *testing.T, version string, assets map[string][]byte, badSum bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		list := []map[string]string{{"name": "checksums.txt", "browser_download_url": srv.URL + "/dl/checksums.txt"}}
		for name := range assets {
			list = append(list, map[string]string{"name": name, "browser_download_url": srv.URL + "/dl/" + name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": version,
			"html_url": "https://github.com/" + Repo + "/releases/tag/" + version,
			"assets":   list,
		})
	})
	for name, body := range assets {
		mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	}
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		var b strings.Builder
		for name, body := range assets {
			sum := sha(body)
			if badSum {
				sum = strings.Repeat("0", 64)
			}
			fmt.Fprintf(&b, "%s  %s\n", sum, name)
		}
		_, _ = w.Write([]byte(b.String()))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestInstall(t *testing.T) {
	const name = "kairo_0.2.0_darwin_arm64.tar.gz"
	archive := tarGz(t, map[string]string{"kairo": "#!/bin/sh\necho neu\n", "README.md": "liesmich"})
	srv := releaseServer(t, "v0.2.0", map[string][]byte{name: archive}, false)

	exe := filepath.Join(t.TempDir(), "kairo")
	if err := os.WriteFile(exe, []byte("alt"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), GOOS: "darwin", GOARCH: "arm64",
		Executable: func() (string, error) { return exe, nil }}

	ctx := context.Background()
	rel, err := c.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.2.0" || rel.URL == "" {
		t.Fatalf("Release = %+v", rel)
	}
	st, err := c.Status(ctx, "0.1.0")
	if err != nil || !st.Available || st.Latest != "0.2.0" {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if st, _ = c.Status(ctx, "0.2.0"); st.Available {
		t.Error("gleiche Fassung gilt als Update")
	}

	got, err := c.Install(ctx, rel)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.2.0" {
		t.Errorf("installierte Fassung = %q", got)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "echo neu") {
		t.Errorf("Binary nicht ersetzt: %q", b)
	}
	fi, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	// Windows kennt kein Ausführungs-Bit; dort entscheidet die Endung .exe.
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("Binary nicht ausführbar: %v", fi.Mode())
	}
}

func TestInstallZip(t *testing.T) {
	const name = "kairo_0.2.0_windows_amd64.zip"
	archive := zipBytes(t, map[string]string{"kairo.exe": "neu.exe"})
	srv := releaseServer(t, "v0.2.0", map[string][]byte{name: archive}, false)

	exe := filepath.Join(t.TempDir(), "kairo.exe")
	if err := os.WriteFile(exe, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), GOOS: "windows", GOARCH: "amd64",
		Executable: func() (string, error) { return exe, nil }}

	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Install(context.Background(), rel); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(exe); err != nil || string(b) != "neu.exe" {
		t.Fatalf("Binary = %q, %v", b, err)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	const name = "kairo_0.2.0_darwin_arm64.tar.gz"
	archive := tarGz(t, map[string]string{"kairo": "neu"})
	srv := releaseServer(t, "v0.2.0", map[string][]byte{name: archive}, true)

	exe := filepath.Join(t.TempDir(), "kairo")
	if err := os.WriteFile(exe, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), GOOS: "darwin", GOARCH: "arm64",
		Executable: func() (string, error) { return exe, nil }}

	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Install(context.Background(), rel); err == nil {
		t.Fatal("falsche Prüfsumme wurde akzeptiert")
	}
	if b, _ := os.ReadFile(exe); string(b) != "alt" {
		t.Errorf("Binary trotz Fehler ersetzt: %q", b)
	}
}

func TestLatestMissingRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("404 als Release durchgereicht")
	}
}
