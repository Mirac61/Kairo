// Package update prüft die neuesten Releases auf GitHub und ersetzt das laufende
// Kairo-Binary. Auf einem Raspberry Pi bleibt das aus, dort aktualisiert ./deploy.sh.
package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kairo/internal/domain"
)

// Repo ist das Repository, dessen Releases Kairo prüft.
const Repo = "Mirac61/Kairo"

// Asset ist eine Datei am Release.
type Asset struct {
	Name string
	URL  string
}

// Release ist eine veröffentlichte Fassung.
type Release struct {
	Version string // ohne führendes v, z. B. "0.2.0"
	URL     string // Seite auf GitHub
	Assets  []Asset
}

// Status beantwortet für die WebUI, ob es etwas zu holen gibt. Latest und URL
// bleiben leer, wenn Updates aus sind oder nichts geprüft wurde.
type Status struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
	Disabled  bool   `json:"disabled"`
}

// Client liest Releases und installiert sie. Die Felder sind für Tests austauschbar.
type Client struct {
	BaseURL string       // Standard https://api.github.com
	HTTP    *http.Client // Standard 2 Minuten, die Archive sind einige MB groß
	GOOS    string       // Standard runtime.GOOS
	GOARCH  string       // Standard runtime.GOARCH
	// Executable liefert den Pfad des laufenden Binaries; Standard os.Executable.
	Executable func() (string, error)
}

// New liefert einen Client für api.github.com.
func New() *Client {
	return &Client{BaseURL: "https://api.github.com", HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

// Latest liest die neueste veröffentlichte Fassung (ohne Vorabversionen und Entwürfe).
func (c *Client) Latest(ctx context.Context) (Release, error) {
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.getJSON(ctx, "/repos/"+Repo+"/releases/latest", &body); err != nil {
		return Release{}, err
	}
	rel := Release{Version: strings.TrimPrefix(body.TagName, "v"), URL: body.HTMLURL}
	for _, a := range body.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL})
	}
	return rel, nil
}

// Status vergleicht die neueste Fassung mit der laufenden.
func (c *Client) Status(ctx context.Context, current string) (Status, error) {
	rel, err := c.Latest(ctx)
	if err != nil {
		return Status{Current: current}, err
	}
	return Status{Current: current, Latest: rel.Version, Available: Newer(rel.Version, current), URL: rel.URL}, nil
}

// Install lädt das Archiv für dieses System, prüft es gegen checksums.txt und
// ersetzt das laufende Binary. Eine laufende Instanz nutzt es nach einem Neustart.
func (c *Client) Install(ctx context.Context, rel Release) (string, error) {
	name := AssetName(rel.Version, c.goos(), c.goarch())
	archive, ok := rel.asset(name)
	if !ok {
		return "", fmt.Errorf("%w: Release %s enthält %s nicht", domain.ErrUnavailable, rel.Version, name)
	}
	sums, ok := rel.asset("checksums.txt")
	if !ok {
		return "", fmt.Errorf("%w: Release %s enthält keine checksums.txt", domain.ErrUnavailable, rel.Version)
	}
	want, err := c.sum(ctx, sums.URL, name)
	if err != nil {
		return "", err
	}

	exe, err := c.executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	// Reste eines abgebrochenen Laufs liegen lassen wäre nur Ballast.
	if leftovers, err := filepath.Glob(filepath.Join(dir, ".kairo-update-*")); err == nil {
		for _, f := range leftovers {
			_ = os.Remove(f)
		}
	}
	_ = os.Remove(exe + ".alt") // Windows: das alte Binary, solange es noch lief

	dl, err := os.CreateTemp("", "kairo-archive-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dl.Name())
	hash := sha256.New()
	if err := c.download(ctx, archive.URL, io.MultiWriter(dl, hash)); err != nil {
		dl.Close()
		return "", err
	}
	if err := dl.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return "", fmt.Errorf("%w: Prüfsumme von %s passt nicht (%s statt %s)", domain.ErrUnavailable, name, got, want)
	}

	// Erst neben das Binary schreiben und am Ende umbenennen, damit ein Abbruch
	// das laufende Kairo nicht beschädigt.
	tmp, err := os.CreateTemp(dir, ".kairo-update-*")
	if err != nil {
		return "", fmt.Errorf("%w: %s ist nicht beschreibbar: %v", domain.ErrUnavailable, dir, err)
	}
	defer os.Remove(tmp.Name())
	if err := extract(dl.Name(), c.goos(), tmp); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := swap(exe, tmp.Name()); err != nil {
		return "", fmt.Errorf("%w: %s ersetzen: %v", domain.ErrUnavailable, exe, err)
	}
	return rel.Version, nil
}

// AssetName ist der Dateiname, den .goreleaser.yaml für dieses System baut.
func AssetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("kairo_%s_%s_%s.%s", strings.TrimPrefix(version, "v"), goos, goarch, ext)
}

// Newer sagt, ob latest eine höhere Fassung als current ist. Unbekannte
// Angaben (z. B. „dev“ aus einem eigenen Build) gelten nie als neuer.
func Newer(latest, current string) bool {
	l, ok := parse(latest)
	if !ok {
		return false
	}
	c, ok := parse(current)
	if !ok {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parse liest major.minor.patch; ein führendes v und Vorab-Kennungen fallen weg.
func parse(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// piModelPaths nennen das Gerätebaum-Modell auf Linux; auf macOS gibt es sie nicht.
var piModelPaths = []string{"/proc/device-tree/model", "/sys/firmware/devicetree/base/model"}

// Disabled sagt, ob Selbst-Updates aus sind: per KAIRO_UPDATE (off/on), sonst
// automatisch auf einem Raspberry Pi. Dort aktualisiert ./deploy.sh.
func Disabled(getenv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv("KAIRO_UPDATE"))) {
	case "off", "false", "0", "no":
		return true
	case "on", "true", "1", "yes":
		return false
	}
	return IsRaspberryPi()
}

// IsRaspberryPi erkennt den Pi am Gerätebaum-Modell; andere ARM-Rechner nicht.
func IsRaspberryPi() bool {
	for _, p := range piModelPaths {
		if b, err := os.ReadFile(p); err == nil && isPiModel(string(b)) {
			return true
		}
	}
	return false
}

func isPiModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "raspberry pi")
}

func (rel Release) asset(name string) (Asset, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		return "https://api.github.com"
	}
	return strings.TrimSuffix(c.BaseURL, "/")
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: 2 * time.Minute}
	}
	return c.HTTP
}

func (c *Client) goos() string {
	if c.GOOS == "" {
		return runtime.GOOS
	}
	return c.GOOS
}

func (c *Client) goarch() string {
	if c.GOARCH == "" {
		return runtime.GOARCH
	}
	return c.GOARCH
}

func (c *Client) executable() (string, error) {
	f := c.Executable
	if f == nil {
		f = os.Executable
	}
	path, err := f()
	if err != nil {
		return "", err
	}
	// Eine Verknüpfung (z. B. /usr/local/bin/kairo) ersetzt das Ziel, nicht den Link.
	return filepath.EvalSymlinks(path)
}

func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	res, err := c.request(ctx, c.baseURL()+path)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(dst); err != nil {
		return fmt.Errorf("%w: GitHub-Antwort lesen: %v", domain.ErrUnavailable, err)
	}
	return nil
}

// sum liest die Prüfsumme von name aus checksums.txt.
func (c *Client) sum(ctx context.Context, url, name string) (string, error) {
	res, err := c.request(ctx, url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: checksums.txt lesen: %v", domain.ErrUnavailable, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%w: checksums.txt nennt %s nicht", domain.ErrUnavailable, name)
}

func (c *Client) download(ctx context.Context, url string, dst io.Writer) error {
	res, err := c.request(ctx, url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if _, err := io.Copy(dst, res.Body); err != nil {
		return fmt.Errorf("%w: Download abgebrochen: %v", domain.ErrUnavailable, err)
	}
	return nil
}

func (c *Client) request(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kairo")
	res, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: GitHub nicht erreichbar: %v", domain.ErrUnavailable, err)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("%w: GitHub antwortet %d", domain.ErrUnavailable, res.StatusCode)
	}
	return res, nil
}

// extract schreibt das Binary kairo (bzw. kairo.exe) aus dem Archiv nach dst.
func extract(archive, goos string, dst io.Writer) error {
	if goos == "windows" { // .goreleaser.yaml packt Windows als zip, alles andere als tar.gz
		return extractZip(archive, "kairo.exe", dst)
	}
	bin := "kairo"
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: Archiv lesen: %v", domain.ErrUnavailable, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: Archiv lesen: %v", domain.ErrUnavailable, err)
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == bin {
			_, err := io.Copy(dst, tr)
			return err
		}
	}
	return fmt.Errorf("%w: %s fehlt im Archiv", domain.ErrUnavailable, bin)
}

func extractZip(path, bin string, dst io.Writer) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: Archiv lesen: %v", domain.ErrUnavailable, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != bin {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("%w: Archiv lesen: %v", domain.ErrUnavailable, err)
		}
		defer rc.Close()
		_, err = io.Copy(dst, rc)
		return err
	}
	return fmt.Errorf("%w: %s fehlt im Archiv", domain.ErrUnavailable, bin)
}
